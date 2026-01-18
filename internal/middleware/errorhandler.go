package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/merlindorin/sshark-api/api/common"
	"github.com/merlindorin/sshark-api/internal/api"
	"go.uber.org/zap"
)

// ErrorHandler captures errors and returns a consistent JSON error response.
func ErrorHandler(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			var httpError *api.APIError
			if ok := errors.As(err, &httpError); ok {
				c.JSON(httpError.StatusCode, httpError)
				return
			}

			logger.Error("Uncatched error in request", zap.Error(err))
			c.JSON(http.StatusInternalServerError, common.InternalError(c))
		}
	}
}
