package metrics

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/metrics/common"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
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
}

// DockerPrometheusDeployer implements PrometheusDeployer for Docker deployment
type DockerPrometheusDeployer struct {
	config      *common.Config
	client      *client.Client
	db          *db.Queries
	nodeService *nodeservice.NodeService
}

// NewDockerPrometheusDeployer creates a new Docker-based Prometheus deployer
func NewDockerPrometheusDeployer(config *common.Config, db *db.Queries, nodeService *nodeservice.NodeService) (*DockerPrometheusDeployer, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &DockerPrometheusDeployer{
		config:      config,
		client:      cli,
		db:          db,
		nodeService: nodeService,
	}, nil
}

// Start starts the Prometheus container
func (d *DockerPrometheusDeployer) Start(ctx context.Context) error {
	containerName := "chainlaunch-prometheus"

	// Create volumes if they don't exist
	volumes := []string{
		"chainlaunch-prometheus-data",
		"chainlaunch-prometheus-config",
	}

	for _, volName := range volumes {
		_, err := d.client.VolumeCreate(ctx, volume.CreateOptions{
			Name: volName,
		})
		if err != nil {
			return fmt.Errorf("failed to create volume %s: %w", volName, err)
		}
	}

	// Generate prometheus.yml
	configData, err := d.generateConfig()
	if err != nil {
		return fmt.Errorf("failed to generate Prometheus config: %w", err)
	}

	// Pull Prometheus image
	imageName := fmt.Sprintf("prom/prometheus:%s", d.config.PrometheusVersion)
	_, err = d.client.ImagePull(ctx, imageName, image.PullOptions{
		// All: true,
	})
	if err != nil {
		return fmt.Errorf("failed to pull Prometheus image: %w", err)
	}

	// Create container config
	containerConfig := &container.Config{
		Image: imageName,
		Cmd: []string{
			"--config.file=/etc/prometheus/prometheus.yml",
			"--storage.tsdb.path=/prometheus",
			"--web.console.libraries=/usr/share/prometheus/console_libraries",
			"--web.console.templates=/usr/share/prometheus/consoles",
			"--web.enable-lifecycle",
			"--web.enable-admin-api",
		},
		ExposedPorts: nat.PortSet{
			nat.Port(fmt.Sprintf("%d/tcp", d.config.PrometheusPort)): struct{}{},
		},
	}

	// Create host config
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			nat.Port(fmt.Sprintf("%d/tcp", d.config.PrometheusPort)): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: fmt.Sprintf("%d", d.config.PrometheusPort),
				},
			},
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: "chainlaunch-prometheus-data",
				Target: "/prometheus",
			},
			{
				Type:   mount.TypeVolume,
				Source: "chainlaunch-prometheus-config",
				Target: "/etc/prometheus",
			},
		},
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
		ExtraHosts: []string{"host.docker.internal:host-gateway"},
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

	// Wait for container to be ready
	time.Sleep(2 * time.Second)

	// Create config file in the config volume
	configPath := "/etc/prometheus/prometheus.yml"
	_, err = d.client.ContainerExecCreate(ctx, containerName, container.ExecOptions{
		Cmd: []string{"sh", "-c", fmt.Sprintf("echo '%s' > %s", configData, configPath)},
	})
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	// Reload configuration
	return d.Reload(ctx)
}

// Stop stops the Prometheus container
func (d *DockerPrometheusDeployer) Stop(ctx context.Context) error {
	containerName := "chainlaunch-prometheus"

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

		// Use host.docker.internal to access host machine from container
		formattedAddress := fmt.Sprintf("host.docker.internal:%s", port)

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

		// Use host.docker.internal to access host machine from container
		formattedAddress := fmt.Sprintf("host.docker.internal:%s", port)

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

	besuNodes := make([]PeerNode, 0)
	for _, node := range nodes.Items {
		// Skip nodes that are not Besu nodes or don't have metrics enabled
		if node.BesuNode == nil || !node.BesuNode.MetricsEnabled {
			continue
		}

		// Get metrics host and port
		metricsHost := node.BesuNode.MetricsHost
		if metricsHost == "" || metricsHost == "0.0.0.0" {
			// Use host.docker.internal to access host machine from container
			metricsHost = "host.docker.internal"
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

// GetStatus returns the current status of the Prometheus container
func (d *DockerPrometheusDeployer) GetStatus(ctx context.Context) (string, error) {
	containerName := "chainlaunch-prometheus"

	// Get container info
	container, err := d.client.ContainerInspect(ctx, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}

	return container.State.Status, nil
}

// generateConfig generates the Prometheus configuration file content
func (d *DockerPrometheusDeployer) generateConfig() (string, error) {
	tmpl := `global:
  scrape_interval: {{ .ScrapeInterval }}

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']
`

	t, err := template.New("prometheus").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, d.config); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// PrometheusManager handles the lifecycle of a Prometheus instance
type PrometheusManager struct {
	deployer PrometheusDeployer
	client   *Client
}

// NewPrometheusManager creates a new PrometheusManager
func NewPrometheusManager(config *common.Config, db *db.Queries, nodeService *nodeservice.NodeService) (*PrometheusManager, error) {
	var deployer PrometheusDeployer
	var err error

	switch config.DeploymentMode {
	case "docker":
		deployer, err = NewDockerPrometheusDeployer(config, db, nodeService)
		if err != nil {
			return nil, fmt.Errorf("failed to create docker deployer: %w", err)
		}
	default:
		deployer, err = NewDockerPrometheusDeployer(config, db, nodeService)
		if err != nil {
			return nil, fmt.Errorf("failed to create docker deployer: %w", err)
		}
	}

	// Create Prometheus client
	client := NewClient(fmt.Sprintf("http://localhost:%d", config.PrometheusPort))

	return &PrometheusManager{
		deployer: deployer,
		client:   client,
	}, nil
}

// Reload reloads the Prometheus configuration
func (pm *PrometheusManager) Reload(ctx context.Context) error {
	return pm.deployer.Reload(ctx)
}

// Start starts the Prometheus instance
func (pm *PrometheusManager) Start(ctx context.Context) error {
	return pm.deployer.Start(ctx)
}

// Stop stops the Prometheus instance
func (pm *PrometheusManager) Stop(ctx context.Context) error {
	return pm.deployer.Stop(ctx)
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

	// Reload Prometheus configuration
	return pm.deployer.Reload(ctx)
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

	// Reload Prometheus configuration
	return pm.deployer.Reload(ctx)
}

// Query executes a PromQL query against Prometheus
func (pm *PrometheusManager) Query(ctx context.Context, query string) (*common.QueryResult, error) {
	return pm.client.Query(ctx, query)
}

// QueryRange executes a PromQL query with a time range
func (pm *PrometheusManager) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*common.QueryResult, error) {
	return pm.client.QueryRange(ctx, query, start, end, step)
}

// GetLabelValues retrieves values for a specific label
func (pm *PrometheusManager) GetLabelValues(ctx context.Context, labelName string, matches []string) ([]string, error) {
	return pm.client.GetLabelValues(ctx, labelName, matches)
}

// Reload reloads the Prometheus configuration
func (d *DockerPrometheusDeployer) Reload(ctx context.Context) error {
	containerName := "chainlaunch-prometheus"

	// Get peer nodes from the database
	peerNodes, err := d.getPeerNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get peer nodes: %w", err)
	}

	// Get orderer nodes from the database
	ordererNodes, err := d.getOrdererNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get orderer nodes: %w", err)
	}

	// Get Besu nodes from the database
	besuNodes, err := d.getBesuNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Besu nodes: %w", err)
	}

	// Generate new config with peer targets
	config := &PrometheusConfig{
		Global: GlobalConfig{
			ScrapeInterval: d.config.ScrapeInterval.String(),
		},
		ScrapeConfigs: []ScrapeConfig{
			{
				JobName: "prometheus",
				StaticConfigs: []StaticConfig{
					{
						Targets: []string{"localhost:9090"},
					},
				},
			},
		},
	}

	// Add peer node targets
	if len(peerNodes) > 0 {
		for _, node := range peerNodes {
			jobName := slugify(fmt.Sprintf("%s-%s", node.ID, node.Name))
			config.ScrapeConfigs = append(config.ScrapeConfigs, ScrapeConfig{
				JobName: jobName,
				StaticConfigs: []StaticConfig{
					{
						Targets: []string{node.OperationAddress},
					},
				},
			})
		}
	}

	// Add orderer node targets
	if len(ordererNodes) > 0 {
		for _, node := range ordererNodes {
			jobName := slugify(fmt.Sprintf("%s-%s", node.ID, node.Name))
			config.ScrapeConfigs = append(config.ScrapeConfigs, ScrapeConfig{
				JobName: jobName,
				StaticConfigs: []StaticConfig{
					{
						Targets: []string{node.OperationAddress},
					},
				},
			})
		}
	}

	// Add Besu node targets
	if len(besuNodes) > 0 {
		for _, node := range besuNodes {
			jobName := slugify(fmt.Sprintf("%s-%s", node.ID, node.Name))
			config.ScrapeConfigs = append(config.ScrapeConfigs, ScrapeConfig{
				JobName: jobName,
				StaticConfigs: []StaticConfig{
					{
						Targets: []string{node.OperationAddress},
					},
				},
			})
		}
	}

	// Marshal config to YAML
	configData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Create config file in the config volume
	configPath := "/etc/prometheus/prometheus.yml"
	_, err = d.client.ContainerExecCreate(ctx, containerName, container.ExecOptions{
		Cmd: []string{"sh", "-c", fmt.Sprintf("echo '%s' > %s", string(configData), configPath)},
	})
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	// Execute the exec command
	execID, err := d.client.ContainerExecCreate(ctx, containerName, container.ExecOptions{
		Cmd: []string{"sh", "-c", fmt.Sprintf("echo '%s' > %s", string(configData), configPath)},
	})
	if err != nil {
		return fmt.Errorf("failed to create exec command: %w", err)
	}

	if err := d.client.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
		return fmt.Errorf("failed to start exec command: %w", err)
	}

	// Reload Prometheus configuration
	reloadExecID, err := d.client.ContainerExecCreate(ctx, containerName, container.ExecOptions{
		Cmd: []string{"wget", "-q", "--post-data", "reload", "http://localhost:9090/-/reload"},
	})
	if err != nil {
		return fmt.Errorf("failed to create reload command: %w", err)
	}

	if err := d.client.ContainerExecStart(ctx, reloadExecID.ID, container.ExecStartOptions{}); err != nil {
		return fmt.Errorf("failed to start reload command: %w", err)
	}

	return nil
}
