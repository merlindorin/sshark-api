package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/merlindorin/go-shared/pkg/net/do"
	"github.com/merlindorin/go-shared/pkg/net/rest"
	"go.uber.org/zap"
)

var (
	ErrNotFound = fmt.Errorf("not found")
)

type Client struct {
	rest.Requester
	httpClient *http.Client
}

func NewClient(logger *zap.Logger, options ...Option) *Client {
	cl := &Client{}

	for _, option := range append([]Option{WithDefaultHTTPClient()}, options...) {
		option.apply(cl)
	}

	githubURL, err := url.Parse("https://github.com")
	if err != nil {
		logger.Fatal("failed to parse github url", zap.Error(err))
	}

	cl.Requester = rest.NewRest(
		githubURL,
		do.WithJSONRequest(),
		do.WithLogger(logger),
		do.WithClient(cl.httpClient),
		WithDefaultHTTPErrorCodeHandler(),
	)

	return cl
}

type ErrorMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Level   string `json:"level"`
}

func WithDefaultHTTPErrorCodeHandler() do.Option {
	return do.WithPostRequestHandler("http_response_errorCode_handler", defaultRequestHandler)
}

func defaultRequestHandler(_ context.Context, _ *http.Request, response *http.Response) error {
	if response.StatusCode >= http.StatusBadRequest {
		if response.StatusCode == http.StatusNotFound {
			return ErrNotFound
		}

		return fmt.Errorf(
			"unexpected response status code %d: %s",
			response.StatusCode,
			http.StatusText(response.StatusCode),
		)
	}

	return nil
}
