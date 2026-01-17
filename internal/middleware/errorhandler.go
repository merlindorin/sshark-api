package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/api/apierrors"
)

// ErrorHandler captures errors and returns a consistent JSON error response.
func ErrorHandler(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			var httpError *apierrors.APIError
			if ok := errors.As(err, &httpError); ok {
				c.JSON(httpError.StatusCode, httpError)
				return
			}

			logger.Error("Uncatched error in request", zap.Error(err))
			c.JSON(http.StatusInternalServerError, apierrors.InternalError(c))
		}
	}
}
