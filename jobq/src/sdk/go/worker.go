package jobq

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type Worker struct {
	workerID string
	client   *Client
	handlers map[string]Handler
	config   WorkerConfig
}

type WorkerConfig struct {
	WorkerID          string
	Queues            []string
	Concurrency       int
	PollInterval      time.Duration
	LeaseDuration     time.Duration //how long server hold the lease for us
	HeartbeatInterval time.Duration //how frequently we send heartbeats to extend lease
	Logger            zerolog.Logger
	JobTimeout        time.Duration
}

func defaultWorkerID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "worker"
	}
	return fmt.Sprintf("%s-%s", hostname, uuid.NewString()[:8])
}

func (w *Worker) ID() string {
	return w.workerID
}

func NewWorker(client *Client, workerConfig WorkerConfig) *Worker {
	if workerConfig.Concurrency <= 0 {
		workerConfig.Concurrency = 1
	}
	if workerConfig.PollInterval <= 0 {
		workerConfig.PollInterval = 500 * time.Millisecond
	}
	if workerConfig.LeaseDuration <= 0 {
		workerConfig.LeaseDuration = 30 * time.Second
	}
	if workerConfig.HeartbeatInterval <= 0 {
		workerConfig.HeartbeatInterval = workerConfig.LeaseDuration / 3
	}
	if workerConfig.JobTimeout <= 0 {
		workerConfig.JobTimeout = 10 * time.Minute
	}
	workerID := workerConfig.WorkerID
	if workerID == "" {
		workerID = defaultWorkerID()
	}
	workerConfig.WorkerID = workerID
	return &Worker{
		workerID: workerID,
		client:   client,
		handlers: make(map[string]Handler), // initialized internally
		config:   workerConfig,
	}
}

func (w *Worker) Handle(jobType string, h Handler) {
	w.handlers[jobType] = h
}

func (w *Worker) wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func (w *Worker) startHeartbeatLoop(ctx context.Context, cancelJobContext context.CancelFunc, job *Job) {
	ticker := time.NewTicker(w.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := w.client.Heartbeat(ctx, job.ID, job.LeaseToken, w.ID(), int(w.config.LeaseDuration.Seconds()))
			if err != nil {
				switch {
				case errors.Is(err, ErrStaleLeaseToken) || errors.Is(err, ErrJobNotFound):
					// handler, it's done bro
					// we cancel the job context so our heartbeat context
					// is also cancelled
					cancelJobContext()
					return
				default:
					w.config.Logger.Warn().
						Err(err).
						Str("job_id", job.ID.String()).
						Msg("heartbeat failed")
				}
			}
		}
	}
}

func (w *Worker) execute(ctx context.Context, job *Job) error {
	handler, ok := w.handlers[job.JobType]
	if !ok {
		//then permanent error - no handler registered
		err := w.client.NAck(ctx, job.ID, job.LeaseToken, fmt.Errorf("no handlers registered for job type: %s", job.JobType), true)
		if err != nil {
			return fmt.Errorf("nack failed for job: %s,%w", job.ID, err)
		}
		return nil
	}
	// handler context
	handlerCtx, cancelHandler := context.WithTimeout(ctx, w.config.JobTimeout)
	defer cancelHandler()
	// start heartbeat [extend lease loop] for this job
	go w.startHeartbeatLoop(handlerCtx, cancelHandler, job)

	handlerError := handler(handlerCtx, job.Payload)
	if handlerError != nil {
		err := w.client.NAck(ctx, job.ID, job.LeaseToken, handlerError, false)
		if err != nil {
			return fmt.Errorf("nack failed for job: %s,%w", job.ID, err)
		}
		return handlerError
	}
	if err := w.client.Ack(ctx, job.ID, job.LeaseToken); err != nil {
		return fmt.Errorf("ack failed for job: %s, %w", job.ID, err)
	}
	return nil

}

func (w *Worker) Run(ctx context.Context) error {
	slots := make(chan struct{}, w.config.Concurrency)
	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		select {
		// claim a slot
		case slots <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		// when there is a slot ,claim a job
		job, err := w.client.Claim(ctx, w.config.Queues, w.ID(), int(w.config.LeaseDuration.Seconds()))
		if err != nil {
			<-slots
			if errors.Is(err, ErrNoJobAvailable) {
				if !w.wait(ctx, w.config.PollInterval) {
					return ctx.Err()
				}
				continue
			}
			// if the error is something else
			// decide if the whether the error is transient
			// claim errors, retry ,backoff or stop run
			// this is workers responsibility - it should wait for sometime and retry claiming jobs in order to not put pressure on our queue
			return err
		}
		wg.Add(1)
		go func(job *Job) {
			defer wg.Done()
			// release slot
			defer func() { <-slots }()
			// this is server's responsibility and it should just let our server know
			// this failed so that it can calcualte offset ,jitter and reschedule the job if attemps are still there and the failure is transient
			err := w.execute(ctx, job)
			if err != nil {
				w.config.Logger.Error().Err(err).
					Str("job", job.ID.String()).
					Str("status", "failed")
			}
		}(job)
	}

}
