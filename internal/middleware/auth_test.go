package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/merlindorin/sshark-api/internal/middleware"
)

func newRouter(clerkConfigured bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/protected", middleware.RequireAuth(clerkConfigured), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	return router
}

func do(t *testing.T, router *gin.Engine, authorization string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

// TestRequireAuthWithoutClerkKey covers the failure that took a production deployment 136 days
// to notice: with no Clerk key the server answers every authenticated request, and a bare 401
// reads as an expired session rather than a server that was never configured.
func TestRequireAuthWithoutClerkKey(t *testing.T) {
	t.Parallel()

	router := newRouter(false)

	for name, authorization := range map[string]string{
		"with a token":    "Bearer some-session-token",
		"without a token": "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rec := do(t, router, authorization)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
			}

			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response is not JSON: %v (body %q)", err, rec.Body.String())
			}

			// The message has to name the cause, or the next person debugs the client instead.
			if !strings.Contains(body["error"], "CLERK_TOKEN") {
				t.Errorf("error = %q, want it to mention CLERK_TOKEN", body["error"])
			}
		})
	}
}

// TestRequireAuthWithClerkKey checks a configured server still rejects a missing token the
// ordinary way, rather than reporting the server as unconfigured.
func TestRequireAuthWithClerkKey(t *testing.T) {
	t.Parallel()

	rec := do(t, newRouter(true), "")

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
