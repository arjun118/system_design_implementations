package trie

type PrefixIndex map[string][]Reco

func NewPrefixIndex() PrefixIndex {
	return PrefixIndex{}
}

func BuildPrefixIndex(t *Trie) PrefixIndex {
	pi := NewPrefixIndex()
	t.walkForIndex(t.Root, "", pi)
	return pi
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
