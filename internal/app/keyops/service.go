// Package keyops carries out the operations a user asks for on their own keys.
//
// It sits between the HTTP layer and the repositories so the same operation can be triggered by
// a request or run later by a queue worker: refreshing a set of accounts or deleting a key at
// its provider takes long enough, and fails in enough interesting ways, that it should not be
// tied to the lifetime of a request.
package keyops

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/domain/profiles"
	"github.com/merlindorin/sshark-api/internal/domain/providers"
	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
	"github.com/merlindorin/sshark-api/internal/domain/scraper"
	"github.com/merlindorin/sshark-api/internal/domain/sources"
	"github.com/merlindorin/sshark-api/internal/infra/identity"
	githubprovider "github.com/merlindorin/sshark-api/internal/infra/providers/github"
	gitlabprovider "github.com/merlindorin/sshark-api/internal/infra/providers/gitlab"
)

var (
	// ErrKeyNotOwned is returned when the key does not belong to an account the user connected.
	// It is deliberately indistinguishable from "not found" to a caller, so the API does not
	// confirm the existence of someone else's key.
	ErrKeyNotOwned = errors.New("key does not belong to a connected account")

	// ErrNoConnectedAccounts is returned when there is nothing to refresh.
	ErrNoConnectedAccounts = errors.New("no connected provider accounts")
)

// Service performs key operations on behalf of a signed-in user.
type Service struct {
	Logger     *zap.Logger
	Profiles   profiles.Repository
	Sources    sources.Repository
	PublicKeys publickeys.Repository
	Identities *identity.Resolver
	// Scrapers fetches a single account on demand, keyed by provider.
	Scrapers map[scraper.Provider]scraper.Service
}

// RefreshSummary is what a refresh did, kept as the task's result.
type RefreshSummary struct {
	Accounts    int `json:"accounts"`
	KeysAdded   int `json:"keys_added"`
	KeysUpdated int `json:"keys_updated"`
	KeysRemoved int `json:"keys_removed"`
}

// RevokeSummary is what a revocation did.
type RevokeSummary struct {
	Provider string `json:"provider"`
	Username string `json:"username"`
	KeyType  string `json:"key_type"`
	// AlreadyGone records that the provider no longer had the key, so only sshark's copy went.
	AlreadyGone bool `json:"already_gone"`
}

// Reporter is called as an operation progresses, so a caller can surface it.
type Reporter func(done, total int, message string)

func (r Reporter) report(done, total int, message string) {
	if r != nil {
		r(done, total, message)
	}
}

// Refresh pulls the current keys of every account the user connected, in whichever providers
// they use, and records which sources they own.
func (s *Service) Refresh(ctx context.Context, subject string, report Reporter) (*RefreshSummary, error) {
	accounts, err := s.Identities.Accounts(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve connected accounts: %w", err)
	}

	if len(accounts) == 0 {
		return nil, ErrNoConnectedAccounts
	}

	summary := &RefreshSummary{Accounts: len(accounts)}
	report.report(0, len(accounts), "Starting")

	for i, account := range accounts {
		report.report(i, len(accounts), fmt.Sprintf("Refreshing %s/%s", account.Provider, account.Username))

		scraperService, ok := s.Scrapers[account.Provider]
		if !ok {
			s.Logger.Warn("no scraper for provider", zap.String("provider", string(account.Provider)))
			continue
		}

		result, scrapeErr := scraperService.ScrapeUser(ctx, account.Provider, account.Username)
		if scrapeErr != nil {
			// One unreachable provider should not lose the work done for the others.
			s.Logger.Warn("failed to refresh account",
				zap.String("provider", string(account.Provider)),
				zap.String("username", account.Username),
				zap.Error(scrapeErr))
			continue
		}

		summary.KeysAdded += result.KeysAdded
		summary.KeysUpdated += result.KeysUpdated
		summary.KeysRemoved += result.KeysRemoved
	}

	if ownershipErr := s.syncOwnership(ctx, subject, accounts); ownershipErr != nil {
		s.Logger.Warn("failed to record source ownership", zap.Error(ownershipErr))
	}

	report.report(len(accounts), len(accounts), "Done")

	return summary, nil
}

// syncOwnership rebuilds which sources this user's profile owns, so a key is attributed to them
// only while they still hold the account that publishes it.
func (s *Service) syncOwnership(ctx context.Context, subject string, accounts []identity.Account) error {
	profile, err := s.Profiles.GetByClerkUserID(ctx, subject)
	if err != nil {
		return err
	}

	if clearErr := s.Sources.ClearProfile(ctx, profile.ID); clearErr != nil {
		return clearErr
	}

	for _, account := range accounts {
		source, sourceErr := s.Sources.GetByProviderAndUsername(ctx, string(account.Provider), account.Username)
		if sourceErr != nil {
			if !errors.Is(sourceErr, sources.ErrSourceNotFound) {
				return sourceErr
			}
			continue
		}

		if setErr := s.Sources.SetProfile(ctx, source.ID, profile.ID); setErr != nil {
			return setErr
		}
	}

	return nil
}

// Revoke deletes a key at the provider that publishes it, then drops sshark's copy. Only keys
// belonging to an account the user proved they own can be revoked.
func (s *Service) Revoke(
	ctx context.Context,
	subject string,
	keyID uuid.UUID,
	report Reporter,
) (*RevokeSummary, error) {
	report.report(0, 3, "Checking ownership")

	entity, err := s.PublicKeys.Get(ctx, keyID)
	if err != nil {
		return nil, err
	}

	source, err := s.Sources.Get(ctx, entity.SourceID)
	if err != nil {
		return nil, fmt.Errorf("cannot load the key's source: %w", err)
	}

	provider := scraper.Provider(source.Provider)

	account, err := s.Identities.Account(ctx, subject, provider)
	if err != nil {
		if errors.Is(err, identity.ErrNotConnected) {
			return nil, ErrKeyNotOwned
		}
		return nil, err
	}

	if !strings.EqualFold(account.Username, source.Username) {
		return nil, ErrKeyNotOwned
	}

	summary := &RevokeSummary{
		Provider: source.Provider,
		Username: source.Username,
		KeyType:  string(entity.KeyType),
	}

	report.report(1, 3, fmt.Sprintf("Deleting the key at %s", source.Provider))

	if deleteErr := s.deleteAtProvider(ctx, subject, provider, entity, summary); deleteErr != nil {
		return nil, deleteErr
	}

	report.report(2, 3, "Removing it from SShark")

	if localErr := s.PublicKeys.Delete(ctx, keyID); localErr != nil &&
		!errors.Is(localErr, publickeys.ErrKeyNotFound) {
		return nil, fmt.Errorf("key deleted at the provider but not in sshark: %w", localErr)
	}

	report.report(3, 3, "Done")

	return summary, nil
}

func (s *Service) deleteAtProvider(
	ctx context.Context,
	subject string,
	provider scraper.Provider,
	entity *publickeys.Entity,
	summary *RevokeSummary,
) error {
	token, scopes, err := s.Identities.Token(ctx, subject, provider)
	if err != nil {
		if errors.Is(err, identity.ErrNotConnected) {
			return ErrKeyNotOwned
		}
		return err
	}

	manager, err := NewKeyManager(provider, token)
	if err != nil {
		return err
	}

	required := RequiredScope(provider, entity.KeyType)
	// An empty scope list means the provider did not report scopes; assume usable rather than
	// blocking on a guess.
	if len(scopes) > 0 && required != "" && !identity.HasScope(scopes, required) {
		return fmt.Errorf("%w: %s is required", providers.ErrForbidden, required)
	}

	providerKeyID, err := s.resolveProviderKeyID(ctx, manager, entity)
	if err != nil && !errors.Is(err, providers.ErrKeyGone) {
		return err
	}

	// A key already absent upstream still has to disappear from sshark, so an unknown id is
	// treated as a deletion that already happened.
	if providerKeyID == "" {
		summary.AlreadyGone = true
		return nil
	}

	if deleteErr := manager.DeleteKey(ctx, entity.KeyType, providerKeyID); deleteErr != nil {
		if errors.Is(deleteErr, providers.ErrKeyGone) {
			summary.AlreadyGone = true
			return nil
		}
		return deleteErr
	}

	return nil
}

// resolveProviderKeyID finds the identifier the provider uses for this key, matching by
// fingerprint so keys stored before sshark recorded the identifier can still be deleted.
func (s *Service) resolveProviderKeyID(
	ctx context.Context,
	manager providers.KeyManager,
	entity *publickeys.Entity,
) (string, error) {
	userKeys, err := manager.ListKeys(ctx, entity.KeyType)
	if err != nil {
		return "", err
	}

	for _, userKey := range userKeys {
		if userKey.Fingerprint != "" && userKey.Fingerprint == entity.Fingerprint {
			return userKey.ID, nil
		}
	}

	if entity.ProviderKeyID != nil {
		for _, userKey := range userKeys {
			if userKey.ID == *entity.ProviderKeyID {
				return userKey.ID, nil
			}
		}
	}

	return "", nil
}

// NewKeyManager builds the client that acts on a user's keys at a provider.
func NewKeyManager(provider scraper.Provider, token string) (providers.KeyManager, error) {
	switch provider {
	case scraper.ProviderGitHub:
		return githubprovider.NewKeyManager(token), nil
	case scraper.ProviderGitLab:
		return gitlabprovider.NewKeyManager(token), nil
	}

	return nil, fmt.Errorf("%w: %s", providers.ErrUnsupportedProvider, provider)
}

// SupportsRevocation reports whether sshark can delete keys for a provider on a user's behalf.
func SupportsRevocation(provider scraper.Provider) bool {
	return provider == scraper.ProviderGitHub || provider == scraper.ProviderGitLab
}

// RevocationScopes lists what a provider must have granted before keys can be deleted.
func RevocationScopes(provider scraper.Provider) []string {
	switch provider {
	case scraper.ProviderGitHub:
		return []string{githubprovider.ScopeSSHKeys, githubprovider.ScopeGPGKeys}
	case scraper.ProviderGitLab:
		// One scope covers both key types, GitLab offering nothing narrower.
		return []string{gitlabprovider.ScopeManageKeys}
	}

	return nil
}

// RequiredScope is the scope needed to delete this kind of key at this provider.
func RequiredScope(provider scraper.Provider, keyType publickeys.KeyType) string {
	switch provider {
	case scraper.ProviderGitHub:
		return githubprovider.ScopeFor(keyType)
	case scraper.ProviderGitLab:
		return gitlabprovider.ScopeManageKeys
	}

	return ""
}
