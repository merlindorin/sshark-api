package github

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/merlindorin/go-shared/pkg/net/do"
	"github.com/merlindorin/go-shared/pkg/net/rest"
	"go.uber.org/zap"

	githubclient "github.com/merlindorin/sshark-api/internal/client/github"
	"github.com/merlindorin/sshark-api/internal/domain/github"
)

type Fetcher struct {
	cl rest.Requester
}

func NewFetcher(logger *zap.Logger) *Fetcher {
	return &Fetcher{cl: githubclient.NewClient(logger)}
}

func (fetcher Fetcher) FetchAuthorizedKeys(
	ctx context.Context,
	username github.Username,
	authorizedKeys *github.AuthorizedKeys,
) error {
	return fetcher.cl.GET(ctx, do.WithPath("/%s.keys", username), withUnmarshalBody(username, authorizedKeys))
}

func withUnmarshalBody(username github.Username, authorizedKeys *github.AuthorizedKeys) do.Option {
	return do.WithPostRequestHandler(
		"http_response_handler",
		func(_ context.Context, req *http.Request, res *http.Response) error {
			if res.StatusCode != http.StatusOK {
				return nil
			}

			_, err := io.Copy(authorizedKeys.Keys, res.Body)
			if err != nil {
				return fmt.Errorf("failed to read and copy response body: %w", err)
			}

			authorizedKeys.Source = req.URL.String()
			authorizedKeys.Username = username

			return nil
		})
}
