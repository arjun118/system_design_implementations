package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// func TestScanLogic(t *testing.T) {
// 	file, err := os.Open("./test.txt")
// 	require.NoError(t, err)
// 	defer file.Close()
// 	scanner := bufio.NewScanner(file)
// 	for scanner.Scan() {
// 		line := strings.TrimSpace(scanner.Text())
// 		if line == "" {
// 			continue
// 		}
// 		fields := strings.Fields(line)
// 		require.GreaterOrEqual(t, len(fields), 2, "line must contain at least a key and a frequency")
// 		word := fields[0]
// 		freqInt, err := strconv.Atoi(fields[1])
// 		require.NoError(t, err)
// 		t.Logf("word: %s, freq: %d", word, freqInt)
// 	}
// 	require.NoError(t, scanner.Err())
// 	if err := scanner.Err(); err != nil {
// 		// panic(fmt.Sprintf("failed build trie..%s", err.Error()))
// 		require.NoError(t, err)
// 	}
// }

func TestWordSearchAndCount(t *testing.T) {
	trie := BuildTrie("./test.txt")
	file, err := os.Open("./test.txt")
	require.NoError(t, err)
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		require.GreaterOrEqual(t, len(fields), 2, "line must contain at least a key and a frequency")
		word := fields[0]
		actual, _ := strconv.Atoi(fields[1])
		// search in trie
		intrie, isword, got, _ := trie.Search(word)
		require.True(t, intrie)
		require.True(t, isword)
		require.Equal(t, int64(actual), got)
		// t.Logf("word: %s, actual freq: %d, expected freq: %d", word, got, actual)
	}
	require.NoError(t, scanner.Err())
}

func TestPrefixSearch(t *testing.T) {
	trie := BuildTrie("./test.txt")
	file, err := os.Open("./test.txt")
	require.NoError(t, err)
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		require.GreaterOrEqual(t, len(fields), 2, "line must contain at least a key and a frequency")
		word := fields[0]
		// _, _ := strconv.Atoi(fields[1])
		// search in trie
		prefix := word[:len(word)/2]
		intrie, isword, got, _ := trie.Search(prefix)
		require.True(t, intrie)
		require.False(t, isword)
		require.Equal(t, int64(0), got)
		// t.Logf("prefix: %s, actual freq: %d, expected freq: %d", prefix, got, actual)
	}
	require.NoError(t, scanner.Err())
}

func TestMergeTopK(t *testing.T) {
	lists := [][]Reco{
		{{Word: "a", Freq: 9}, {Word: "b", Freq: 5}, {Word: "c", Freq: 2}},
		{{Word: "d", Freq: 8}, {Word: "e", Freq: 7}},
		{{Word: "f", Freq: 4}},
	}
	recos := mergeTopK(lists, 6)
	got := make([]int64, 0, 6)
	for _, x := range recos {
		got = append(got, x.Freq)
	}
	want := []int64{9, 8, 7, 5, 4, 2}
	require.Equal(t, got, want)
}
func BenchmarkBuild(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		trie := BuildTrie("./out.txt")
		trie.BuildCache()
	}
}

// Cache population only — isolates the merge cost from file I/O.
// The trie is built once in setup (outside the timer); BuildCache is
// idempotent, so re-running it per iteration measures just the merge.
func BenchmarkBuildCache(b *testing.B) {
	trie := BuildTrie("./out.txt") // setup: not timed
	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		trie.BuildCache()
	}
}

// k scaling bench
func BenchmarkBuildCacheK(b *testing.B) {
	trie := BuildTrie("./out.txt")
	for _, k := range []int{10, 50, 100} {
		b.Run(fmt.Sprintf("K-%d", k), func(b *testing.B) {
			trie.K = k
			b.ResetTimer()
			for range b.N {
				trie.BuildCache()
			}
		})
	}
}

func BenchmarkSuggest(b *testing.B) {
	trie := BuildTrie("./out.txt")
	trie.BuildCache()
	table := []struct {
		name   string
		prefix string
	}{
		{"prefix_1char", "s"},
		{"prefix_2char", "ca"},
		{"prefix_rare", "zzz"},
		{"exact_word", "google"},
	}
	for _, tc := range table {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				_ = trie.Suggest(tc.prefix, true)
			}
		})
	}
}
