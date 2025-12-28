package sshkeys

import (
	"fmt"
	"net/http"

	"github.com/merlindorin/sshark-api/internal/domain/sshkeys"
	"github.com/merlindorin/sshark-api/internal/infra/sshkeys/redis"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type GetSSHKeyURIParams struct {
	ID string `uri:"id" binding:"required,uuid"`
}

func GetSSHKey(repo *redis.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		uriParams := GetSSHKeyURIParams{}
		err := c.BindUri(&uriParams)
		if err != nil {
			_ = c.Error(fmt.Errorf("failed to parse uri params: %w", err))
			return
		}

		var key sshkeys.Entity

		err = repo.Get(c.Request.Context(), uuid.MustParse(uriParams.ID), &key)
		if err != nil {
			_ = c.Error(fmt.Errorf("failed to fetch sshkey: %w", err))
			return
		}

		c.JSON(http.StatusOK, key)
	}
}
