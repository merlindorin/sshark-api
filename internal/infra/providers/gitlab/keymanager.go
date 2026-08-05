// Package gitlab acts on a GitLab user's own keys using a delegated OAuth token.
package gitlab

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
	defaultBaseURL = "https://gitlab.com/api/v4"
	defaultTimeout = 15 * time.Second

	// ScopeManageKeys is what GitLab requires to delete a user's keys.
	//
	// GitLab has no granular key scope the way GitHub does: `read_api` cannot write, and the
	// only scope that can is `api`, which grants full read/write over everything the user can
	// reach. Asking for it is a much larger request than GitHub's admin:public_key, and worth
	// being deliberate about.
	ScopeManageKeys = "api"
)

// KeyManager manages the keys of the GitLab user the token belongs to.
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
				do.WithExtraHeader("Accept", "application/json"),
				// OAuth tokens go in Authorization; PRIVATE-TOKEN is for personal access tokens.
				do.WithExtraHeaderf("Authorization", "Bearer %s", token),
			}, opts...)...,
		),
	}
}

type userKey struct {
	ID  int    `json:"id"`
	Key string `json:"key"`
}

// ListKeys lists the authenticated user's keys of the given type.
func (m *KeyManager) ListKeys(ctx context.Context, keyType publickeys.KeyType) ([]providers.UserKey, error) {
	path := "/user/keys"
	if keyType == publickeys.KeyTypeGPG {
		path = "/user/gpg_keys"
	}

	var keys []userKey

	err := m.httpClient.Do(
		ctx,
		do.WithMethod(http.MethodGet),
		do.WithPath("%s", path),
		do.WithQuery("per_page", "100"),
		withResponse(&keys))
	if err != nil {
		return nil, fmt.Errorf("cannot list %s keys: %w", keyType, err)
	}

	userKeys := make([]providers.UserKey, 0, len(keys))
	for _, k := range keys {
		userKeys = append(userKeys, providers.UserKey{
			ID:          strconv.Itoa(k.ID),
			Fingerprint: fingerprintOf(keyType, k.Key),
		})
	}

	return userKeys, nil
}

// fingerprintOf computes the fingerprint the same way the scraper did when it stored the key,
// so a stored key can be matched to its counterpart at the provider.
func fingerprintOf(keyType publickeys.KeyType, material string) string {
	if keyType == publickeys.KeyTypeGPG {
		parsed, err := gpgparse.Parse([]byte(material))
		if err != nil {
			return ""
		}
		return parsed.Fingerprint
	}

	return sshparse.Fingerprint(material)
}

// DeleteKey removes one of the authenticated user's keys from GitLab.
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

// withResponse checks the status and decodes the body in one handler, because the do library
// keeps post-request handlers in a map and two of them would run in an arbitrary order.
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

// compile-time check that the manager satisfies the port.
var _ providers.KeyManager = (*KeyManager)(nil)
