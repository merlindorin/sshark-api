package probe

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Probe struct{}

func MountProbe(r gin.IRouter) *Probe {
	p := &Probe{}

	r.GET("/liveness", p.LivenessHandler)
	r.GET("/readiness", p.LivenessHandler)

	return p
}

func (s *Probe) LivenessHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func (s *Probe) ReadinessHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
