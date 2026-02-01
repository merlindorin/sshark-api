package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/merlindorin/go-shared/pkg/must"
	"github.com/merlindorin/go-shared/pkg/net/do"
	"github.com/merlindorin/go-shared/pkg/net/rest"
	"go.uber.org/zap"

	githubclient "github.com/merlindorin/sshark-api/internal/client/github"
)

//nolint:gochecknoglobals // GitHub API base URL constant
var GitHubAPIURL = must.Get(url.Parse("https://api.github.com"))

type User struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Type  string `json:"type"`
}

type UsersFetcher struct {
	cl     rest.Requester
	logger *zap.Logger
}

func NewUsersFetcher(logger *zap.Logger, options ...do.Option) *UsersFetcher {
	cl := &githubclient.Client{}

	cl.Requester = rest.NewRest(
		GitHubAPIURL,
		append([]do.Option{
			do.WithJSONRequest(),
			do.WithLogger(logger),
			githubclient.WithDefaultHTTPErrorCodeHandler(),
		}, options...)...,
	)

	return &UsersFetcher{
		cl:     cl,
		logger: logger,
	}
}

func (f *UsersFetcher) FetchUsers(ctx context.Context, since int64, perPage int) ([]User, error) {
	var users []User

	err := f.cl.GET(
		ctx,
		do.WithPath("/users"),
		do.WithQuery("since", fmt.Sprintf("%d", since)),
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
