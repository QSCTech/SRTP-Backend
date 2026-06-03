package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/QSCTech/SRTP-Backend/pkg/response"
	"github.com/gin-gonic/gin"
)

type contextKey string

const AuthUIDKey contextKey = "auth_uid"

const authUIDHeader = "X-Auth-UID"

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authUID := extractAuthUID(c)
		if authUID == "" {
			response.Error(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		setAuthUID(c, authUID)
		c.Next()
	}
}

func CurrentUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		setAuthUID(c, extractAuthUID(c))
		c.Next()
	}
}

func AuthByRoute() gin.HandlerFunc {
	requireAuth := RequireAuth()
	currentUser := CurrentUser()

	return func(c *gin.Context) {
		switch {
		case routeNeedsAuth(c.Request.Method, c.FullPath()):
			requireAuth(c)
		case routeAllowsCurrentUser(c.Request.Method, c.FullPath()):
			currentUser(c)
		default:
			c.Next()
		}
	}
}

func routeNeedsAuth(method, path string) bool {
	switch {
	case method == http.MethodPost && path == "/auth/logout":
		return true
	case strings.HasPrefix(path, "/me"):
		return true
	case method == http.MethodPost && path == "/rooms":
		return true
	case method == http.MethodPost && path == "/rooms/join-by-code":
		return true
	case path == "/rooms/:roomId" && method == http.MethodPut:
		return true
	case strings.HasPrefix(path, "/rooms/:roomId/") && method == http.MethodPost:
		return true
	default:
		return false
	}
}

func routeAllowsCurrentUser(method, path string) bool {
	return method == http.MethodGet && path == "/rooms/:roomId"
}

func extractAuthUID(c *gin.Context) string {
	return strings.TrimSpace(c.GetHeader(authUIDHeader))
}

func setAuthUID(c *gin.Context, authUID string) {
	ctx := context.WithValue(c.Request.Context(), AuthUIDKey, authUID)
	c.Request = c.Request.WithContext(ctx)
}
