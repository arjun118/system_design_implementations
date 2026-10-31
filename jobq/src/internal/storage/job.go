package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Job struct {
	ID          uuid.UUID       `json:"id"           db:"id"`
	Queue       string          `json:"queue"        db:"queue"`
	JobType     string          `json:"job_type"     db:"job_type"`
	Payload     json.RawMessage `json:"payload"      db:"payload"`
	Status      string          `json:"status"       db:"status"`
	Priority    int             `json:"priority"     db:"priority"`
	Attempts    int             `json:"attempts"     db:"attempts"`
	MaxAttempts int             `json:"max_attempts" db:"max_attempts"`
	RunAt       time.Time       `json:"run_at"       db:"run_at"`
	CreatedAt   time.Time       `json:"created_at"   db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"   db:"updated_at"`
}

func (s *Store) CreateJob(ctx context.Context, jobDetails *Job) error {
	if jobDetails.Queue == "" {
		jobDetails.Queue = "default"
	}
	if jobDetails.MaxAttempts <= 0 {
		jobDetails.MaxAttempts = 5
	}
	query := `
	INSERT into jobs (queue,job_type, payload,priority, max_attempts)
	values ($1,$2,$3,$4,$5)
	returning id;
	`
	args := []any{jobDetails.Queue, jobDetails.JobType, jobDetails.Payload, jobDetails.Priority, jobDetails.MaxAttempts}
	err := s.pool.QueryRow(ctx, query, args...).Scan(&jobDetails.ID)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}
	return nil
}

func (s *Store) CancelJob(jobId string) error {
	//
	panic("random")
}

func (s *Store) GetJob(ctx context.Context, jobID uuid.UUID) (*Job, error) {
	query := `
       SELECT id, queue, job_type, payload, status, priority, attempts, max_attempts, run_at, created_at, updated_at
	   FROM jobs
	   WHERE id = $1
      `
	rows, err := s.pool.Query(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	job, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Job])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("failed to scan job: %w", err)
	}
	return job, nil
}
