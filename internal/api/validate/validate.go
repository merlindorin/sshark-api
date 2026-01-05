package validate

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/api/apierrors"
	"github.com/merlindorin/sshark-api/internal/domain/query"
)

type URI struct {
	Query string `uri:"query" binding:"required"`
}

func Validate(logger *zap.Logger, explainer query.Validator) gin.HandlerFunc {
	return func(c *gin.Context) {
		uriParams := URI{}
		err := c.BindUri(&uriParams)
		if err != nil {
			logger.Info("failed to parse uri params", zap.Error(err))
			_ = c.Error(apierrors.InvalidPathParamError(c))
			return
		}

		_, err = explainer.ValidateQuery(c.Request.Context(), uriParams.Query)
		if err != nil {
			logger.Info("failed to explain query", zap.String("query", uriParams.Query), zap.Error(err))
			_ = c.Error(
				apierrors.InvalidSearchQueryError(
					c,
					err,
					uriParams.Query,
					[]string{"merlindorin", "@username:merlindorin", "@key:{XXX}"},
				),
			)
			return
		}

		c.JSON(http.StatusNoContent, nil)
	}
}
