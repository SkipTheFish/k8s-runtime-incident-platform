package traces

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type TempoClient struct {
	baseURL string
	client  *http.Client
}

func NewTempoClient(baseURL string) *TempoClient {
	return &TempoClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// 查询单条 trace
func (c *TempoClient) GetTrace(ctx context.Context, traceID string) (map[string]any, error) {
	url := fmt.Sprintf("%s/api/traces/%s", c.baseURL, traceID)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tempo error: %s", string(body))
	}

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

// 简化 search（Tempo 默认 API 很复杂，这里做 mock 级别）
func (c *TempoClient) Search(ctx context.Context, service string) (map[string]any, error) {
	return map[string]any{
		"message": "tempo search (mock)",
		"service": service,
	}, nil
}
