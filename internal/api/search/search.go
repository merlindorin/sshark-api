package search

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/api/apierrors"
	"github.com/merlindorin/sshark-api/internal/domain/ingester"
	"github.com/merlindorin/sshark-api/internal/domain/query"
	"github.com/merlindorin/sshark-api/internal/domain/sshkeys"
)

type SSHKeysQuery struct {
	Limit  int `form:"limit,default=10"`
	Offset int `form:"offset,default=0"`
}

type SSHKeysURI struct {
	Query string `uri:"query" binding:"required"`
}

// BuildQuery transforms the query for Dragonfly compatibility.
// Since all fields are TAGs, plain text queries like "merlindorin" must be
// converted to TAG syntax searching both username and key fields with wildcard.
// If the query already contains field syntax (@field:), it's returned as-is.
func (receiver SSHKeysURI) BuildQuery() string {
	q := strings.TrimSpace(receiver.Query)
	if q == "" || q == "*" {
		return "*"
	}

	// If query already has field syntax, return as-is
	if strings.Contains(q, "@") {
		return q
	}

	// Plain text query - search both username and key with wildcard
	// Escape special characters for TAG field
	q = strings.ReplaceAll(q, " ", "\\ ")

	// Add wildcard only if not already present
	wildcard := "*"
	if strings.HasSuffix(q, "*") {
		wildcard = ""
	}

	return "@username:{" + q + wildcard + "} | @key:{" + q + wildcard + "}"
}

// Usernames extracts usernames from the search query.
// It looks for usernames in:
//   - Plain words: "merlindorin"
//   - Field TEXT search: "@username:merlindorin"
//   - Field TAG search: "@username_exact:{merlindorin}" or "@username_exact:{user1|user2}"
func (receiver *SSHKeysURI) Usernames() []string {
	q := receiver.Query
	seen := make(map[string]struct{})
	var usernames []string

	addUsername := func(name string) {
		name = strings.TrimSuffix(name, "*")
		name = strings.Trim(name, "\"")
		if name != "" {
			if _, ok := seen[name]; !ok {
				seen[name] = struct{}{}
				usernames = append(usernames, name)
			}
		}
	}

	// Match @username:{val1|val2} or @username_exact:{val1|val2}
	tagPattern := `@username(?:_exact)?:\{([^}]+)\}`
	tagRegex := regexp.MustCompile(tagPattern)
	for _, match := range tagRegex.FindAllStringSubmatch(q, -1) {
		if len(match) > 1 {
			for _, val := range strings.Split(match[1], "|") {
				addUsername(val)
			}
		}
	}
	// Remove matched patterns from query for next step
	q = tagRegex.ReplaceAllString(q, "")

	// Match @username:value or @username_exact:value (TEXT field)
	textPattern := `@username(?:_exact)?:([^\s]+)`
	textRegex := regexp.MustCompile(textPattern)
	for _, match := range textRegex.FindAllStringSubmatch(q, -1) {
		if len(match) > 1 {
			addUsername(match[1])
		}
	}
	// Remove matched patterns
	q = textRegex.ReplaceAllString(q, "")

	// Remove other field patterns like @type:{...} or @provider:...
	otherFieldPattern := `@\w+:(?:\{[^}]*\}|[^\s]+)`
	q = regexp.MustCompile(otherFieldPattern).ReplaceAllString(q, "")

	// Remove operators and special chars, then split remaining words
	q = strings.NewReplacer(
		"(", " ", ")", " ",
		"|", " ", "-", " ",
		"~", " ", "*", "",
	).Replace(q)

	// Remaining words are potential usernames
	for _, word := range strings.Fields(q) {
		addUsername(word)
	}

	return usernames
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
	explainer query.Validator,
	service *ingester.Service,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		searchStart := time.Now()

		uriParams := SSHKeysURI{}
		err := c.BindUri(&uriParams)
		if err != nil {
			logger.Info("failed to bind URI", zap.Error(err))
			_ = c.Error(apierrors.InvalidPathParamError(c))
			return
		}

		queryParams := SSHKeysQuery{}
		err = c.BindQuery(&queryParams)
		if err != nil {
			logger.Info("failed to bind query", zap.Error(err))
			_ = c.Error(apierrors.InvalidQueryParamError(c, []string{"limit", "offset"}))
			return
		}

		builtQuery := uriParams.BuildQuery()

		_, err = explainer.ValidateQuery(c.Request.Context(), builtQuery)
		if err != nil {
			logger.Info("failed to validate query", zap.Error(err))
			_ = c.Error(
				apierrors.InvalidSearchQueryError(
					c,
					err,
					uriParams.Query,
					[]string{"merlindorin", "@username:{merlindorin}", "@type:{ssh-ed25519}"},
				),
			)
			return
		}

		searchResult := sshkeys.NewSearchResult()

		for _, username := range uriParams.Usernames() {
			ingestErr := service.Ingest(ctx, username)
			if ingestErr != nil {
				logger.Error("failed to ingest username", zap.String("username", username), zap.Error(ingestErr))
				continue
			}
		}

		err = rSSHKeys.Search(c.Request.Context(), builtQuery, queryParams.Limit, queryParams.Offset, searchResult)
		if err != nil {
			logger.Error("failed to search query", zap.String("query", uriParams.Query), zap.Error(err))
			_ = c.Error(apierrors.InternalError(c))
			return
		}

		c.JSON(http.StatusOK, SSHKeysResponse{
			Entities: searchResult.Entities,
			Query:    builtQuery,
			Total:    searchResult.Total,
			Limit:    queryParams.Limit,
			Offset:   queryParams.Offset,
			Duration: time.Since(searchStart),
		})
	}
}
