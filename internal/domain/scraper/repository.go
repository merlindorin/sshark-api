package scraper

import (
	"context"
)

// ProgressRepository manages scrape progress.
type ProgressRepository interface {
	// GetProgress returns the current progress for a provider.
	// Returns empty cursor if no progress exists.
	GetProgress(ctx context.Context, provider Provider) (*Progress, error)

	// SaveProgress saves the current progress for a provider.
	SaveProgress(ctx context.Context, progress *Progress) error
}
