package middleware

import "github.com/gin-gonic/gin"

const TenantKey = "tenant"

func TenantMiddleware(tenant string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(TenantKey, tenant)
		c.Next()
	}
}
