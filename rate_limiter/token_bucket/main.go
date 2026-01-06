package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// this is the final version
// refactored, made better using chatgpt
//

var (
	bucketSize int
	refillRate int
)

var (
	store = make(map[string]*Bucket)
	mu    sync.Mutex
)

type Bucket struct {
	capacity   float64
	tokens     float64
	rate       float64
	lastRefill time.Time
	mu         sync.Mutex
}

// refil logic is changed from background ticker in a separate goroutine to
//
// a time based on - on demand - more efficient
func (b *Bucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.lastRefill = now

	if b.tokens += elapsed * b.rate; b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false

}

func bucketCheckerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)

		// lock store while getting or creating a bucket
		//
		mu.Lock()
		bucket, ok := store[host]
		if !ok {
			bucket = &Bucket{
				capacity:   float64(bucketSize),
				tokens:     float64(bucketSize),
				rate:       float64(refillRate),
				lastRefill: time.Now(),
			}
			store[host] = bucket
		}
		mu.Unlock()

		// check rate limit
		if bucket.allow() {
			next.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("rate limit hit"))
		}

	})
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("you are allowed"))
}

func main() {
	flag.IntVar(&bucketSize, "size", 50, "size/capacity of the bucket")
	flag.IntVar(&refillRate, "rate", 10, "refill rate of the bucket")

	flag.Parse()

	fmt.Println("bucket size: ", bucketSize)
	fmt.Println("refill rate: ", refillRate)

	mux := http.NewServeMux()
	// middleware for initiating
	mux.Handle("/", bucketCheckerMiddleware(http.HandlerFunc(handleHome)))
	log.Fatal(http.ListenAndServe("localhost:8081", mux))
}
