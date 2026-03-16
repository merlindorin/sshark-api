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
	stats := &sources.Stats{
		Facets: make(map[string][]sources.Facet),
	}

	providerFacet, err := r.getValueFacet(ctx, `
		SELECT s.provider, COUNT(pk.id)
		FROM public_keys pk
		JOIN sources s ON pk.source_id = s.id
		WHERE pk.key_type = 'ssh'
		GROUP BY s.provider
		ORDER BY COUNT(pk.id) DESC
	`)
	if err != nil {
		return nil, err
	}
	stats.Facets["source.provider"] = []sources.Facet{providerFacet}

	algorithmFacet, err := r.getValueFacet(ctx, `
		SELECT algorithm, COUNT(*)
		FROM ssh_key_metadata
		GROUP BY algorithm
		ORDER BY COUNT(*) DESC
	`)
	if err != nil {
		return nil, err
	}
	stats.Facets["algorithm"] = []sources.Facet{algorithmFacet}

	var usernameCount int64
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT s.username)
		FROM public_keys pk
		JOIN sources s ON pk.source_id = s.id
		WHERE pk.key_type = 'ssh'
	`).Scan(&usernameCount)
	if err != nil {
		return nil, err
	}
	usernameFacet := sources.Facet{
		Type: "value",
		Data: []sources.FacetValue{{Value: "total", Count: int(usernameCount)}},
	}
	stats.Facets["source.username"] = []sources.Facet{usernameFacet}

	return stats, nil
}

func (r *Repository) getValueFacet(ctx context.Context, query string) (sources.Facet, error) {
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return sources.Facet{}, err
	}
	defer rows.Close()

	facet := sources.Facet{
		Type: "value",
		Data: []sources.FacetValue{},
	}
	for rows.Next() {
		var value string
		var count int64
		if scanErr := rows.Scan(&value, &count); scanErr != nil {
			return sources.Facet{}, scanErr
		}
		facet.Data = append(facet.Data, sources.FacetValue{Value: value, Count: int(count)})
	}
	return facet, rows.Err()
}
