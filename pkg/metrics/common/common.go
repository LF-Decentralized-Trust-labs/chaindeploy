package common

import (
	"context"
	"runtime"
	"time"
)

// DeploymentMode represents the deployment mode for Prometheus
type DeploymentMode string

const (
	DeploymentModeDocker  DeploymentMode = "docker"
	DeploymentModeService DeploymentMode = "service"
)

// NetworkMode represents the Docker network mode
type NetworkMode string

const (
	NetworkModeBridge NetworkMode = "bridge"
	NetworkModeHost   NetworkMode = "host"
)

// ServiceType represents the type of system service
type ServiceType string

const (
	ServiceTypeSystemd ServiceType = "systemd"
	ServiceTypeLaunchd ServiceType = "launchd"
)

// Config represents the configuration for the metrics service
type Config struct {
	// PrometheusVersion is the version of Prometheus to deploy
	PrometheusVersion string
	// PrometheusPort is the port Prometheus will listen on
	PrometheusPort int
	// ScrapeInterval is the interval between scrapes
	ScrapeInterval time.Duration
	// DeploymentMode is the deployment mode (docker or service)
	DeploymentMode DeploymentMode
	// DockerConfig contains Docker-specific configuration
	DockerConfig *DockerConfig
}

// DockerConfig contains Docker-specific configuration
type DockerConfig struct {
	// NetworkMode is the Docker network mode
	NetworkMode NetworkMode
}

// DefaultConfig returns a default configuration for the metrics service
func DefaultConfig() *Config {
	return &Config{
		PrometheusVersion: "v3.5.0",
		PrometheusPort:    9090,
		ScrapeInterval:    15 * time.Second,
		DeploymentMode:    DeploymentModeDocker,
		DockerConfig: &DockerConfig{
			NetworkMode: NetworkModeBridge,
		},
	}
}

// GetServiceType returns the appropriate service type for the current OS
func GetServiceType() ServiceType {
	switch runtime.GOOS {
	case "linux":
		return ServiceTypeSystemd
	case "darwin":
		return ServiceTypeLaunchd
	default:
		return ServiceTypeSystemd
	}
}

// Status represents the status of a Prometheus instance
type Status struct {
	// Status is the current status (running, stopped, etc.)
	Status string `json:"status"`
	// Version is the Prometheus version
	Version string `json:"version,omitempty"`
	// Port is the port Prometheus is listening on
	Port int `json:"port,omitempty"`
	// ScrapeInterval is the current scrape interval
	ScrapeInterval time.Duration `json:"scrape_interval,omitempty"`
	// DeploymentMode is the current deployment mode
	DeploymentMode DeploymentMode `json:"deployment_mode,omitempty"`
	// NetworkMode is the Docker network mode (only for Docker deployments)
	NetworkMode NetworkMode `json:"network_mode,omitempty"`
	// StartedAt is when the instance was started
	StartedAt *time.Time `json:"started_at,omitempty"`
	// Error contains any error message
	Error string `json:"error,omitempty"`
}

// QueryResult represents the result of a PromQL query
type QueryResult struct {
	// Status is the query status
	Status string `json:"status"`
	// Data contains the query results
	Data interface{} `json:"data,omitempty"`
	// Error contains any error message
	Error string `json:"error,omitempty"`
}

// Service defines the interface for metrics operations
type Service interface {
	// Start starts the Prometheus instance
	Start(ctx context.Context, config *Config) error
	// Stop stops the Prometheus instance
	Stop(ctx context.Context) error
	// GetStatus returns the current status of the Prometheus instance
	GetStatus(ctx context.Context) (*Status, error)
	// Reload reloads the Prometheus configuration
	Reload(ctx context.Context) error
	// Query executes a PromQL query against Prometheus
	Query(ctx context.Context, query string) (*QueryResult, error)
	// QueryRange executes a PromQL query with a time range
	QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*QueryResult, error)
	// GetLabelValues retrieves values for a specific label
	GetLabelValues(ctx context.Context, labelName string, matches []string) ([]string, error)
	// GetDefaults returns default values for Prometheus deployment
	GetDefaults(ctx context.Context) (interface{}, error)
	// CheckPortAvailability checks if a specific port is available for Prometheus
	CheckPortAvailability(ctx context.Context, port int) bool
	// RefreshPrometheus refreshes the Prometheus deployment with new configuration
	RefreshPrometheus(ctx context.Context, req interface{}) error
	// QueryMetrics retrieves metrics for a specific node
	QueryMetrics(ctx context.Context, nodeID int64, query string) (map[string]interface{}, error)
	// QueryMetricsRange retrieves metrics for a specific node within a time range
	QueryMetricsRange(ctx context.Context, nodeID int64, query string, start, end time.Time, step time.Duration) (map[string]interface{}, error)
	// GetLabelValuesForNode retrieves values for a specific label for a specific node
	GetLabelValuesForNode(ctx context.Context, nodeID int64, labelName string, matches []string) ([]string, error)
	// QueryForNode executes a PromQL query for a specific node
	QueryForNode(ctx context.Context, nodeID int64, query string) (*QueryResult, error)
	// QueryRangeForNode executes a PromQL query with a time range for a specific node
	QueryRangeForNode(ctx context.Context, nodeID int64, query string, start, end time.Time, step time.Duration) (*QueryResult, error)
	// GetCurrentConfig returns the current configuration from the database
	GetCurrentConfig(ctx context.Context) (*Config, error)
	// TailLogs retrieves Prometheus logs with optional tail and follow functionality
	TailLogs(ctx context.Context, tail int, follow bool) (<-chan string, error)
}
