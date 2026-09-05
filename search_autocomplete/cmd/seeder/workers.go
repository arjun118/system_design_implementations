package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"go.mongodb.org/mongo-driver/mongo"
)

func IndexBuildingWorkers(path string, wg *sync.WaitGroup) {
	for range 6 {
		// there are 5 workers at any time - working on building prefix indexes
		// each's memory limit is capped to 500mb (done via estimated memory from the processBucket)
		wg.Go(func() {
			f, err := os.Open(path) // one handle per worker
			if err != nil {
				panic(err)
			}
			defer f.Close()
			for child := range indexBuildJobs {
				estimated := float64(child.DistinctPrefixes) * bytesPerPrefix
				fmt.Printf(
					"BUILD  %-10s prefixes=%d estimated=%.2f MB\n",
					child.Prefix,
					child.DistinctPrefixes,
					estimated/(1024*1024),
				)
				pi := buildPrefixIndex(f, child)
				fmt.Printf("QUEUEING - MERGE  %-10s prefixes=%d\n",
					child.Prefix,
					len(pi),
				)
				indexResults <- pi
			}

		})
	}
}

func MongoMergeWorker(ctx context.Context, collection *mongo.Collection, done chan struct{}) {
	for pi := range indexResults {
		mergeIntoMongo(ctx, collection, pi)
	}
	close(done)
}
