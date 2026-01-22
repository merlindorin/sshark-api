package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/merlindorin/sshark-api/api/public"
	"github.com/merlindorin/sshark-api/internal/domain/ingester"
	sshkeysrepository "github.com/merlindorin/sshark-api/internal/infra/sshkeys/redis"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

type Server struct {
	logger          *zap.Logger
	repository      *sshkeysrepository.Repository
	ingesterService *ingester.Service
}

func (s Server) SearchKeys(c *gin.Context, params public.SearchKeysParams) {
	SSHKeys(c, s.logger, s.repository, params)
}

//nolint:revive // method name from generated interface
func (s Server) GetSSHKeyById(c *gin.Context, id openapi_types.UUID) {
	GetSSHKey(c, s.logger, s.repository, id)
}

func (s Server) GetStats(c *gin.Context) {
	Stats(c, s.logger, s.repository)
}

func NewServer(logger *zap.Logger, srepo *sshkeysrepository.Repository, service *ingester.Service) *Server {
	return &Server{
		logger:          logger,
		repository:      srepo,
		ingesterService: service,
	}
}
