package github

import (
	"errors"
)

// Domain errors for GitHub user operations.
var (
	// ErrUserNotFound is returned when a GitHub user cannot be found.
	ErrUserNotFound = errors.New("user not found")

	// ErrUserAlreadyExist is returned when attempting to create a user that already exists.
	ErrUserAlreadyExist = errors.New("user already exists")
)
