package httpapi

import (
	"net/http"

	"arjun118.github.io/jobq/internal/config"
	"arjun118.github.io/jobq/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

type Server struct {
	logger zerolog.Logger
	store  *storage.Store
	router chi.Router
	config *config.Config
}

func NewServer(logger zerolog.Logger, store *storage.Store) *Server {
	s := &Server{
		logger: logger,
		store:  store,
		config: config.Load(logger),
	}
	s.setupRoutes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
