package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"taskmanager/internal/realtime"
	"taskmanager/internal/store"
)

type Server struct {
	cfg   Config
	store store.Store
	hub   *realtime.Hub
}

func NewRouter(cfg Config, st store.Store, hub *realtime.Hub) http.Handler {
	s := &Server{cfg: cfg, store: st, hub: hub}

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: cfg.CORSOrigins,
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		MaxAge:         300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/signup", s.handleSignup)
		r.Post("/auth/login", s.handleLogin)
		r.Get("/events", s.handleEvents) // SSE; authenticates via ?token=

		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/auth/me", s.handleMe)

			r.Route("/tasks", func(r chi.Router) {
				r.Get("/", s.handleListTasks)
				r.Post("/", s.handleCreateTask)
				r.Get("/{id}", s.handleGetTask)
				r.Patch("/{id}", s.handleUpdateTask)
				r.Delete("/{id}", s.handleDeleteTask)
				r.Get("/{id}/activity", s.handleListActivity)
				r.Get("/{id}/attachments", s.handleListAttachments)
				r.Post("/{id}/attachments", s.handleUploadAttachment)
			})

			r.Get("/attachments/{id}/download", s.handleDownloadAttachment)
			r.Delete("/attachments/{id}", s.handleDeleteAttachment)
		})
	})

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "Route not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	})
	return r
}
