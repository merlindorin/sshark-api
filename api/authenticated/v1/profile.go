package v1

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/api/authenticated"
	"github.com/merlindorin/sshark-api/api/common"
	"github.com/merlindorin/sshark-api/internal/domain/profiles"
	"github.com/merlindorin/sshark-api/internal/infra/identity"
)

// maxUsernameSuffix bounds the search for a free variant of a taken default username. A user
// who cannot get any of these is asked to pick one instead of the server spinning.
const maxUsernameSuffix = 20

// ProfileServices holds what the profile endpoints need to read and claim usernames.
type ProfileServices struct {
	Profiles   profiles.Repository
	Identities *identity.Resolver
}

// SetMyUsername claims a username for the signed-in user, moving their public profile to it.
func SetMyUsername(c *gin.Context, logger *zap.Logger, services ProfileServices) {
	ctx := c.Request.Context()

	subject, ok := subjectFromContext(c)
	if !ok {
		return
	}

	var req authenticated.SetUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(common.InvalidUsernameError(c, "The request body must contain a username."))
		return
	}

	username := profiles.NormalizeUsername(req.Username)
	if err := profiles.ValidateUsername(username); err != nil {
		_ = c.Error(common.InvalidUsernameError(c, err.Error()))
		return
	}

	// Make sure the profile exists before moving it, so a user who never had one can claim a
	// name in a single call.
	if _, err := services.ensureProfile(ctx, logger, subject); err != nil {
		logger.Error("failed to ensure profile", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	profile, err := services.Profiles.SetUsername(ctx, subject, username)
	if err != nil {
		if errors.Is(err, profiles.ErrUsernameTaken) {
			_ = c.Error(common.UsernameTakenError(c, username))
			return
		}
		logger.Error("failed to set username", zap.Error(err), zap.String("username", username))
		_ = c.Error(common.InternalError(c))
		return
	}

	logger.Info("username claimed", zap.String("username", profile.Username))

	c.JSON(http.StatusOK, toMyProfile(profile))
}

// CheckUsernameAvailable reports whether a username can be claimed, so the UI can say so
// before the user submits.
func CheckUsernameAvailable(
	c *gin.Context,
	logger *zap.Logger,
	services ProfileServices,
	params authenticated.CheckUsernameAvailableParams,
) {
	ctx := c.Request.Context()

	subject, ok := subjectFromContext(c)
	if !ok {
		return
	}

	username := profiles.NormalizeUsername(params.Username)

	if err := profiles.ValidateUsername(username); err != nil {
		c.JSON(http.StatusOK, authenticated.UsernameAvailability{
			Username:  username,
			Available: false,
			Reason:    reasonPtr(err.Error()),
		})
		return
	}

	available, err := services.Profiles.IsUsernameAvailable(ctx, username, subject)
	if err != nil {
		logger.Error("failed to check username", zap.Error(err), zap.String("username", username))
		_ = c.Error(common.InternalError(c))
		return
	}

	response := authenticated.UsernameAvailability{Username: username, Available: available}
	if !available {
		response.Reason = reasonPtr("Someone else already holds this username.")
	}

	c.JSON(http.StatusOK, response)
}

// DeleteMyProfile releases the profile and its username, so an account being deleted does not
// leave its name held forever.
func DeleteMyProfile(c *gin.Context, logger *zap.Logger, services ProfileServices) {
	subject, ok := subjectFromContext(c)
	if !ok {
		return
	}

	if err := services.Profiles.DeleteByClerkUserID(c.Request.Context(), subject); err != nil {
		logger.Error("failed to delete profile", zap.Error(err))
		_ = c.Error(common.InternalError(c))
		return
	}

	logger.Info("profile released", zap.String("subject", subject))

	c.Status(http.StatusNoContent)
}

// ensureProfile returns the user's profile, creating one on first use. The default username
// is the login of the first provider account they connected, which is the name they already
// go by; when that is taken or unusable a numbered variant is used until they pick their own.
func (s ProfileServices) ensureProfile(
	ctx context.Context,
	logger *zap.Logger,
	subject string,
) (*profiles.Entity, error) {
	profile, err := s.Profiles.GetByClerkUserID(ctx, subject)
	if err == nil {
		return profile, nil
	}

	if !errors.Is(err, profiles.ErrProfileNotFound) {
		return nil, err
	}

	accounts, err := s.Identities.Accounts(ctx, subject)
	if err != nil {
		return nil, err
	}

	username, err := s.pickDefaultUsername(ctx, subject, accounts)
	if err != nil {
		return nil, err
	}

	profile = &profiles.Entity{
		ID:          uuid.New(),
		ClerkUserID: subject,
		Username:    username,
	}

	if createErr := s.Profiles.Create(ctx, profile); createErr != nil {
		// Someone claimed the name between the check and the insert. Re-read rather than
		// retrying: a profile now exists either way.
		if errors.Is(createErr, profiles.ErrUsernameTaken) {
			return s.Profiles.GetByClerkUserID(ctx, subject)
		}
		return nil, createErr
	}

	logger.Info("profile created", zap.String("username", profile.Username))

	return profile, nil
}

func (s ProfileServices) pickDefaultUsername(
	ctx context.Context,
	subject string,
	accounts []identity.Account,
) (string, error) {
	candidate := ""
	for _, account := range accounts {
		if profiles.ValidateUsername(account.Username) == nil {
			candidate = account.Username
			break
		}
	}

	// Nothing usable to derive from, so fall back to a name built from the account id. The
	// user is expected to replace it.
	if candidate == "" {
		candidate = fallbackUsername(subject)
	}

	for suffix := 0; suffix <= maxUsernameSuffix; suffix++ {
		attempt := candidate
		if suffix > 0 {
			attempt = candidate + "-" + strconv.Itoa(suffix+1)
		}

		if profiles.ValidateUsername(attempt) != nil {
			continue
		}

		available, err := s.Profiles.IsUsernameAvailable(ctx, attempt, subject)
		if err != nil {
			return "", err
		}
		if available {
			return attempt, nil
		}
	}

	return "", fmt.Errorf("could not derive a free username from %q", candidate)
}

// fallbackUsername builds a valid username from the Clerk user id, which is unique already.
func fallbackUsername(subject string) string {
	trimmed := subject
	const clerkPrefix = "user_"
	if len(subject) > len(clerkPrefix) && subject[:len(clerkPrefix)] == clerkPrefix {
		trimmed = subject[len(clerkPrefix):]
	}

	if len(trimmed) > profiles.MaxUsernameLength {
		trimmed = trimmed[:profiles.MaxUsernameLength]
	}

	return "user-" + trimmed
}

func toMyProfile(profile *profiles.Entity) authenticated.MyProfile {
	return authenticated.MyProfile{
		Username:   profile.Username,
		ProfileUrl: "/@" + profile.Username,
	}
}

func reasonPtr(reason string) *string {
	return &reason
}
