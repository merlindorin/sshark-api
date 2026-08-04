package profiles

import "errors"

var (
	// ErrProfileNotFound is returned when no profile matches the lookup.
	ErrProfileNotFound = errors.New("profile not found")

	// ErrUsernameTaken is returned when the username already belongs to someone else.
	ErrUsernameTaken = errors.New("username already taken")

	// ErrUsernameReserved is returned when the username collides with a sshark route or a name
	// kept back for the platform.
	ErrUsernameReserved = errors.New("username is reserved")

	// ErrUsernameInvalid is returned when the username does not match the allowed format.
	ErrUsernameInvalid = errors.New("username is not valid")
)
