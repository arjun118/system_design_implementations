package jobq

import (
	"encoding/json"

	"github.com/google/uuid"
)

type SubmitRequest struct {
	Queue       string          `json:"queue,omitempty"`
	JobType     string          `json:"job_type"`
	Payload     json.RawMessage `json:"payload"`
	Priority    int             `json:"priority,omitempty"`
	MaxAttempts int             `json:"max_attempts,omitempty"`
}

type AcceptJobResponse struct {
	ID uuid.UUID `json:"id"`
}
