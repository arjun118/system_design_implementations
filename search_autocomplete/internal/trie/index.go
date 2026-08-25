package trie

import (
	"fmt"
	"slices"
)

type Suggester interface {
	Suggest(prefix string) []string
}

func Load(path string, impl string, k int) (Suggester, error) {
	if !slices.Contains([]string{"trie", "prefix-hash"}, impl) {
		return nil, fmt.Errorf("invalid implementation")

	}
	trie := BuildTrie(path)
	trie.K = k
	trie.BuildCache()
	if impl == "trie" {
		return trie, nil
	} else {
		pi := BuildPrefixIndex(trie)
		return pi, nil
	}
}
