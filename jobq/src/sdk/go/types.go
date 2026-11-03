package jobq

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type SubmitJobRequest struct {
	Queue        string          `json:"queue,omitempty"`
	JobType      string          `json:"job_type"`
	Payload      json.RawMessage `json:"payload"`
	Priority     int             `json:"priority,omitempty"`
	MaxAttempts  int             `json:"max_attempts,omitempty"`
	RunAt        *time.Time      `json:"run_at,omitempty"`
	DelaySeconds int             `json:"delay_seconds,omitempty"`
}

type SubmitRequest = SubmitJobRequest

type ClaimJobRequest struct {
	Queues               []string `json:"queues"`
	WorkerID             string   `json:"worker_id"`
	LeaseDurationSeconds int      `json:"lease_duration_seconds"`
}

type HeartBeatRequest struct {
	LeaseToken           int    `json:"lease_token"`
	WorkerID             string `json:"worker_id"`
	LeaseDurationSeconds int    `json:"lease_duration_seconds,omitempty"`
}
type AcceptJobResponse struct {
	ID uuid.UUID `json:"id"`
}

type Job struct {
	ID          uuid.UUID       `json:"id"           db:"id"`
	Queue       string          `json:"queue"        db:"queue"`
	JobType     string          `json:"job_type"     db:"job_type"`
	Payload     json.RawMessage `json:"payload"      db:"payload"`
	Status      string          `json:"status"       db:"status"`
	Priority    int             `json:"priority"     db:"priority"`
	Attempts    int             `json:"attempts"     db:"attempts"`
	MaxAttempts int             `json:"max_attempts" db:"max_attempts"`
	LeaseOwner  *string         `json:"lease_owner,omitempty" db:"lease_owner"`
	LeaseUntil  *time.Time      `json:"lease_until,omitempty" db:"lease_until"`
	LeaseToken  int             `json:"lease_token" db:"lease_token"`
	LastError   *string         `json:"last_error,omitempty" db:"last_error"`
	RunAt       time.Time       `json:"run_at"       db:"run_at"`
	CreatedAt   time.Time       `json:"created_at"   db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"   db:"updated_at"`
}
type QueueStats struct {
	Queue        string `json:"queue" db:"queue"`
	Pending      int64  `json:"pending" db:"pending"`
	Running      int64  `json:"running" db:"running"`
	Completed    int64  `json:"completed" db:"completed"`
	Failed       int64  `json:"failed" db:"failed"`
	DeadLettered int64  `json:"dead_lettered" db:"dead_lettered"`
	Total        int64  `json:"total" db:"total"`
}

type AckRequest struct {
	LeaseToken int `json:"lease_token"`
}

type NAckRequest struct {
	LeaseToken int    `json:"lease_token"`
	Error      string `json:"error"`
	Permanent  bool   `json:"permanent"`
}

// handler types
type Handler func(ctx context.Context, payload json.RawMessage) error
