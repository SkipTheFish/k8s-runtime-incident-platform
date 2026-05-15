package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) Query(ctx context.Context, query string) (*QueryResponse, error) {
	u := fmt.Sprintf("%s/api/v1/query?query=%s",
		c.baseURL,
		url.QueryEscape(query),
	)

	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result QueryResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	return &result, err
}

func (c *Client) RangeQuery(ctx context.Context, query string, start, end time.Time, step string) (*QueryResponse, error) {

	u := fmt.Sprintf(
		"%s/api/v1/query_range?query=%s&start=%d&end=%d&step=%s",
		c.baseURL,
		url.QueryEscape(query),
		start.Unix(),
		end.Unix(),
		step,
	)

	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result QueryResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	return &result, err
}

func (c *Client) Labels(ctx context.Context) (*QueryResponse, error) {
	u := fmt.Sprintf("%s/api/v1/labels", c.baseURL)

	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result QueryResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	return &result, err
}