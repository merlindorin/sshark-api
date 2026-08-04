package v1

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/api/common"
	"github.com/merlindorin/sshark-api/api/public"
	"github.com/merlindorin/sshark-api/internal/domain/profiles"
	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
	"github.com/merlindorin/sshark-api/internal/domain/sources"
	"github.com/merlindorin/sshark-api/internal/infra/identity"
)

// maxProfileKeys bounds how many keys one profile page lists per type.
const maxProfileKeys = 200

// GetUserProfile serves the public page of an sshark account. Only accounts the owner proved
// they hold contribute keys, so the page never attributes a stranger's key to them.
func GetUserProfile(
	c *gin.Context,
	logger *zap.Logger,
	profilesRepo profiles.Repository,
	identities *identity.Resolver,
	sourcesRepo sources.Repository,
	publickeysRepo publickeys.Repository,
	username string,
) {
	ctx := c.Request.Context()

	profile, err := profilesRepo.GetByUsername(ctx, profiles.NormalizeUsername(username))
	if err != nil {
		if errors.Is(err, profiles.ErrProfileNotFound) {
			_ = c.Error(common.ProfileNotFoundError(c))
			return
		}
		logger.Error("failed to look up profile", zap.Error(err), zap.String("username", username))
		_ = c.Error(common.InternalError(c))
		return
	}

	details, err := identities.Details(ctx, profile.ClerkUserID)
	if err != nil {
		logger.Error("failed to load profile identity", zap.Error(err), zap.String("username", username))
		_ = c.Error(common.InternalError(c))
		return
	}

	response := public.PublicProfile{
		Username:  profile.Username,
		CreatedAt: profile.CreatedAt,
		Accounts:  make([]public.ProfileAccount, 0, len(details.Accounts)),
		SshKeys:   make([]common.SSHPublicKey, 0),
		GpgKeys:   make([]common.GPGPublicKey, 0),
	}

	if details.DisplayName != "" {
		response.DisplayName = &details.DisplayName
	}
	if details.AvatarURL != "" {
		response.AvatarUrl = &details.AvatarURL
	}
	if details.CreatedAt > 0 {
		joined := time.UnixMilli(details.CreatedAt)
		response.CreatedAt = joined
	}

	if accountsErr := addAccounts(c, sourcesRepo, publickeysRepo, details.Accounts, &response); accountsErr != nil {
		logger.Error("failed to load profile keys", zap.Error(accountsErr), zap.String("username", username))
		_ = c.Error(common.InternalError(c))
		return
	}

	c.JSON(http.StatusOK, response)
}

// addAccounts fills in each connected account and the keys published under it.
func addAccounts(
	c *gin.Context,
	sourcesRepo sources.Repository,
	publickeysRepo publickeys.Repository,
	accounts []identity.Account,
	response *public.PublicProfile,
) error {
	ctx := c.Request.Context()

	for _, account := range accounts {
		provider := string(account.Provider)
		entry := public.ProfileAccount{Provider: provider, Username: account.Username}

		source, err := sourcesRepo.GetByProviderAndUsername(ctx, provider, account.Username)
		if err != nil && !errors.Is(err, sources.ErrSourceNotFound) {
			return err
		}

		// An account sshark has never crawled still belongs on the page, just without keys.
		if source == nil {
			response.Accounts = append(response.Accounts, entry)
			continue
		}

		if source.URI != "" {
			entry.Uri = &source.URI
		}
		response.Accounts = append(response.Accounts, entry)

		sshKeys, err := listKeys(c, publickeysRepo, source, publickeys.KeyTypeSSH)
		if err != nil {
			return err
		}
		for i := range sshKeys {
			response.SshKeys = append(response.SshKeys, entityToSSHPublicKey(sshKeys[i], source))
		}

		gpgKeys, err := listKeys(c, publickeysRepo, source, publickeys.KeyTypeGPG)
		if err != nil {
			return err
		}
		for i := range gpgKeys {
			response.GpgKeys = append(response.GpgKeys, entityToGPGPublicKey(gpgKeys[i], source))
		}
	}

	return nil
}

func listKeys(
	c *gin.Context,
	publickeysRepo publickeys.Repository,
	source *sources.Entity,
	keyType publickeys.KeyType,
) ([]publickeys.Entity, error) {
	result, err := publickeysRepo.Search(c.Request.Context(), publickeys.SearchFilter{
		SourceID: &source.ID,
		KeyType:  &keyType,
	}, maxProfileKeys, 0)
	if err != nil {
		return nil, err
	}

	return result.Entities, nil
}
