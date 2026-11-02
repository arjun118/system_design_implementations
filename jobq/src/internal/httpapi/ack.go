package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"arjun118.github.io/jobq/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) Ack(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		s.badRequestResponse(w, r, fmt.Errorf("invalid job id: must be a valid uuid"))
		return
	}
	var input AckRequest
	if !s.readJSONAndValidate(w, r, s.config, &input) {
		return
	}
	err = s.store.MarkComplete(r.Context(), jobID, input.LeaseToken)
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
