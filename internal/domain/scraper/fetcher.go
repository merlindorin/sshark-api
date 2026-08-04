package scraper

import (
	"context"
	"time"
)

// Provider represents a key provider (github, gitlab, etc.)
type Provider string

const (
	ProviderGitHub Provider = "github"
	ProviderGitLab Provider = "gitlab"
)

// KeyType represents the type of key (ssh, gpg).
type KeyType string

const (
	KeyTypeSSH KeyType = "ssh"
	KeyTypeGPG KeyType = "gpg"
)

// FetchedKey represents a key fetched from a provider.
type FetchedKey struct {
	KeyID       string  // Provider's key ID
	KeyType     KeyType // ssh or gpg
	KeyData     []byte
	Fingerprint string
	Algorithm   string
	Comment     string
	KeyBits     *int
	// GPG-specific fields
	ExpiresAt    *time.Time
	UserIDs      []string
	Capabilities []string
}

// FetchedUser represents user info from a provider.
type FetchedUser struct {
	UserID   string
	Username string
	URI      string
	Keys     []FetchedKey // SSH keys
	GPGKeys  []FetchedKey // GPG keys
}

// UsersPage represents a page of users from the provider.
type UsersPage struct {
	Users      []FetchedUser
	NextCursor string // Empty if no more pages
}

// Fetcher fetches keys from a provider.
type Fetcher interface {
	// Provider returns the provider name.
	Provider() Provider

	// ListUsers lists users from the provider starting from cursor.
	// Returns a page of users and the next cursor.
	// Pass empty cursor to start from the beginning.
	ListUsers(ctx context.Context, cursor string, limit int) (*UsersPage, error)

	// FetchUser fetches a single user by username, without their keys.
	// Returns ErrUserNotFound when the provider does not know the username.
	FetchUser(ctx context.Context, username string) (*FetchedUser, error)

	// FetchUserKeys fetches SSH keys for a specific user.
	FetchUserKeys(ctx context.Context, user *FetchedUser) error

	// FetchUserGPGKeys fetches GPG keys for a specific user.
	FetchUserGPGKeys(ctx context.Context, user *FetchedUser) error
}

// Progress tracks scraping progress for a provider.
type Progress struct {
	Provider   Provider
	LastCursor string
}
