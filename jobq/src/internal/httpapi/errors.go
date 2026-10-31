package httpapi

import "net/http"

func (s *Server) errorResponse(w http.ResponseWriter, r *http.Request,
	status int, message any) {
	env := map[string]any{"error": message}
	if err := s.writeJSON(w, status, env, nil); err != nil {
		s.logger.Error().
			Err(err).
			Msg("failed to write error response")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Server) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error().
		Err(err).
		Str("method", r.Method).
		Str("uri", r.URL.RequestURI()).
		Msg("internal server error")
	message := "the server encountered a problem and could not process your request"
	s.errorResponse(w, r, http.StatusInternalServerError, message)
}

func (s *Server) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	s.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

func (s *Server) notFoundResponse(w http.ResponseWriter, r *http.Request) {
	s.errorResponse(w, r, http.StatusNotFound, "the requested resource could not be found")
}

func (s *Server) failedValidationResponse(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	s.errorResponse(w, r, http.StatusUnprocessableEntity, errors)
}
