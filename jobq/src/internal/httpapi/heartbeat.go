package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"arjun118.github.io/jobq/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) ExtendLease(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		s.badRequestResponse(w, r, fmt.Errorf("invalid job id: must be a valid uuid"))
		return
	}
	var input ExtendLeaseRequest
	if !s.readJSONAndValidate(w, r, s.config, &input) {
		return
	}
	duration := input.LeaseDurationSeconds
	if duration <= 0 {
		duration = s.config.LeaseDurationIncrement
	}
	if duration < 5 {
		duration = s.config.MinLeaseDurationSeconds
	}
	if duration > 600 { // Max 10 minutes per heartbeat
		duration = s.config.MaxLeaseDurationSeconds
	}

	err = s.store.ExtendLease(r.Context(), jobID, input.LeaseToken, duration, input.WorkerID)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrJobNotFound):
			s.notFoundResponse(w, r)
		case errors.Is(err, storage.ErrStaleLeaseToken):
			w.WriteHeader(http.StatusConflict)
		default:
			s.serverErrorResponse(w, r, err)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
