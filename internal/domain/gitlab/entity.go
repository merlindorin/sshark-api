package gitlab

import (
	"bytes"
	"io"
	"time"
)

// User represents a GitLab user entity.
type User struct {
	Username            Username
	LastScrapedAt       *time.Time
	ScrapedSuccessfully *bool
}

// AuthorizedKeys holds SSH public keys in authorized_keys format for a GitLab user.
type AuthorizedKeys struct {
	Username Username
	Keys     io.ReadWriter
	Source   string
}

// NewAuthorizedKeys creates a new AuthorizedKeys with an empty buffer.
func NewAuthorizedKeys() *AuthorizedKeys {
	return &AuthorizedKeys{
		Keys: &bytes.Buffer{},
	}
}
