package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"arjun118.github.io/jobq/internal/storage"
	"arjun118.github.io/jobq/internal/validator"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) RetryDeadJob(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		s.badRequestResponse(w, r, fmt.Errorf("invalid job id: must be a valid uuid"))
		return
	}

	err = s.store.ResetDeadJob(r.Context(), jobID)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrJobNotFound):
			s.notFoundResponse(w, r)
		case errors.Is(err, storage.ErrJobNotDead):
			s.errorResponse(w, r, http.StatusConflict, "job is not in dead status")
		default:
			s.serverErrorResponse(w, r, err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) GetDeadLetteredJobs(w http.ResponseWriter, r *http.Request) {
	queueName := chi.URLParam(r, "name")
	if !validator.IsValidQueueName(queueName) {
		s.badRequestResponse(w, r, fmt.Errorf("invalid queue name: only alphanumeric, hyphens, and underscores allowed"))
		return
	}

	jobs, err := s.store.GetDeadLetteredJobs(r.Context(), queueName)
	if err != nil {
		s.serverErrorResponse(w, r, err)
		return
	}

	if err := s.writeJSON(w, http.StatusOK, jobs, nil); err != nil {
		s.serverErrorResponse(w, r, err)
		return
	}
}
