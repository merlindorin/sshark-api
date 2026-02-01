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
	"github.com/merlindorin/sshark-api/internal/domain/gitlab"
)

type SSHKey struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Key       string `json:"key"`
	CreatedAt string `json:"created_at"`
}

type Fetcher struct {
	cl rest.Requester
}

func NewFetcher(logger *zap.Logger, token string, options ...do.Option) *Fetcher {
	return &Fetcher{cl: gitlabclient.NewClient(logger, token, options...)}
}

func (fetcher Fetcher) FetchAuthorizedKeys(
	ctx context.Context,
	username gitlab.Username,
	authorizedKeys *gitlab.AuthorizedKeys,
) error {
	return fetcher.cl.GET(
		ctx,
		do.WithPath("/users/%s/keys", username),
		withUnmarshalKeys(username, authorizedKeys),
	)
}

func withUnmarshalKeys(username gitlab.Username, authorizedKeys *gitlab.AuthorizedKeys) do.Option {
	return do.WithPostRequestHandler(
		"http_response_handler",
		func(_ context.Context, req *http.Request, res *http.Response) error {
			if res.StatusCode != http.StatusOK {
				return nil
			}

			var keys []SSHKey
			if err := json.NewDecoder(res.Body).Decode(&keys); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}

			for _, key := range keys {
				line := fmt.Sprintf("%s %s\n", key.Key, key.Title)
				if _, err := authorizedKeys.Keys.Write([]byte(line)); err != nil {
					return fmt.Errorf("failed to write key: %w", err)
				}
			}

			authorizedKeys.Source = req.URL.String()
			authorizedKeys.Username = username

			return nil
		})
}
