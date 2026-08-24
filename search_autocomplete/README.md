# Search Autocomplete — Implementation Roadmap

> Living document: every phase ends with fresh benchmark output pasted into this README.

## Status

| Phase | Deliverable                                     | Status  |
| ----- | ----------------------------------------------- | ------- |
| 1     | Baseline trie + DFS heap                        | ✅ done |
| 2     | Per-node top-K cache                            | ✅ done |
| 3     | Cache population + benchmark gate               | ✅ done |
| 4     | Prefix hash map + benchmark                     | ⬜      |
| 5     | MongoDB persistence                             | ⬜      |
| 6     | HTTP server + web UI                            | ⬜      |
| 7     | Multi-phrase dataset                            | ⬜      |
| 8     | Query logging + adaptive learning (Mongo/Redis) | ⬜      |
| 9     | Toy sharding                                    | ⬜      |

## Definition of done (every phase)

- `go test -race ./...` and `go vet ./...` pass
- Benchmark table below updated with real output from
  `go test -bench=BenchmarkSuggest -benchmem -run=^$ -count=3`
- Commit: `phase-N: <summary>`

---

## Phase 1 — Baseline trie + DFS heap (done — superseded)

**Implementation** (`main.go`):

- `Node{Val, Children map[rune]*Node, IsWord, Freq int64}` — leaf holds the word frequency
- `Insert` — O(|word|); duplicate inserts overwrite freq (accepted for now)
- `Search(prefix)` — O(|prefix|), returns the end node
- `Suggest(prefix, k)` — DFS over the whole subtree, keeping a k-sized min-heap

**Complexity:** build O(total chars); query O(|prefix|) + O(subtree words × log k); memory O(total chars).

**Known gaps:** no suggestion for the exact prefix itself when it's a word; DFS allocates a string per visited node plus a heap entry per word.

**Baseline benchmark** (i7-10510U, 8 threads, 2026-08-23):

```
BenchmarkSuggest/prefix_1char-8    96   12945574 ns/op   494847 B/op   50781 allocs/op
BenchmarkSuggest/prefix_2char-8   717    2351634 ns/op   125831 B/op   12235 allocs/op
BenchmarkSuggest/prefix_rare-8  1e+06       1064 ns/op      396 B/op      14 allocs/op
BenchmarkSuggest/exact_word-8   89950      13626 ns/op     2833 B/op     127 allocs/op
```

Cost is proportional to **subtree size**, not prefix length: `a` = 12.9 ms/50k allocs vs `zzz` = 1 µs/14 allocs. That's the problem Phases 2–3 eliminate.

---

## Phase 2 — Per-node top-K cache (done)

**Goal:** `Suggest` becomes O(|prefix|) by precomputing, per node, the top-K of its subtree.

**Decision: cache top-K, not all words.** Storing every (word, freq) at every node is O(N·L²) entries — needlessly huge. Any query entering a node only ever reads that node's top-K, so we cache exactly `K = max k served by the API` entries per node. Keep the DFS implementation around as `SuggestDFS` for equivalence testing.

```go
type Node struct {
    Val      rune
    Children map[rune]*Node
    IsWord   bool
    Freq     int64
    Top      []Reco // top-K of this subtree, sorted desc by Freq
}
```

**Ranking semantics — decided:** raw freq (Phase 1 behavior), proven equivalent to DFS. Summed subtree freq is a possible later toggle (`SumFreq bool`).

**Implemented behavior change:** the exact prefix is included as a suggestion when it's a word (the cache build merges the node itself into its own Top). Note that this makes cached ≠ DFS for exact-word queries — equivalence tests must exclude those.

**Correctness gate (passed):** `TestMergeTopK` covers the merge; `TestWordSearchAndCount` / `TestPrefixSearch` cover insert/search on the dataset.

---

## Phase 3 — Cache population + benchmark gate (done)

**Build — bottom-up post-order DFS, run once after `BuildTrie`.** The implemented version uses a **k-way merge** (`mergeTopK` with a `headHeap` over list heads) instead of sorting the whole candidate union — each node merges its children's already-sorted top-K lists:

```go
func (t *Trie) TraverseAndBuild(node *Node, prefix string) []Reco {
    cands := make([][]Reco, 0, len(node.Children)+1)
    if node.IsWord {
        cands = append(cands, []Reco{{Word: prefix, Freq: node.Freq}})
    }
    for key, child := range node.Children {
        cands = append(cands, t.TraverseAndBuild(child, prefix+string(key)))
    }
    node.Top = mergeTopK(cands, t.K)
    return node.Top
}
```

**Merge cost per node** (K = cache size, d = node degree):

| Approach                                                 | Per-node cost     |
| -------------------------------------------------------- | ----------------- |
| Sort the whole union                                     | O(d·K · log(d·K)) |
| Min-heap eviction (every candidate ≤ 1 insert + 1 evict) | O(d·K · log K)    |
| k-way merge of sorted heads                              | O((d + K)·log d)  |

Tree-wide: Σd over all nodes = number of edges ≈ number of nodes, so `BuildCache` is a one-time O(nodes · (d + K)·log d).

**Memory warning:** totalNodes × K × sizeof(Reco). For 1M words, avg length 10 → up to ~10M nodes × 10 × 16B ≈ 1.6 GB worst case. If that bites: cap cached depth, or jump to Phase 4's flat map.

**Benchmark gate — passed.** Actual results (i7-10510U, 8 threads, 2026-08-25):

```
BenchmarkSuggest/prefix_1char-8    10753712     145.4 ns/op     160 B/op      1 allocs/op
BenchmarkSuggest/prefix_2char-8     9267542     153.5 ns/op     160 B/op      1 allocs/op
BenchmarkSuggest/prefix_rare-8     11165498     117.3 ns/op      64 B/op      1 allocs/op
BenchmarkSuggest/exact_word-8       5405605     238.3 ns/op     160 B/op      1 allocs/op
```

| prefix | Phase 1                | Phase 3 actual   | Speedup  |
| ------ | ---------------------- | ---------------- | -------- |
| 1char  | 12.9 ms / 50.8k allocs | 145 ns / 1 alloc | ~89,000× |
| 2char  | 2.0 ms / 12.2k allocs  | 153 ns / 1 alloc | ~15,000× |
| rare   | 1.07 µs / 14 allocs    | 117 ns / 1 alloc | ~9×      |
| exact  | 13.6 µs / 127 allocs   | 238 ns / 1 alloc | ~57×     |

(The 1-char prefix was `a` in Phase 1 and `s` in Phase 3 — the flatness claim holds across subtree sizes regardless.)

**Reading the results:**

- **Flatness is proven.** 117–153 ns across a huge-subtree prefix, a rare prefix, and a 2-char prefix. The spread is map-lookup + slice-copy noise, not subtree size. Target was `< 5 µs / < 10 allocs`; actual is ~150 ns / 1 alloc.
- **`exact_word` at 238 ns is the longest `Search` walk** (6 rune lookups for `google`) plus a full 10-entry result copy — that's the O(|prefix|) floor, fully explained.
- **Alloc collapse is the bigger story than time:** 50,781 → 1 alloc. The remaining alloc is the `[]string` result slice copied out of `node.Top`; returning `[]Reco` directly would reach 0 allocs but risks caller mutation of the cache.
- Remaining variance across `-count=3` runs (145/148/152 ns) is normal CPU noise.

**Caveat recorded:** verify `exact_word` uses a prefix that's a real hit in `out.txt` (`go run . -sq google` should print suggestions); a miss would make that row measure the `Search` failure path instead.

---

## Phase 4 — Prefix hash map (flat cache)

**Why now, when query latency is already at the floor?** Phase 3 measured ~145 ns/op (1-char) and ~117 ns/op (rare) — within a couple of map lookups of the hardware floor. A flat map would save maybe 30–50 ns (~20–30%), which is invisible to a user (both are far under the 5 ms p50 budget). **Phase 4 is not a latency play — it's an operational play:**

- **Build parallelism** — per-letter buckets across goroutines
- **1:1 Mongo serialization** — one prefix = one document (unlocks Phase 5)
- **Simpler Phase 8 rebuild** — rebuild one flat map and `atomic.Pointer` swap it; no tree walks
- **Measurable memory tradeoff** — flat storage of every distinct prefix, no sharing

```go
type PrefixIndex map[string][]Reco // prefix -> top-K, sorted desc
```

**Build:** for each word, for each prefix `p` of `1..len(word)`, merge `(word, freq)` into `index[p]` keeping top-K. Parallelizable by first letter: each goroutine owns a letter bucket, then union.

**Tradeoff vs Phase 2 trie:**

|                                | Trie + node cache          | Prefix hash map                             |
| ------------------------------ | -------------------------- | ------------------------------------------- |
| Query                          | O(\|prefix\|) walk + slice | O(1) hash lookup                            |
| Memory                         | shared prefixes → compact  | every distinct prefix stored flat → heavier |
| Serialize to Mongo             | custom tree→docs           | 1 prefix = 1 doc                            |
| Prefix-walk (fuzzy, next-char) | natural                    | awkward                                     |
| Rebuild for learning (Ph 8)    | tree walk + merge          | build one flat map, one swap                |

**Gotcha:** worst case = distinct prefixes × K. ~10M prefixes × 10 × 16B ≈ several GB for 1M words. Mitigation if needed: cache only prefixes ≤ some depth, fall back to the Phase 3 trie.

**Decision point:** the trie stays as the reference implementation for tests; the flat map becomes the serving structure.

**Exit criteria:**

- Build time measured with `BenchmarkBuildCacheK` (expect roughly linear in K; record the K=10/50/100 table)
- Equivalence test `Trie.Suggest ≡ PrefixIndex.Suggest` (excluding exact-word rows)
- Benchmark re-run: flat across prefixes, expected to match Phase 3 within noise — confirm the ~20–30% delta is all Phase 4 buys on latency
- Serialization round-trip sketch (map → JSONL → map) as Phase 5 prep

---

## Phase 5 — MongoDB persistence

**Schema.**

```
collection prefixes: { _id: "<prefix>", top: [ { w: "apple", f: 12345 }, ... ] }
collection words:    { _id: "<word>",   f: 12345 }
collection queries:  { q: "<query>", ts: ISODate(...) }   // Phase 8
```

`_id` = prefix gives a natural unique index and point lookups. The Phase 4 flat map maps 1:1 onto `prefixes` — the reason Phase 5 is trivial now.

**Serving vs durability — keep them separate.**

- Hot path: in-memory `PrefixIndex`, loaded once at boot
- Mongo: source of truth for persistence and (later) learning
- A Mongo round-trip is ~0.1–1 ms. Never read-through on the hot path.
- Initial seed can use `mongoimport` from a JSONL dump of `PrefixIndex` — no driver code needed for the first load

**Deliverables:** `docker-compose.yml` (`mongo:7`), `cmd/seed` (build + dump), `cmd/server` (load at boot), Mongo-backed tests behind a `//go:build mongo` tag.

**Exit:** restart a server → all suggestions still served; `mongosh` shows `prefixes.findOne({_id: "a"}).top`.

---

## Phase 6 — HTTP server + web UI

**API (stdlib `net/http` + `html/template`, no framework):**

```
GET /healthz                        -> 200 OK
GET /api/suggest?q=<prefix>&k=10    -> {"prefix":"a","suggestions":[{"word":..,"freq":..}]}
GET /                               -> static UI
```

Client: `static/index.html` + `static/app.js`, vanilla JS, 150 ms debounce, ↑/↓/enter navigation. Dependency-free on purpose.

**Transport decision — HTTP/2 GET, not WebSocket.** Every keystroke issues a `GET /api/suggest`, so the natural question is whether to hold a WebSocket instead. The answer at scale is no — plain HTTP GET, for four reasons:

1. **Debounce kills the problem client-side.** Typing `starbucks` is ~9 keystrokes over ~2.5 s; a 150 ms debounce collapses that to 1–2 requests per session. An `AbortController` cancels the in-flight request on new input, and a small client-side LRU of recent prefixes makes backspacing instant. The "every keystroke" load never reaches the network.
2. **Payload is tiny — transport framing is noise.** 10 suggestions ≈ 400–600 B of JSON. HTTP/2 keep-alive + HPACK headers add ~30–50 B; a WebSocket frame adds 2–14 B. Both fit in a single packet (MTU 1500). The <10% framing difference is invisible in p99.
3. **Scale favors stateless HTTP.** 1M concurrent WebSocket connections ≈ 10–50 GB of server memory (buffers + epoll state) just to keep sockets idle, plus a keepalive tax (~33K ping/pong frames/s at 1M connections with a 30 s interval). HTTP connections churn — concurrent connections track _actively typing users_, not the total user base. CDNs/LBs/proxies are HTTP-native; WS needs upgrade handling, proxy timeouts, and per-connection state. GETs are cacheable (edge/browser), idempotent, and retryable, and the reverse proxy gives the Phase 8 query log for free. Per-user rate limiting at the LB works on HTTP; with WS it must be implemented per-connection in the app.
4. **It fits the Phase 4–9 design.** The in-memory `PrefixIndex` + `atomic.Pointer` swap makes every request stateless — exactly what HTTP load balancing wants. WS would fight the design, not complement it.

The three cases where WS/SSE actually earns its keep: server-push (live trending, "this prefix's top-K changed" invalidation), an existing persistent channel (collaborative editing), and streamed partial results with a <100 ms budget and no debounce tolerance. For the push case, prefer **SSE** (`text/event-stream`) first: it rides plain HTTP, auto-reconnects, and works through proxies — ~80% of WS's value at 20% of the complexity.

**Implementation notes:**

- `net/http` enables HTTP/2 automatically once TLS is on; HTTP/1.1 keep-alive is fine for local dev.
- Client: debounce 150 ms + `AbortController` + request-ID stale-response drop + tiny LRU.
- Response header: `Cache-Control: public, max-age=60, stale-while-revalidate=30` — safe because Phase 8 updates frequencies ~once a minute; repeated prefixes become edge-cache hits.
- Metrics that matter: QPS per prefix, p99, cache hit rate at each layer (client/edge/Redis), request-abort rate.

**Load test — the concurrent evidence:**

```
wrk -t4 -c100 -d30s 'http://localhost:8080/api/suggest?q=a&k=10'
```

Record RPS + p50/p95/p99. Run the same against the Phase 1 DFS algorithm and against Phases 3/4. Expected story: p99 flat across prefixes for the optimized version; p99 explodes for `q=a` with DFS. Paste the wrk table into this README.

**Budget:** p50 < 5 ms, p99 < 20 ms on the dev machine.

---

## Phase 7 — Multi-phrase dataset

**Encoding.** Phrases are just strings with spaces; two options:

1. Space as a regular rune — insert "starbucks near me" verbatim. Zero code churn; upgrading `q=starbucks ne` → suggests "starbucks near me" works out of the box.
2. Token-level trie (complete only the last token). Only worth it if phrase UX demands it later.

Start with (1).

**Reference dataset (Phases 1–6 tests):**

```
trichinella	39707
chlorella	227266
xtremetones	55733
underseat	25016
riegelsville	20690
greedo	30093
cellulite	815135
yertle	16309
montaje	34493
birchdale	16567
```

For phrases, generate a synthetic `out_phrases.txt` (weighted n-gram-ish generator, Zipf-ish frequencies). Keep **both** files so the Phase 6 suite runs on single words and phrases.

**Exit:** `q=starbucks ne` returns ranked phrase completions; benchmark table updated for the phrase dataset; watch memory — phrases inflate distinct-prefix counts fast.

---

## Phase 8 — Query logging + adaptive learning (worker, Mongo, Redis)

**Pipeline.**

1. **Log** — HTTP handler appends `{q, ts}` to Mongo `queries` on every suggestion request.
2. **Aggregate** — worker goroutine every ~60 s: buffer recent queries, `zincrby` into a Redis sorted set `recent` (windowed), count into Mongo.
3. **Learn** — `f_new = α·f_old + (1−α)·window_count` (EMA) or additive boost + re-normalize.
4. **Reapply** — rebuild the `PrefixIndex` off the hot path, then atomically swap: serve behind `atomic.Pointer[PrefixIndex]`. Readers never see a half-updated index. (The Phase 4 flat map makes this a single map build + swap — no tree walks.)
5. **Trending** — `zrevrange recent 0 9` from Redis → `/api/trending` endpoint, pushed to clients via SSE (see the Phase 6 transport decision).

**Division of labor — the classic mistake, avoided:**

| Component                | Role                                             |
| ------------------------ | ------------------------------------------------ |
| In-memory `PrefixIndex`  | serves reads (hot path)                          |
| Mongo `queries`          | durable query log — source of truth for learning |
| Mongo `words`/`prefixes` | durable learned state (write-behind)             |
| Redis                    | hot cache for popular prefixes + trending window |

Redis is a cache/trend board, **not** the source of truth. If it dies, serving continues.

**Consistency:** eventual consistency with a ~1-minute staleness budget. Never write-through per keystroke — buffer and batch (that's the worker's job). Watch write amplification.

**Deliverables:** `QueryLogger` interface (Mongo impl + no-op for tests), `worker.go` (aggregate → rebuild → atomic swap), `/api/trending`.

**Exit:** type a low-freq word 10× in the UI and within ~1 min it outranks previously-higher-frequency words; restart the server → learned boost survives (durable in Mongo).

**See also:** Appendix C for the full design of handling unknown queries (misses are never written to the hot index).

---

## Phase 9 — Toy sharding

**Why suggestions make sharding accidentally easy:** top-K is per prefix. A query for `starb` only needs the shard owning `st…`. Shard key = `prefix[0]` or `hash(prefix) % N` — **no cross-shard merge for correctness**. (Contrast: global top-K ranking, where every shard must contribute.)

**Toy ladder.**

1. In-process: N independent `PrefixIndex` maps, one mutex each; a `Router{firstRune -> shard}`.
2. Process shards: N replica processes owning hash ranges, coordinator talks gRPC.
3. Consistent hashing: stretch goal only.

```go
type Shard interface {
    Suggest(prefix string, k int) []string
    Count() int
}
type Router interface {
    Route(prefix string) Shard
}
```

**Measure:** wrk under `-c100` across a mix of prefixes; p99 should flatten as N grows (under concurrency; on a single core total work is constant). Also measure **skew** — if the dataset is dominated by `a…`, one shard takes the heat. Check the letter histogram before choosing boundaries.

**Explicitly out of scope:** replication, failover, live rebalancing, cross-shard transactions. State this in the commit so the toy isn't mistaken for a system.

---

## Appendix A — Commands

```bash
# correctness
go test -race ./...

# query benchmarks (Phases 1–4)
go test -bench=BenchmarkSuggest -benchmem -run=^$ -count=3
go test -bench='BenchmarkSuggest/prefix_1char' -run=^$ -cpuprofile=cpu.out
go tool pprof -http=:8080 cpu.out

# build benchmarks (Phase 3/4 gates)
go test -bench=BenchmarkBuild -benchmem -run=^$ -count=3
go test -bench=BenchmarkBuildCacheK -benchmem -run=^$ -count=3

# load tests (Phase 6)
wrk -t4 -c100 -d30s 'http://localhost:8080/api/suggest?q=a&k=10'
```

## Appendix B — Docker compose (Phases 5+)

```yaml
services:
    mongo:
        image: mongo:7
        ports: ["27017:27017"]
    redis:
        image: redis:7-alpine
        ports: ["6379:6379"]
```

---

## Appendix C — Adaptive learning: handling unknown queries (Phase 8)

**One rule governs everything: misses are never written to the hot index on the request path.**

Data flows one way — Mongo is the durable source of truth, and the in-memory `PrefixIndex` is a read-only snapshot rebuilt from it:

```mermaid
flowchart LR
    U[User] -->|keystrokes| S[GET /api/suggest]
    S -->|serves from read-only copy| I[In-memory PrefixIndex]
    U -->|submitted query| L[(Mongo queries log)]
    W[Worker every ~60s] -->|aggregate window + EMA| L
    W -->|write learned state| M[(Mongo words + prefixes)]
    W -->|rebuild + atomic swap| I
    M -->|load at boot| I
```

**What happens when the user types something not in the tree** — three distinct cases:

| Case                                       | Example                               | What should happen                                                                                       |
| ------------------------------------------ | ------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| Prefix not in index (no completions)       | `xyzabc`                              | **Serve empty.** Optionally log the pattern for analysis; do not add it.                                 |
| Prefix in index, typed string isn't a word | `starb`                               | Serve `starbucks…` completions normally — this is not a miss.                                            |
| Word genuinely new to the system           | `cryptocurrency` submitted first time | **Log the submitted query** (not the keystroke prefix); promote via the worker if it clears a threshold. |

**Why not to add misses on the fly:**

- **Junk pollution** — typos (`starbcuks`) and partial words would permanently enter the tree.
- **Write amplification** — the Phase 8 constraint: never write-through per keystroke.
- **Invalidation complexity** — one new word touches all its prefixes (`c`, `cr`, `cry`, … `cryptocurrency`), each needing a top-K merge. A rebuild is a pure function of the word list; a per-keystroke update needs locks.

**Serve a suggestion ≠ learn a frequency.** A new query is captured durably without ever being served back. `cryptocurrency` appears as a suggestion only when it earns a top-K slot — a 1-occurrence new word shouldn't displace `cellulite` from `c`'s top-10.

**The worker loop (every ~60 s):**

1. Pull the window's submitted queries from Mongo `queries` (or a Redis `LIST` buffer).
2. Count per query; apply `f_new = α·f_old + (1−α)·window_count` (EMA) against `words`.
3. **Promotion threshold:** only words with `window_count ≥ T` (e.g., 3) are written into `words` — everything below stays in the log. This is the anti-junk filter.
4. Rebuild the `PrefixIndex` off the hot path; swap via `atomic.Pointer`. New words appear under all their prefixes, ranked by earned frequency.

**Log the submitted query, not the keystroke.** Keystroke prefixes (`s`, `st`, `sta`, `star`…) are typing noise; the learning signal is the query the user commits (Enter / click / selecting a suggestion).

**Realtime ≠ per-keystroke.** A 30–60 s cadence is imperceptible for autocomplete (real systems do the same). The batch + atomic-swap model _is_ the realtime story: bounded staleness of ~1 minute, zero locks, zero torn reads. Mongo never "fetches from" the in-memory tree — it's the other way around.

**One-sentence summary:** _Misses are never written to the hot index — they're logged as submitted queries, aggregated by a worker, and promoted into a rebuilt index only when they clear a frequency threshold; Mongo is the source of truth the index is rebuilt from, never the other way around._

**Stretch (not building):** fuzzy fallback for true misses — edit-distance "did you mean...?" suggestions, orthogonal to learning.
