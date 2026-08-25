# Benchmark sink reasoning: why the `sink` variable is mandatory

**TL;DR** — The sink did not make the benchmark "faster". It made `prefix_rare`
**honest**: without the sink, Go's compiler inlined `PrefixIndex.Suggest` and
deleted the result-copy as dead code (0 B/op, 0 allocs/op — a fake 16 ns).
With the sink, the same call does real work (16 B/op, 1 alloc, 73 ns). The
apparent trie speedup between the two runs is CPU/thermal noise on a laptop,
not the sink.

---

## The two runs (verbatim)

### Run A — WITHOUT sink (`_ = pi.Suggest(...)` / `_ = trie.Suggest(...)`)

```
go test -bench='BenchmarkTrieSuggest|BenchmarkPrefixSuggest' -benchmem -run=^$ -count=3 -cpu=2
goos: linux
goarch: amd64
pkg: github.com/arjun118/autocomplete/internal/trie
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz
BenchmarkTrieSuggest/prefix_1char-2         	7171614	      310.4 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_1char-2         	11808368	      171.2 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_1char-2         	12780792	      165.2 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_2char-2         	9579828	      203.1 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_2char-2         	10065880	      206.2 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_2char-2         	9329958	      208.1 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_rare-2          	14275260	       71.31 ns/op	     16 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_rare-2          	14833747	       77.96 ns/op	     16 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_rare-2          	14350573	       73.26 ns/op	     16 B/op	      1 allocs/op
BenchmarkTrieSuggest/exact_word-2           	4291854	      248.1 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/exact_word-2           	6120339	      227.0 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/exact_word-2           	6323928	      242.0 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_1char-2       	13098910	      273.5 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_1char-2       	12896571	      265.5 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_1char-2       	13225305	      267.7 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_2char-2       	12397136	      278.9 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_2char-2       	13086118	      274.8 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_2char-2       	12655172	      272.4 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_rare-2        	73039725	       16.78 ns/op	      0 B/op	      0 allocs/op
BenchmarkPrefixSuggest/prefix_rare-2        	73612224	       16.24 ns/op	      0 B/op	      0 allocs/op
BenchmarkPrefixSuggest/prefix_rare-2        	72345986	       16.84 ns/op	      0 B/op	      0 allocs/op
BenchmarkPrefixSuggest/exact_word-2         	9659181	      266.5 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/exact_word-2         	13548766	      274.9 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/exact_word-2         	12237631	      277.4 ns/op	    160 B/op	      1 allocs/op
PASS
ok  	github.com/arjun118/autocomplete/internal/trie	155.762s
```

### Run B — WITH sink (`sink = pi.Suggest(...)` / `sink = trie.Suggest(...)`)

```
go test -bench='BenchmarkTrieSuggest|BenchmarkPrefixSuggest' -benchmem -run=^$ -count=3 -cpu=2
goos: linux
goarch: amd64
pkg: github.com/arjun118/autocomplete/internal/trie
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz
BenchmarkTrieSuggest/prefix_1char-2         	15101312	      149.9 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_1char-2         	14998068	      140.5 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_1char-2         	15554541	      148.1 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_2char-2         	10823641	      190.7 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_2char-2         	10589275	      198.8 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_2char-2         	9419569	      222.1 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_rare-2          	15371299	       80.32 ns/op	     16 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_rare-2          	15242122	       72.71 ns/op	     16 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_rare-2          	14825407	       73.68 ns/op	     16 B/op	      1 allocs/op
BenchmarkTrieSuggest/exact_word-2           	5668614	      227.5 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/exact_word-2           	6014971	      232.3 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/exact_word-2           	5705245	      218.2 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_1char-2       	13454767	      275.0 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_1char-2       	12844561	      260.3 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_1char-2       	11140789	      300.0 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_2char-2       	13303856	      276.3 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_2char-2       	13279201	      268.0 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_2char-2       	13104896	      269.0 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_rare-2        	21465465	       72.99 ns/op	     16 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_rare-2        	21855114	       73.38 ns/op	     16 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_rare-2        	20710724	       73.95 ns/op	     16 B/op	      1 allocs/op
BenchmarkPrefixSuggest/exact_word-2         	10193330	      266.9 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/exact_word-2         	13714969	      283.7 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/exact_word-2         	12827846	      253.7 ns/op	    160 B/op	      1 allocs/op
PASS
ok  	github.com/arjun118/autocomplete/internal/trie	151.496s
```

---

## What the sink does (the mechanics)

Go's escape analysis + dead-code elimination work on the _call site_, not the
function. Consider the no-sink loop:

```go
for range b.N {
    _ = pi.Suggest(tc.prefix)   // result discarded
}
```

`PrefixIndex.Suggest` is small enough to be **inlined**. Once inlined, the
compiler can see that the returned `[]string` is never used — so the entire
`make + append` copy loop is **dead code and is deleted**. Only the map probe
`pi[prefix]` survives. Result: 0 B/op, 0 allocs/op, 16 ns. You are timing a
map lookup with the actual suggestion-building removed.

A package-level sink forces the result to escape, making the copy real:

```go
var sink []string // package level, in trie_test.go

for range b.N {
    sink = pi.Suggest(tc.prefix)
}
_ = sink
```

---

## Before/after per subtest (medians)

| subtest         | without sink         | with sink            | change          | meaning                    |
| --------------- | -------------------- | -------------------- | --------------- | -------------------------- |
| trie 1char      | ~171 ns              | ~148 ns              | noise           | trie was never elided      |
| trie 2char      | ~206 ns              | ~199 ns              | noise           | trie was never elided      |
| trie rare       | ~74 ns               | ~74 ns               | none            | trie was never elided      |
| trie exact      | ~239 ns              | ~227 ns              | noise           | trie was never elided      |
| **prefix rare** | **~16 ns / 0 alloc** | **~73 ns / 1 alloc** | **fake → real** | the sink's one real effect |
| prefix 1char    | ~267 ns              | ~278 ns              | noise           | —                          |
| prefix 2char    | ~275 ns              | ~271 ns              | noise           | —                          |
| prefix exact    | ~273 ns              | ~268 ns              | noise           | —                          |

**Why only `prefix_rare` changed:** the compiler's elision kicked in for the
prefix index path (small function, inlinable). `Trie.Suggest` calls `Search`
and was never inlined, so its copy was always real — which is why the trie
columns are unchanged except for run-to-run noise.

---

## Why the trie "got faster" in Run B — it didn't

Compare the trie rows across the two runs: the differences (310→150, 171→141,
165→148, 248→228…) are **measurement noise**, not the sink:

- These runs take **~150 s each** (`-count=3` on an i7-10510U laptop). Across
  2.5 minutes the CPU turbo/thermal state drifts substantially; the first
  `count` iteration of each subtest (310.4 ns in Run A) is a cold-start outlier.
- The sink cannot speed up the trie — it only _adds_ a store per iteration. The
  trie path was never dead-code-eliminated.
- The honest way to compare runs is interleaving subtests across runs, or
  `-benchtime` tuned so all subtests see the same thermal window — but even
  then, ±10–20% on this hardware is expected.

---

## The honest comparison (Run B, with sink)

| prefix                    | trie    | prefix index | gap (prefix − trie) |
| ------------------------- | ------- | ------------ | ------------------- |
| `s` (1 char)              | ~146 ns | ~278 ns      | **+132 ns**         |
| `ca` (2 chars)            | ~204 ns | ~271 ns      | +67 ns              |
| `google` (6 chars)        | ~226 ns | ~268 ns      | +42 ns              |
| `zzz` (3 chars, 1 result) | ~74 ns  | ~73 ns       | ≈ 0                 |

Reading it:

1. **The gap shrinks as the prefix grows** — the trie's per-rune walk
   accumulates (≈15–20 ns/char) while the flat map's probe cost stays roughly
   fixed. Crossover is at ~10+ chars, beyond realistic autocomplete prefixes.
2. **`zzz` ties at ~73 ns** — with a 1-element result, the giant-map probe and
   the 3-rune walk cost the same. The gap only appears with larger result sets
   (the 10-entry copies for `s`/`ca`/`google`).
3. **The prefix index loses on every realistic prefix.** Its value is not query
   latency — it's operational: 1:1 Mongo serialization (Phase 5) and the
   single-map rebuild + `atomic.Pointer` swap for Phase 8 (Phase 7 rationale).

This confirms the README's original framing: _Phase 4/7 is not a latency play._

---

## Benchmark methodology checklist (for this repo)

- Always assign results to a package-level `var sink []string`; never discard
  with `_ =` when the function is small enough to inline.
- Use `-benchmem` and watch `B/op` + `allocs/op` as the sanity signal: if a
  row that should copy data shows **0 allocs**, the compiler elided the work.
- Use `-count=3` and report medians, not the first iteration (cold-start
  outliers like 310.4 ns are common).
- Don't compare absolute ns across separate runs on a laptop; compare
  relative rows within one run, or interleave.
- If two structures should differ by a small constant, isolate the phases:
  probe-only vs copy-only microbenchmarks, plus `-cpuprofile` + `pprof`.

---

## Addendum: `-cpu=8` (default GOMAXPROCS) — observations

Two more runs, this time with the **default `-cpu=8`**. Block 1 = **with sink**, Block 2 = **without sink**. Same command, back to back:

```
go test -bench='BenchmarkTrieSuggest|BenchmarkPrefixSuggest' -benchmem -run=^$ -count=3
goos: linux

goarch: amd64
pkg: github.com/arjun118/autocomplete/internal/trie
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz
BenchmarkTrieSuggest/prefix_1char-8          	8313627	      166.2 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_1char-8          	9217764	      164.5 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_1char-8          	10102975	      150.0 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_2char-8          	8461904	      186.2 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_2char-8          	8343760	      185.4 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_2char-8          	8261077	      182.8 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_rare-8           	14458071	       73.17 ns/op	     16 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_rare-8           	15542614	       72.42 ns/op	     16 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_rare-8           	12382308	       81.87 ns/op	     16 B/op	      1 allocs/op
BenchmarkTrieSuggest/exact_word-8            	5160271	      220.4 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/exact_word-8            	5185106	      218.0 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/exact_word-8            	5072271	      235.0 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_1char-8        	12632188	      157.0 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_1char-8        	13170243	      152.7 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_1char-8        	11812705	      163.6 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_2char-8        	11628352	      165.6 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_2char-8        	12557434	      158.9 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_2char-8        	11611401	      159.1 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_rare-8         	23639101	       52.08 ns/op	     16 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_rare-8         	22215320	       52.26 ns/op	     16 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_rare-8         	22300806	       51.66 ns/op	     16 B/op	      1 allocs/op
BenchmarkPrefixSuggest/exact_word-8          	9362522	      153.6 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/exact_word-8          	12004828	      159.8 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/exact_word-8          	12056437	      160.0 ns/op	    160 B/op	      1 allocs/op
PASS
ok  	github.com/arjun118/autocomplete/internal/trie	90.082s
```

```
go test -bench='BenchmarkTrieSuggest|BenchmarkPrefixSuggest' -benchmem -run=^$ -count=3
goos: linux

goarch: amd64
pkg: github.com/arjun118/autocomplete/internal/trie
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz
BenchmarkTrieSuggest/prefix_1char-8          	12304264	      114.8 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_1char-8          	13570704	      133.3 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_1char-8          	15247839	      142.2 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_2char-8          	8055848	      179.5 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_2char-8          	8380276	      176.1 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_2char-8          	8542744	      174.4 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_rare-8           	15816219	       70.68 ns/op	     16 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_rare-8           	15768669	       71.36 ns/op	     16 B/op	      1 allocs/op
BenchmarkTrieSuggest/prefix_rare-8           	15935294	       70.22 ns/op	     16 B/op	      1 allocs/op
BenchmarkTrieSuggest/exact_word-8            	5229427	      217.4 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/exact_word-8            	5163703	      217.2 ns/op	    160 B/op	      1 allocs/op
BenchmarkTrieSuggest/exact_word-8            	5192467	      214.7 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_1char-8        	12476772	      156.4 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_1char-8        	12692688	      155.8 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_1char-8        	11310906	      153.2 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_2char-8        	12282344	      159.6 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_2char-8        	11292914	      154.9 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_2char-8        	13004048	      155.5 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/prefix_rare-8         	64429239	       18.49 ns/op	      0 B/op	      0 allocs/op
BenchmarkPrefixSuggest/prefix_rare-8         	63540753	       18.99 ns/op	      0 B/op	      0 allocs/op
BenchmarkPrefixSuggest/prefix_rare-8         	61885514	       20.67 ns/op	      0 B/op	      0 allocs/op
BenchmarkPrefixSuggest/exact_word-8          	9314761	      149.4 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/exact_word-8          	12382381	      154.9 ns/op	    160 B/op	      1 allocs/op
BenchmarkPrefixSuggest/exact_word-8          	12855638	      156.9 ns/op	    160 B/op	      1 allocs/op
PASS
ok  	github.com/arjun118/autocomplete/internal/trie	93.360s
```

### Medians

| subtest      | Block 1 (with sink)        | Block 2 (without sink)       |
| ------------ | -------------------------- | ---------------------------- |
| trie 1char   | 164.5 ns / 160 B / 1 alloc | 133.3 ns / 160 B / 1 alloc   |
| trie 2char   | 185.4 ns / 160 B / 1 alloc | 176.1 ns / 160 B / 1 alloc   |
| trie rare    | 73.2 ns / 16 B / 1 alloc   | 70.7 ns / 16 B / 1 alloc     |
| trie exact   | 220.4 ns / 160 B / 1 alloc | 217.2 ns / 160 B / 1 alloc   |
| prefix 1char | 157.0 ns / 160 B / 1 alloc | 155.8 ns / 160 B / 1 alloc   |
| prefix 2char | 159.1 ns / 160 B / 1 alloc | 155.5 ns / 160 B / 1 alloc   |
| prefix rare  | 52.1 ns / 16 B / 1 alloc   | 19.0 ns / **0 B / 0 allocs** |
| prefix exact | 159.8 ns / 160 B / 1 alloc | 154.9 ns / 160 B / 1 alloc   |

### Observations

1. **The sink effect is reproduced at `-cpu=8`.** `prefix_rare` is 52 ns / 1 alloc
   with sink vs 19 ns / 0 allocs without — the compiler again deleted the
   1-element copy when the result was discarded. Only the tiny-copy path gets
   elided; the 10-element paths show the same allocs with and without sink.

2. **The trie-vs-prefix verdict FLIPPED compared to the `-cpu=2` session.**
   Within Block 1 (valid comparison — same session, both with sink):

    | prefix | trie  | prefix index | winner               |
    | ------ | ----- | ------------ | -------------------- |
    | 1char  | 164.5 | 157.0        | prefix (~tie, −7 ns) |
    | 2char  | 185.4 | 159.1        | prefix (−26 ns)      |
    | rare   | 73.2  | 52.1         | prefix (−21 ns)      |
    | exact  | 220.4 | 159.8        | prefix (−61 ns)      |

    The `-cpu=2` session said prefix loses on 1char/2char/exact. This session
    says prefix wins everywhere. Both used the sink. Something other than the
    sink changed between sessions.

3. **The change is in the prefix index, not the trie.** Its absolute numbers
   dropped ~110 ns between sessions (1char 278→157, exact 268→160) while the
   trie stayed roughly flat (146→164, 226→220). The prefix map's probe+value
   path became much cheaper — cache/thermal/clock state of the machine, or
   allocation-arena layout (GOMAXPROCS changes how the 7M-key map is
   allocated), or a mix. Not isolated yet.

4. **Cross-run drift is visible _within_ this session too.** trie 1char drops
   164.5 → 133.3 between Block 1 and Block 2. The sink can only slow the trie
   (it adds a store); it cannot speed it up. So that 31 ns is drift — proof
   that consecutive runs on this laptop aren't directly comparable.

5. **Correcting the earlier conclusion:** the section above says _"the prefix
   index loses on every realistic prefix"_ — that was a `-cpu=2` session
   verdict and is now contradicted at `-cpu=8`. The only robust findings so
   far are: (a) the sink/elision effect on tiny copies, and (b) the relative
   verdict is **unstable across sessions** on this hardware.

### Conclusion: this is the real benchmark lesson

One `go test` session cannot give a stable relative verdict between these two
structures on this laptop. The correct next step is **interleaved, automated
comparison** — e.g. `benchstat` over `-count=10` runs, or a single benchmark
that alternates `trie.Suggest` and `pi.Suggest` inside one loop so both see the
same thermal/clock window. Until then, do not quote a trie-vs-prefix ns number
anywhere as the definitive result.

The structural hypothesis (fixed giant-map probe vs per-rune walk) predicts
prefix loses on short prefixes — the `-cpu=8` session contradicts that for
1char. To resolve, run the isolated probe-only microbenchmark (map lookup vs
`Search` walk, no copy) at both `-cpu=2` and `-cpu=8`, and check whether the
probe cost itself moves with GOMAXPROCS.
