package v1

import (
	"fmt"
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/domain/sshkeys"
	"github.com/merlindorin/sshark-api/internal/infra/sshkeys/redis"

	"github.com/gin-gonic/gin"
)

func GetSSHKey(c *gin.Context, logger *zap.Logger, repo *redis.Repository, id openapi_types.UUID) {
	var key sshkeys.Entity

	err := repo.Get(c.Request.Context(), id, &key)
	if err != nil {
		logger.Error("failed to find ssh key", zap.String("id", id.String()), zap.Error(err))
		_ = c.Error(fmt.Errorf("failed to fetch sshkey: %w", err))
		return
	}

	c.JSON(http.StatusOK, key)
}
