package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"observability-platform/internal/httpapi"

	"github.com/gin-gonic/gin"
)

func AdminToken(expected string) gin.HandlerFunc {
	expectedHash := sha256.Sum256([]byte(expected))
	return func(c *gin.Context) {
		if expected == "" {
			httpapi.WriteError(c, http.StatusServiceUnavailable, "ADMIN_API_DISABLED", "administrative API is disabled", nil)
			c.Abort()
			return
		}
		provided := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		providedHash := sha256.Sum256([]byte(provided))
		if subtle.ConstantTimeCompare(expectedHash[:], providedHash[:]) != 1 {
			httpapi.WriteError(c, http.StatusUnauthorized, "ADMIN_UNAUTHORIZED", "administrative authorization failed", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}
