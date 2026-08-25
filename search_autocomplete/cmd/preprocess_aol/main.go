// Command preprocess_aol converts AOL user-query logs into a single
// "query TAB freq" file for the autocomplete trie.
//
// Input format (verified against user-ct-test-collection-01..10):
//
//	AnonID TAB Query TAB QueryTime TAB ItemRank TAB ClickURL
//
// with a header row on line 1 and empty ItemRank/ClickURL on query-only rows.
// The Query field is already lowercase. The files are sorted by AnonID.
//
// Counting: by default one vote per distinct (user, query) pair. Raw line
// counts over-count queries with multiple clicks ("one query, two lines per
// click") and "next page" repeats (identical query, later timestamp) — the
// per-user dedupe avoids both. Since files are sorted by AnonID, we dedupe the
// current user's queries in a small local set and flush it on user change.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type item struct {
	q    string
	freq int64
}

func main() {
	in := flag.String("in", "AOL-user-ct-collection/user-ct-test-collection-*.txt", "glob of AOL log files")
	out := flag.String("out", "aol_queries.txt", "output file (query TAB freq)")
	minFreq := flag.Int64("min-freq", 3, "drop queries seen fewer than this many times")
	topN := flag.Int("top-n", 1_000_000, "keep the top-N queries by freq (0 = keep all)")
	mode := flag.String("mode", "user", "counting mode: 'user' (distinct user+query pairs) or 'raw' (every line)")
	flag.Parse()

	files, err := filepath.Glob(*in)
	if err != nil {
		log.Fatalf("bad -in glob %q: %v", *in, err)
	}
	if len(files) == 0 {
		log.Fatalf("no files match %q", *in)
	}
	sort.Strings(files)

	global := make(map[string]int64)
	start := time.Now()
	var totalLines int64
	for _, f := range files {
		n, err := processFile(f, global, *mode)
		if err != nil {
			log.Fatalf("%s: %v", f, err)
		}
		totalLines += n
		log.Printf("done %s: %d lines, %d distinct queries so far (%s elapsed)",
			f, n, len(global), time.Since(start).Round(time.Second))
	}

	items := make([]item, 0, len(global))
	for q, freq := range global {
		if freq >= *minFreq {
			items = append(items, item{q: q, freq: freq})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].freq != items[j].freq {
			return items[i].freq > items[j].freq
		}
		return items[i].q < items[j].q
	})
	if *topN > 0 && len(items) > *topN {
		items = items[:*topN]
	}

	if err := writeOut(*out, items); err != nil {
		log.Fatalf("write %s: %v", *out, err)
	}
	log.Printf("done: %d lines read, %d queries -> %s (%s)",
		totalLines, len(items), *out, time.Since(start).Round(time.Second))
}

func processFile(path string, global map[string]int64, mode string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)

	var (
		lines    int64
		curUser  string
		userSeen map[string]struct{} // distinct queries for the current user
	)

	flushUser := func() {
		for q := range userSeen {
			global[q]++
		}
		userSeen = nil
	}

	for sc.Scan() {
		lines++
		fields := strings.Split(sc.Text(), "\t")
		if len(fields) < 3 {
			continue
		}
		// skip the per-file header row
		if fields[1] == "Query" {
			continue
		}
		q, ok := clean(fields[1])
		if !ok {
			continue
		}

		if mode == "raw" {
			global[q]++
			continue
		}

		// user mode: one vote per distinct (user, query) pair
		if fields[0] != curUser {
			flushUser()
			curUser = fields[0]
			userSeen = make(map[string]struct{})
		}
		if _, seen := userSeen[q]; !seen {
			userSeen[q] = struct{}{}
		}
	}
	flushUser()

	if err := sc.Err(); err != nil {
		return lines, err
	}
	return lines, nil
}

// clean normalizes a query and rejects junk rows: empty, whitespace-only,
// punctuation/digit-only fragments, URLs and URL-ish concatenations, and
// single-char noise.
func clean(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), " ") // collapse internal whitespace
	if s == "" || utf8.RuneCountInString(s) < 2 {
		return "", false
	}
	if strings.Contains(s, "http") || strings.HasPrefix(s, "www.") {
		return "", false
	}
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return "", false
	}
	return s, true
}

func writeOut(path string, items []item) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriterSize(f, 1<<20)
	for _, it := range items {
		if _, err := fmt.Fprintf(w, "%s\t%d\n", it.q, it.freq); err != nil {
			return err
		}
	}
	return w.Flush()
}
