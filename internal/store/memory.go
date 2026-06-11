package store

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"taskmanager/internal/models"
)

// Memory is an in-memory Store used by the unit tests. It mirrors the
// filtering, sorting and pagination semantics of the Postgres store.
type Memory struct {
	mu          sync.RWMutex
	users       map[string]*models.User
	tasks       map[string]*models.Task
	activities  map[string][]models.Activity
	attachments map[string]*models.Attachment
}

func NewMemory() *Memory {
	return &Memory{
		users:       map[string]*models.User{},
		tasks:       map[string]*models.Task{},
		activities:  map[string][]models.Activity{},
		attachments: map[string]*models.Attachment{},
	}
}

func (m *Memory) CreateUser(_ context.Context, u *models.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.users {
		if existing.Email == u.Email {
			return ErrEmailTaken
		}
	}
	u.ID = uuid.NewString()
	u.CreatedAt = time.Now()
	cp := *u
	m.users[u.ID] = &cp
	return nil
}

func (m *Memory) GetUserByEmail(_ context.Context, email string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, u := range m.users {
		if u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) GetUserByID(_ context.Context, id string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (m *Memory) CreateTask(_ context.Context, t *models.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t.ID = uuid.NewString()
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	cp := *t
	m.tasks[t.ID] = &cp
	return nil
}

func (m *Memory) GetTask(_ context.Context, id string) (*models.Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tasks[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *t
	if u, ok := m.users[t.UserID]; ok {
		cp.OwnerEmail = u.Email
	}
	return &cp, nil
}

var priorityRank = map[string]int{
	models.PriorityLow:    1,
	models.PriorityMedium: 2,
	models.PriorityHigh:   3,
}

func (m *Memory) ListTasks(_ context.Context, f TaskFilter) ([]models.Task, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	matched := []models.Task{}
	for _, t := range m.tasks {
		if f.UserID != "" && t.UserID != f.UserID {
			continue
		}
		if f.Status != "" && t.Status != f.Status {
			continue
		}
		if f.Search != "" && !strings.Contains(strings.ToLower(t.Title), strings.ToLower(f.Search)) {
			continue
		}
		cp := *t
		if u, ok := m.users[t.UserID]; ok {
			cp.OwnerEmail = u.Email
		}
		matched = append(matched, cp)
	}

	asc := strings.EqualFold(f.Order, "asc")
	sort.SliceStable(matched, func(i, j int) bool {
		a, b := matched[i], matched[j]
		switch f.SortBy {
		case "due_date":
			// nil due dates sort last regardless of direction, like NULLS LAST
			switch {
			case a.DueDate == nil && b.DueDate == nil:
				return a.CreatedAt.After(b.CreatedAt)
			case a.DueDate == nil:
				return false
			case b.DueDate == nil:
				return true
			case a.DueDate.Equal(*b.DueDate):
				return a.CreatedAt.After(b.CreatedAt)
			}
			if asc {
				return a.DueDate.Before(*b.DueDate)
			}
			return a.DueDate.After(*b.DueDate)
		case "priority":
			if priorityRank[a.Priority] == priorityRank[b.Priority] {
				return a.CreatedAt.After(b.CreatedAt)
			}
			if asc {
				return priorityRank[a.Priority] < priorityRank[b.Priority]
			}
			return priorityRank[a.Priority] > priorityRank[b.Priority]
		default:
			if asc {
				return a.CreatedAt.Before(b.CreatedAt)
			}
			return a.CreatedAt.After(b.CreatedAt)
		}
	})

	total := len(matched)
	start := (f.Page - 1) * f.Limit
	if start >= total {
		return []models.Task{}, total, nil
	}
	end := start + f.Limit
	if end > total {
		end = total
	}
	return matched[start:end], total, nil
}

func (m *Memory) UpdateTask(_ context.Context, t *models.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tasks[t.ID]; !ok {
		return ErrNotFound
	}
	t.UpdatedAt = time.Now()
	cp := *t
	m.tasks[t.ID] = &cp
	return nil
}

func (m *Memory) DeleteTask(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tasks[id]; !ok {
		return ErrNotFound
	}
	delete(m.tasks, id)
	delete(m.activities, id)
	for aid, a := range m.attachments {
		if a.TaskID == id {
			delete(m.attachments, aid)
		}
	}
	return nil
}

func (m *Memory) AddActivity(_ context.Context, a *models.Activity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a.ID = uuid.NewString()
	a.CreatedAt = time.Now()
	m.activities[a.TaskID] = append(m.activities[a.TaskID], *a)
	return nil
}

func (m *Memory) ListActivity(_ context.Context, taskID string) ([]models.Activity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	src := m.activities[taskID]
	items := make([]models.Activity, 0, len(src))
	for i := len(src) - 1; i >= 0; i-- { // newest first
		items = append(items, src[i])
	}
	return items, nil
}

func (m *Memory) CreateAttachment(_ context.Context, a *models.Attachment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a.ID = uuid.NewString()
	a.CreatedAt = time.Now()
	cp := *a
	m.attachments[a.ID] = &cp
	return nil
}

func (m *Memory) GetAttachment(_ context.Context, id string) (*models.Attachment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.attachments[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *a
	return &cp, nil
}

func (m *Memory) ListAttachments(_ context.Context, taskID string) ([]models.Attachment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := []models.Attachment{}
	for _, a := range m.attachments {
		if a.TaskID == taskID {
			items = append(items, *a)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (m *Memory) DeleteAttachment(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.attachments[id]; !ok {
		return ErrNotFound
	}
	delete(m.attachments, id)
	return nil
}
