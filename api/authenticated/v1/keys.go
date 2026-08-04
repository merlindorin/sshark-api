package v1

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/api/authenticated"
	"github.com/merlindorin/sshark-api/api/common"
	"github.com/merlindorin/sshark-api/internal/domain/providers"
	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
	"github.com/merlindorin/sshark-api/internal/domain/scraper"
	"github.com/merlindorin/sshark-api/internal/domain/sources"
	"github.com/merlindorin/sshark-api/internal/infra/identity"
	githubprovider "github.com/merlindorin/sshark-api/internal/infra/providers/github"
)

// maxKeysPerSource caps how many keys are listed for one provider account. Real accounts hold
// a handful of keys, so this only guards against an unbounded read.
const maxKeysPerSource = 200

// KeyServices holds everything the key endpoints need to attribute, refresh and revoke the
// keys of the signed-in user.
type KeyServices struct {
	Sources    sources.Repository
	PublicKeys publickeys.Repository
	Profiles   ProfileServices
	Identities *identity.Resolver
	// Scrapers refreshes a single account on demand, keyed by provider. A provider without a
	// scraper can still be listed, it just cannot be refreshed.
	Scrapers map[scraper.Provider]scraper.Service
}

// candidate is a provider account whose keys are attributed to the signed-in user.
type candidate struct {
	account identity.Account
	// verified is true when the user proved they own the account by signing in with it.
	// Only verified accounts can have their keys revoked.
	verified bool
}

// ListMyKeys returns the keys sshark holds for the accounts of the signed-in user.
func ListMyKeys(c *gin.Context, logger *zap.Logger, services KeyServices) {
	ctx := c.Request.Context()

	subject, ok := subjectFromContext(c)
	if !ok {
		return
	}

	candidates, err := services.resolveCandidates(ctx, subject)
	if err != nil {
		logger.Error("failed to resolve connected accounts", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	response, err := services.buildResponse(ctx, logger, subject, candidates)
	if err != nil {
		logger.Error("failed to list keys", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	c.JSON(http.StatusOK, response)
}

// RefreshMyKeys pulls the current keys from every provider account of the signed-in user
// before returning them, so keys added since the last crawl show up right away.
func RefreshMyKeys(c *gin.Context, logger *zap.Logger, services KeyServices) {
	ctx := c.Request.Context()

	subject, ok := subjectFromContext(c)
	if !ok {
		return
	}

	candidates, err := services.resolveCandidates(ctx, subject)
	if err != nil {
		logger.Error("failed to resolve connected accounts", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	for _, cand := range candidates {
		scraperService, hasScraper := services.Scrapers[cand.account.Provider]
		if !hasScraper {
			continue
		}

		result, scrapeErr := scraperService.ScrapeUser(ctx, cand.account.Provider, cand.account.Username)
		if scrapeErr != nil {
			// A provider that is unreachable or does not know the username should not sink the
			// whole refresh: the remaining accounts and the stored keys are still worth returning.
			logger.Warn("failed to refresh account",
				zap.String("provider", string(cand.account.Provider)),
				zap.String("username", cand.account.Username),
				zap.Error(scrapeErr),
			)
			continue
		}

		logger.Info("refreshed account",
			zap.String("provider", string(cand.account.Provider)),
			zap.String("username", cand.account.Username),
			zap.Int("keys_added", result.KeysAdded),
			zap.Int("keys_updated", result.KeysUpdated),
			zap.Int("keys_removed", result.KeysRemoved),
		)
	}

	// A refresh may have created the sources for accounts sshark had never crawled, so record
	// ownership now that they exist.
	if profile, profileErr := services.Profiles.ensureProfile(ctx, logger, subject); profileErr == nil {
		accounts := make([]identity.Account, 0, len(candidates))
		for _, cand := range candidates {
			accounts = append(accounts, cand.account)
		}
		services.Profiles.syncOwnership(ctx, logger, profile, accounts)
	}

	response, err := services.buildResponse(ctx, logger, subject, candidates)
	if err != nil {
		logger.Error("failed to list keys", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	refreshedAt := time.Now()
	response.RefreshedAt = &refreshedAt

	c.JSON(http.StatusOK, response)
}

// RevokeMyKey deletes a key at the provider it was published on, then removes it from sshark.
//
//nolint:funlen // one linear flow: authorise, resolve the provider key, delete, forget
func RevokeMyKey(c *gin.Context, logger *zap.Logger, services KeyServices, id uuid.UUID) {
	ctx := c.Request.Context()

	subject, ok := subjectFromContext(c)
	if !ok {
		return
	}

	entity, err := services.PublicKeys.Get(ctx, id)
	if err != nil {
		if errors.Is(err, publickeys.ErrKeyNotFound) {
			_ = c.Error(common.KeyNotFoundError(c))
			return
		}
		logger.Error("failed to get key", zap.Error(err), zap.String("key_id", id.String()))
		_ = c.Error(common.InternalError(c))
		return
	}

	source, err := services.Sources.Get(ctx, entity.SourceID)
	if err != nil {
		logger.Error("failed to get source", zap.Error(err), zap.String("key_id", id.String()))
		_ = c.Error(common.InternalError(c))
		return
	}

	// Only a connected account authorises a revocation: matching the user's name is not proof
	// of ownership, signing in with the provider is.
	account, err := services.Identities.Account(ctx, subject, scraper.Provider(source.Provider))
	if err != nil {
		if errors.Is(err, identity.ErrNotConnected) {
			_ = c.Error(common.ProviderNotConnectedError(c, source.Provider))
			return
		}
		logger.Error("failed to resolve connected account", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	if !strings.EqualFold(account.Username, source.Username) {
		logger.Warn("refused to revoke a key owned by another account",
			zap.String("key_id", id.String()),
			zap.String("key_username", source.Username),
			zap.String("account_username", account.Username),
		)
		_ = c.Error(common.KeyNotFoundError(c))
		return
	}

	token, scopes, err := services.Identities.Token(ctx, subject, account.Provider)
	if err != nil {
		if errors.Is(err, identity.ErrNotConnected) {
			_ = c.Error(common.ProviderNotConnectedError(c, source.Provider))
			return
		}
		logger.Error("failed to fetch provider credential", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	manager, err := newKeyManager(account.Provider, token)
	if err != nil {
		_ = c.Error(common.ProviderNotConnectedError(c, source.Provider))
		return
	}

	requiredScope := requiredScopeFor(account.Provider, entity.KeyType)
	if len(scopes) > 0 && !identity.HasScope(scopes, requiredScope) {
		_ = c.Error(common.ProviderForbiddenError(c, source.Provider, requiredScope))
		return
	}

	providerKeyID, err := resolveProviderKeyID(ctx, manager, entity)
	if err != nil && !errors.Is(err, providers.ErrKeyGone) {
		if errors.Is(err, providers.ErrForbidden) {
			_ = c.Error(common.ProviderForbiddenError(c, source.Provider, requiredScope))
			return
		}
		logger.Error("failed to resolve provider key id", zap.Error(err), zap.String("key_id", id.String()))
		_ = c.Error(common.ProviderUnavailableError(c, source.Provider))
		return
	}

	// A key already absent from the provider still has to disappear from sshark, so an unknown
	// key id is treated as an already-completed deletion rather than an error.
	if providerKeyID != "" {
		if deleteErr := manager.DeleteKey(ctx, entity.KeyType, providerKeyID); deleteErr != nil {
			switch {
			case errors.Is(deleteErr, providers.ErrKeyGone):
			case errors.Is(deleteErr, providers.ErrForbidden):
				_ = c.Error(common.ProviderForbiddenError(c, source.Provider, requiredScope))
				return
			default:
				logger.Error("failed to delete key at provider",
					zap.Error(deleteErr), zap.String("key_id", id.String()))
				_ = c.Error(common.ProviderUnavailableError(c, source.Provider))
				return
			}
		}
	}

	if deleteErr := services.PublicKeys.Delete(ctx, id); deleteErr != nil &&
		!errors.Is(deleteErr, publickeys.ErrKeyNotFound) {
		logger.Error("key deleted at provider but not in sshark",
			zap.Error(deleteErr), zap.String("key_id", id.String()))
		_ = c.Error(common.InternalError(c))
		return
	}

	logger.Info("revoked key",
		zap.String("key_id", id.String()),
		zap.String("provider", source.Provider),
		zap.String("username", source.Username),
	)

	c.Status(http.StatusNoContent)
}

// resolveCandidates lists the provider accounts whose keys belong to the user. A key is the
// user's because they signed in with the account that publishes it: sharing a username with an
// account they have not connected proves nothing, so it is deliberately not considered.
func (s KeyServices) resolveCandidates(ctx context.Context, subject string) ([]candidate, error) {
	accounts, err := s.Identities.Accounts(ctx, subject)
	if err != nil {
		return nil, err
	}

	candidates := make([]candidate, 0, len(accounts))
	for _, account := range accounts {
		candidates = append(candidates, candidate{account: account, verified: true})
	}

	return candidates, nil
}

func (s KeyServices) buildResponse(
	ctx context.Context,
	logger *zap.Logger,
	subject string,
	candidates []candidate,
) (*authenticated.MyKeysResponse, error) {
	response := &authenticated.MyKeysResponse{
		Accounts: make([]authenticated.ConnectedAccount, 0, len(candidates)),
		SshKeys:  make([]authenticated.MySSHKey, 0),
		GpgKeys:  make([]authenticated.MyGPGKey, 0),
	}

	for _, cand := range candidates {
		source, err := s.Sources.GetByProviderAndUsername(
			ctx, string(cand.account.Provider), cand.account.Username)
		if err != nil && !errors.Is(err, sources.ErrSourceNotFound) {
			return nil, err
		}

		if cand.verified {
			response.Accounts = append(response.Accounts, s.describeAccount(ctx, logger, subject, cand.account))
		}

		if source == nil {
			continue
		}

		revocable := cand.verified && supportsRevocation(cand.account.Provider)

		sshKeys, err := s.listKeys(ctx, source.ID, publickeys.KeyTypeSSH)
		if err != nil {
			return nil, err
		}
		for i := range sshKeys {
			response.SshKeys = append(response.SshKeys, toMySSHKey(sshKeys[i], source, cand.verified, revocable))
		}

		gpgKeys, err := s.listKeys(ctx, source.ID, publickeys.KeyTypeGPG)
		if err != nil {
			return nil, err
		}
		for i := range gpgKeys {
			response.GpgKeys = append(response.GpgKeys, toMyGPGKey(gpgKeys[i], source, cand.verified, revocable))
		}
	}

	return response, nil
}

// describeAccount reports whether sshark can actually act on the account, so the UI can
// explain an unusable connection instead of failing at revoke time.
func (s KeyServices) describeAccount(
	ctx context.Context,
	logger *zap.Logger,
	subject string,
	account identity.Account,
) authenticated.ConnectedAccount {
	described := authenticated.ConnectedAccount{
		Provider: string(account.Provider),
		Username: account.Username,
	}

	if !supportsRevocation(account.Provider) {
		return described
	}

	_, scopes, err := s.Identities.Token(ctx, subject, account.Provider)
	if err != nil {
		logger.Warn("no usable credential for connected account",
			zap.String("provider", string(account.Provider)), zap.Error(err))
		return described
	}

	missing := make([]string, 0, 2)
	for _, scope := range revocationScopes(account.Provider) {
		// An empty scope list means the provider did not report scopes: assume the grant is
		// usable rather than blocking the user on a guess.
		if len(scopes) > 0 && !identity.HasScope(scopes, scope) {
			missing = append(missing, scope)
		}
	}

	described.CanRevoke = len(missing) == 0
	if len(missing) > 0 {
		described.MissingScopes = &missing
	}

	return described
}

func (s KeyServices) listKeys(
	ctx context.Context,
	sourceID uuid.UUID,
	keyType publickeys.KeyType,
) ([]publickeys.Entity, error) {
	result, err := s.PublicKeys.Search(ctx, publickeys.SearchFilter{
		SourceID: &sourceID,
		KeyType:  &keyType,
	}, maxKeysPerSource, 0)
	if err != nil {
		return nil, err
	}

	return result.Entities, nil
}

// resolveProviderKeyID returns the identifier the provider uses for this key. Keys stored
// before sshark started recording it, or whose identifier went stale, are matched back by
// fingerprint. An empty result means the provider does not hold the key anymore.
func resolveProviderKeyID(
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

	// Fall back to the recorded identifier when the fingerprint could not be matched, for
	// instance because the provider does not expose the key material in a comparable form.
	if entity.ProviderKeyID != nil {
		for _, userKey := range userKeys {
			if userKey.ID == *entity.ProviderKeyID {
				return userKey.ID, nil
			}
		}
	}

	return "", nil
}

func newKeyManager(provider scraper.Provider, token string) (providers.KeyManager, error) {
	if provider == scraper.ProviderGitHub {
		return githubprovider.NewKeyManager(token), nil
	}

	return nil, providers.ErrUnsupportedProvider
}

func supportsRevocation(provider scraper.Provider) bool {
	return provider == scraper.ProviderGitHub
}

func revocationScopes(provider scraper.Provider) []string {
	if provider == scraper.ProviderGitHub {
		return []string{githubprovider.ScopeSSHKeys, githubprovider.ScopeGPGKeys}
	}

	return nil
}

func requiredScopeFor(provider scraper.Provider, keyType publickeys.KeyType) string {
	if provider == scraper.ProviderGitHub {
		return githubprovider.ScopeFor(keyType)
	}

	return ""
}

func subjectFromContext(c *gin.Context) (string, bool) {
	claims, ok := clerk.SessionClaimsFromContext(c.Request.Context())
	if !ok {
		_ = c.Error(common.UnauthorizedError(c))
		return "", false
	}

	return claims.Subject, true
}

func toMySSHKey(
	entity publickeys.Entity,
	source *sources.Entity,
	verified bool,
	revocable bool,
) authenticated.MySSHKey {
	key := authenticated.MySSHKey{
		Id:          entity.ID,
		KeyData:     base64.StdEncoding.EncodeToString(entity.KeyData),
		Fingerprint: entity.Fingerprint,
		Verified:    verified,
		Revocable:   revocable,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
		Source:      toSource(source),
	}

	if entity.SSHMetadata != nil {
		key.Algorithm = &entity.SSHMetadata.Algorithm
		key.Comment = &entity.SSHMetadata.Comment
		key.KeyBits = entity.SSHMetadata.KeyBits
	}

	return key
}

func toMyGPGKey(
	entity publickeys.Entity,
	source *sources.Entity,
	verified bool,
	revocable bool,
) authenticated.MyGPGKey {
	key := authenticated.MyGPGKey{
		Id:          entity.ID,
		KeyData:     base64.StdEncoding.EncodeToString(entity.KeyData),
		Fingerprint: entity.Fingerprint,
		Verified:    verified,
		Revocable:   revocable,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
		Source:      toSource(source),
	}

	if entity.GPGMetadata != nil {
		key.Algorithm = &entity.GPGMetadata.Algorithm
		key.KeyBits = entity.GPGMetadata.KeyBits
		key.ExpiresAt = entity.GPGMetadata.ExpiresAt
		key.UserIds = &entity.GPGMetadata.UserIDs
		key.Capabilities = &entity.GPGMetadata.Capabilities
	}

	return key
}

func toSource(source *sources.Entity) *common.Source {
	if source == nil {
		return nil
	}

	return &common.Source{
		Id:       &source.ID,
		Provider: &source.Provider,
		Username: &source.Username,
		Uri:      &source.URI,
		UserId:   &source.UserID,
		// Only set when someone proved they own the account, which is what makes a key
		// attributable to a person rather than merely published under a name.
		ProfileUsername: source.ProfileUsername,
	}
}
