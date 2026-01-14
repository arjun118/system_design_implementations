package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFirstRequestAllowed(t *testing.T) {
	// t.Parallel()
	bucketSize = 2
	refillRate = 1

	store = make(map[string]*Bucket)

	handler := bucketCheckerMiddleware(http.HandlerFunc(handleHome))

	req := httptest.NewRequest("GET", "http://localhost:8081", nil)

	req.RemoteAddr = "1.2.3.4:1111"

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	// first reqeust should be allowed
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRateLimitExceeded(t *testing.T) {
	// t.Parallel()
	bucketSize = 1
	refillRate = 0
	// no tokens at all - so no request should be served

	store = make(map[string]*Bucket)

	handler := bucketCheckerMiddleware(http.HandlerFunc(handleHome))

	req := httptest.NewRequest("GET", "http://localhost:8081", nil)
	req.RemoteAddr = "1.2.3.4:1111"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("first request : expected 200, got %d", rec.Code)
	}
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", rec2.Code)
	}

}

func TestRefillAllowsRequestsAgain(t *testing.T) {
	// this will fail , need to change this test
	// t.Parallel()
	bucketSize = 1
	refillRate = 1
	store = make(map[string]*Bucket)

	handler := bucketCheckerMiddleware(http.HandlerFunc(handleHome))

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.RemoteAddr = "1.2.3.4:1111"

	// First request - allowed
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)

	if rec1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rec1.Code)
	}

	// Second request immediately - blocked
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", rec2.Code)
	}

	// Third request - should be allowed again
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req)

	if rec3.Code != http.StatusOK {
		t.Fatalf("after refill: expected 200, got %d", rec3.Code)
	}
}

func TestDifferentIPsHaveDifferentBuckets(t *testing.T) {
	// t.Parallel()
	bucketSize = 1
	refillRate = 0
	store = make(map[string]*Bucket)

	handler := bucketCheckerMiddleware(http.HandlerFunc(handleHome))

	req1 := httptest.NewRequest("GET", "http://example.com/", nil)
	req1.RemoteAddr = "1.1.1.1:1234"

	req2 := httptest.NewRequest("GET", "http://example.com/", nil)
	req2.RemoteAddr = "2.2.2.2:1234"

	// First IP
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("ip1 first request: expected 200")
	}

	// Same IP again -> should be blocked
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req1)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("ip1 second request: expected 429")
	}

	// Different IP -> should still be allowed
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req2)
	if rec3.Code != http.StatusOK {
		t.Fatalf("ip2 first request: expected 200")
	}
}

func TestConcurrentSameIP_NoRefill(t *testing.T) {
	// t.Parallel()
	bucketSize = 5
	refillRate = 0

	store = make(map[string]*Bucket)

	handler := bucketCheckerMiddleware(http.HandlerFunc(handleHome))

	req := httptest.NewRequest("GET", "http://localhost:8081", nil)
	req.RemoteAddr = "1.2.3.4:1111"

	var wg sync.WaitGroup

	const goroutines = 100

	var allowed int64
	var blocked int64

	for range 100 {
		wg.Add(1)

		go func() {
			wg.Done()

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			switch rec.Code {
			case http.StatusOK:
				atomic.AddInt64(&allowed, 1)
			case http.StatusTooManyRequests:
				atomic.AddInt64(&blocked, 1)
			default:
				t.Errorf("unexpected status: %d", rec.Code)
			}

		}()
	}
	wg.Wait()
	if allowed != int64(bucketSize) {
		t.Fatalf("expected %d allowed requests, got %d", bucketSize, allowed)
	}

	if blocked != int64(goroutines-bucketSize) {
		t.Fatalf("expected %d blocked requests, got %d", goroutines-bucketSize, blocked)
	}
}

func TestConcurrentSameIP_WithRefill(t *testing.T) {
	bucketSize = 5
	refillRate = 5 // 5 tokens per second
	store = make(map[string]*Bucket)

	handler := bucketCheckerMiddleware(http.HandlerFunc(handleHome))

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.RemoteAddr = "1.2.3.4:1111"

	var wg sync.WaitGroup
	const goroutines = 50

	var allowed int64
	start := time.Now()

	for range goroutines {
		wg.Go(func() {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code == http.StatusOK {
				atomic.AddInt64(&allowed, 1)
			}
		})
	}

	wg.Wait()
	elapsed := time.Since(start).Seconds()

	maxAllowed := float64(bucketSize) + elapsed*float64(refillRate)

	if float64(allowed) > maxAllowed+1 { // +1 to tolerate tiny timing drift
		t.Fatalf("too many allowed requests: got %d, expected at most %.2f",
			allowed, maxAllowed)
	}
}

func TestConcurrentDifferentIPs(t *testing.T) {
	bucketSize = 1
	refillRate = 0
	store = make(map[string]*Bucket)

	handler := bucketCheckerMiddleware(http.HandlerFunc(handleHome))

	var wg sync.WaitGroup
	const clients = 50

	var allowed int64

	for i := range clients {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			req := httptest.NewRequest("GET", "http://example.com/", nil)
			req.RemoteAddr = fmt.Sprintf("10.0.0.%d:1234", i)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code == http.StatusOK {
				atomic.AddInt64(&allowed, 1)
			}
		}(i)
	}

	wg.Wait()

	if allowed != int64(clients) {
		t.Fatalf("expected %d allowed (one per IP), got %d", clients, allowed)
	}
}
