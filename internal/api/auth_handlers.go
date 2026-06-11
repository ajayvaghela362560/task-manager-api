package api

import (
	"errors"
	"net/http"
	"strings"

	"taskmanager/internal/auth"
	"taskmanager/internal/models"
	"taskmanager/internal/store"
)

func (s *Server) issueToken(w http.ResponseWriter, status int, u *models.User) {
	token, err := auth.NewToken(u.ID, u.Name, u.Role, s.cfg.JWTSecret, s.cfg.TokenTTL)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, status, map[string]any{"token": token, "user": u})
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var in models.SignupInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if errs := in.Validate(); len(errs) > 0 {
		writeValidationError(w, errs)
		return
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	role := models.RoleUser
	if s.cfg.AdminEmails[in.Email] {
		role = models.RoleAdmin
	}
	u := &models.User{Name: in.Name, Email: in.Email, PasswordHash: hash, Role: role}
	if err := s.store.CreateUser(r.Context(), u); err != nil {
		if errors.Is(err, store.ErrEmailTaken) {
			writeError(w, http.StatusConflict, "email_taken", "An account with this email already exists")
			return
		}
		writeInternalError(w, err)
		return
	}
	s.issueToken(w, http.StatusCreated, u)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in models.LoginInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))

	u, err := s.store.GetUserByEmail(r.Context(), in.Email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "Incorrect email or password")
			return
		}
		writeInternalError(w, err)
		return
	}
	if !auth.CheckPassword(u.PasswordHash, in.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Incorrect email or password")
		return
	}
	s.issueToken(w, http.StatusOK, u)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.store.GetUserByID(r.Context(), userID(r))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Account no longer exists")
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
}
