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

func (s *Server) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		s.badRequestResponse(w, r, fmt.Errorf("invalid job id: must be a valid uuid"))
		return
	}

	job, err := s.store.GetJob(r.Context(), jobID)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrJobNotFound):
			s.notFoundResponse(w, r)
		default:
			s.serverErrorResponse(w, r, err)
		}
		return
	}

	if err := s.writeJSON(w, http.StatusOK, job, nil); err != nil {
		s.serverErrorResponse(w, r, err)
		return
	}
}

func (s *Server) GetQueueStats(w http.ResponseWriter, r *http.Request) {
	queueName := chi.URLParam(r, "queue")
	if !validator.IsValidQueueName(queueName) {
		s.badRequestResponse(w, r, fmt.Errorf("invalid queue name: only alphanumeric, hyphens, and underscores allowed"))
		return
	}
	stats, err := s.store.GetStats(r.Context(), queueName)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrQueueNotFound):
			s.notFoundResponse(w, r)
		default:
			s.serverErrorResponse(w, r, err)
			return
		}
	}
	if err := s.writeJSON(w, http.StatusOK, stats, nil); err != nil {
		s.serverErrorResponse(w, r, err)
		return
	}
}
