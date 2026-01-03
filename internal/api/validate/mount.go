package validate

import (
	"github.com/gin-gonic/gin"

	"github.com/merlindorin/sshark-api/internal/domain/query"
)

func MountV1(r gin.IRouter, explainer query.Explainer) {
	r.GET("/:query", Validate(explainer))
}
