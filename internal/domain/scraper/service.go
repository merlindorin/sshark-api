package scraper

import (
	"context"
)

// ScrapeResult contains the result of a scrape operation.
type ScrapeResult struct {
	Provider    Provider
	Username    string
	KeysAdded   int
	KeysUpdated int
	KeysRemoved int
	Error       error
}

// Service orchestrates scraping keys from providers and storing them.
type Service interface {
	// ScrapeUser scrapes keys for a single user from a provider.
	ScrapeUser(ctx context.Context, provider Provider, username string) (*ScrapeResult, error)

	// ScrapeUsers scrapes keys for multiple users from a provider.
	ScrapeUsers(ctx context.Context, provider Provider, usernames []string) ([]ScrapeResult, error)
}
