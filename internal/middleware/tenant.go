package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const TenantKey = "tenant"

func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenant := c.GetHeader("X-Tenant-ID")

		if tenant == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "missing tenant",
			})
			c.Abort()
			return
		}

		c.Set(TenantKey, tenant)
		c.Next()
	}
}