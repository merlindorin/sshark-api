// Package github acts on a GitHub user's own keys using a delegated OAuth token.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/merlindorin/go-shared/pkg/net/do"

	"github.com/merlindorin/sshark-api/internal/domain/providers"
	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
	"github.com/merlindorin/sshark-api/internal/infra/fetchers/gpgparse"
	"github.com/merlindorin/sshark-api/internal/infra/fetchers/sshparse"
)

const (
	defaultBaseURL = "https://api.github.com"
	defaultTimeout = 15 * time.Second

	// ScopeSSHKeys is the OAuth scope GitHub requires to delete a user's SSH keys.
	ScopeSSHKeys = "admin:public_key"
	// ScopeGPGKeys is the OAuth scope GitHub requires to delete a user's GPG keys.
	ScopeGPGKeys = "admin:gpg_key"
)

// KeyManager manages the keys of the GitHub user the token belongs to.
type KeyManager struct {
	httpClient do.Doer
}

// NewKeyManager creates a key manager bound to a user's OAuth access token.
func NewKeyManager(token string, opts ...do.Option) *KeyManager {
	base, err := url.Parse(defaultBaseURL)
	if err != nil {
		panic(err)
	}

	return &KeyManager{
		httpClient: do.NewDoer(
			base,
			append([]do.Option{
				do.WithClient(&http.Client{Timeout: defaultTimeout}),
				do.WithExtraHeader("Accept", "application/vnd.github+json"),
				do.WithExtraHeader("X-GitHub-Api-Version", "2022-11-28"),
				do.WithExtraHeaderf("Authorization", "Bearer %s", token),
			}, opts...)...,
		),
	}
}

// withResponse checks the response status and, when the call succeeded and out is not nil,
// decodes the JSON body into it.
//
// The status check and the decoding have to live in the same handler: the do library keeps
// post-request handlers in a map, so two separate handlers would run in an arbitrary order
// and an error body could be decoded before the status was ever looked at.
func withResponse(out any) do.Option {
	return do.WithPostRequestHandler(
		"check_status_and_decode",
		func(_ context.Context, _ *http.Request, res *http.Response) error {
			if err := statusError(res.StatusCode); err != nil {
				return err
			}

			if out == nil {
				return nil
			}

			if err := json.NewDecoder(res.Body).Decode(out); err != nil {
				return fmt.Errorf("cannot decode response: %w", err)
			}

			return nil
		},
	)
}

func statusError(statusCode int) error {
	switch statusCode {
	case http.StatusNotFound:
		return providers.ErrKeyGone
	case http.StatusUnauthorized, http.StatusForbidden:
		return providers.ErrForbidden
	}

	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected status %d", statusCode)
	}

	return nil
}

// ScopeFor returns the OAuth scope required to delete a key of the given type.
func ScopeFor(keyType publickeys.KeyType) string {
	if keyType == publickeys.KeyTypeGPG {
		return ScopeGPGKeys
	}

	return ScopeSSHKeys
}

type userSSHKey struct {
	ID  int    `json:"id"`
	Key string `json:"key"`
}

type userGPGKey struct {
	ID     int    `json:"id"`
	RawKey string `json:"raw_key"`
}

// ListKeys lists the authenticated user's keys of the given type.
func (m *KeyManager) ListKeys(ctx context.Context, keyType publickeys.KeyType) ([]providers.UserKey, error) {
	if keyType == publickeys.KeyTypeGPG {
		return m.listGPGKeys(ctx)
	}

	return m.listSSHKeys(ctx)
}

func (m *KeyManager) listSSHKeys(ctx context.Context) ([]providers.UserKey, error) {
	var keys []userSSHKey

	err := m.httpClient.Do(
		ctx,
		do.WithMethod(http.MethodGet),
		do.WithPath("/user/keys"),
		do.WithQuery("per_page", "100"),
		withResponse(&keys))
	if err != nil {
		return nil, fmt.Errorf("cannot list SSH keys: %w", err)
	}

	userKeys := make([]providers.UserKey, 0, len(keys))
	for _, k := range keys {
		userKeys = append(userKeys, providers.UserKey{
			ID:          strconv.Itoa(k.ID),
			Fingerprint: sshparse.Fingerprint(k.Key),
		})
	}

	return userKeys, nil
}

func (m *KeyManager) listGPGKeys(ctx context.Context) ([]providers.UserKey, error) {
	var keys []userGPGKey

	err := m.httpClient.Do(
		ctx,
		do.WithMethod(http.MethodGet),
		do.WithPath("/user/gpg_keys"),
		do.WithQuery("per_page", "100"),
		withResponse(&keys))
	if err != nil {
		return nil, fmt.Errorf("cannot list GPG keys: %w", err)
	}

	userKeys := make([]providers.UserKey, 0, len(keys))
	for _, k := range keys {
		userKey := providers.UserKey{ID: strconv.Itoa(k.ID)}
		if parsed, parseErr := gpgparse.Parse([]byte(k.RawKey)); parseErr == nil {
			userKey.Fingerprint = parsed.Fingerprint
		}
		userKeys = append(userKeys, userKey)
	}

	return userKeys, nil
}

// DeleteKey removes one of the authenticated user's keys from GitHub.
func (m *KeyManager) DeleteKey(ctx context.Context, keyType publickeys.KeyType, keyID string) error {
	path := "/user/keys/%s"
	if keyType == publickeys.KeyTypeGPG {
		path = "/user/gpg_keys/%s"
	}

	err := m.httpClient.Do(
		ctx,
		do.WithMethod(http.MethodDelete),
		do.WithPath(path, keyID),
		withResponse(nil))
	if err != nil {
		return fmt.Errorf("cannot delete key %s: %w", keyID, err)
	}

	return nil
}
