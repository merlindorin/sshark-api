package metrics

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// PgxQueryTracer implements pgx query tracing to record query duration metrics.
type PgxQueryTracer struct {
	metrics *Metrics
	logger  *zap.Logger
}

// NewPgxQueryTracer creates a new pgx query tracer that records query metrics.
func NewPgxQueryTracer(metrics *Metrics, logger *zap.Logger) *PgxQueryTracer {
	return &PgxQueryTracer{
		metrics: metrics,
		logger:  logger,
	}
}

// queryContextKey is used to store query metadata in context.
type queryContextKey struct{}

type queryContext struct {
	startTime time.Time
	sql       string
}

// TraceQueryStart is called at the beginning of Query, QueryRow, and Exec calls.
func (t *PgxQueryTracer) TraceQueryStart(
	ctx context.Context,
	_ *pgx.Conn,
	data pgx.TraceQueryStartData,
) context.Context {
	qctx := queryContext{
		startTime: time.Now(),
		sql:       data.SQL,
	}
	return context.WithValue(ctx, queryContextKey{}, qctx)
}

// TraceQueryEnd is called at the end of Query, QueryRow, and Exec calls.
func (t *PgxQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	qctx, ok := ctx.Value(queryContextKey{}).(queryContext)
	if !ok {
		return
	}

	duration := time.Since(qctx.startTime).Seconds()
	operation := extractOperation(qctx.sql)

	// Record query duration
	t.metrics.DBQueryDuration.Record(ctx, duration, WithOperation(operation))

	// Log slow queries (queries taking more than 1 second)
	if duration > 1.0 {
		t.logger.Warn("Slow database query detected",
			zap.String("operation", operation),
			zap.Float64("duration_seconds", duration),
			zap.String("sql", truncateSQL(qctx.sql)),
			zap.Error(data.Err),
		)
	}
}

// TraceBatchStart is called at the beginning of SendBatch calls.
func (t *PgxQueryTracer) TraceBatchStart(
	ctx context.Context,
	_ *pgx.Conn,
	_ pgx.TraceBatchStartData,
) context.Context {
	return context.WithValue(ctx, batchStartTimeKey{}, time.Now())
}

// TraceBatchEnd is called at the end of SendBatch calls.
func (t *PgxQueryTracer) TraceBatchEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceBatchEndData) {
	startTime, ok := ctx.Value(batchStartTimeKey{}).(time.Time)
	if !ok {
		return
	}

	duration := time.Since(startTime).Seconds()

	// Record batch query duration as a transaction
	t.metrics.DBTransactionDuration.Record(ctx, duration, WithOperation("batch"))

	if duration > 1.0 {
		t.logger.Warn("Slow database batch detected",
			zap.Float64("duration_seconds", duration),
			zap.Error(data.Err),
		)
	}
}

// TraceCopyFromStart is called at the beginning of CopyFrom calls.
func (t *PgxQueryTracer) TraceCopyFromStart(
	ctx context.Context,
	_ *pgx.Conn,
	_ pgx.TraceCopyFromStartData,
) context.Context {
	return context.WithValue(ctx, copyFromStartTimeKey{}, time.Now())
}

// TraceCopyFromEnd is called at the end of CopyFrom calls.
func (t *PgxQueryTracer) TraceCopyFromEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceCopyFromEndData) {
	startTime, ok := ctx.Value(copyFromStartTimeKey{}).(time.Time)
	if !ok {
		return
	}

	duration := time.Since(startTime).Seconds()

	// Record copy operation duration
	t.metrics.DBQueryDuration.Record(ctx, duration, WithOperation("copy_from"))

	t.logger.Debug("Database COPY FROM completed",
		zap.Float64("duration_seconds", duration),
		zap.Error(data.Err),
	)
}

// TracePrepareStart is called at the beginning of Prepare calls.
func (t *PgxQueryTracer) TracePrepareStart(
	ctx context.Context,
	_ *pgx.Conn,
	_ pgx.TracePrepareStartData,
) context.Context {
	return ctx
}

// TracePrepareEnd is called at the end of Prepare calls.
func (t *PgxQueryTracer) TracePrepareEnd(_ context.Context, _ *pgx.Conn, _ pgx.TracePrepareEndData) {
	// Prepare statements are not recorded separately, as they are typically fast
	// and the actual query execution is more important
}

// TraceConnectStart is called at the beginning of Connect and ConnectConfig calls.
func (t *PgxQueryTracer) TraceConnectStart(ctx context.Context, _ pgx.TraceConnectStartData) context.Context {
	return ctx
}

// TraceConnectEnd is called at the end of Connect and ConnectConfig calls.
func (t *PgxQueryTracer) TraceConnectEnd(_ context.Context, data pgx.TraceConnectEndData) {
	if data.Err != nil {
		t.logger.Error("Database connection failed",
			zap.Error(data.Err),
		)
	}
}

// Context keys for storing start times.
type batchStartTimeKey struct{}
type copyFromStartTimeKey struct{}

// extractOperation attempts to determine the operation type from the SQL query.
// Returns one of: select, insert, update, delete, or other.
func extractOperation(sql string) string {
	trimmed := strings.TrimSpace(strings.ToLower(sql))

	if strings.HasPrefix(trimmed, "select") {
		return "select"
	}
	if strings.HasPrefix(trimmed, "insert") {
		return "insert"
	}
	if strings.HasPrefix(trimmed, "update") {
		return "update"
	}
	if strings.HasPrefix(trimmed, "delete") {
		return "delete"
	}
	if strings.HasPrefix(trimmed, "begin") || strings.HasPrefix(trimmed, "commit") ||
		strings.HasPrefix(trimmed, "rollback") {
		return "transaction"
	}

	return "other"
}

// truncateSQL truncates SQL queries for logging to avoid excessively long log lines.
func truncateSQL(sql string) string {
	const maxLen = 200
	if len(sql) <= maxLen {
		return sql
	}
	return sql[:maxLen] + "..."
}
