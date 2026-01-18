package middleware

import (
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/apikey"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	"github.com/gin-gonic/gin"
)

func RequireAuth() gin.HandlerFunc {
	sessionMiddleware := AdaptClerk(clerkhttp.RequireHeaderAuthorization())

	return func(c *gin.Context) {
		ctx := c.Request.Context()

		authorization := strings.TrimSpace(c.GetHeader("Authorization"))
		token := strings.TrimPrefix(authorization, "Bearer ")

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization token"})
			return
		}

		if strings.HasPrefix(token, "ak_") {
			params := &apikey.VerifyParams{
				Secret: token,
			}

			key, err := apikey.Verify(ctx, params)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
				return
			}

			if key.Revoked {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key has been revoked"})
				return
			}

			if key.Expired {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key has expired"})
				return
			}

			claims := &clerk.SessionClaims{
				RegisteredClaims: clerk.RegisteredClaims{
					Subject: key.Subject,
				},
			}

			newCtx := clerk.ContextWithSessionClaims(ctx, claims)
			c.Request = c.Request.WithContext(newCtx)

			c.Next()
			return
		}

		sessionMiddleware(c)
	}
}
