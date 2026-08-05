// Package postgres stores sshark profiles in PostgreSQL.
package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merlindorin/sshark-api/internal/domain/profiles"
)

// uniqueViolation is the PostgreSQL error code for a broken UNIQUE constraint. Claiming a
// username races against every other claim, so the constraint is what actually decides and the
// error it raises is translated rather than pre-checked.
const uniqueViolation = "23505"

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const selectColumns = `id, clerk_user_id, username, created_at, updated_at`

func (r *Repository) GetByUsername(ctx context.Context, username string) (*profiles.Entity, error) {
	return r.getBy(ctx, `SELECT `+selectColumns+` FROM profiles WHERE username = $1`, username)
}

func (r *Repository) GetByClerkUserID(ctx context.Context, clerkUserID string) (*profiles.Entity, error) {
	return r.getBy(ctx, `SELECT `+selectColumns+` FROM profiles WHERE clerk_user_id = $1`, clerkUserID)
}

func (r *Repository) List(ctx context.Context, after uuid.UUID, limit int) ([]profiles.Entity, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+selectColumns+` FROM profiles WHERE id > $1 ORDER BY id LIMIT $2`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entities := make([]profiles.Entity, 0, limit)

	for rows.Next() {
		var entity profiles.Entity
		if scanErr := rows.Scan(
			&entity.ID,
			&entity.ClerkUserID,
			&entity.Username,
			&entity.CreatedAt,
			&entity.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}

		entities = append(entities, entity)
	}

	return entities, rows.Err()
}

func (r *Repository) getBy(ctx context.Context, query string, arg string) (*profiles.Entity, error) {
	var entity profiles.Entity

	err := r.pool.QueryRow(ctx, query, arg).Scan(
		&entity.ID,
		&entity.ClerkUserID,
		&entity.Username,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, profiles.ErrProfileNotFound
		}
		return nil, err
	}

	return &entity, nil
}

func (r *Repository) Create(ctx context.Context, entity *profiles.Entity) error {
	if entity.ID == uuid.Nil {
		entity.ID = uuid.New()
	}

	err := r.pool.QueryRow(ctx, `
		INSERT INTO profiles (id, clerk_user_id, username)
		VALUES ($1, $2, $3)
		RETURNING created_at, updated_at
	`, entity.ID, entity.ClerkUserID, entity.Username).Scan(&entity.CreatedAt, &entity.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return profiles.ErrUsernameTaken
		}
		return err
	}

	return nil
}

func (r *Repository) SetUsername(
	ctx context.Context,
	clerkUserID string,
	username string,
) (*profiles.Entity, error) {
	var entity profiles.Entity

	err := r.pool.QueryRow(ctx, `
		UPDATE profiles
		SET username = $2, updated_at = NOW()
		WHERE clerk_user_id = $1
		RETURNING `+selectColumns, clerkUserID, username).Scan(
		&entity.ID,
		&entity.ClerkUserID,
		&entity.Username,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, profiles.ErrProfileNotFound
		}
		if isUniqueViolation(err) {
			return nil, profiles.ErrUsernameTaken
		}
		return nil, err
	}

	return &entity, nil
}

func (r *Repository) IsUsernameAvailable(
	ctx context.Context,
	username string,
	forClerkUserID string,
) (bool, error) {
	var holder string

	err := r.pool.QueryRow(ctx, `SELECT clerk_user_id FROM profiles WHERE username = $1`, username).Scan(&holder)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil
		}
		return false, err
	}

	// Re-submitting the name you already hold is not a conflict.
	return holder == forClerkUserID, nil
}

func (r *Repository) DeleteByClerkUserID(ctx context.Context, clerkUserID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM profiles WHERE clerk_user_id = $1`, clerkUserID)

	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == uniqueViolation
	}

	return false
}
