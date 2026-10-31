package httpapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

func (s *Server) ClaimJob(w http.ResponseWriter, r *http.Request) {
	var input ClaimJobRequest
	if !s.readJSONAndValidate(w, r, s.config, &input) {
		return
	}
	response := ClaimJobResponse{
		ID:          uuid.NewString(),
		Queue:       input.Queues[0],
		JobType:     "dummy",
		Payload:     []byte(`{"key": "value"}`),
		Attempts:    1,
		MaxAttempts: 4,
		LeaseToken:  1,
		LeaseUntil:  time.Now().Add(10 * time.Minute),
	}
	s.writeJSON(w, http.StatusOK, envelope{"job_details": response}, nil)
}
