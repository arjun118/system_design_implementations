package trie

import "slices"

type PrefixIndex map[string][]Reco

func NewPrefixIndex() PrefixIndex {
	return PrefixIndex{}
}

func BuildPrefixIndex(t *Trie) PrefixIndex {
	pi := NewPrefixIndex()
	// traverse trie and populate the prefix index and return
	IterateTrie(t.Root, "", pi)
	return pi
}

func IterateTrie(node *Node, prefix string, pi PrefixIndex) {
	if len(node.Children) == 0 {
		return
	}
	for key, child := range node.Children {
		IterateTrie(child, prefix+string(key), pi)
		pi[prefix+string(key)] = slices.Clone(child.Top)
	}
}

func (pi PrefixIndex) Suggest(prefix string) []string {
	recos, ok := pi[prefix]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(recos))
	for _, r := range recos {
		result = append(result, r.Word)
	}
	return result
}
