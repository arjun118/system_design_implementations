package main

import (
	"bufio"
	"container/heap"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// estimate before start runtime metrics

// read the file line by line
// for each line
//
//	for prefix by phrase
//		mp[prefix] is a min heap and we will append to that the logic - thats fine
//		mp[prefix].append({phrase, freq})
//
// estimate after start runtime metrics
func lcp(a, b string) int {
	ar := []rune(a)
	br := []rune(b)

	n := min(len(ar), len(br))

	i := 0
	for i < n && ar[i] == br[i] {
		i++
	}

	return i
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

type PI map[string]*RecHeap

func ParseLine(line string) (string, int64, bool) {
	lastTab := strings.LastIndexByte(line, '\t')
	if lastTab < 0 {
		return "", 0, false // no separator → not a data line
	}
	phrase := strings.Join(strings.Fields(line[:lastTab]), " ")
	if phrase == "" {
		return "", 0, false
	}
	freq, err := strconv.Atoi(strings.TrimSpace(line[lastTab+1:]))
	if err != nil {
		return "", 0, false
	}
	return phrase, int64(freq), true
}

func main() {
	var (
		k    = flag.Int("k", 10, "-k 10")
		file = flag.String("file", "./sample.txt", "-file filename")
	)
	flag.Parse()
	fmt.Printf("File name: %s, K %d\n", *file, *k)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	reader, err := os.Open(*file)
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(reader)
	prev := ""
	distinctPrefixes := 0
	pi := make(PI)
	for scanner.Scan() {
		phrase, freq, ok := ParseLine(scanner.Text())
		runeList := []rune(phrase)
		if !ok {
			continue
		}
		distinctPrefixes += len(runeList) - lcp(phrase, prev)
		prev = phrase
		for i := 1; i <= len(runeList); i++ {
			pref := string(runeList[:i])
			// pi[string(runeList[:i])] = &Reco{Word: phrase, Freq: freq}
			if pi[pref] == nil {
				hp := &RecHeap{}
				heap.Init(hp)
				pi[pref] = hp
			}
			if pi[pref].Len() < *k {
				heap.Push(pi[pref], Reco{Word: phrase, Freq: freq})
			} else {
				if pi[pref].Top().Freq < freq {
					heap.Pop(pi[pref])
					heap.Push(pi[pref], Reco{Word: phrase, Freq: freq})
				}
			}
		}
	}
	if scanner.Err() != nil {
		panic(scanner.Err())
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	heapUsed := after.HeapAlloc - before.HeapAlloc
	heapInuse := after.HeapInuse - before.HeapInuse
	sysUsed := after.Sys - before.Sys

	fmt.Printf("Calculated distinct prefixes: %d\n", distinctPrefixes)
	fmt.Printf("Actual map entries: %d\n", len(pi))

	fmt.Printf("HeapAlloc increase: %.2f MB\n",
		float64(heapUsed)/(1024*1024))

	fmt.Printf("HeapInuse increase: %.2f MB\n",
		float64(heapInuse)/(1024*1024))

	fmt.Printf("Sys increase: %.2f MB\n",
		float64(sysUsed)/(1024*1024))

	fmt.Printf("Bytes / map entry: %.2f\n",
		float64(heapUsed)/float64(len(pi)))
	totalRecos := 0

	for _, h := range pi {
		totalRecos += h.Len()
	}

	fmt.Printf("Total recommendations stored: %d\n", totalRecos)
	fmt.Printf("Average recommendations/prefix: %.2f\n",
		float64(totalRecos)/float64(len(pi)))
}
