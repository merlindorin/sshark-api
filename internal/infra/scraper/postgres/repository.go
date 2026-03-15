package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merlindorin/sshark-api/internal/domain/scraper"
)

// ProgressRepository implements scraper.ProgressRepository.
type ProgressRepository struct {
	pool *pgxpool.Pool
}

// NewProgressRepository creates a new progress repository.
func NewProgressRepository(pool *pgxpool.Pool) *ProgressRepository {
	return &ProgressRepository{pool: pool}
}

// GetProgress returns the current progress for a provider.
func (r *ProgressRepository) GetProgress(
	ctx context.Context,
	provider scraper.Provider,
) (*scraper.Progress, error) {
	var cursor string
	err := r.pool.QueryRow(ctx, `
		SELECT last_cursor FROM scrape_progress WHERE provider = $1
	`, string(provider)).Scan(&cursor)

	if errors.Is(err, pgx.ErrNoRows) {
		return &scraper.Progress{
			Provider:   provider,
			LastCursor: "",
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return &scraper.Progress{
		Provider:   provider,
		LastCursor: cursor,
	}, nil
}

// SaveProgress saves the current progress for a provider.
func (r *ProgressRepository) SaveProgress(ctx context.Context, progress *scraper.Progress) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO scrape_progress (provider, last_cursor, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (provider) DO UPDATE SET
			last_cursor = EXCLUDED.last_cursor,
			updated_at = NOW()
	`, string(progress.Provider), progress.LastCursor)
	return err
}
