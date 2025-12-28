package sshkeys

import (
	"github.com/merlindorin/sshark-api/internal/infra/sshkeys/redis"

	"github.com/gin-gonic/gin"
)

func MountV1(r gin.IRouter, repo *redis.Repository) {
	r.GET("/:id", GetSSHKey(repo))
}
