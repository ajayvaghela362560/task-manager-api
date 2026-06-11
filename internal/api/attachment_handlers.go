package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"taskmanager/internal/models"
	"taskmanager/internal/realtime"
	"taskmanager/internal/store"
)

var allowedExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".pdf": true, ".txt": true, ".md": true, ".doc": true, ".docx": true,
}

func (s *Server) handleUploadAttachment(w http.ResponseWriter, r *http.Request) {
	t := s.loadTask(w, r, true)
	if t == nil {
		return
	}

	maxBytes := s.cfg.MaxUploadMB << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+1<<20) // headroom for multipart framing
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		writeError(w, http.StatusBadRequest, "upload_too_large",
			fmt.Sprintf("File exceeds the %d MB limit", s.cfg.MaxUploadMB))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", `Missing "file" form field`)
		return
	}
	defer file.Close()
	if header.Size > maxBytes {
		writeError(w, http.StatusBadRequest, "upload_too_large",
			fmt.Sprintf("File exceeds the %d MB limit", s.cfg.MaxUploadMB))
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExtensions[ext] {
		writeError(w, http.StatusBadRequest, "unsupported_type",
			"Allowed file types: png, jpg, jpeg, gif, webp, pdf, txt, md, doc, docx")
		return
	}

	if err := os.MkdirAll(s.cfg.UploadDir, 0o755); err != nil {
		writeInternalError(w, err)
		return
	}
	storedName := uuid.NewString() + ext
	dst, err := os.Create(filepath.Join(s.cfg.UploadDir, storedName))
	if err != nil {
		writeInternalError(w, err)
		return
	}
	size, err := io.Copy(dst, file)
	dst.Close()
	if err != nil {
		writeInternalError(w, err)
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	a := &models.Attachment{
		TaskID:      t.ID,
		FileName:    header.Filename,
		StoredName:  storedName,
		ContentType: contentType,
		SizeBytes:   size,
	}
	if err := s.store.CreateAttachment(r.Context(), a); err != nil {
		_ = os.Remove(filepath.Join(s.cfg.UploadDir, storedName))
		writeInternalError(w, err)
		return
	}
	s.recordActivity(r, t.ID, "attachment_added", fmt.Sprintf("Attached file %q", a.FileName))
	s.hub.Publish(t.UserID, realtime.Event{Type: "task.updated", Payload: t})
	writeJSON(w, http.StatusCreated, map[string]any{"attachment": a})
}

func (s *Server) handleListAttachments(w http.ResponseWriter, r *http.Request) {
	t := s.loadTask(w, r, false)
	if t == nil {
		return
	}
	items, err := s.store.ListAttachments(r.Context(), t.ID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"attachments": items})
}

// loadAttachment resolves the attachment in the URL and checks access to its
// parent task. Returns nils after writing the error response.
func (s *Server) loadAttachment(w http.ResponseWriter, r *http.Request, forWrite bool) (*models.Attachment, *models.Task) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Attachment not found")
		return nil, nil
	}
	a, err := s.store.GetAttachment(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Attachment not found")
		return nil, nil
	}
	if err != nil {
		writeInternalError(w, err)
		return nil, nil
	}
	t, err := s.store.GetTask(r.Context(), a.TaskID)
	if err != nil {
		writeInternalError(w, err)
		return nil, nil
	}
	if t.UserID == userID(r) || (!forWrite && isAdmin(r)) {
		return a, t
	}
	writeError(w, http.StatusNotFound, "not_found", "Attachment not found")
	return nil, nil
}

func (s *Server) handleDownloadAttachment(w http.ResponseWriter, r *http.Request) {
	a, _ := s.loadAttachment(w, r, false)
	if a == nil {
		return
	}
	w.Header().Set("Content-Type", a.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", a.FileName))
	http.ServeFile(w, r, filepath.Join(s.cfg.UploadDir, a.StoredName))
}

func (s *Server) handleDeleteAttachment(w http.ResponseWriter, r *http.Request) {
	a, t := s.loadAttachment(w, r, true)
	if a == nil {
		return
	}
	if err := s.store.DeleteAttachment(r.Context(), a.ID); err != nil {
		writeInternalError(w, err)
		return
	}
	_ = os.Remove(filepath.Join(s.cfg.UploadDir, a.StoredName))
	s.recordActivity(r, t.ID, "attachment_removed", fmt.Sprintf("Removed file %q", a.FileName))
	s.hub.Publish(t.UserID, realtime.Event{Type: "task.updated", Payload: t})
	w.WriteHeader(http.StatusNoContent)
}
