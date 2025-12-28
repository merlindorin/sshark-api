package sshkeys

import "errors"

// Domain errors for SSHkeys operations.
var (
	// ErrSSHKeyNotFound is returned when an SSH key cannot be found.
	ErrSSHKeyNotFound = errors.New("sshkey not found")
)
