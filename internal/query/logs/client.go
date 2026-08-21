package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type LokiClient struct {
	baseURL string
	client  *http.Client
}

func NewLokiClient(baseURL string) *LokiClient {
	return &LokiClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *LokiClient) Query(ctx context.Context, query string, start, end time.Time) (map[string]any, error) {
	u := fmt.Sprintf(
		"%s/loki/api/v1/query_range?query=%s&start=%d&end=%d",
		c.baseURL,
		url.QueryEscape(query),
		start.UnixNano(),
		end.UnixNano(),
	)

	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 检验状态码
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("loki error: %s", string(body))
	}

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)

	return result, err
}
