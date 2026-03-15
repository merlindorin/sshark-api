package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/api/common"
	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
)

type StatsResponse struct {
	TotalKeys int `json:"total_keys"`
}

// Stats returns aggregated statistics.
func Stats(c *gin.Context, logger *zap.Logger, repo publickeys.Repository) {
	ctx := c.Request.Context()

	// Get total count via search with no filter
	result, err := repo.Search(ctx, publickeys.SearchFilter{}, 0, 0)
	if err != nil {
		logger.Error("failed to get stats", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	c.JSON(http.StatusOK, StatsResponse{
		TotalKeys: result.Total,
	})
}
