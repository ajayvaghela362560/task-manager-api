package store

import (
	"context"
	"errors"

	"taskmanager/internal/models"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrEmailTaken = errors.New("email already registered")
)

// TaskFilter describes the list query. All filters combine with AND so
// status, search, sort and pagination work together.
type TaskFilter struct {
	UserID string // restrict to this owner; empty means all users (admin scope)
	Status string
	Search string // case-insensitive substring match on title
	SortBy string // "created_at", "due_date" or "priority"
	Order  string // "asc" or "desc"
	Page   int    // 1-based
	Limit  int
}

type Store interface {
	CreateUser(ctx context.Context, u *models.User) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id string) (*models.User, error)

	CreateTask(ctx context.Context, t *models.Task) error
	GetTask(ctx context.Context, id string) (*models.Task, error)
	ListTasks(ctx context.Context, f TaskFilter) ([]models.Task, int, error)
	UpdateTask(ctx context.Context, t *models.Task) error
	DeleteTask(ctx context.Context, id string) error

	AddActivity(ctx context.Context, a *models.Activity) error
	ListActivity(ctx context.Context, taskID string) ([]models.Activity, error)

	CreateAttachment(ctx context.Context, a *models.Attachment) error
	GetAttachment(ctx context.Context, id string) (*models.Attachment, error)
	ListAttachments(ctx context.Context, taskID string) ([]models.Attachment, error)
	DeleteAttachment(ctx context.Context, id string) error
}
