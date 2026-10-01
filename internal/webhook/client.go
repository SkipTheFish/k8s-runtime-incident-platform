package webhook

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"observability-platform/internal/outbox"
)

type Client struct {
	url        string
	signer     *Signer
	httpClient *http.Client
	now        func() time.Time
}

func NewClient(url, secret string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{url: url, signer: NewSigner(secret), httpClient: httpClient, now: time.Now}
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
		return fmt.Errorf("webhook returned HTTP %d", response.StatusCode)
	}
	return nil
}
