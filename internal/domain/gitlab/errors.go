package gitlab

import (
	"errors"
)

var (
	// ErrUserNotFound is returned when a GitLab user cannot be found.
	ErrUserNotFound = errors.New("user not found")

	// ErrUserAlreadyExist is returned when attempting to create a user that already exists.
	ErrUserAlreadyExist = errors.New("user already exists")
)
