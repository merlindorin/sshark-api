package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/merlindorin/sshark-api/api/common"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/domain/stats"
)

// Stats returns a handler that retrieves aggregated statistics.
func Stats(c *gin.Context, logger *zap.Logger, repo stats.Repository) {
	result := &stats.Stats{}

	if err := repo.GetStats(c.Request.Context(), result); err != nil {
		logger.Info("failed to get stats", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	c.JSON(http.StatusOK, result)
}
