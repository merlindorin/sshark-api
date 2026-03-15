package v1

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/api/common"
	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
)

func GetKey(c *gin.Context, logger *zap.Logger, repo publickeys.Repository, id openapi_types.UUID) {
	ctx := c.Request.Context()

	key, err := repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, publickeys.ErrKeyNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
			return
		}
		logger.Error("failed to find key", zap.String("id", id.String()), zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	c.JSON(http.StatusOK, key)
}
