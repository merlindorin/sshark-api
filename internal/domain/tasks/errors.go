package tasks

import "errors"

var (
	// ErrTaskNotFound is returned when no task matches the lookup.
	ErrTaskNotFound = errors.New("task not found")

	// ErrTaskAlreadyRunning is returned when the user already has this kind of task in flight.
	ErrTaskAlreadyRunning = errors.New("a task of this kind is already running")
)
