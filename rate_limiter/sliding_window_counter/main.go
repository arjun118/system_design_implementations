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

type Limiter struct {
	// limiter has map
	Store      map[string]*WindowManager
	WindowSize int64
	Limit      int
	Ttl        time.Duration
	now        func() time.Time
	mu         sync.Mutex
}

func NewLimiter(windowsize int64, limit int, ttl time.Duration, now func() time.Time) *Limiter {
	if windowsize <= 0 {
		panic("window size should be > 0")
	}
	return &Limiter{
		Store:      make(map[string]*WindowManager),
		WindowSize: windowsize,
		Limit:      limit,
		Ttl:        ttl,
		now:        now,
	}
}
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// middle ware logic
		// handler ip and all
		// also evict here itself, no background eviction
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		fmt.Println(host)
		now := l.now()
		l.mu.Lock()
		windowManager, ok := l.Store[host]
		l.mu.Unlock()
		if ok && windowManager.Expired(now, l.Ttl) {

			l.mu.Lock()

			// double-check (important!) - gpt
			// We are making sure we only delete the same object we originally observed,
			// not a newer replacement that another goroutine may have installed.
			// This is about time-of-check vs time-of-use under concurrency.
			if current, stillOk := l.Store[host]; stillOk && current == windowManager {
				delete(l.Store, host)
				ok = false
			}

			l.mu.Unlock()
		}
		if !ok {
			windowManager = &WindowManager{
				windowSize: l.WindowSize,
				limit:      l.Limit,
				counter: WindowCounter{
					windowId:     0,
					currentCount: 0,
					prevCount:    0,
				},
				lastSeen: now,
				now:      l.now,
			}
			l.mu.Lock()
			l.Store[host] = windowManager
			l.mu.Unlock()
		}

		if windowManager.allow() {
			next.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("you are ratelimited"))
			return
		}
	})
}

type WindowCounter struct {
	windowId     int64
	currentCount int
	// prevwindow is always currentwindow-1
	prevCount int
}
type WindowManager struct {
	// similar to bucket in the previous leaking bucket and token bucket
	windowSize int64
	limit      int
	// at a time we have to deal with only one window and we also will have only one window basis the elapsed time since unix epoch
	counter  WindowCounter
	lastSeen time.Time
	now      func() time.Time
	mu       sync.Mutex
}

func (wm *WindowManager) Expired(now time.Time, ttl time.Duration) bool {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	return now.Sub(wm.lastSeen) >= ttl
}

func (wm *WindowManager) allow() bool {
	// check rate limiting logic here
	wm.mu.Lock()
	defer wm.mu.Unlock()
	now := wm.now()
	wm.lastSeen = now
	currentWindowId := now.Unix() / wm.windowSize
	if wm.counter.windowId == currentWindowId {
		// we still in the same window
		// if wm.counter.currentCount < wm.limit {
		// 	wm.counter.currentCount++
		// 	return true
		// } else {
		// 	return false
		// }

	} else if currentWindowId == wm.counter.windowId+1 {
		// we went into the next window
		wm.counter.prevCount = wm.counter.currentCount
		wm.counter.currentCount = 0
		wm.counter.windowId = currentWindowId
	} else {
		// we gone too far, next request arrived very late
		wm.counter.windowId = currentWindowId
		wm.counter.prevCount = 0
		wm.counter.currentCount = 0
	}
	currentWindowStart := currentWindowId * wm.windowSize
	elapsedIntoCurrentWindow := now.Unix() - currentWindowStart
	overlapPercentage := float64(wm.windowSize-elapsedIntoCurrentWindow) / float64(wm.windowSize)
	if overlapPercentage < 0 {
		overlapPercentage = 0
	}
	estimatedRequestsInRollingWindow := float64(wm.counter.currentCount) + float64(wm.counter.prevCount)*overlapPercentage
	if int(estimatedRequestsInRollingWindow) >= wm.limit {
		return false
	}
	wm.counter.currentCount++
	return true
}

var (
	WindowSize int64
	Limit      int
	TTL        time.Duration
)

func HandleHome(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("you are allowed"))
}

func main() {
	flag.Int64Var(&WindowSize, "window_size", 5, "window size in seconds")
	flag.IntVar(&Limit, "limit", 1, "number of allowed requests per window")
	flag.DurationVar(&TTL, "ttl", 5*time.Minute, "time to live for the window")
	fmt.Printf("window size %d, limit %d\n", WindowSize, Limit)
	NewLimiter := NewLimiter(WindowSize, Limit, TTL, time.Now)
	handler := NewLimiter.Middleware(http.HandlerFunc(HandleHome))
	mux := http.NewServeMux()
	mux.Handle("/", handler)
	log.Fatal(http.ListenAndServe("localhost:8081", mux))
}
