package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func main() {
	// 1. Setup Flags
	query := flag.String("query", "", "The filename substring to search for")
	root := flag.String("path", ".", "The root directory to start searching")
	flag.Parse()

	if *query == "" {
		fmt.Println("Error: -query is required")
		return
	}

	results := make(chan string)
	var wg sync.WaitGroup

	// 2. Throttling (The "Pro" Challenge)
	// This channel acts as a semaphore to limit concurrent goroutines to 100.
	// This prevents "too many open files" errors on large systems.
	semaphore := make(chan struct{}, 100)

	// 3. Kick off initial search
	wg.Add(1)
	go searchDir(*root, *query, results, &wg, semaphore)

	// 4. The "Done" Signal (The Closer)
	// We must wait and close in a separate goroutine so the main loop below
	// can start printing results immediately without blocking.
	go func() {
		wg.Wait()
		close(results)
	}()

	// 5. Main Goroutine: Listen and Print
	for path := range results {
		fmt.Println(path)
	}
}

func searchDir(path string, query string, results chan<- string, wg *sync.WaitGroup, sem chan struct{}) {
	defer wg.Done()

	// Acquire semaphore slot (blocks if 100 are already running)
	sem <- struct{}{}
	defer func() { <-sem }() // Release slot when function finishes

	entries, err := os.ReadDir(path)
	if err != nil {
		// Silently ignore permission errors or restricted system folders
		return
	}

	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())

		if entry.IsDir() {
			// Found a sub-folder: Spin up a new worker
			wg.Add(1)
			go searchDir(fullPath, query, results, wg, sem)
		} else {
			// Found a file: Check name against query
			if strings.Contains(entry.Name(), query) {
				results <- fullPath
			}
		}
	}
}
