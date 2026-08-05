// Package tasks tracks the long-running work a user asked for, so the UI can say what is
// happening rather than leaving them watching a spinner.
package tasks

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Status is where a task has got to.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

// Done reports whether the task has settled and will not change again.
func (s Status) Done() bool {
	return s == StatusSucceeded || s == StatusFailed
}

// Kind names what a task does. Kinds are part of the API, so they are spelled out rather than
// derived from a Go type name that might be renamed.
type Kind string

const (
	// KindRefreshKeys pulls a user's keys from every connected provider.
	KindRefreshKeys Kind = "refresh_keys"
	// KindRevokeKey deletes one key at its provider and then from sshark.
	KindRevokeKey Kind = "revoke_key"
)

// Entity is a unit of work a user can watch.
type Entity struct {
	ID          uuid.UUID `json:"id"`
	ClerkUserID string    `json:"clerk_user_id"`
	Kind        Kind      `json:"kind"`
	// DedupKey identifies the work requested. Two unfinished tasks with the same key for the
	// same user are the same request, and the database refuses the second.
	DedupKey string `json:"-"`
	Status   Status `json:"status"`
	// Progress and Total are counted in whatever unit the task chose. Total is 0 until the task
	// has worked out how much there is to do.
	Progress int `json:"progress"`
	Total    int `json:"total"`
	// Message describes the current step in words meant for the person watching.
	Message    *string         `json:"message,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
	Error      *string         `json:"error,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	StartedAt  *time.Time      `json:"started_at,omitempty"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
	UpdatedAt  time.Time       `json:"updated_at"`
}
