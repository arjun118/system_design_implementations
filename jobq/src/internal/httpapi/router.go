package httpapi

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (s *Server) setupRoutes() {
	r := chi.NewRouter()

	// global middleware
	r.Use(middleware.RequestID)
	r.Use(s.recoverPanic)
	r.Use(s.logRequest)

	//health
	r.Get("/v1/health", s.Health)

	// producer endpoints
	r.Post("/v1/jobs", s.SubmitJob)
	r.Get("/v1/jobs/{id}", s.GetJobStatus)
	r.Delete("/v1/jobs/{id}", s.CancelJob)
	r.Get("/v1/queues/{name}/stats", s.GetQueueStats)

	// worker endpoints
	r.Post("/v1/jobs/claim", s.ClaimJob)
	r.Post("/v1/jobs/{id}/ack", s.AckJob)
	r.Post("/v1/jobs/{id}/nack", s.NAckJob)
	r.Post("/v1/jobs/{id}/heartbeat", s.ExtendLease)
	s.router = r
}
