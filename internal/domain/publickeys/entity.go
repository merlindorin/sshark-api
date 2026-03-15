package publickeys

import (
	"time"

	"github.com/google/uuid"
)

type KeyType string

const (
	KeyTypeSSH KeyType = "ssh"
	KeyTypeGPG KeyType = "gpg"
)

type Entity struct {
	ID          uuid.UUID    `json:"id"`
	SourceID    uuid.UUID    `json:"source_id"`
	KeyType     KeyType      `json:"key_type"`
	KeyData     []byte       `json:"key_data"`
	Fingerprint string       `json:"fingerprint"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	SSHMetadata *SSHMetadata `json:"ssh_metadata,omitempty"`
	GPGMetadata *GPGMetadata `json:"gpg_metadata,omitempty"`
}

type SSHMetadata struct {
	Algorithm string   `json:"algorithm"`
	Comment   string   `json:"comment"`
	Options   []string `json:"options"`
	KeyBits   *int     `json:"key_bits,omitempty"`
}

type GPGMetadata struct {
	Algorithm    string     `json:"algorithm"`
	KeyBits      *int       `json:"key_bits,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	UserIDs      []string   `json:"user_ids"`
	Capabilities []string   `json:"capabilities"`
}

type ScrapeHistory struct {
	ID         uuid.UUID `json:"id"`
	KeyID      uuid.UUID `json:"key_id"`
	ScrapedAt  time.Time `json:"scraped_at"`
	Success    bool      `json:"success"`
	Error      *string   `json:"error,omitempty"`
	KeyChanged bool      `json:"key_changed"`
}

type SearchFilter struct {
	SourceID  *uuid.UUID
	KeyType   *KeyType
	Algorithm *string
}

type SearchResult struct {
	Entities []Entity
	Total    int
}
