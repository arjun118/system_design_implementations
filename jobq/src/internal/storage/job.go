package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"arjun118.github.io/jobq/internal/scheduler"
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
	LeaseOwner  *string         `json:"lease_owner" db:"lease_owner"`
	LeaseUntil  *time.Time      `json:"lease_until,omitempty" db:"lease_until"`
	LeaseToken  int             `json:"lease_token,omitempty" db:"lease_token"`
	LastError   *string         `json:"last_error,omitempty" db:"last_error"`
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
	runAt := time.Now()
	if !jobDetails.RunAt.IsZero() {
		runAt = jobDetails.RunAt
	}
	query := `
	INSERT into jobs (queue, job_type, payload, priority, max_attempts, run_at)
	values ($1, $2, $3, $4, $5, $6)
	returning id;
	`
	args := []any{jobDetails.Queue, jobDetails.JobType, jobDetails.Payload, jobDetails.Priority, jobDetails.MaxAttempts, runAt}
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
       SELECT id, queue, job_type, payload, status, priority, attempts, max_attempts,
              lease_owner, lease_until, lease_token, last_error,
              run_at, created_at, updated_at
	   FROM jobs
	   WHERE id = $1;
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

func (s *Store) MarkComplete(ctx context.Context, jobID uuid.UUID, leaseToken int) error {
	query := `
	update jobs
		set status='completed',
		lease_until = NULL,
		updated_at = now()
	where id=$1 and status = 'running' and lease_token=$2;
	`
	args := []any{jobID, leaseToken}
	result, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to mark job complete, %w", err)
	}
	if result.RowsAffected() == 0 {
		var currentToken int
		var status string
		err := s.pool.QueryRow(ctx, "SELECT lease_token, status FROM jobs WHERE id = $1", jobID).
			Scan(&currentToken, &status)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrJobNotFound
			}
			return err
		}
		// s.logger.Warn().
		// 	Str("job_id", jobID.String()).
		// 	Int("received_token", leaseToken).
		// 	Int("current_token", currentToken).
		// 	Str("current_status", status).
		// 	Msg("rejected stale acknowledgment")
		return ErrStaleLeaseToken
	}

	return nil
}

func (s *Store) MarkFailure(ctx context.Context, jobID uuid.UUID, leaseToken int, errMsg string, permanent bool) error {
	// 1. Authoritative check: fetch current attempts and verify lease_token + status
	var attempts, maxAttempts int
	checkQuery := `
		SELECT attempts, max_attempts
		FROM jobs
		WHERE id = $1 AND lease_token = $2 AND status = 'running';
	`
	err := s.pool.QueryRow(ctx, checkQuery, jobID, leaseToken).Scan(&attempts, &maxAttempts)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var currentToken int
			var status string
			err := s.pool.QueryRow(ctx, "SELECT lease_token, status FROM jobs WHERE id = $1", jobID).
				Scan(&currentToken, &status)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrJobNotFound
				}
				return err
			}
			return ErrStaleLeaseToken
		}
		return fmt.Errorf("failed to query job attempts: %w", err)
	}

	// 2. Decide next state in Go
	if permanent || attempts >= maxAttempts {
		updateQuery := `
			UPDATE jobs
			SET status = 'dead',
			    last_error = $2,
			    lease_owner = NULL,
			    lease_until = NULL,
			    updated_at = now()
			WHERE id = $1 AND lease_token = $3 AND status = 'running';
		`
		result, err := s.pool.Exec(ctx, updateQuery, jobID, errMsg, leaseToken)
		if err != nil {
			return fmt.Errorf("failed to mark job dead: %w", err)
		}
		if result.RowsAffected() == 0 {
			return ErrStaleLeaseToken
		}
		return nil
	}

	// 3. Compute exponential backoff with full jitter in Go
	delay := scheduler.ComputeBackoff(attempts, 0, 0, s.config)

	// 4. Update status back to pending with calculated delay
	updateQuery := `
		UPDATE jobs
		SET status = 'pending',
		    run_at = now() + ($2 * INTERVAL '1 second'),
		    last_error = $3,
		    lease_owner = NULL,
		    lease_until = NULL,
		    updated_at = now()
		WHERE id = $1 AND lease_token = $4 AND status = 'running';
	`
	result, err := s.pool.Exec(ctx, updateQuery, jobID, delay.Seconds(), errMsg, leaseToken)
	if err != nil {
		return fmt.Errorf("failed to reschedule failed job: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrStaleLeaseToken
	}

	return nil
}

func (s *Store) ClaimPendingJob(ctx context.Context, queues []string, workerId string, leaseDuration int) (*Job, error) {
	query := `
        WITH claimable AS (
            SELECT id FROM jobs
            WHERE status = 'pending'
              AND run_at <= now()
              AND attempts < max_attempts
              AND queue = ANY($1)
            ORDER BY priority DESC, created_at ASC
            FOR UPDATE SKIP LOCKED
            LIMIT 1
        )
        UPDATE jobs
        SET status = 'running',
        	lease_owner = $2,
         	lease_until =  now() + ($3 * INTERVAL '1 second'),
            attempts = jobs.attempts + 1,
            lease_token = lease_token+1,
            updated_at = now()
        FROM claimable
        WHERE jobs.id = claimable.id
        RETURNING jobs.id, jobs.queue, jobs.job_type, jobs.payload, jobs.status,
                  jobs.priority, jobs.attempts, jobs.max_attempts,
                  jobs.lease_owner, jobs.lease_until, jobs.lease_token, jobs.last_error,
                  jobs.run_at, jobs.created_at, jobs.updated_at;
        `
	var j Job
	args := []any{queues, workerId, leaseDuration}
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&j.ID, &j.Queue, &j.JobType, &j.Payload, &j.Status,
		&j.Priority, &j.Attempts, &j.MaxAttempts, &j.LeaseOwner, &j.LeaseUntil, &j.LeaseToken, &j.LastError, &j.RunAt,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoJobAvailable
		}
		return nil, fmt.Errorf("failed to claim pending job: %w", err)
	}
	return &j, nil
}

func (s *Store) MarkDead(ctx context.Context, jobID uuid.UUID) error {
	query := `
	update jobs
	set status='dead'
	where id=$1;
	`
	args := []any{jobID}
	result, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to mark job dead, %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("statue mismatch, no job found to mark dead, %w", err)
	}
	return nil
}

func (s *Store) ReapExpiredLeases(ctx context.Context) (int64, error) {
	query := `
		update jobs set
			status = 'pending',
			lease_owner=NULL,
			lease_until=NULL,
			updated_at=now()
		where status = 'running'
		and lease_until < now();
	`
	rows, err := s.pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to reap expired leases, %w", err)
	}
	return rows.RowsAffected(), nil
}

func (s *Store) ExtendLease(ctx context.Context, jobId uuid.UUID, leaseToken int, leaseDurationSeconds int, workerId string) error {
	query := `
	update jobs
	set lease_until = now() + ($3 * INTERVAL '1 second'),
		updated_at= now()
	where id = $1 and status = 'running' and lease_token = $2 and lease_owner = $4;
	`
	args := []any{jobId, leaseToken, leaseDurationSeconds, workerId}
	rows, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to extend lease, %w", err)
	}
	if rows.RowsAffected() == 0 {
		var currentToken int
		var status string
		err := s.pool.QueryRow(ctx, "SELECT lease_token, status FROM jobs WHERE id = $1", jobId).
			Scan(&currentToken, &status)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrJobNotFound
			}
			return err
		}
		return ErrStaleLeaseToken
	}
	return nil
}
func (s *Store) GetDeadLetteredJobs(ctx context.Context, queueName string) ([]*Job, error) {
	query := `
           SELECT
               id,
               queue,
               job_type,
               payload,
               status,
               priority,
               attempts,
               max_attempts,
               lease_owner,
               lease_until,
               lease_token,
               last_error,
               run_at,
               created_at,
               updated_at
           FROM jobs
           WHERE queue = $1 AND status = 'dead'
           ORDER BY updated_at DESC
           LIMIT 100;
       `
	rows, err := s.pool.Query(ctx, query, queueName)
	if err != nil {
		return nil, fmt.Errorf("failed to query dead-lettered jobs: %w", err)
	}

	jobs, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[Job])
	if err != nil {
		return nil, fmt.Errorf("failed to scan dead-lettered jobs: %w", err)
	}

	return jobs, nil
}

func (s *Store) ResetDeadJob(ctx context.Context, id uuid.UUID) error {
	query := `
          UPDATE jobs
          SET status = 'pending',
              attempts = 0,
              lease_owner = NULL,
              lease_until = NULL,
              lease_token = 0,
              run_at = now(),
              updated_at = now()
          WHERE id = $1 AND status = 'dead';
      `
	result, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to reset dead job: %w", err)
	}
	if result.RowsAffected() == 0 {
		var status string
		err := s.pool.QueryRow(ctx, "SELECT status FROM jobs WHERE id = $1", id).Scan(&status)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrJobNotFound
			}
			return fmt.Errorf("failed to check job status: %w", err)
		}
		return ErrJobNotDead
	}

	return nil
}
