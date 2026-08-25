package trie

import (
	"bufio"

	"fmt"
	"os"
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
	}
}

func (t *Trie) Insert(s string, freq int64) {
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
	node.Freq = freq
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
		phrase, freq, ok := ParseLine(line)
		if ok {
			trie.Insert(phrase, freq)
		}

	}
	if err := scanner.Err(); err != nil {
		panic(fmt.Sprintf("failed build trie..%s", err.Error()))
	}
	return trie
}
