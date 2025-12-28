package github

import (
	"net/http"
)

type Option func(cl *Client)

func (w Option) apply(cl *Client) {
	w(cl)
}

func WithHTTPClient(cl *http.Client) Option {
	return func(a *Client) {
		a.httpClient = cl
	}
}

func WithDefaultHTTPClient() Option {
	return WithHTTPClient(http.DefaultClient)
}
