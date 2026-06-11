package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"taskmanager/internal/auth"
	"taskmanager/internal/models"
)

// handleEvents streams task events over Server-Sent Events. The token is
// passed as a query parameter because the browser EventSource API cannot set
// an Authorization header.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.ParseToken(r.URL.Query().Get("token"), s.cfg.JWTSecret)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid or expired token")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal_error", "Streaming unsupported")
		return
	}

	sub := s.hub.Subscribe(claims.Subject, claims.Role == models.RoleAdmin)
	defer s.hub.Unsubscribe(sub)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-sub.Ch:
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
