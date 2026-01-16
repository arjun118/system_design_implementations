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
	return &fakeClock{now: time.Unix(0, 0)}
}

// 2. TestRateLimitExceeded
// - with normal params of capacity and leakRate, send more requests - see them ratelimited with no time advance
// 3. TestLeakAllowRequests
// - same im
//   - fill bucket
//   - hit it and face rate limit
//   - advance time (some requests should be leaked)
//   - test again, see the requests go through
func TestRateLimitExceeded(t *testing.T) {
	// same ip too many requests
	// simulating fakeclock - no time advance, hence no leak and rest all requests are rejected execept first cap no of req
	capacity = 20
	leakRate = 5
	clock := newFakeClock()
	limiter := NewLimiter(capacity, leakRate, clock.Now, ttl)
	handler := limiter.MiddleWare(http.HandlerFunc(HandleHome))

	var allowed, rejected int64
	var wg sync.WaitGroup
	total := 200
	for range total {
		wg.Go(func() {
			req := httptest.NewRequest("GET", "http://localhost:8081", nil)
			req.RemoteAddr = "1.2.3.4:1111"

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			switch rec.Code {
			case http.StatusOK:
				atomic.AddInt64(&allowed, 1)
			case http.StatusTooManyRequests:
				atomic.AddInt64(&rejected, 1)
			default:
				t.Fatalf("unexpected status: %d", rec.Code)
			}
		})
	}
	wg.Wait()
	if allowed != int64(capacity) {
		t.Fatalf("expected %d allowed req, got %d\n", int(capacity), allowed)
	}
	if rejected != int64(total-int(capacity)) {
		t.Fatalf("expected %d rejected req, got %d\n", total-int(capacity), rejected)
	}
}

func TestLeakAllowRequests(t *testing.T) {
	capacity = 20
	leakRate = 5
	clock := newFakeClock()
	limiter := NewLimiter(capacity, leakRate, clock.Now, ttl)
	handler := limiter.MiddleWare(http.HandlerFunc(HandleHome))
	var allowed, rejected int64
	var wg sync.WaitGroup
	// fill the bucket,until brim, i.e upto capacity - should be processed
	// all these requests should be allowed
	for range int(capacity) {
		wg.Go(func() {
			req := httptest.NewRequest("GET", "http://localhost:8081", nil)
			req.RemoteAddr = "1.2.3.4:1111"
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)
			switch rec.Code {
			case http.StatusOK:
				atomic.AddInt64(&allowed, 1)
			case http.StatusTooManyRequests:
				atomic.AddInt64(&rejected, 1)
			default:
				t.Fatalf("unexpected status: %d", rec.Code)
			}
		})
	}
	wg.Wait()
	if allowed != int64(capacity) {
		t.Fatalf("expected %d allowed req, got %d\n", int(capacity), allowed)
	}
	if rejected != 0 {
		t.Fatalf("expected %d rejected req, got %d\n", 0, rejected)
	}
	// without advancing time, lets send one more request - that should be rejected
	req := httptest.NewRequest("GET", "http://localhost:8081", nil)
	req.RemoteAddr = "1.2.3.4:1111"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
	// advance time and send 5 requests - these should be accepted
	clock.Advance(1 * time.Second)
	allowed = 0
	rejected = 0
	for range int(leakRate) {
		wg.Go(func() {
			req := httptest.NewRequest("GET", "http://localhost:8081", nil)
			req.RemoteAddr = "1.2.3.4:1111"
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)
			switch rec.Code {
			case http.StatusOK:
				atomic.AddInt64(&allowed, 1)
			case http.StatusTooManyRequests:
				atomic.AddInt64(&rejected, 1)
			default:
				t.Fatalf("unexpected status: %d", rec.Code)
			}
		})
	}
	wg.Wait()
	if allowed != int64(leakRate) {
		t.Fatalf("expected %d allowed req, got %d\n", int(leakRate), allowed)
	}
	if rejected != 0 {
		t.Fatalf("expected %d rejected req, got %d\n", 0, rejected)
	}
}

//  turns out i dont really need to use the concurrent requests when i am using a fakeclock via clock injection
// it takes care of it
//
// Concurrency tests mutexes
// Time tests rate limiters

func TestLeakAllowRequestsGPTVersion(t *testing.T) {
	const (
		capacity = 20
		leakRate = 5
	)

	clock := newFakeClock()
	limiter := NewLimiter(
		float64(capacity),
		float64(leakRate),
		clock.Now,
		10*time.Minute,
	)

	handler := limiter.MiddleWare(http.HandlerFunc(HandleHome))

	// 1️⃣ Fill the bucket
	for i := 0; i < capacity; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "1.2.3.4:1111"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d unexpectedly rejected", i)
		}
	}

	// 2️⃣ One more → rejected
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1111"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected rate limit, got %d", rec.Code)
	}

	// 3️⃣ Advance time → leak 5
	clock.Advance(1 * time.Second)

	// 4️⃣ Exactly 5 should be allowed
	for i := 0; i < leakRate; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "1.2.3.4:1111"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected request %d to be allowed after leak", i)
		}
	}

	// 5️⃣ Next one should be rejected again
	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1111"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected rate limit after refill, got %d", rec.Code)
	}
}
