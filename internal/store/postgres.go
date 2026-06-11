package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskmanager/internal/models"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (p *Postgres) CreateUser(ctx context.Context, u *models.User) error {
	err := p.pool.QueryRow(ctx,
		`INSERT INTO users (name, email, password_hash, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id::text, created_at`,
		u.Name, u.Email, u.PasswordHash, u.Role,
	).Scan(&u.ID, &u.CreatedAt)
	if isUniqueViolation(err) {
		return ErrEmailTaken
	}
	return err
}

func (p *Postgres) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	u := &models.User{}
	err := p.pool.QueryRow(ctx,
		`SELECT id::text, name, email, password_hash, role, created_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (p *Postgres) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	u := &models.User{}
	err := p.pool.QueryRow(ctx,
		`SELECT id::text, name, email, password_hash, role, created_at
		 FROM users WHERE id = $1::uuid`,
		id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (p *Postgres) CreateTask(ctx context.Context, t *models.Task) error {
	return p.pool.QueryRow(ctx,
		`INSERT INTO tasks (user_id, title, description, status, priority, due_date)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6)
		 RETURNING id::text, created_at, updated_at`,
		t.UserID, t.Title, t.Description, t.Status, t.Priority, t.DueDate,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

func (p *Postgres) GetTask(ctx context.Context, id string) (*models.Task, error) {
	t := &models.Task{}
	err := p.pool.QueryRow(ctx,
		`SELECT t.id::text, t.user_id::text, t.title, t.description, t.status,
		        t.priority, t.due_date, t.created_at, t.updated_at, u.email
		 FROM tasks t
		 JOIN users u ON u.id = t.user_id
		 WHERE t.id = $1::uuid`,
		id,
	).Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Status,
		&t.Priority, &t.DueDate, &t.CreatedAt, &t.UpdatedAt, &t.OwnerEmail)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func orderClause(sortBy, order string) string {
	dir := "DESC"
	if strings.EqualFold(order, "asc") {
		dir = "ASC"
	}
	switch sortBy {
	case "due_date":
		return "t.due_date " + dir + " NULLS LAST, t.created_at DESC"
	case "priority":
		return "CASE t.priority WHEN 'high' THEN 3 WHEN 'medium' THEN 2 ELSE 1 END " + dir + ", t.created_at DESC"
	default:
		return "t.created_at " + dir
	}
}

func (p *Postgres) ListTasks(ctx context.Context, f TaskFilter) ([]models.Task, int, error) {
	where := " WHERE 1=1"
	args := []any{}
	if f.UserID != "" {
		args = append(args, f.UserID)
		where += fmt.Sprintf(" AND t.user_id = $%d::uuid", len(args))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		where += fmt.Sprintf(" AND t.status = $%d", len(args))
	}
	if f.Search != "" {
		args = append(args, "%"+likeEscaper.Replace(f.Search)+"%")
		where += fmt.Sprintf(" AND t.title ILIKE $%d", len(args))
	}

	total := 0
	if err := p.pool.QueryRow(ctx, "SELECT COUNT(*) FROM tasks t"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT t.id::text, t.user_id::text, t.title, t.description, t.status,
	                 t.priority, t.due_date, t.created_at, t.updated_at, u.email
	          FROM tasks t
	          JOIN users u ON u.id = t.user_id` + where +
		" ORDER BY " + orderClause(f.SortBy, f.Order)
	args = append(args, f.Limit, (f.Page-1)*f.Limit)
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	tasks := []models.Task{}
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Status,
			&t.Priority, &t.DueDate, &t.CreatedAt, &t.UpdatedAt, &t.OwnerEmail); err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, t)
	}
	return tasks, total, rows.Err()
}

func (p *Postgres) UpdateTask(ctx context.Context, t *models.Task) error {
	err := p.pool.QueryRow(ctx,
		`UPDATE tasks
		 SET title = $1, description = $2, status = $3, priority = $4,
		     due_date = $5, updated_at = now()
		 WHERE id = $6::uuid
		 RETURNING updated_at`,
		t.Title, t.Description, t.Status, t.Priority, t.DueDate, t.ID,
	).Scan(&t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (p *Postgres) DeleteTask(ctx context.Context, id string) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1::uuid`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) AddActivity(ctx context.Context, a *models.Activity) error {
	return p.pool.QueryRow(ctx,
		`INSERT INTO task_activities (task_id, user_name, action, detail)
		 VALUES ($1::uuid, $2, $3, $4)
		 RETURNING id::text, created_at`,
		a.TaskID, a.UserName, a.Action, a.Detail,
	).Scan(&a.ID, &a.CreatedAt)
}

func (p *Postgres) ListActivity(ctx context.Context, taskID string) ([]models.Activity, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT id::text, task_id::text, user_name, action, detail, created_at
		 FROM task_activities
		 WHERE task_id = $1::uuid
		 ORDER BY created_at DESC, id DESC`,
		taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []models.Activity{}
	for rows.Next() {
		var a models.Activity
		if err := rows.Scan(&a.ID, &a.TaskID, &a.UserName, &a.Action, &a.Detail, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (p *Postgres) CreateAttachment(ctx context.Context, a *models.Attachment) error {
	return p.pool.QueryRow(ctx,
		`INSERT INTO attachments (task_id, file_name, stored_name, content_type, size_bytes)
		 VALUES ($1::uuid, $2, $3, $4, $5)
		 RETURNING id::text, created_at`,
		a.TaskID, a.FileName, a.StoredName, a.ContentType, a.SizeBytes,
	).Scan(&a.ID, &a.CreatedAt)
}

func (p *Postgres) GetAttachment(ctx context.Context, id string) (*models.Attachment, error) {
	a := &models.Attachment{}
	err := p.pool.QueryRow(ctx,
		`SELECT id::text, task_id::text, file_name, stored_name, content_type, size_bytes, created_at
		 FROM attachments WHERE id = $1::uuid`,
		id,
	).Scan(&a.ID, &a.TaskID, &a.FileName, &a.StoredName, &a.ContentType, &a.SizeBytes, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (p *Postgres) ListAttachments(ctx context.Context, taskID string) ([]models.Attachment, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT id::text, task_id::text, file_name, stored_name, content_type, size_bytes, created_at
		 FROM attachments
		 WHERE task_id = $1::uuid
		 ORDER BY created_at ASC`,
		taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []models.Attachment{}
	for rows.Next() {
		var a models.Attachment
		if err := rows.Scan(&a.ID, &a.TaskID, &a.FileName, &a.StoredName, &a.ContentType, &a.SizeBytes, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (p *Postgres) DeleteAttachment(ctx context.Context, id string) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM attachments WHERE id = $1::uuid`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
