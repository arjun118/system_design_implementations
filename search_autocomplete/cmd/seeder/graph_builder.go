package main

import (
	"fmt"
	"os"
	"sync"
)

type RecursionGraph struct {
	file   *os.File
	mu     sync.Mutex
	nextID int
}

func NewRecursionGraph(filename string) (*RecursionGraph, error) {
	f, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(f, "digraph Recursion {")

	// Top-to-bottom recursion tree.
	fmt.Fprintln(f, `  rankdir=TB;`)

	// Keep nodes small and compact.
	fmt.Fprintln(f, `  node [fontname="Helvetica", fontsize=12];`)

	// Compact edges.
	fmt.Fprintln(f, `  edge [arrowsize=0.6];`)

	return &RecursionGraph{
		file: f,
	}, nil
}

func (g *RecursionGraph) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	fmt.Fprintln(g.file, "}")

	return g.file.Close()
}

func (g *RecursionGraph) NewNode(
	label string,
	shape string,
	tooltip string,
) int {
	g.mu.Lock()
	defer g.mu.Unlock()

	id := g.nextID
	g.nextID++

	fmt.Fprintf(
		g.file,
		"  n%d [label=%q, shape=%s, tooltip=%q];\n",
		id,
		label,
		shape,
		tooltip,
	)

	return id
}

func (g *RecursionGraph) Edge(from, to int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	fmt.Fprintf(
		g.file,
		"  n%d -> n%d;\n",
		from,
		to,
	)
}

// BucketNode represents a processBucket invocation.
//
// The visible label is ONLY the prefix.
// Detailed information is kept in the SVG tooltip.
func (g *RecursionGraph) BucketNode(
	prefix string,
	start int64,
	end int64,
) int {
	tooltip := fmt.Sprintf(
		"processBucket(%s) [%d, %d]",
		prefix,
		start,
		end,
	)

	return g.NewNode(
		prefix,
		diamondShape(),
		tooltip,
	)
}

// BuildNode represents a bucket that fits in memory.
//
// Visible label is ONLY the prefix.
func (g *RecursionGraph) BuildNode(
	parent int,
	prefix string,
	prefixes int64,
	estimated float64,
) {
	tooltip := fmt.Sprintf(
		"BUILD %s | prefixes=%d | estimated=%.2f MB",
		prefix,
		prefixes,
		estimated/(1024*1024),
	)

	id := g.NewNode(
		prefix,
		"circle",
		tooltip,
	)

	g.Edge(parent, id)
}

// SplitNode represents a child that does not fit in memory
// and therefore becomes a recursive processBucket call.
//
// Visible label is ONLY the prefix.
func (g *RecursionGraph) SplitNode(
	parent int,
	prefix string,
	start int64,
	end int64,
	prefixes int64,
	estimated float64,
) int {
	tooltip := fmt.Sprintf(
		"SPLIT %s | range=[%d, %d] | prefixes=%d | estimated=%.2f MB",
		prefix,
		start,
		end,
		prefixes,
		estimated/(1024*1024),
	)

	id := g.NewNode(
		prefix,
		"diamond",
		tooltip,
	)

	g.Edge(parent, id)

	return id
}

// Just keeps the shape choice in one place.
// This makes it easy to change the visual later.
func diamondShape() string {
	return "diamond"
}
