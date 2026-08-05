# SShark Metrics Catalog

This document provides a comprehensive reference for all metrics exported by the sshark-api service. Metrics are exposed via the `/metrics` endpoint in Prometheus format and can be scraped by Prometheus or any compatible monitoring system.

## Metric Naming Convention

All metrics follow OpenTelemetry naming conventions:
- **Counters**: Always end with `.total` (e.g., `api.requests.total`)
- **Histograms**: Use `.duration` for time-based measurements (e.g., `api.request.duration`)
- **Gauges**: Indicate current state (e.g., `db.connections.active`)

## API Metrics

These metrics track HTTP request patterns, authentication, and user operations.

### `api_requests_total`

**Type**: Counter
**Description**: Total number of API requests
**Labels**:
- `endpoint` - The request path (e.g., `/api/v1/search`, `/api/v1/keys`)
- `method` - HTTP method (GET, POST, PUT, DELETE, etc.)
- `status_code` - HTTP response status code (200, 404, 500, etc.)

**Example PromQL Queries**:
```promql
# Overall request rate
sum(rate(api_requests_total[5m]))

# Request rate by endpoint
sum by (endpoint, method) (rate(api_requests_total[5m]))

# Error rate (5xx responses)
sum(rate(api_requests_total{status_code=~"5.."}[5m])) / sum(rate(api_requests_total[5m]))

# 4xx error rate by endpoint
sum by (endpoint) (rate(api_requests_total{status_code=~"4.."}[5m]))
```

### `api_request_duration`

**Type**: Histogram
**Description**: Duration of API requests in seconds
**Buckets**: 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0 seconds
**Labels**:
- `endpoint` - The request path
- `method` - HTTP method

**Example PromQL Queries**:
```promql
# p95 response time
histogram_quantile(0.95, sum by (le) (rate(api_request_duration_bucket[5m])))

# p99 response time by endpoint
histogram_quantile(0.99, sum by (le, endpoint) (rate(api_request_duration_bucket[5m])))

# Average response time
rate(api_request_duration_sum[5m]) / rate(api_request_duration_count[5m])
```

### `api_auth_attempts`

**Type**: Counter
**Description**: Number of authentication attempts
**Labels**:
- `result` - Authentication result: `success`, `invalid`, `expired`, `revoked`, `missing_token`
- `auth_type` - Type of authentication: `session`, `api_key`

**Example PromQL Queries**:
```promql
# Authentication success rate
sum(rate(api_auth_attempts{result="success"}[5m])) / sum(rate(api_auth_attempts[5m]))

# Failed authentication rate by reason
sum by (result) (rate(api_auth_attempts{result!="success"}[5m]))
```

### `api_search_operations` (Not Yet Instrumented)

**Type**: Counter
**Description**: Number of search operations performed
**Labels**:
- `key_type` - Type of key searched: `ssh`, `gpg`

### `api_search_results` (Not Yet Instrumented)

**Type**: Histogram
**Description**: Number of results returned per search operation
**Buckets**: 0, 1, 5, 10, 25, 50, 100, 250, 500, 1000

### `api_user_operations` (Not Yet Instrumented)

**Type**: Counter
**Description**: User operations performed
**Labels**:
- `operation_type` - Type of operation: `profile`, `key`

## Job Queue Metrics

These metrics track the health and performance of the River job queue that processes background tasks.

### `job_queue_depth`

**Type**: Gauge
**Description**: Number of pending jobs waiting in the queue
**Labels**:
- `job_type` - Type of job (e.g., `key_refresh`, `key_revocation`)

**Example PromQL Queries**:
```promql
# Current queue depth
job_queue_depth

# Queue depth by job type
sum by (job_type) (job_queue_depth)
```

### `job_processing_duration`

**Type**: Histogram
**Description**: Time taken to process a job in seconds
**Buckets**: 0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0, 120.0, 300.0 seconds
**Labels**:
- `job_type` - Type of job

**Example PromQL Queries**:
```promql
# p95 job processing time
histogram_quantile(0.95, sum by (le, job_type) (rate(job_processing_duration_bucket[5m])))

# Average job duration by type
rate(job_processing_duration_sum[5m]) / rate(job_processing_duration_count[5m])
```

### `job_completed_total`

**Type**: Counter
**Description**: Number of successfully completed jobs
**Labels**:
- `job_type` - Type of job

**Example PromQL Queries**:
```promql
# Job completion rate (jobs per minute)
sum(rate(job_completed_total[5m])) * 60

# Completion rate by job type
sum by (job_type) (rate(job_completed_total[5m])) * 60
```

### `job_failed_total`

**Type**: Counter
**Description**: Number of failed jobs
**Labels**:
- `job_type` - Type of job
- `error_category` - Category of error (e.g., `network`, `database`, `validation`)

**Example PromQL Queries**:
```promql
# Job failure rate
sum(rate(job_failed_total[5m]))

# Failure rate by error category
sum by (error_category) (rate(job_failed_total[5m]))

# Job success rate
sum(rate(job_completed_total[5m])) / (sum(rate(job_completed_total[5m])) + sum(rate(job_failed_total[5m])))
```

### `job_workers_active` (Not Yet Instrumented)

**Type**: Gauge
**Description**: Number of workers currently processing jobs

### `job_workers_total` (Not Yet Instrumented)

**Type**: Gauge
**Description**: Total number of job workers available

### `job_age`

**Type**: Histogram
**Description**: Time jobs spend waiting in queue before processing (seconds)
**Buckets**: 1.0, 5.0, 10.0, 30.0, 60.0, 300.0, 600.0, 1800.0 seconds

**Example PromQL Queries**:
```promql
# p95 queue wait time
histogram_quantile(0.95, sum by (le) (rate(job_age_bucket[5m])))

# Jobs waiting more than 5 minutes
sum(rate(job_age_bucket{le="300"}[5m])) - sum(rate(job_age_bucket{le="+Inf"}[5m]))
```

## Database Metrics

These metrics track PostgreSQL database performance and connection pool health.

### `db_query_duration`

**Type**: Histogram
**Description**: Database query execution time in seconds
**Buckets**: 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5 seconds
**Labels**:
- `operation` - SQL operation type: `select`, `insert`, `update`, `delete`, `transaction`, `other`

**Example PromQL Queries**:
```promql
# p95 query latency by operation
histogram_quantile(0.95, sum by (le, operation) (rate(db_query_duration_bucket[5m])))

# Slow queries (>1 second)
sum by (operation) (rate(db_query_duration_count{le="+Inf"}[5m])) - sum by (operation) (rate(db_query_duration_count{le="1.0"}[5m]))

# Average query time
rate(db_query_duration_sum[5m]) / rate(db_query_duration_count[5m])
```

### `db_connections_active`

**Type**: Gauge
**Description**: Number of active database connections currently in use

**Example PromQL Queries**:
```promql
# Current active connections
db_connections_active

# Connection pool utilization
db_connections_active / (db_connections_active + db_connections_idle)
```

### `db_connections_idle`

**Type**: Gauge
**Description**: Number of idle database connections in the pool

### `db_connection_wait_time`

**Type**: Histogram
**Description**: Time spent waiting to acquire a database connection (seconds)
**Buckets**: 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0 seconds

**Example PromQL Queries**:
```promql
# p95 connection wait time
histogram_quantile(0.95, sum by (le) (rate(db_connection_wait_time_bucket[5m])))

# Connections waiting >100ms (potential pool exhaustion)
sum(rate(db_connection_wait_time_count{le="+Inf"}[5m])) - sum(rate(db_connection_wait_time_count{le="0.1"}[5m]))
```

### `db_transaction_duration`

**Type**: Histogram
**Description**: Database transaction duration in seconds
**Buckets**: 0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0 seconds
**Labels**:
- `operation` - Transaction type

**Example PromQL Queries**:
```promql
# p95 transaction duration
histogram_quantile(0.95, sum by (le) (rate(db_transaction_duration_bucket[5m])))
```

## SSH Key Scraping Metrics (Not Yet Instrumented)

These metrics will track SSH key scraping operations once the scraper service is instrumented.

### `scraping_requests_total` (Not Yet Instrumented)

**Type**: Counter
**Description**: Total number of scraping requests
**Labels**:
- `provider` - Provider name: `github`, `gitlab`
- `status` - Request status: `success`, `failure`

### `scraping_keys_discovered` (Not Yet Instrumented)

**Type**: Counter
**Description**: Number of SSH keys discovered per scraping operation
**Labels**:
- `provider` - Provider name: `github`, `gitlab`

### `scraping_duration` (Not Yet Instrumented)

**Type**: Histogram
**Description**: Duration of scraping operations in seconds
**Buckets**: 0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0 seconds
**Labels**:
- `provider` - Provider name
- `status` - Operation status

### `scraping_rate_limit_hits` (Not Yet Instrumented)

**Type**: Counter
**Description**: Number of times rate limits were hit
**Labels**:
- `provider` - Provider name

### `scraping_rate_limit_remaining` (Not Yet Instrumented)

**Type**: Gauge
**Description**: Remaining rate limit quota
**Labels**:
- `provider` - Provider name

### `scraping_errors_total` (Not Yet Instrumented)

**Type**: Counter
**Description**: Number of scraping errors
**Labels**:
- `provider` - Provider name
- `error_type` - Error type: `network`, `auth`, `api_limit`, `parsing`

## Dashboard Usage Guide

### Accessing the Dashboard

1. Import the dashboard JSON from `grafana/dashboards/sshark-overview.json` into your Grafana instance
2. Configure the Prometheus data source variable to point to your Prometheus server
3. The dashboard will automatically start displaying metrics

### Dashboard Sections

The dashboard is organized into collapsible row panels:

1. **System Overview**: High-level KPIs (request rate, error rate, connections, auth success)
2. **HTTP Metrics**: Request patterns, response times, errors, authentication
3. **Job Queue**: Queue depth, throughput, failures, processing times
4. **Database Performance**: Query latency, connection pool, slow queries
5. **SSH Key Operations** (future): Scraping metrics when instrumented

### Time Range Variables

The dashboard uses Grafana's built-in time range selector. Recommended ranges:
- **Real-time monitoring**: Last 15 minutes with 10-second refresh
- **Incident investigation**: Last 1-6 hours
- **Performance analysis**: Last 24 hours to 7 days

### Customization

All panels support filtering by label values. Use the Grafana query editor to:
- Filter by specific endpoints: `{endpoint="/api/v1/search"}`
- Filter by status codes: `{status_code=~"5.."}`
- Filter by job type: `{job_type="key_refresh"}`

## Troubleshooting Common Issues

### No Metrics Showing

**Problem**: Dashboard panels show "No data"

**Solutions**:
1. Verify the application is running and `/metrics` endpoint is accessible
2. Check Prometheus is scraping the application (check Prometheus targets)
3. Verify the Prometheus data source is configured correctly in Grafana
4. Check the time range - metrics may not exist for historical periods

### High Query Latency

**Problem**: `db_query_duration` p95 > 100ms

**Investigation**:
```promql
# Identify slow operation types
histogram_quantile(0.95, sum by (le, operation) (rate(db_query_duration_bucket[5m])))

# Count slow queries
sum by (operation) (rate(db_query_duration_count{le="+Inf"}[5m])) - sum by (operation) (rate(db_query_duration_count{le="0.1"}[5m]))
```

**Common Causes**:
- Missing database indexes
- Large result sets without pagination
- Lock contention
- Connection pool exhaustion (check `db_connection_wait_time`)

### Job Queue Backlog

**Problem**: `job_queue_depth` increasing over time

**Investigation**:
```promql
# Queue depth trend
job_queue_depth

# Job completion vs failure rate
sum(rate(job_completed_total[5m])) / (sum(rate(job_completed_total[5m])) + sum(rate(job_failed_total[5m])))

# Job processing time
histogram_quantile(0.95, sum by (le, job_type) (rate(job_processing_duration_bucket[5m])))
```

**Common Causes**:
- Jobs processing slower than they're being enqueued
- High failure rate preventing completion
- Insufficient worker capacity

### High Error Rate

**Problem**: `api_requests_total` error rate > 1%

**Investigation**:
```promql
# Error rate by endpoint
sum by (endpoint, status_code) (rate(api_requests_total{status_code=~"[45].."}[5m]))

# Top error status codes
topk(5, sum by (status_code) (rate(api_requests_total{status_code=~"[45].."}[5m])))
```

**Common Causes**:
- Database connection issues (check `db_connections_active`)
- Authentication failures (check `api_auth_attempts`)
- External API failures (once scraping metrics are instrumented)
- Application bugs (check logs)

### Connection Pool Exhaustion

**Problem**: `db_connections_active` near maximum, high `db_connection_wait_time`

**Investigation**:
```promql
# Connection utilization
db_connections_active / (db_connections_active + db_connections_idle)

# Wait time p95
histogram_quantile(0.95, sum by (le) (rate(db_connection_wait_time_bucket[5m])))
```

**Solutions**:
- Increase connection pool size
- Investigate slow queries holding connections
- Check for connection leaks (connections not being released)

## Alert Rule Examples

Here are recommended Prometheus alerting rules for critical metrics:

```yaml
groups:
  - name: sshark_alerts
    interval: 30s
    rules:
      # High error rate
      - alert: HighErrorRate
        expr: |
          sum(rate(api_requests_total{status_code=~"5.."}[5m]))
          / sum(rate(api_requests_total[5m])) > 0.05
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "High API error rate (> 5%)"
          description: "Error rate is {{ $value | humanizePercentage }}"

      # Slow response time
      - alert: SlowResponseTime
        expr: |
          histogram_quantile(0.95,
            sum by (le) (rate(api_request_duration_bucket[5m]))
          ) > 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "API p95 response time > 2s"

      # Job queue backlog
      - alert: JobQueueBacklog
        expr: job_queue_depth > 1000
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Job queue depth > 1000"
          description: "{{ $value }} jobs pending"

      # High job failure rate
      - alert: HighJobFailureRate
        expr: |
          sum(rate(job_failed_total[5m]))
          / (sum(rate(job_completed_total[5m])) + sum(rate(job_failed_total[5m]))) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Job failure rate > 10%"

      # Database connection pool near exhaustion
      - alert: DatabaseConnectionPoolExhaustion
        expr: |
          db_connections_active
          / (db_connections_active + db_connections_idle) > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Database connection pool > 90% utilized"

      # Slow database queries
      - alert: SlowDatabaseQueries
        expr: |
          histogram_quantile(0.95,
            sum by (le, operation) (rate(db_query_duration_bucket[5m]))
          ) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Database p95 query time > 1s"
          description: "{{ $labels.operation }} queries are slow"

      # Authentication failures
      - alert: HighAuthFailureRate
        expr: |
          sum(rate(api_auth_attempts{result!="success"}[5m]))
          / sum(rate(api_auth_attempts[5m])) > 0.2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Auth failure rate > 20%"
```

## Metric Collection Implementation

### Instrumentation Status

| Category | Instrumented | Notes |
|----------|--------------|-------|
| API Metrics | ✅ Partial | Request/duration/auth are instrumented; search/user ops pending |
| Job Queue | ✅ Full | All job metrics fully instrumented including depth gauge |
| Database | ✅ Full | All metrics instrumented via pgx tracer and DB stats collector |
| Scraping | ❌ Pending | Metrics defined but not yet called in scraper service |

### Adding Metrics to New Code

When adding new features, instrument them with appropriate metrics:

```go
import (
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

// Counter example
m.APIRequestsTotal.Add(ctx, 1,
    metric.WithAttributes(
        attribute.String("endpoint", "/api/v1/search"),
        attribute.String("method", "GET"),
        attribute.String("status_code", "200"),
    ),
)

// Histogram example
start := time.Now()
// ... do work ...
duration := time.Since(start).Seconds()
m.APIRequestDuration.Record(ctx, duration,
    metric.WithAttributes(
        attribute.String("endpoint", "/api/v1/search"),
        attribute.String("method", "GET"),
    ),
)

// Gauge example (asynchronous callback)
m.JobQueueDepth.Set(ctx, int64(queueSize),
    metric.WithAttributes(
        attribute.String("job_type", "key_refresh"),
    ),
)
```

## Performance Considerations

### Metric Cardinality

Keep label cardinality low to avoid Prometheus performance issues:

- ✅ **Good**: `{provider="github"}` (2-3 values)
- ✅ **Good**: `{status_code="200"}` (10-20 values)
- ❌ **Bad**: `{user_id="12345"}` (unbounded values)
- ❌ **Bad**: `{full_endpoint="/api/v1/users/12345/keys"}` (unbounded values)

Current metrics follow best practices with controlled label values.

### Histogram Bucket Design

Bucket boundaries are tailored to each metric's expected distribution:

- **Query latency**: Sub-millisecond to 2.5s (most queries are fast)
- **Request duration**: 10ms to 10s (API requests)
- **Job processing**: 100ms to 5 minutes (background work)
- **Queue wait time**: 1s to 30 minutes (backlog scenarios)

### Collection Overhead

Metrics collection has minimal performance impact:
- Counters: ~100ns per increment
- Histograms: ~500ns per observation
- Gauges: ~100ns per update

The DB stats collector runs every 10 seconds by default (configurable).
The job queue stats collector runs every 10 seconds by default (configurable).

## References

- [OpenTelemetry Go SDK Documentation](https://opentelemetry.io/docs/languages/go/)
- [Prometheus Query Language](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Grafana Documentation](https://grafana.com/docs/grafana/latest/)
- [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
