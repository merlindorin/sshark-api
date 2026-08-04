package sources

import (
	"time"

	"github.com/google/uuid"
)

type Entity struct {
	ID       uuid.UUID `json:"id"`
	Provider string    `json:"provider"`
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	URI      string    `json:"uri"`
	// ProfileID is the sshark account that proved it owns this provider account, if any.
	ProfileID *uuid.UUID `json:"profile_id,omitempty"`
	// ProfileUsername is that account's username, carried alongside so callers can link to the
	// public profile without a second lookup.
	ProfileUsername *string   `json:"profile_username,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ListResult struct {
	Entities []Entity
	Total    int
}
