package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/app/keyops"
	"github.com/merlindorin/sshark-api/internal/domain/tasks"
)

// maxWorkers bounds concurrent jobs. The ceiling is the providers' rate limits, not this
// number, so it is set for responsiveness rather than throughput.
const maxWorkers = 5

// Queue accepts work and runs it. It owns both sides so a caller enqueues and watches through
// one thing rather than knowing about River.
type Queue struct {
	client *river.Client[pgx.Tx]
	tasks  tasks.Repository
	logger *zap.Logger
}

// NewQueue builds the queue and registers the workers.
func NewQueue(
	logger *zap.Logger,
	pool *pgxpool.Pool,
	taskRepository tasks.Repository,
	keys *keyops.Service,
) (*Queue, error) {
	workers := river.NewWorkers()

	river.AddWorker(workers, &RefreshKeysWorker{Logger: logger, Keys: keys, Tasks: taskRepository})
	river.AddWorker(workers, &RevokeKeyWorker{Logger: logger, Keys: keys, Tasks: taskRepository})

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: maxWorkers}},
		Workers: workers,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create the job queue: %w", err)
	}

	return &Queue{client: client, tasks: taskRepository, logger: logger}, nil
}

// Start begins processing jobs.
func (q *Queue) Start(ctx context.Context) error {
	return q.client.Start(ctx)
}

// Stop finishes the jobs in flight and stops taking more.
func (q *Queue) Stop(ctx context.Context) error {
	return q.client.Stop(ctx)
}

// EnqueueRefresh queues a refresh for the user, returning the task they can watch.
func (q *Queue) EnqueueRefresh(ctx context.Context, subject string) (*tasks.Entity, error) {
	return q.enqueue(ctx, subject, string(tasks.KindRefreshKeys), 0, func(taskID uuid.UUID) river.JobArgs {
		return RefreshKeysArgs{TaskID: taskID, ClerkUserID: subject}
	}, tasks.KindRefreshKeys)
}

// EnqueueRevoke queues the revocation of one key, returning the task to watch. Revoking a
// different key is different work, so the key is part of what makes the request unique.
func (q *Queue) EnqueueRevoke(ctx context.Context, subject string, keyID uuid.UUID) (*tasks.Entity, error) {
	dedupKey := string(tasks.KindRevokeKey) + ":" + keyID.String()

	return q.enqueue(ctx, subject, dedupKey, revokeSteps, func(taskID uuid.UUID) river.JobArgs {
		return RevokeKeyArgs{TaskID: taskID, ClerkUserID: subject, KeyID: keyID}
	}, tasks.KindRevokeKey)
}

// enqueue records the task and queues the job behind it.
//
// Asking for the same work twice returns the task already doing it rather than starting a
// second one. The uniqueness is enforced by the database, so two requests arriving together
// cannot both win: whichever loses reads back the winner.
func (q *Queue) enqueue(
	ctx context.Context,
	subject string,
	dedupKey string,
	total int,
	buildArgs func(taskID uuid.UUID) river.JobArgs,
	kind tasks.Kind,
) (*tasks.Entity, error) {
	task := &tasks.Entity{
		ID:          uuid.New(),
		ClerkUserID: subject,
		Kind:        kind,
		DedupKey:    dedupKey,
		Status:      tasks.StatusPending,
		Total:       total,
		Message:     message("Queued"),
	}

	if err := q.tasks.Create(ctx, task); err != nil {
		if errors.Is(err, tasks.ErrTaskAlreadyRunning) {
			return q.tasks.FindActiveByDedupKey(ctx, subject, dedupKey)
		}
		return nil, err
	}

	if _, err := q.client.Insert(ctx, buildArgs(task.ID), nil); err != nil {
		// The task exists but nothing will run it, so settle it rather than leaving it pending
		// forever and blocking the next attempt on the uniqueness constraint.
		_ = q.tasks.Finish(ctx, task.ID, nil, fmt.Errorf("could not be queued: %w", err))
		return nil, err
	}

	return task, nil
}

// revokeSteps is how many stages a revocation reports, so the UI has a total before it starts.
const revokeSteps = 3

func message(text string) *string {
	return &text
}
