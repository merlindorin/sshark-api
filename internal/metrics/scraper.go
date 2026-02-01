package metrics

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type ScraperMetrics struct {
	UsersProcessed  metric.Int64Counter
	UsersIngested   metric.Int64Counter
	IngestErrors    metric.Int64Counter
	FetchErrors     metric.Int64Counter
	ScrapeDuration  metric.Float64Histogram
	BatchSize       metric.Int64Histogram
	KeysPerUser     metric.Int64Histogram
	CurrentPosition metric.Int64ObservableGauge
	RateLimitWait   metric.Float64Histogram
}

func NewScraperMetrics(_ string) (*ScraperMetrics, error) {
	meter := otel.Meter("sshark-api")

	usersProcessed, err := meter.Int64Counter(
		"sshark.scraper.users.processed",
		metric.WithDescription("Total number of users processed by the scraper"),
		metric.WithUnit("{user}"),
	)
	if err != nil {
		return nil, err
	}

	usersIngested, err := meter.Int64Counter(
		"sshark.scraper.users.ingested",
		metric.WithDescription("Total number of users successfully ingested"),
		metric.WithUnit("{user}"),
	)
	if err != nil {
		return nil, err
	}

	ingestErrors, err := meter.Int64Counter(
		"sshark.scraper.errors.ingest",
		metric.WithDescription("Total number of ingest errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	fetchErrors, err := meter.Int64Counter(
		"sshark.scraper.errors.fetch",
		metric.WithDescription("Total number of API fetch errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	scrapeDuration, err := meter.Float64Histogram(
		"sshark.scraper.duration",
		metric.WithDescription("Duration of user ingest operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	batchSize, err := meter.Int64Histogram(
		"sshark.scraper.batch.size",
		metric.WithDescription("Number of users returned per API batch"),
		metric.WithUnit("{user}"),
	)
	if err != nil {
		return nil, err
	}

	keysPerUser, err := meter.Int64Histogram(
		"sshark.scraper.keys.per_user",
		metric.WithDescription("Distribution of SSH keys per user"),
		metric.WithUnit("{key}"),
	)
	if err != nil {
		return nil, err
	}

	rateLimitWait, err := meter.Float64Histogram(
		"sshark.scraper.ratelimit.wait",
		metric.WithDescription("Time spent waiting for rate limiter"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	return &ScraperMetrics{
		UsersProcessed: usersProcessed,
		UsersIngested:  usersIngested,
		IngestErrors:   ingestErrors,
		FetchErrors:    fetchErrors,
		ScrapeDuration: scrapeDuration,
		BatchSize:      batchSize,
		KeysPerUser:    keysPerUser,
		RateLimitWait:  rateLimitWait,
	}, nil
}

func (m *ScraperMetrics) RegisterPositionGauge(_ context.Context, _ string, positionFunc func() int64) error {
	meter := otel.Meter("sshark-api")

	_, err := meter.Int64ObservableGauge(
		"sshark.scraper.position",
		metric.WithDescription("Current scraper position (user ID or page number)"),
		metric.WithInt64Callback(func(_ context.Context, observer metric.Int64Observer) error {
			observer.Observe(positionFunc())
			return nil
		}),
	)

	return err
}
