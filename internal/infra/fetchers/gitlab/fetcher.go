package gitlab

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/merlindorin/go-shared/pkg/net/do"

	"github.com/merlindorin/sshark-api/internal/domain/scraper"
	"github.com/merlindorin/sshark-api/internal/infra/fetchers/gpgparse"
)

const (
	defaultBaseURL = "https://gitlab.com/api/v4"
	defaultTimeout = 30 * time.Second
)

// Fetcher fetches SSH keys from GitLab.
type Fetcher struct {
	httpClient do.Doer

	// Rate limiting
	mu             sync.Mutex
	rateLimitReset time.Time
	rateRemaining  int
}

// WithToken sets the GitLab API token.
func WithToken(token string) do.Option {
	return do.WithExtraHeader("PRIVATE-TOKEN", token)
}

// NewFetcher creates a new GitLab fetcher.
//
//nolint:bodyclose // Response body is closed by the do library
func NewFetcher(opts ...do.Option) *Fetcher {
	f := &Fetcher{
		rateRemaining: 100, // Assume some capacity initially
	}

	f.httpClient = do.NewDoer(
		must(url.Parse(defaultBaseURL)),
		append([]do.Option{
			do.WithClient(&http.Client{Timeout: defaultTimeout}),
			do.WithExtraHeader("Accept", "application/json"),
			do.WithPostRequestHandler("update_rate_limit", updateRateLimit(f)),
			do.WithPostRequestHandler("check_status", checkStatus),
		}, opts...)...,
	)

	return f
}

// Provider returns the provider name.
func (f *Fetcher) Provider() scraper.Provider {
	return scraper.ProviderGitLab
}

// gitlabUser represents a GitLab user response.
type gitlabUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	WebURL   string `json:"web_url"`
}

// gitlabKey represents a GitLab SSH key response.
type gitlabKey struct {
	ID    int    `json:"id"`
	Key   string `json:"key"`
	Title string `json:"title"`
}

// gitlabGPGKey represents a GitLab GPG key response.
type gitlabGPGKey struct {
	ID        int    `json:"id"`
	Key       string `json:"key"`
	CreatedAt string `json:"created_at"`
}

// ListUsers lists users from GitLab starting from the cursor (user ID).
func (f *Fetcher) ListUsers(ctx context.Context, cursor string, limit int) (*scraper.UsersPage, error) {
	if err := f.waitForRateLimit(ctx); err != nil {
		return nil, err
	}

	var users []gitlabUser

	opts := []do.Option{
		do.WithMethod(http.MethodGet),
		do.WithPath("/users"),
		do.WithQuery("per_page", strconv.Itoa(limit)),
		do.WithQuery("order_by", "id"),
		do.WithQuery("sort", "asc"),
		do.WithUnmarshalBody(&users),
	}

	if cursor != "" {
		opts = append(opts, do.WithQuery("id_after", cursor))
	}

	err := f.httpClient.Do(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch users: %w", err)
	}

	page := &scraper.UsersPage{
		Users: make([]scraper.FetchedUser, 0, len(users)),
	}

	for _, u := range users {
		page.Users = append(page.Users, scraper.FetchedUser{
			UserID:   strconv.Itoa(u.ID),
			Username: u.Username,
			URI:      u.WebURL,
		})
	}

	// Set next cursor to last user ID
	if len(users) > 0 {
		page.NextCursor = strconv.Itoa(users[len(users)-1].ID)
	}

	return page, nil
}

// FetchUser fetches a single GitLab user by username, without their keys.
func (f *Fetcher) FetchUser(ctx context.Context, username string) (*scraper.FetchedUser, error) {
	if err := f.waitForRateLimit(ctx); err != nil {
		return nil, err
	}

	var users []gitlabUser

	err := f.httpClient.Do(
		ctx,
		do.WithMethod(http.MethodGet),
		do.WithPath("/users"),
		do.WithQuery("username", username),
		do.WithUnmarshalBody(&users))
	if err != nil {
		return nil, fmt.Errorf("cannot fetch user %s: %w", username, err)
	}

	if len(users) == 0 {
		return nil, scraper.ErrUserNotFound
	}

	u := users[0]

	return &scraper.FetchedUser{
		UserID:   strconv.Itoa(u.ID),
		Username: u.Username,
		URI:      u.WebURL,
	}, nil
}

// FetchUserKeys fetches SSH keys for a user and populates the Keys field.
func (f *Fetcher) FetchUserKeys(ctx context.Context, user *scraper.FetchedUser) error {
	if err := f.waitForRateLimit(ctx); err != nil {
		return err
	}

	var keys []gitlabKey

	err := f.httpClient.Do(
		ctx,
		do.WithMethod(http.MethodGet),
		do.WithPath("users/%s/keys", user.UserID),
		do.WithUnmarshalBody(&keys))
	if err != nil {
		return fmt.Errorf("cannot fetch keys for userid %s: %w", user.UserID, err)
	}

	user.Keys = make([]scraper.FetchedKey, 0, len(keys))

	for _, k := range keys {
		parsed := parseSSHKey(k.Key, k.Title)
		parsed.KeyID = strconv.Itoa(k.ID)
		parsed.KeyType = scraper.KeyTypeSSH
		user.Keys = append(user.Keys, parsed)
	}

	return nil
}

// FetchUserGPGKeys fetches GPG keys for a user and populates the GPGKeys field.
func (f *Fetcher) FetchUserGPGKeys(ctx context.Context, user *scraper.FetchedUser) error {
	if err := f.waitForRateLimit(ctx); err != nil {
		return err
	}

	var keys []gitlabGPGKey

	err := f.httpClient.Do(
		ctx,
		do.WithMethod(http.MethodGet),
		do.WithPath("users/%s/gpg_keys", user.UserID),
		do.WithUnmarshalBody(&keys))
	if err != nil {
		return fmt.Errorf("cannot fetch GPG keys for userid %s: %w", user.UserID, err)
	}

	user.GPGKeys = make([]scraper.FetchedKey, 0, len(keys))

	for _, k := range keys {
		gpgKey := parseGitLabGPGKey(k)
		user.GPGKeys = append(user.GPGKeys, gpgKey)
	}

	return nil
}

// parseGitLabGPGKey parses a GitLab GPG key response into a FetchedKey.
func parseGitLabGPGKey(k gitlabGPGKey) scraper.FetchedKey {
	fetchedKey := scraper.FetchedKey{
		KeyID:   strconv.Itoa(k.ID),
		KeyType: scraper.KeyTypeGPG,
		KeyData: []byte(k.Key),
	}

	// Parse the PGP key block to extract metadata
	if parsed, parseErr := gpgparse.Parse([]byte(k.Key)); parseErr == nil {
		fetchedKey.Fingerprint = parsed.Fingerprint
		fetchedKey.Algorithm = parsed.Algorithm
		fetchedKey.KeyBits = parsed.KeyBits
		fetchedKey.ExpiresAt = parsed.ExpiresAt
		fetchedKey.UserIDs = parsed.UserIDs
		fetchedKey.Capabilities = parsed.Capabilities
	}

	return fetchedKey
}

func (f *Fetcher) waitForRateLimit(ctx context.Context) error {
	f.mu.Lock()
	remaining := f.rateRemaining
	resetTime := f.rateLimitReset
	f.mu.Unlock()

	if remaining > 0 {
		return nil
	}

	waitDuration := time.Until(resetTime)
	if waitDuration <= 0 {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitDuration):
		return nil
	}
}

// parseSSHKey parses an SSH public key string.
func parseSSHKey(keyStr, title string) scraper.FetchedKey {
	parts := strings.Fields(keyStr)
	if len(parts) < 2 {
		return scraper.FetchedKey{
			KeyData: []byte(keyStr),
			Comment: title,
		}
	}

	algorithm := parts[0]
	keyData := parts[1]
	comment := title
	if comment == "" && len(parts) > 2 {
		comment = strings.Join(parts[2:], " ")
	}

	// Decode base64 to calculate fingerprint
	decoded, err := base64.StdEncoding.DecodeString(keyData)
	fingerprint := ""
	if err == nil {
		hash := sha256.Sum256(decoded)
		fingerprint = "SHA256:" + base64.RawStdEncoding.EncodeToString(hash[:])
	}

	return scraper.FetchedKey{
		KeyData:     []byte(keyStr),
		Fingerprint: fingerprint,
		Algorithm:   algorithm,
		Comment:     comment,
	}
}

func updateRateLimit(f *Fetcher) func(ctx context.Context, req *http.Request, res *http.Response) error {
	return func(_ context.Context, _ *http.Request, res *http.Response) error {
		f.mu.Lock()
		defer f.mu.Unlock()

		if remaining := res.Header.Get("RateLimit-Remaining"); remaining != "" {
			if val, err := strconv.Atoi(remaining); err == nil {
				f.rateRemaining = val
			}
		}

		if reset := res.Header.Get("RateLimit-Reset"); reset != "" {
			if val, err := strconv.ParseInt(reset, 10, 64); err == nil {
				f.rateLimitReset = time.Unix(val, 0)
			}
		}

		return nil
	}
}

func checkStatus(_ context.Context, _ *http.Request, res *http.Response) error {
	if res.StatusCode == http.StatusTooManyRequests {
		return scraper.ErrRateLimited
	}

	if res.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized, please check your credentials")
	}

	if res.StatusCode == http.StatusNotFound {
		return nil
	}

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("unexpected status %d with body %s", res.StatusCode, b)
	}

	return nil
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}

	return v
}
