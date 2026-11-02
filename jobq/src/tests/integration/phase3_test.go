//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"arjun118.github.io/jobq-go"
	"arjun118.github.io/jobq/internal/leaser"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestPhase3(t *testing.T) {
	ts := newTestServer(t)
	client := jobq.NewClient(ts.URL)

	t.Run("TestClaim_SetsLease", func(t *testing.T) {
		ctx := context.Background()
		queueName := "p3_claim_" + uuid.NewString()[:8]

		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queueName,
			JobType: "test_job",
			Payload: json.RawMessage(`{"hello":"world"}`),
		})
		require.NoError(t, err)

		workerID := "worker-" + uuid.NewString()[:8]
		claimedJob, err := client.Claim(ctx, []string{queueName}, workerID, 30)
		require.NoError(t, err)
		require.Equal(t, jobID, claimedJob.ID)
		require.Equal(t, "running", claimedJob.Status)
		require.NotNil(t, claimedJob.LeaseOwner)
		require.Equal(t, workerID, *claimedJob.LeaseOwner)
		require.Equal(t, 1, claimedJob.LeaseToken)
		require.NotNil(t, claimedJob.LeaseUntil)
		require.True(t, claimedJob.LeaseUntil.After(time.Now()))

		dbJob, err := client.GetJob(ctx, jobID)
		require.NoError(t, err)
		require.NotNil(t, dbJob.LeaseOwner)
		require.Equal(t, workerID, *dbJob.LeaseOwner)
		require.Equal(t, 1, dbJob.LeaseToken)
		require.NotNil(t, dbJob.LeaseUntil)
		require.True(t, dbJob.LeaseUntil.After(time.Now()))
	})

	t.Run("TestHeartbeat_ExtendsLease", func(t *testing.T) {
		ctx := context.Background()
		queueName := "p3_hb_" + uuid.NewString()[:8]

		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queueName,
			JobType: "test_job",
			Payload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		workerID := "worker-" + uuid.NewString()[:8]
		claimedJob, err := client.Claim(ctx, []string{queueName}, workerID, 10)
		require.NoError(t, err)

		require.NotNil(t, claimedJob.LeaseUntil)
		originalLeaseUntil := *claimedJob.LeaseUntil
		time.Sleep(100 * time.Millisecond)

		err = client.Heartbeat(ctx, jobID, claimedJob.LeaseToken, workerID, 30)
		require.NoError(t, err)

		dbJob, err := client.GetJob(ctx, jobID)
		require.NoError(t, err)
		require.NotNil(t, dbJob.LeaseUntil)
		require.True(t, dbJob.LeaseUntil.After(originalLeaseUntil), "lease_until must advance into future")
	})

	t.Run("TestHeartbeat_WrongToken", func(t *testing.T) {
		ctx := context.Background()
		queueName := "p3_wrong_hb_" + uuid.NewString()[:8]

		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queueName,
			JobType: "test_job",
			Payload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		workerID := "worker-" + uuid.NewString()[:8]
		_, err = client.Claim(ctx, []string{queueName}, workerID, 15)
		require.NoError(t, err)

		err = client.Heartbeat(ctx, jobID, 9999, workerID, 30)
		require.Error(t, err)
		require.ErrorIs(t, err, jobq.ErrStaleLeaseToken)
	})

	t.Run("TestReaper_RecoverExpiredLease", func(t *testing.T) {
		ctx := context.Background()
		queueName := "p3_reaper_" + uuid.NewString()[:8]

		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queueName,
			JobType: "test_job",
			Payload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		workerID := "worker-" + uuid.NewString()[:8]
		claimedJob, err := client.Claim(ctx, []string{queueName}, workerID, 5)
		require.NoError(t, err)
		require.Equal(t, "running", claimedJob.Status)

		// Expire lease in database
		_, err = ts.store.Pool().Exec(ctx, "UPDATE jobs SET lease_until = now() - INTERVAL '1 second' WHERE id = $1", jobID)
		require.NoError(t, err)

		// Run reaper
		reaped, err := ts.store.ReapExpiredLeases(ctx)
		require.NoError(t, err)
		require.GreaterOrEqual(t, reaped, int64(1))

		// Check job is pending again
		dbJob, err := client.GetJob(ctx, jobID)
		require.NoError(t, err)
		require.Equal(t, "pending", dbJob.Status)
		require.Nil(t, dbJob.LeaseOwner)
		require.Nil(t, dbJob.LeaseUntil)
	})

	t.Run("TestFencingToken_RejectsStaleAck", func(t *testing.T) {
		ctx := context.Background()
		queueName := "p3_fencing_ack_" + uuid.NewString()[:8]

		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queueName,
			JobType: "test_job",
			Payload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		// Worker 1 claims job (lease_token = 1)
		w1 := "worker-1"
		job1, err := client.Claim(ctx, []string{queueName}, w1, 5)
		require.NoError(t, err)
		require.Equal(t, 1, job1.LeaseToken)

		// Expire lease and reclaim by Worker 2 (lease_token = 2)
		_, err = ts.store.Pool().Exec(ctx, "UPDATE jobs SET lease_until = now() - INTERVAL '1 second' WHERE id = $1", jobID)
		require.NoError(t, err)

		_, err = ts.store.ReapExpiredLeases(ctx)
		require.NoError(t, err)

		w2 := "worker-2"
		job2, err := client.Claim(ctx, []string{queueName}, w2, 30)
		require.NoError(t, err)
		require.Equal(t, 2, job2.LeaseToken)

		// Stale Worker 1 acks with token 1 -> 409 Conflict
		err = client.Ack(ctx, jobID, job1.LeaseToken)
		require.Error(t, err)
		require.ErrorIs(t, err, jobq.ErrStaleLeaseToken)

		// Active Worker 2 acks with token 2 -> 204 Success
		err = client.Ack(ctx, jobID, job2.LeaseToken)
		require.NoError(t, err)

		dbJob, err := client.GetJob(ctx, jobID)
		require.NoError(t, err)
		require.Equal(t, "completed", dbJob.Status)
	})

	t.Run("TestFencingToken_RejectsStaleNAck", func(t *testing.T) {
		ctx := context.Background()
		queueName := "p3_fencing_nack_" + uuid.NewString()[:8]

		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queueName,
			JobType: "test_job",
			Payload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		// Worker 1 claims (token 1)
		job1, err := client.Claim(ctx, []string{queueName}, "worker-1", 5)
		require.NoError(t, err)
		require.Equal(t, 1, job1.LeaseToken)

		// Expire lease and reclaim by Worker 2 (token 2)
		_, err = ts.store.Pool().Exec(ctx, "UPDATE jobs SET lease_until = now() - INTERVAL '1 second' WHERE id = $1", jobID)
		require.NoError(t, err)
		_, err = ts.store.ReapExpiredLeases(ctx)
		require.NoError(t, err)

		job2, err := client.Claim(ctx, []string{queueName}, "worker-2", 30)
		require.NoError(t, err)
		require.Equal(t, 2, job2.LeaseToken)

		// Stale Worker 1 nacks with token 1 -> 409 Conflict
		err = client.NAck(ctx, jobID, job1.LeaseToken, fmt.Errorf("late crash"), false)
		require.Error(t, err)
		require.ErrorIs(t, err, jobq.ErrStaleLeaseToken)
	})

	t.Run("TestLeaseRenewal_LongJob", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		queueName := "p3_longjob_" + uuid.NewString()[:8]

		// Background reaper
		reaper := leaser.NewReaper(ts.store, 200*time.Millisecond, zerolog.Nop())
		go reaper.Start(ctx)

		// Worker with 5s lease and 1s heartbeat
		worker := jobq.NewWorker(client, jobq.WorkerConfig{
			Queues:            []string{queueName},
			Concurrency:       1,
			PollInterval:      50 * time.Millisecond,
			LeaseDuration:     5 * time.Second,
			HeartbeatInterval: 1 * time.Second,
			Logger:            zerolog.Nop(),
		})

		worker.Handle("long_task", func(ctx context.Context, payload json.RawMessage) error {
			time.Sleep(3 * time.Second)
			return nil
		})

		go func() {
			_ = worker.Run(ctx)
		}()

		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queueName,
			JobType: "long_task",
			Payload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			job, err := client.GetJob(ctx, jobID)
			return err == nil && job.Status == "completed"
		}, 8*time.Second, 200*time.Millisecond)
	})
}

func BenchmarkHeartbeat(b *testing.B) {
	ts := newTestServer(b)
	client := jobq.NewClient(ts.URL)
	ctx := context.Background()

	queueName := "bench_hb_" + uuid.NewString()[:8]
	jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
		Queue:   queueName,
		JobType: "bench_hb",
		Payload: json.RawMessage(`{}`),
	})
	require.NoError(b, err)

	workerID := "worker-bench"
	job, err := client.Claim(ctx, []string{queueName}, workerID, 600)
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := client.Heartbeat(ctx, jobID, job.LeaseToken, workerID, 600)
		if err != nil {
			b.Fatalf("heartbeat failed: %v", err)
		}
	}
}
