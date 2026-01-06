# why Bucket need to have a mutex on it, why not a mutex on store good enough
- dont the mutex on a bucket become a bottle neck in serving multiple requests from an ip
at the same time (concurrency will be hit?)

ans:

- store mutex protects the map
- Bucket.mu protects the state of the single protect

# Doesn’t Bucket Mutex Become a Bottleneck for One IP?
ans:
Yes — and that is correct behavior.

For a single client:

> Requests must be serialized

Otherwise, 10 concurrent requests could all read tokens = 1 and all pass

Concurrency across different IPs:
✔ Fully parallel

Concurrency within same IP:
✔ Serialized (by design)

That’s exactly what a rate limiter is supposed to do.

🧠 Best Practice You Should Internalize

Use fine-grained locks that protect only the data that must be consistent.

One global lock = terrible scalability

One per-client lock = correct and scalable

# Why Time-Based Instead of Background Goroutine Token Fill?
- background refill
  - Goroutine Explosion
    - 1 IP = 1 goroutine
    - 100k IPs = 100k goroutines
    - They never die

  - Wasted CPU
    - Tickers fire even when:
      - No traffic
      - No tokens needed
- timebased accounting
  - Zero Goroutines
    - No background workers
    - No leaks
      
  - O(1) Work Per Request
    - Only compute when request arrives
      
  - Deterministic & Testable
    - You can inject time
    - No sleeping
    
# in the allow logic, wont the available tokens be in float? and isn't that bad?

- fractional tokens just accumulate until they reach one
