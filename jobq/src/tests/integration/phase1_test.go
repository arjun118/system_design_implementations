//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"arjun118.github.io/jobq-go"
	"arjun118.github.io/jobq/internal/config"
	"arjun118.github.io/jobq/internal/httpapi"
	"arjun118.github.io/jobq/internal/storage"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

type testServer struct {
	*httptest.Server
	store *storage.Store
}

func newTestServer(t testing.TB) *testServer {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://jobq:durable@localhost:5433/jobq?sslmode=disable"
	}

	cfg := &config.Config{
		DB: config.DBConfig{
			DatabaseURL: dbURL,
			MaxOpenConn: 25,
			MaxIdleConn: 10,
			MaxIdleTime: "15m",
		},
		MinLeaseDurationSeconds: 5,
		MaxLeaseDurationSeconds: 600,
	}

	store, err := storage.NewStore(cfg)
	require.NoError(t, err, "failed to connect to test postgres")

	logger := zerolog.Nop()
	server := httpapi.NewServer(logger, store)

	ts := httptest.NewServer(server)
	t.Cleanup(func() {
		ts.Close()
		store.Close()
	})

	return &testServer{
		Server: ts,
		store:  store,
	}
}

func TestPhase1(t *testing.T) {
	ts := newTestServer(t)

	var createdID uuid.UUID

	t.Run("SubmitJob_Success", func(t *testing.T) {
		reqBody := []byte(`{
			"queue": "emails",
			"job_type": "send_welcome",
			"payload": {"to": "user@example.com"},
			"priority": 10,
			"max_attempts": 3
		}`)

		resp, err := http.Post(ts.URL+"/v1/jobs", "application/json", bytes.NewReader(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var body struct {
			ID uuid.UUID `json:"id"`
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, body.ID)
		createdID = body.ID

		// Verify persisted state in PostgreSQL
		job, err := ts.store.GetJob(context.Background(), body.ID)
		require.NoError(t, err)
		require.Equal(t, body.ID, job.ID)
		require.Equal(t, "emails", job.Queue)
		require.Equal(t, "send_welcome", job.JobType)
		require.Equal(t, "pending", job.Status)
		require.Equal(t, 10, job.Priority)
		require.Equal(t, 3, job.MaxAttempts)
		require.JSONEq(t, `{"to":"user@example.com"}`, string(job.Payload))
	})

	t.Run("GetJob_Success", func(t *testing.T) {
		require.NotEqual(t, uuid.Nil, createdID)

		resp, err := http.Get(ts.URL + "/v1/jobs/" + createdID.String())
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var job struct {
			ID          uuid.UUID       `json:"id"`
			Queue       string          `json:"queue"`
			JobType     string          `json:"job_type"`
			Payload     json.RawMessage `json:"payload"`
			Status      string          `json:"status"`
			Priority    int             `json:"priority"`
			Attempts    int             `json:"attempts"`
			MaxAttempts int             `json:"max_attempts"`
			RunAt       time.Time       `json:"run_at"`
			CreatedAt   time.Time       `json:"created_at"`
			UpdatedAt   time.Time       `json:"updated_at"`
		}
		err = json.NewDecoder(resp.Body).Decode(&job)
		require.NoError(t, err)
		require.Equal(t, createdID, job.ID)
		require.Equal(t, "emails", job.Queue)
	})

	t.Run("GetQueueStats_Success", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/v1/queues/emails/stats")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var stats struct {
			Queue        string `json:"queue"`
			Pending      int64  `json:"pending"`
			Running      int64  `json:"running"`
			Completed    int64  `json:"completed"`
			Failed       int64  `json:"failed"`
			DeadLettered int64  `json:"dead_lettered"`
			Total        int64  `json:"total"`
		}
		err = json.NewDecoder(resp.Body).Decode(&stats)
		require.NoError(t, err)
		require.Equal(t, "emails", stats.Queue)
		require.GreaterOrEqual(t, stats.Pending, int64(1))
		require.GreaterOrEqual(t, stats.Total, int64(1))
	})

	t.Run("GetJob_NotFound", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/v1/jobs/" + uuid.NewString())
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("GetJob_InvalidID", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/v1/jobs/not-a-valid-uuid")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("SubmitJob_InvalidPayload", func(t *testing.T) {
		// Missing job_type and payload
		reqBody := []byte(`{"queue": "emails"}`)
		resp, err := http.Post(ts.URL+"/v1/jobs", "application/json", bytes.NewReader(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func BenchmarkSubmitJob(b *testing.B) {
	ts := newTestServer(b)
	defer ts.Close()

	client := jobq.NewClient(ts.URL)
	ctx := context.Background()

	req := jobq.SubmitRequest{
		Queue:   "bench_queue",
		JobType: "noop",
		Payload: json.RawMessage(`{"benchmark": true}`),
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := client.SubmitJob(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}
