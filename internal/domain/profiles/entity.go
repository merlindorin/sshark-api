// Package profiles owns the sshark account a signed-in user presents publicly: the username
// they claimed and the page it resolves to.
package profiles

import (
	"time"

	"github.com/google/uuid"
)

// Entity is the sshark-side account of a signed-in user. Identity itself lives in Clerk; this
// only holds what sshark needs to serve a public page at a stable, user-chosen address.
type Entity struct {
	ID uuid.UUID `json:"id"`
	// ClerkUserID ties the profile to the authenticated user it belongs to.
	ClerkUserID string `json:"clerk_user_id"`
	// Username is what /@username resolves on. Unique regardless of case.
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
