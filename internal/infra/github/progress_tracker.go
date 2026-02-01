package github

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const progressKey = "sshark:scraper:last_user_id"

type ProgressTracker struct {
	client *redis.Client
}

func NewProgressTracker(client *redis.Client) *ProgressTracker {
	return &ProgressTracker{client: client}
}

func (p *ProgressTracker) GetLastUserID(ctx context.Context) (int64, error) {
	val, err := p.client.Get(ctx, progressKey).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get last user ID from Redis: %w", err)
	}

	id, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse last user ID: %w", err)
	}

	return id, nil
}

func (p *ProgressTracker) SetLastUserID(ctx context.Context, id int64) error {
	err := p.client.Set(ctx, progressKey, id, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to set last user ID in Redis: %w", err)
	}

	return nil
}
