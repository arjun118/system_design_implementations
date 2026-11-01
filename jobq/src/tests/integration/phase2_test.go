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

func TestPhase2(t *testing.T) {
	ts := newTestServer(t)
	client := jobq.NewClient(ts.URL)

	t.Run("WorkerExecutesJob_Success", func(t *testing.T) {
		queueName := "phase2_exec_" + uuid.NewString()[:8]
		var executed atomic.Bool

		worker := jobq.NewWorker(client, jobq.WorkerConfig{
			Queues:       []string{queueName},
			Concurrency:  1,
			PollInterval: 50 * time.Millisecond,
			Logger:       zerolog.Nop(),
		})

		worker.Handle("greet", func(ctx context.Context, payload json.RawMessage) error {
			executed.Store(true)
			return nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			_ = worker.Run(ctx)
		}()

		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queueName,
			JobType: "greet",
			Payload: json.RawMessage(`{"name":"alice"}`),
		})
		require.NoError(t, err)

		// Await execution
		require.Eventually(t, func() bool {
			return executed.Load()
		}, 3*time.Second, 50*time.Millisecond)

		// Verify job reached completed in DB
		require.Eventually(t, func() bool {
			job, err := client.GetJob(ctx, jobID)
			return err == nil && job.Status == "completed"
		}, 3*time.Second, 50*time.Millisecond)
	})

	t.Run("WorkerHandlerError_MarksFailed", func(t *testing.T) {
		queueName := "phase2_fail_" + uuid.NewString()[:8]

		worker := jobq.NewWorker(client, jobq.WorkerConfig{
			Queues:       []string{queueName},
			Concurrency:  1,
			PollInterval: 50 * time.Millisecond,
			Logger:       zerolog.Nop(),
		})

		worker.Handle("broken", func(ctx context.Context, payload json.RawMessage) error {
			return fmt.Errorf("simulated handler crash")
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			_ = worker.Run(ctx)
		}()

		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queueName,
			JobType: "broken",
			Payload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		// Verify status becomes failed in DB
		require.Eventually(t, func() bool {
			job, err := client.GetJob(ctx, jobID)
			return err == nil && job.Status == "failed"
		}, 3*time.Second, 50*time.Millisecond)
	})

	t.Run("UnknownJobType_NacksGracefully", func(t *testing.T) {
		queueName := "phase2_unknown_" + uuid.NewString()[:8]

		worker := jobq.NewWorker(client, jobq.WorkerConfig{
			Queues:       []string{queueName},
			Concurrency:  1,
			PollInterval: 50 * time.Millisecond,
			Logger:       zerolog.Nop(),
		})
		// Note: no handler registered for "missing_handler"

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			_ = worker.Run(ctx)
		}()

		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queueName,
			JobType: "missing_handler",
			Payload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			job, err := client.GetJob(ctx, jobID)
			return err == nil && job.Status == "failed"
		}, 3*time.Second, 50*time.Millisecond)
	})

	t.Run("ConcurrentWorkers_NoDuplicateClaim", func(t *testing.T) {
		queueName := "phase2_nodup_" + uuid.NewString()[:8]
		var executionCount atomic.Int32

		handler := func(ctx context.Context, payload json.RawMessage) error {
			executionCount.Add(1)
			time.Sleep(50 * time.Millisecond)
			return nil
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start 5 separate worker loops
		for i := 0; i < 5; i++ {
			w := jobq.NewWorker(client, jobq.WorkerConfig{
				Queues:       []string{queueName},
				Concurrency:  1,
				PollInterval: 20 * time.Millisecond,
				Logger:       zerolog.Nop(),
			})
			w.Handle("unique_job", handler)
			go func() {
				_ = w.Run(ctx)
			}()
		}

		// Submit exactly 1 job
		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queueName,
			JobType: "unique_job",
			Payload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		// Wait until completed
		require.Eventually(t, func() bool {
			job, err := client.GetJob(ctx, jobID)
			return err == nil && job.Status == "completed"
		}, 3*time.Second, 50*time.Millisecond)

		// Give extra margin to ensure no other worker duplicates it
		time.Sleep(200 * time.Millisecond)
		require.Equal(t, int32(1), executionCount.Load(), "job must be executed exactly once")
	})

	t.Run("WorkerPool_ConcurrencyLimit", func(t *testing.T) {
		queueName := "phase2_concurrency_" + uuid.NewString()[:8]
		concurrencyLimit := 3
		var activeGoroutines atomic.Int32
		var maxObserved atomic.Int32

		worker := jobq.NewWorker(client, jobq.WorkerConfig{
			Queues:       []string{queueName},
			Concurrency:  concurrencyLimit,
			PollInterval: 20 * time.Millisecond,
			Logger:       zerolog.Nop(),
		})

		worker.Handle("slow_task", func(ctx context.Context, payload json.RawMessage) error {
			curr := activeGoroutines.Add(1)
			for {
				oldMax := maxObserved.Load()
				if curr <= oldMax || maxObserved.CompareAndSwap(oldMax, curr) {
					break
				}
			}
			time.Sleep(100 * time.Millisecond)
			activeGoroutines.Add(-1)
			return nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			_ = worker.Run(ctx)
		}()

		// Submit 10 jobs
		for i := 0; i < 10; i++ {
			_, err := client.SubmitJob(ctx, jobq.SubmitRequest{
				Queue:   queueName,
				JobType: "slow_task",
				Payload: json.RawMessage(`{}`),
			})
			require.NoError(t, err)
		}

		// Wait for all 10 jobs to complete
		require.Eventually(t, func() bool {
			stats, err := client.GetQueueStats(ctx, queueName)
			return err == nil && stats.Completed == 10
		}, 5*time.Second, 100*time.Millisecond)

		require.LessOrEqual(t, int(maxObserved.Load()), concurrencyLimit,
			"active concurrent handlers must never exceed configured limit")
	})

	t.Run("Worker_GracefulShutdown", func(t *testing.T) {
		queueName := "phase2_shutdown_" + uuid.NewString()[:8]
		var jobStarted atomic.Bool
		var jobCompleted atomic.Bool

		worker := jobq.NewWorker(client, jobq.WorkerConfig{
			Queues:       []string{queueName},
			Concurrency:  1,
			PollInterval: 20 * time.Millisecond,
			Logger:       zerolog.Nop(),
		})

		worker.Handle("sleepy", func(ctx context.Context, payload json.RawMessage) error {
			jobStarted.Store(true)
			time.Sleep(200 * time.Millisecond)
			jobCompleted.Store(true)
			return nil
		})

		ctx, cancel := context.WithCancel(context.Background())

		workerDone := make(chan error, 1)
		go func() {
			workerDone <- worker.Run(ctx)
		}()

		jobID, err := client.SubmitJob(context.Background(), jobq.SubmitRequest{
			Queue:   queueName,
			JobType: "sleepy",
			Payload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		// Wait until handler starts executing
		require.Eventually(t, func() bool {
			return jobStarted.Load()
		}, 2*time.Second, 20*time.Millisecond)

		// Signal shutdown while job is running
		cancel()

		// Worker must finish in-flight job and exit
		select {
		case err := <-workerDone:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(3 * time.Second):
			t.Fatal("worker did not exit cleanly within timeout")
		}

		require.True(t, jobCompleted.Load(), "in-flight job must finish before worker returns")

		job, err := client.GetJob(context.Background(), jobID)
		require.NoError(t, err)
		require.Equal(t, "completed", job.Status)
	})
}

func BenchmarkWorkerThroughput(b *testing.B) {
	ts := newTestServer(b)
	client := jobq.NewClient(ts.URL)
	ctx := context.Background()
	queueName := "bench_worker_queue"

	// Pre-insert b.N jobs
	req := jobq.SubmitRequest{
		Queue:   queueName,
		JobType: "noop",
		Payload: json.RawMessage(`{}`),
	}
	for i := 0; i < b.N; i++ {
		_, err := client.SubmitJob(ctx, req)
		if err != nil {
			b.Fatalf("failed to submit benchmark job: %v", err)
		}
	}

	worker := jobq.NewWorker(client, jobq.WorkerConfig{
		Queues:       []string{queueName},
		Concurrency:  8,
		PollInterval: 5 * time.Millisecond,
		Logger:       zerolog.Nop(),
	})
	worker.Handle("noop", func(ctx context.Context, payload json.RawMessage) error {
		return nil
	})

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = worker.Run(workerCtx)
	}()

	b.ResetTimer()
	b.ReportAllocs()

	// Wait until all b.N jobs are completed
	for {
		stats, err := client.GetQueueStats(ctx, queueName)
		if err == nil && stats.Completed >= int64(b.N) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	b.StopTimer()
	cancel()
	wg.Wait()
}
