package search

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/domain/ingester"
	"github.com/merlindorin/sshark-api/internal/domain/query"
	"github.com/merlindorin/sshark-api/internal/domain/sshkeys"
)

func MountV1(
	r gin.IRouter,
	l *zap.Logger,
	srepo sshkeys.Repository,
	explainer query.Explainer,
	service *ingester.Service,
) {
	r.GET("/:query", SSHKeys(l, srepo, explainer, service))
}
