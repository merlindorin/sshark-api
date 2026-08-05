// Package jobs runs a user's key operations in the background and keeps the task they are
// watching up to date.
//
// The work belongs here rather than in an HTTP handler because it can take minutes: a refresh
// waits out GitHub's rate limit, which resets on the hour, and no request should be held open
// that long.
package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/app/keyops"
	"github.com/merlindorin/sshark-api/internal/domain/tasks"
)

// RefreshKeysArgs refreshes every account a user has connected.
type RefreshKeysArgs struct {
	TaskID      uuid.UUID `json:"task_id"`
	ClerkUserID string    `json:"clerk_user_id"`
}

// Kind is the queue's name for this job.
func (RefreshKeysArgs) Kind() string { return string(tasks.KindRefreshKeys) }

// RevokeKeyArgs deletes one key at its provider and then from sshark.
type RevokeKeyArgs struct {
	TaskID      uuid.UUID `json:"task_id"`
	ClerkUserID string    `json:"clerk_user_id"`
	KeyID       uuid.UUID `json:"key_id"`
}

// Kind is the queue's name for this job.
func (RevokeKeyArgs) Kind() string { return string(tasks.KindRevokeKey) }

// RefreshKeysWorker runs a refresh.
type RefreshKeysWorker struct {
	river.WorkerDefaults[RefreshKeysArgs]

	Logger *zap.Logger
	Keys   *keyops.Service
	Tasks  tasks.Repository
}

func (w *RefreshKeysWorker) Work(ctx context.Context, job *river.Job[RefreshKeysArgs]) error {
	return run(ctx, w.Logger, w.Tasks, job.Args.TaskID, func(report keyops.Reporter) (any, error) {
		return w.Keys.Refresh(ctx, job.Args.ClerkUserID, report)
	})
}

// RevokeKeyWorker runs a revocation.
type RevokeKeyWorker struct {
	river.WorkerDefaults[RevokeKeyArgs]

	Logger *zap.Logger
	Keys   *keyops.Service
	Tasks  tasks.Repository
}

func (w *RevokeKeyWorker) Work(ctx context.Context, job *river.Job[RevokeKeyArgs]) error {
	return run(ctx, w.Logger, w.Tasks, job.Args.TaskID, func(report keyops.Reporter) (any, error) {
		return w.Keys.Revoke(ctx, job.Args.ClerkUserID, job.Args.KeyID, report)
	})
}

// run is the shape every job shares: mark the task running, report progress as the operation
// goes, and settle the task with a result or an error.
//
// The task is settled even when the operation fails, and the error is returned so River can
// retry. A retry marks the task running again, which is honest: it is being attempted again.
func run(
	ctx context.Context,
	logger *zap.Logger,
	repository tasks.Repository,
	taskID uuid.UUID,
	operation func(report keyops.Reporter) (any, error),
) error {
	if err := repository.MarkRunning(ctx, taskID); err != nil {
		return fmt.Errorf("cannot mark task running: %w", err)
	}

	report := keyops.Reporter(func(done, total int, message string) {
		if err := repository.ReportProgress(ctx, taskID, tasks.Progress{
			Done:    done,
			Total:   total,
			Message: message,
		}); err != nil {
			// Losing a progress update is not worth failing the work over.
			logger.Warn("cannot report task progress", zap.Error(err), zap.String("task_id", taskID.String()))
		}
	})

	result, opErr := operation(report)

	var encoded json.RawMessage
	if opErr == nil && result != nil {
		if marshalled, err := json.Marshal(result); err == nil {
			encoded = marshalled
		} else {
			logger.Warn("cannot encode task result", zap.Error(err))
		}
	}

	if err := repository.Finish(ctx, taskID, encoded, opErr); err != nil {
		logger.Error("cannot finish task", zap.Error(err), zap.String("task_id", taskID.String()))
	}

	return opErr
}
