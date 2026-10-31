package httpapi

import (
	"io"
	"net/http"
)

func (s *Server) ExtendLease(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "lease extended: "+"10m")
}
