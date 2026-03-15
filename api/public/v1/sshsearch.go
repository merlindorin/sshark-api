package v1

import (
	"encoding/base64"
	"net/http"
	"strings"
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

func SearchSSHKeys(
	c *gin.Context,
	logger *zap.Logger,
	sourcesRepo sources.Repository,
	publickeysRepo publickeys.Repository,
	params public.SearchSSHKeysParams,
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
			builder := querypostgres.NewBuilder(querypostgres.SSHFieldMapping)
			whereClause, args, err = builder.Build(parsed)
			if err != nil {
				_ = c.Error(api.NewAPIError(c, http.StatusBadRequest, api.NewDetailedError(
					c, "INVALID_SEARCH_QUERY", "Invalid query", err.Error(),
					"Check field names: user, provider, algorithm, fingerprint, comment, key_bits",
				)))
				return
			}
		}
	} else if params.Query != nil && *params.Query != "" {
		queryStr = *params.Query
		fields := []public.SearchSSHKeysParamsFields{
			public.SearchSSHKeysParamsFieldsSourceUsername,
			public.SearchSSHKeysParamsFieldsSourceProvider,
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
			builder := querypostgres.NewBuilder(querypostgres.SSHFieldMapping)
			whereClause, args, err = builder.Build(parsed)
			if err != nil {
				_ = c.Error(api.NewAPIError(c, http.StatusBadRequest, api.NewDetailedError(
					c, "INVALID_SEARCH_QUERY", "Invalid query", err.Error(),
					"Check field names: user, provider, algorithm, fingerprint, comment",
				)))
				return
			}
		}
	}

	result, err := publickeysRepo.SearchWithQuery(ctx, publickeys.KeyTypeSSH, whereClause, args, limit, offset)
	if err != nil {
		logger.Error("failed to search SSH keys", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	entities := make([]common.SSHPublicKey, 0, len(result.Entities))
	for _, entity := range result.Entities {
		source, sourceErr := sourcesRepo.Get(ctx, entity.SourceID)
		if sourceErr != nil {
			logger.Warn("failed to get source for key",
				zap.String("key_id", entity.ID.String()), zap.Error(sourceErr))
			continue
		}

		pk := entityToSSHPublicKey(entity, source)
		entities = append(entities, pk)
	}

	c.JSON(http.StatusOK, public.SSHSearchResponse{
		Entities: entities,
		Query:    queryStr,
		Total:    result.Total,
		Limit:    limit,
		Offset:   offset,
		Duration: int(time.Since(searchStart).Nanoseconds()),
	})
}

func buildBasicQuery[T ~string](term string, fields []T) string {
	if len(fields) == 0 {
		return ""
	}

	escapedTerm := escapeQueryValue(term)
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, "@"+string(field)+":{*"+escapedTerm+"*}")
	}

	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts, " | ")
}

func escapeQueryValue(s string) string {
	replacer := strings.NewReplacer(
		"{", "\\{",
		"}", "\\}",
		"|", "\\|",
		"&", "\\&",
		"(", "\\(",
		")", "\\)",
		"@", "\\@",
		":", "\\:",
		"*", "\\*",
	)
	return replacer.Replace(s)
}

func defaultValue(ptr *int, defaultVal int) int {
	if ptr == nil {
		return defaultVal
	}
	return *ptr
}

func entityToSSHPublicKey(entity publickeys.Entity, source *sources.Entity) common.SSHPublicKey {
	pk := common.SSHPublicKey{
		Id:          entity.ID,
		KeyData:     base64.StdEncoding.EncodeToString(entity.KeyData),
		Fingerprint: entity.Fingerprint,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}

	if source != nil {
		pk.Source = &common.Source{
			Id:       &source.ID,
			Provider: &source.Provider,
			Username: &source.Username,
			Uri:      &source.URI,
			UserId:   &source.UserID,
		}
	}

	if entity.SSHMetadata != nil {
		pk.Algorithm = &entity.SSHMetadata.Algorithm
		pk.Comment = &entity.SSHMetadata.Comment
		pk.KeyBits = entity.SSHMetadata.KeyBits
	}

	return pk
}

func entityToGPGPublicKey(entity publickeys.Entity, source *sources.Entity) common.GPGPublicKey {
	pk := common.GPGPublicKey{
		Id:          entity.ID,
		KeyData:     base64.StdEncoding.EncodeToString(entity.KeyData),
		Fingerprint: entity.Fingerprint,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}

	if source != nil {
		pk.Source = &common.Source{
			Id:       &source.ID,
			Provider: &source.Provider,
			Username: &source.Username,
			Uri:      &source.URI,
			UserId:   &source.UserID,
		}
	}

	if entity.GPGMetadata != nil {
		pk.Algorithm = &entity.GPGMetadata.Algorithm
		pk.KeyBits = entity.GPGMetadata.KeyBits
		pk.ExpiresAt = entity.GPGMetadata.ExpiresAt
		pk.UserIds = &entity.GPGMetadata.UserIDs
		pk.Capabilities = &entity.GPGMetadata.Capabilities
	}

	return pk
}
