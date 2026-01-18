package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func LivenessHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func ReadinessHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
