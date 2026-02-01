// Package github provides domain types and interfaces for GitHub user management.
package github

import (
	"context"
)

// ListResult holds the paginated results of a GitHub user list operation.
type ListResult struct {
	Entities []User
	Offset   int
	Limit    int
	Count    int
	Total    int
}

// Repository defines the interface for GitHub user persistence operations.
type Repository interface {

	// Exist checks if a GitHub user exists in the repository.
	Exist(ctx context.Context, username Username) (bool, error)

	// List retrieves GitHub users with pagination.
	List(ctx context.Context, limit, offset int, result *ListResult) error

	// Create adds a new GitHub user to the repository.
	Create(ctx context.Context, username Username, u *User) error

	// Get retrieves a GitHub user by username.
	Get(ctx context.Context, username Username, u *User) error

	// Delete removes a GitHub user from the repository.
	Delete(ctx context.Context, username Username) error

	// UpdateScrapeMetadata updates the scrape timestamp and success status for a user.
	UpdateScrapeMetadata(ctx context.Context, username Username, success bool) error
}

// KeyFetcher defines the interface for fetching SSH keys from GitHub.
type KeyFetcher interface {

	// FetchAuthorizedKeys retrieves the public SSH keys for a GitHub user.
	FetchAuthorizedKeys(ctx context.Context, username Username, authorizedKeys *AuthorizedKeys) error
}
