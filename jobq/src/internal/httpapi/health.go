package httpapi

import (
	"io"
	"net/http"
)

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "healthy")
}
