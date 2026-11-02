package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"arjun118.github.io/jobq/internal/storage"
)

func (s *Server) Claim(w http.ResponseWriter, r *http.Request) {
	// return 204 no job avaialble
	var input ClaimJobRequest
	if !s.readJSONAndValidate(w, r, s.config, &input) {
		return
	}
	job, err := s.store.ClaimPendingJob(r.Context(), input.Queues, input.WorkerID, input.LeaseDurationSeconds)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrNoJobAvailable):
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			s.serverErrorResponse(w, r, fmt.Errorf("error claiming a pending job: %w", err))
			return
		}
	}
	if err := s.writeJSON(w, http.StatusOK, job, nil); err != nil {
		s.serverErrorResponse(w, r, err)
		return
	}
}
