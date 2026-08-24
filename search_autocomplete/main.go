package main

import (
	"bufio"
	"container/heap"
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

type Node struct {
	Val      rune
	Children map[rune]*Node
	IsWord   bool
	Freq     int64
	Top      []Reco
}

func NewNode() *Node {
	return &Node{
		Children: make(map[rune]*Node),
	}
}

type Trie struct {
	Root *Node
	K    int
}

func NewTrie() *Trie {
	return &Trie{
		Root: NewNode(),
		K:    10,
	}
}

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

func (t *Trie) Insert(s string, freq int) {
	// insert and return the freq of the word
	node := t.Root
	for _, x := range s {
		nextnode, ok := node.Children[x]
		if !ok {
			// add new
			nextnode = NewNode()
			nextnode.Val = x
			node.Children[x] = nextnode
		}
		node = nextnode
	}
	node.IsWord = true
	node.Freq = int64(freq)
}

func (t *Trie) Search(s string) (bool, bool, int64, *Node) {
	// search for a string in the trie
	// first bool -> if the string exists
	// second bool -> if the string is a word or not
	node := t.Root
	for _, x := range s {
		nextnode, ok := node.Children[x]
		if !ok {
			return false, false, 0, nil
		}
		node = nextnode
	}
	return true, node.IsWord, node.Freq, node
}

func BuildTrie(ipfile string) *Trie {
	trie := NewTrie()
	file, err := os.Open(ipfile)
	if err != nil {
		panic("error opening file,cannot build trie")
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		word := fields[0]
		freqInt, err := strconv.Atoi(fields[1])
		if err != nil {
			fmt.Printf("failed: word: %s freq: %s", word, fields[1])
			continue
		}
		trie.Insert(word, freqInt)
	}
	if err := scanner.Err(); err != nil {
		panic(fmt.Sprintf("failed build trie..%s", err.Error()))
	}
	return trie
}

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
	// populate the slice of strings - Word at every node
	// this will be naive first - we call suggst for every node and then get the recommendations
	node := t.Root
	t.TraverseAndBuild(node, "")
	return nil
}

// dfs from a particular node to find the top-k words
func DFS(node *Node, prefix string, hp *RecHeap, k int) {
	for key, child := range node.Children {
		newPrefix := prefix + string(key)
		if child.IsWord {
			if hp.Len() < k {
				heap.Push(hp, Reco{Word: newPrefix, Freq: child.Freq})
			} else if hp.Top().Freq < child.Freq {
				heap.Pop(hp)
				heap.Push(hp, Reco{Word: newPrefix, Freq: child.Freq})
			}
		}
		DFS(child, newPrefix, hp, k)
	}
}

func (t *Trie) Suggest(s string, cached bool) []string {
	if t.K <= 0 {
		return nil
	}
	prefixExists, _, _, node := t.Search(s)
	if !prefixExists {
		return nil
	}
	if cached {
		result := make([]string, 0, len(node.Top))
		for _, r := range node.Top {
			result = append(result, r.Word)
		}
		return result
	}
	minHeap := &RecHeap{}
	heap.Init(minHeap)
	DFS(node, s, minHeap, t.K)
	result := make([]string, 0, minHeap.Len())
	for minHeap.Len() > 0 {
		word := heap.Pop(minHeap).(Reco).Word
		result = append(result, word)
	}
	slices.Reverse(result)
	// return node.Words
	return result
}

func main() {
	// building the trie
	var sq string
	flag.StringVar(&sq, "sq", "", "search query to execute (e.g., -sq go)")
	flag.Parse()

	if sq == "" {
		fmt.Println("Error: -sq flag is required")
		flag.Usage()
		return
	}
	trie := BuildTrie("./out.txt")
	trie.BuildCache()
	res := trie.Suggest(sq, true)
	fmt.Println(res)
	// fmt.Println(trie.K)
}
