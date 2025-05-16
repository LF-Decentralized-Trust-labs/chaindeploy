package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/metrics/common"
)

// Client handles querying Prometheus for metrics
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new metrics client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Query executes a PromQL query against Prometheus
func (c *Client) Query(ctx context.Context, query string) (*common.QueryResult, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	u.Path = "/api/v1/query"
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result common.QueryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// QueryRange executes a PromQL query with a time range
func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*common.QueryResult, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	u.Path = "/api/v1/query_range"
	q := u.Query()
	q.Set("query", query)
	q.Set("start", fmt.Sprintf("%d", start.Unix()))
	q.Set("end", fmt.Sprintf("%d", end.Unix()))
	q.Set("step", step.String())
	u.RawQuery = q.Encode()
	queryUrl := u.String()
	req, err := http.NewRequestWithContext(ctx, "GET", queryUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read the response body for error details
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("unexpected status code: %d (failed to read response body: %v)", resp.StatusCode, readErr)
		}

		// Reset the response body for potential further reads
		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	var result common.QueryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Common metric queries
const (
	// NodeCPUUsage returns CPU usage percentage for a node
	NodeCPUUsage = `rate(node_cpu_seconds_total{mode="user"}[5m]) * 100`
	// NodeMemoryUsage returns memory usage percentage for a node
	NodeMemoryUsage = `(node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) / node_memory_MemTotal_bytes * 100`
	// NodeDiskUsage returns disk usage percentage for a node
	NodeDiskUsage = `(node_filesystem_size_bytes{mountpoint="/"} - node_filesystem_free_bytes{mountpoint="/"}) / node_filesystem_size_bytes{mountpoint="/"} * 100`
	// NodeNetworkIO returns network I/O in bytes per second
	NodeNetworkIO = `rate(node_network_receive_bytes_total[5m])`
)

// GetNodeMetrics returns common metrics for a specific node
func (c *Client) GetNodeMetrics(ctx context.Context, nodeName string) (map[string]float64, error) {
	metrics := make(map[string]float64)

	// Query CPU usage
	cpuResult, err := c.Query(ctx, fmt.Sprintf(`%s{instance="%s"}`, NodeCPUUsage, nodeName))
	if err != nil {
		return nil, fmt.Errorf("failed to query CPU usage: %w", err)
	}
	if len(cpuResult.Data.Result) > 0 {
		if value, ok := cpuResult.Data.Result[0].Value[1].(float64); ok {
			metrics["cpu_usage"] = value
		}
	}

	// Query memory usage
	memResult, err := c.Query(ctx, fmt.Sprintf(`%s{instance="%s"}`, NodeMemoryUsage, nodeName))
	if err != nil {
		return nil, fmt.Errorf("failed to query memory usage: %w", err)
	}
	if len(memResult.Data.Result) > 0 {
		if value, ok := memResult.Data.Result[0].Value[1].(float64); ok {
			metrics["memory_usage"] = value
		}
	}

	// Query disk usage
	diskResult, err := c.Query(ctx, fmt.Sprintf(`%s{instance="%s"}`, NodeDiskUsage, nodeName))
	if err != nil {
		return nil, fmt.Errorf("failed to query disk usage: %w", err)
	}
	if len(diskResult.Data.Result) > 0 {
		if value, ok := diskResult.Data.Result[0].Value[1].(float64); ok {
			metrics["disk_usage"] = value
		}
	}

	return metrics, nil
}

// GetLabelValues retrieves values for a specific label
func (c *Client) GetLabelValues(ctx context.Context, labelName string, matches []string) ([]string, error) {
	baseURL := fmt.Sprintf("%s/api/v1/label/%s/values", c.baseURL, labelName)

	queryUrl := baseURL
	if len(matches) > 0 {
		var matchParams []string
		for _, match := range matches {
			// URL encode the match parameter to handle special characters
			encodedMatch := url.QueryEscape(match)
			matchParams = append(matchParams, "match[]="+encodedMatch)
		}
		queryUrl = baseURL + "?" + strings.Join(matchParams, "&")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", queryUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prometheus API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus API returned non-success status: %s", result.Status)
	}

	return result.Data, nil
}
