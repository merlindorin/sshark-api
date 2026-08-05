package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/merlindorin/sshark-api/internal/metrics"
)

// Metrics returns a middleware that records API request metrics.
func Metrics(m *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics after request completes
		duration := time.Since(start).Seconds()
		path := c.FullPath()
		method := c.Request.Method
		status := c.Writer.Status()

		ctx := c.Request.Context()

		// Record request count
		m.APIRequestsTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("endpoint", path),
				attribute.String("method", method),
				attribute.Int("status", status),
			),
		)

		// Record request duration
		m.APIRequestDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("endpoint", path),
				attribute.String("method", method),
				attribute.Int("status", status),
			),
		)
	}
}
