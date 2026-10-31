package httpapi

import (
	"io"
	"net/http"

	"github.com/google/uuid"
)

func (s *Server) NAckJob(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "failure: "+uuid.New().String())
}
