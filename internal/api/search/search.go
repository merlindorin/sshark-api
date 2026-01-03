package search

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/api/apierrors"
	"github.com/merlindorin/sshark-api/internal/domain/ingester"
	"github.com/merlindorin/sshark-api/internal/domain/query"
	"github.com/merlindorin/sshark-api/internal/domain/sshkeys"
	"github.com/merlindorin/sshark-api/internal/redisquery"
)

type SSHKeysQuery struct {
	Limit  int `form:"limit,default=10"`
	Offset int `form:"offset,default=0"`
}

type SSHKeysURI struct {
	Query string `uri:"query" binding:"required"`
}

func (s *SSHKeysURI) Usernames() ([]string, error) {
	match := []string{}

	parse, err := redisquery.Parse(s.Query)
	if err != nil {
		return nil, err
	}

	for _, term := range parse.Terms {
		found := ""
		if term.Field != nil {
			if term.Field.Name == "@username" {
				found = *term.Field.Text
			}
		}

		if term.Word != nil {
			found = *term.Word
		}

		if term.Fuzzy != nil {
			found = *term.Fuzzy
		}

		if term.Phrase != nil {
			found = *term.Phrase
		}

		found = strings.Trim(found, "*")
		found = strings.Trim(found, "%")
		found = strings.Trim(found, "\"")

		if len(found) > 0 {
			match = append(match, found)
		}
	}

	return match, nil
}

type SSHKeysResponse struct {
	Entities []sshkeys.Entity `json:"entities"`
	Total    int              `json:"total"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
	Duration time.Duration    `json:"duration"`
	Query    string           `json:"query"`
}

func SSHKeys(
	logger *zap.Logger,
	rSSHKeys sshkeys.Repository,
	explainer query.Explainer,
	service *ingester.Service,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		searchStart := time.Now()

		uriParams := SSHKeysURI{}
		err := c.BindUri(&uriParams)
		if err != nil {
			_ = c.Error(apierrors.InvalidPathParamError(c))
			return
		}

		queryParams := SSHKeysQuery{}
		err = c.BindQuery(&queryParams)
		if err != nil {
			_ = c.Error(apierrors.InvalidQueryParamError(c, []string{"limit", "offset"}))
			return
		}

		_, err = explainer.ExplainQuery(c.Request.Context(), uriParams.Query)
		if err != nil {
			_ = c.Error(
				apierrors.InvalidSearchQueryError(
					c,
					err,
					uriParams.Query,
					[]string{"merlindorin", "@username:merlindorin", "@key:{XXX}"},
				),
			)
			return
		}

		searchResult := sshkeys.NewSearchResult()

		usernames, err := uriParams.Usernames()
		if err == nil {
			for _, username := range usernames {
				ingestErr := service.Ingest(ctx, username)
				if ingestErr != nil {
					logger.Error("failed to ingest username", zap.String("username", username), zap.Error(ingestErr))
					continue
				}
			}

			err = rSSHKeys.Search(c.Request.Context(), uriParams.Query, queryParams.Limit, queryParams.Offset, searchResult)
			if err != nil {
				logger.Error("failed to search query", zap.String("query", uriParams.Query), zap.Error(err))

				_ = c.Error(apierrors.InternalError(c))
				return
			}
		}

		c.JSON(http.StatusOK, SSHKeysResponse{
			Entities: searchResult.Entities,
			Query:    uriParams.Query,
			Total:    searchResult.Total,
			Limit:    queryParams.Limit,
			Offset:   queryParams.Offset,
			Duration: time.Since(searchStart),
		})
	}
}
