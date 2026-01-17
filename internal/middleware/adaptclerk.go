package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func AdaptClerk(clerkMiddleware func(http.Handler) http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var handlerCalled bool

		wrappedHandler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			c.Request = r
			c.Next()
		})

		clerkMiddleware(wrappedHandler).ServeHTTP(c.Writer, c.Request)

		if !handlerCalled {
			c.Abort()
		}
	}
}
