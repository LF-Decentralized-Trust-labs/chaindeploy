package metrics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"os/exec"

	configservice "github.com/chainlaunch/chainlaunch/pkg/config"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/metrics/common"
	"github.com/chainlaunch/chainlaunch/pkg/metrics/types"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
)

// service implements the Service interface
type service struct {
	manager     *PrometheusManager
	nodeService *nodeservice.NodeService
	db          *db.Queries
}

// NewService creates a new metrics service
func NewService(config *common.Config, db *db.Queries, nodeService *nodeservice.NodeService, configService *configservice.ConfigService) (common.Service, error) {
	manager, err := NewPrometheusManager(config, db, nodeService, configService)
	if err != nil {
		return nil, err
	}
	return &service{
		manager:     manager,
		nodeService: nodeService,
		db:          db,
	}, nil
}

// Start starts the Prometheus instance
func (s *service) Start(ctx context.Context, config *common.Config) error {
	return s.manager.Start(ctx, config)
}

// Stop stops the Prometheus instance
func (s *service) Stop(ctx context.Context) error {
	return s.manager.Stop(ctx)
}

// GetStatus returns the current status of the Prometheus instance
func (s *service) GetStatus(ctx context.Context) (*common.Status, error) {
	return s.manager.GetStatus(ctx)
}

// Reload reloads the Prometheus configuration
func (s *service) Reload(ctx context.Context) error {
	return s.manager.Reload(ctx)
}

// Query executes a PromQL query against Prometheus
func (s *service) Query(ctx context.Context, query string) (*common.QueryResult, error) {
	return s.manager.Query(ctx, query)
}

// QueryRange executes a PromQL query with a time range
func (s *service) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*common.QueryResult, error) {
	return s.manager.QueryRange(ctx, query, start, end, step)
}

// GetLabelValues retrieves values for a specific label
func (s *service) GetLabelValues(ctx context.Context, labelName string, matches []string) ([]string, error) {
	return s.manager.GetLabelValues(ctx, labelName, matches)
}

// GetDefaults returns default values for Prometheus deployment
func (s *service) GetDefaults(ctx context.Context) (interface{}, error) {
	// Find an available port starting from 9090
	availablePort, err := FindAvailablePrometheusPort()
	if err != nil {
		return nil, fmt.Errorf("failed to find available port: %w", err)
	}

	// Get additional available ports for reference
	additionalPorts, err := GetAvailablePrometheusPorts(5)
	if err != nil {
		// If we can't get additional ports, just use the first one
		additionalPorts = []int{availablePort}
	}

	// Release the port we just allocated since this is just for defaults
	ReleasePrometheusPort(availablePort)

	return &types.PrometheusDefaultsResponse{
		DeploymentMode:    common.DeploymentModeService,
		PrometheusVersion: "v3.5.0",
		PrometheusPort:    availablePort,
		ScrapeInterval:    15, // 15 seconds
		AvailablePorts:    additionalPorts,
		DockerConfig: &types.DockerDeployConfig{
			NetworkMode: common.NetworkModeBridge,
		},
	}, nil
}

// CheckPortAvailability checks if a specific port is available for Prometheus
func (s *service) CheckPortAvailability(ctx context.Context, port int) bool {
	return CheckPrometheusPortAvailability(port)
}

// RefreshPrometheus refreshes the Prometheus deployment with new configuration
func (s *service) RefreshPrometheus(ctx context.Context, req interface{}) error {
	// Type assert the request
	refreshReq, ok := req.(*types.RefreshPrometheusRequest)
	if !ok {
		return fmt.Errorf("invalid request type for refresh")
	}

	// Get current status to check if Prometheus is running
	currentStatus, err := s.GetStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current status: %w", err)
	}

	// Get current configuration from database
	currentConfig, err := s.db.GetPrometheusConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current configuration: %w", err)
	}

	// Check if this is a port change in service mode
	isPortChange := false
	previousPort := 0
	if currentStatus.Status == "running" &&
		currentConfig.DeploymentMode == "service" &&
		refreshReq.DeploymentMode == common.DeploymentModeService &&
		refreshReq.PrometheusPort != 0 &&
		refreshReq.PrometheusPort != int(currentConfig.PrometheusPort) {
		isPortChange = true
		previousPort = int(currentConfig.PrometheusPort)
	}

	// If Prometheus is running, stop it first
	if currentStatus.Status == "running" {
		if err := s.Stop(ctx); err != nil {
			return fmt.Errorf("failed to stop current Prometheus instance: %w", err)
		}
	}

	// If this is a port change in service mode, clean up the previous service
	if isPortChange {
		if err := s.cleanupPreviousService(ctx, previousPort); err != nil {
			return fmt.Errorf("failed to cleanup previous service: %w", err)
		}
	}

	// Create new config with updated values, reusing previous data if only port changed
	config := &common.Config{
		PrometheusVersion: refreshReq.PrometheusVersion,
		PrometheusPort:    refreshReq.PrometheusPort,
		ScrapeInterval:    time.Duration(refreshReq.ScrapeInterval) * time.Second,
		DeploymentMode:    refreshReq.DeploymentMode,
	}

	// If only port is being changed, reuse the same data
	if isPortChange {
		// Reuse the same version and scrape interval from current config
		if refreshReq.PrometheusVersion == "" {
			config.PrometheusVersion = currentConfig.PrometheusVersion.String
		}
		if refreshReq.ScrapeInterval == 0 {
			config.ScrapeInterval = time.Duration(currentConfig.ScrapeInterval) * time.Second
		}
	}

	// Configure Docker settings if provided
	if refreshReq.DockerConfig != nil {
		config.DockerConfig = &common.DockerConfig{
			NetworkMode: refreshReq.DockerConfig.NetworkMode,
		}
	}

	// Start with new configuration
	if err := s.Start(ctx, config); err != nil {
		return fmt.Errorf("failed to start Prometheus with new configuration: %w", err)
	}

	return nil
}

// cleanupPreviousService cleans up the previous service when changing ports
func (s *service) cleanupPreviousService(ctx context.Context, previousPort int) error {
	// Get the service type
	serviceType := common.GetServiceType()

	// Determine the previous service name
	var previousServiceName string
	switch serviceType {
	case common.ServiceTypeSystemd:
		previousServiceName = fmt.Sprintf("chainlaunch-prometheus-%d", previousPort)
	case common.ServiceTypeLaunchd:
		previousServiceName = fmt.Sprintf("dev.chainlaunch.prometheus.%d", previousPort)
	default:
		return fmt.Errorf("unsupported service type: %s", serviceType)
	}

	// Stop the previous service
	switch serviceType {
	case common.ServiceTypeSystemd:
		cmd := exec.Command("systemctl", "stop", previousServiceName)
		if err := cmd.Run(); err != nil {
			// Ignore errors if service doesn't exist
			return nil
		}

		// Disable the service
		cmd = exec.Command("systemctl", "disable", previousServiceName)
		cmd.Run() // Ignore errors

		// Remove the service file
		serviceFilePath := fmt.Sprintf("/etc/systemd/system/%s.service", previousServiceName)
		os.Remove(serviceFilePath)

		// Reload systemd
		cmd = exec.Command("systemctl", "daemon-reload")
		cmd.Run() // Ignore errors

	case common.ServiceTypeLaunchd:
		homeDir, _ := os.UserHomeDir()
		plistPath := filepath.Join(homeDir, "Library/LaunchAgents", previousServiceName+".plist")

		// Unload the service
		cmd := exec.Command("launchctl", "unload", plistPath)
		cmd.Run() // Ignore errors

		// Remove the plist file
		os.Remove(plistPath)
	}

	// Release the previous port
	ReleasePrometheusPort(previousPort)

	return nil
}

// QueryMetrics retrieves metrics for a specific node
func (s *service) QueryMetrics(ctx context.Context, nodeID int64, query string) (map[string]interface{}, error) {
	// Get node type and create job name
	node, err := s.nodeService.GetNodeByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	jobName := slugify(fmt.Sprintf("%d-%s", node.ID, node.Name))

	// If no query is provided, use default metrics
	if query == "" {
		query = fmt.Sprintf(`{job="%s"}`, jobName)
	} else {
		// If query is provided, it's just a label, so add job filter
		query = fmt.Sprintf(`%s{job="%s"}`, query, jobName)
	}

	// Query Prometheus for metrics
	result, err := s.manager.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}

	return map[string]interface{}{
		"node_id": nodeID,
		"job":     jobName,
		"query":   query,
		"result":  result,
	}, nil
}

// QueryMetricsRange retrieves metrics for a specific node within a time range
func (s *service) QueryMetricsRange(ctx context.Context, nodeID int64, query string, start, end time.Time, step time.Duration) (map[string]interface{}, error) {
	node, err := s.nodeService.GetNodeByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	jobName := slugify(fmt.Sprintf("%d-%s", node.ID, node.Name))

	// Add job filter to query
	if !strings.Contains(query, "job=") {
		query = fmt.Sprintf(`%s{job="%s"}`, query, jobName)
	}

	// Query Prometheus for metrics with time range
	result, err := s.manager.QueryRange(ctx, query, start, end, step)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics range: %w", err)
	}

	return map[string]interface{}{
		"node_id": nodeID,
		"job":     jobName,
		"query":   query,
		"result":  result,
	}, nil
}

// GetLabelValuesForNode retrieves values for a specific label for a specific node
func (s *service) GetLabelValuesForNode(ctx context.Context, nodeID int64, labelName string, matches []string) ([]string, error) {
	node, err := s.nodeService.GetNodeByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	jobName := slugify(fmt.Sprintf("%d-%s", node.ID, node.Name))
	realMatches := []string{}
	// Add job filter to matches
	for _, match := range matches {
		realMatches = append(realMatches, fmt.Sprintf(`%s{job="%s"}`, match, jobName))
	}

	result, err := s.manager.GetLabelValues(ctx, labelName, realMatches)
	if err != nil {
		return nil, fmt.Errorf("failed to get label values: %w", err)
	}
	return result, nil
}

// QueryForNode executes a PromQL query for a specific node
func (s *service) QueryForNode(ctx context.Context, nodeID int64, query string) (*common.QueryResult, error) {
	node, err := s.nodeService.GetNodeByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	jobName := slugify(fmt.Sprintf("%d-%s", node.ID, node.Name))

	// Add job filter to query if not already present
	if !strings.Contains(query, "job=") {
		query = fmt.Sprintf(`%s{job="%s"}`, query, jobName)
	}

	return s.manager.Query(ctx, query)
}

// QueryRangeForNode executes a PromQL query with a time range for a specific node
func (s *service) QueryRangeForNode(ctx context.Context, nodeID int64, query string, start, end time.Time, step time.Duration) (*common.QueryResult, error) {
	node, err := s.nodeService.GetNodeByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	jobName := slugify(fmt.Sprintf("%d-%s", node.ID, node.Name))

	// Add job filter to query if not already present
	if strings.Contains(query, "{jobName") {
		query = strings.Replace(query, "{jobName}", jobName, 1)
	}

	return s.manager.QueryRange(ctx, query, start, end, step)
}

// GetCurrentConfig returns the current configuration from the database
func (s *service) GetCurrentConfig(ctx context.Context) (*common.Config, error) {
	// Get current configuration from database
	dbConfig, err := s.db.GetPrometheusConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current configuration from database: %w", err)
	}

	// Convert database config to common.Config
	config := &common.Config{
		PrometheusVersion: dbConfig.PrometheusVersion.String,
		PrometheusPort:    int(dbConfig.PrometheusPort),
		ScrapeInterval:    time.Duration(dbConfig.ScrapeInterval) * time.Second,
		DeploymentMode:    common.DeploymentMode(dbConfig.DeploymentMode),
	}

	// Add Docker config if network mode is specified
	if dbConfig.NetworkMode.Valid {
		config.DockerConfig = &common.DockerConfig{
			NetworkMode: common.NetworkMode(dbConfig.NetworkMode.String),
		}
	}

	return config, nil
}

// TailLogs retrieves Prometheus logs with optional tail and follow functionality
func (s *service) TailLogs(ctx context.Context, tail int, follow bool) (<-chan string, error) {
	// Get current deployer from manager
	deployer := s.manager.GetCurrentDeployer()
	if deployer == nil {
		return nil, fmt.Errorf("no active Prometheus deployment found")
	}

	// Use the deployer's TailLogs method
	return deployer.TailLogs(ctx, tail, follow)
}
