package jobs_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/domain/profiles"
	"github.com/merlindorin/sshark-api/internal/domain/tasks"
	"github.com/merlindorin/sshark-api/internal/infra/jobs"
)

// profileAt builds a profile whose id sorts by n, so a fake can page the way Postgres does.
//
// The ids start at one: uuid.Nil is what a keyset walk passes to mean "from the beginning", so a
// profile sitting on it would be skipped. Real profiles are random v4 and never land there.
func profileAt(n int) profiles.Entity {
	var id uuid.UUID
	binary.BigEndian.PutUint16(id[14:], uint16(n)+1)

	return profiles.Entity{ID: id, ClerkUserID: fmt.Sprintf("user_%d", n)}
}

// fakeProfiles serves a fixed set of profiles with the keyset paging the worker relies on.
type fakeProfiles struct {
	profiles.Repository

	entities []profiles.Entity
	calls    int
	err      error
}

func (f *fakeProfiles) List(_ context.Context, after uuid.UUID, limit int) ([]profiles.Entity, error) {
	f.calls++

	if f.err != nil {
		return nil, f.err
	}

	page := make([]profiles.Entity, 0, limit)

	for _, entity := range f.entities {
		if bytes.Compare(entity.ID[:], after[:]) <= 0 {
			continue
		}

		page = append(page, entity)

		if len(page) == limit {
			break
		}
	}

	return page, nil
}

type enqueueFunc func(ctx context.Context, subject string) (*tasks.Entity, error)

func newWorker(repo profiles.Repository, enqueue enqueueFunc) *jobs.RefreshAllWorker {
	return &jobs.RefreshAllWorker{Logger: zap.NewNop(), Profiles: repo, Enqueue: enqueue}
}

// errNotQueued stands in where a test asserts nothing should have been queued at all.
var errNotQueued = errors.New("should not have been queued")

func work(t *testing.T, worker *jobs.RefreshAllWorker) error {
	t.Helper()

	return worker.Work(context.Background(), &river.Job[jobs.RefreshAllArgs]{})
}

func TestRefreshAllQueuesEveryProfileOnce(t *testing.T) {
	t.Parallel()

	// More than one batch, so a cursor that failed to advance would either loop or repeat.
	const count = 250

	entities := make([]profiles.Entity, 0, count)
	for i := range count {
		entities = append(entities, profileAt(i))
	}

	repo := &fakeProfiles{entities: entities}
	seen := make(map[string]int)

	err := work(t, newWorker(repo, func(_ context.Context, subject string) (*tasks.Entity, error) {
		seen[subject]++
		return &tasks.Entity{}, nil
	}))
	if err != nil {
		t.Fatalf("Work: %v", err)
	}

	if len(seen) != count {
		t.Errorf("queued %d distinct profiles, want %d", len(seen), count)
	}

	for subject, times := range seen {
		if times != 1 {
			t.Errorf("queued %s %d times, want once", subject, times)
		}
	}
}

func TestRefreshAllContinuesPastAProfileThatCannotBeQueued(t *testing.T) {
	t.Parallel()

	repo := &fakeProfiles{entities: []profiles.Entity{profileAt(1), profileAt(2), profileAt(3)}}
	queued := 0

	err := work(t, newWorker(repo, func(_ context.Context, subject string) (*tasks.Entity, error) {
		if subject == profileAt(2).ClerkUserID {
			return nil, errors.New("boom")
		}

		queued++

		return &tasks.Entity{}, nil
	}))
	if err != nil {
		t.Fatalf("Work: %v", err)
	}

	// The failure is logged and skipped: the profiles on either side still get their refresh.
	if queued != 2 {
		t.Errorf("queued %d profiles, want the 2 that did not fail", queued)
	}
}

func TestRefreshAllStopsWhenThereAreNoProfiles(t *testing.T) {
	t.Parallel()

	repo := &fakeProfiles{}

	err := work(t, newWorker(repo, func(context.Context, string) (*tasks.Entity, error) {
		t.Error("nothing should be queued when there are no profiles")
		return nil, errNotQueued
	}))
	if err != nil {
		t.Fatalf("Work: %v", err)
	}

	if repo.calls != 1 {
		t.Errorf("listed %d times, want a single empty page", repo.calls)
	}
}

func TestRefreshAllReportsAListingFailure(t *testing.T) {
	t.Parallel()

	repo := &fakeProfiles{err: errors.New("database is away")}

	err := work(t, newWorker(repo, func(context.Context, string) (*tasks.Entity, error) {
		t.Error("nothing should be queued when the profiles cannot be listed")
		return nil, errNotQueued
	}))
	// Returned rather than swallowed, so River retries the pass instead of silently skipping it.
	if err == nil {
		t.Fatal("Work returned no error when profiles could not be listed")
	}
}
