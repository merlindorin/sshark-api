package validate

import (
	"github.com/gin-gonic/gin"
)

func MountV1(r gin.IRouter) {
	r.GET("/:query", Validate())
}
