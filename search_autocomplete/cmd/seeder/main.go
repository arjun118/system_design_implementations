package main

import (
	"bufio"
	"container/heap"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/arjun118/autocomplete/internal/trie"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// our impl is not case sensitive
// func normalize(s string) string {
// 	runes := []rune(strings.TrimSpace(s))

// 	i := 0
// 	for i < len(runes) {
// 		r := runes[i]

// 		if unicode.IsLetter(r) || unicode.IsDigit(r) {
// 			break
// 		}

// 		i++
// 	}

// 	return strings.ToLower(string(runes[i:]))
// }

// func ParseLine(line string) (string, int64, bool) {
// 	lastTab := strings.LastIndexByte(line, '\t')
// 	if lastTab < 0 {
// 		return "", 0, false // no separator → not a data line
// 	}
// 	phrase := strings.Join(strings.Fields(line[:lastTab]), " ")
// 	if phrase == "" {
// 		return "", 0, false
// 	}
// 	freq, err := strconv.Atoi(strings.TrimSpace(line[lastTab+1:]))
// 	if err != nil {
// 		return "", 0, false
// 	}
// 	return phrase, int64(freq), true
// }

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

type BucketMetrics struct {
	Prefix           string
	Start            int64
	End              int64
	DistinctPrefixes int64
}

type Meta map[string]BucketMetrics

type PI map[string]*trie.RecHeap
type ScanResult struct {
	Children []BucketMetrics
}

const (
	B  = 1
	KB = 1024 * B
	MB = 1024 * KB
	GB = 1024 * MB
)

var k int

const bytesPerPrefix = 125

// a memory budget of 3 gb - estimated distinct prefixs * bytes per prefix to store in the index
// leaving rest of the memory for overheads
const memoryBudget = 500 * MB

var (
	indexBuildJobs = make(chan BucketMetrics, 5)
	indexResults   = make(chan PI, 5)
)

var (
	meta = make(Meta)
)

func Scan(f *os.File, prefix string, start int64, end int64) ScanResult {
	_, err := f.Seek(start, io.SeekStart)
	if err != nil {
		panic(err)
	}
	limited := io.LimitReader(f, end-start)
	scanner := bufio.NewScanner(limited)
	prefixRunes := []rune(prefix)
	prefixLen := len(prefixRunes)
	var children []BucketMetrics
	var currentChild string
	var childStart int64
	var childDP int64
	childDP = 1
	prevPhrase := ""
	var offset int64
	for scanner.Scan() {
		line := scanner.Text()
		//advance offset
		lineSize := int64(len(line) + 1)
		offset += lineSize
		phrase, _, ok := trie.ParseLine(line)
		if !ok {
			continue
		}
		runes := []rune(phrase)
		// processing children, if have no extra text after the prefix for the current prefix, lets continue
		if len(runes) <= prefixLen {
			continue
		}
		// if the prefix of the current phrase doesnot equal to prefix
		// continue - doubt that happens here at all -safety check
		if prefixLen > 0 && !strings.HasPrefix(phrase, prefix) {
			panic(fmt.Sprintf(
				"phrase %q outside prefix bucket %q",
				phrase,
				prefix,
			))
		}
		// child= prefix + next rune.
		child := string(runes[:prefixLen+1])
		// new child bucket
		// but here the currentCHild is techncially previousChild isnt it?
		if child != currentChild {
			// complete prev child
			if currentChild != "" {
				children = append(children, BucketMetrics{
					Prefix:           currentChild,
					Start:            childStart,
					End:              start + offset - lineSize,
					DistinctPrefixes: childDP,
				})
			}
			//new child
			currentChild = child
			// since we already increased the offset, we need to substract the lineSize from offset to get the start of the
			// new child
			childStart = start + offset - lineSize
			childDP = 0
			// reset lcp for every child.
			prevPhrase = ""
		}
		// count new distinct prefixes
		childDP += int64(
			len(runes) - max(prefixLen+1, lcp(phrase, prevPhrase)),
		)
		prevPhrase = phrase
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	//close final child
	if currentChild != "" {
		children = append(children, BucketMetrics{
			Prefix:           currentChild,
			Start:            childStart,
			End:              start + offset,
			DistinctPrefixes: childDP,
		})
	}

	return ScanResult{
		Children: children,
	}
}

func processBucket(f *os.File, prefix string, start int64, end int64, collection *mongo.Collection,
	ctx context.Context) {
	scanResult := Scan(f, prefix, start, end)

	for _, child := range scanResult.Children {
		estimated := float64(child.DistinctPrefixes) * bytesPerPrefix

		if estimated <= memoryBudget {
			// fmt.Printf(
			// 	"BUILD  %-10s prefixes=%d estimated=%.2f MB\n",
			// 	child.Prefix,
			// 	child.DistinctPrefixes,
			// 	estimated/(1024*1024),
			// )
			// pi := buildPrefixIndex(f, child)
			// fmt.Printf("MERGE  %-10s prefixes=%d\n",
			// 	child.Prefix,
			// 	len(pi),
			// )
			// if err := mergeIntoMongo(ctx, collection, pi); err != nil {
			// 	panic(err)
			// }
			// we will just queue the index build job
			indexBuildJobs <- child
		} else {
			fmt.Printf(
				"SPLIT  %-10s prefixes=%d estimated=%.2f MB\n",
				child.Prefix,
				child.DistinctPrefixes,
				estimated/(1024*1024),
			)

			processBucket(
				f,
				child.Prefix,
				child.Start,
				child.End,
				collection,
				ctx,
			)
		}
	}
}

type IndexRecord struct {
	ID     primitive.ObjectID `bson:"_id,omitempty"`
	Prefix string             `bson:"prefix"`
	Top    []trie.Reco        `bson:"top"`
}

func buildPrefixIndex(f *os.File, child BucketMetrics) PI {
	_, err := f.Seek(child.Start, io.SeekStart)
	if err != nil {
		panic(err)
	}
	limited := io.LimitReader(f, child.End-child.Start)
	scanner := bufio.NewScanner(limited)
	pi := make(PI)
	for scanner.Scan() {
		phrase, freq, ok := trie.ParseLine(scanner.Text())
		if !ok {
			continue
		}
		runeList := []rune(phrase)
		for i := 1; i <= len(runeList); i++ {
			pref := string(runeList[:i])
			if pi[pref] == nil {
				hp := &trie.RecHeap{}
				heap.Init(hp)
				pi[pref] = hp
			}
			if pi[pref].Len() < k {
				heap.Push(pi[pref], trie.Reco{Word: phrase, Freq: freq})
			} else {
				if pi[pref].Top().Freq < freq {
					heap.Pop(pi[pref])
					heap.Push(pi[pref], trie.Reco{Word: phrase, Freq: freq})
				}
			}
		}
	}
	if scanner.Err() != nil {
		panic(scanner.Err())
	}
	return pi

}

const recordBatch = 1000

func mergeTop(
	a []trie.Reco,
	b []trie.Reco,
	k int,
) []trie.Reco {

	all := make([]trie.Reco, 0, len(a)+len(b))

	all = append(all, a...)
	all = append(all, b...)

	sort.Slice(all, func(i, j int) bool {
		return all[i].Freq > all[j].Freq
	})

	if len(all) > k {
		all = all[:k]
	}

	return all
}
func mergeIntoMongo(
	ctx context.Context,
	collection *mongo.Collection,
	pi map[string]*trie.RecHeap,
) error {

	if len(pi) == 0 {
		return nil
	}

	const batchSize = 1000

	batch := make([]IndexRecord, 0, batchSize)

	flush := func(batch []IndexRecord) error {
		if len(batch) == 0 {
			return nil
		}
		prefixes := make([]string, 0, len(batch))
		for _, record := range batch {
			prefixes = append(prefixes, record.Prefix)
		}
		cursor, err := collection.Find(
			ctx,
			bson.M{
				"prefix": bson.M{
					"$in": prefixes,
				},
			},
		)
		if err != nil {
			return err
		}
		existing := make(map[string]IndexRecord, len(batch))
		for cursor.Next(ctx) {
			var record IndexRecord
			if err := cursor.Decode(&record); err != nil {
				cursor.Close(ctx)
				return err
			}
			existing[record.Prefix] = record
		}
		if err := cursor.Err(); err != nil {
			cursor.Close(ctx)
			return err
		}
		if err := cursor.Close(ctx); err != nil {
			return err
		}
		models := make([]mongo.WriteModel, 0, len(batch))
		for _, incoming := range batch {
			old, exists := existing[incoming.Prefix]
			if !exists {
				models = append(
					models,
					mongo.NewInsertOneModel().
						SetDocument(incoming),
				)
				continue
			}
			merged := mergeTop(
				old.Top,
				incoming.Top,
				k,
			)
			models = append(
				models,
				mongo.NewUpdateOneModel().
					SetFilter(
						bson.M{"prefix": incoming.Prefix},
					).
					SetUpdate(
						bson.M{
							"$set": bson.M{
								"top": merged,
							},
						},
					),
			)
		}
		if len(models) == 0 {
			return nil
		}
		_, err = collection.BulkWrite(
			ctx,
			models,
		)
		return err
	}
	for prefix, hp := range pi {
		top := make([]trie.Reco, hp.Len())
		for i := len(top) - 1; i >= 0; i-- {
			top[i] = heap.Pop(hp).(trie.Reco)
		}
		batch = append(batch, IndexRecord{
			Prefix: prefix,
			Top:    top,
		})
		if len(batch) == batchSize {
			if err := flush(batch); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
	return flush(batch)
}

func Seed(file string) {
	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	fileInfo, err := f.Stat()
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	client := GetDBConn("mongodb://localhost:27017")
	defer func() {
		if err = client.Disconnect(ctx); err != nil {
			log.Fatalf("Error closing MongoDB connection: %v", err)
		}
	}()
	collection := setupMongo(client, ctx)
	// start prefix index builders
	wg := &sync.WaitGroup{}
	go IndexBuildingWorkers(file, wg)
	// start the mongo merge worker
	// merges pi to mongo collection in the batches of 1000
	done := make(chan struct{})
	go MongoMergeWorker(ctx, collection, done)
	processBucket(
		f,
		"",
		0,
		fileInfo.Size(),
		collection,
		ctx,
	)
	// close- no new work , inflight /queued job is not affected
	close(indexBuildJobs)
	//wait for index builders
	wg.Wait()
	// close - no new work, again inflight jobs are not affected
	close(indexResults)
	//wait for merge worker
	<-done
}

func main() {
	// the canonical file is already created
	// var (
	// 	file = flag.String("file", "./sample.txt", "-file filename")
	// )
	// flag.Parse()
	// fmt.Printf("File name: %s\n", *file)
	// f, err := os.Open(*file)
	// if err != nil {
	// 	panic(err)
	// }
	// defer f.Close()
	// scanner := bufio.NewScanner(f)
	// w, err := os.Create("./normailzed_ds.txt")
	// if err != nil {
	// 	panic(err)
	// }
	// defer w.Close()
	// writer := bufio.NewWriterSize(w, 1<<20)
	// defer writer.Flush()
	// // normalize the file
	// for scanner.Scan() {
	// 	phrase, freq, ok := ParseLine(scanner.Text())
	// 	if !ok {
	// 		continue
	// 	}
	// 	normPhrase := normalize(phrase)
	// 	if normPhrase == "" {
	// 		continue
	// 	}

	// 	fmt.Fprintf(writer, "%s\t%d\n", normPhrase, freq)
	// }
	// if scanner.Err() != nil {
	// 	panic(scanner.Err())
	// }
	// // sort the file
	// cmd := exec.Command(
	// 	"sort",
	// 	"-S", "4G",
	// 	"-T", "/tmp",
	// 	"-t", "\t",
	// 	"-k1,1",
	// 	"./normailzed_ds.txt",
	// 	"-o", "./sorted_normalized.txt",
	// )

	// // Inherit existing environment variables and override LC_ALL
	// cmd.Env = append(os.Environ(), "LC_ALL=C")
	// cmd.Stderr = os.Stderr

	// if err := cmd.Run(); err != nil {
	// 	fmt.Fprintf(os.Stderr, "sort failed: %v\n", err)
	// 	os.Exit(1)
	// }

	// fmt.Println("Sorting completed successfully.")
	// var prevPhrase string
	// var prevFreq int64
	// f, err = os.Open("./sorted_normalized.txt")
	// if err != nil {
	// 	panic(err)
	// }
	// scanner = bufio.NewScanner(f)
	// finalFile, err := os.Create("./canonical_sorted.txt")
	// if err != nil {
	// 	panic(err)
	// }
	// defer finalFile.Close()
	// finalWriter := bufio.NewWriterSize(finalFile, 1<<20)
	// defer finalWriter.Flush()
	// for scanner.Scan() {
	// 	phrase, freq, ok := ParseLine(scanner.Text())
	// 	if !ok {
	// 		continue
	// 	}

	// 	if phrase == prevPhrase {
	// 		prevFreq += freq
	// 		continue
	// 	}

	// 	if prevPhrase != "" {
	// 		fmt.Fprintf(finalWriter, "%s\t%d\n", prevPhrase, prevFreq)
	// 	}

	// 	prevPhrase = phrase
	// 	prevFreq = freq
	// }

	// if prevPhrase != "" {
	// 	fmt.Fprintf(finalWriter, "%s\t%d\n", prevPhrase, prevFreq)
	// }
	// fmt.Println("final file is created")
	var (
		file = flag.String("file", "./sample.txt", "-file filename")
		kval = flag.Int("k", 10, "-k 10")
	)
	flag.Parse()
	k = *kval
	Seed(*file)
}
