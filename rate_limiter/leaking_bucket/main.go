package main

import (
	"flag"
	"log"
	"net/http"
	"sync"
	"time"
)

func HandleHome(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("you are allowed"))
}

type Limiter struct {
	store map[string]*Bucket
	mu    sync.Mutex
}

type Bucket struct {
	capacity float64
	level    float64
	leakRate float64
	last     time.Time //last time leak happened
	mu       sync.Mutex
}

func (l *Limiter) MiddleWare(next http.Handler) http.Handler {
	// logic
}

var (
	capacity float64
	leakRate float64
)

func main() {
	flag.Float64Var(&capacity, "capacity of the bucket", 20, "capacity of the bucket")
	flag.Float64Var(&leakRate, "leak rate", 5, "leak rate of the bucket")
	limiter := Limiter{
		store: make(map[string]*Bucket),
	}
	mux := http.NewServeMux()
	mux.Handle("/", limiter.MiddleWare(http.HandlerFunc(HandleHome)))
	log.Fatal(http.ListenAndServe("localhost:8081", mux))
}
