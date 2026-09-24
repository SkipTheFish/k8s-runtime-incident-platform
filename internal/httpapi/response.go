package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const requestIDKey = "requestID"

type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestID"`
	Details   any    `json:"details,omitempty"`
}

type Page struct {
	Items      any    `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
	Page       int    `json:"page,omitempty"`
	PageSize   int    `json:"pageSize"`
	Total      int64  `json:"total,omitempty"`
}

func RequestID(c *gin.Context) string { return c.GetString(requestIDKey) }

func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey{}).(string)
	return value
}

type requestIDContextKey struct{}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func SetRequestID(c *gin.Context, requestID string) {
	c.Set(requestIDKey, requestID)
	c.Request = c.Request.WithContext(WithRequestID(c.Request.Context(), requestID))
}

func WriteError(c *gin.Context, status int, code, message string, details any) {
	c.JSON(status, Error{Code: code, Message: message, RequestID: RequestID(c), Details: details})
}

func WriteMappedError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		WriteError(c, http.StatusGatewayTimeout, "REQUEST_TIMEOUT", "request timed out", nil)
	case errors.Is(err, context.Canceled):
		WriteError(c, http.StatusRequestTimeout, "REQUEST_CANCELED", "request was canceled", nil)
	default:
		WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
	}
}
