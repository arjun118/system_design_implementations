package trie

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// var sink []string

const TestFileName string = "/home/cicada/system_design_implementations/search_autocomplete/test.txt"
const OrgFileName string = "/home/cicada/system_design_implementations/search_autocomplete/aol_queries.txt"

func TestParseLine(t *testing.T) {
	lines := [][]string{
		{"bluehost.com\t3", "bluehost.com", "3"},
		{"razor bumps\t12", "razor bumps", "12"},
		{"ing retirement\t11", "ing retirement", "11"},
		{"how to get rid of bees\t5", "how to get rid of bees", "5"},
		{"sgs\t4", "sgs", "4"},
		{"dell preferred account\t16", "dell preferred account", "16"},
		{"anita bryant\t10", "anita bryant", "10"},
		{"putas.com\t8", "putas.com", "8"},
		{"fix a toilet\t4", "fix a toilet", "4"},
		{"purina.com\t10", "purina.com", "10"},
	}
	for _, line := range lines {
		phrase, freq, _ := ParseLine(line[0])
		require.Equal(t, line[1], phrase)
		require.Equal(t, line[2], fmt.Sprintf("%d", freq))
	}
}
func TestWordSearchAndCount(t *testing.T) {
	trie := BuildTrie(TestFileName)
	file, err := os.Open(TestFileName)
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
	trie := BuildTrie(TestFileName)
	file, err := os.Open(TestFileName)
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

// only build - no cache
func BenchmarkBuild(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		_ = BuildTrie(OrgFileName)
		// trie.BuildCache()
	}
}

// k scaling bench
func BenchmarkBuildCacheK(b *testing.B) {
	trie := BuildTrie(OrgFileName)
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

func BenchmarkTrieSuggest(b *testing.B) {
	trie := BuildTrie(OrgFileName)
	trie.K = 10
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
				_ = trie.Suggest(tc.prefix)
			}
			// _ = sink
		})
	}
}

func BenchmarkPrefixSuggest(b *testing.B) {
	trie := BuildTrie(OrgFileName)
	trie.K = 10
	trie.BuildCache()
	pi := BuildPrefixIndex(trie)
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
				_ = pi.Suggest(tc.prefix)
			}
			// _ = sink
		})
	}
}
