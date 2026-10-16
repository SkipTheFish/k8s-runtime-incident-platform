package webhook

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"observability-platform/internal/outbox"
)

func TestClientClassifiesResponsesAndSanitizesBody(t *testing.T) {
	for _, test := range []struct {
		status    int
		retryable bool
	}{{204, false}, {400, false}, {429, true}, {503, true}} {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "30")
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(`{"token":"must-not-persist"}`))
			}))
			defer server.Close()
			err := NewClient(server.URL, "secret", server.Client()).Deliver(context.Background(), outbox.Event{EventID: "event", Payload: []byte(`{}`)})
			if test.status == 204 {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var failure *DeliveryError
			if !errors.As(err, &failure) || failure.Retryable() != test.retryable || failure.ResponseSummary() != "[REDACTED RESPONSE BODY]" || failure.RetryAfter() != 30*time.Second {
				t.Fatalf("failure=%+v err=%v", failure, err)
			}
		})
	}
}

func TestParseRetryAfterIsBounded(t *testing.T) {
	now := time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC)
	if got := ParseRetryAfter("120", now, time.Minute); got != time.Minute {
		t.Fatalf("seconds=%v", got)
	}
	if got := ParseRetryAfter(now.Add(30*time.Second).Format(http.TimeFormat), now, time.Minute); got != 30*time.Second {
		t.Fatalf("date=%v", got)
	}
	if got := ParseRetryAfter("invalid", now, time.Minute); got != 0 {
		t.Fatalf("invalid=%v", got)
	}
}

func TestDeliveryErrorContractAndRetryAfterEdges(t *testing.T) {
	failure := &DeliveryError{retryable: true, status: http.StatusServiceUnavailable, summary: "unavailable", retryAfter: time.Second}
	if failure.Error() != "webhook delivery failed with HTTP 503" || failure.HTTPStatus() != http.StatusServiceUnavailable {
		t.Fatalf("failure contract=%+v error=%q", failure, failure.Error())
	}
	now := time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC)
	for _, value := range []string{"", "-1", now.Add(-time.Second).Format(http.TimeFormat)} {
		if got := ParseRetryAfter(value, now, time.Minute); got != 0 {
			t.Fatalf("ParseRetryAfter(%q)=%v", value, got)
		}
	}
	if got := sanitizeResponse(strings.Repeat("x", 5000)); len(got) != 4110 || !strings.HasSuffix(got, "...[TRUNCATED]") {
		t.Fatalf("sanitized response length=%d suffix=%q", len(got), got[len(got)-14:])
	}
	if NewClient("https://example.invalid", "secret", nil).httpClient != http.DefaultClient {
		t.Fatal("nil HTTP client did not use the default client")
	}
}
