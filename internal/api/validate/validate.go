package validate

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/merlindorin/sshark-api/internal/api/apierrors"
	"github.com/merlindorin/sshark-api/internal/domain/query"
)

type URI struct {
	Query string `uri:"query" binding:"required"`
}

func Validate(explainer query.Explainer) gin.HandlerFunc {
	return func(c *gin.Context) {
		uriParams := URI{}
		err := c.BindUri(&uriParams)
		if err != nil {
			_ = c.Error(apierrors.InvalidPathParamError(c))
			return
		}

		_, explainErr := explainer.ExplainQuery(c.Request.Context(), uriParams.Query)
		if explainErr != nil {
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
