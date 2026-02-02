package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/merlindorin/go-shared/pkg/net/do"
	"github.com/merlindorin/go-shared/pkg/net/rest"
	"go.uber.org/zap"

	gitlabclient "github.com/merlindorin/sshark-api/internal/client/gitlab"
)

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	State    string `json:"state"`
}

type UsersFetcher struct {
	cl     rest.Requester
	logger *zap.Logger
}

func NewUsersFetcher(logger *zap.Logger, token string, options ...do.Option) *UsersFetcher {
	return &UsersFetcher{
		cl:     gitlabclient.NewClient(logger, token, options...),
		logger: logger,
	}
}

func (f *UsersFetcher) FetchUsers(ctx context.Context, page int, perPage int) ([]User, error) {
	f.logger.Debug("FetchUsers called",
		zap.Int("page", page),
		zap.Int("per_page", perPage),
	)

	var users []User

	err := f.cl.GET(
		ctx,
		do.WithPath("/users"),
		do.WithQuery("page", fmt.Sprintf("%d", page)),
		do.WithQuery("per_page", fmt.Sprintf("%d", perPage)),
		withUnmarshalUsers(&users),
	)
	if err != nil {
		f.logger.Error("FetchUsers failed",
			zap.Int("page", page),
			zap.Int("per_page", perPage),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to fetch users: %w", err)
	}

	f.logger.Debug("FetchUsers succeeded",
		zap.Int("page", page),
		zap.Int("per_page", perPage),
		zap.Int("users_count", len(users)),
	)

	return users, nil
}

func withUnmarshalUsers(users *[]User) do.Option {
	return do.WithPostRequestHandler(
		"unmarshal_users",
		func(_ context.Context, req *http.Request, res *http.Response) error {
			// Get logger from context or create a basic one
			logger := zap.L()

			logger.Debug("unmarshal_users handler",
				zap.String("url", req.URL.String()),
				zap.Int("status_code", res.StatusCode),
				zap.String("status", res.Status),
				zap.String("content_type", res.Header.Get("Content-Type")),
			)

			if res.StatusCode != http.StatusOK {
				// Try to read the error response body
				bodyBytes, _ := io.ReadAll(res.Body)
				logger.Error("unexpected status code",
					zap.Int("status_code", res.StatusCode),
					zap.String("response_body", string(bodyBytes)),
				)
				return fmt.Errorf("unexpected status code: %d", res.StatusCode)
			}

			if err := json.NewDecoder(res.Body).Decode(users); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}

			logger.Debug("unmarshal_users succeeded",
				zap.Int("users_decoded", len(*users)),
			)

			return nil
		},
	)
}
