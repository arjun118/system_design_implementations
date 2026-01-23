package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(20, 0)}
}

func TestRateLimit(t *testing.T) {
	// just test ratelimit
	fc := newFakeClock()
	limiter := NewLimiter(1, 5, 5*time.Minute, fc.Now)
	handler := limiter.Middleware(http.HandlerFunc(HandleHome))
	for range 5 {
		// all these should be accepted
		req := httptest.NewRequest("GET", "http://localhost:8081", nil)
		req.RemoteAddr = "1.2.3.4"
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.Code)
		}
	}
	// this should fail
	req := httptest.NewRequest("GET", "http://localhost:8081", nil)
	req.RemoteAddr = "1.2.3.4"
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", res.Code)
	}
	fc.Advance(time.Second)
	// this should pass
	for range 5 {
		// all these should be accepted
		req := httptest.NewRequest("GET", "http://localhost:8081", nil)
		req.RemoteAddr = "1.2.3.4"
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.Code)
		}
	}

}

func TestEviction(t *testing.T) {
	fc := newFakeClock()
	limiter := NewLimiter(5, 1, 2*time.Second, fc.Now)
	handler := limiter.Middleware(http.HandlerFunc(HandleHome))
	// this should be accepted
	req := httptest.NewRequest("GET", "http://localhost:8081", nil)
	req.RemoteAddr = "1.2.3.4"
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	// this should fail
	req = httptest.NewRequest("GET", "http://localhost:8081", nil)
	req.RemoteAddr = "1.2.3.4"
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", res.Code)
	}
	// advance time such that eviction is occured but still in time window
	fc.Advance(3 * time.Second)
	// this should pass
	// if not evicted, this wil not be allowed
	req = httptest.NewRequest("GET", "http://localhost:8081", nil)
	req.RemoteAddr = "1.2.3.4"
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

}

func TestBurstTraffic(t *testing.T) {
	fc := newFakeClock()
	limiter := NewLimiter(5, 10, 5*time.Minute, fc.Now)
	handler := limiter.Middleware(http.HandlerFunc(HandleHome))
	// lets advance to the 4th second
	fc.Advance(4 * time.Second)
	// still in the first window - so all should be accepted
	for range 10 {
		// all these should be accepted
		req := httptest.NewRequest("GET", "http://localhost:8081", nil)
		req.RemoteAddr = "1.2.3.4"
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.Code)
		}
	}
	// lets advance further to the second window
	fc.Advance(time.Second)
	// this should pass too because the next window starts
	for range 10 {
		// all these should be accepted
		req := httptest.NewRequest("GET", "http://localhost:8081", nil)
		req.RemoteAddr = "1.2.3.4"
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.Code)
		}
	}

}

func TestConcurrentRateLimit(t *testing.T) {
	fc := newFakeClock()
	limiter := NewLimiter(1, 5, 5*time.Minute, fc.Now)
	handler := limiter.Middleware(http.HandlerFunc(HandleHome))

	var wg sync.WaitGroup
	success := int32(0)

	requests := 50
	wg.Add(requests)

	for range requests {
		go func() {
			defer wg.Done()

			req := httptest.NewRequest("GET", "http://localhost:8081", nil)
			req.RemoteAddr = "1.2.3.4"

			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)

			if res.Code == http.StatusOK {
				atomic.AddInt32(&success, 1)
			}
		}()
	}

	wg.Wait()

	if success != 5 {
		t.Fatalf("expected exactly 5 allowed requests, got %d", success)
	}
}
