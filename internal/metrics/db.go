package metrics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// DBStatsCollector periodically reads pgxpool stats and records them to metrics.
type DBStatsCollector struct {
	pool     *pgxpool.Pool
	metrics  *Metrics
	logger   *zap.Logger
	interval time.Duration
	stopCh   chan struct{}
}

// NewDBStatsCollector creates a new database stats collector that will poll stats
// at the specified interval and record them to metrics.
func NewDBStatsCollector(
	pool *pgxpool.Pool,
	metrics *Metrics,
	logger *zap.Logger,
	interval time.Duration,
) *DBStatsCollector {
	return &DBStatsCollector{
		pool:     pool,
		metrics:  metrics,
		logger:   logger,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the periodic collection of database stats.
func (c *DBStatsCollector) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Collect immediately on start
	c.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("DB stats collector stopped due to context cancellation")
			return
		case <-c.stopCh:
			c.logger.Info("DB stats collector stopped")
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

// Stop gracefully stops the stats collector.
func (c *DBStatsCollector) Stop() {
	close(c.stopCh)
}

// collect reads the current database connection pool stats and records them to metrics.
func (c *DBStatsCollector) collect(ctx context.Context) {
	stats := c.pool.Stat()

	// Record connection pool metrics
	c.metrics.DBConnectionsActive.Record(ctx, int64(stats.AcquiredConns()))
	c.metrics.DBConnectionsIdle.Record(ctx, int64(stats.IdleConns()))

	// Log stats for debugging
	c.logger.Debug("DB stats collected",
		zap.Int32("total_conns", stats.TotalConns()),
		zap.Int32("acquired_conns", stats.AcquiredConns()),
		zap.Int32("idle_conns", stats.IdleConns()),
		zap.Int64("acquire_count", stats.AcquireCount()),
		zap.Duration("acquire_duration", stats.AcquireDuration()),
		zap.Int64("canceled_acquire_count", stats.CanceledAcquireCount()),
		zap.Int32("constructing_conns", stats.ConstructingConns()),
		zap.Int64("empty_acquire_count", stats.EmptyAcquireCount()),
		zap.Int32("max_conns", stats.MaxConns()),
	)

	// Record connection wait time if there have been any acquires
	if stats.AcquireCount() > 0 {
		// Average acquire duration in seconds
		avgWaitTime := stats.AcquireDuration().Seconds() / float64(stats.AcquireCount())
		c.metrics.DBConnectionWaitTime.Record(ctx, avgWaitTime)
	}
}
