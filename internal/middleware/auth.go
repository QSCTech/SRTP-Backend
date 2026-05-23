package middleware

import (
	"net/http"
	"strings"

	"github.com/QSCTech/SRTP-Backend/pkg/response"
	"github.com/gin-gonic/gin"
)

const (
	ContextUserIDKey = "userID"
)

func Auth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, http.StatusUnauthorized, "missing authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Error(c, http.StatusUnauthorized, "invalid authorization header format")
			c.Abort()
			return
		}

		tokenString := parts[1]
		userID, err := ParseToken(tokenString, jwtSecret)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set(ContextUserIDKey, userID)
		c.Next()
	}
}