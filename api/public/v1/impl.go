package v1

import (
	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/api/public"
	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
	"github.com/merlindorin/sshark-api/internal/domain/sources"
)

type Server struct {
	logger         *zap.Logger
	sourcesRepo    sources.Repository
	publickeysRepo publickeys.Repository
}

func (s Server) SearchKeys(c *gin.Context, params public.SearchKeysParams) {
	SearchKeys(c, s.logger, s.sourcesRepo, s.publickeysRepo, params)
}

//nolint:revive // method name from generated interface
func (s Server) GetSSHKeyById(c *gin.Context, id openapi_types.UUID) {
	GetKey(c, s.logger, s.publickeysRepo, id)
}

func (s Server) GetStats(c *gin.Context) {
	Stats(c, s.logger, s.publickeysRepo)
}

func NewServer(logger *zap.Logger, sourcesRepo sources.Repository, publickeysRepo publickeys.Repository) *Server {
	return &Server{
		logger:         logger,
		sourcesRepo:    sourcesRepo,
		publickeysRepo: publickeysRepo,
	}
}
