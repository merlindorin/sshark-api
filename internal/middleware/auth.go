package middleware

import (
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/apikey"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	"github.com/gin-gonic/gin"
)

// errorKey is the field the auth failures respond with.
const errorKey = "error"

// RequireAuth authenticates a request with Clerk.
//
// clerkConfigured says whether the server actually holds a Clerk key. Without one, Clerk has no
// signing keys to check against and rejects every token as unverifiable — a 401 that reads like
// an expired session but is really a misconfigured server. Saying so plainly turns a confusing
// symptom into an obvious cause.
func RequireAuth(clerkConfigured bool) gin.HandlerFunc {
	sessionMiddleware := AdaptClerk(clerkhttp.RequireHeaderAuthorization())

	return func(c *gin.Context) {
		ctx := c.Request.Context()

		if !clerkConfigured {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				errorKey: "authentication is not configured on this server: CLERK_TOKEN is unset",
			})
			return
		}

		authorization := strings.TrimSpace(c.GetHeader("Authorization"))
		token := strings.TrimPrefix(authorization, "Bearer ")

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{errorKey: "missing authorization token"})
			return
		}

		if strings.HasPrefix(token, "ak_") {
			params := &apikey.VerifyParams{
				Secret: token,
			}

			key, err := apikey.Verify(ctx, params)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{errorKey: "invalid API key"})
				return
			}

			if key.Revoked {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{errorKey: "API key has been revoked"})
				return
			}

			if key.Expired {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{errorKey: "API key has expired"})
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
