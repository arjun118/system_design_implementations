package httpapi

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) CancelJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	io.WriteString(w, "cancelled: "+id)
}
