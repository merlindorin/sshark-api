package v1

import (
	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/api/public"
	"github.com/merlindorin/sshark-api/internal/domain/profiles"
	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
	"github.com/merlindorin/sshark-api/internal/domain/sources"
	"github.com/merlindorin/sshark-api/internal/infra/identity"
)

type Server struct {
	logger         *zap.Logger
	sourcesRepo    sources.Repository
	publickeysRepo publickeys.Repository
	profilesRepo   profiles.Repository
	identities     *identity.Resolver
}

//nolint:revive // method name from generated interface
func (s Server) GetPublicKeyById(c *gin.Context, id openapi_types.UUID) {
	GetKey(c, s.logger, s.publickeysRepo, id)
}

func (s Server) GetStats(c *gin.Context) {
	Stats(c, s.logger, s.sourcesRepo)
}

func (s Server) SearchSSHKeys(c *gin.Context, params public.SearchSSHKeysParams) {
	SearchSSHKeys(c, s.logger, s.sourcesRepo, s.publickeysRepo, params)
}

func (s Server) SearchGPGKeys(c *gin.Context, params public.SearchGPGKeysParams) {
	SearchGPGKeys(c, s.logger, s.sourcesRepo, s.publickeysRepo, params)
}

func (s Server) GetSourceByProviderAndUsername(
	c *gin.Context,
	provider public.GetSourceByProviderAndUsernameParamsProvider,
	username string,
) {
	GetSourceByProviderAndUsername(c, s.logger, s.sourcesRepo, s.publickeysRepo, provider, username)
}

func (s Server) ListSources(c *gin.Context, params public.ListSourcesParams) {
	ListSources(c, s.logger, s.sourcesRepo, params)
}

func (s Server) GetUserProfile(c *gin.Context, username string) {
	GetUserProfile(c, s.logger, s.profilesRepo, s.identities, s.sourcesRepo, s.publickeysRepo, username)
}

func NewServer(
	logger *zap.Logger,
	sourcesRepo sources.Repository,
	publickeysRepo publickeys.Repository,
	profilesRepo profiles.Repository,
	identities *identity.Resolver,
) *Server {
	return &Server{
		logger:         logger,
		sourcesRepo:    sourcesRepo,
		publickeysRepo: publickeysRepo,
		profilesRepo:   profilesRepo,
		identities:     identities,
	}
}
