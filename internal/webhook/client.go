package webhook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"observability-platform/internal/outbox"
)

type Client struct {
	url        string
	signer     *Signer
	httpClient *http.Client
	now        func() time.Time
}

type DeliveryError struct {
	retryable  bool
	status     int
	summary    string
	retryAfter time.Duration
}

func (e *DeliveryError) Error() string {
	return fmt.Sprintf("webhook delivery failed with HTTP %d", e.status)
}
func (e *DeliveryError) Retryable() bool           { return e.retryable }
func (e *DeliveryError) HTTPStatus() int           { return e.status }
func (e *DeliveryError) ResponseSummary() string   { return e.summary }
func (e *DeliveryError) RetryAfter() time.Duration { return e.retryAfter }

func NewClient(url, secret string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{url: url, signer: NewSigner(secret), httpClient: httpClient, now: time.Now}
}

func NewSecureClient(rawURL, secret string, policy *NetworkPolicy, connectTimeout, requestTimeout time.Duration) (*Client, error) {
	if _, err := policy.ValidateURL(rawURL); err != nil {
		return nil, err
	}
	return NewClient(rawURL, secret, policy.HTTPClient(connectTimeout, requestTimeout)), nil
}

func (c *Client) Deliver(ctx context.Context, event outbox.Event) error {
	now := c.now().UTC()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(event.Payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(HeaderTimestamp, strconv.FormatInt(now.Unix(), 10))
	request.Header.Set(HeaderEventID, event.EventID)
	request.Header.Set(HeaderSignature, c.signer.Sign(now, event.EventID, event.Payload))
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4097))
		return &DeliveryError{retryable: response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500, status: response.StatusCode, summary: sanitizeResponse(string(body)), retryAfter: ParseRetryAfter(response.Header.Get("Retry-After"), now, time.Minute)}
	}
	return nil
}

func ParseRetryAfter(value string, now time.Time, maximum time.Duration) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0
		}
		delay := time.Duration(seconds) * time.Second
		if delay > maximum {
			return maximum
		}
		return delay
	}
	when, err := http.ParseTime(value)
	if err != nil || !when.After(now) {
		return 0
	}
	delay := when.Sub(now)
	if delay > maximum {
		return maximum
	}
	return delay
}

func sanitizeResponse(value string) string {
	if len(value) > 4096 {
		value = value[:4096] + "...[TRUNCATED]"
	}
	lower := strings.ToLower(value)
	for _, key := range []string{"authorization", "password", "passwd", "secret", "token"} {
		if strings.Contains(lower, key) {
			return "[REDACTED RESPONSE BODY]"
		}
	}
	return value
}
