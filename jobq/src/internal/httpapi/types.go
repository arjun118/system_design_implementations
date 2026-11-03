package httpapi

import (
	"encoding/json"
	"strings"
	"time"

	"arjun118.github.io/jobq/internal/config"
	"arjun118.github.io/jobq/internal/validator"
	"github.com/google/uuid"
)

type ClaimJobRequest struct {
	Queues               []string `json:"queues"`
	WorkerID             string   `json:"worker_id"`
	LeaseDurationSeconds int      `json:"lease_duration_seconds"`
}

func (r ClaimJobRequest) Validate(v *validator.Validator, cfg *config.Config) {
	v.Check(len(r.Queues) > 0, "queues", "must provide at least one queue")
	v.Check(r.WorkerID != "", "worker_id", "worker id must not be empty")
	if r.LeaseDurationSeconds != 0 {
		v.Check(r.LeaseDurationSeconds >= cfg.MinLeaseDurationSeconds, "lease_duration_seconds", "must be at least 5s")
		v.Check(r.LeaseDurationSeconds <= cfg.MaxLeaseDurationSeconds, "lease_duration_seconds", "cannot exceed 600s")
	}
}

type ClaimJobResponse struct {
	ID          string          `json:"id"`
	Queue       string          `json:"queue"`
	JobType     string          `json:"job_type"`
	Payload     json.RawMessage `json:"payload"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	LeaseToken  int             `json:"lease_token"`
	LeaseUntil  time.Time       `json:"lease_until"`
}

type AckRequest struct {
	LeaseToken int `json:"lease_token"`
}

func (a *AckRequest) Validate(v *validator.Validator, cfg *config.Config) {
	v.Check(a.LeaseToken > 0, "lease_token", "lease token must be greater than 0")
}

type NAckRequest struct {
	LeaseToken int    `json:"lease_token"`
	Error      string `json:"error"`
	Permanent  bool   `json:"permanent"`
}

func (nk *NAckRequest) Validate(v *validator.Validator, cfg *config.Config) {
	v.Check(nk.LeaseToken > 0, "lease_token", "lease token must be greater than 0")
	v.Check(nk.Error != "", "error", "error cannot be an empty string")
}

type AcceptJobResponse struct {
	ID uuid.UUID `json:"id"`
}
type SubmitJobRequest struct {
	Queue        string          `json:"queue"`
	JobType      string          `json:"job_type"`
	Payload      json.RawMessage `json:"payload"`
	Priority     int             `json:"priority"`
	MaxAttempts  int             `json:"max_attempts"`
	RunAt        *time.Time      `json:"run_at,omitempty"`
	DelaySeconds int             `json:"delay_seconds,omitempty"`
}

func (r SubmitJobRequest) Validate(v *validator.Validator, cfg *config.Config) {
	if r.Queue != "" {
		v.Check(validator.IsValidQueueName(r.Queue), "queue", "queue name must only contain alphanumeric characters,hyphens, or underscores")
	}
	v.Check(r.JobType != "", "job_type", "job type cannot be empty")
	payloadStr := strings.TrimSpace(string(r.Payload))
	v.Check(payloadStr != "" && payloadStr != "null", "payload", "payload cannot be empty or null")
	v.Check(json.Valid(r.Payload), "payload", "payload must be valid JSON")
	v.Check(strings.HasPrefix(payloadStr, "{") && strings.HasSuffix(payloadStr, "}"), "payload", "payload must be a JSON object")
}

type ExtendLeaseRequest struct {
	WorkerID             string `json:"worker_id"`
	LeaseToken           int    `json:"lease_token"`
	LeaseDurationSeconds int    `json:"lease_duration_seconds,omitempty"`
}

func (el *ExtendLeaseRequest) Validate(v *validator.Validator, cfg *config.Config) {
	v.Check(el.LeaseToken > 0, "lease_token", "lease token must be greater than 0")
}
