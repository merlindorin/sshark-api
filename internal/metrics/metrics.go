package metrics

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	// Meter name for all sshark metrics.
	meterName = "github.com/merlindorin/sshark-api"
)

// Metrics holds all the application metrics instruments.
type Metrics struct {
	// SSH Key Scraping metrics
	ScrapingRequestsTotal   metric.Int64Counter
	ScrapingKeysDiscovered  metric.Int64Counter
	ScrapingDuration        metric.Float64Histogram
	ScrapingRateLimitHits   metric.Int64Counter
	ScrapingRateLimitRemain metric.Int64Gauge
	ScrapingErrors          metric.Int64Counter

	// Job Queue metrics
	JobQueueDepth     metric.Int64Gauge
	JobProcessingTime metric.Float64Histogram
	JobCompleted      metric.Int64Counter
	JobFailed         metric.Int64Counter
	JobWorkersActive  metric.Int64Gauge
	JobWorkersTotal   metric.Int64Gauge
	JobAge            metric.Float64Histogram

	// Database metrics
	DBQueryDuration       metric.Float64Histogram
	DBConnectionsActive   metric.Int64Gauge
	DBConnectionsIdle     metric.Int64Gauge
	DBConnectionWaitTime  metric.Float64Histogram
	DBTransactionDuration metric.Float64Histogram

	// API metrics
	APIRequestsTotal   metric.Int64Counter
	APIRequestDuration metric.Float64Histogram
	APIAuthAttempts    metric.Int64Counter
	APISearchOps       metric.Int64Counter
	APISearchResults   metric.Int64Histogram
	APIUserOps         metric.Int64Counter
}

// New creates and registers all application metrics.
func New() (*Metrics, error) {
	meter := otel.GetMeterProvider().Meter(meterName)
	m := &Metrics{}

	if err := initScrapingMetrics(meter, m); err != nil {
		return nil, err
	}

	if err := initJobMetrics(meter, m); err != nil {
		return nil, err
	}

	if err := initDatabaseMetrics(meter, m); err != nil {
		return nil, err
	}

	if err := initAPIMetrics(meter, m); err != nil {
		return nil, err
	}

	return m, nil
}

func initScrapingMetrics(meter metric.Meter, m *Metrics) error {
	var err error

	m.ScrapingRequestsTotal, err = meter.Int64Counter(
		"scraping_requests_total",
		metric.WithDescription("Total number of scraping requests by provider and status"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create scraping_requests_total: %w", err)
	}

	m.ScrapingKeysDiscovered, err = meter.Int64Counter(
		"scraping_keys_discovered",
		metric.WithDescription("Number of keys discovered per scraping operation by provider"),
		metric.WithUnit("{key}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create scraping_keys_discovered: %w", err)
	}

	m.ScrapingDuration, err = meter.Float64Histogram(
		"scraping_duration",
		metric.WithDescription("Duration of scraping operations by provider"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0),
	)
	if err != nil {
		return fmt.Errorf("failed to create scraping_duration: %w", err)
	}

	m.ScrapingRateLimitHits, err = meter.Int64Counter(
		"scraping_rate_limit_hits",
		metric.WithDescription("Number of rate limit hits by provider"),
		metric.WithUnit("{hit}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create scraping_rate_limit_hits: %w", err)
	}

	m.ScrapingRateLimitRemain, err = meter.Int64Gauge(
		"scraping_rate_limit_remaining",
		metric.WithDescription("Remaining rate limit quota by provider"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create scraping_rate_limit_remaining: %w", err)
	}

	m.ScrapingErrors, err = meter.Int64Counter(
		"scraping_errors_total",
		metric.WithDescription("Number of scraping errors by provider and error type"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create scraping_errors_total: %w", err)
	}

	return nil
}

func initJobMetrics(meter metric.Meter, m *Metrics) error {
	var err error

	m.JobQueueDepth, err = meter.Int64Gauge(
		"job_queue_depth",
		metric.WithDescription("Number of pending jobs by type"),
		metric.WithUnit("{job}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create job_queue_depth: %w", err)
	}

	m.JobProcessingTime, err = meter.Float64Histogram(
		"job_processing_duration",
		metric.WithDescription("Job processing time by job type"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0, 120.0, 300.0),
	)
	if err != nil {
		return fmt.Errorf("failed to create job_processing_duration: %w", err)
	}

	m.JobCompleted, err = meter.Int64Counter(
		"job_completed_total",
		metric.WithDescription("Number of completed jobs by type"),
		metric.WithUnit("{job}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create job_completed_total: %w", err)
	}

	m.JobFailed, err = meter.Int64Counter(
		"job_failed_total",
		metric.WithDescription("Number of failed jobs by type and error category"),
		metric.WithUnit("{job}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create job_failed_total: %w", err)
	}

	m.JobWorkersActive, err = meter.Int64Gauge(
		"job_workers_active",
		metric.WithDescription("Number of active job workers"),
		metric.WithUnit("{worker}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create job_workers_active: %w", err)
	}

	m.JobWorkersTotal, err = meter.Int64Gauge(
		"job_workers_total",
		metric.WithDescription("Total number of job workers"),
		metric.WithUnit("{worker}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create job_workers_total: %w", err)
	}

	m.JobAge, err = meter.Float64Histogram(
		"job_age",
		metric.WithDescription("Time jobs spend waiting in queue before processing"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(1.0, 5.0, 10.0, 30.0, 60.0, 300.0, 600.0, 1800.0),
	)
	if err != nil {
		return fmt.Errorf("failed to create job_age: %w", err)
	}

	return nil
}

func initDatabaseMetrics(meter metric.Meter, m *Metrics) error {
	var err error

	m.DBQueryDuration, err = meter.Float64Histogram(
		"db_query_duration",
		metric.WithDescription("Database query duration by operation type"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5),
	)
	if err != nil {
		return fmt.Errorf("failed to create db_query_duration: %w", err)
	}

	m.DBConnectionsActive, err = meter.Int64Gauge(
		"db_connections_active",
		metric.WithDescription("Number of active database connections"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create db_connections_active: %w", err)
	}

	m.DBConnectionsIdle, err = meter.Int64Gauge(
		"db_connections_idle",
		metric.WithDescription("Number of idle database connections"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create db_connections_idle: %w", err)
	}

	m.DBConnectionWaitTime, err = meter.Float64Histogram(
		"db_connection_wait_time",
		metric.WithDescription("Time spent waiting for a database connection"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0),
	)
	if err != nil {
		return fmt.Errorf("failed to create db_connection_wait_time: %w", err)
	}

	m.DBTransactionDuration, err = meter.Float64Histogram(
		"db_transaction_duration",
		metric.WithDescription("Database transaction duration by operation type"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0),
	)
	if err != nil {
		return fmt.Errorf("failed to create db_transaction_duration: %w", err)
	}

	return nil
}

func initAPIMetrics(meter metric.Meter, m *Metrics) error {
	var err error

	m.APIRequestsTotal, err = meter.Int64Counter(
		"api_requests_total",
		metric.WithDescription("Total number of API requests by endpoint and method"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create api_requests_total: %w", err)
	}

	m.APIRequestDuration, err = meter.Float64Histogram(
		"api_request_duration",
		metric.WithDescription("API request duration by endpoint"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0),
	)
	if err != nil {
		return fmt.Errorf("failed to create api_request_duration: %w", err)
	}

	m.APIAuthAttempts, err = meter.Int64Counter(
		"api_auth_attempts",
		metric.WithDescription("Number of authentication attempts by status (success/failure)"),
		metric.WithUnit("{attempt}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create api_auth_attempts: %w", err)
	}

	m.APISearchOps, err = meter.Int64Counter(
		"api_search_operations",
		metric.WithDescription("Number of search operations by key type"),
		metric.WithUnit("{operation}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create api_search_operations: %w", err)
	}

	m.APISearchResults, err = meter.Int64Histogram(
		"api_search_results",
		metric.WithDescription("Number of results returned per search operation"),
		metric.WithUnit("{result}"),
		metric.WithExplicitBucketBoundaries(0, 1, 5, 10, 25, 50, 100, 250, 500, 1000),
	)
	if err != nil {
		return fmt.Errorf("failed to create api_search_results: %w", err)
	}

	m.APIUserOps, err = meter.Int64Counter(
		"api_user_operations",
		metric.WithDescription("Number of user operations by operation type (profile management, key management)"),
		metric.WithUnit("{operation}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create api_user_operations: %w", err)
	}

	return nil
}

// Attribute helpers for common labels

// WithProvider returns an attribute for the provider (github, gitlab).
func WithProvider(provider string) metric.MeasurementOption {
	return metric.WithAttributes(attribute.String("provider", provider))
}

// WithStatus returns an attribute for operation status (success, failure).
func WithStatus(status string) metric.MeasurementOption {
	return metric.WithAttributes(attribute.String("status", status))
}

// WithErrorType returns an attribute for error categorization (network, auth, api_limit, parsing).
func WithErrorType(errorType string) metric.MeasurementOption {
	return metric.WithAttributes(attribute.String("error_type", errorType))
}

// WithJobType returns an attribute for job type.
func WithJobType(jobType string) metric.MeasurementOption {
	return metric.WithAttributes(attribute.String("job_type", jobType))
}

// WithOperation returns an attribute for database operation type.
func WithOperation(operation string) metric.MeasurementOption {
	return metric.WithAttributes(attribute.String("operation", operation))
}

// WithEndpoint returns an attribute for API endpoint.
func WithEndpoint(endpoint string) metric.MeasurementOption {
	return metric.WithAttributes(attribute.String("endpoint", endpoint))
}

// WithMethod returns an attribute for HTTP method.
func WithMethod(method string) metric.MeasurementOption {
	return metric.WithAttributes(attribute.String("method", method))
}

// WithKeyType returns an attribute for key type (ssh, gpg).
func WithKeyType(keyType string) metric.MeasurementOption {
	return metric.WithAttributes(attribute.String("key_type", keyType))
}

// WithErrorCategory returns an attribute for error category in job processing.
func WithErrorCategory(category string) metric.MeasurementOption {
	return metric.WithAttributes(attribute.String("error_category", category))
}

// WithOperationType returns an attribute for user operation type (profile, key).
func WithOperationType(opType string) metric.MeasurementOption {
	return metric.WithAttributes(attribute.String("operation_type", opType))
}
