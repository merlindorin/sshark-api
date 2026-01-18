package v1

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Server struct {
	logger *zap.Logger
}

func (s Server) GetLiveness(c *gin.Context) {
	LivenessHandler(c)
}

func (s Server) GetReadiness(c *gin.Context) {
	ReadinessHandler(c)
}

func NewServer(logger *zap.Logger) *Server {
	return &Server{
		logger: logger,
	}
}
