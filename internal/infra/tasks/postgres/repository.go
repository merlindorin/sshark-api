// Package postgres stores user-visible tasks in PostgreSQL.
package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merlindorin/sshark-api/internal/domain/tasks"
)

const selectColumns = `id, clerk_user_id, kind, dedup_key, status, progress, total, message,
	result, error, created_at, started_at, finished_at, updated_at`

// uniqueViolation is PostgreSQL's code for a broken UNIQUE constraint, which here means the
// user already has this exact work queued or running.
const uniqueViolation = "23505"

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, entity *tasks.Entity) error {
	if entity.ID == uuid.Nil {
		entity.ID = uuid.New()
	}
	if entity.Status == "" {
		entity.Status = tasks.StatusPending
	}

	err := r.pool.QueryRow(ctx, `
		INSERT INTO tasks (id, clerk_user_id, kind, dedup_key, status, total, message)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`, entity.ID, entity.ClerkUserID, entity.Kind, entity.DedupKey, entity.Status, entity.Total,
		entity.Message).Scan(&entity.CreatedAt, &entity.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return tasks.ErrTaskAlreadyRunning
		}
		return err
	}

	return nil
}

func (r *Repository) FindActiveByDedupKey(
	ctx context.Context,
	clerkUserID string,
	dedupKey string,
) (*tasks.Entity, error) {
	return r.one(ctx, `
		SELECT `+selectColumns+` FROM tasks
		WHERE clerk_user_id = $1 AND dedup_key = $2 AND status IN ('pending', 'running')
	`, clerkUserID, dedupKey)
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*tasks.Entity, error) {
	return r.one(ctx, `SELECT `+selectColumns+` FROM tasks WHERE id = $1`, id)
}

func (r *Repository) one(ctx context.Context, query string, args ...any) (*tasks.Entity, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entities, err := scanTasks(rows)
	if err != nil {
		return nil, err
	}
	if len(entities) == 0 {
		return nil, tasks.ErrTaskNotFound
	}

	return &entities[0], nil
}

func (r *Repository) ListByUser(
	ctx context.Context,
	clerkUserID string,
	limit int,
) ([]tasks.Entity, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+selectColumns+` FROM tasks
		WHERE clerk_user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, clerkUserID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTasks(rows)
}

func scanTasks(rows pgx.Rows) ([]tasks.Entity, error) {
	entities := make([]tasks.Entity, 0)

	for rows.Next() {
		var entity tasks.Entity
		if err := rows.Scan(
			&entity.ID,
			&entity.ClerkUserID,
			&entity.Kind,
			&entity.DedupKey,
			&entity.Status,
			&entity.Progress,
			&entity.Total,
			&entity.Message,
			&entity.Result,
			&entity.Error,
			&entity.CreatedAt,
			&entity.StartedAt,
			&entity.FinishedAt,
			&entity.UpdatedAt,
		); err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	return entities, rows.Err()
}

func (r *Repository) MarkRunning(ctx context.Context, id uuid.UUID) error {
	// COALESCE keeps the first start time if a retry runs the job again.
	_, err := r.pool.Exec(ctx, `
		UPDATE tasks
		SET status = 'running', started_at = COALESCE(started_at, NOW()), updated_at = NOW()
		WHERE id = $1
	`, id)

	return err
}

func (r *Repository) ReportProgress(
	ctx context.Context,
	id uuid.UUID,
	progress tasks.Progress,
) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tasks
		SET progress = $2, total = $3, message = $4, updated_at = NOW()
		WHERE id = $1
	`, id, progress.Done, progress.Total, nullable(progress.Message))

	return err
}

func (r *Repository) Finish(
	ctx context.Context,
	id uuid.UUID,
	result json.RawMessage,
	taskErr error,
) error {
	status := tasks.StatusSucceeded
	var message *string
	if taskErr != nil {
		status = tasks.StatusFailed
		text := taskErr.Error()
		message = &text
	}

	_, err := r.pool.Exec(ctx, `
		UPDATE tasks
		SET status = $2, result = $3, error = $4, finished_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, id, status, result, message)

	return err
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

// compile-time check that the repository satisfies the port.
var _ tasks.Repository = (*Repository)(nil)
