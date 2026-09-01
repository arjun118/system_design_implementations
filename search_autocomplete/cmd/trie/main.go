package main

import (
	"flag"
	"fmt"

	"github.com/arjun118/autocomplete/internal/trie"
)

// cli
func main() {
	var (
		file = flag.String("file", "/home/cicada/system_design_implementations/search_autocomplete/aol_queries.txt",
			"data file (phrase TAB freq)")
		impl       = flag.String("impl", "trie", "implementation: trie | prefix-hash")
		k          = flag.Int("k", 5, "cache size (top-k suggestions)")
		cacheDepth = flag.Int("cd", 6, "cache depth (cache only shallow nodes - not deep nodes")
		sq         = flag.String("sq", "", "search query to execute (e.g., -sq go)")
	)
	flag.Parse()

	if *sq == "" {
		fmt.Println("Error: -sq flag is required")
		flag.Usage()
		return
	}
	suggestor, err := trie.Load(*file, *impl, *k, *cacheDepth)
	if err != nil {
		panic(err)
	}
	res := suggestor.Suggest(*sq)
	fmt.Println(res)
	// fmt.Println(trie.K)

}
