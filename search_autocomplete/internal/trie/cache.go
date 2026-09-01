package trie

import (
	"container/heap"
	"unicode/utf8"
)

func (t *Trie) TraverseAndBuild(node *Node, path []byte) []Reco {
	cands := make([][]Reco, 0, (len(node.Children) + 1))
	// includes exact word suggestions
	if node.IsWord {
		cands = append(cands, []Reco{{Word: string(path), Freq: node.Freq}})
	}
	for key, child := range node.Children {
		n := len(path)
		path = utf8.AppendRune(path, key) // no alloc if capacity allows
		childTop := t.TraverseAndBuild(child, path)
		path = path[:n] // truncate back for siblings
		if len(childTop) > 0 {
			cands = append(cands, childTop)
		}
	}
	var top []Reco
	switch len(cands) {
	case 0:
		// leaf, no own word
	case 1:
		top = cands[0] // already ≤ K, sorted — no merge
	default:
		top = mergeTopK(cands, t.K)
	}
	if utf8.RuneCount(path) <= t.CacheDepth {
		node.Top = top
	}
	return top
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

func (t *Trie) BuildCache() error {
	// default - safety
	if t.K == 0 {
		t.K = 10
	}
	node := t.Root
	// passing depth
	t.TraverseAndBuild(node, make([]byte, 0, 32))
	return nil
}

func (t *Trie) BuildCacheParallel() error {
	type res struct {
		top []Reco
	}
	ch := make(chan res, len(t.Root.Children))
	for key, child := range t.Root.Children {
		go func(c *Node, key rune) {
			ch <- res{top: t.TraverseAndBuild(c, []byte{byte(key)})}
		}(child, key)
	}
	cands := make([][]Reco, 0, len(t.Root.Children))
	for range t.Root.Children {
		r := <-ch
		cands = append(cands, r.top)
	}
	t.Root.Top = mergeTopK(cands, t.K)
	return nil
}
