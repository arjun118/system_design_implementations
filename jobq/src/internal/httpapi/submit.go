package httpapi

import (
	"net/http"

	"arjun118.github.io/jobq/internal/storage"
)

func (s *Server) SubmitJob(w http.ResponseWriter, r *http.Request) {
	var input SubmitJobRequest
	if !s.readJSONAndValidate(w, r, s.config, &input) {
		return
	}
	jobDetails := &storage.Job{
		Queue:       input.Queue,
		JobType:     input.JobType,
		Payload:     input.Payload,
		Priority:    input.Priority,
		MaxAttempts: input.MaxAttempts,
	}
	err := s.store.CreateJob(r.Context(), jobDetails)
	if err != nil {
		s.serverErrorResponse(w, r, err)
		return
	}
	resp := AcceptJobResponse{
		ID: jobDetails.ID,
	}
	err = s.writeJSON(w, http.StatusCreated, resp, nil)
	if err != nil {
		s.serverErrorResponse(w, r, err)
		return
	}
}
