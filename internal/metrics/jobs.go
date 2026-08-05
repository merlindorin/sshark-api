package metrics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// JobQueueStatsCollector periodically reads River queue stats and records them to metrics.
type JobQueueStatsCollector struct {
	pool     *pgxpool.Pool
	metrics  *Metrics
	logger   *zap.Logger
	interval time.Duration
	stopCh   chan struct{}
}

// NewJobQueueStatsCollector creates a new job queue stats collector that will poll stats
// at the specified interval and record them to metrics.
func NewJobQueueStatsCollector(
	pool *pgxpool.Pool,
	metrics *Metrics,
	logger *zap.Logger,
	interval time.Duration,
) *JobQueueStatsCollector {
	return &JobQueueStatsCollector{
		pool:     pool,
		metrics:  metrics,
		logger:   logger,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the periodic collection of job queue stats.
func (c *JobQueueStatsCollector) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Collect immediately on start
	c.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Job queue stats collector stopped due to context cancellation")
			return
		case <-c.stopCh:
			c.logger.Info("Job queue stats collector stopped")
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

// Stop gracefully stops the stats collector.
func (c *JobQueueStatsCollector) Stop() {
	close(c.stopCh)
}

// collect reads the current job queue stats and records them to metrics.
func (c *JobQueueStatsCollector) collect(ctx context.Context) {
	// Query pending jobs count (queue depth)
	var queueDepth int64
	err := c.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM river_job
		WHERE state IN ('available', 'scheduled', 'retryable')
	`).Scan(&queueDepth)
	if err != nil {
		c.logger.Warn("Failed to query queue depth", zap.Error(err))
	} else {
		c.metrics.JobQueueDepth.Record(ctx, queueDepth)
	}

	// Query active workers (jobs currently running)
	var activeWorkers int64
	err = c.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM river_job
		WHERE state = 'running'
	`).Scan(&activeWorkers)
	if err != nil {
		c.logger.Warn("Failed to query active workers", zap.Error(err))
	} else {
		c.metrics.JobWorkersActive.Record(ctx, activeWorkers)
	}

	// Record total workers (configured max workers)
	// This is a constant based on the queue configuration
	c.metrics.JobWorkersTotal.Record(ctx, 5) // maxWorkers from queue.go

	c.logger.Debug("Job queue stats collected",
		zap.Int64("queue_depth", queueDepth),
		zap.Int64("active_workers", activeWorkers),
	)
}
