package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/api/common"
	"github.com/merlindorin/sshark-api/api/public"
	"github.com/merlindorin/sshark-api/internal/domain/sources"
)

func Stats(c *gin.Context, logger *zap.Logger, repo sources.Repository) {
	ctx := c.Request.Context()

	stats, err := repo.GetStats(ctx)
	if err != nil {
		logger.Error("failed to get stats", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	c.JSON(http.StatusOK, public.Statistics{
		TotalKeys:      stats.TotalKeys,
		TotalSshKeys:   stats.TotalSSHKeys,
		TotalGpgKeys:   stats.TotalGPGKeys,
		TotalUsernames: stats.TotalUsernames,
		TotalProviders: stats.TotalProviders,
	})
}
