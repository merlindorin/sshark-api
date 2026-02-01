package metrics

import (
	"context"
	"time"

	"github.com/merlindorin/sshark-api/internal/domain/stats"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

type Collector struct {
	statsRepo stats.Repository
	logger    *zap.Logger
	interval  time.Duration
}

func NewCollector(statsRepo stats.Repository, logger *zap.Logger, interval time.Duration) *Collector {
	return &Collector{
		statsRepo: statsRepo,
		logger:    logger,
		interval:  interval,
	}
}

func (c *Collector) Start(_ context.Context) error {
	meter := otel.Meter("sshark-api")

	_, err := meter.Int64ObservableGauge(
		"sshark.stats",
		metric.WithDescription("SSH keys statistics"),
		metric.WithInt64Callback(func(ctx context.Context, observer metric.Int64Observer) error {
			var s stats.Stats
			if err := c.statsRepo.GetStats(ctx, &s); err != nil {
				c.logger.Error("failed to fetch stats", zap.Error(err))
				return err
			}

			observer.Observe(int64(s.TotalKeys), metric.WithAttributes(
				attribute.String("metric", "total_keys"),
			))
			observer.Observe(int64(s.TotalUsernames), metric.WithAttributes(
				attribute.String("metric", "total_usernames"),
			))
			observer.Observe(int64(s.TotalProviders), metric.WithAttributes(
				attribute.String("metric", "total_providers"),
			))
			return nil
		}),
	)
	if err != nil {
		c.logger.Error("failed to register stats gauge", zap.Error(err))
		return err
	}

	return nil
}
