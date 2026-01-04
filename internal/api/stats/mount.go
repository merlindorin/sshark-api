package stats

import (
	"github.com/gin-gonic/gin"
)

func MountV1(r gin.IRouter, repo Repository) {
	r.GET("", GetStats(repo))
}
