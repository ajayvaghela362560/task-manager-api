package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"taskmanager/internal/realtime"
	"taskmanager/internal/store"
)

func newTestRouter(t *testing.T, adminEmails ...string) http.Handler {
	t.Helper()
	cfg := Config{
		JWTSecret:   []byte("test-secret"),
		TokenTTL:    time.Hour,
		CORSOrigins: []string{"http://localhost:3000"},
		UploadDir:   t.TempDir(),
		AdminEmails: map[string]bool{},
		MaxUploadMB: 5,
	}
	for _, e := range adminEmails {
		cfg.AdminEmails[e] = true
	}
	return NewRouter(cfg, store.NewMemory(), realtime.NewHub())
}

func doReq(t *testing.T, h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON response %q: %v", rec.Body.String(), err)
	}
	return m
}

func signup(t *testing.T, h http.Handler, name, email string) string {
	t.Helper()
	rec := doReq(t, h, "POST", "/api/auth/signup", "", map[string]string{
		"name": name, "email": email, "password": "password123",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup returned %d: %s", rec.Code, rec.Body.String())
	}
	return decodeBody(t, rec)["token"].(string)
}

func createTask(t *testing.T, h http.Handler, token string, body map[string]any) map[string]any {
	t.Helper()
	rec := doReq(t, h, "POST", "/api/tasks", token, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create task returned %d: %s", rec.Code, rec.Body.String())
	}
	return decodeBody(t, rec)["task"].(map[string]any)
}

func TestSignupAndLogin(t *testing.T) {
	h := newTestRouter(t)

	token := signup(t, h, "Ada", "ada@example.com")
	if token == "" {
		t.Fatal("signup did not return a token")
	}

	// Duplicate email is rejected with 409.
	rec := doReq(t, h, "POST", "/api/auth/signup", "", map[string]string{
		"name": "Ada Again", "email": "ada@example.com", "password": "password123",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate signup returned %d, want 409", rec.Code)
	}

	rec = doReq(t, h, "POST", "/api/auth/login", "", map[string]string{
		"email": "ada@example.com", "password": "password123",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login returned %d: %s", rec.Code, rec.Body.String())
	}

	rec = doReq(t, h, "POST", "/api/auth/login", "", map[string]string{
		"email": "ada@example.com", "password": "wrong-password",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("login with wrong password returned %d, want 401", rec.Code)
	}

	rec = doReq(t, h, "GET", "/api/auth/me", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("me returned %d: %s", rec.Code, rec.Body.String())
	}
	user := decodeBody(t, rec)["user"].(map[string]any)
	if user["email"] != "ada@example.com" {
		t.Fatalf("me returned wrong user: %v", user)
	}

	// Task routes are protected.
	rec = doReq(t, h, "GET", "/api/tasks", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list returned %d, want 401", rec.Code)
	}
}

func TestTaskValidation(t *testing.T) {
	h := newTestRouter(t)
	token := signup(t, h, "Ada", "ada@example.com")

	rec := doReq(t, h, "POST", "/api/tasks", token, map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create without title returned %d, want 400", rec.Code)
	}
	errBody := decodeBody(t, rec)["error"].(map[string]any)
	if errBody["code"] != "validation_error" {
		t.Fatalf("unexpected error code: %v", errBody)
	}
	if _, ok := errBody["fields"].(map[string]any)["title"]; !ok {
		t.Fatalf("expected a field error for title, got: %v", errBody)
	}

	rec = doReq(t, h, "POST", "/api/tasks", token, map[string]any{"title": "ok", "status": "archived"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create with invalid status returned %d, want 400", rec.Code)
	}

	rec = doReq(t, h, "POST", "/api/tasks", token, map[string]any{"title": "ok", "dueDate": "tomorrow"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create with invalid due date returned %d, want 400", rec.Code)
	}
}

func TestTaskCRUDAndOwnership(t *testing.T) {
	h := newTestRouter(t)
	tokenA := signup(t, h, "Ada", "ada@example.com")
	tokenB := signup(t, h, "Bob", "bob@example.com")

	task := createTask(t, h, tokenA, map[string]any{
		"title": "Write assessment", "priority": "high", "dueDate": "2026-06-20T00:00:00Z",
	})
	id := task["id"].(string)

	// Owner can read it.
	rec := doReq(t, h, "GET", "/api/tasks/"+id, tokenA, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner get returned %d: %s", rec.Code, rec.Body.String())
	}

	// Another user cannot see or touch it; existence is hidden.
	for _, method := range []string{"GET", "PATCH", "DELETE"} {
		var body any
		if method == "PATCH" {
			body = map[string]any{"status": "done"}
		}
		rec := doReq(t, h, method, "/api/tasks/"+id, tokenB, body)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s by non-owner returned %d, want 404", method, rec.Code)
		}
	}

	// Owner can update it.
	rec = doReq(t, h, "PATCH", "/api/tasks/"+id, tokenA, map[string]any{"status": "done"})
	if rec.Code != http.StatusOK {
		t.Fatalf("owner patch returned %d: %s", rec.Code, rec.Body.String())
	}
	if got := decodeBody(t, rec)["task"].(map[string]any)["status"]; got != "done" {
		t.Fatalf("status not updated, got %v", got)
	}

	// The activity log recorded both the creation and the update.
	rec = doReq(t, h, "GET", "/api/tasks/"+id+"/activity", tokenA, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("activity returned %d: %s", rec.Code, rec.Body.String())
	}
	if entries := decodeBody(t, rec)["activity"].([]any); len(entries) < 2 {
		t.Fatalf("expected at least 2 activity entries, got %d", len(entries))
	}

	// Owner can delete it, after which it is gone.
	rec = doReq(t, h, "DELETE", "/api/tasks/"+id, tokenA, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete returned %d, want 204", rec.Code)
	}
	rec = doReq(t, h, "GET", "/api/tasks/"+id, tokenA, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete returned %d, want 404", rec.Code)
	}
}

func TestListFilterSearchSortPagination(t *testing.T) {
	h := newTestRouter(t)
	token := signup(t, h, "Ada", "ada@example.com")

	createTask(t, h, token, map[string]any{"title": "Alpha report", "status": "todo", "priority": "low", "dueDate": "2026-07-01T00:00:00Z"})
	createTask(t, h, token, map[string]any{"title": "Beta launch", "status": "in_progress", "priority": "high", "dueDate": "2026-06-20T00:00:00Z"})
	createTask(t, h, token, map[string]any{"title": "Gamma cleanup", "status": "done", "priority": "medium"})
	createTask(t, h, token, map[string]any{"title": "Alpha follow-up", "status": "todo", "priority": "high", "dueDate": "2026-06-15T00:00:00Z"})

	list := func(query string) (items []any, meta map[string]any) {
		t.Helper()
		rec := doReq(t, h, "GET", "/api/tasks"+query, token, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("list %q returned %d: %s", query, rec.Code, rec.Body.String())
		}
		body := decodeBody(t, rec)
		return body["tasks"].([]any), body["meta"].(map[string]any)
	}

	if _, meta := list(""); meta["total"].(float64) != 4 {
		t.Fatalf("expected 4 tasks, got %v", meta["total"])
	}

	// Status filter.
	if items, _ := list("?status=todo"); len(items) != 2 {
		t.Fatalf("status filter: expected 2 tasks, got %d", len(items))
	}

	// Search by title, combined with a status filter.
	if items, _ := list("?search=alpha"); len(items) != 2 {
		t.Fatalf("search: expected 2 tasks, got %d", len(items))
	}
	if items, _ := list("?search=alpha&status=todo"); len(items) != 2 {
		t.Fatalf("search+filter: expected 2 tasks, got %d", len(items))
	}
	if items, _ := list("?search=beta&status=todo"); len(items) != 0 {
		t.Fatalf("search+filter: expected 0 tasks, got %d", len(items))
	}

	// Sort by priority descending: a high-priority task comes first.
	items, _ := list("?sortBy=priority&order=desc")
	if got := items[0].(map[string]any)["priority"]; got != "high" {
		t.Fatalf("priority sort: first task has priority %v, want high", got)
	}

	// Sort by due date ascending: earliest due date first, no-due-date last.
	items, _ = list("?sortBy=due_date&order=asc")
	if got := items[0].(map[string]any)["title"]; got != "Alpha follow-up" {
		t.Fatalf("due date sort: first task is %v, want Alpha follow-up", got)
	}
	if got := items[len(items)-1].(map[string]any)["title"]; got != "Gamma cleanup" {
		t.Fatalf("due date sort: last task is %v, want Gamma cleanup (no due date)", got)
	}

	// Pagination metadata.
	items, meta := list("?limit=2&page=2")
	if len(items) != 2 || meta["totalPages"].(float64) != 2 || meta["total"].(float64) != 4 {
		t.Fatalf("pagination: got %d items, meta %v", len(items), meta)
	}

	// Search combined with sort and pagination all at once.
	items, meta = list("?search=alpha&sortBy=due_date&order=asc&limit=1&page=1")
	if len(items) != 1 || meta["total"].(float64) != 2 {
		t.Fatalf("combined query: got %d items, meta %v", len(items), meta)
	}
	if got := items[0].(map[string]any)["title"]; got != "Alpha follow-up" {
		t.Fatalf("combined query: first task is %v, want Alpha follow-up", got)
	}
}

func TestAdminScope(t *testing.T) {
	h := newTestRouter(t, "admin@example.com")
	adminToken := signup(t, h, "Admin", "admin@example.com")
	userToken := signup(t, h, "Bob", "bob@example.com")

	task := createTask(t, h, userToken, map[string]any{"title": "Bob's private task"})
	id := task["id"].(string)

	// Regular users cannot request the all-users scope.
	rec := doReq(t, h, "GET", "/api/tasks?scope=all", userToken, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("scope=all as user returned %d, want 403", rec.Code)
	}

	// Admins see other users' tasks in the all scope, with owner emails.
	rec = doReq(t, h, "GET", "/api/tasks?scope=all", adminToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("scope=all as admin returned %d: %s", rec.Code, rec.Body.String())
	}
	tasks := decodeBody(t, rec)["tasks"].([]any)
	if len(tasks) != 1 {
		t.Fatalf("admin scope=all: expected 1 task, got %d", len(tasks))
	}
	if got := tasks[0].(map[string]any)["ownerEmail"]; got != "bob@example.com" {
		t.Fatalf("admin scope=all: ownerEmail is %v, want bob@example.com", got)
	}

	// Admins can read a single task they do not own, but not modify it.
	rec = doReq(t, h, "GET", "/api/tasks/"+id, adminToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin get returned %d, want 200", rec.Code)
	}
	rec = doReq(t, h, "PATCH", "/api/tasks/"+id, adminToken, map[string]any{"status": "done"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin patch returned %d, want 403", rec.Code)
	}
	rec = doReq(t, h, "DELETE", "/api/tasks/"+id, adminToken, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin delete returned %d, want 403", rec.Code)
	}
}
