// Package redis provides Redis-based storage for GitHub users.
package redis

import (
	"time"

	"github.com/merlindorin/sshark-api/internal/domain/github"
)

// Model represents the Redis JSON document structure for a GitHub user.
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

// ToGithubUser populates the provided github.User with data from the model.
func (receiver Model) ToGithubUser(u *github.User) {
	u.Username = github.Username(receiver.Username)
	u.LastScrapedAt = receiver.LastScrapedAt
	u.ScrapedSuccessfully = receiver.ScrapedSuccessfully
}

// GetGithubUser returns a new github.User populated with data from the model.
func (receiver Model) GetGithubUser() github.User {
	return github.User{
		Username:            github.Username(receiver.Username),
		LastScrapedAt:       receiver.LastScrapedAt,
		ScrapedSuccessfully: receiver.ScrapedSuccessfully,
	}
}

// NewGithubUserModel creates a new Model with the given username and current timestamps.
func NewGithubUserModel(u github.Username) *Model {
	return &Model{
		Username:  u.String(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
