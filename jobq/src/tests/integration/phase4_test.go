//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"arjun118.github.io/jobq-go"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestPhase4(t *testing.T) {
	ts := newTestServer(t)
	client := jobq.NewClient(ts.URL)
	ctx := context.Background()

	t.Run("TestRetry_ExponentialBackoff", func(t *testing.T) {
		queue := "p4_backoff_" + uuid.NewString()[:8]
		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:       queue,
			JobType:     "flaky",
			Payload:     json.RawMessage(`{}`),
			MaxAttempts: 5,
		})
		require.NoError(t, err)

		// Test 3 failure progressions and verify exponential ceiling for each
		for attempt := 1; attempt <= 3; attempt++ {
			// Fast forward run_at so it can be claimed
			_, err = ts.store.Pool().Exec(ctx, "UPDATE jobs SET run_at = now() WHERE id = $1", jobID)
			require.NoError(t, err)

			j, err := client.Claim(ctx, []string{queue}, "w1", 30)
			require.NoError(t, err)

			beforeNack := time.Now()
			err = client.NAck(ctx, j.ID, j.LeaseToken, fmt.Errorf("fail attempt %d", attempt), false)
			require.NoError(t, err)

			dbJob, err := client.GetJob(ctx, jobID)
			require.NoError(t, err)
			require.Equal(t, "pending", dbJob.Status)
			require.Equal(t, attempt, dbJob.Attempts)

			// Full Jitter ceiling: BaseDelay * 2^attempt
			ceiling := 1 * time.Second * (1 << attempt)
			delay := dbJob.RunAt.Sub(beforeNack)
			require.GreaterOrEqual(t, delay, -50*time.Millisecond, "delay must not be in the past")
			require.LessOrEqual(t, delay, ceiling+500*time.Millisecond, "delay must be bounded by exponential ceiling")
		}
	})

	t.Run("TestRetry_MaxAttempts", func(t *testing.T) {
		queue := "p4_maxatt_" + uuid.NewString()[:8]
		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:       queue,
			JobType:     "always_fails",
			Payload:     json.RawMessage(`{}`),
			MaxAttempts: 3,
		})
		require.NoError(t, err)

		for i := 1; i <= 3; i++ {
			// Fast-forward run_at so it is claimable
			_, err = ts.store.Pool().Exec(ctx, "UPDATE jobs SET run_at = now() WHERE id = $1", jobID)
			require.NoError(t, err)

			j, err := client.Claim(ctx, []string{queue}, "w1", 30)
			require.NoError(t, err)

			err = client.NAck(ctx, j.ID, j.LeaseToken, fmt.Errorf("fail attempt %d", i), false)
			require.NoError(t, err)
		}

		// After 3rd attempt, status must be 'dead'
		dbJob, err := client.GetJob(ctx, jobID)
		require.NoError(t, err)
		require.Equal(t, "dead", dbJob.Status)
		require.Equal(t, 3, dbJob.Attempts)
		require.Nil(t, dbJob.LeaseOwner)
		require.Nil(t, dbJob.LeaseUntil)
	})

	t.Run("TestRetry_SuccessOnThird", func(t *testing.T) {
		queue := "p4_success3_" + uuid.NewString()[:8]
		var attempts atomic.Int32

		worker := jobq.NewWorker(client, jobq.WorkerConfig{
			Queues:       []string{queue},
			Concurrency:  1,
			PollInterval: 50 * time.Millisecond,
			Logger:       zerolog.Nop(),
		})

		worker.Handle("retry_flaky", func(ctx context.Context, payload json.RawMessage) error {
			n := attempts.Add(1)
			if n < 3 {
				return fmt.Errorf("transient failure %d", n)
			}
			return nil
		})

		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:       queue,
			JobType:     "retry_flaky",
			Payload:     json.RawMessage(`{}`),
			MaxAttempts: 5,
		})
		require.NoError(t, err)

		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go func() {
			_ = worker.Run(runCtx)
		}()

		// Job completes on third attempt naturally after waiting for backoff run_at
		require.Eventually(t, func() bool {
			job, err := client.GetJob(ctx, jobID)
			return err == nil && job.Status == "completed"
		}, 10*time.Second, 100*time.Millisecond)

		finalJob, err := client.GetJob(ctx, jobID)
		require.NoError(t, err)
		require.Equal(t, "completed", finalJob.Status)
		require.Equal(t, 3, finalJob.Attempts)
	})

	t.Run("TestPermanentNack_SkipsRetry", func(t *testing.T) {
		queue := "p4_perm_" + uuid.NewString()[:8]
		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:       queue,
			JobType:     "fatal",
			Payload:     json.RawMessage(`{}`),
			MaxAttempts: 5,
		})
		require.NoError(t, err)

		j, err := client.Claim(ctx, []string{queue}, "w1", 30)
		require.NoError(t, err)

		// Permanent = true
		err = client.NAck(ctx, j.ID, j.LeaseToken, fmt.Errorf("bad payload format"), true)
		require.NoError(t, err)

		// Dead immediately on attempt 1
		dbJob, err := client.GetJob(ctx, jobID)
		require.NoError(t, err)
		require.Equal(t, "dead", dbJob.Status)
		require.Equal(t, 1, dbJob.Attempts)
	})

	t.Run("TestDelayedJob", func(t *testing.T) {
		queue := "p4_delayed_" + uuid.NewString()[:8]

		// Job delayed by 2 seconds
		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:        queue,
			JobType:      "delayed",
			Payload:      json.RawMessage(`{}`),
			DelaySeconds: 2,
		})
		require.NoError(t, err)

		// Should not be claimable immediately
		_, err = client.Claim(ctx, []string{queue}, "w1", 30)
		require.ErrorIs(t, err, jobq.ErrNoJobAvailable)

		// Wait until delay passes
		time.Sleep(2100 * time.Millisecond)

		// Now claimable
		j, err := client.Claim(ctx, []string{queue}, "w1", 30)
		require.NoError(t, err)
		require.Equal(t, jobID, j.ID)

		err = client.Ack(ctx, j.ID, j.LeaseToken)
		require.NoError(t, err)
	})

	t.Run("TestDeadLetter_ManualRetry", func(t *testing.T) {
		queue := "p4_retry_" + uuid.NewString()[:8]
		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:       queue,
			JobType:     "work",
			Payload:     json.RawMessage(`{}`),
			MaxAttempts: 1,
		})
		require.NoError(t, err)

		job, err := client.Claim(ctx, []string{queue}, "w1", 30)
		require.NoError(t, err)

		err = client.NAck(ctx, job.ID, job.LeaseToken, fmt.Errorf("fatal error"), true)
		require.NoError(t, err)

		deadJob, err := client.GetJob(ctx, jobID)
		require.NoError(t, err)
		require.Equal(t, "dead", deadJob.Status)
		require.Equal(t, 1, deadJob.Attempts)

		// Retry endpoint via SDK
		err = client.RetryDeadJob(ctx, jobID)
		require.NoError(t, err)

		retriedJob, err := client.GetJob(ctx, jobID)
		require.NoError(t, err)
		require.Equal(t, "pending", retriedJob.Status)
		require.Equal(t, 0, retriedJob.Attempts)
		require.Nil(t, retriedJob.LeaseOwner)
		require.Nil(t, retriedJob.LeaseUntil)

		// Calling retry again on pending job -> ErrJobNotDead (409 Conflict)
		err = client.RetryDeadJob(ctx, jobID)
		require.Error(t, err)
		require.ErrorIs(t, err, jobq.ErrJobNotDead)
	})

	t.Run("TestDeadLetter_ListEndpoint", func(t *testing.T) {
		queue := "p4_list_" + uuid.NewString()[:8]

		for i := 0; i < 3; i++ {
			jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
				Queue:       queue,
				JobType:     "work",
				Payload:     json.RawMessage(`{}`),
				MaxAttempts: 1,
			})
			require.NoError(t, err)

			job, err := client.Claim(ctx, []string{queue}, "w1", 30)
			require.NoError(t, err)
			require.Equal(t, jobID, job.ID)

			err = client.NAck(ctx, job.ID, job.LeaseToken, fmt.Errorf("fatal error %d", i), true)
			require.NoError(t, err)
		}

		deadJobs, err := client.GetDeadLetteredJobs(ctx, queue)
		require.NoError(t, err)
		require.Len(t, deadJobs, 3)
		for _, j := range deadJobs {
			require.Equal(t, "dead", j.Status)
			require.Equal(t, queue, j.Queue)
		}
	})

	t.Run("TestJitter_SpreadRetries", func(t *testing.T) {
		queue := "p4_jitter_" + uuid.NewString()[:8]
		numJobs := 10
		jobIDs := make([]uuid.UUID, numJobs)

		for i := 0; i < numJobs; i++ {
			id, err := client.SubmitJob(ctx, jobq.SubmitRequest{
				Queue:       queue,
				JobType:     "jitter_test",
				Payload:     json.RawMessage(`{}`),
				MaxAttempts: 5,
			})
			require.NoError(t, err)
			jobIDs[i] = id

			j, err := client.Claim(ctx, []string{queue}, "w1", 30)
			require.NoError(t, err)

			err = client.NAck(ctx, j.ID, j.LeaseToken, fmt.Errorf("simulated fail"), false)
			require.NoError(t, err)
		}

		// Verify run_at values have jitter spread (not identical)
		uniqueTimes := make(map[int64]bool)
		for _, id := range jobIDs {
			job, err := client.GetJob(ctx, id)
			require.NoError(t, err)
			uniqueTimes[job.RunAt.UnixNano()] = true
		}
		// Full jitter guarantees spread across random intervals
		require.Greater(t, len(uniqueTimes), 1, "jitter must spread out run_at timestamps")
	})

	t.Run("TestBackoff_Cap", func(t *testing.T) {
		queue := "p4_cap_" + uuid.NewString()[:8]
		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:       queue,
			JobType:     "capped_flaky",
			Payload:     json.RawMessage(`{}`),
			MaxAttempts: 20,
		})
		require.NoError(t, err)

		// Simulate 15 attempts manually in DB
		_, err = ts.store.Pool().Exec(ctx, "UPDATE jobs SET attempts = 15 WHERE id = $1", jobID)
		require.NoError(t, err)

		// Fast forward run_at and claim
		_, err = ts.store.Pool().Exec(ctx, "UPDATE jobs SET run_at = now() WHERE id = $1", jobID)
		require.NoError(t, err)

		j, err := client.Claim(ctx, []string{queue}, "w1", 30)
		require.NoError(t, err)

		err = client.NAck(ctx, j.ID, j.LeaseToken, fmt.Errorf("high attempt failure"), false)
		require.NoError(t, err)

		dbJob, err := client.GetJob(ctx, jobID)
		require.NoError(t, err)
		// Delay should never exceed maxDelay (default 60s) + small epsilon
		maxDelayCap := 65 * time.Second
		require.True(t, dbJob.RunAt.Before(time.Now().Add(maxDelayCap)), "delay must be capped by max_delay")
	})
}

func BenchmarkRetryThroughput(b *testing.B) {
	ts := newTestServer(b)
	client := jobq.NewClient(ts.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queue := "bench_retry_" + uuid.NewString()[:8]
	var jobAttempts sync.Map
	var completedCount atomic.Int64

	// Real concurrent worker pool
	worker := jobq.NewWorker(client, jobq.WorkerConfig{
		Queues:       []string{queue},
		Concurrency:  8,
		PollInterval: 10 * time.Millisecond,
		Logger:       zerolog.Nop(),
	})

	// Fail on 1st attempt, succeed on 2nd
	worker.Handle("bench_flaky", func(ctx context.Context, p json.RawMessage) error {
		var payload struct{ ID string }
		_ = json.Unmarshal(p, &payload)

		val, _ := jobAttempts.LoadOrStore(payload.ID, new(atomic.Int32))
		counter := val.(*atomic.Int32)
		n := counter.Add(1)

		if n == 1 {
			return fmt.Errorf("transient failure")
		}
		completedCount.Add(1)
		return nil
	})

	go func() {
		_ = worker.Run(ctx)
	}()

	// Pre-submit b.N jobs
	for i := 0; i < b.N; i++ {
		jobID := uuid.NewString()
		_, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:       queue,
			JobType:     "bench_flaky",
			Payload:     json.RawMessage(fmt.Sprintf(`{"id":%q}`, jobID)),
			MaxAttempts: 5,
		})
		if err != nil {
			b.Fatalf("submit failed: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	// Wait until all b.N jobs succeed on retry
	for completedCount.Load() < int64(b.N) {
		time.Sleep(10 * time.Millisecond)
	}

	b.StopTimer()
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "retried-jobs/sec")
}
