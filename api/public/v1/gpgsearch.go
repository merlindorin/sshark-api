package v1

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/api/common"
	"github.com/merlindorin/sshark-api/api/public"
	"github.com/merlindorin/sshark-api/internal/api"
	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
	"github.com/merlindorin/sshark-api/internal/domain/query"
	"github.com/merlindorin/sshark-api/internal/domain/sources"
	querypostgres "github.com/merlindorin/sshark-api/internal/infra/query/postgres"
)

func SearchGPGKeys(
	c *gin.Context,
	logger *zap.Logger,
	sourcesRepo sources.Repository,
	publickeysRepo publickeys.Repository,
	params public.SearchGPGKeysParams,
) {
	ctx := c.Request.Context()
	searchStart := time.Now()

	limit := defaultValue(params.Limit, 10)
	offset := defaultValue(params.Offset, 0)

	var whereClause string
	var args []any
	var queryStr string

	//nolint:nestif // branching for advanced vs basic query modes
	if params.Q != nil && *params.Q != "" {
		queryStr = *params.Q
		parsed, err := query.Parse(queryStr)
		if err != nil {
			_ = c.Error(api.NewAPIError(c, http.StatusBadRequest, api.NewDetailedError(
				c, "INVALID_SEARCH_QUERY", "Invalid query syntax", err.Error(),
				"Use @field:{value} syntax, e.g., @user:{torvalds}",
			)))
			return
		}

		if parsed != nil {
			builder := querypostgres.NewBuilder(querypostgres.GPGFieldMapping)
			whereClause, args, err = builder.Build(parsed)
			if err != nil {
				_ = c.Error(api.NewAPIError(c, http.StatusBadRequest, api.NewDetailedError(
					c, "INVALID_SEARCH_QUERY", "Invalid query", err.Error(),
					"Check field names: user, provider, algorithm, fingerprint, key_bits, user_id",
				)))
				return
			}
		}
	} else if params.Query != nil && *params.Query != "" {
		queryStr = *params.Query
		fields := []public.SearchGPGKeysParamsFields{
			public.SearchGPGKeysParamsFieldsSourceUsername,
			public.SearchGPGKeysParamsFieldsSourceProvider,
		}
		if params.Fields != nil && len(*params.Fields) > 0 {
			fields = *params.Fields
		}

		advancedQuery := buildBasicQuery(queryStr, fields)
		parsed, err := query.Parse(advancedQuery)
		if err != nil {
			_ = c.Error(api.NewAPIError(c, http.StatusBadRequest, api.NewDetailedError(
				c, "INVALID_SEARCH_QUERY", "Invalid query", err.Error(),
				"Basic search term contains invalid characters",
			)))
			return
		}

		if parsed != nil {
			builder := querypostgres.NewBuilder(querypostgres.GPGFieldMapping)
			whereClause, args, err = builder.Build(parsed)
			if err != nil {
				_ = c.Error(api.NewAPIError(c, http.StatusBadRequest, api.NewDetailedError(
					c, "INVALID_SEARCH_QUERY", "Invalid query", err.Error(),
					"Check field names: user, provider, algorithm, fingerprint, user_id",
				)))
				return
			}
		}
	}

	result, err := publickeysRepo.SearchWithQuery(ctx, publickeys.KeyTypeGPG, whereClause, args, limit, offset)
	if err != nil {
		logger.Error("failed to search GPG keys", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	entities := make([]common.GPGPublicKey, 0, len(result.Entities))
	for _, entity := range result.Entities {
		source, sourceErr := sourcesRepo.Get(ctx, entity.SourceID)
		if sourceErr != nil {
			logger.Warn("failed to get source for key",
				zap.String("key_id", entity.ID.String()), zap.Error(sourceErr))
			continue
		}

		pk := entityToGPGPublicKey(entity, source)
		entities = append(entities, pk)
	}

	c.JSON(http.StatusOK, public.GPGSearchResponse{
		Entities: entities,
		Query:    queryStr,
		Total:    result.Total,
		Limit:    limit,
		Offset:   offset,
		Duration: int(time.Since(searchStart).Nanoseconds()),
	})
}
