package main

import (
	"fmt"
	"strings"
)

// We use this to track parent-child relationships between calls
var graphBuilder strings.Builder

func fib(n int, parentID string) int {
	// Create a unique ID for this execution node
	currentID := fmt.Sprintf("node_%d_%p", n, &n)

	if parentID != "" {
		// Log the edge connecting parent call to this child call
		graphBuilder.WriteString(fmt.Sprintf("    %s -> %s;\n", parentID, currentID))
	}

	// Label the node with its variable state
	graphBuilder.WriteString(fmt.Sprintf("    %s [label=\"fib(%d)\"];\n", currentID, n))

	// Base cases
	if n <= 1 {
		return n
	}

	// Recursive calls
	return fib(n-1, currentID) + fib(n-2, currentID)
}

func Builer() {
	graphBuilder.WriteString("digraph G {\n")
	fib(4, "") // Run recursion for fib(4)
	graphBuilder.WriteString("}\n")

	// Print out the DOT language output
	fmt.Println(graphBuilder.String())
}
