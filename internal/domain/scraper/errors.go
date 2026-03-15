package scraper

import "errors"

var (
	// ErrUserNotFound is returned when a user is not found on the provider.
	ErrUserNotFound = errors.New("user not found")

	// ErrRateLimited is returned when the provider rate limits the request.
	ErrRateLimited = errors.New("rate limited")

	// ErrProviderUnavailable is returned when the provider is unavailable.
	ErrProviderUnavailable = errors.New("provider unavailable")
)
