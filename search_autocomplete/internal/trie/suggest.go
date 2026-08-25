package trie

func (t *Trie) Suggest(s string) []string {
	if t.K <= 0 {
		return nil
	}
	prefixExists, _, _, node := t.Search(s)
	if !prefixExists {
		return nil
	}
	result := make([]string, 0, len(node.Top))
	for _, r := range node.Top {
		result = append(result, r.Word)
	}
	return result

}
