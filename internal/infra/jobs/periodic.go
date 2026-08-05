package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/domain/profiles"
	"github.com/merlindorin/sshark-api/internal/domain/tasks"
)

// refreshInterval is how often every profile is refreshed unprompted.
//
// Keys change rarely, and a user who has just added one can refresh by hand, so this is set to
// keep the index from drifting rather than to notice a change quickly.
const refreshInterval = 24 * time.Hour

// refreshBatchSize bounds how many profiles are held at once while walking them all.
const refreshBatchSize = 100

// RefreshAllArgs refreshes every profile. It carries no fields: the work is the same whenever it
// runs, and the set of profiles is whatever exists at the time.
type RefreshAllArgs struct{}

// Kind is the queue's name for this job.
func (RefreshAllArgs) Kind() string { return "refresh_all" }

// RefreshAllWorker queues one refresh per profile.
//
// It fans out rather than refreshing inline so each user's refresh is an ordinary job: it shows
// up in their activity like one they asked for, retries on its own, and one account that is
// rate limited or unreachable holds up nobody else.
type RefreshAllWorker struct {
	river.WorkerDefaults[RefreshAllArgs]

	Logger   *zap.Logger
	Profiles profiles.Repository

	// Enqueue queues one user's refresh. It is a function rather than the Queue itself because
	// the Queue registers this worker while being built, so the worker cannot hold it.
	Enqueue func(ctx context.Context, subject string) (*tasks.Entity, error)
}

// Work walks every profile and queues a refresh for each.
//
// A user already being refreshed keeps the task they have: Enqueue returns the running one
// rather than starting a second, so a periodic pass never competes with a manual refresh.
func (w *RefreshAllWorker) Work(ctx context.Context, _ *river.Job[RefreshAllArgs]) error {
	after := uuid.Nil
	queued := 0
	failed := 0

	for {
		batch, err := w.Profiles.List(ctx, after, refreshBatchSize)
		if err != nil {
			return fmt.Errorf("cannot list profiles to refresh: %w", err)
		}

		if len(batch) == 0 {
			break
		}

		for i := range batch {
			after = batch[i].ID

			if _, enqueueErr := w.Enqueue(ctx, batch[i].ClerkUserID); enqueueErr != nil {
				// One profile that cannot be queued should not cost the rest their refresh.
				w.Logger.Warn("cannot queue periodic refresh",
					zap.String("profile_id", batch[i].ID.String()),
					zap.Error(enqueueErr))

				failed++

				continue
			}

			queued++
		}
	}

	w.Logger.Info("queued periodic refresh", zap.Int("queued", queued), zap.Int("failed", failed))

	return nil
}

// periodicRefresh describes the schedule the queue runs RefreshAllArgs on.
//
// River elects one leader across all running instances, so this fires once however many replicas
// are up. It does not run on start: a deploy should not trigger a full pass.
func periodicRefresh() *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(refreshInterval),
		func() (river.JobArgs, *river.InsertOpts) { return RefreshAllArgs{}, nil },
		&river.PeriodicJobOpts{ID: "refresh-all-profiles", RunOnStart: false},
	)
}
