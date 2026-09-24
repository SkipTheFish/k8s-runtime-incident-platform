package incident

import (
	"context"

	"observability-platform/internal/httpapi"
)

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return httpapi.WithRequestID(ctx, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	return httpapi.RequestIDFromContext(ctx)
}
