package stats

import "context"

// Stats holds aggregated statistics about SSH keys.
type Stats struct {
	TotalKeys      int `json:"total_keys"`
	TotalUsernames int `json:"total_usernames"`
	TotalProviders int `json:"total_providers"`
}

// Repository defines the interface for retrieving statistics.
type Repository interface {
	// GetStats retrieves aggregated statistics about SSH keys.
	GetStats(ctx context.Context, result *Stats) error
}
