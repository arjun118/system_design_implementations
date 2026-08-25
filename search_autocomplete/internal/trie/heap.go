package trie

type Reco struct {
	Word string
	Freq int64
}

type RecHeap []Reco

func (h RecHeap) Len() int           { return len(h) }
func (h RecHeap) Less(i, j int) bool { return h[i].Freq < h[j].Freq } // Minimal value stays at root
func (h RecHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *RecHeap) Push(x any) {
	*h = append(*h, x.(Reco))
}

func (h RecHeap) Top() Reco {
	return h[0]
}

func (h *RecHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type head struct {
	list []Reco
	idx  int
}

type headHeap []head

func (h headHeap) Len() int           { return len(h) }
func (h headHeap) Less(i, j int) bool { return h[i].list[h[i].idx].Freq > h[j].list[h[j].idx].Freq } //max heap
func (h headHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *headHeap) Push(x any) {
	*h = append(*h, x.(head))
}

func (h headHeap) Top() head {
	return h[0]
}

func (h *headHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
