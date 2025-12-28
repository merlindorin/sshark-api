// Package sshkeys provides domain types and interfaces for SSH key management.
package sshkeys

import (
	"time"

	"github.com/google/uuid"
)

// Entity represents an SSH key with its metadata.
type Entity struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Source    string    `json:"source"`
	Type      string    `json:"type"`
	Comment   string    `json:"comment"`
	Options   []string  `json:"options"`
	Key       []byte    `json:"key"`
	Provider  string    `json:"provider"`
	UpdatedAt time.Time `json:"updated_at"`
}
