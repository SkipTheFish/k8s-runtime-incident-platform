package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"observability-platform/internal/httpapi"

	"github.com/gin-gonic/gin"
)

func TestRequestIDErrorAndServerTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestIDMiddleware(), TenantMiddleware("server-tenant"))
	router.GET("/error", func(c *gin.Context) {
		if got := c.GetString(TenantKey); got != "server-tenant" {
			t.Fatalf("tenant=%q", got)
		}
		httpapi.WriteError(c, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid request", map[string]string{"field": "page"})
	})
	request := httptest.NewRequest(http.MethodGet, "/error", nil)
	request.Header.Set("X-Tenant-ID", "attacker-controlled")
	request.Header.Set("X-Request-ID", "request-123")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var body httpapi.Error
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadRequest || body.RequestID != "request-123" || response.Header().Get("X-Request-ID") != body.RequestID {
		t.Fatalf("status=%d header=%q body=%+v", response.Code, response.Header().Get("X-Request-ID"), body)
	}
}

func TestRequestLimitsBodyAndTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestIDMiddleware(), RequestLimits(10*time.Millisecond, 4))
	router.POST("/body", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			httpapi.WriteError(c, http.StatusRequestEntityTooLarge, "REQUEST_BODY_TOO_LARGE", "request body exceeds limit", nil)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.GET("/timeout", func(c *gin.Context) {
		<-c.Request.Context().Done()
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/body", strings.NewReader("12345")))
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("body status=%d", response.Code)
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/timeout", nil))
	if response.Code != http.StatusGatewayTimeout || !strings.Contains(response.Body.String(), "REQUEST_TIMEOUT") {
		t.Fatalf("timeout status=%d body=%s", response.Code, response.Body.String())
	}
}
