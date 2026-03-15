package sources

import (
	"context"

	"github.com/google/uuid"
)

type FacetValue struct {
	Value string
	Count int
}

type Facet struct {
	Type string
	Data []FacetValue
}

type Stats struct {
	Facets map[string][]Facet
}

type Repository interface {
	Get(ctx context.Context, id uuid.UUID) (*Entity, error)
	GetByProviderAndUserID(ctx context.Context, provider, userID string) (*Entity, error)
	GetByProviderAndUsername(ctx context.Context, provider, username string) (*Entity, error)
	List(ctx context.Context, limit, offset int) (*ListResult, error)
	Create(ctx context.Context, entity *Entity) error
	Update(ctx context.Context, entity *Entity) error
	Delete(ctx context.Context, id uuid.UUID) error
	Exists(ctx context.Context, provider, userID string) (bool, error)
	GetStats(ctx context.Context) (*Stats, error)
}
