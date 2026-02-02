package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/merlindorin/go-shared/pkg/must"
	"github.com/merlindorin/go-shared/pkg/net/do"
	"github.com/merlindorin/go-shared/pkg/net/rest"
	"go.uber.org/zap"
)

var (
	ErrNotFound     = fmt.Errorf("not found")
	ErrUnauthorized = fmt.Errorf("unauthorized")
	ErrForbidden    = fmt.Errorf("forbidden")
	GitLabURL       = must.Get(url.Parse("https://gitlab.com"))        //nolint:gochecknoglobals // constant URL
	GitLabAPIURL    = must.Get(url.Parse("https://gitlab.com/api/v4")) //nolint:gochecknoglobals // constant URL
)

type Client struct {
	rest.Requester
	token string
}

func NewClient(logger *zap.Logger, token string, options ...do.Option) *Client {
	cl := &Client{token: token}

	cl.Requester = rest.NewRest(
		GitLabAPIURL,
		append([]do.Option{
			do.WithJSONRequest(),
			do.WithLogger(logger),
			WithDebugPreRequestLogger(logger),
			WithTokenAuth(token),
			WithDebugPostRequestLogger(logger),
			WithDefaultHTTPErrorCodeHandler(),
		}, options...)...,
	)

	return cl
}

func WithDebugPreRequestLogger(logger *zap.Logger) do.Option {
	return do.WithPreRequestHandler("debug_pre_request", func(_ context.Context, req *http.Request) error {
		logger.Debug("GitLab API Request [PRE]",
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
			zap.String("host", req.Host),
			zap.String("path", req.URL.Path),
			zap.String("raw_query", req.URL.RawQuery),
			zap.Any("headers", req.Header),
			zap.Int64("content_length", req.ContentLength),
			zap.String("proto", req.Proto),
		)
		return nil
	})
}

func WithDebugPostRequestLogger(logger *zap.Logger) do.Option {
	return do.WithPostRequestHandler("debug_post_request",
		func(_ context.Context, req *http.Request, resp *http.Response) error {
			logger.Debug("GitLab API Response [POST]",
				zap.String("method", req.Method),
				zap.String("url", req.URL.String()),
				zap.Int("status_code", resp.StatusCode),
				zap.String("status", resp.Status),
				zap.Any("response_headers", resp.Header),
				zap.Int64("content_length", resp.ContentLength),
			)
			return nil
		})
}

func WithTokenAuth(token string) do.Option {
	return do.WithPreRequestHandler("gitlab_token_auth", func(_ context.Context, req *http.Request) error {
		req.Header.Set("PRIVATE-TOKEN", token)
		return nil
	})
}

type ErrorMessage struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

func WithDefaultHTTPErrorCodeHandler() do.Option {
	return do.WithPostRequestHandler("http_response_errorCode_handler", defaultRequestHandler)
}

func defaultRequestHandler(_ context.Context, _ *http.Request, response *http.Response) error {
	if response.StatusCode >= http.StatusBadRequest {
		switch response.StatusCode {
		case http.StatusNotFound:
			return ErrNotFound
		case http.StatusUnauthorized:
			return ErrUnauthorized
		case http.StatusForbidden:
			return ErrForbidden
		}

		return fmt.Errorf(
			"unexpected response status code %d: %s",
			response.StatusCode,
			http.StatusText(response.StatusCode),
		)
	}

	return nil
}
