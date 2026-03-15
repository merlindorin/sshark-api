package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merlindorin/sshark-api/internal/domain/sources"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*sources.Entity, error) {
	var entity sources.Entity
	err := r.pool.QueryRow(ctx, `
		SELECT id, provider, user_id, username, uri, created_at, updated_at
		FROM sources
		WHERE id = $1
	`, id).Scan(
		&entity.ID,
		&entity.Provider,
		&entity.UserID,
		&entity.Username,
		&entity.URI,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sources.ErrSourceNotFound
		}
		return nil, err
	}
	return &entity, nil
}

func (r *Repository) GetByProviderAndUserID(ctx context.Context, provider, userID string) (*sources.Entity, error) {
	var entity sources.Entity
	err := r.pool.QueryRow(ctx, `
		SELECT id, provider, user_id, username, uri, created_at, updated_at
		FROM sources
		WHERE provider = $1 AND user_id = $2
	`, provider, userID).Scan(
		&entity.ID,
		&entity.Provider,
		&entity.UserID,
		&entity.Username,
		&entity.URI,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sources.ErrSourceNotFound
		}
		return nil, err
	}
	return &entity, nil
}

func (r *Repository) GetByProviderAndUsername(ctx context.Context, provider, username string) (*sources.Entity, error) {
	var entity sources.Entity
	err := r.pool.QueryRow(ctx, `
		SELECT id, provider, user_id, username, uri, created_at, updated_at
		FROM sources
		WHERE provider = $1 AND username = $2
	`, provider, username).Scan(
		&entity.ID,
		&entity.Provider,
		&entity.UserID,
		&entity.Username,
		&entity.URI,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sources.ErrSourceNotFound
		}
		return nil, err
	}
	return &entity, nil
}

func (r *Repository) List(ctx context.Context, limit, offset int) (*sources.ListResult, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM sources`).Scan(&total)
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, provider, user_id, username, uri, created_at, updated_at
		FROM sources
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []sources.Entity
	for rows.Next() {
		var entity sources.Entity
		scanErr := rows.Scan(
			&entity.ID,
			&entity.Provider,
			&entity.UserID,
			&entity.Username,
			&entity.URI,
			&entity.CreatedAt,
			&entity.UpdatedAt,
		)
		if scanErr != nil {
			return nil, scanErr
		}
		entities = append(entities, entity)
	}

	return &sources.ListResult{Entities: entities, Total: total}, nil
}

func (r *Repository) Create(ctx context.Context, entity *sources.Entity) error {
	if entity.ID == uuid.Nil {
		entity.ID = uuid.New()
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO sources (id, provider, user_id, username, uri)
		VALUES ($1, $2, $3, $4, $5)
	`, entity.ID, entity.Provider, entity.UserID, entity.Username, entity.URI)
	if err != nil {
		return err
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, entity *sources.Entity) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE sources
		SET username = $2, uri = $3, updated_at = NOW()
		WHERE id = $1
	`, entity.ID, entity.Username, entity.URI)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return sources.ErrSourceNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM sources WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return sources.ErrSourceNotFound
	}
	return nil
}

func (r *Repository) Exists(ctx context.Context, provider, userID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM sources WHERE provider = $1 AND user_id = $2)
	`, provider, userID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *Repository) GetStats(ctx context.Context) (*sources.Stats, error) {
	var stats sources.Stats

	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM public_keys) AS total_keys,
			(SELECT COUNT(*) FROM public_keys WHERE key_type = 'ssh') AS total_ssh_keys,
			(SELECT COUNT(*) FROM public_keys WHERE key_type = 'gpg') AS total_gpg_keys,
			(SELECT COUNT(DISTINCT username) FROM sources) AS total_usernames,
			(SELECT COUNT(DISTINCT provider) FROM sources) AS total_providers
	`).Scan(
		&stats.TotalKeys,
		&stats.TotalSSHKeys,
		&stats.TotalGPGKeys,
		&stats.TotalUsernames,
		&stats.TotalProviders,
	)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}
