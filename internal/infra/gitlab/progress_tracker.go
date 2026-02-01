package gitlab

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const progressKey = "sshark:scraper:gitlab:last_page"

type ProgressTracker struct {
	client *redis.Client
}

func NewProgressTracker(client *redis.Client) *ProgressTracker {
	return &ProgressTracker{client: client}
}

func (p *ProgressTracker) GetLastPage(ctx context.Context) (int, error) {
	val, err := p.client.Get(ctx, progressKey).Result()
	if errors.Is(err, redis.Nil) {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get last page from Redis: %w", err)
	}

	page, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("failed to parse last page: %w", err)
	}

	return page, nil
}

func (p *ProgressTracker) SetLastPage(ctx context.Context, page int) error {
	err := p.client.Set(ctx, progressKey, page, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to set last page in Redis: %w", err)
	}

	return nil
}
