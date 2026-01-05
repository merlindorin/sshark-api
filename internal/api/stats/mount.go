package stats

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func MountV1(r gin.IRouter, logger *zap.Logger, repo Repository) {
	r.GET("", GetStats(logger, repo))
}
