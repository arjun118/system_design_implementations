package trie

import (
	"container/heap"
	"slices"
)

func (t *Trie) Suggest(s string) []string {
	if t.K <= 0 {
		return nil
	}
	prefixExists, _, _, node := t.Search(s)
	if !prefixExists {
		return nil
	}
	var recos []Reco
	// if node.Top is nil - fall back to dfs
	if node.Top == nil {
		DFS(s, node, &recos)
		recos = topK(recos, t.K)
	} else {
		recos = node.Top
	}
	result := make([]string, 0, len(recos))
	for _, r := range recos {
		result = append(result, r.Word)
	}
	return result

}

func topK(cands []Reco, k int) []Reco {
	h := &RecHeap{}
	heap.Init(h)
	for _, r := range cands {
		if h.Len() < k {
			heap.Push(h, r)
		} else if h.Top().Freq < r.Freq {
			heap.Pop(h)
			heap.Push(h, r)
		}
	}
	out := make([]Reco, 0, h.Len())
	for h.Len() > 0 {
		out = append(out, heap.Pop(h).(Reco))
	}
	slices.Reverse(out)
	return out
}

func DFS(prefix string, node *Node, out *[]Reco) {
	if node.IsWord {
		*out = append(*out, Reco{Word: prefix, Freq: node.Freq})
	}
	for key, value := range node.Children {
		DFS(prefix+string(key), value, out)
	}
}
