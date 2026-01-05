package validate

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/domain/query"
)

func MountV1(r gin.IRouter, logger *zap.Logger, explainer query.Validator) {
	r.GET("/:query", Validate(logger, explainer))
}
