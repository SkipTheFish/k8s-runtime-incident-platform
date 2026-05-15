package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	baseURL string

	httpClient *http.Client
}

func NewClient(baseURL string) *Client {

	return &Client{
		baseURL: baseURL,

		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) GetAlerts(
	ctx context.Context,
) (*AlertsResponse, error) {

	url := fmt.Sprintf(
		"%s/api/v1/alerts",
		c.baseURL,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var result AlertsResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

type QueryResult struct {
	Status string `json:"status"`

	Data struct {
		Result []struct {
			Metric map[string]string `json:"metric"`

			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func (c *Client) Query(
	ctx context.Context,
	query string,
) (float64, error) {

	value, _, err := c.QueryWithLabels(
		ctx,
		query,
	)

	return value, err
}

func (c *Client) QueryWithLabels(
	ctx context.Context,
	query string,
) (float64, map[string]string, error) {

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/api/v1/query",
		nil,
	)
	if err != nil {
		return 0, nil, err
	}

	q := req.URL.Query()
	q.Set("query", query)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer resp.Body.Close()

	var result QueryResult

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, nil, err
	}

	if result.Status != "success" {
		return 0, nil, fmt.Errorf("prometheus query failed: %s", result.Status)
	}

	if len(result.Data.Result) == 0 {
		return 0, map[string]string{}, nil
	}

	first := result.Data.Result[0]

	if len(first.Value) < 2 {
		return 0, first.Metric, nil
	}

	raw := first.Value[1]

	str, ok := raw.(string)
	if !ok {
		return 0, first.Metric, nil
	}

	v, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0, first.Metric, err
	}

	return v, first.Metric, nil
}