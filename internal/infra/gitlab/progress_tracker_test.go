package gitlab_test

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/merlindorin/sshark-api/internal/infra/gitlab"
)

func TestProgressTracker_GetLastPage(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{UnstableResp3: true})
	ctx := context.Background()

	// Clean up
	defer rdb.Del(ctx, "sshark:scraper:gitlab:last_page")

	tracker := gitlab.NewProgressTracker(rdb)

	// First call should return 1 (no previous progress)
	page, err := tracker.GetLastPage(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, page)
}

func TestProgressTracker_SetAndGetLastPage(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{UnstableResp3: true})
	ctx := context.Background()

	// Clean up
	defer rdb.Del(ctx, "sshark:scraper:gitlab:last_page")

	tracker := gitlab.NewProgressTracker(rdb)

	// Set page to 42
	err := tracker.SetLastPage(ctx, 42)
	require.NoError(t, err)

	// Get page should return 42
	page, err := tracker.GetLastPage(ctx)
	require.NoError(t, err)
	assert.Equal(t, 42, page)
}

func TestProgressTracker_Resume(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{UnstableResp3: true})
	ctx := context.Background()

	// Clean up
	defer rdb.Del(ctx, "sshark:scraper:gitlab:last_page")

	tracker := gitlab.NewProgressTracker(rdb)

	// Simulate scraper saving progress
	err := tracker.SetLastPage(ctx, 5)
	require.NoError(t, err)

	// Simulate scraper restart - should resume from page 5
	page, err := tracker.GetLastPage(ctx)
	require.NoError(t, err)
	assert.Equal(t, 5, page, "Should resume from saved page")

	// Continue scraping
	err = tracker.SetLastPage(ctx, 6)
	require.NoError(t, err)

	// Verify new page is saved
	page, err = tracker.GetLastPage(ctx)
	require.NoError(t, err)
	assert.Equal(t, 6, page)
}
