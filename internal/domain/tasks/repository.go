package tasks

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// Progress is an update to a running task, reported as it works.
type Progress struct {
	// Done and Total are counted in the task's own unit.
	Done  int
	Total int
	// Message describes the current step for the person watching.
	Message string
}

// Repository stores the tasks a user can watch.
type Repository interface {
	// Create records a newly queued task. It returns ErrTaskAlreadyRunning when the user already
	// has an unfinished task for the same work.
	Create(ctx context.Context, entity *Entity) error

	// FindActiveByDedupKey returns the user's unfinished task for this work, or ErrTaskNotFound.
	FindActiveByDedupKey(ctx context.Context, clerkUserID string, dedupKey string) (*Entity, error)

	// Get returns a task, or ErrTaskNotFound.
	Get(ctx context.Context, id uuid.UUID) (*Entity, error)

	// ListByUser returns a user's most recent tasks, newest first.
	ListByUser(ctx context.Context, clerkUserID string, limit int) ([]Entity, error)

	// MarkRunning moves a task out of the queue and records when it started.
	MarkRunning(ctx context.Context, id uuid.UUID) error

	// ReportProgress updates how far along a running task is.
	ReportProgress(ctx context.Context, id uuid.UUID, progress Progress) error

	// Finish settles a task. A nil taskErr means it succeeded.
	Finish(ctx context.Context, id uuid.UUID, result json.RawMessage, taskErr error) error
}
