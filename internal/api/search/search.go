package search

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/domain/ingester"
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
}

func SSHKeys(logger *zap.Logger, rSSHKeys sshkeys.Repository, service *ingester.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		searchStart := time.Now()

		queryParams := SSHKeysQuery{}
		err := c.BindQuery(&queryParams)
		if err != nil {
			_ = c.Error(fmt.Errorf("failed to parse query: %w", err))
			return
		}

		uriParams := SSHKeysURI{}
		err = c.BindUri(&uriParams)
		if err != nil {
			_ = c.Error(fmt.Errorf("failed to parse uri: %w", err))
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
				_ = c.Error(fmt.Errorf("failed to search: %w", err))
				return
			}
		}

		c.JSON(http.StatusOK, SSHKeysResponse{
			Entities: searchResult.Entities,
			Total:    searchResult.Total,
			Limit:    queryParams.Limit,
			Offset:   queryParams.Offset,
			Duration: time.Since(searchStart),
		})
	}
}
