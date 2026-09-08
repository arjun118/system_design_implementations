package store

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

var benchmarkResult any

// Heavy, real-infra setup (Mongo connect + Redis warm + prefix list) is done a
// single time per process via sync.Once. go test -bench invokes each benchmark
// function repeatedly to calibrate b.N; without this, NewSuggester + Init would
// run on every pass, reconnecting to Mongo and re-seeding Redis each time.
// Both benchmarks below share this one-time setup.
var (
	setupOnce     sync.Once
	setupStore    *Suggester
	setupPrefixes []string
	setupErr      error
)

func setup() (*Suggester, []string) {
	setupOnce.Do(func() {
		var err error

		f, err := os.Open("../../google_books_ds/bench_prefix_ds.txt")
		if err != nil {
			setupErr = fmt.Errorf("open prefix file: %w", err)
			return
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			setupPrefixes = append(setupPrefixes, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			setupErr = fmt.Errorf("scan prefix file: %w", err)
			return
		}
		if len(setupPrefixes) == 0 {
			setupErr = fmt.Errorf("prefix file is empty")
			return
		}

		setupStore, err = NewSuggester()
		if err != nil {
			setupErr = fmt.Errorf("NewSuggester: %w", err)
			return
		}
		if err := setupStore.Init(context.Background(), 10000, 100000); err != nil {
			setupErr = fmt.Errorf("store.Init: %w", err)
			return
		}
	})
	return setupStore, setupPrefixes
}

func getStore(tb testing.TB) (*Suggester, []string) {
	st, prefixes := setup()
	require.NoError(tb, setupErr)
	require.NotNil(tb, st)
	require.NotEmpty(tb, prefixes)
	return st, prefixes
}

// Single client, issuing suggests one at a time.
func BenchmarkStore(b *testing.B) {
	st, prefixes := getStore(b)
	numPrefixes := len(prefixes)

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		prefix := prefixes[i%numPrefixes]
		res, _ := st.Suggest(prefix)
		benchmarkResult = res // Ensure the compiler doesn't eliminate the call
	}
}

// concurrentUsers simulates that many simultaneous requesters. The total number
// of timed operations is still b.N: each worker issues its share, and any
// remainder runs on this goroutine. Run with -race to also check the concurrent
// read path (Suggest + its async Redis write-back) for data races.
const concurrentUsers = 1000

func BenchmarkStoreConcurrent(b *testing.B) {
	st, prefixes := getStore(b)
	numPrefixes := len(prefixes)

	workers := concurrentUsers
	perWorker := b.N / workers
	rem := b.N % workers

	// Per-worker sink so concurrent writes never race on a shared variable.
	results := make([]any, workers)

	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(seed, n int, sink *any) {
			defer wg.Done()
			for j := 0; j < n; j++ {
				res, _ := st.Suggest(prefixes[(seed+j)%numPrefixes])
				*sink = res // own slot: no data race
			}
		}(w, perWorker, &results[w])
	}
	// Remainder (< concurrentUsers ops) on the benchmark goroutine so total == b.N.
	for j := 0; j < rem; j++ {
		res, _ := st.Suggest(prefixes[j%numPrefixes])
		benchmarkResult = res
	}
	wg.Wait()
	benchmarkResult = results[0]
}
