package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
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
	var users []User

	err := f.cl.GET(
		ctx,
		do.WithPath("/users"),
		do.WithQuery("page", fmt.Sprintf("%d", page)),
		do.WithQuery("per_page", fmt.Sprintf("%d", perPage)),
		withUnmarshalUsers(&users),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch users: %w", err)
	}

	return users, nil
}

func withUnmarshalUsers(users *[]User) do.Option {
	return do.WithPostRequestHandler(
		"unmarshal_users",
		func(_ context.Context, _ *http.Request, res *http.Response) error {
			if res.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected status code: %d", res.StatusCode)
			}

			if err := json.NewDecoder(res.Body).Decode(users); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}

			return nil
		},
	)
}
