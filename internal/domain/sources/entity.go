package sources

import (
	"time"

	"github.com/google/uuid"
)

type Entity struct {
	ID        uuid.UUID `json:"id"`
	Provider  string    `json:"provider"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	URI       string    `json:"uri"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ListResult struct {
	Entities []Entity
	Total    int
}
