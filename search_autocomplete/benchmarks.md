# naive version

**The "10 vs 100 queries" comparison won't prove what you think it will.** Both the naive and optimized versions scale _linearly_ in the number of queries — 100 queries is ~10× the time of 10 queries in both. The thing that distinguishes them is the **per-query cost**, and that varies with _subtree size_, not query count. So the benchmark that tells the real story is one that varies the prefix: `"a"` (huge subtree → slow DFS) vs a rare long prefix (tiny subtree → fast DFS). The optimized version shows the same flat `ns/op` for both.

# results

## 1. The header block (machine context)

```
goos: linux        → host OS (for reproducibility)
goarch: amd64      → CPU architecture
pkg: github.com/arjun118/autocomplete   → package tested
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz   → machine this ran on
```

This block exists so you never mistake _machine speed_ for _code speed_. Never compare `ns/op` numbers across different machines.

## 2. One benchmark row, column by column

```
BenchmarkSuggest/prefix_1char-8   96    12945574 ns/op    494847 B/op    50781 allocs/op
        └─ subtest name   └─┘    └─┘       └─┘             └─┘           └─┘
                          │       │          │               │              │
                       GOMAXPROCS  b.N     time per call    bytes per call  allocations per call
```

| Column            | Meaning                                                                                              | Your value |
| ----------------- | ---------------------------------------------------------------------------------------------------- | ---------- |
| `prefix_1char`    | The subtest name (the `tc.name` from your benchmark table)                                           | —          |
| `-8`              | 8 logical CPUs available (4 cores × hyperthreading on your i7). **Not** "8 goroutines ran parallel." | 8          |
| `96`              | `b.N` — how many times the harness called `Suggest` in this run                                      | 96         |
| `12945574 ns/op`  | **The headline number.** Average wall time per call ≈ 12.9 ms. Smaller is better.                    | ~12.9 ms   |
| `494847 B/op`     | Average bytes allocated on the heap per call ≈ 483 KB. Smaller is better (less GC pressure).         | ~483 KB    |
| `50781 allocs/op` | Average number of heap allocations per call. GC cost tracks this more than total bytes.              | ~50.8 K    |

## 3. Why `b.N` differs wildly between rows (96 vs 1,000,000)

The harness doesn't decide `b.N` — it **auto-tunes** it. The rule is: _double `b.N` until the benchmark has run for roughly 1 second, then report._

- `prefix_1char`: each call costs ~12 ms, so ~1s ÷ 12ms ≈ **96 iterations**. Done after ~1.2s.
- `prefix_rare`: each call costs ~1 µs, so it had to run **1,000,000** iterations to fill 1 second.

So a tiny `b.N` is itself diagnostic: **it means your operation is expensive.** You don't "fix" `b.N`; it fixes itself in response to your code's speed. That's why `-count=3` shows 96, 96, 98 — the auto-tuner settling at slightly different thresholds across runs.

## 4. What your data actually says

| Subtest                 | Time/op     | Allocs/op | Re-read as                                      |
| ----------------------- | ----------- | --------- | ----------------------------------------------- |
| `prefix_1char` ("a")    | **12.9 ms** | 50,781    | Massive subtree → DFS visits thousands of nodes |
| `prefix_2char` ("ca")   | **2.0 ms**  | 12,234    | Subtree ~6× smaller → ~6× faster                |
| `prefix_rare` ("zzz")   | **1.07 µs** | 14        | Tiny subtree → nothing to traverse              |
| `exact_word` ("google") | **13.6 µs** | 127       | Small subtree → cheap, but not free             |

Three things to notice:

**a) Cost scales with subtree size, not prefix length.** "a" is 1 char and takes 12 ms; "zzz" is 3 chars and takes 1 µs. The input lengths are nearly identical — the subtree sizes are 12,000× apart. This is _the_ visual proof that your `Suggest` is O(subtree), exactly what you wanted to demonstrate.

**b) 50,781 allocs/op tells you where the cost lives.** That's (approximately) the number of words under `"a"`. Every `DFS` visit allocates: the `newPrefix := prefix + string(key)` string, plus `heap.Push(hp, Reco{...})` which boxes the `Reco` into `any` and grows the slice. The memory numbers are your optimization roadmap: kill the temp strings and heap churn, and the time follows.

**c) The spread 12.9 ms → 1.07 µs is the benchmark to keep.** When you build the optimized version (precomputed top-k per node), these rows should collapse to roughly equal flat values — all near the `prefix_rare` scale, regardless of subtree size. That flatness _is_ the O(1) claim.

# cache top-k words at individual trie nodes

## command

❯ go test -bench=BenchmarkSuggest -benchmem -run=^$ -count=3

```bash
❯ go test -bench=BenchmarkSuggest -benchmem -run=^$ -count=3
goos: linux
goarch: amd64
pkg: github.com/arjun118/autocomplete
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz
BenchmarkSuggest/prefix_1char-8         	10753712	      145.4 ns/op	    160 B/op	      1 allocs/op
BenchmarkSuggest/prefix_1char-8         	10695302	      147.9 ns/op	    160 B/op	      1 allocs/op
BenchmarkSuggest/prefix_1char-8         	10694947	      152.5 ns/op	    160 B/op	      1 allocs/op
BenchmarkSuggest/prefix_2char-8         	9267542	      153.5 ns/op	    160 B/op	      1 allocs/op
BenchmarkSuggest/prefix_2char-8         	9432664	      151.3 ns/op	    160 B/op	      1 allocs/op
BenchmarkSuggest/prefix_2char-8         	9550872	      149.9 ns/op	    160 B/op	      1 allocs/op
BenchmarkSuggest/prefix_rare-8          	11165498	      117.3 ns/op	     64 B/op	      1 allocs/op
BenchmarkSuggest/prefix_rare-8          	11976854	      119.4 ns/op	     64 B/op	      1 allocs/op
BenchmarkSuggest/prefix_rare-8          	11766018	      117.5 ns/op	     64 B/op	      1 allocs/op
BenchmarkSuggest/exact_word-8           	5405605	      238.3 ns/op	    160 B/op	      1 allocs/op
BenchmarkSuggest/exact_word-8           	5333444	      234.5 ns/op	    160 B/op	      1 allocs/op
BenchmarkSuggest/exact_word-8           	5586363	      252.5 ns/op	    160 B/op	      1 allocs/op
PASS
ok  	github.com/arjun118/autocomplete	30.589s
```

## explanation

This is the Phase 3 gate being blown out of the water. Let me put the numbers in context against your Phase 1 baseline:

| prefix       | Phase 1                       | Now                  | Speedup      | Allocs     |
| ------------ | ----------------------------- | -------------------- | ------------ | ---------- |
| `1char`      | 12,945,574 ns / 50,781 allocs | **145 ns** / 1 alloc | **~89,000×** | 50,781 → 1 |
| `2char`      | 2,351,634 ns / 12,235 allocs  | **150 ns** / 1 alloc | ~15,000×     | 12,235 → 1 |
| `rare`       | 1,064 ns / 14 allocs          | **117 ns** / 1 alloc | ~9×          | 14 → 1     |
| `exact_word` | 13,626 ns / 127 allocs        | **238 ns** / 1 alloc | ~57×         | 127 → 1    |

## Why this is the right result

**1. The flatness claim is now provable.** 117–153 ns across a 1-char prefix (huge subtree), a rare prefix (tiny subtree), and a 2-char prefix — the ~30 ns spread is map-lookup + slice-copy noise, not subtree size. That's exactly the Phase 3 acceptance criterion: _"the three rows become roughly equal — flatness is the proof."_ Your README's target was `< 5 µs / < 10 allocs`; you're at 150 ns / 1 alloc, ~30× under target.

**2. `exact_word` at 238 ns is the diagnostic detail.** It's slower than the others because it does the longest `Search` walk (6 rune→node map lookups for `google`) and copies a full 10-entry result. The extra ~90 ns is the prefix walk, which is O(|prefix|) — the theoretical floor. Perfectly explained, not a red flag.

**3. Alloc collapse is the bigger story than the time.** 50,781 → 1 alloc. The one remaining alloc is `result := make([]string, 0, len(node.Top))` in the cached path — the API returns `[]string`, so it must copy out of `node.Top`. You _could_ get to 0 allocs by having `Suggest` return `[]Reco` directly (no copy, but callers could mutate the cache — you'd need the clone back), or by reusing a caller-provided buffer. Not worth it for the toy; worth a README note.

## Two honest caveats

**a) Verify `exact_word` is a hit, not a miss.** The row uses prefix `"google"` — if `google` isn't in `out.txt`, that 238 ns is measuring the _miss_ path (`Search` fails → return nil), which would make the row meaningless. Quick check: `go run . -sq google` — if it prints suggestions, it's a hit. (Phase 1's `exact_word` was 13.6 µs with 127 allocs, which _looks_ like a real hit then; but re-verify since the dataset may have changed.)

**b) Cached and DFS paths now intentionally diverge.** Your `TraverseAndBuild` includes `node.IsWord` self-suggestions (good — that's the README's optional improvement), but `DFS` still doesn't. So `Suggest("google", true)` ≠ `Suggest("google", false)` for exact words. That's fine — just don't write the Phase 3 equivalence test expecting them to match byte-for-byte; it must exclude exact-word cases.
