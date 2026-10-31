package httpapi

import (
	"io"
	"net/http"

	"github.com/google/uuid"
)

func (s *Server) AckJob(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "completed: "+uuid.New().String())
}
