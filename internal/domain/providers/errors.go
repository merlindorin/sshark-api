package providers

import "errors"

var (
	// ErrKeyGone is returned when the provider no longer knows about the key.
	ErrKeyGone = errors.New("key does not exist at the provider")

	// ErrForbidden is returned when the delegated credential is not allowed to perform the
	// operation, typically because the OAuth grant is missing the required scope.
	ErrForbidden = errors.New("provider refused the operation")

	// ErrNoCredential is returned when no delegated credential is available for the user.
	ErrNoCredential = errors.New("no provider credential available")

	// ErrUnsupportedProvider is returned when sshark cannot act on the provider on behalf
	// of the user.
	ErrUnsupportedProvider = errors.New("provider not supported")
)
