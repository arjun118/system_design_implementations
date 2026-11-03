One distinction: if the status update itself fails—for example, the handler succeeded but Ack failed—don’t treat that as an ordinary handler retry. Log and surface it as an infrastructure/reporting error; the server may eventually redeliver the job, so handlers should be designed for possible duplicate execution.

## on LeaseDuration and HeartbeatInterval for WorkerConfig

1. HeartbeatInterval is better when = LeaseDuration/3

## migrations

ALTER TABLE jobs ADD COLUMN lease_owner TEXT;
ALTER TABLE jobs ADD COLUMN lease_until TIMESTAMPTZ;
ALTER TABLE jobs ADD COLUMN lease_token INT NOT NULL DEFAULT 0;
ALTER TABLE jobs ADD COLUMN last_error TEXT;

CREATE INDEX idx_jobs_lease_expired ON jobs (lease_until)
WHERE status = 'running';

add the index later

# notes on jitter and backoff

naive exponential backoff

```py
sleep = min(cap, base* 2^attempt)
```

## full jitter

```py
temp = min(cap, base * 2^ attempt)
sleep = random_between(0,temp)
```

> It significantly reduces the peak load on the downstream system. However, because it can pick 0 or a very low number, some jobs will retry almost immediately.

## equal jitter

```py
temp = min(cap, base * 2^attempt)
sleep = (temp / 2) + random_between(0, temp / 2)
```

> Equal jitter strikes a balance by guaranteeing that the job waits at least half of the exponential backoff time, while adding randomness to the remaining half.

> It prevents the immediate, rapid-fire retries that can occasionally happen with Full Jitter, ensuring the downstream service always gets at least a brief cooling-off period, though it clusters the retries slightly more than Full Jitter.

## decorrelated jitter

```py
// Initialization (first failure)
sleep = base

// Subsequent failures
sleep = min(cap, random_between(base, sleep * 3))
```

> This approach creates a highly dispersed distribution of retries over time. It is particularly effective in massive distributed systems where you want to ensure that concurrent failures instantly drift apart in their retry cadences.

## conclusion

Most production-grade schedulers default to Full Jitter because it yields the lowest peak workload on recovering dependencies while maintaining a high overall completion rate.

## benchmark engineering rules

1. **Never fake benchmarks with backdoors**: Never inject direct SQL updates (`UPDATE run_at = now()`) inside benchmark loops to artificially bypass real delays.
2. **Exercise real system end-to-end**: Use the real SDK `Worker` pool with realistic concurrency.
3. **Measure actual throughput**: Pre-submit jobs, run workers through HTTP `/claim`, `/nack`, reschedule, and `/ack`, and measure wall time until all completed.
