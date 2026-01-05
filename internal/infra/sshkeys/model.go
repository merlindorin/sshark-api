package sshkeys

import (
	"time"

	"github.com/google/uuid"

	"github.com/merlindorin/sshark-api/internal/domain/sshkeys"
)

type SSHKey struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Source    string    `json:"source"`
	Provider  string    `json:"provider"`
	Type      string    `json:"type"`
	Comment   string    `json:"comment"`
	Options   []string  `json:"options"`
	Key       []byte    `json:"key"`
	Raw       []byte    `json:"raw"`
	Rest      []byte    `json:"rest"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (receiver *SSHKey) ToEntity() sshkeys.Entity {
	return sshkeys.Entity{
		ID:        receiver.ID,
		Username:  receiver.Username,
		Source:    receiver.Source,
		Provider:  receiver.Provider,
		Type:      receiver.Type,
		Comment:   receiver.Comment,
		Options:   receiver.Options,
		Key:       receiver.Key,
		UpdatedAt: receiver.UpdatedAt,
	}
}
