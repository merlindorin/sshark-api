package v1

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/api/common"
	"github.com/merlindorin/sshark-api/api/public"
	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
	"github.com/merlindorin/sshark-api/internal/domain/sources"
)

func defaultValue[T any](value *T, defaultValue T) T {
	if value == nil {
		return defaultValue
	}
	return *value
}

func SearchKeys(
	c *gin.Context,
	logger *zap.Logger,
	sourcesRepo sources.Repository,
	publickeysRepo publickeys.Repository,
	params public.SearchKeysParams,
) {
	ctx := c.Request.Context()
	searchStart := time.Now()

	limit := defaultValue(params.Limit, 10)
	offset := defaultValue(params.Offset, 0)

	// Build filter from query params
	filter := publickeys.SearchFilter{}

	// If username is provided, find the source first
	if params.Query != "" {
		source, err := sourcesRepo.GetByProviderAndUsername(ctx, "github", params.Query)
		if err == nil {
			filter.SourceID = &source.ID
		}
		// If not found, we'll return empty results
	}

	result, err := publickeysRepo.Search(ctx, filter, limit, offset)
	if err != nil {
		logger.Error("failed to search keys", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	// Convert to API response format
	searchResult := make([]common.SSHKey, 0, len(result.Entities))
	for _, entity := range result.Entities {
		// Get source info for this key
		source, sourceErr := sourcesRepo.Get(ctx, entity.SourceID)
		if sourceErr != nil {
			logger.Warn("failed to get source for key",
				zap.String("key_id", entity.ID.String()), zap.Error(sourceErr))
			continue
		}

		key := common.SSHKey{
			Id:        entity.ID,
			Key:       entity.KeyData,
			Provider:  source.Provider,
			Source:    source.URI,
			UpdatedAt: entity.UpdatedAt,
			Username:  source.Username,
		}

		if entity.SSHMetadata != nil {
			key.Type = common.SSHKeyType(entity.SSHMetadata.Algorithm)
			key.Comment = &entity.SSHMetadata.Comment
			key.Options = &entity.SSHMetadata.Options
		}

		searchResult = append(searchResult, key)
	}

	c.JSON(http.StatusOK, public.SearchResponse{
		Entities: searchResult,
		Query:    params.Query,
		Total:    result.Total,
		Limit:    limit,
		Offset:   offset,
		Duration: int(time.Since(searchStart).Nanoseconds()),
	})
}
