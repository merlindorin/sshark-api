package publickeys

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Get(ctx context.Context, id uuid.UUID) (*Entity, error)
	GetByFingerprint(ctx context.Context, fingerprint string) (*Entity, error)
	Search(ctx context.Context, filter SearchFilter, limit, offset int) (*SearchResult, error)
	SearchWithQuery(
		ctx context.Context, keyType KeyType, whereClause string, args []any, limit, offset int,
	) (*SearchResult, error)
	Create(ctx context.Context, entity *Entity) error
	CreateBatch(ctx context.Context, entities []Entity) error
	Update(ctx context.Context, entity *Entity) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteBySourceID(ctx context.Context, sourceID uuid.UUID) error

	// Scrape history
	AddScrapeHistory(ctx context.Context, history *ScrapeHistory) error
	GetScrapeHistory(ctx context.Context, keyID uuid.UUID, limit, offset int) ([]ScrapeHistory, error)
}
