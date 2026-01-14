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
type Bucket struct {
	capacity   float64
	tokens     float64
	rate       float64
	lastRefill time.Time
	lastSeen   time.Time
	now        func() time.Time //injected
	mu         sync.Mutex
}

// time is injected so tests doesnot sleep

type Limiter struct {
	store      map[string]*Bucket
	now        func() time.Time //injected clock
	mu         sync.Mutex
	ttl        time.Duration //ttl
	capacity   float64
	refillRate float64
}

// constructor of a new limiter - dependency injection point
func NewLimiter(now func() time.Time, ttl time.Duration, capacity float64, refillRate float64) *Limiter {
	return &Limiter{
		store:      make(map[string]*Bucket),
		now:        now,
		ttl:        ttl,
		capacity:   capacity,
		refillRate: refillRate,
	}
}

// limiter becomes the middleware
func (l *Limiter) MiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		// lock the store
		l.mu.Lock()
		bucket, ok := l.store[host]
		if !ok {
			// bucket doesnot exist,lets create one
			// get your now calculation logic from
			// the injection
			now := l.now()
			bucket = &Bucket{
				capacity:   l.capacity,
				tokens:     l.capacity,
				rate:       l.refillRate,
				lastRefill: now,
				lastSeen:   now,
				now:        l.now,
			}
			l.store[host] = bucket
		}
		// lock only when accessing the bucket , not when serving the request
		l.mu.Unlock()

		if bucket.allow() {
			next.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("rate limit hit"))
		}
		// here are the steps in middleware
		// get the host ip
		// lock the store
		// check if we already have a bucket for this
		// if yes, get that bucket and unlock the store
		//
		// lock the bucket
		// check if we can allow and unlock
		// if okay, allow and then process the request
		//
		// if not then return ratelimit exceeded
		//
		// if bucket doesnot exits
		//
		// then lock the store, create  a new bucket and unlock the store  and processs the request the smae way
	})
}

func (l *Limiter) StartEviction(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		// returns a Ticker type which has a
		// channel var, Reset and Stop methods on it
		defer ticker.Stop()

		for range ticker.C {
			l.evictExpired()
		}
	}()
}

// actual eviction logic
func (l *Limiter) evictExpired() {
	// get your now logic from the injection
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	for ip, bucket := range l.store {
		bucket.mu.Lock()
		idle := now.Sub(bucket.lastSeen)
		bucket.mu.Unlock()

		if idle > l.ttl {
			delete(l.store, ip)
		}
	}
}

var (
	store = make(map[string]*Bucket)
	mu    sync.Mutex
)

// refil logic is changed from background ticker in a separate goroutine to
//
// a time based on - on demand - more efficient
func (b *Bucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.lastRefill = now
	b.lastSeen = now
	if b.tokens += elapsed * b.rate; b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false

}

func handleHome(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("you are allowed"))
}

var bucketSize float64
var refillRate float64

func main() {
	flag.Float64Var(&bucketSize, "size", 10, "size/capacity of the bucket")
	flag.Float64Var(&refillRate, "rate", 2, "refill rate of the bucket")

	flag.Parse()

	fmt.Println("bucket size: ", bucketSize)
	fmt.Println("refill rate: ", refillRate)

	limiter := NewLimiter(time.Now, 6*time.Minute, bucketSize, refillRate)

	limiter.StartEviction(3 * time.Minute)

	mux := http.NewServeMux()
	// middleware for initiating
	mux.Handle("/", limiter.MiddleWare(http.HandlerFunc(handleHome)))
	log.Fatal(http.ListenAndServe("localhost:8081", mux))
}
