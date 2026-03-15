package github

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
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
	defaultBaseURL = "https://api.github.com"
	defaultTimeout = 30 * time.Second
)

// Fetcher fetches SSH keys from GitHub.
type Fetcher struct {
	httpClient do.Doer

	// Rate limiting
	mu             sync.Mutex
	rateLimitReset time.Time
	rateRemaining  int
}

// WithToken sets the GitHub API token.
func WithToken(token string) do.Option {
	return do.WithExtraHeader("Authorization", fmt.Sprintf("Bearer %s", token))
}

// NewFetcher creates a new GitHub fetcher.
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
			do.WithExtraHeader("Accept", "application/vnd.github+json"),
			do.WithExtraHeader("X-GitHub-Api-Version", "2022-11-28"),
			do.WithPostRequestHandler("update_rate_limit", updateRateLimit(f)),
			do.WithPostRequestHandler("check_status", checkStatus),
		}, opts...)...,
	)

	return f
}

// Provider returns the provider name.
func (f *Fetcher) Provider() scraper.Provider {
	return scraper.ProviderGitHub
}

// githubUser represents a GitHub user response.
type githubUser struct {
	ID      int    `json:"id"`
	Login   string `json:"login"`
	HTMLURL string `json:"html_url"`
}

// githubKey represents a GitHub SSH key response.
type githubKey struct {
	ID  int    `json:"id"`
	Key string `json:"key"`
}

// githubGPGKey represents a GitHub GPG key response.
type githubGPGKey struct {
	ID                int              `json:"id"`
	KeyID             string           `json:"key_id"`
	RawKey            string           `json:"raw_key"`
	Emails            []githubGPGEmail `json:"emails"`
	CanSign           bool             `json:"can_sign"`
	CanEncryptComms   bool             `json:"can_encrypt_comms"`
	CanEncryptStorage bool             `json:"can_encrypt_storage"`
	CanCertify        bool             `json:"can_certify"`
	ExpiresAt         *time.Time       `json:"expires_at"`
	PrimaryKeyID      *int             `json:"primary_key_id"`
	PublicKey         string           `json:"public_key"`
	Revoked           bool             `json:"revoked"`
}

type githubGPGEmail struct {
	Email    string `json:"email"`
	Verified bool   `json:"verified"`
}

// ListUsers lists users from GitHub starting from the cursor (user ID).
func (f *Fetcher) ListUsers(ctx context.Context, cursor string, limit int) (*scraper.UsersPage, error) {
	if err := f.waitForRateLimit(ctx); err != nil {
		return nil, err
	}

	var users []githubUser

	err := f.httpClient.Do(
		ctx,
		do.WithMethod(http.MethodGet),
		do.WithPath("/users"),
		do.WithQuery("per_page", strconv.Itoa(limit)),
		do.WithQuery("since", cursor),
		do.WithUnmarshalBody(&users))
	if err != nil {
		return nil, fmt.Errorf("cannot fetch users: %w", err)
	}

	page := &scraper.UsersPage{
		Users: make([]scraper.FetchedUser, 0, len(users)),
	}

	for _, u := range users {
		page.Users = append(page.Users, scraper.FetchedUser{
			UserID:   strconv.Itoa(u.ID),
			Username: u.Login,
			URI:      u.HTMLURL,
		})
	}

	// Set next cursor to last user ID
	if len(users) > 0 {
		page.NextCursor = strconv.Itoa(users[len(users)-1].ID)
	}

	return page, nil
}

// FetchUserKeys fetches SSH keys for a user and populates the Keys field.
func (f *Fetcher) FetchUserKeys(ctx context.Context, user *scraper.FetchedUser) error {
	if err := f.waitForRateLimit(ctx); err != nil {
		return err
	}

	var keys []githubKey

	err := f.httpClient.Do(
		ctx,
		do.WithMethod(http.MethodGet),
		do.WithPath("users/%s/keys", user.Username),
		do.WithUnmarshalBody(&keys))
	if err != nil {
		return fmt.Errorf("cannot fetch keys for userid %s: %w", user.UserID, err)
	}

	user.Keys = make([]scraper.FetchedKey, 0, len(keys))

	for _, k := range keys {
		parsed := parseSSHKey(k.Key)
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

	var keys []githubGPGKey

	err := f.httpClient.Do(
		ctx,
		do.WithMethod(http.MethodGet),
		do.WithPath("users/%s/gpg_keys", user.Username),
		do.WithUnmarshalBody(&keys))
	if err != nil {
		return fmt.Errorf("cannot fetch GPG keys for userid %s: %w", user.UserID, err)
	}

	user.GPGKeys = make([]scraper.FetchedKey, 0, len(keys))

	for _, k := range keys {
		// Skip revoked keys
		if k.Revoked {
			continue
		}

		// Skip subkeys (they have a PrimaryKeyID)
		if k.PrimaryKeyID != nil {
			continue
		}

		gpgKey := parseGitHubGPGKey(k)
		user.GPGKeys = append(user.GPGKeys, gpgKey)
	}

	return nil
}

// parseGitHubGPGKey parses a GitHub GPG key response into a FetchedKey.
func parseGitHubGPGKey(k githubGPGKey) scraper.FetchedKey {
	// Extract user IDs from emails
	userIDs := make([]string, 0, len(k.Emails))
	for _, email := range k.Emails {
		userIDs = append(userIDs, email.Email)
	}

	// Build capabilities list
	capabilities := make([]string, 0, 4)
	if k.CanSign {
		capabilities = append(capabilities, "sign")
	}
	if k.CanCertify {
		capabilities = append(capabilities, "certify")
	}
	if k.CanEncryptComms {
		capabilities = append(capabilities, "encrypt_comms")
	}
	if k.CanEncryptStorage {
		capabilities = append(capabilities, "encrypt_storage")
	}

	// Try to parse raw key for additional metadata
	var algorithm string
	var keyBits *int
	var fingerprint string

	if k.RawKey != "" {
		if parsed, parseErr := gpgparse.Parse([]byte(k.RawKey)); parseErr == nil {
			algorithm = parsed.Algorithm
			keyBits = parsed.KeyBits
			fingerprint = parsed.Fingerprint
		}
	}

	return scraper.FetchedKey{
		KeyID:        k.KeyID,
		KeyType:      scraper.KeyTypeGPG,
		KeyData:      []byte(k.RawKey),
		Fingerprint:  fingerprint,
		Algorithm:    algorithm,
		KeyBits:      keyBits,
		ExpiresAt:    k.ExpiresAt,
		UserIDs:      userIDs,
		Capabilities: capabilities,
	}
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
func parseSSHKey(keyStr string) scraper.FetchedKey {
	parts := strings.Fields(keyStr)
	if len(parts) < 2 {
		return scraper.FetchedKey{
			KeyData: []byte(keyStr),
		}
	}

	algorithm := parts[0]
	keyData := parts[1]
	comment := ""
	if len(parts) > 2 {
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

		if remaining := res.Header.Get("X-RateLimit-Remaining"); remaining != "" {
			if val, err := strconv.Atoi(remaining); err == nil {
				f.rateRemaining = val
			}
		}

		if reset := res.Header.Get("X-RateLimit-Reset"); reset != "" {
			if val, err := strconv.ParseInt(reset, 10, 64); err == nil {
				f.rateLimitReset = time.Unix(val, 0)
			}
		}

		return nil
	}
}

func checkStatus(_ context.Context, _ *http.Request, res *http.Response) error {
	if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusTooManyRequests {
		return scraper.ErrRateLimited
	}

	if res.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized, please check your credentials")
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", res.StatusCode)
	}

	return nil
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}

	return v
}
