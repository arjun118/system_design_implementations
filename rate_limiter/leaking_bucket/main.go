package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"sync"
	"time"
)

const epsilon = 1e-9

func HandleHome(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("you are allowed"))
}

type Limiter struct {
	store    map[string]*Bucket
	mu       sync.Mutex
	capacity float64
	leakRate float64
	now      func() time.Time //clock injected
	ttl      time.Duration
}

type Bucket struct {
	capacity  float64
	level     float64
	leakRate  float64
	lastDrain time.Time //last time leak happened
	lastSeen  time.Time //use for eviction
	now       func() time.Time
	mu        sync.Mutex
}

func (b *Bucket) allow() bool {
	// logic to process the requests
	// lock the bucket
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	// current := b.now()
	elapsed := now.Sub(b.lastDrain)
	// simulate leaking since last request
	levelDrainPredicted := b.leakRate * elapsed.Seconds()
	// update last drained
	b.lastDrain = now
	// update lastSeen
	// client spamming rejected requests will never be evicted
	// client seems always active
	b.lastSeen = now
	// leak before capacity check
	b.level = math.Max(0, b.level-levelDrainPredicted)
	if b.level > b.capacity {
		b.level = b.capacity
	}
	if b.level+1 > b.capacity+epsilon {
		// overflows, drop the request
		// fmt.Printf("capcity over: level: %f cap: %f\n", b.level+1, b.capacity)
		return false
	} else {
		b.level += 1
		return true
	}
}

// create  a new limiter
func NewLimiter(capacity float64, leakRate float64, now func() time.Time, ttl time.Duration) *Limiter {
	return &Limiter{
		store:    make(map[string]*Bucket),
		capacity: capacity,
		leakRate: leakRate,
		now:      now,
		ttl:      ttl,
	}
}

func (l *Limiter) MiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// lock the store while getting
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		l.mu.Lock()
		bucket, ok := l.store[host]
		now := l.now()
		if ok {
			bucket.mu.Lock()
			idle := now.Sub(bucket.lastSeen)
			bucket.mu.Unlock()
			// expired ,delete the old and create a new one
			if idle > l.ttl {
				delete(l.store, host)
				bucket = nil
				ok = false
			}
		}
		if !ok {
			bucket = &Bucket{
				capacity:  l.capacity,
				level:     0, //initial level is zero
				leakRate:  l.leakRate,
				lastDrain: l.now(),
				lastSeen:  l.now(),
				now:       l.now,
			}
			l.store[host] = bucket
		}
		l.mu.Unlock()

		// check and process
		if bucket.allow() {
			next.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("you are rate limited"))
		}
	})
}

var (
	capacity float64
	leakRate float64
	ttl      time.Duration
)

func main() {
	flag.Float64Var(&capacity, "size", 2, "capacity of the bucket- set the capacity of the bucket")
	flag.Float64Var(&leakRate, "rate", 1, "leak rate of the bucket - number of requests to be processed per second")
	flag.DurationVar(&ttl, "ttl", 2*time.Minute, "time to live, inactive more than this duration,will be evicted")
	fmt.Printf("capacity: %f \nleak rate: %f\n", capacity, leakRate)
	limiter := NewLimiter(capacity, leakRate, time.Now, ttl)
	mux := http.NewServeMux()
	mux.Handle("/", limiter.MiddleWare(http.HandlerFunc(HandleHome)))
	log.Fatal(http.ListenAndServe("localhost:8081", mux))
}
