//go:build integration

package sdk_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"arjun118.github.io/jobq-go"
	"arjun118.github.io/jobq/internal/config"
	"arjun118.github.io/jobq/internal/httpapi"
	"arjun118.github.io/jobq/internal/storage"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func newSDKTestServer(t *testing.T) (*jobq.Client, *httptest.Server) {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://jobq:durable@localhost:5433/jobq?sslmode=disable"
	}

	cfg := &config.Config{
		DB: config.DBConfig{
			DatabaseURL: dbURL,
			MaxOpenConn: 10,
			MaxIdleConn: 5,
			MaxIdleTime: "15m",
		},
		MinLeaseDurationSeconds: 5,
		MaxLeaseDurationSeconds: 600,
	}

	store, err := storage.NewStore(cfg)
	require.NoError(t, err, "failed to connect to test postgres")

	server := httpapi.NewServer(zerolog.Nop(), store)
	ts := httptest.NewServer(server)
	t.Cleanup(func() {
		ts.Close()
	})

	client := jobq.NewClient(ts.URL)
	return client, ts
}

func TestSDK_Phase1(t *testing.T) {
	client, _ := newSDKTestServer(t)
	ctx := context.Background()

	t.Run("SubmitJob_Success", func(t *testing.T) {
		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:       "sdk_test_queue",
			JobType:     "send_email",
			Payload:     json.RawMessage(`{"recipient": "bob@example.com"}`),
			Priority:    5,
			MaxAttempts: 4,
		})

		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, jobID)

		// Verify retrieval via SDK
		job, err := client.GetJob(ctx, jobID)
		require.NoError(t, err)
		require.Equal(t, jobID, job.ID)
		require.Equal(t, "sdk_test_queue", job.Queue)
		require.Equal(t, "send_email", job.JobType)
		require.Equal(t, "pending", job.Status)
		require.Equal(t, 5, job.Priority)
		require.Equal(t, 4, job.MaxAttempts)
		require.JSONEq(t, `{"recipient": "bob@example.com"}`, string(job.Payload))
	})

	t.Run("GetQueueStats", func(t *testing.T) {
		queueName := "sdk_stats_queue"

		// Submit 2 jobs
		_, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queueName,
			JobType: "task_1",
			Payload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		_, err = client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queueName,
			JobType: "task_2",
			Payload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		// Check stats via SDK
		stats, err := client.GetQueueStats(ctx, queueName)
		require.NoError(t, err)
		require.Equal(t, queueName, stats.Queue)
		require.GreaterOrEqual(t, stats.Pending, int64(2))
		require.GreaterOrEqual(t, stats.Total, int64(2))
	})

	t.Run("GetJob_NotFound", func(t *testing.T) {
		randomID := uuid.New()
		_, err := client.GetJob(ctx, randomID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "404")
	})
}
