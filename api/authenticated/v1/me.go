package v1

import (
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type UserResponse struct {
	ID        string  `json:"id"`
	Email     *string `json:"email,omitempty"`
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Username  *string `json:"username,omitempty"`
	// ProfileURL is the path of the public page this account is served from.
	ProfileURL string  `json:"profile_url"`
	ImageURL   *string `json:"image_url,omitempty"`
	CreatedAt  int64   `json:"created_at"`
}

func Me(c *gin.Context, logger *zap.Logger, services ProfileServices) {
	ctx := c.Request.Context()

	claims, ok := clerk.SessionClaimsFromContext(ctx)
	if !ok {
		logger.Error("failed to get user claims")
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	usr, err := user.Get(ctx, claims.Subject)
	if err != nil {
		logger.Error("failed to get user from clerk", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user"})
		return
	}

	// First visit is where the sshark profile comes into being, defaulting to the login of the
	// first connected account. Everything downstream can then assume a username exists.
	profile, err := services.ensureProfile(ctx, logger, claims.Subject)
	if err != nil {
		logger.Error("failed to ensure profile", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load profile"})
		return
	}

	var primaryEmail *string
	for _, email := range usr.EmailAddresses {
		if email.ID == *usr.PrimaryEmailAddressID {
			primaryEmail = &email.EmailAddress
			break
		}
	}

	c.JSON(http.StatusOK, UserResponse{
		ID:        usr.ID,
		Email:     primaryEmail,
		FirstName: usr.FirstName,
		LastName:  usr.LastName,
		// The sshark username, not Clerk's: it is what /@username resolves on.
		Username:   &profile.Username,
		ProfileURL: "/@" + profile.Username,
		ImageURL:   usr.ImageURL,
		CreatedAt:  usr.CreatedAt,
	})
}
