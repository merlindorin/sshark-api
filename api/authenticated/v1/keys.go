package v1

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/api/authenticated"
	"github.com/merlindorin/sshark-api/api/common"
	"github.com/merlindorin/sshark-api/internal/app/keyops"
	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
	"github.com/merlindorin/sshark-api/internal/domain/scraper"
	"github.com/merlindorin/sshark-api/internal/domain/sources"
	"github.com/merlindorin/sshark-api/internal/infra/identity"
	"github.com/merlindorin/sshark-api/internal/infra/jobs"
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
	// Queue runs the operations that take too long to hold a request open for.
	Queue *jobs.Queue
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

// RefreshMyKeys queues a refresh of every connected account and answers with the task to watch.
//
// The work is queued rather than done here because a refresh waits out the provider's rate
// limit, which can be most of an hour — far longer than a request should be held open.
func RefreshMyKeys(c *gin.Context, logger *zap.Logger, services KeyServices) {
	subject, ok := subjectFromContext(c)
	if !ok {
		return
	}

	task, err := services.Queue.EnqueueRefresh(c.Request.Context(), subject)
	if err != nil {
		logger.Error("failed to queue refresh", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	c.JSON(http.StatusAccepted, toTask(task))
}

// RevokeMyKey queues the revocation of a key and answers with the task to watch.
func RevokeMyKey(c *gin.Context, logger *zap.Logger, services KeyServices, id uuid.UUID) {
	ctx := c.Request.Context()

	subject, ok := subjectFromContext(c)
	if !ok {
		return
	}

	// Fail obviously wrong requests now, so the user is not sent to poll a task that was only
	// ever going to say "no such key".
	if _, err := services.PublicKeys.Get(ctx, id); err != nil {
		if errors.Is(err, publickeys.ErrKeyNotFound) {
			_ = c.Error(common.KeyNotFoundError(c))
			return
		}
		logger.Error("failed to get key", zap.Error(err), zap.String("key_id", id.String()))
		_ = c.Error(common.InternalError(c))
		return
	}

	task, err := services.Queue.EnqueueRevoke(ctx, subject, id)
	if err != nil {
		logger.Error("failed to queue revocation", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	c.JSON(http.StatusAccepted, toTask(task))
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

		revocable := cand.verified && keyops.SupportsRevocation(cand.account.Provider)

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

	if !keyops.SupportsRevocation(account.Provider) {
		return described
	}

	_, scopes, err := s.Identities.Token(ctx, subject, account.Provider)
	if err != nil {
		logger.Warn("no usable credential for connected account",
			zap.String("provider", string(account.Provider)), zap.Error(err))
		return described
	}

	missing := make([]string, 0, 2)
	for _, scope := range keyops.RevocationScopes(account.Provider) {
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
