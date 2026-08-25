package main

import (
	"flag"
	"fmt"

	"github.com/arjun118/autocomplete/internal/trie"
)

// cli
func main() {
	var sq string
	var k int
	var impl string
	flag.StringVar(&sq, "sq", "", "search query to execute (e.g., -sq go)")
	flag.IntVar(&k, "k", 10, "top-k results (e,g: -k 10)")
	flag.StringVar(&impl, "impl", "trie", "implementation : one of trie , prefix-hash,, eg: -impl prefix-hash ")
	flag.Parse()

	if sq == "" {
		fmt.Println("Error: -sq flag is required")
		flag.Usage()
		return
	}
	suggestor, err := trie.Load("/home/cicada/system_design_implementations/search_autocomplete/aol_queries.txt", impl, k)
	if err != nil {
		panic(err)
	}
	res := suggestor.Suggest(sq)
	fmt.Println(res)
	// fmt.Println(trie.K)

}
