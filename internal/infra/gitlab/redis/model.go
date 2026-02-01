// Package redis provides Redis-based storage for GitLab users.
package redis

import (
	"time"

	"github.com/merlindorin/sshark-api/internal/domain/gitlab"
)

// Model represents the Redis JSON document structure for a GitLab user.
// This is the storage model used for RediSearch indexing.
//
// Index fields:
//   - $.username              -> username (TAG)
//   - $.created_at            -> created_at (TEXT, sortable)
//   - $.updated_at            -> updated_at (TEXT, sortable)
//   - $.last_scraped_at       -> last_scraped_at (NUMERIC, sortable)
//   - $.scraped_successfully  -> scraped_successfully (TAG)
type Model struct {
	Username            string     `json:"username"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	LastScrapedAt       *time.Time `json:"last_scraped_at,omitempty"`
	ScrapedSuccessfully *bool      `json:"scraped_successfully,omitempty"`
}

// ToGitlabUser populates the provided gitlab.User with data from the model.
func (receiver Model) ToGitlabUser(u *gitlab.User) {
	u.Username = gitlab.Username(receiver.Username)
	u.LastScrapedAt = receiver.LastScrapedAt
	u.ScrapedSuccessfully = receiver.ScrapedSuccessfully
}

// GetGitlabUser returns a new gitlab.User populated with data from the model.
func (receiver Model) GetGitlabUser() gitlab.User {
	return gitlab.User{
		Username:            gitlab.Username(receiver.Username),
		LastScrapedAt:       receiver.LastScrapedAt,
		ScrapedSuccessfully: receiver.ScrapedSuccessfully,
	}
}

// NewGitlabUserModel creates a new Model with the given username and current timestamps.
func NewGitlabUserModel(u gitlab.Username) *Model {
	return &Model{
		Username:  u.String(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
