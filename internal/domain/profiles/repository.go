package profiles

import (
	"context"
)

// Repository stores the sshark profiles.
type Repository interface {
	// GetByUsername returns the profile holding this username, matched case-insensitively.
	// It returns ErrProfileNotFound when nobody holds it.
	GetByUsername(ctx context.Context, username string) (*Entity, error)

	// GetByClerkUserID returns the profile of a signed-in user, or ErrProfileNotFound.
	GetByClerkUserID(ctx context.Context, clerkUserID string) (*Entity, error)

	// Create stores a new profile. It returns ErrUsernameTaken when the username is held.
	Create(ctx context.Context, entity *Entity) error

	// SetUsername moves a profile to a new username, returning ErrUsernameTaken if it is held
	// by someone else.
	SetUsername(ctx context.Context, clerkUserID string, username string) (*Entity, error)

	// IsUsernameAvailable reports whether the username is free for the given user. A user's own
	// current username counts as available so re-submitting it is not an error.
	IsUsernameAvailable(ctx context.Context, username string, forClerkUserID string) (bool, error)

	// DeleteByClerkUserID releases a user's profile, freeing the username. Deleting an account
	// that has none is not an error.
	DeleteByClerkUserID(ctx context.Context, clerkUserID string) error
}
