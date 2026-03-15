package sources

import (
	"context"

	"github.com/google/uuid"
)

type Stats struct {
	TotalKeys      int
	TotalSSHKeys   int
	TotalGPGKeys   int
	TotalUsernames int
	TotalProviders int
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
