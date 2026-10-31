package httpapi

import (
	"encoding/json"
	"maps"
	"net/http"

	"arjun118.github.io/jobq/internal/config"
	"arjun118.github.io/jobq/internal/validator"
)

type envelope map[string]any

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any, headers http.Header) error {
	js, err := json.Marshal(data)
	if err != nil {
		return err
	}
	js = append(js, '\n')
	maps.Copy(w.Header(), headers)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)
	return nil
}

func (s *Server) readJSONAndValidate(w http.ResponseWriter, r *http.Request, cfg *config.Config, dst validator.Validatable) bool {
	err := json.NewDecoder(r.Body).Decode(dst)
	if err != nil {
		s.badRequestResponse(w, r, err)
		return false
	}
	defer r.Body.Close()
	v := validator.New()
	dst.Validate(v, cfg)
	if !v.Valid() {
		s.failedValidationResponse(w, r, v.Errors)
		return false
	}
	return true
}
