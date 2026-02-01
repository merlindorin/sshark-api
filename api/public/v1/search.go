package v1

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/merlindorin/sshark-api/api/common"
	"github.com/merlindorin/sshark-api/api/public"
	sshkeysrepository "github.com/merlindorin/sshark-api/internal/infra/sshkeys/redis"
	"go.uber.org/zap"

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
func BuildQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" || q == "*" {
		return "*"
	}

	// If query already has field syntax, return as-is
	if strings.Contains(q, "@") {
		return q
	}

	// Check for wildcard BEFORE escaping
	wildcard := "*"
	if strings.HasSuffix(q, "*") {
		wildcard = ""
		q = strings.TrimSuffix(q, "*")
	}

	// Plain text query - search both username and key with wildcard
	// Escape special characters for RediSearch TAG field
	q = escapeTagQuery(q)

	return "@username:{" + q + wildcard + "} | @key:{" + q + wildcard + "}"
}

// escapeTagQuery escapes special RediSearch characters in a TAG query.
// Characters like - are operators in RediSearch and must be escaped.
func escapeTagQuery(s string) string {
	// Escape backslash FIRST to avoid double-escaping
	s = strings.ReplaceAll(s, "\\", "\\\\")

	// RediSearch special characters that need escaping in TAG queries
	specialChars := []string{
		"-", ".", "_", "+", "=", "&", "|", "!", "(", ")", "{", "}",
		"[", "]", "^", "\"", "~", "*", "?", ":", "/", " ",
	}
	for _, char := range specialChars {
		s = strings.ReplaceAll(s, char, "\\"+char)
	}
	return s
}

// escapeAdvancedQuery escapes TAG field values in advanced queries.
// Finds patterns like @field:{value} and escapes special chars in value.
func escapeAdvancedQuery(query string) string {
	// Pattern to match TAG queries: @field:{content}
	// Captures field name and content inside braces
	tagPattern := regexp.MustCompile(`@(\w+):\{([^}]*)\}`)

	return tagPattern.ReplaceAllStringFunc(query, func(match string) string {
		// Extract field and value from the match
		parts := tagPattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}

		field := parts[1]
		value := parts[2]

		// Escape the value
		escapedValue := escapeTagQuery(value)

		// Reconstruct the TAG query
		return fmt.Sprintf("@%s:{%s}", field, escapedValue)
	})
}

// Usernames extracts usernames from the search query.
// It looks for usernames in:
//   - Plain words: "merlindorin"
//   - Field TEXT search: "@username:merlindorin"
//   - Field TAG search: "@username_exact:{merlindorin}" or "@username_exact:{user1|user2}"
func Usernames(q string) []string {
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

func defaultValue[T any](value *T, defaultValue T) T {
	if value == nil {
		return defaultValue
	}

	return *value
}

func SSHKeys(
	c *gin.Context,
	logger *zap.Logger,
	repository *sshkeysrepository.Repository,
	params public.SearchKeysParams,
) {
	ctx := c.Request.Context()
	searchStart := time.Now()

	query := strings.TrimSpace(params.Query)
	limit := defaultValue(params.Limit, 10)
	offset := defaultValue(params.Offset, 0)
	advanced := defaultValue(params.Advanced, false)
	fields := defaultValue(params.Fields, []public.SearchKeysParamsFields{
		public.Key, public.Provider, public.Type, public.Username,
	})

	if !advanced {
		query = ""
		escapedQuery := escapeTagQuery(params.Query)

		for i, field := range fields {
			if i > 0 {
				query += " OR "
			}
			query += fmt.Sprintf("@%s:{%s}", field, escapedQuery)
		}
	} else {
		// Escape TAG field values in advanced queries
		query = escapeAdvancedQuery(query)
	}

	_, err := repository.ValidateQuery(ctx, query)
	if err != nil {
		logger.Info("failed to validate query", zap.Error(err))
		_ = c.Error(
			common.InvalidSearchQueryError(
				c,
				err,
				query,
				[]string{"merlindorin", "@username:{merlindorin}", "@type:{ssh-ed25519}"},
			),
		)
		return
	}

	searchResult := []common.SSHKey{}

	total, err := repository.Search(ctx, query, limit, offset, func(entity *sshkeys.Entity) {
		searchResult = append(searchResult, common.SSHKey{
			Comment:   &entity.Comment,
			Id:        entity.ID,
			Key:       entity.Key,
			Options:   &entity.Options,
			Provider:  entity.Provider,
			Source:    entity.Source,
			Type:      common.SSHKeyType(entity.Type),
			UpdatedAt: entity.UpdatedAt,
			Username:  entity.Username,
		})
	})
	if err != nil {
		logger.Error("failed to search query", zap.String("query", query), zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	c.JSON(http.StatusOK, public.SearchResponse{
		Entities: searchResult,
		Query:    query,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
		Duration: int(time.Since(searchStart).Nanoseconds()),
	})
}
