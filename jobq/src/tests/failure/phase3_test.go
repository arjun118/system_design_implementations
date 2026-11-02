//go:build failure

package failure_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"arjun118.github.io/jobq-go"
	"arjun118.github.io/jobq/internal/config"
	"arjun118.github.io/jobq/internal/httpapi"
	"arjun118.github.io/jobq/internal/leaser"
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

	os.Setenv("MIN_LEASE_DURATION", "1")
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://jobq:durable@localhost:5433/jobq?sslmode=disable"
	}

	cfg := config.Load()
	cfg.DB.DatabaseURL = dbURL
	cfg.MinLeaseDurationSeconds = 1

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

// TestHelperWorkerProcess is executed when invoked as a subprocess
func TestHelperWorkerProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_WORKER") != "1" {
		return
	}

	serverURL := os.Getenv("WORKER_SERVER_URL")
	queue := os.Getenv("WORKER_QUEUE")
	action := os.Getenv("WORKER_ACTION")
	markerFile := os.Getenv("WORKER_MARKER_FILE")
	leaseSec, _ := strconv.Atoi(os.Getenv("WORKER_LEASE_SEC"))
	if leaseSec <= 0 {
		leaseSec = 2
	}

	client := jobq.NewClient(serverURL)
	worker := jobq.NewWorker(client, jobq.WorkerConfig{
		Queues:            []string{queue},
		Concurrency:       1,
		PollInterval:      50 * time.Millisecond,
		LeaseDuration:     time.Duration(leaseSec) * time.Second,
		HeartbeatInterval: time.Duration(leaseSec) * time.Second / 2,
		Logger:            zerolog.Nop(),
	})

	worker.Handle("work", func(ctx context.Context, payload json.RawMessage) error {
		switch action {
		case "sleep":
			time.Sleep(30 * time.Second)
		case "marker":
			if markerFile != "" {
				f, err := os.OpenFile(markerFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err == nil {
					_, _ = f.WriteString("executed\n")
					_ = f.Sync()
					_ = f.Close()
				}
			}
			time.Sleep(10 * time.Second)
		case "quick":
			time.Sleep(10 * time.Millisecond)
		}
		return nil
	})

	_ = worker.Run(context.Background())
	os.Exit(0)
}

func startWorkerProcess(t *testing.T, serverURL, queue, action, markerFile string, leaseSec int) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperWorkerProcess$")
	cmd.Env = append(os.Environ(),
		"GO_WANT_HELPER_WORKER=1",
		fmt.Sprintf("WORKER_SERVER_URL=%s", serverURL),
		fmt.Sprintf("WORKER_QUEUE=%s", queue),
		fmt.Sprintf("WORKER_ACTION=%s", action),
		fmt.Sprintf("WORKER_MARKER_FILE=%s", markerFile),
		fmt.Sprintf("WORKER_LEASE_SEC=%d", leaseSec),
	)
	err := cmd.Start()
	require.NoError(t, err)
	return cmd
}

func waitForStatus(t *testing.T, client *jobq.Client, jobID uuid.UUID, expectedStatus string, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		job, err := client.GetJob(context.Background(), jobID)
		return err == nil && job.Status == expectedStatus
	}, timeout, 50*time.Millisecond)
}

func TestPhase3_FailureRecovery(t *testing.T) {
	ts := newTestServer(t)
	client := jobq.NewClient(ts.URL)

	// Background reaper running continuously
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reaper := leaser.NewReaper(ts.store, 100*time.Millisecond, zerolog.Nop())
	go reaper.Start(ctx)

	t.Run("TestCrash_AfterClaim", func(t *testing.T) {
		queue := "crash_claim_" + uuid.NewString()[:8]
		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queue,
			JobType: "work",
			Payload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		// Start Worker 1 (sleep action)
		cmd1 := startWorkerProcess(t, ts.URL, queue, "sleep", "", 1)
		defer func() {
			if cmd1.Process != nil {
				_ = cmd1.Process.Kill()
			}
		}()

		// Wait until job is claimed and running
		waitForStatus(t, client, jobID, "running", 5*time.Second)

		// SIGKILL Worker 1
		err = cmd1.Process.Signal(syscall.SIGKILL)
		require.NoError(t, err)
		_ = cmd1.Wait()

		// Start Worker 2 (quick action)
		cmd2 := startWorkerProcess(t, ts.URL, queue, "quick", "", 1)
		defer func() {
			if cmd2.Process != nil {
				_ = cmd2.Process.Kill()
			}
		}()

		// Job should be reaped and completed by Worker 2
		waitForStatus(t, client, jobID, "completed", 6*time.Second)
	})

	t.Run("TestCrash_DuringExecution", func(t *testing.T) {
		queue := "crash_exec_" + uuid.NewString()[:8]
		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queue,
			JobType: "work",
			Payload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		cmd1 := startWorkerProcess(t, ts.URL, queue, "sleep", "", 1)
		defer func() {
			if cmd1.Process != nil {
				_ = cmd1.Process.Kill()
			}
		}()

		waitForStatus(t, client, jobID, "running", 5*time.Second)
		time.Sleep(300 * time.Millisecond)

		// Kill worker mid-execution
		_ = cmd1.Process.Signal(syscall.SIGKILL)
		_ = cmd1.Wait()

		// Start Worker 2 to recover
		cmd2 := startWorkerProcess(t, ts.URL, queue, "quick", "", 1)
		defer func() {
			if cmd2.Process != nil {
				_ = cmd2.Process.Kill()
			}
		}()

		waitForStatus(t, client, jobID, "completed", 6*time.Second)
	})

	t.Run("TestCrash_AfterSideEffect", func(t *testing.T) {
		queue := "crash_side_" + uuid.NewString()[:8]
		markerFile := filepath.Join(t.TempDir(), "marker.txt")

		jobID, err := client.SubmitJob(ctx, jobq.SubmitRequest{
			Queue:   queue,
			JobType: "work",
			Payload: json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		// Worker 1 writes marker then sleeps before ack
		cmd1 := startWorkerProcess(t, ts.URL, queue, "marker", markerFile, 1)
		defer func() {
			if cmd1.Process != nil {
				_ = cmd1.Process.Kill()
			}
		}()

		// Wait for side effect to occur
		require.Eventually(t, func() bool {
			data, err := os.ReadFile(markerFile)
			return err == nil && strings.Count(string(data), "executed") == 1
		}, 5*time.Second, 50*time.Millisecond)

		// Kill Worker 1 before it acks
		_ = cmd1.Process.Signal(syscall.SIGKILL)
		_ = cmd1.Wait()

		// Worker 2 recovers job and executes it
		cmd2 := startWorkerProcess(t, ts.URL, queue, "quick", "", 1)
		defer func() {
			if cmd2.Process != nil {
				_ = cmd2.Process.Kill()
			}
		}()

		waitForStatus(t, client, jobID, "completed", 6*time.Second)
	})

	t.Run("TestMultiProcess_NoDuplicates", func(t *testing.T) {
		queue := "multiproc_" + uuid.NewString()[:8]
		totalJobs := 20

		for i := 0; i < totalJobs; i++ {
			_, err := client.SubmitJob(ctx, jobq.SubmitRequest{
				Queue:   queue,
				JobType: "work",
				Payload: json.RawMessage(`{}`),
			})
			require.NoError(t, err)
		}

		// Start 2 concurrent worker processes
		cmd1 := startWorkerProcess(t, ts.URL, queue, "quick", "", 2)
		cmd2 := startWorkerProcess(t, ts.URL, queue, "quick", "", 2)
		defer func() {
			if cmd1.Process != nil {
				_ = cmd1.Process.Kill()
			}
			if cmd2.Process != nil {
				_ = cmd2.Process.Kill()
			}
		}()

		// Wait until all 20 jobs complete
		require.Eventually(t, func() bool {
			stats, err := client.GetQueueStats(ctx, queue)
			return err == nil && stats.Completed == int64(totalJobs)
		}, 10*time.Second, 100*time.Millisecond)
	})
}
