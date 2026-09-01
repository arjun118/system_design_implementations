// Command preprocess_ngrams converts raw Google Books ngram files into a
// single "phrase TAB freq" file for the autocomplete trie.
//
// Input format (20090715 CSV release, tab-separated despite the .csv suffix):
//
//	ngram TAB year TAB match_count TAB page_count TAB volume_count
//
// The 20120701/20200217 releases drop or reorder page_count; both variants
// are accepted. The ngram field contains spaces, so lines must be split on
// tabs, never on whitespace. Files may be plain or gzip-compressed (.gz).
//
// The files are sorted by ngram and sharded by first token, so each phrase's
// rows are contiguous within a file — we aggregate with a streaming group-by
// instead of loading everything into memory.
//
// Scoring: freq = volume_count * log2(1 + match_count/volume_count), scaled to
// int64. Volume is the breadth signal (resists one-book spikes and corpus-size
// drift across years); the log-compressed per-volume intensity keeps a single
// book from dominating the rank.
package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type agg struct {
	match int64
	vol   int64
}

type item struct {
	phrase string
	freq   int64
}

func main() {
	dir := flag.String("dir", "", "folder containing the raw ngram files")
	pattern := flag.String("pattern", "*", "glob for files inside -dir (e.g. \"*.csv\", \"*.gz\")")
	out := flag.String("out", "out_phrases.txt", "output file (phrase TAB freq)")
	minVol := flag.Int64("min-vol", 5, "drop phrases appearing in fewer than this many volumes")
	minMatch := flag.Int64("min-match", 20, "drop phrases with fewer than this many total matches")
	topN := flag.Int("top-n", 0, "keep the top-N phrases by score (0 = keep all)")
	maxWords := flag.Int("max-words", 0, "drop phrases with more than this many words (0 = no limit)")
	flag.Parse()

	if *dir == "" {
		log.Fatal("-dir is required: folder containing the raw ngram files")
	}

	files, err := filepath.Glob(filepath.Join(*dir, *pattern))
	if err != nil {
		log.Fatalf("bad -pattern %q: %v", *pattern, err)
	}
	if len(files) == 0 {
		log.Fatalf("no files match %q in %q", *pattern, *dir)
	}
	sort.Strings(files)

	acc := make(map[string]*agg)
	start := time.Now()
	var totalLines int64
	for _, f := range files {
		n, err := processFile(f, acc, *minVol, *minMatch, *maxWords)
		if err != nil {
			log.Fatalf("%s: %v", f, err)
		}
		totalLines += n
		log.Printf("done %s: %d lines (%d phrases kept so far, %s elapsed)",
			f, n, len(acc), time.Since(start).Round(time.Second))
	}

	items := scoreAndFilter(acc, *minVol, *minMatch)
	sort.Slice(items, func(i, j int) bool {
		if items[i].freq != items[j].freq {
			return items[i].freq > items[j].freq
		}
		return items[i].phrase < items[j].phrase
	})
	if *topN > 0 && len(items) > *topN {
		items = items[:*topN]
	}

	if err := writeOut(*out, items); err != nil {
		log.Fatalf("write %s: %v", *out, err)
	}
	log.Printf("done: %d lines read, %d phrases -> %s (%s)",
		totalLines, len(items), *out, time.Since(start).Round(time.Second))
}

// processFile streams one shard, grouping consecutive rows of the same phrase
// and accumulating match/volume totals into acc once the phrase completes.
func processFile(path string, acc map[string]*agg, minVol, minMatch int64, maxWords int) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return 0, fmt.Errorf("gzip: %w", err)
		}
		defer gz.Close()
		r = gz
	}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)

	var (
		lines       int64
		curPhrase   string
		curMatch    int64
		curVol      int64
		curAccepted bool // current phrase passed the token/word-count filters
	)

	flush := func() {
		if curAccepted && curVol >= minVol && curMatch >= minMatch {
			if a, ok := acc[curPhrase]; ok {
				a.match += curMatch
				a.vol += curVol
			} else {
				acc[curPhrase] = &agg{match: curMatch, vol: curVol}
			}
		}
		curPhrase, curMatch, curVol, curAccepted = "", 0, 0, false
	}

	for sc.Scan() {
		lines++
		match, vol, ok := parseCounts(sc.Text())
		if !ok {
			continue // malformed row; phrase grouping below still tracks changes
		}
		fields := strings.Split(sc.Text(), "\t")
		phrase := fields[0]
		if phrase != curPhrase {
			flush()
			curPhrase = phrase
			curAccepted = keepPhrase(phrase, maxWords)
		}
		if curAccepted {
			curMatch += match
			curVol += vol
		}
	}
	flush()

	if err := sc.Err(); err != nil {
		return lines, err
	}
	return lines, nil
}

// parseCounts extracts (match_count, volume_count) from a 4- or 5-column row.
//
//	4 columns: ngram year match volume          (20120701 release)
//	5 columns: ngram year match page volume     (20090715, 20200217 releases)
func parseCounts(line string) (match, vol int64, ok bool) {
	fields := strings.Split(line, "\t")
	switch len(fields) {
	case 4: // ngram year match volume
		m, e1 := strconv.ParseInt(fields[2], 10, 64)
		v, e2 := strconv.ParseInt(fields[3], 10, 64)
		return m, v, e1 == nil && e2 == nil
	case 5: // ngram year match page volume
		m, e1 := strconv.ParseInt(fields[2], 10, 64)
		v, e2 := strconv.ParseInt(fields[4], 10, 64)
		return m, v, e1 == nil && e2 == nil
	}
	return 0, 0, false
}

// keepPhrase keeps only genuine English phrases: no meta tokens (_START_,
// _END_, POS tags, underscores), no non-Latin scripts (Cyrillic/CJK/Arabic),
// and enough real words that the phrase isn't dominated by OCR punctuation or
// digit noise. URL-ish tokens are tolerated because they contain letters.
func keepPhrase(phrase string, maxWords int) bool {
	if phrase == "" {
		return false
	}
	tokens := strings.Fields(phrase)
	if len(tokens) == 0 || (maxWords > 0 && len(tokens) > maxWords) {
		return false
	}

	words := 0
	for _, tok := range tokens {
		hasLetter, hasUnderscore, hasForeign := false, false, false
		for _, r := range tok {
			switch {
			case r == '_':
				hasUnderscore = true
			case unicode.IsLetter(r):
				hasLetter = true
				if !unicode.Is(unicode.Latin, r) {
					hasForeign = true
				}
			}
		}
		if hasUnderscore {
			return false // _START_/_END_/POS tags/meta — noise
		}
		if hasForeign {
			return false // not English (Cyrillic, CJK, Arabic, ...)
		}
		if hasLetter {
			words++
		}
	}
	// at least one real word, and words must not be a minority of the tokens
	return words >= 1 && words*2 >= len(tokens)
}

func scoreAndFilter(acc map[string]*agg, minVol, minMatch int64) []item {
	items := make([]item, 0, len(acc))
	for phrase, a := range acc {
		if a.vol < minVol || a.match < minMatch || a.vol <= 0 {
			continue
		}
		score := float64(a.vol) * math.Log2(1+float64(a.match)/float64(a.vol))
		items = append(items, item{phrase: phrase, freq: int64(math.Round(score * 1000))})
	}
	return items
}

func writeOut(path string, items []item) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriterSize(f, 1<<20)
	for _, it := range items {
		if _, err := fmt.Fprintf(w, "%s\t%d\n", it.phrase, it.freq); err != nil {
			return err
		}
	}
	return w.Flush()
}
