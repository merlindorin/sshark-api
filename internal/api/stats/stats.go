package stats

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/api/apierrors"
	"github.com/merlindorin/sshark-api/internal/domain/stats"
)

// Repository defines the interface for retrieving statistics.
type Repository interface {
	GetStats(ctx context.Context, result *stats.Stats) error
}

// GetStats returns a handler that retrieves aggregated statistics.
func GetStats(logger *zap.Logger, repo Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		result := &stats.Stats{}
		if err := repo.GetStats(c.Request.Context(), result); err != nil {
			logger.Info("failed to get stats", zap.Error(err))
			_ = c.Error(apierrors.InternalError(c))
			return
		}

		c.JSON(http.StatusOK, result)
	}
}
