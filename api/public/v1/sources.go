package v1

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/api/common"
	"github.com/merlindorin/sshark-api/api/public"
	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
	"github.com/merlindorin/sshark-api/internal/domain/sources"
)

const (
	sourceCacheControl     = "public, max-age=300, s-maxage=3600, stale-while-revalidate=86400"
	sourceListCacheControl = "public, max-age=60, s-maxage=300, stale-while-revalidate=600"
	defaultSourceListLimit = 12
	maxSourceListLimit     = 100
)

func ListSources(
	c *gin.Context,
	logger *zap.Logger,
	sourcesRepo sources.Repository,
	params public.ListSourcesParams,
) {
	limit := defaultSourceListLimit
	if params.Limit != nil {
		limit = *params.Limit
	}
	if limit < 1 {
		limit = defaultSourceListLimit
	}
	if limit > maxSourceListLimit {
		limit = maxSourceListLimit
	}

	result, err := sourcesRepo.List(c.Request.Context(), limit, 0)
	if err != nil {
		logger.Error("failed to list sources", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	entities := make([]public.SourceSummary, 0, len(result.Entities))
	for _, e := range result.Entities {
		entities = append(entities, public.SourceSummary{
			Id:        e.ID,
			Provider:  e.Provider,
			UserId:    e.UserID,
			Username:  e.Username,
			Uri:       e.URI,
			CreatedAt: e.CreatedAt,
		})
	}

	c.Header("Cache-Control", sourceListCacheControl)
	c.JSON(http.StatusOK, public.SourceListResponse{
		Entities: entities,
		Total:    result.Total,
	})
}

func isSupportedSourceProvider(p public.GetSourceByProviderAndUsernameParamsProvider) bool {
	switch p {
	case public.Github, public.Gitlab:
		return true
	default:
		return false
	}
}

func GetSourceByProviderAndUsername(
	c *gin.Context,
	logger *zap.Logger,
	sourcesRepo sources.Repository,
	publickeysRepo publickeys.Repository,
	provider public.GetSourceByProviderAndUsernameParamsProvider,
	username string,
) {
	ctx := c.Request.Context()
	if !isSupportedSourceProvider(provider) || username == "" {
		_ = c.Error(common.InvalidPathParamError(c))
		return
	}
	providerStr := string(provider)

	source, err := sourcesRepo.GetByProviderAndUsername(ctx, providerStr, username)
	if err != nil {
		if errors.Is(err, sources.ErrSourceNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
			return
		}
		logger.Error("failed to get source",
			zap.String("provider", providerStr),
			zap.String("username", username),
			zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	sshEntities, err := publickeysRepo.ListBySourceID(ctx, source.ID, publickeys.KeyTypeSSH)
	if err != nil {
		logger.Error("failed to list SSH keys for source",
			zap.String("source_id", source.ID.String()), zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	gpgEntities, err := publickeysRepo.ListBySourceID(ctx, source.ID, publickeys.KeyTypeGPG)
	if err != nil {
		logger.Error("failed to list GPG keys for source",
			zap.String("source_id", source.ID.String()), zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	sshKeys := make([]common.SSHPublicKey, 0, len(sshEntities))
	for _, entity := range sshEntities {
		sshKeys = append(sshKeys, entityToSSHPublicKey(entity, source))
	}

	gpgKeys := make([]common.GPGPublicKey, 0, len(gpgEntities))
	for _, entity := range gpgEntities {
		gpgKeys = append(gpgKeys, entityToGPGPublicKey(entity, source))
	}

	c.Header("Cache-Control", sourceCacheControl)
	c.JSON(http.StatusOK, public.SourceDetail{
		Id:        source.ID,
		Provider:  source.Provider,
		UserId:    source.UserID,
		Username:  source.Username,
		Uri:       source.URI,
		CreatedAt: source.CreatedAt,
		UpdatedAt: source.UpdatedAt,
		SshKeys:   sshKeys,
		GpgKeys:   gpgKeys,
	})
}
