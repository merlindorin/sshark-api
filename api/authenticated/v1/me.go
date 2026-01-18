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
	ImageURL  *string `json:"image_url,omitempty"`
	CreatedAt int64   `json:"created_at"`
}

func Me(c *gin.Context, logger *zap.Logger) {
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
		Username:  usr.Username,
		ImageURL:  usr.ImageURL,
		CreatedAt: usr.CreatedAt,
	})
}
