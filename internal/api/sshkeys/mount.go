package sshkeys

import (
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/infra/sshkeys/redis"

	"github.com/gin-gonic/gin"
)

func MountV1(r gin.IRouter, logger *zap.Logger, repo *redis.Repository) {
	r.GET("/:id", GetSSHKey(logger, repo))
}
