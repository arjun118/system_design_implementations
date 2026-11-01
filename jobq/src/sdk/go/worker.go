package jobq

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type Worker struct {
	client   *Client
	handlers map[string]Handler
	config   WorkerConfig
}

type WorkerConfig struct {
	Queues       []string
	Concurrency  int
	PollInterval time.Duration
	Logger       zerolog.Logger
}

func NewWorker(client *Client, workerConfig WorkerConfig) *Worker {
	if workerConfig.Concurrency <= 0 {
		workerConfig.Concurrency = 1
	}
	if workerConfig.PollInterval <= 0 {
		workerConfig.PollInterval = 500 * time.Millisecond
	}

	return &Worker{
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

func (w *Worker) execute(ctx context.Context, job *Job) error {
	handler, ok := w.handlers[job.JobType]
	if !ok {
		//then permanent error - no handler registered
		err := w.client.NAck(ctx, job.ID, 1, fmt.Errorf("no handlers registered for job type: %s", job.JobType), true)
		if err != nil {
			return fmt.Errorf("nack failed for job: %s,%w", job.ID.String(), err)
		}
		return err
	}
	handlerError := handler(ctx, job.Payload)
	if handlerError != nil {
		err := w.client.NAck(ctx, job.ID, 1, handlerError, false)
		if err != nil {
			return fmt.Errorf("nack failed for job: %s,%w", job.ID.String(), err)
		}
		return handlerError
	}
	if err := w.client.Ack(ctx, job.ID); err != nil {
		return fmt.Errorf("ack failed for job: %s, %w", job.ID.String(), err)
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
		job, err := w.client.Claim(ctx, w.config.Queues)
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
		jobContext := context.Background()
		wg.Add(1)
		go func(job *Job) {
			defer wg.Done()
			// release slot
			defer func() { <-slots }()
			// this is server's responsibility and it should just let our server know
			// this failed so that it can calcualte offset ,jitter and reschedule the job if attemps are still there and the failure is transient
			err := w.execute(jobContext, job)
			if err != nil {
				w.config.Logger.Error().Err(err).
					Str("job", job.ID.String()).
					Str("status", "failed")
			}
		}(job)
	}

}
