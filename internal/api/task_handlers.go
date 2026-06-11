package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"taskmanager/internal/models"
	"taskmanager/internal/realtime"
	"taskmanager/internal/store"
)

func (s *Server) recordActivity(r *http.Request, taskID, action, detail string) {
	a := &models.Activity{TaskID: taskID, UserName: userName(r), Action: action, Detail: detail}
	if err := s.store.AddActivity(r.Context(), a); err != nil {
		// Activity logging must not fail the main operation.
		fmt.Printf("record activity: %v\n", err)
	}
}

// loadTask fetches the task in the URL and enforces access rules: owners get
// full access, admins may read any task, everyone else is told the task does
// not exist. Returns nil after writing the error response.
func (s *Server) loadTask(w http.ResponseWriter, r *http.Request, forWrite bool) *models.Task {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Task not found")
		return nil
	}
	t, err := s.store.GetTask(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Task not found")
		return nil
	}
	if err != nil {
		writeInternalError(w, err)
		return nil
	}
	if t.UserID == userID(r) {
		return t
	}
	if isAdmin(r) {
		if !forWrite {
			return t
		}
		writeError(w, http.StatusForbidden, "forbidden", "Admins can view but not modify other users' tasks")
		return nil
	}
	// Hide the existence of other users' tasks.
	writeError(w, http.StatusNotFound, "not_found", "Task not found")
	return nil
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var in models.TaskInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	if errs := in.Validate(true); len(errs) > 0 {
		writeValidationError(w, errs)
		return
	}

	t := &models.Task{
		UserID:   userID(r),
		Title:    strings.TrimSpace(*in.Title),
		Status:   models.StatusTodo,
		Priority: models.PriorityMedium,
	}
	if in.Description != nil {
		t.Description = *in.Description
	}
	if in.Status != nil {
		t.Status = *in.Status
	}
	if in.Priority != nil {
		t.Priority = *in.Priority
	}
	if in.DueDate != nil && *in.DueDate != "" {
		d, _ := time.Parse(time.RFC3339, *in.DueDate)
		t.DueDate = &d
	}

	if err := s.store.CreateTask(r.Context(), t); err != nil {
		writeInternalError(w, err)
		return
	}
	s.recordActivity(r, t.ID, "created", fmt.Sprintf("Created task %q", t.Title))
	s.hub.Publish(t.UserID, realtime.Event{Type: "task.created", Payload: t})
	writeJSON(w, http.StatusCreated, map[string]any{"task": t})
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.TaskFilter{
		UserID: userID(r),
		Status: q.Get("status"),
		Search: strings.TrimSpace(q.Get("search")),
		Order:  q.Get("order"),
		Page:   1,
		Limit:  10,
	}

	if f.Status != "" && !models.ValidStatus(f.Status) {
		writeValidationError(w, map[string]string{"status": "status must be one of: todo, in_progress, done"})
		return
	}
	switch q.Get("sortBy") {
	case "", "created_at", "createdAt":
		f.SortBy = "created_at"
	case "due_date", "dueDate":
		f.SortBy = "due_date"
	case "priority":
		f.SortBy = "priority"
	default:
		writeValidationError(w, map[string]string{"sortBy": "sortBy must be one of: created_at, due_date, priority"})
		return
	}
	if f.Order == "" {
		// Newest first by default; due dates and priority read more naturally ascending.
		if f.SortBy == "due_date" {
			f.Order = "asc"
		} else {
			f.Order = "desc"
		}
	}
	if f.Order != "asc" && f.Order != "desc" {
		writeValidationError(w, map[string]string{"order": "order must be asc or desc"})
		return
	}
	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeValidationError(w, map[string]string{"page": "page must be a positive integer"})
			return
		}
		f.Page = n
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			writeValidationError(w, map[string]string{"limit": "limit must be between 1 and 100"})
			return
		}
		f.Limit = n
	}
	if q.Get("scope") == "all" {
		if !isAdmin(r) {
			writeError(w, http.StatusForbidden, "forbidden", "Only admins can view all users' tasks")
			return
		}
		f.UserID = ""
	}

	tasks, total, err := s.store.ListTasks(r.Context(), f)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + f.Limit - 1) / f.Limit
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tasks": tasks,
		"meta": map[string]any{
			"page":       f.Page,
			"limit":      f.Limit,
			"total":      total,
			"totalPages": totalPages,
		},
	})
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	t := s.loadTask(w, r, false)
	if t == nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": t})
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	var in models.TaskInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}
	if errs := in.Validate(false); len(errs) > 0 {
		writeValidationError(w, errs)
		return
	}
	t := s.loadTask(w, r, true)
	if t == nil {
		return
	}

	changes := []string{}
	if in.Title != nil {
		if newTitle := strings.TrimSpace(*in.Title); newTitle != t.Title {
			changes = append(changes, fmt.Sprintf("renamed to %q", newTitle))
			t.Title = newTitle
		}
	}
	if in.Description != nil && *in.Description != t.Description {
		t.Description = *in.Description
		changes = append(changes, "updated the description")
	}
	if in.Status != nil && *in.Status != t.Status {
		changes = append(changes, fmt.Sprintf("moved from %s to %s", t.Status, *in.Status))
		t.Status = *in.Status
	}
	if in.Priority != nil && *in.Priority != t.Priority {
		changes = append(changes, fmt.Sprintf("changed priority from %s to %s", t.Priority, *in.Priority))
		t.Priority = *in.Priority
	}
	if in.DueDate != nil {
		if *in.DueDate == "" {
			if t.DueDate != nil {
				t.DueDate = nil
				changes = append(changes, "cleared the due date")
			}
		} else {
			d, _ := time.Parse(time.RFC3339, *in.DueDate)
			if t.DueDate == nil || !t.DueDate.Equal(d) {
				t.DueDate = &d
				changes = append(changes, fmt.Sprintf("set the due date to %s", d.Format("Jan 2, 2006")))
			}
		}
	}

	if err := s.store.UpdateTask(r.Context(), t); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Task not found")
			return
		}
		writeInternalError(w, err)
		return
	}
	if len(changes) > 0 {
		s.recordActivity(r, t.ID, "updated", strings.Join(changes, "; "))
	}
	s.hub.Publish(t.UserID, realtime.Event{Type: "task.updated", Payload: t})
	writeJSON(w, http.StatusOK, map[string]any{"task": t})
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	t := s.loadTask(w, r, true)
	if t == nil {
		return
	}
	attachments, err := s.store.ListAttachments(r.Context(), t.ID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if err := s.store.DeleteTask(r.Context(), t.ID); err != nil {
		writeInternalError(w, err)
		return
	}
	for _, a := range attachments {
		_ = os.Remove(filepath.Join(s.cfg.UploadDir, a.StoredName))
	}
	s.hub.Publish(t.UserID, realtime.Event{Type: "task.deleted", Payload: map[string]string{"id": t.ID}})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListActivity(w http.ResponseWriter, r *http.Request) {
	t := s.loadTask(w, r, false)
	if t == nil {
		return
	}
	items, err := s.store.ListActivity(r.Context(), t.ID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"activity": items})
}
