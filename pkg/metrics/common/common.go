package common

import (
	"context"
	"time"
)

// Config represents the configuration for the metrics service
type Config struct {
	// PrometheusVersion is the version of Prometheus to deploy
	PrometheusVersion string
	// PrometheusPort is the port Prometheus will listen on
	PrometheusPort int
	// ScrapeInterval is the interval between scrapes
	ScrapeInterval time.Duration
	// DeploymentMode specifies how Prometheus is deployed (currently only supports "docker")
	DeploymentMode string
}

// DefaultConfig returns a Config with sensible default values
func DefaultConfig() *Config {
	return &Config{
		PrometheusVersion: "v3.3.1",
		PrometheusPort:    9090,
		ScrapeInterval:    15 * time.Second,
		DeploymentMode:    "docker",
	}
}

// Service defines the interface for metrics operations
type Service interface {
	// Start starts the Prometheus instance with the given configuration
	Start(ctx context.Context, config *Config) error

	// Stop stops the Prometheus instance
	Stop(ctx context.Context) error

	// QueryMetrics retrieves metrics for a specific node
	QueryMetrics(ctx context.Context, nodeID int64, query string) (map[string]interface{}, error)

	// QueryMetricsRange retrieves metrics for a specific node within a time range
	QueryMetricsRange(ctx context.Context, nodeID int64, query string, start, end time.Time, step time.Duration) (map[string]interface{}, error)

	// GetLabelValues retrieves values for a specific label
	GetLabelValues(ctx context.Context, nodeID int64, labelName string, matches []string) ([]string, error)

	// Reload reloads the Prometheus configuration
	Reload(ctx context.Context) error

	// Query executes a PromQL query for a specific node
	Query(ctx context.Context, nodeID int64, query string) (*QueryResult, error)

	// QueryRange executes a PromQL query with a time range for a specific node
	QueryRange(ctx context.Context, nodeID int64, query string, start, end time.Time, step time.Duration) (*QueryResult, error)
}

// QueryResult represents the result of a Prometheus query
type QueryResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			// For instant queries
			Value []interface{} `json:"value,omitempty"`
			// For range queries (matrix)
			Values [][]interface{} `json:"values,omitempty"`
		} `json:"result"`
	} `json:"data"`
}
