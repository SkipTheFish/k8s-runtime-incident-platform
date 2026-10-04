package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAdminTokenProtectsRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name, configured, provided string
		want                       int
	}{
		{"disabled", "", "", http.StatusServiceUnavailable},
		{"missing", "admin-secret", "", http.StatusUnauthorized},
		{"wrong", "admin-secret", "other", http.StatusUnauthorized},
		{"allowed", "admin-secret", "admin-secret", http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(AdminToken(test.configured))
			router.GET("/admin", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			request := httptest.NewRequest(http.MethodGet, "/admin", nil)
			if test.provided != "" {
				request.Header.Set("Authorization", "Bearer "+test.provided)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
