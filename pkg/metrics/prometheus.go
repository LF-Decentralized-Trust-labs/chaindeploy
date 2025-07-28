package metrics

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"os/exec"
	"path/filepath"
	"runtime"

	"database/sql"

	configservice "github.com/chainlaunch/chainlaunch/pkg/config"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/metrics/common"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/yaml.v2"
)

// slugify converts a string to a URL-friendly slug
func slugify(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Replace spaces and special characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	s = reg.ReplaceAllString(s, "-")

	// Remove leading and trailing hyphens
	s = strings.Trim(s, "-")

	return s
}

// PrometheusDeployer defines the interface for different Prometheus deployment methods
type PrometheusDeployer interface {
	// Start starts the Prometheus instance
	Start(ctx context.Context) error
	// Stop stops the Prometheus instance
	Stop(ctx context.Context) error
	// Reload reloads the Prometheus configuration
	Reload(ctx context.Context) error
	// GetStatus returns the current status of the Prometheus instance
	GetStatus(ctx context.Context) (string, error)
	// TailLogs retrieves Prometheus logs with optional tail and follow functionality
	TailLogs(ctx context.Context, tail int, follow bool) (<-chan string, error)
}

// DockerPrometheusDeployer implements PrometheusDeployer for Docker deployment
type DockerPrometheusDeployer struct {
	config        *common.Config
	client        *client.Client
	db            *db.Queries
	nodeService   *nodeservice.NodeService
	configDir     string
	dataDir       string
	binDir        string
	configService *configservice.ConfigService
}

// ServicePrometheusDeployer implements PrometheusDeployer for system service deployment
type ServicePrometheusDeployer struct {
	config        *common.Config
	db            *db.Queries
	nodeService   *nodeservice.NodeService
	serviceType   common.ServiceType
	configDir     string
	dataDir       string
	binDir        string
	configService *configservice.ConfigService
}

// NewDockerPrometheusDeployer creates a new Docker-based Prometheus deployer
func NewDockerPrometheusDeployer(config *common.Config, db *db.Queries, nodeService *nodeservice.NodeService, configService *configservice.ConfigService) (*DockerPrometheusDeployer, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	// Use the nodeService's configService to determine the data directory
	// This follows the same pattern as other components like peers, orderers, and besu nodes
	chainlaunchDir := configService.GetDataPath()
	prometheusDir := filepath.Join(chainlaunchDir, "prometheus")

	// Use the same directories regardless of port - only the service arguments change
	configDir := filepath.Join(prometheusDir, "config")
	prometheusDataDir := filepath.Join(prometheusDir, "data")
	binDir := filepath.Join(prometheusDir, "bin")

	// Create config service with the determined data path

	return &DockerPrometheusDeployer{
		config:        config,
		client:        cli,
		db:            db,
		nodeService:   nodeService,
		configDir:     configDir,
		dataDir:       prometheusDataDir,
		binDir:        binDir,
		configService: configService,
	}, nil
}

// NewServicePrometheusDeployer creates a new service-based Prometheus deployer
func NewServicePrometheusDeployer(config *common.Config, db *db.Queries, nodeService *nodeservice.NodeService, configService *configservice.ConfigService) (*ServicePrometheusDeployer, error) {
	serviceType := common.GetServiceType()

	// Use the nodeService's configService to determine the data directory
	// This follows the same pattern as other components like peers, orderers, and besu nodes
	chainlaunchDir := configService.GetDataPath()
	prometheusDir := filepath.Join(chainlaunchDir, "prometheus")

	// Use the same directories regardless of port - only the service arguments change
	configDir := filepath.Join(prometheusDir, "config")
	prometheusDataDir := filepath.Join(prometheusDir, "data")
	binDir := filepath.Join(prometheusDir, "bin")

	deployer := &ServicePrometheusDeployer{
		config:        config,
		db:            db,
		nodeService:   nodeService,
		serviceType:   serviceType,
		configDir:     configDir,
		dataDir:       prometheusDataDir,
		binDir:        binDir,
		configService: configService,
	}

	return deployer, nil
}

// getServiceName returns the systemd service name with port
func (s *ServicePrometheusDeployer) getServiceName() string {
	return fmt.Sprintf("chainlaunch-prometheus-%d", s.config.PrometheusPort)
}

// getLaunchdServiceName returns the launchd service name with port
func (s *ServicePrometheusDeployer) getLaunchdServiceName() string {
	return fmt.Sprintf("dev.chainlaunch.prometheus.%d", s.config.PrometheusPort)
}

// getServiceFilePath returns the path to the service file
func (s *ServicePrometheusDeployer) getServiceFilePath() string {
	switch s.serviceType {
	case common.ServiceTypeSystemd:
		return fmt.Sprintf("/etc/systemd/system/%s.service", s.getServiceName())
	case common.ServiceTypeLaunchd:
		return s.getLaunchdPlistPath()
	default:
		return ""
	}
}

// getLaunchdPlistPath returns the launchd plist file path
func (s *ServicePrometheusDeployer) getLaunchdPlistPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "Library/LaunchAgents", s.getLaunchdServiceName()+".plist")
}

// GetStdOutPath returns the path to the stdout log file
func (s *ServicePrometheusDeployer) GetStdOutPath() string {
	return filepath.Join(s.dataDir, "prometheus", fmt.Sprintf("%s.log", s.getServiceName()))
}

// Start starts the Prometheus container
func (d *DockerPrometheusDeployer) Start(ctx context.Context) error {
	containerName := fmt.Sprintf("chainlaunch-prometheus-%d", d.config.PrometheusPort)
	// Remove any existing container
	containerDocker, err := d.client.ContainerInspect(ctx, containerName)
	if err == nil {
		if containerDocker.State.Running {
			if err := d.client.ContainerStop(ctx, containerName, container.StopOptions{}); err != nil {
				return fmt.Errorf("failed to stop existing container: %w", err)
			}
		}
		if err := d.client.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: true}); err != nil {
			return fmt.Errorf("failed to remove existing container: %w", err)
		}
	}
	if err != nil && !client.IsErrNotFound(err) {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	// Create directories if they don't exist
	dirs := []string{d.configDir, d.dataDir, d.binDir, filepath.Join(d.dataDir, "prometheus")}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Check if config file already exists (could be from migration)
	configPath := filepath.Join(d.configDir, "prometheus.yml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Config file doesn't exist, generate it
		configData, err := d.buildPrometheusConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to generate Prometheus config: %w", err)
		}

		// Write config file
		if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}
	} else {
		// Config file exists, update it with current configuration
		configData, err := d.buildPrometheusConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to generate Prometheus config: %w", err)
		}

		// Write updated config file
		if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}
	}

	// Pull Prometheus image
	imageName := fmt.Sprintf("prom/prometheus:%s", d.config.PrometheusVersion)
	reader, err := d.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull Prometheus image: %w", err)
	}
	_, err = io.Copy(os.Stdout, reader)
	if err != nil {
		return fmt.Errorf("failed to copy Prometheus image: %w", err)
	}

	// Build command with extra args
	cmd := []string{
		"--config.file=/etc/prometheus/prometheus.yml",
		"--storage.tsdb.path=/prometheus",
		"--web.console.libraries=/usr/share/prometheus/console_libraries",
		"--web.console.templates=/usr/share/prometheus/consoles",
		"--web.enable-lifecycle",
		"--web.enable-admin-api",
		fmt.Sprintf("--web.listen-address=0.0.0.0:%d", d.config.PrometheusPort),
	}

	// Create container config
	containerConfig := &container.Config{
		Image: imageName,
		Cmd:   cmd,
		ExposedPorts: nat.PortSet{
			nat.Port(fmt.Sprintf("%d/tcp", d.config.PrometheusPort)): struct{}{},
		},
	}

	// Create host config with bind mounts to local directories
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: d.dataDir,
				Target: "/prometheus",
			},
			{
				Type:   mount.TypeBind,
				Source: d.configDir,
				Target: "/etc/prometheus",
			},
		},
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyMode("unless-stopped"),
		},
	}

	// Configure network mode and port bindings
	if d.config.DockerConfig.NetworkMode == common.NetworkModeHost {
		hostConfig.NetworkMode = container.NetworkMode("host")
	} else {
		// Bridge mode - configure port bindings and add extra hosts
		hostConfig.PortBindings = nat.PortMap{
			nat.Port(fmt.Sprintf("%d/tcp", d.config.PrometheusPort)): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: fmt.Sprintf("%d", d.config.PrometheusPort),
				},
			},
		}
		// Add extra hosts for bridge mode to allow host.docker.internal to resolve to host-gateway
		hostConfig.ExtraHosts = []string{"host.docker.internal:host-gateway"}
	}

	// Create container
	resp, err := d.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, &v1.Platform{}, containerName)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	// Start container
	if err := d.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	// Wait for container to be ready and active
	if err := d.waitForPrometheusActive(ctx); err != nil {
		return fmt.Errorf("failed to wait for Prometheus to be active: %w", err)
	}

	// Update database with current configuration
	if err := d.updateDatabaseConfig(ctx); err != nil {
		return fmt.Errorf("failed to update database configuration: %w", err)
	}

	// Reload configuration
	return d.Reload(ctx)
}

// Stop stops the Prometheus container
func (d *DockerPrometheusDeployer) Stop(ctx context.Context) error {
	containerName := fmt.Sprintf("chainlaunch-prometheus-%d", d.config.PrometheusPort)

	// Stop container
	if err := d.client.ContainerStop(ctx, containerName, container.StopOptions{}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Remove container
	if err := d.client.ContainerRemove(ctx, containerName, container.RemoveOptions{
		Force: true,
	}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}

// Reload reloads the Prometheus configuration
func (d *DockerPrometheusDeployer) Reload(ctx context.Context) error {
	containerName := fmt.Sprintf("chainlaunch-prometheus-%d", d.config.PrometheusPort)

	configData, err := d.buildPrometheusConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to build Prometheus config: %w", err)
	}

	// Write config file to local directory (will be mounted in container)
	configPath := filepath.Join(d.configDir, "prometheus.yml")
	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Reload Prometheus configuration using POST request
	reloadExecID, err := d.client.ContainerExecCreate(ctx, containerName, container.ExecOptions{
		Cmd: []string{"curl", "-X", "POST", fmt.Sprintf("http://localhost:%d/-/reload", d.config.PrometheusPort)},
	})
	if err != nil {
		return fmt.Errorf("failed to create reload command: %w", err)
	}

	if err := d.client.ContainerExecStart(ctx, reloadExecID.ID, container.ExecStartOptions{}); err != nil {
		return fmt.Errorf("failed to start reload command: %w", err)
	}

	return nil
}

// waitForPrometheusActive waits for Prometheus to be active and ready to accept requests
func (d *DockerPrometheusDeployer) waitForPrometheusActive(ctx context.Context) error {
	containerName := fmt.Sprintf("chainlaunch-prometheus-%d", d.config.PrometheusPort)

	// Wait for container to be running
	maxWaitTime := 60 * time.Second
	checkInterval := 2 * time.Second
	elapsed := time.Duration(0)

	for elapsed < maxWaitTime {
		// Check container status
		dockerContainer, err := d.client.ContainerInspect(ctx, containerName)
		if err != nil {
			if client.IsErrNotFound(err) {
				time.Sleep(checkInterval)
				elapsed += checkInterval
				continue
			}
			return fmt.Errorf("failed to inspect container: %w", err)
		}

		if dockerContainer.State.Status != "running" {
			time.Sleep(checkInterval)
			elapsed += checkInterval
			continue
		}

		// Container is running, now check if Prometheus is ready
		// Try to connect to the health endpoint
		healthURL := fmt.Sprintf("http://localhost:%d/-/healthy", d.config.PrometheusPort)

		// For bridge mode, we need to check from inside the container
		if d.config.DockerConfig.NetworkMode != common.NetworkModeHost {
			// Execute curl command inside the container to check health
			execResp, err := d.client.ContainerExecCreate(ctx, containerName, container.ExecOptions{
				Cmd: []string{"wget", "-q", "--spider", healthURL},
			})
			if err != nil {
				time.Sleep(checkInterval)
				elapsed += checkInterval
				continue
			}

			if err := d.client.ContainerExecStart(ctx, execResp.ID, container.ExecStartOptions{}); err != nil {
				time.Sleep(checkInterval)
				elapsed += checkInterval
				continue
			}

			// Check the exit code
			inspectResp, err := d.client.ContainerExecInspect(ctx, execResp.ID)
			if err != nil || inspectResp.ExitCode != 0 {
				time.Sleep(checkInterval)
				elapsed += checkInterval
				continue
			}
		} else {
			// For host mode, check directly from host
			resp, err := http.Get(healthURL)
			if err != nil || resp.StatusCode != 200 {
				time.Sleep(checkInterval)
				elapsed += checkInterval
				continue
			}
			resp.Body.Close()
		}

		// Prometheus is ready
		return nil
	}

	return fmt.Errorf("timeout waiting for Prometheus to be active after %v", maxWaitTime)
}

// PrometheusConfig represents the Prometheus configuration structure
type PrometheusConfig struct {
	Global        GlobalConfig   `yaml:"global"`
	ScrapeConfigs []ScrapeConfig `yaml:"scrape_configs"`
}

// GlobalConfig represents the global Prometheus configuration
type GlobalConfig struct {
	ScrapeInterval string `yaml:"scrape_interval"`
}

// ScrapeConfig represents a Prometheus scrape configuration
type ScrapeConfig struct {
	JobName       string         `yaml:"job_name"`
	StaticConfigs []StaticConfig `yaml:"static_configs"`
}

// StaticConfig represents a static target configuration
type StaticConfig struct {
	Targets []string `yaml:"targets"`
}

// PeerNode represents a peer node in the system
type PeerNode struct {
	ID               string
	Name             string
	OperationAddress string
}

// getPeerNodes retrieves peer nodes from the database
func (d *DockerPrometheusDeployer) getPeerNodes(ctx context.Context) ([]PeerNode, error) {
	// Get peer nodes from database
	nodes, err := d.nodeService.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer nodes: %w", err)
	}

	// Determine the appropriate host based on deployment mode
	var host string
	if d.config.DockerConfig.NetworkMode == common.NetworkModeHost {
		host = "localhost"
	} else {
		host = "host.docker.internal"
	}

	peerNodes := make([]PeerNode, 0)
	for _, node := range nodes.Items {
		if node.FabricPeer == nil {
			continue
		}
		operationAddress := node.FabricPeer.OperationsAddress
		if operationAddress == "" {
			operationAddress = node.FabricPeer.ExternalEndpoint
		}

		// Extract port from operations address
		var port string
		if parts := strings.Split(operationAddress, ":"); len(parts) > 1 {
			port = parts[len(parts)-1]
		} else {
			port = "9443" // Default operations port if not specified
		}

		formattedAddress := fmt.Sprintf("%s:%s", host, port)

		peerNodes = append(peerNodes, PeerNode{
			ID:               strconv.FormatInt(node.ID, 10),
			Name:             node.Name,
			OperationAddress: formattedAddress,
		})
	}

	return peerNodes, nil
}

// getOrdererNodes retrieves orderer nodes from the database
func (d *DockerPrometheusDeployer) getOrdererNodes(ctx context.Context) ([]PeerNode, error) {
	// Get all nodes from database
	nodes, err := d.nodeService.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	// Determine the appropriate host based on deployment mode
	var host string
	if d.config.DockerConfig.NetworkMode == common.NetworkModeHost {
		host = "localhost"
	} else {
		host = "host.docker.internal"
	}

	ordererNodes := make([]PeerNode, 0)
	for _, node := range nodes.Items {
		if node.FabricOrderer == nil {
			continue
		}

		operationAddress := node.FabricOrderer.OperationsAddress
		if operationAddress == "" {
			operationAddress = node.FabricOrderer.ExternalEndpoint
		}

		// Extract port from operations address
		var port string
		if parts := strings.Split(operationAddress, ":"); len(parts) > 1 {
			port = parts[len(parts)-1]
		} else {
			port = "9443" // Default operations port if not specified
		}

		formattedAddress := fmt.Sprintf("%s:%s", host, port)

		ordererNodes = append(ordererNodes, PeerNode{
			ID:               strconv.FormatInt(node.ID, 10),
			Name:             node.Name,
			OperationAddress: formattedAddress,
		})
	}

	return ordererNodes, nil
}

// getBesuNodes retrieves Besu nodes from the database that have metrics enabled
func (d *DockerPrometheusDeployer) getBesuNodes(ctx context.Context) ([]PeerNode, error) {
	// Get all nodes from database
	nodes, err := d.nodeService.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	// Determine the appropriate host based on deployment mode
	var defaultHost string
	if d.config.DockerConfig.NetworkMode == common.NetworkModeHost {
		defaultHost = "localhost"
	} else {
		defaultHost = "host.docker.internal"
	}

	besuNodes := make([]PeerNode, 0)
	for _, node := range nodes.Items {
		// Skip nodes that are not Besu nodes or don't have metrics enabled
		if node.BesuNode == nil || !node.BesuNode.MetricsEnabled {
			continue
		}

		// Get metrics host and port
		metricsHost := node.BesuNode.MetricsHost
		if metricsHost == "" || metricsHost == "0.0.0.0" {
			// Use appropriate host based on deployment mode
			metricsHost = defaultHost
		}

		metricsPort := fmt.Sprintf("%d", node.BesuNode.MetricsPort)
		if metricsPort == "0" {
			metricsPort = "9545" // Default metrics port if not specified
		}

		formattedAddress := fmt.Sprintf("%s:%s", metricsHost, metricsPort)

		besuNodes = append(besuNodes, PeerNode{
			ID:               strconv.FormatInt(node.ID, 10),
			Name:             node.Name,
			OperationAddress: formattedAddress,
		})
	}

	return besuNodes, nil
}

// buildPrometheusConfig builds the Prometheus YAML config from current state (peers, orderers, besu, etc.)
func (d *DockerPrometheusDeployer) buildPrometheusConfig(ctx context.Context) (string, error) {
	// Get peer, orderer, and besu nodes
	peerNodes, err := d.getPeerNodes(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get peer nodes: %w", err)
	}
	ordererNodes, err := d.getOrdererNodes(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get orderer nodes: %w", err)
	}
	besuNodes, err := d.getBesuNodes(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get Besu nodes: %w", err)
	}

	config := &PrometheusConfig{
		Global: GlobalConfig{
			ScrapeInterval: d.config.ScrapeInterval.String(),
		},
		ScrapeConfigs: []ScrapeConfig{
			{
				JobName:       "prometheus",
				StaticConfigs: []StaticConfig{{Targets: []string{fmt.Sprintf("localhost:%d", d.config.PrometheusPort)}}},
			},
		},
	}
	// Add peer node targets
	for _, node := range peerNodes {
		jobName := slugify(fmt.Sprintf("%s-%s", node.ID, node.Name))
		config.ScrapeConfigs = append(config.ScrapeConfigs, ScrapeConfig{
			JobName:       jobName,
			StaticConfigs: []StaticConfig{{Targets: []string{node.OperationAddress}}},
		})
	}
	// Add orderer node targets
	for _, node := range ordererNodes {
		jobName := slugify(fmt.Sprintf("%s-%s", node.ID, node.Name))
		config.ScrapeConfigs = append(config.ScrapeConfigs, ScrapeConfig{
			JobName:       jobName,
			StaticConfigs: []StaticConfig{{Targets: []string{node.OperationAddress}}},
		})
	}
	// Add Besu node targets
	for _, node := range besuNodes {
		jobName := slugify(fmt.Sprintf("%s-%s", node.ID, node.Name))
		config.ScrapeConfigs = append(config.ScrapeConfigs, ScrapeConfig{
			JobName:       jobName,
			StaticConfigs: []StaticConfig{{Targets: []string{node.OperationAddress}}},
		})
	}
	configData, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}
	return string(configData), nil
}

// GetStatus returns the current status of the Prometheus container
func (d *DockerPrometheusDeployer) GetStatus(ctx context.Context) (string, error) {
	containerName := fmt.Sprintf("chainlaunch-prometheus-%d", d.config.PrometheusPort)

	// Get container info
	container, err := d.client.ContainerInspect(ctx, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}

	return container.State.Status, nil
}

// TailLogs retrieves Prometheus logs from the Docker container with optional tail and follow functionality
func (d *DockerPrometheusDeployer) TailLogs(ctx context.Context, tail int, follow bool) (<-chan string, error) {
	containerName := fmt.Sprintf("chainlaunch-prometheus-%d", d.config.PrometheusPort)

	// Check if container exists
	_, err := d.client.ContainerInspect(ctx, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Create log options
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Details:    true,
		Tail:       fmt.Sprintf("%d", tail),
	}

	// Get container logs
	logs, err := d.client.ContainerLogs(ctx, containerName, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get container logs: %w", err)
	}

	// Create channel for streaming logs
	logChan := make(chan string, 100)

	// Start goroutine to read logs and send to channel
	go func() {
		defer close(logChan)
		defer logs.Close()

		buffer := make([]byte, 8192)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Read from logs
				n, err := logs.Read(buffer)
				if err != nil {
					if err != io.EOF {
						// Send error message to channel
						logChan <- fmt.Sprintf("Error reading logs: %v", err)
					}
					return
				}

				if n > 0 {
					// Docker logs include an 8-byte header, skip it
					if n > 8 {
						logData := string(buffer[8:n])

						// Split by newlines and send each line
						lines := strings.Split(strings.TrimSpace(logData), "\n")
						for _, line := range lines {
							if line != "" {
								select {
								case logChan <- line:
								case <-ctx.Done():
									return
								}
							}
						}
					}
				}
			}
		}
	}()

	return logChan, nil
}

// updateDatabaseConfig updates the prometheus_config table with the current configuration
func (d *DockerPrometheusDeployer) updateDatabaseConfig(ctx context.Context) error {
	// Get current config from the file
	configData, err := d.buildPrometheusConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to build Prometheus config for database: %w", err)
	}

	// Parse the config to get scrape intervals and targets
	var config struct {
		Global struct {
			ScrapeInterval string `yaml:"scrape_interval"`
		} `yaml:"global"`
		ScrapeConfigs []struct {
			JobName       string `yaml:"job_name"`
			StaticConfigs []struct {
				Targets []string `yaml:"targets"`
			} `yaml:"static_configs"`
		} `yaml:"scrape_configs"`
	}

	if err := yaml.Unmarshal([]byte(configData), &config); err != nil {
		return fmt.Errorf("failed to parse Prometheus config for database: %w", err)
	}

	// Get the current scrape interval from the config
	scrapeInterval, err := time.ParseDuration(config.Global.ScrapeInterval)
	if err != nil {
		return fmt.Errorf("failed to parse scrape interval from config: %w", err)
	}

	// Get the current Prometheus port from the config
	prometheusPort := d.config.PrometheusPort

	// Get the current deployment mode from the config
	deploymentMode := string(d.config.DeploymentMode)

	// Get the current Docker image from the config
	dockerImage := fmt.Sprintf("prom/prometheus:%s", d.config.PrometheusVersion)

	// Get the current container name from the config
	containerName := fmt.Sprintf("chainlaunch-prometheus-%d", d.config.PrometheusPort)

	// Get the current data directory from the config
	dataDir := d.dataDir

	// Get the current config directory from the config
	configDir := d.configDir

	// Get the current binary path from the config
	binaryPath := filepath.Join(d.binDir, "prometheus")

	// Check if a record exists
	_, err = d.db.GetPrometheusConfig(ctx)
	if err != nil {
		// Record doesn't exist, create a new one
		createParams := &db.CreatePrometheusConfigParams{
			PrometheusPort:     int64(prometheusPort),
			DataDir:            dataDir,
			ConfigDir:          configDir,
			ContainerName:      containerName,
			ScrapeInterval:     int64(scrapeInterval.Seconds()),
			EvaluationInterval: int64(scrapeInterval.Seconds()), // Use same as scrape interval
			DeploymentMode:     deploymentMode,
			DockerImage:        dockerImage,
			NetworkMode:        sql.NullString{String: string(d.config.DockerConfig.NetworkMode), Valid: true},
			ExtraHosts:         sql.NullString{String: "host.docker.internal:host-gateway", Valid: true},
			RestartPolicy:      sql.NullString{String: "unless-stopped", Valid: true},
			ServiceName:        sql.NullString{String: containerName, Valid: true},
			ServiceUser:        sql.NullString{String: "", Valid: false},
			ServiceGroup:       sql.NullString{String: "", Valid: false},
			BinaryPath:         sql.NullString{String: binaryPath, Valid: true},
			PrometheusVersion:  sql.NullString{String: d.config.PrometheusVersion, Valid: true},
		}

		_, err = d.db.CreatePrometheusConfig(ctx, createParams)
		if err != nil {
			return fmt.Errorf("failed to create Prometheus config in database: %w", err)
		}
	} else {
		// Record exists, update it
		updateParams := &db.UpdatePrometheusConfigParams{
			PrometheusPort: int64(prometheusPort),
			DataDir:        dataDir,
			ConfigDir:      configDir,
			ContainerName:  containerName,
			ScrapeInterval: int64(scrapeInterval.Seconds()),
			DeploymentMode: deploymentMode,
			DockerImage:    dockerImage,
			NetworkMode:    sql.NullString{String: string(d.config.DockerConfig.NetworkMode), Valid: true},
			ExtraHosts:     sql.NullString{String: "host.docker.internal:host-gateway", Valid: true},
			RestartPolicy:  sql.NullString{String: "unless-stopped", Valid: true},
			ServiceName:    sql.NullString{String: containerName, Valid: true},
			ServiceUser:    sql.NullString{String: "", Valid: false},
			ServiceGroup:   sql.NullString{String: "", Valid: false},
			BinaryPath:     sql.NullString{String: binaryPath, Valid: true},
		}

		_, err = d.db.UpdatePrometheusConfig(ctx, updateParams)
		if err != nil {
			return fmt.Errorf("failed to update Prometheus config in database: %w", err)
		}
	}

	return nil
}

// Start starts the Prometheus service
func (s *ServicePrometheusDeployer) Start(ctx context.Context) error {
	// Check if we need to stop a previous service with a different port
	if err := s.stopPreviousServiceIfNeeded(ctx); err != nil {
		return fmt.Errorf("failed to stop previous service: %w", err)
	}

	// Create directories if they don't exist
	dirs := []string{s.configDir, s.dataDir, s.binDir, filepath.Join(s.dataDir, "prometheus")}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Download Prometheus binary if not exists
	prometheusBin := filepath.Join(s.binDir, "prometheus")
	if _, err := os.Stat(prometheusBin); os.IsNotExist(err) {
		if err := s.downloadPrometheus(); err != nil {
			return fmt.Errorf("failed to download Prometheus: %w", err)
		}
	}

	// Check if config file already exists (could be from migration)
	configPath := filepath.Join(s.configDir, "prometheus.yml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Config file doesn't exist, generate it
		configData, err := s.buildPrometheusConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to generate Prometheus config: %w", err)
		}

		// Write config file
		if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}
	} else {
		// Config file exists, update it with current configuration
		configData, err := s.buildPrometheusConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to generate Prometheus config: %w", err)
		}

		// Write updated config file
		if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}
	}

	// Create service file
	if err := s.createServiceFile(); err != nil {
		return fmt.Errorf("failed to create service file: %w", err)
	}

	// Update database with current configuration
	if err := s.updateDatabaseConfig(ctx); err != nil {
		return fmt.Errorf("failed to update database configuration: %w", err)
	}

	// Start the service
	if err := s.startService(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Wait for Prometheus to be active and ready
	if err := s.waitForPrometheusActive(ctx); err != nil {
		return fmt.Errorf("failed to wait for Prometheus to be active: %w", err)
	}

	// Reload configuration
	return s.Reload(ctx)
}

// Stop stops the Prometheus service
func (s *ServicePrometheusDeployer) Stop(ctx context.Context) error {
	return s.stopService()
}

// Reload reloads the Prometheus configuration
func (s *ServicePrometheusDeployer) Reload(ctx context.Context) error {
	// Generate new config
	configData, err := s.buildPrometheusConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to generate Prometheus config: %w", err)
	}

	// Write new config
	configPath := filepath.Join(s.configDir, "prometheus.yml")
	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Send POST request to reload endpoint
	reloadURL := fmt.Sprintf("http://localhost:%d/-/reload", s.config.PrometheusPort)

	// Create POST request
	req, err := http.NewRequestWithContext(ctx, "POST", reloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create reload request: %w", err)
	}

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send reload request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != 200 {
		return fmt.Errorf("reload request failed with status: %d", resp.StatusCode)
	}

	return nil
}

// GetStatus returns the current status of the Prometheus service
func (s *ServicePrometheusDeployer) GetStatus(ctx context.Context) (string, error) {
	return s.getServiceStatus()
}

// TailLogs retrieves Prometheus logs from the service log file with optional tail and follow functionality
func (s *ServicePrometheusDeployer) TailLogs(ctx context.Context, tail int, follow bool) (<-chan string, error) {
	logPath := s.GetStdOutPath()

	// Check if log file exists
	if _, err := os.Stat(logPath); err != nil {
		return nil, fmt.Errorf("log file not found: %w", err)
	}

	// Create channel for streaming logs
	logChan := make(chan string, 100)

	// Start goroutine to read logs
	go func() {
		defer close(logChan)

		if follow {
			// Use tail command with follow for real-time streaming
			var cmd *exec.Cmd
			switch runtime.GOOS {
			case "windows":
				// Use PowerShell for Windows
				cmd = exec.CommandContext(ctx, "powershell", "-Command",
					fmt.Sprintf("Get-Content -Path '%s' -Tail %d -Wait", logPath, tail))
			default:
				// Use tail for Unix-like systems
				cmd = exec.CommandContext(ctx, "tail", "-n", fmt.Sprintf("%d", tail), "-f", logPath)
			}

			// Set up stdout pipe
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				logChan <- fmt.Sprintf("Error creating stdout pipe: %v", err)
				return
			}

			// Start the command
			if err := cmd.Start(); err != nil {
				logChan <- fmt.Sprintf("Error starting tail command: %v", err)
				return
			}

			// Read lines from stdout
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				select {
				case logChan <- scanner.Text():
				case <-ctx.Done():
					cmd.Process.Kill()
					return
				}
			}

			if err := scanner.Err(); err != nil {
				logChan <- fmt.Sprintf("Error reading logs: %v", err)
			}

			// Wait for command to finish
			cmd.Wait()
		} else {
			// Read last N lines without following
			var cmd *exec.Cmd
			switch runtime.GOOS {
			case "windows":
				// Use PowerShell for Windows
				cmd = exec.CommandContext(ctx, "powershell", "-Command",
					fmt.Sprintf("Get-Content -Path '%s' -Tail %d", logPath, tail))
			default:
				// Use tail for Unix-like systems
				cmd = exec.CommandContext(ctx, "tail", "-n", fmt.Sprintf("%d", tail), logPath)
			}

			// Get output
			output, err := cmd.Output()
			if err != nil {
				logChan <- fmt.Sprintf("Error reading logs: %v", err)
				return
			}

			// Send each line to the channel
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if line != "" {
					select {
					case logChan <- line:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return logChan, nil
}

// waitForPrometheusActive waits for Prometheus to be active and ready to accept requests
func (s *ServicePrometheusDeployer) waitForPrometheusActive(ctx context.Context) error {
	// Wait for service to be running
	maxWaitTime := 60 * time.Second
	checkInterval := 2 * time.Second
	elapsed := time.Duration(0)

	for elapsed < maxWaitTime {
		// Check service status
		status, err := s.getServiceStatus()
		if err != nil || status != "running" {
			time.Sleep(checkInterval)
			elapsed += checkInterval
			continue
		}

		// Service is running, now check if Prometheus is ready
		// Try to connect to the health endpoint
		healthURL := fmt.Sprintf("http://localhost:%d/-/healthy", s.config.PrometheusPort)

		resp, err := http.Get(healthURL)
		if err != nil || resp.StatusCode != 200 {
			time.Sleep(checkInterval)
			elapsed += checkInterval
			continue
		}
		resp.Body.Close()

		// Prometheus is ready
		return nil
	}

	return fmt.Errorf("timeout waiting for Prometheus to be active after %v", maxWaitTime)
}

// downloadPrometheus downloads the Prometheus binary from official releases
func (s *ServicePrometheusDeployer) downloadPrometheus() error {
	version := s.config.PrometheusVersion
	if version == "" {
		version = "v3.5.0" // Default version
	}

	// Check if binary already exists
	prometheusBin := filepath.Join(s.binDir, "prometheus")
	if _, err := os.Stat(prometheusBin); err == nil {
		// Binary already exists, check if it's the right version
		cmd := exec.Command(prometheusBin, "--version")
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), version) {
			return nil // Correct version already installed
		}
	}

	// Create bin directory if it doesn't exist
	if err := os.MkdirAll(s.binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Determine OS and architecture
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Map Go architecture to Prometheus architecture names
	archMap := map[string]string{
		"amd64": "amd64",
		"arm64": "arm64",
		"386":   "386",
		"arm":   "armv7",
	}
	prometheusArch, ok := archMap[arch]
	if !ok {
		return fmt.Errorf("unsupported architecture: %s", arch)
	}

	// Map Go OS to Prometheus OS names
	osMap := map[string]string{
		"linux":   "linux",
		"darwin":  "darwin",
		"windows": "windows",
	}
	prometheusOS, ok := osMap[osName]
	if !ok {
		return fmt.Errorf("unsupported operating system: %s", osName)
	}

	// Construct download URL
	// Format: https://github.com/prometheus/prometheus/releases/download/v2.45.0/prometheus-2.45.0.linux-amd64.tar.gz
	downloadURL := fmt.Sprintf("https://github.com/prometheus/prometheus/releases/download/%s/prometheus-%s.%s-%s.tar.gz",
		version, strings.TrimPrefix(version, "v"), prometheusOS, prometheusArch)

	// Create temporary directory for download
	tmpDir, err := os.MkdirTemp("", "prometheus-download-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download the archive
	archivePath := filepath.Join(tmpDir, "prometheus.tar.gz")
	if err := s.downloadFile(downloadURL, archivePath); err != nil {
		return fmt.Errorf("failed to download Prometheus: %w", err)
	}

	// Extract the archive
	if err := s.extractTarGz(archivePath, tmpDir); err != nil {
		return fmt.Errorf("failed to extract Prometheus archive: %w", err)
	}

	// Find the extracted directory (should be something like prometheus-2.45.0.linux-amd64)
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return fmt.Errorf("failed to read temp directory: %w", err)
	}

	var extractedDir string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "prometheus-") {
			extractedDir = filepath.Join(tmpDir, entry.Name())
			break
		}
	}

	if extractedDir == "" {
		return fmt.Errorf("could not find extracted Prometheus directory")
	}

	// Copy the prometheus binary to our bin directory
	srcBinary := filepath.Join(extractedDir, "prometheus")
	if runtime.GOOS == "windows" {
		srcBinary += ".exe"
	}

	if err := s.copyFile(srcBinary, prometheusBin); err != nil {
		return fmt.Errorf("failed to copy Prometheus binary: %w", err)
	}

	// Make the binary executable
	if err := os.Chmod(prometheusBin, 0755); err != nil {
		return fmt.Errorf("failed to make Prometheus binary executable: %w", err)
	}

	// Copy console libraries and templates if they exist
	consoleLibrariesSrc := filepath.Join(extractedDir, "console_libraries")
	consoleLibrariesDst := filepath.Join(s.binDir, "console_libraries")
	if _, err := os.Stat(consoleLibrariesSrc); err == nil {
		if err := s.copyDir(consoleLibrariesSrc, consoleLibrariesDst); err != nil {
			return fmt.Errorf("failed to copy console libraries: %w", err)
		}
	}

	consolesSrc := filepath.Join(extractedDir, "consoles")
	consolesDst := filepath.Join(s.binDir, "consoles")
	if _, err := os.Stat(consolesSrc); err == nil {
		if err := s.copyDir(consolesSrc, consolesDst); err != nil {
			return fmt.Errorf("failed to copy consoles: %w", err)
		}
	}

	return nil
}

// downloadFile downloads a file from a URL to a local path
func (s *ServicePrometheusDeployer) downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: HTTP %d", resp.StatusCode)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// extractTarGz extracts a tar.gz file to a destination directory
func (s *ServicePrometheusDeployer) extractTarGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		// Skip if not a file
		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Check for directory traversal
		if strings.Contains(header.Name, "..") {
			return fmt.Errorf("invalid file path in tar: %s", header.Name)
		}

		// Get the target path
		targetPath := filepath.Join(destDir, header.Name)
		cleanTargetPath := filepath.Clean(targetPath)

		// Ensure the target path is within the destination directory
		if !strings.HasPrefix(cleanTargetPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path in tar: %s", header.Name)
		}

		// Create directory structure
		if err := os.MkdirAll(filepath.Dir(cleanTargetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory structure: %w", err)
		}

		// Create file
		f, err := os.OpenFile(cleanTargetPath, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		// Copy contents
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return fmt.Errorf("failed to write file: %w", err)
		}
		f.Close()
	}

	return nil
}

// copyFile copies a single file from src to dst
func (s *ServicePrometheusDeployer) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// copyDir recursively copies a directory structure
func (s *ServicePrometheusDeployer) copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := s.copyDir(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy directory %s: %w", srcPath, err)
			}
		} else {
			// Copy file
			input, err := os.ReadFile(srcPath)
			if err != nil {
				return fmt.Errorf("failed to read source file %s: %w", srcPath, err)
			}

			// Preserve original file mode
			srcInfo, err := os.Stat(srcPath)
			if err != nil {
				return fmt.Errorf("failed to get source file info %s: %w", srcPath, err)
			}

			if err := os.WriteFile(dstPath, input, srcInfo.Mode()); err != nil {
				return fmt.Errorf("failed to write destination file %s: %w", dstPath, err)
			}
		}
	}

	return nil
}

// createServiceFile creates the system service file
func (s *ServicePrometheusDeployer) createServiceFile() error {
	var serviceContent string

	switch s.serviceType {
	case common.ServiceTypeSystemd:
		serviceContent = s.generateSystemdService()
	case common.ServiceTypeLaunchd:
		serviceContent = s.generateLaunchdService()
	default:
		return fmt.Errorf("unsupported service type: %s", s.serviceType)
	}

	// Write service file
	servicePath := s.getServiceFilePath()
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload service manager
	return s.reloadServiceManager()
}

// generateSystemdService generates the systemd service file content
func (s *ServicePrometheusDeployer) generateSystemdService() string {
	// Get current user
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = os.Getenv("USERNAME") // Fallback for Windows
	}
	if currentUser == "" {
		currentUser = "root" // Final fallback
	}

	return fmt.Sprintf(`[Unit]
Description=Prometheus (Port %d)
Wants=network-online.target
After=network-online.target

[Service]
User=%s
Type=simple
ExecStart=%s/prometheus \
  --config.file=%s/prometheus.yml \
  --storage.tsdb.path=%s \
  --web.console.libraries=%s/console_libraries \
  --web.console.templates=%s/consoles \
  --web.enable-lifecycle \
  --web.enable-admin-api \
  --web.listen-address=:%d

StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=multi-user.target
`, s.config.PrometheusPort, currentUser, s.binDir, s.configDir, s.dataDir, s.binDir, s.binDir, s.config.PrometheusPort, s.GetStdOutPath(), s.GetStdOutPath())
}

// generateLaunchdService generates the launchd service file content
func (s *ServicePrometheusDeployer) generateLaunchdService() string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s/prometheus</string>
        <string>--config.file=%s/prometheus.yml</string>
        <string>--storage.tsdb.path=%s</string>
        <string>--web.console.libraries=%s/console_libraries</string>
        <string>--web.console.templates=%s/consoles</string>
        <string>--web.enable-lifecycle</string>
        <string>--web.enable-admin-api</string>
        <string>--web.listen-address=:%d</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
</dict>
</plist>
`, s.getLaunchdServiceName(), s.binDir, s.configDir, s.dataDir, s.binDir, s.binDir, s.config.PrometheusPort, s.GetStdOutPath(), s.GetStdOutPath())
}

// reloadServiceManager reloads the service manager configuration
func (s *ServicePrometheusDeployer) reloadServiceManager() error {
	switch s.serviceType {
	case common.ServiceTypeSystemd:
		cmd := exec.Command("systemctl", "daemon-reload")
		return cmd.Run()
	case common.ServiceTypeLaunchd:
		// launchd doesn't need reload for new plist files
		return nil
	default:
		return fmt.Errorf("unsupported service type: %s", s.serviceType)
	}
}

// startService starts the Prometheus service
func (s *ServicePrometheusDeployer) startService() error {
	switch s.serviceType {
	case common.ServiceTypeSystemd:
		cmd := exec.Command("systemctl", "start", s.getServiceName())
		return cmd.Run()
	case common.ServiceTypeLaunchd:
		cmd := exec.Command("launchctl", "load", s.getServiceFilePath())
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported service type: %s", s.serviceType)
	}
}

// stopService stops the Prometheus service
func (s *ServicePrometheusDeployer) stopService() error {
	switch s.serviceType {
	case common.ServiceTypeSystemd:
		cmd := exec.Command("systemctl", "stop", s.getServiceName())
		return cmd.Run()
	case common.ServiceTypeLaunchd:
		cmd := exec.Command("launchctl", "unload", s.getServiceFilePath())
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported service type: %s", s.serviceType)
	}
}

// reloadService reloads the Prometheus service
func (s *ServicePrometheusDeployer) reloadService() error {
	switch s.serviceType {
	case common.ServiceTypeSystemd:
		cmd := exec.Command("systemctl", "reload", s.getServiceName())
		return cmd.Run()
	case common.ServiceTypeLaunchd:
		// For launchd, we need to restart the service
		if err := s.stopService(); err != nil {
			return err
		}
		return s.startService()
	default:
		return fmt.Errorf("unsupported service type: %s", s.serviceType)
	}
}

// getServiceStatus gets the status of the Prometheus service
func (s *ServicePrometheusDeployer) getServiceStatus() (string, error) {
	switch s.serviceType {
	case common.ServiceTypeSystemd:
		cmd := exec.Command("systemctl", "is-active", s.getServiceName())
		output, err := cmd.Output()
		if err != nil {
			return "inactive", nil
		}
		status := strings.TrimSpace(string(output))
		if status == "active" {
			return "running", nil
		}
		return status, nil
	case common.ServiceTypeLaunchd:
		cmd := exec.Command("launchctl", "list", s.getLaunchdServiceName())
		err := cmd.Run()
		if err != nil {
			return "inactive", nil
		}
		return "running", nil
	default:
		return "unknown", fmt.Errorf("unsupported service type: %s", s.serviceType)
	}
}

// buildPrometheusConfig builds the Prometheus YAML config for service deployment
func (s *ServicePrometheusDeployer) buildPrometheusConfig(ctx context.Context) (string, error) {
	// Get peer, orderer, and besu nodes
	peerNodes, err := s.getPeerNodes(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get peer nodes: %w", err)
	}
	ordererNodes, err := s.getOrdererNodes(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get orderer nodes: %w", err)
	}
	besuNodes, err := s.getBesuNodes(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get Besu nodes: %w", err)
	}

	config := &PrometheusConfig{
		Global: GlobalConfig{
			ScrapeInterval: s.config.ScrapeInterval.String(),
		},
		ScrapeConfigs: []ScrapeConfig{
			{
				JobName:       "prometheus",
				StaticConfigs: []StaticConfig{{Targets: []string{fmt.Sprintf("localhost:%d", s.config.PrometheusPort)}}},
			},
		},
	}

	// Add peer node targets
	for _, node := range peerNodes {
		jobName := slugify(fmt.Sprintf("%s-%s", node.ID, node.Name))
		config.ScrapeConfigs = append(config.ScrapeConfigs, ScrapeConfig{
			JobName:       jobName,
			StaticConfigs: []StaticConfig{{Targets: []string{node.OperationAddress}}},
		})
	}

	// Add orderer node targets
	for _, node := range ordererNodes {
		jobName := slugify(fmt.Sprintf("%s-%s", node.ID, node.Name))
		config.ScrapeConfigs = append(config.ScrapeConfigs, ScrapeConfig{
			JobName:       jobName,
			StaticConfigs: []StaticConfig{{Targets: []string{node.OperationAddress}}},
		})
	}

	// Add Besu node targets
	for _, node := range besuNodes {
		jobName := slugify(fmt.Sprintf("%s-%s", node.ID, node.Name))
		config.ScrapeConfigs = append(config.ScrapeConfigs, ScrapeConfig{
			JobName:       jobName,
			StaticConfigs: []StaticConfig{{Targets: []string{node.OperationAddress}}},
		})
	}

	configData, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}
	return string(configData), nil
}

// getPeerNodes retrieves peer nodes from the database (service version)
func (s *ServicePrometheusDeployer) getPeerNodes(ctx context.Context) ([]PeerNode, error) {
	// Get peer nodes from database
	nodes, err := s.nodeService.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer nodes: %w", err)
	}

	peerNodes := make([]PeerNode, 0)
	for _, node := range nodes.Items {
		if node.FabricPeer == nil {
			continue
		}
		operationAddress := node.FabricPeer.OperationsAddress
		if operationAddress == "" {
			operationAddress = node.FabricPeer.ExternalEndpoint
		}

		// Extract port from operations address
		var port string
		if parts := strings.Split(operationAddress, ":"); len(parts) > 1 {
			port = parts[len(parts)-1]
		} else {
			port = "9443" // Default operations port if not specified
		}

		// For service deployment, always use localhost
		formattedAddress := fmt.Sprintf("localhost:%s", port)

		peerNodes = append(peerNodes, PeerNode{
			ID:               strconv.FormatInt(node.ID, 10),
			Name:             node.Name,
			OperationAddress: formattedAddress,
		})
	}

	return peerNodes, nil
}

// getOrdererNodes retrieves orderer nodes from the database (service version)
func (s *ServicePrometheusDeployer) getOrdererNodes(ctx context.Context) ([]PeerNode, error) {
	// Get all nodes from database
	nodes, err := s.nodeService.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	ordererNodes := make([]PeerNode, 0)
	for _, node := range nodes.Items {
		if node.FabricOrderer == nil {
			continue
		}

		operationAddress := node.FabricOrderer.OperationsAddress
		if operationAddress == "" {
			operationAddress = node.FabricOrderer.ExternalEndpoint
		}

		// Extract port from operations address
		var port string
		if parts := strings.Split(operationAddress, ":"); len(parts) > 1 {
			port = parts[len(parts)-1]
		} else {
			port = "9443" // Default operations port if not specified
		}

		// For service deployment, always use localhost
		formattedAddress := fmt.Sprintf("localhost:%s", port)

		ordererNodes = append(ordererNodes, PeerNode{
			ID:               strconv.FormatInt(node.ID, 10),
			Name:             node.Name,
			OperationAddress: formattedAddress,
		})
	}

	return ordererNodes, nil
}

// getBesuNodes retrieves Besu nodes from the database that have metrics enabled (service version)
func (s *ServicePrometheusDeployer) getBesuNodes(ctx context.Context) ([]PeerNode, error) {
	// Get all nodes from database
	nodes, err := s.nodeService.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	besuNodes := make([]PeerNode, 0)
	for _, node := range nodes.Items {
		// Skip nodes that are not Besu nodes or don't have metrics enabled
		if node.BesuNode == nil || !node.BesuNode.MetricsEnabled {
			continue
		}

		// Get metrics host and port
		metricsHost := node.BesuNode.MetricsHost
		if metricsHost == "" || metricsHost == "0.0.0.0" {
			// For service deployment, always use localhost
			metricsHost = "localhost"
		}

		metricsPort := fmt.Sprintf("%d", node.BesuNode.MetricsPort)
		if metricsPort == "0" {
			metricsPort = "9545" // Default metrics port if not specified
		}

		formattedAddress := fmt.Sprintf("%s:%s", metricsHost, metricsPort)

		besuNodes = append(besuNodes, PeerNode{
			ID:               strconv.FormatInt(node.ID, 10),
			Name:             node.Name,
			OperationAddress: formattedAddress,
		})
	}

	return besuNodes, nil
}

// updateDatabaseConfig updates the prometheus_config table with the current configuration
func (s *ServicePrometheusDeployer) updateDatabaseConfig(ctx context.Context) error {
	// Get current config from the file
	configData, err := s.buildPrometheusConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to build Prometheus config for database: %w", err)
	}

	// Parse the config to get scrape intervals and targets
	var config struct {
		Global struct {
			ScrapeInterval string `yaml:"scrape_interval"`
		} `yaml:"global"`
		ScrapeConfigs []struct {
			JobName       string `yaml:"job_name"`
			StaticConfigs []struct {
				Targets []string `yaml:"targets"`
			} `yaml:"static_configs"`
		} `yaml:"scrape_configs"`
	}

	if err := yaml.Unmarshal([]byte(configData), &config); err != nil {
		return fmt.Errorf("failed to parse Prometheus config for database: %w", err)
	}

	// Get the current scrape interval from the config
	scrapeInterval, err := time.ParseDuration(config.Global.ScrapeInterval)
	if err != nil {
		return fmt.Errorf("failed to parse scrape interval from config: %w", err)
	}

	// Get the current Prometheus port from the config
	prometheusPort := s.config.PrometheusPort

	// Get the current deployment mode from the config
	deploymentMode := string(s.config.DeploymentMode)

	// Get the current Docker image from the config
	dockerImage := fmt.Sprintf("prom/prometheus:%s", s.config.PrometheusVersion)

	// Get the current data directory from the config
	dataDir := s.dataDir

	// Get the current config directory from the config
	configDir := s.configDir

	// Get the current container name from the config
	containerName := s.getServiceName()

	// Get the current service name from the config
	serviceName := s.getServiceName()

	// Get the current binary path from the config
	binaryPath := filepath.Join(s.binDir, "prometheus")

	// Check if a record exists
	_, err = s.db.GetPrometheusConfig(ctx)
	if err != nil {
		// Record doesn't exist, create a new one
		// Get current user
		currentUser := os.Getenv("USER")
		if currentUser == "" {
			currentUser = os.Getenv("USERNAME") // Fallback for Windows
		}
		if currentUser == "" {
			currentUser = "root" // Final fallback
		}

		createParams := &db.CreatePrometheusConfigParams{
			PrometheusPort:     int64(prometheusPort),
			DataDir:            dataDir,
			ConfigDir:          configDir,
			ContainerName:      containerName,
			ScrapeInterval:     int64(scrapeInterval.Seconds()),
			EvaluationInterval: int64(scrapeInterval.Seconds()), // Use same as scrape interval
			DeploymentMode:     deploymentMode,
			DockerImage:        dockerImage,
			NetworkMode:        sql.NullString{String: "bridge", Valid: true},
			ExtraHosts:         sql.NullString{String: "", Valid: false},
			RestartPolicy:      sql.NullString{String: "unless-stopped", Valid: true},
			ServiceName:        sql.NullString{String: serviceName, Valid: true},
			ServiceUser:        sql.NullString{String: currentUser, Valid: true},
			ServiceGroup:       sql.NullString{String: "", Valid: false},
			BinaryPath:         sql.NullString{String: binaryPath, Valid: true},
			PrometheusVersion:  sql.NullString{String: s.config.PrometheusVersion, Valid: true},
		}

		_, err = s.db.CreatePrometheusConfig(ctx, createParams)
		if err != nil {
			return fmt.Errorf("failed to create Prometheus config in database: %w", err)
		}
	} else {
		// Record exists, update it
		// Get current user
		currentUser := os.Getenv("USER")
		if currentUser == "" {
			currentUser = os.Getenv("USERNAME") // Fallback for Windows
		}
		if currentUser == "" {
			currentUser = "root" // Final fallback
		}

		updateParams := &db.UpdatePrometheusConfigParams{
			PrometheusPort: int64(prometheusPort),
			DataDir:        dataDir,
			ConfigDir:      configDir,
			ContainerName:  containerName,
			ScrapeInterval: int64(scrapeInterval.Seconds()),
			DeploymentMode: deploymentMode,
			DockerImage:    dockerImage,
			NetworkMode:    sql.NullString{String: "bridge", Valid: true},
			ExtraHosts:     sql.NullString{String: "", Valid: false},
			RestartPolicy:  sql.NullString{String: "unless-stopped", Valid: true},
			ServiceName:    sql.NullString{String: serviceName, Valid: true},
			ServiceUser:    sql.NullString{String: currentUser, Valid: true},
			ServiceGroup:   sql.NullString{String: "", Valid: false},
			BinaryPath:     sql.NullString{String: binaryPath, Valid: true},
		}

		_, err = s.db.UpdatePrometheusConfig(ctx, updateParams)
		if err != nil {
			return fmt.Errorf("failed to update Prometheus config in database: %w", err)
		}
	}

	return nil
}

// stopPreviousServiceIfNeeded stops any existing Prometheus service if the port has changed
func (s *ServicePrometheusDeployer) stopPreviousServiceIfNeeded(ctx context.Context) error {
	// Get current configuration from database to check if this is a port change
	currentConfig, err := s.db.GetPrometheusConfig(ctx)
	if err != nil {
		// No existing config, no previous service to stop
		return nil
	}

	// Check if this is a port change
	if int(currentConfig.PrometheusPort) != s.config.PrometheusPort {
		// This is a port change, stop the previous service
		previousPort := int(currentConfig.PrometheusPort)
		previousServiceName := fmt.Sprintf("chainlaunch-prometheus-%d", previousPort)
		previousLaunchdServiceName := fmt.Sprintf("dev.chainlaunch.prometheus.%d", previousPort)

		// Stop the previous service based on service type
		switch s.serviceType {
		case common.ServiceTypeSystemd:
			cmd := exec.Command("systemctl", "stop", previousServiceName)
			if err := cmd.Run(); err != nil {
				// Ignore errors if service doesn't exist
				fmt.Printf("Note: Could not stop previous systemd service %s: %v\n", previousServiceName, err)
			}
		case common.ServiceTypeLaunchd:
			cmd := exec.Command("launchctl", "unload", s.getLaunchdPlistPathForPort(previousPort))
			if err := cmd.Run(); err != nil {
				// Ignore errors if service doesn't exist
				fmt.Printf("Note: Could not stop previous launchd service %s: %v\n", previousLaunchdServiceName, err)
			}
		}

		fmt.Printf("Stopped previous Prometheus service on port %d\n", previousPort)
	}

	return nil
}

// getLaunchdPlistPathForPort returns the launchd plist file path for a specific port
func (s *ServicePrometheusDeployer) getLaunchdPlistPathForPort(port int) string {
	homeDir, _ := os.UserHomeDir()
	serviceName := fmt.Sprintf("dev.chainlaunch.prometheus.%d", port)
	return filepath.Join(homeDir, "Library/LaunchAgents", serviceName+".plist")
}

// PrometheusManager handles the lifecycle of a Prometheus instance
type PrometheusManager struct {
	db            *db.Queries
	nodeService   *nodeservice.NodeService
	configService *configservice.ConfigService
}

// NewPrometheusManager creates a new PrometheusManager
func NewPrometheusManager(config *common.Config, db *db.Queries, nodeService *nodeservice.NodeService, configService *configservice.ConfigService) (*PrometheusManager, error) {
	pm := &PrometheusManager{
		db:            db,
		nodeService:   nodeService,
		configService: configService,
	}

	return pm, nil
}

// createDeployer creates a deployer from the given config
func (pm *PrometheusManager) createDeployer(config *common.Config) (PrometheusDeployer, error) {
	switch config.DeploymentMode {
	case common.DeploymentModeDocker:
		return NewDockerPrometheusDeployer(config, pm.db, pm.nodeService, pm.configService)
	case common.DeploymentModeService:
		return NewServicePrometheusDeployer(config, pm.db, pm.nodeService, pm.configService)
	default:
		return nil, fmt.Errorf("unsupported deployment mode: %s", config.DeploymentMode)
	}
}

// createClient creates a client from the given config
func (pm *PrometheusManager) createClient(config *common.Config) *Client {
	return NewClient(fmt.Sprintf("http://localhost:%d", config.PrometheusPort))
}

// Start starts the Prometheus instance with the given configuration
func (pm *PrometheusManager) Start(ctx context.Context, config *common.Config) error {
	// Check if Prometheus is already deployed
	currentConfig, loadErr := pm.loadConfigFromDatabase(ctx)
	isNewDeployment := loadErr != nil // If error, it's a new deployment
	isPortChange := false
	isDeploymentModeChange := false

	if !isNewDeployment {
		// Check if this is a port change
		isPortChange = currentConfig.PrometheusPort != config.PrometheusPort
		// Check if this is a deployment mode change
		isDeploymentModeChange = currentConfig.DeploymentMode != config.DeploymentMode
	}

	// Only check port availability for new deployments or port changes
	if isNewDeployment || isPortChange {
		if !CheckPrometheusPortAvailability(config.PrometheusPort) {
			return fmt.Errorf("port %d is not available", config.PrometheusPort)
		}
	}

	// If this is a deployment mode change, we need to stop the current instance and migrate data
	if isDeploymentModeChange {
		if err := pm.handleDeploymentModeMigration(ctx, currentConfig, config); err != nil {
			// Release the port if migration fails
			if isNewDeployment || isPortChange {
				ReleasePrometheusPort(config.PrometheusPort)
			}
			return fmt.Errorf("failed to migrate deployment mode: %w", err)
		}
	}

	// Create deployer with the new config
	deployer, deployerErr := pm.createDeployer(config)
	if deployerErr != nil {
		// Release the port if deployment fails (only for new deployments or port changes)
		if isNewDeployment || isPortChange {
			ReleasePrometheusPort(config.PrometheusPort)
		}
		return fmt.Errorf("failed to create deployer: %w", deployerErr)
	}

	// Start the deployer
	if startErr := deployer.Start(ctx); startErr != nil {
		// Release the port if start fails (only for new deployments or port changes)
		if isNewDeployment || isPortChange {
			ReleasePrometheusPort(config.PrometheusPort)
		}
		return fmt.Errorf("failed to start Prometheus: %w", startErr)
	}

	return nil
}

// handleDeploymentModeMigration handles migration between different deployment modes
func (pm *PrometheusManager) handleDeploymentModeMigration(ctx context.Context, oldConfig, newConfig *common.Config) error {
	// Stop the current instance first
	if err := pm.Stop(ctx); err != nil {
		// Log a warning instead of failing the migration
		fmt.Printf("Warning: Failed to stop current instance during deployment mode migration: %v\n", err)
		fmt.Printf("Continuing with migration from %s to %s mode...\n", oldConfig.DeploymentMode, newConfig.DeploymentMode)
	}

	// Handle migration based on the direction
	if oldConfig.DeploymentMode == common.DeploymentModeDocker && newConfig.DeploymentMode == common.DeploymentModeService {
		// Migrating from Docker to Service
		return pm.migrateFromDockerToService(ctx, oldConfig, newConfig)
	} else if oldConfig.DeploymentMode == common.DeploymentModeService && newConfig.DeploymentMode == common.DeploymentModeDocker {
		// Migrating from Service to Docker
		return pm.migrateFromServiceToDocker(ctx, oldConfig, newConfig)
	}

	return fmt.Errorf("unsupported deployment mode migration: %s -> %s", oldConfig.DeploymentMode, newConfig.DeploymentMode)
}

// migrateFromDockerToService migrates data from Docker volumes to service directories
func (pm *PrometheusManager) migrateFromDockerToService(ctx context.Context, oldConfig, newConfig *common.Config) error {
	// Get service deployer to determine target directories
	serviceDeployer, err := NewServicePrometheusDeployer(newConfig, pm.db, pm.nodeService, pm.configService)
	if err != nil {
		return fmt.Errorf("failed to create service deployer: %w", err)
	}

	// Create target directories
	dirs := []string{serviceDeployer.configDir, serviceDeployer.dataDir, serviceDeployer.binDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Since both deployers now use the same local directory structure,
	// we just need to ensure the directories exist and download the binary
	// Download Prometheus binary for service mode
	if err := serviceDeployer.downloadPrometheus(); err != nil {
		return fmt.Errorf("failed to download Prometheus binary: %w", err)
	}

	fmt.Printf("Successfully migrated from Docker to Service mode\n")
	return nil
}

// migrateFromServiceToDocker migrates data from service directories to Docker volumes
func (pm *PrometheusManager) migrateFromServiceToDocker(ctx context.Context, oldConfig, newConfig *common.Config) error {
	// Get service deployer to determine source directories
	serviceDeployer, err := NewServicePrometheusDeployer(oldConfig, pm.db, pm.nodeService, pm.configService)
	if err != nil {
		return fmt.Errorf("failed to create service deployer: %w", err)
	}

	// Get Docker deployer to determine target directories
	dockerDeployer, err := NewDockerPrometheusDeployer(newConfig, pm.db, pm.nodeService, pm.configService)
	if err != nil {
		return fmt.Errorf("failed to create docker deployer: %w", err)
	}

	// Create target directories for Docker deployer
	dirs := []string{dockerDeployer.configDir, dockerDeployer.dataDir, dockerDeployer.binDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Since both deployers now use the same local directory structure,
	// we just need to ensure the directories exist and copy any missing files
	// Copy config files if they don't exist in target
	if _, err := os.Stat(filepath.Join(dockerDeployer.configDir, "prometheus.yml")); os.IsNotExist(err) {
		if err := pm.copyDir(serviceDeployer.configDir, dockerDeployer.configDir); err != nil {
			return fmt.Errorf("failed to copy config files: %w", err)
		}
	}

	// Copy data files if they don't exist in target
	if _, err := os.Stat(dockerDeployer.dataDir); os.IsNotExist(err) {
		if err := pm.copyDir(serviceDeployer.dataDir, dockerDeployer.dataDir); err != nil {
			return fmt.Errorf("failed to copy data files: %w", err)
		}
	}

	fmt.Printf("Successfully migrated from Service to Docker mode\n")
	return nil
}

// copyDir recursively copies a directory structure
func (pm *PrometheusManager) copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := pm.copyDir(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy directory %s: %w", srcPath, err)
			}
		} else {
			// Copy file
			input, err := os.ReadFile(srcPath)
			if err != nil {
				return fmt.Errorf("failed to read source file %s: %w", srcPath, err)
			}

			// Preserve original file mode
			srcInfo, err := os.Stat(srcPath)
			if err != nil {
				return fmt.Errorf("failed to get source file info %s: %w", srcPath, err)
			}

			if err := os.WriteFile(dstPath, input, srcInfo.Mode()); err != nil {
				return fmt.Errorf("failed to write destination file %s: %w", dstPath, err)
			}
		}
	}

	return nil
}

// Stop stops the Prometheus instance
func (pm *PrometheusManager) Stop(ctx context.Context) error {
	// Load config from database
	config, err := pm.loadConfigFromDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config from database: %w", err)
	}

	// Create deployer
	deployer, err := pm.createDeployer(config)
	if err != nil {
		return fmt.Errorf("failed to create deployer: %w", err)
	}

	// Stop the deployer
	if err := deployer.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop Prometheus: %w", err)
	}

	// Release the port
	ReleasePrometheusPort(config.PrometheusPort)

	return nil
}

// GetCurrentConfig returns the current configuration from the database
func (pm *PrometheusManager) GetCurrentConfig(ctx context.Context) (*common.Config, error) {
	return pm.loadConfigFromDatabase(ctx)
}

// loadConfigFromDatabase loads the current configuration from the database
func (pm *PrometheusManager) loadConfigFromDatabase(ctx context.Context) (*common.Config, error) {
	// Get current configuration from database
	dbConfig, err := pm.db.GetPrometheusConfig(ctx)
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

// GetCurrentDeployer returns the current deployer based on the configuration in the database
func (pm *PrometheusManager) GetCurrentDeployer() PrometheusDeployer {
	ctx := context.Background()

	// Load config from database
	config, err := pm.loadConfigFromDatabase(ctx)
	if err != nil {
		return nil
	}

	// Create deployer
	deployer, err := pm.createDeployer(config)
	if err != nil {
		return nil
	}

	return deployer
}

// AddTarget adds a new target to the Prometheus configuration
func (pm *PrometheusManager) AddTarget(ctx context.Context, jobName string, targets []string) error {
	// Read existing config
	configPath := "/etc/prometheus/prometheus.yml"
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse existing config
	var config struct {
		Global struct {
			ScrapeInterval string `yaml:"scrape_interval"`
		} `yaml:"global"`
		ScrapeConfigs []struct {
			JobName       string `yaml:"job_name"`
			StaticConfigs []struct {
				Targets []string `yaml:"targets"`
			} `yaml:"static_configs"`
		} `yaml:"scrape_configs"`
	}

	if err := yaml.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Add new target
	config.ScrapeConfigs = append(config.ScrapeConfigs, struct {
		JobName       string `yaml:"job_name"`
		StaticConfigs []struct {
			Targets []string `yaml:"targets"`
		} `yaml:"static_configs"`
	}{
		JobName: jobName,
		StaticConfigs: []struct {
			Targets []string `yaml:"targets"`
		}{
			{
				Targets: targets,
			},
		},
	})

	// Write updated config
	newConfigData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, newConfigData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Load config from database
	dbConfig, err := pm.loadConfigFromDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config from database: %w", err)
	}

	// Create deployer
	deployer, err := pm.createDeployer(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to create deployer: %w", err)
	}

	// Reload Prometheus configuration
	return deployer.Reload(ctx)
}

// RemoveTarget removes a target from the Prometheus configuration
func (pm *PrometheusManager) RemoveTarget(ctx context.Context, jobName string) error {
	// Read existing config
	configPath := "/etc/prometheus/prometheus.yml"
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse existing config
	var config struct {
		Global struct {
			ScrapeInterval string `yaml:"scrape_interval"`
		} `yaml:"global"`
		ScrapeConfigs []struct {
			JobName       string `yaml:"job_name"`
			StaticConfigs []struct {
				Targets []string `yaml:"targets"`
			} `yaml:"static_configs"`
		} `yaml:"scrape_configs"`
	}

	if err := yaml.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Remove target
	newScrapeConfigs := make([]struct {
		JobName       string `yaml:"job_name"`
		StaticConfigs []struct {
			Targets []string `yaml:"targets"`
		} `yaml:"static_configs"`
	}, 0)

	for _, sc := range config.ScrapeConfigs {
		if sc.JobName != jobName {
			newScrapeConfigs = append(newScrapeConfigs, sc)
		}
	}

	config.ScrapeConfigs = newScrapeConfigs

	// Write updated config
	newConfigData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, newConfigData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Load config from database
	dbConfig, err := pm.loadConfigFromDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config from database: %w", err)
	}

	// Create deployer
	deployer, err := pm.createDeployer(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to create deployer: %w", err)
	}

	// Reload Prometheus configuration
	return deployer.Reload(ctx)
}

// Query executes a PromQL query against Prometheus
func (pm *PrometheusManager) Query(ctx context.Context, query string) (*common.QueryResult, error) {
	// Load config from database
	config, err := pm.loadConfigFromDatabase(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from database: %w", err)
	}

	// Create client
	client := pm.createClient(config)

	return client.Query(ctx, query)
}

// QueryRange executes a PromQL query with a time range
func (pm *PrometheusManager) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*common.QueryResult, error) {
	// Load config from database
	config, err := pm.loadConfigFromDatabase(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from database: %w", err)
	}

	// Create client
	client := pm.createClient(config)

	return client.QueryRange(ctx, query, start, end, step)
}

// GetLabelValues retrieves values for a specific label
func (pm *PrometheusManager) GetLabelValues(ctx context.Context, labelName string, matches []string) ([]string, error) {
	// Load config from database
	config, err := pm.loadConfigFromDatabase(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from database: %w", err)
	}

	// Create client
	client := pm.createClient(config)

	return client.GetLabelValues(ctx, labelName, matches)
}

// GetStatus returns the current status of the Prometheus instance
func (pm *PrometheusManager) GetStatus(ctx context.Context) (*common.Status, error) {
	status := &common.Status{
		Status: "not_deployed",
	}

	// Try to load config from database first
	config, err := pm.loadConfigFromDatabase(ctx)
	if err != nil {
		// If no config in database, return not_deployed status
		return status, nil
	}

	// Get status based on deployment mode
	switch config.DeploymentMode {
	case common.DeploymentModeDocker:
		return pm.getDockerStatus(ctx, status)
	case common.DeploymentModeService:
		return pm.getServiceStatus(ctx, status)
	default:
		return nil, fmt.Errorf("unsupported deployment mode: %s", config.DeploymentMode)
	}
}

// Reload reloads the Prometheus configuration
func (pm *PrometheusManager) Reload(ctx context.Context) error {
	// Load config from database
	config, err := pm.loadConfigFromDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config from database: %w", err)
	}

	// Create deployer
	deployer, err := pm.createDeployer(config)
	if err != nil {
		return fmt.Errorf("failed to create deployer: %w", err)
	}

	return deployer.Reload(ctx)
}

// getDockerStatus gets status for Docker deployment
func (pm *PrometheusManager) getDockerStatus(ctx context.Context, status *common.Status) (*common.Status, error) {
	// Load config from database
	config, loadErr := pm.loadConfigFromDatabase(ctx)
	if loadErr != nil {
		return nil, fmt.Errorf("failed to load config from database: %w", loadErr)
	}

	// Create deployer
	deployer, deployerErr := pm.createDeployer(config)
	if deployerErr != nil {
		return nil, fmt.Errorf("failed to create deployer: %w", deployerErr)
	}

	dockerDeployer, ok := deployer.(*DockerPrometheusDeployer)
	if !ok {
		return nil, fmt.Errorf("deployer is not a DockerPrometheusDeployer")
	}

	// Load config from database to get the port
	config, err := pm.loadConfigFromDatabase(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from database: %w", err)
	}

	containerName := fmt.Sprintf("chainlaunch-prometheus-%d", config.PrometheusPort)
	container, err := dockerDeployer.client.ContainerInspect(ctx, containerName)
	if err != nil {
		if client.IsErrNotFound(err) {
			return status, nil
		}
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Container exists, get its status
	status.Status = container.State.Status
	startedAt, err := time.Parse(time.RFC3339, container.State.StartedAt)
	if err != nil {
		status.Error = fmt.Sprintf("failed to parse start time: %v", err)
	} else {
		status.StartedAt = &startedAt
	}

	// Get configuration from database
	dbConfig, err := dockerDeployer.db.GetPrometheusConfig(ctx)
	if err != nil {
		status.Error = fmt.Sprintf("failed to get configuration: %v", err)
		return status, nil
	}

	// Add configuration details
	status.Version = strings.TrimPrefix(dbConfig.DockerImage, "prom/prometheus:")
	status.Port = int(dbConfig.PrometheusPort)
	status.ScrapeInterval = time.Duration(dbConfig.ScrapeInterval) * time.Second
	status.DeploymentMode = common.DeploymentMode(dbConfig.DeploymentMode)
	// Add network mode information for Docker deployments
	if dbConfig.NetworkMode.Valid {
		status.NetworkMode = common.NetworkMode(dbConfig.NetworkMode.String)
	}

	return status, nil
}

// getServiceStatus gets status for systemd/launchd deployment
func (pm *PrometheusManager) getServiceStatus(ctx context.Context, status *common.Status) (*common.Status, error) {
	// Load config from database
	config, loadErr := pm.loadConfigFromDatabase(ctx)
	if loadErr != nil {
		return nil, fmt.Errorf("failed to load config from database: %w", loadErr)
	}

	// Create deployer
	deployer, deployerErr := pm.createDeployer(config)
	if deployerErr != nil {
		return nil, fmt.Errorf("failed to create deployer: %w", deployerErr)
	}

	serviceDeployer, ok := deployer.(*ServicePrometheusDeployer)
	if !ok {
		return nil, fmt.Errorf("deployer is not a ServicePrometheusDeployer")
	}

	serviceStatus, err := serviceDeployer.GetStatus(ctx)
	if err != nil {
		status.Error = fmt.Sprintf("failed to get service status: %v", err)
		return status, nil
	}

	status.Status = serviceStatus

	// Get configuration from database
	dbConfig, err := serviceDeployer.db.GetPrometheusConfig(ctx)
	if err != nil {
		status.Error = fmt.Sprintf("failed to get configuration: %v", err)
		return status, nil
	}

	// Add configuration details
	status.Version = strings.TrimPrefix(dbConfig.DockerImage, "prom/prometheus:")
	status.Port = int(dbConfig.PrometheusPort)
	status.ScrapeInterval = time.Duration(dbConfig.ScrapeInterval) * time.Second
	status.DeploymentMode = common.DeploymentMode(dbConfig.DeploymentMode)

	return status, nil
}
