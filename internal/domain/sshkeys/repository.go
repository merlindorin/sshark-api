package sshkeys

import (
	"context"
	"io"

	"github.com/google/uuid"
)

// SearchResult holds the results of an SSH key search operation.
type SearchResult struct {
	Entities []Entity
	Total    int
}

// NewSearchResult creates a new SearchResult with an empty Entities slice.
func NewSearchResult() *SearchResult {
	return &SearchResult{
		Entities: []Entity{},
	}
}

// ListResult holds the results of an SSH key list operation.
type ListResult struct {
	Entities []Entity
	Total    int
}

// AuthorizedKeys holds SSH public keys data from any provider.
type AuthorizedKeys struct {
	Username string
	Source   string
	Keys     io.ReadWriter
}

// Repository defines the interface for SSH key persistence operations.
type Repository interface {
	// Search finds SSH keys matching the search term with pagination.
	Search(ctx context.Context, search string, limit, offset int, entityfn func(entity *Entity)) (total int, err error)

	// List retrieves all SSH keys with pagination.
	List(ctx context.Context, limit, offset *int, result *ListResult) error

	// Get retrieves a single SSH key by its ID.
	Get(ctx context.Context, id uuid.UUID, key *Entity) error

	// Delete removes an SSH key by its ID.
	Delete(ctx context.Context, id uuid.UUID) error

	// CreateFromAuthorizedKeys parses authorized_keys data and creates SSH key entities.
	CreateFromAuthorizedKeys(
		ctx context.Context,
		authorizedKeys *AuthorizedKeys,
		provider string,
		entities *[]Entity,
	) error
}
