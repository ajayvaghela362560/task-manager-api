package api

import (
	"context"
	"net/http"
	"strings"

	"taskmanager/internal/auth"
	"taskmanager/internal/models"
)

type ctxKey int

const (
	ctxUserID ctxKey = iota
	ctxUserName
	ctxRole
)

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Missing or malformed Authorization header")
			return
		}
		claims, err := auth.ParseToken(token, s.cfg.JWTSecret)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid or expired token")
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, claims.Subject)
		ctx = context.WithValue(ctx, ctxUserName, claims.Name)
		ctx = context.WithValue(ctx, ctxRole, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userID(r *http.Request) string {
	v, _ := r.Context().Value(ctxUserID).(string)
	return v
}

func userName(r *http.Request) string {
	v, _ := r.Context().Value(ctxUserName).(string)
	return v
}

func isAdmin(r *http.Request) bool {
	v, _ := r.Context().Value(ctxRole).(string)
	return v == models.RoleAdmin
}
