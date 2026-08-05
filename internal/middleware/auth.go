package middleware

import (
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/apikey"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/merlindorin/sshark-api/internal/metrics"
)

// errorKey is the field the auth failures respond with.
const errorKey = "error"

// RequireAuth authenticates a request with Clerk.
//
// clerkConfigured says whether the server actually holds a Clerk key. Without one, Clerk has no
// signing keys to check against and rejects every token as unverifiable — a 401 that reads like
// an expired session but is really a misconfigured server. Saying so plainly turns a confusing
// symptom into an obvious cause.
func RequireAuth(clerkConfigured bool, m *metrics.Metrics) gin.HandlerFunc {
	sessionMiddleware := AdaptClerk(clerkhttp.RequireHeaderAuthorization())

	return func(c *gin.Context) {
		ctx := c.Request.Context()

		if !clerkConfigured {
			m.APIAuthAttempts.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("result", "unconfigured"),
					attribute.String("auth_type", "clerk"),
				),
			)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				errorKey: "authentication is not configured on this server: CLERK_TOKEN is unset",
			})
			return
		}

		authorization := strings.TrimSpace(c.GetHeader("Authorization"))
		token := strings.TrimPrefix(authorization, "Bearer ")

		if token == "" {
			m.APIAuthAttempts.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("result", "missing_token"),
					attribute.String("auth_type", "clerk"),
				),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{errorKey: "missing authorization token"})
			return
		}

		if strings.HasPrefix(token, "ak_") {
			params := &apikey.VerifyParams{
				Secret: token,
			}

			key, err := apikey.Verify(ctx, params)
			if err != nil {
				m.APIAuthAttempts.Add(ctx, 1,
					metric.WithAttributes(
						attribute.String("result", "invalid"),
						attribute.String("auth_type", "api_key"),
					),
				)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{errorKey: "invalid API key"})
				return
			}

			if key.Revoked {
				m.APIAuthAttempts.Add(ctx, 1,
					metric.WithAttributes(
						attribute.String("result", "revoked"),
						attribute.String("auth_type", "api_key"),
					),
				)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{errorKey: "API key has been revoked"})
				return
			}

			if key.Expired {
				m.APIAuthAttempts.Add(ctx, 1,
					metric.WithAttributes(
						attribute.String("result", "expired"),
						attribute.String("auth_type", "api_key"),
					),
				)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{errorKey: "API key has expired"})
				return
			}

			m.APIAuthAttempts.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("result", "success"),
					attribute.String("auth_type", "api_key"),
				),
			)

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

		// For session tokens, record success/failure based on the outcome
		// The session middleware will handle the actual verification
		initialStatus := c.Writer.Status()
		sessionMiddleware(c)
		finalStatus := c.Writer.Status()

		if finalStatus == http.StatusUnauthorized || c.IsAborted() {
			m.APIAuthAttempts.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("result", "invalid"),
					attribute.String("auth_type", "session"),
				),
			)
		} else if initialStatus == finalStatus {
			// Session verification succeeded
			m.APIAuthAttempts.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("result", "success"),
					attribute.String("auth_type", "session"),
				),
			)
		}
	}
}
