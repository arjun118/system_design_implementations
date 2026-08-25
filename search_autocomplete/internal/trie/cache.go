package trie

import "container/heap"

func (t *Trie) TraverseAndBuild(node *Node, prefix string) []Reco {
	cands := make([][]Reco, 0, (len(node.Children) + 1))
	// includes exact word suggestions
	if node.IsWord {
		cands = append(cands, []Reco{{Word: prefix, Freq: node.Freq}})
	}
	for key, child := range node.Children {
		cands = append(cands, t.TraverseAndBuild(child, prefix+string(key)))
	}
	node.Top = mergeTopK(cands, t.K)
	return node.Top
}

func mergeTopK(lists [][]Reco, k int) []Reco {
	h := &headHeap{}
	heap.Init(h)
	for _, l := range lists {
		if len(l) > 0 {
			heap.Push(h, head{l, 0})
		}
	}
	out := make([]Reco, 0, k)
	for h.Len() > 0 && len(out) < k {
		t := heap.Pop(h).(head) // take the largest current head
		out = append(out, t.list[t.idx])
		if t.idx+1 < len(t.list) { // that list still has elements?
			heap.Push(h, head{t.list, t.idx + 1}) // put the next one in the race
		}
	}
	return out
}

// func topK(cands []Reco, k int) []Reco {
// 	h := &RecHeap{}
// 	heap.Init(h)
// 	for _, r := range cands {
// 		if h.Len() < k {
// 			heap.Push(h, r)
// 		} else if h.Top().Freq < r.Freq {
// 			heap.Pop(h)
// 			heap.Push(h, r)
// 		}
// 	}
// 	out := make([]Reco, 0, h.Len())
// 	for h.Len() > 0 {
// 		out = append(out, heap.Pop(h).(Reco))
// 	}
// 	slices.Reverse(out)
// 	return out
// }

func (t *Trie) BuildCache() error {
	// default - safety
	if t.K == 0 {
		t.K = 10
	}
	node := t.Root
	t.TraverseAndBuild(node, "")
	return nil
}
