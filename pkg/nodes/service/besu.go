package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"bytes"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/errors"
	networktypes "github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/besu"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/utils"
)

// GetBesuPorts attempts to find available ports for P2P and RPC, starting from default ports
func GetBesuPorts(baseP2PPort, baseRPCPort uint) (p2pPort uint, rpcPort uint, err error) {
	maxAttempts := 100
	// Try to find available ports for P2P and RPC
	p2pPorts, err := findConsecutivePorts(int(baseP2PPort), 1, int(baseP2PPort)+maxAttempts)
	if err != nil {
		return 0, 0, fmt.Errorf("could not find available P2P port: %w", err)
	}
	p2pPort = uint(p2pPorts[0])

	rpcPorts, err := findConsecutivePorts(int(baseRPCPort), 1, int(baseRPCPort)+maxAttempts)
	if err != nil {
		return 0, 0, fmt.Errorf("could not find available RPC port: %w", err)
	}
	rpcPort = uint(rpcPorts[0])

	return p2pPort, rpcPort, nil
}

// GetBesuNodeDefaults returns the default configuration for Besu nodes
func (s *NodeService) GetBesuNodeDefaults(besuNodes int) ([]BesuNodeDefaults, error) {
	// Validate node count
	if besuNodes <= 0 {
		besuNodes = 1
	}
	if besuNodes > 15 {
		return nil, fmt.Errorf("besu node count exceeds maximum supported nodes (15)")
	}

	// Fetch default IP from settings
	defaultIP := "127.0.0.1"
	if setting, err := s.settingsService.GetSetting(context.Background()); err == nil {
		if setting.Config.DefaultNodeExposeIP != "" {
			defaultIP = setting.Config.DefaultNodeExposeIP
		}
	}

	// Base ports for Besu nodes with sufficient spacing
	const (
		baseP2PPort     = 30303 // Starting P2P port
		baseRPCPort     = 8545  // Starting RPC port
		baseMetricsPort = 9545  // Starting metrics port
		portOffset      = 100   // Each node gets a 100 port range
	)

	// Create array to hold all node defaults
	nodeDefaults := make([]BesuNodeDefaults, besuNodes)

	// Generate defaults for each node
	for i := 0; i < besuNodes; i++ {
		// Try to get ports for each node
		p2pPort, rpcPort, err := GetBesuPorts(
			uint(baseP2PPort+(i*portOffset)),
			uint(baseRPCPort+(i*portOffset)),
		)
		if err != nil {
			// If we can't get the preferred ports, try from a higher range
			p2pPort, rpcPort, err = GetBesuPorts(
				uint(40303+(i*portOffset)),
				uint(18545+(i*portOffset)),
			)
			if err != nil {
				return nil, fmt.Errorf("failed to find available ports for node %d: %w", i+1, err)
			}
		}

		// Find available metrics port
		metricsPorts, err := findConsecutivePorts(int(baseMetricsPort+(i*portOffset)), 1, int(baseMetricsPort+(i*portOffset))+100)
		if err != nil {
			// If we can't get the preferred metrics port, try from a higher range
			metricsPorts, err = findConsecutivePorts(int(19545+(i*portOffset)), 1, int(19545+(i*portOffset))+100)
			if err != nil {
				return nil, fmt.Errorf("failed to find available metrics port for node %d: %w", i+1, err)
			}
		}

		// Create node defaults with unique ports
		nodeDefaults[i] = BesuNodeDefaults{
			P2PHost:    defaultIP, // Use default IP for p2p host
			P2PPort:    p2pPort,
			RPCHost:    defaultIP, // Use default IP for rpc host
			RPCPort:    rpcPort,
			InternalIP: defaultIP,
			ExternalIP: defaultIP,
			Mode:       ModeService,
			Env: map[string]string{
				"JAVA_OPTS": "-Xmx4g",
			},
			// Set metrics configuration
			MetricsEnabled:  true,
			MetricsHost:     defaultIP, // Use default IP for metrics host
			MetricsPort:     uint(metricsPorts[0]),
			MetricsProtocol: "PROMETHEUS",
		}
	}

	return nodeDefaults, nil
}

func (s *NodeService) getBesuFromConfig(ctx context.Context, dbNode *db.Node, config *types.BesuNodeConfig, deployConfig *types.BesuNodeDeploymentConfig) (*besu.LocalBesu, error) {
	network, err := s.db.GetNetwork(ctx, deployConfig.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}
	key, err := s.keymanagementService.GetKey(ctx, int(config.KeyID))
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}
	privateKeyDecrypted, err := s.keymanagementService.GetDecryptedPrivateKey(int(config.KeyID))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt key: %w", err)
	}
	var networkConfig networktypes.BesuNetworkConfig
	if err := json.Unmarshal([]byte(network.Config.String), &networkConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal network config: %w", err)
	}

	localBesu := besu.NewLocalBesu(
		besu.StartBesuOpts{
			ID:              dbNode.Slug,
			GenesisFile:     network.GenesisBlockB64.String,
			NetworkID:       deployConfig.NetworkID,
			P2PPort:         fmt.Sprintf("%d", deployConfig.P2PPort),
			RPCPort:         fmt.Sprintf("%d", deployConfig.RPCPort),
			ListenAddress:   deployConfig.P2PHost,
			MinerAddress:    key.EthereumAddress,
			ConsensusType:   "qbft", // TODO: get consensus type from network
			BootNodes:       config.BootNodes,
			Version:         config.Version,
			NodePrivateKey:  strings.TrimPrefix(privateKeyDecrypted, "0x"),
			Env:             config.Env,
			P2PHost:         config.P2PHost,
			RPCHost:         config.RPCHost,
			MetricsEnabled:  config.MetricsEnabled,
			MetricsPort:     config.MetricsPort,
			MetricsProtocol: config.MetricsProtocol,
		},
		string(config.Mode),
		dbNode.ID,
		s.logger,
		s.configService,
		s.settingsService,
		networkConfig,
	)

	return localBesu, nil
}

// stopBesuNode stops a Besu node
func (s *NodeService) stopBesuNode(ctx context.Context, dbNode *db.Node) error {
	// Load node configuration
	nodeConfig, err := utils.LoadNodeConfig([]byte(dbNode.NodeConfig.String))
	if err != nil {
		return fmt.Errorf("failed to deserialize node config: %w", err)
	}
	besuNodeConfig, ok := nodeConfig.(*types.BesuNodeConfig)
	if !ok {
		return fmt.Errorf("failed to assert node config to BesuNodeConfig")
	}

	// Load deployment configuration
	deploymentConfig, err := utils.DeserializeDeploymentConfig(dbNode.DeploymentConfig.String)
	if err != nil {
		return fmt.Errorf("failed to deserialize deployment config: %w", err)
	}
	besuDeployConfig, ok := deploymentConfig.(*types.BesuNodeDeploymentConfig)
	if !ok {
		return fmt.Errorf("failed to assert deployment config to BesuNodeDeploymentConfig")
	}

	// Get Besu instance
	localBesu, err := s.getBesuFromConfig(ctx, dbNode, besuNodeConfig, besuDeployConfig)
	if err != nil {
		return fmt.Errorf("failed to get besu instance: %w", err)
	}

	// Stop the node
	err = localBesu.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop besu node: %w", err)
	}

	return nil
}

// startBesuNode starts a Besu node
func (s *NodeService) startBesuNode(ctx context.Context, dbNode *db.Node) error {
	// Load node configuration
	nodeConfig, err := utils.LoadNodeConfig([]byte(dbNode.NodeConfig.String))
	if err != nil {
		return fmt.Errorf("failed to deserialize node config: %w", err)
	}
	besuNodeConfig, ok := nodeConfig.(*types.BesuNodeConfig)
	if !ok {
		return fmt.Errorf("failed to assert node config to BesuNodeConfig")
	}

	// Load deployment configuration
	deploymentConfig, err := utils.DeserializeDeploymentConfig(dbNode.DeploymentConfig.String)
	if err != nil {
		return fmt.Errorf("failed to deserialize deployment config: %w", err)
	}
	besuDeployConfig, ok := deploymentConfig.(*types.BesuNodeDeploymentConfig)
	if !ok {
		return fmt.Errorf("failed to assert deployment config to BesuNodeDeploymentConfig")
	}

	// Get key for node
	key, err := s.keymanagementService.GetKey(ctx, int(besuNodeConfig.KeyID))
	if err != nil {
		return fmt.Errorf("failed to get key: %w", err)
	}
	network, err := s.db.GetNetwork(ctx, besuDeployConfig.NetworkID)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}
	privateKeyDecrypted, err := s.keymanagementService.GetDecryptedPrivateKey(int(besuNodeConfig.KeyID))
	if err != nil {
		return fmt.Errorf("failed to decrypt key: %w", err)
	}
	var networkConfig networktypes.BesuNetworkConfig
	if err := json.Unmarshal([]byte(network.Config.String), &networkConfig); err != nil {
		return fmt.Errorf("failed to unmarshal network config: %w", err)
	}

	// Create LocalBesu instance
	localBesu := besu.NewLocalBesu(
		besu.StartBesuOpts{
			ID:              dbNode.Slug,
			GenesisFile:     network.GenesisBlockB64.String,
			NetworkID:       besuDeployConfig.NetworkID,
			ChainID:         networkConfig.ChainID,
			P2PPort:         fmt.Sprintf("%d", besuDeployConfig.P2PPort),
			RPCPort:         fmt.Sprintf("%d", besuDeployConfig.RPCPort),
			ListenAddress:   besuDeployConfig.P2PHost,
			MinerAddress:    key.EthereumAddress,
			ConsensusType:   "qbft", // TODO: get consensus type from network
			BootNodes:       besuNodeConfig.BootNodes,
			Version:         "25.4.1", // TODO: get version from network
			NodePrivateKey:  strings.TrimPrefix(privateKeyDecrypted, "0x"),
			Env:             besuNodeConfig.Env,
			P2PHost:         besuNodeConfig.P2PHost,
			RPCHost:         besuNodeConfig.RPCHost,
			MetricsEnabled:  besuDeployConfig.MetricsEnabled,
			MetricsPort:     besuDeployConfig.MetricsPort,
			MetricsProtocol: "PROMETHEUS",
		},
		string(besuNodeConfig.Mode),
		dbNode.ID,
		s.logger,
		s.configService,
		s.settingsService,
		networkConfig,
	)

	// Start the node
	_, err = localBesu.Start()
	if err != nil {
		return fmt.Errorf("failed to start besu node: %w", err)
	}

	s.logger.Info("Started Besu node",
		"nodeID", dbNode.ID,
		"name", dbNode.Name,
		"networkID", besuDeployConfig.NetworkID,
	)

	return nil
}

// UpdateBesuNodeOpts contains the options for updating a Besu node
type UpdateBesuNodeRequest struct {
	NetworkID  uint              `json:"networkId" validate:"required"`
	Mode       string            `json:"mode,omitempty"`
	P2PHost    string            `json:"p2pHost" validate:"required"`
	P2PPort    uint              `json:"p2pPort" validate:"required"`
	RPCHost    string            `json:"rpcHost" validate:"required"`
	RPCPort    uint              `json:"rpcPort" validate:"required"`
	Bootnodes  []string          `json:"bootnodes,omitempty"`
	ExternalIP string            `json:"externalIp,omitempty"`
	InternalIP string            `json:"internalIp,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	// Metrics configuration
	MetricsEnabled bool  `json:"metricsEnabled"`
	MetricsPort    int64 `json:"metricsPort"`
}

// UpdateBesuNode updates an existing Besu node configuration
func (s *NodeService) UpdateBesuNode(ctx context.Context, nodeID int64, req UpdateBesuNodeRequest) (*NodeResponse, error) {
	// Get existing node
	node, err := s.db.GetNode(ctx, nodeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("node not found", nil)
		}
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	// Verify node type
	if types.NodeType(node.NodeType.String) != types.NodeTypeBesuFullnode {
		return nil, errors.NewValidationError("node is not a Besu node", nil)
	}

	// Load current config
	nodeConfig, err := utils.LoadNodeConfig([]byte(node.NodeConfig.String))
	if err != nil {
		return nil, fmt.Errorf("failed to load besu config: %w", err)
	}

	besuConfig, ok := nodeConfig.(*types.BesuNodeConfig)
	if !ok {
		return nil, fmt.Errorf("invalid besu config type")
	}

	// Load deployment config
	deployBesuConfig := &types.BesuNodeDeploymentConfig{}
	if node.DeploymentConfig.Valid {
		deploymentConfig, err := utils.DeserializeDeploymentConfig(node.DeploymentConfig.String)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize deployment config: %w", err)
		}
		var ok bool
		deployBesuConfig, ok = deploymentConfig.(*types.BesuNodeDeploymentConfig)
		if !ok {
			return nil, fmt.Errorf("invalid besu deployment config type")
		}
	}

	// --- MODE CHANGE LOGIC ---
	modeChanged := false
	var newMode string
	if req.Mode != "" {
		if req.Mode != string(besuConfig.Mode) {
			modeChanged = true
			newMode = req.Mode
		}
	}
	if modeChanged {
		if err := s.stopBesuNode(ctx, node); err != nil {
			s.logger.Warn("Failed to stop Besu node before mode change", "error", err)
		}
		besuConfig.Mode = newMode
		deployBesuConfig.Mode = newMode
	}

	// Update configuration fields
	besuConfig.NetworkID = int64(req.NetworkID)
	besuConfig.P2PPort = req.P2PPort
	besuConfig.RPCPort = req.RPCPort
	besuConfig.P2PHost = req.P2PHost
	besuConfig.RPCHost = req.RPCHost
	deployBesuConfig.NetworkID = int64(req.NetworkID)
	deployBesuConfig.P2PPort = req.P2PPort
	deployBesuConfig.RPCPort = req.RPCPort
	deployBesuConfig.P2PHost = req.P2PHost
	deployBesuConfig.RPCHost = req.RPCHost
	if req.Bootnodes != nil {
		besuConfig.BootNodes = req.Bootnodes
	}

	if req.ExternalIP != "" {
		besuConfig.ExternalIP = req.ExternalIP
		deployBesuConfig.ExternalIP = req.ExternalIP
	}
	if req.InternalIP != "" {
		besuConfig.InternalIP = req.InternalIP
		deployBesuConfig.InternalIP = req.InternalIP
	}

	// Update metrics configuration
	besuConfig.MetricsEnabled = req.MetricsEnabled
	besuConfig.MetricsPort = req.MetricsPort
	deployBesuConfig.MetricsEnabled = req.MetricsEnabled
	deployBesuConfig.MetricsPort = req.MetricsPort

	// Update environment variables
	if req.Env != nil {
		besuConfig.Env = req.Env
		deployBesuConfig.Env = req.Env
	}

	// Validate the updated configuration
	if err := s.validateBesuNodeConfig(besuConfig); err != nil {
		return nil, fmt.Errorf("invalid besu configuration: %w", err)
	}

	// Get the key to update the enodeURL
	key, err := s.keymanagementService.GetKey(ctx, int(besuConfig.KeyID))
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	// Update enodeURL based on the public key, external IP and P2P port
	if key.PublicKey != "" {
		publicKey := key.PublicKey[2:]
		deployBesuConfig.EnodeURL = fmt.Sprintf("enode://%s@%s:%d", publicKey, besuConfig.ExternalIP, besuConfig.P2PPort)
	}

	// Store updated node config
	configBytes, err := utils.StoreNodeConfig(besuConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to store node config: %w", err)
	}

	node, err = s.db.UpdateNodeConfig(ctx, &db.UpdateNodeConfigParams{
		ID: nodeID,
		NodeConfig: sql.NullString{
			String: string(configBytes),
			Valid:  true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update node config: %w", err)
	}

	// Update deployment config
	deploymentConfigBytes, err := json.Marshal(deployBesuConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deployment config: %w", err)
	}

	node, err = s.db.UpdateDeploymentConfig(ctx, &db.UpdateDeploymentConfigParams{
		ID: nodeID,
		DeploymentConfig: sql.NullString{
			String: string(deploymentConfigBytes),
			Valid:  true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment config: %w", err)
	}

	// If mode changed, start the node with the new mode
	if modeChanged {
		if err := s.startBesuNode(ctx, node); err != nil {
			s.logger.Warn("Failed to start Besu node after mode change", "error", err)
		}
	}
	err = s.metricsService.Reload(ctx)
	if err != nil {
		s.logger.Warn("Failed to reload metrics", "error", err)
	}

	// Return updated node
	_, nodeResponse := s.mapDBNodeToServiceNode(node)
	return nodeResponse, nil
}

// validateBesuConfig validates the Besu node configuration
func (s *NodeService) validateBesuConfig(config *types.BesuNodeConfig) error {

	if config.P2PPort == 0 {
		return fmt.Errorf("p2p port is required")
	}
	if config.RPCPort == 0 {
		return fmt.Errorf("rpc port is required")
	}
	if config.NetworkID == 0 {
		return fmt.Errorf("network ID is required")
	}
	if config.P2PHost == "" {
		return fmt.Errorf("p2p host is required")
	}
	if config.RPCHost == "" {
		return fmt.Errorf("rpc host is required")
	}
	if config.ExternalIP == "" {
		return fmt.Errorf("external IP is required")
	}
	if config.InternalIP == "" {
		return fmt.Errorf("internal IP is required")
	}

	return nil
}

// cleanupBesuResources cleans up resources specific to a Besu node
func (s *NodeService) cleanupBesuResources(ctx context.Context, node *db.Node) error {

	// Load node configuration
	nodeConfig, err := utils.LoadNodeConfig([]byte(node.NodeConfig.String))
	if err != nil {
		s.logger.Warn("Failed to load node config during cleanup", "error", err)
		// Continue with cleanup even if config loading fails
	}

	// Load deployment configuration
	deploymentConfig, err := utils.DeserializeDeploymentConfig(node.DeploymentConfig.String)
	if err != nil {
		s.logger.Warn("Failed to load deployment config during cleanup", "error", err)
		// Continue with cleanup even if config loading fails
	}

	// Create Besu instance for cleanup
	var localBesu *besu.LocalBesu
	if nodeConfig != nil && deploymentConfig != nil {
		besuNodeConfig, ok := nodeConfig.(*types.BesuNodeConfig)
		if !ok {
			s.logger.Warn("Invalid node config type during cleanup")
		}
		besuDeployConfig, ok := deploymentConfig.(*types.BesuNodeDeploymentConfig)
		if !ok {
			s.logger.Warn("Invalid deployment config type during cleanup")
		}
		if besuNodeConfig != nil && besuDeployConfig != nil {
			localBesu, err = s.getBesuFromConfig(ctx, node, besuNodeConfig, besuDeployConfig)
			if err != nil {
				s.logger.Warn("Failed to create Besu instance during cleanup", "error", err)
			}
		}
	}

	// Stop the service if it's running and we have a valid Besu instance
	if localBesu != nil {
		if err := localBesu.Stop(); err != nil {
			s.logger.Warn("Failed to stop Besu service during cleanup", "error", err)
			// Continue with cleanup even if stop fails
		}
	}

	// Clean up Besu-specific directories
	dirsToClean := []string{
		filepath.Join(s.configService.GetDataPath(), "nodes", node.Slug),
		filepath.Join(s.configService.GetDataPath(), "besu", node.Slug),
		filepath.Join(s.configService.GetDataPath(), "besu", "nodes", node.Slug),
	}

	for _, dir := range dirsToClean {
		if err := os.RemoveAll(dir); err != nil {
			if !os.IsNotExist(err) {
				s.logger.Warn("Failed to remove Besu directory",
					"path", dir,
					"error", err)
			}
		} else {
			s.logger.Info("Successfully removed Besu directory",
				"path", dir)
		}
	}

	// Clean up service files based on platform
	switch runtime.GOOS {
	case "linux":
		// Remove systemd service file
		if localBesu != nil {
			serviceFile := fmt.Sprintf("/etc/systemd/system/besu-%s.service", node.Slug)
			if err := os.Remove(serviceFile); err != nil {
				if !os.IsNotExist(err) {
					s.logger.Warn("Failed to remove systemd service file", "error", err)
				}
			}
		}

	case "darwin":
		// Remove launchd plist file
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		if localBesu != nil {
			plistFile := filepath.Join(homeDir, "Library/LaunchAgents", fmt.Sprintf("dev.chainlaunch.besu.%s.plist", node.Slug))
			if err := os.Remove(plistFile); err != nil {
				if !os.IsNotExist(err) {
					s.logger.Warn("Failed to remove launchd plist file", "error", err)
				}
			}
		}
	}

	// Clean up any data directories
	dataDir := filepath.Join(s.configService.GetDataPath(), "data", "besu", node.Slug)
	if err := os.RemoveAll(dataDir); err != nil {
		if !os.IsNotExist(err) {
			s.logger.Warn("Failed to remove Besu data directory",
				"path", dataDir,
				"error", err)
		}
	} else {
		s.logger.Info("Successfully removed Besu data directory",
			"path", dataDir)
	}

	return nil
}

// initializeBesuNode initializes a Besu node
func (s *NodeService) initializeBesuNode(ctx context.Context, dbNode *db.Node, config *types.BesuNodeConfig) (types.NodeDeploymentConfig, error) {
	// Validate key exists
	key, err := s.keymanagementService.GetKey(ctx, int(config.KeyID))
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}
	if key.EthereumAddress == "" {
		return nil, fmt.Errorf("key %d has no ethereum address", config.KeyID)
	}
	enodeURL := fmt.Sprintf("enode://%s@%s:%d", key.PublicKey[2:], config.ExternalIP, config.P2PPort)

	// Validate ports
	if err := s.validatePort(config.P2PHost, int(config.P2PPort)); err != nil {
		return nil, fmt.Errorf("invalid P2P port: %w", err)
	}
	if err := s.validatePort(config.RPCHost, int(config.RPCPort)); err != nil {
		return nil, fmt.Errorf("invalid RPC port: %w", err)
	}

	// Create deployment config
	deploymentConfig := &types.BesuNodeDeploymentConfig{
		BaseDeploymentConfig: types.BaseDeploymentConfig{
			Type: "besu",
			Mode: string(config.Mode),
		},
		KeyID:          config.KeyID,
		P2PPort:        config.P2PPort,
		RPCPort:        config.RPCPort,
		NetworkID:      config.NetworkID,
		ExternalIP:     config.ExternalIP,
		P2PHost:        config.P2PHost,
		RPCHost:        config.RPCHost,
		InternalIP:     config.InternalIP,
		EnodeURL:       enodeURL,
		MetricsEnabled: config.MetricsEnabled,
		MetricsPort:    config.MetricsPort,
	}

	// Update node endpoint
	endpoint := fmt.Sprintf("%s:%d", config.P2PHost, config.P2PPort)
	_, err = s.db.UpdateNodeEndpoint(ctx, &db.UpdateNodeEndpointParams{
		ID: dbNode.ID,
		Endpoint: sql.NullString{
			String: endpoint,
			Valid:  true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update node endpoint: %w", err)
	}

	// Update node public endpoint if external IP is set
	if config.ExternalIP != "" {
		publicEndpoint := fmt.Sprintf("%s:%d", config.ExternalIP, config.P2PPort)
		_, err = s.db.UpdateNodePublicEndpoint(ctx, &db.UpdateNodePublicEndpointParams{
			ID: dbNode.ID,
			PublicEndpoint: sql.NullString{
				String: publicEndpoint,
				Valid:  true,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to update node public endpoint: %w", err)
		}
	}

	return deploymentConfig, nil
}

// RPCClient represents a JSON-RPC client for Besu nodes
type RPCClient struct {
	client  *http.Client
	baseURL string
}

// NewRPCClient creates a new RPC client for a Besu node
func NewRPCClient(rpcHost string, rpcPort uint) *RPCClient {
	return &RPCClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: fmt.Sprintf("http://%s:%d", rpcHost, rpcPort),
	}
}

// callRPC makes a JSON-RPC call to the Besu node
func (c *RPCClient) callRPC(ctx context.Context, method string, params []interface{}) ([]byte, error) {
	requestBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON-RPC request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for RPC error
	var rpcResp struct {
		Jsonrpc string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s (code: %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	return rpcResp.Result, nil
}

// GetAccounts returns accounts managed by the node
func (c *RPCClient) GetAccounts(ctx context.Context) ([]string, error) {
	result, err := c.callRPC(ctx, "eth_accounts", []interface{}{})
	if err != nil {
		return nil, err
	}

	var accounts []string
	if err := json.Unmarshal(result, &accounts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal accounts: %w", err)
	}

	return accounts, nil
}

// GetBalance gets balance of an address in Wei
func (c *RPCClient) GetBalance(ctx context.Context, address, blockTag string) (string, error) {
	result, err := c.callRPC(ctx, "eth_getBalance", []interface{}{address, blockTag})
	if err != nil {
		return "", err
	}

	var balance string
	if err := json.Unmarshal(result, &balance); err != nil {
		return "", fmt.Errorf("failed to unmarshal balance: %w", err)
	}

	return balance, nil
}

// GetCode gets bytecode at an address
func (c *RPCClient) GetCode(ctx context.Context, address, blockTag string) (string, error) {
	result, err := c.callRPC(ctx, "eth_getCode", []interface{}{address, blockTag})
	if err != nil {
		return "", err
	}

	var code string
	if err := json.Unmarshal(result, &code); err != nil {
		return "", fmt.Errorf("failed to unmarshal code: %w", err)
	}

	return code, nil
}

// GetStorageAt gets storage value at a position for an address
func (c *RPCClient) GetStorageAt(ctx context.Context, address, position, blockTag string) (string, error) {
	result, err := c.callRPC(ctx, "eth_getStorageAt", []interface{}{address, position, blockTag})
	if err != nil {
		return "", err
	}

	var value string
	if err := json.Unmarshal(result, &value); err != nil {
		return "", fmt.Errorf("failed to unmarshal storage value: %w", err)
	}

	return value, nil
}

// GetTransactionCount gets nonce for an address
func (c *RPCClient) GetTransactionCount(ctx context.Context, address, blockTag string) (string, error) {
	result, err := c.callRPC(ctx, "eth_getTransactionCount", []interface{}{address, blockTag})
	if err != nil {
		return "", err
	}

	var count string
	if err := json.Unmarshal(result, &count); err != nil {
		return "", fmt.Errorf("failed to unmarshal transaction count: %w", err)
	}

	return count, nil
}

// GetBlockNumber gets the latest block number
func (c *RPCClient) GetBlockNumber(ctx context.Context) (string, error) {
	result, err := c.callRPC(ctx, "eth_blockNumber", []interface{}{})
	if err != nil {
		return "", err
	}

	var blockNumber string
	if err := json.Unmarshal(result, &blockNumber); err != nil {
		return "", fmt.Errorf("failed to unmarshal block number: %w", err)
	}

	return blockNumber, nil
}

// GetBlockByHash gets block details by hash
func (c *RPCClient) GetBlockByHash(ctx context.Context, blockHash string, fullTx bool) (map[string]interface{}, error) {
	result, err := c.callRPC(ctx, "eth_getBlockByHash", []interface{}{blockHash, fullTx})
	if err != nil {
		return nil, err
	}

	var block map[string]interface{}
	if err := json.Unmarshal(result, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return block, nil
}

// GetBlockByNumber gets block by number or tag
func (c *RPCClient) GetBlockByNumber(ctx context.Context, blockNumber, blockTag string, fullTx bool) (map[string]interface{}, error) {
	param := blockNumber
	if blockTag != "" {
		param = blockTag
	}
	result, err := c.callRPC(ctx, "eth_getBlockByNumber", []interface{}{param, fullTx})
	if err != nil {
		return nil, err
	}

	var block map[string]interface{}
	if err := json.Unmarshal(result, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return block, nil
}

// GetUncleByBlockHashAndIndex gets uncle block by parent hash and index
func (c *RPCClient) GetUncleByBlockHashAndIndex(ctx context.Context, blockHash, uncleIndex string) (map[string]interface{}, error) {
	result, err := c.callRPC(ctx, "eth_getUncleByBlockHashAndIndex", []interface{}{blockHash, uncleIndex})
	if err != nil {
		return nil, err
	}

	var uncle map[string]interface{}
	if err := json.Unmarshal(result, &uncle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal uncle: %w", err)
	}

	return uncle, nil
}

// GetUncleByBlockNumberAndIndex gets uncle by block number and index
func (c *RPCClient) GetUncleByBlockNumberAndIndex(ctx context.Context, blockNumber, blockTag, uncleIndex string) (map[string]interface{}, error) {
	param := blockNumber
	if blockTag != "" {
		param = blockTag
	}
	result, err := c.callRPC(ctx, "eth_getUncleByBlockNumberAndIndex", []interface{}{param, uncleIndex})
	if err != nil {
		return nil, err
	}

	var uncle map[string]interface{}
	if err := json.Unmarshal(result, &uncle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal uncle: %w", err)
	}

	return uncle, nil
}

// GetUncleCountByBlockHash gets uncle count for a block by hash
func (c *RPCClient) GetUncleCountByBlockHash(ctx context.Context, blockHash string) (string, error) {
	result, err := c.callRPC(ctx, "eth_getUncleCountByBlockHash", []interface{}{blockHash})
	if err != nil {
		return "", err
	}

	var count string
	if err := json.Unmarshal(result, &count); err != nil {
		return "", fmt.Errorf("failed to unmarshal uncle count: %w", err)
	}

	return count, nil
}

// GetUncleCountByBlockNumber gets uncle count by block number
func (c *RPCClient) GetUncleCountByBlockNumber(ctx context.Context, blockNumber, blockTag string) (string, error) {
	param := blockNumber
	if blockTag != "" {
		param = blockTag
	}
	result, err := c.callRPC(ctx, "eth_getUncleCountByBlockNumber", []interface{}{param})
	if err != nil {
		return "", err
	}

	var count string
	if err := json.Unmarshal(result, &count); err != nil {
		return "", fmt.Errorf("failed to unmarshal uncle count: %w", err)
	}

	return count, nil
}

// GetTransactionByHash gets tx details by hash
func (c *RPCClient) GetTransactionByHash(ctx context.Context, txHash string) (map[string]interface{}, error) {
	result, err := c.callRPC(ctx, "eth_getTransactionByHash", []interface{}{txHash})
	if err != nil {
		return nil, err
	}

	var tx map[string]interface{}
	if err := json.Unmarshal(result, &tx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	return tx, nil
}

// GetTransactionByBlockHashAndIndex gets tx by block hash and index
func (c *RPCClient) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash, txIndex string) (map[string]interface{}, error) {
	result, err := c.callRPC(ctx, "eth_getTransactionByBlockHashAndIndex", []interface{}{blockHash, txIndex})
	if err != nil {
		return nil, err
	}

	var tx map[string]interface{}
	if err := json.Unmarshal(result, &tx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	return tx, nil
}

// GetTransactionByBlockNumberAndIndex gets tx by block number and index
func (c *RPCClient) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNumber, blockTag, txIndex string) (map[string]interface{}, error) {
	param := blockNumber
	if blockTag != "" {
		param = blockTag
	}
	result, err := c.callRPC(ctx, "eth_getTransactionByBlockNumberAndIndex", []interface{}{param, txIndex})
	if err != nil {
		return nil, err
	}

	var tx map[string]interface{}
	if err := json.Unmarshal(result, &tx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	return tx, nil
}

// GetTransactionReceipt gets receipt for a transaction
func (c *RPCClient) GetTransactionReceipt(ctx context.Context, txHash string) (map[string]interface{}, error) {
	result, err := c.callRPC(ctx, "eth_getTransactionReceipt", []interface{}{txHash})
	if err != nil {
		return nil, err
	}

	var receipt map[string]interface{}
	if err := json.Unmarshal(result, &receipt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction receipt: %w", err)
	}

	return receipt, nil
}

// GetFeeHistory gets historical gas fees
func (c *RPCClient) GetFeeHistory(ctx context.Context, blockCount, newestBlock, rewardPercentiles string) (map[string]interface{}, error) {
	result, err := c.callRPC(ctx, "eth_feeHistory", []interface{}{blockCount, newestBlock, rewardPercentiles})
	if err != nil {
		return nil, err
	}

	var feeHistory map[string]interface{}
	if err := json.Unmarshal(result, &feeHistory); err != nil {
		return nil, fmt.Errorf("failed to unmarshal fee history: %w", err)
	}

	return feeHistory, nil
}

// QbftSignerMetric represents a QBFT signer metric
type QbftSignerMetric struct {
	Address                 string `json:"address"`
	ProposedBlockCount      string `json:"proposedBlockCount"`
	LastProposedBlockNumber string `json:"lastProposedBlockNumber"`
}

// GetQbftSignerMetrics gets QBFT signer metrics
func (c *RPCClient) GetQbftSignerMetrics(ctx context.Context) ([]QbftSignerMetric, error) {
	result, err := c.callRPC(ctx, "qbft_getSignerMetrics", []interface{}{})
	if err != nil {
		return nil, err
	}

	var metrics []QbftSignerMetric
	if err := json.Unmarshal(result, &metrics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal QBFT signer metrics: %w", err)
	}

	return metrics, nil
}

// GetQbftRequestTimeoutSeconds gets QBFT request timeout in seconds
func (c *RPCClient) GetQbftRequestTimeoutSeconds(ctx context.Context) (int64, error) {
	result, err := c.callRPC(ctx, "qbft_getRequestTimeoutSeconds", []interface{}{})
	if err != nil {
		return 0, err
	}

	var timeout int64
	if err := json.Unmarshal(result, &timeout); err != nil {
		return 0, fmt.Errorf("failed to unmarshal QBFT request timeout: %w", err)
	}

	return timeout, nil
}

// QbftPendingVote represents a pending vote for a validator proposal
type QbftPendingVote struct {
	Proposer string `json:"proposer"`
	Vote     bool   `json:"vote"`
}

// QbftPendingVotes represents a map of pending validator proposals and their votes
// The actual format is: {"validatorAddress": true}
type QbftPendingVotes map[string]bool

// QbftDiscardValidatorVote discards a pending vote for a validator proposal
func (c *RPCClient) QbftDiscardValidatorVote(ctx context.Context, validatorAddress string) (bool, error) {
	result, err := c.callRPC(ctx, "qbft_discardValidatorVote", []interface{}{validatorAddress})
	if err != nil {
		return false, err
	}

	var success bool
	if err := json.Unmarshal(result, &success); err != nil {
		return false, fmt.Errorf("failed to unmarshal QBFT discard validator vote response: %w, raw response: %s", err, string(result))
	}

	return success, nil
}

// QbftGetPendingVotes retrieves a map of pending validator proposals and their votes
func (c *RPCClient) QbftGetPendingVotes(ctx context.Context) (QbftPendingVotes, error) {
	result, err := c.callRPC(ctx, "qbft_getPendingVotes", []interface{}{})
	if err != nil {
		return nil, err
	}

	// Try to unmarshal as the expected map structure (validator address -> bool)
	var pendingVotes QbftPendingVotes
	if err := json.Unmarshal(result, &pendingVotes); err == nil {
		return pendingVotes, nil
	}

	// If that fails, check if it's a boolean (indicating no pending votes)
	var boolResult bool
	if err := json.Unmarshal(result, &boolResult); err == nil {
		// If it's a boolean and false, return empty map (no pending votes)
		if !boolResult {
			return make(QbftPendingVotes), nil
		}
		// If it's true, this might indicate an error or different response format
		return nil, fmt.Errorf("unexpected QBFT pending votes response: got boolean true, expected map structure")
	}

	// If neither map nor boolean works, try to unmarshal as null
	var nullResult interface{}
	if err := json.Unmarshal(result, &nullResult); err == nil && nullResult == nil {
		return make(QbftPendingVotes), nil
	}

	// If all else fails, return the raw response for debugging
	return nil, fmt.Errorf("failed to unmarshal QBFT pending votes: unexpected response format: %s", string(result))
}

// QbftProposeValidatorVote proposes a vote to add (true) or remove (false) a validator
func (c *RPCClient) QbftProposeValidatorVote(ctx context.Context, validatorAddress string, vote bool) (bool, error) {
	result, err := c.callRPC(ctx, "qbft_proposeValidatorVote", []interface{}{validatorAddress, vote})
	if err != nil {
		return false, err
	}

	var success bool
	if err := json.Unmarshal(result, &success); err != nil {
		return false, fmt.Errorf("failed to unmarshal QBFT propose validator vote response: %w, raw response: %s", err, string(result))
	}

	return success, nil
}

// QbftGetValidatorsByBlockHash retrieves the list of validators for a specific block by its hash
func (c *RPCClient) QbftGetValidatorsByBlockHash(ctx context.Context, blockHash string) ([]string, error) {
	result, err := c.callRPC(ctx, "qbft_getValidatorsByBlockHash", []interface{}{blockHash})
	if err != nil {
		return nil, err
	}

	var validators []string
	if err := json.Unmarshal(result, &validators); err != nil {
		return nil, fmt.Errorf("failed to unmarshal QBFT validators by block hash: %w, raw response: %s", err, string(result))
	}

	return validators, nil
}

// QbftGetValidatorsByBlockNumber retrieves the list of validators for a specific block by its number
func (c *RPCClient) QbftGetValidatorsByBlockNumber(ctx context.Context, blockNumber string) ([]string, error) {
	result, err := c.callRPC(ctx, "qbft_getValidatorsByBlockNumber", []interface{}{blockNumber})
	if err != nil {
		return nil, err
	}

	var validators []string
	if err := json.Unmarshal(result, &validators); err != nil {
		return nil, fmt.Errorf("failed to unmarshal QBFT validators by block number: %w, raw response: %s", err, string(result))
	}

	return validators, nil
}

// GetBlockTransactionCountByHash gets tx count in a block by hash
func (c *RPCClient) GetBlockTransactionCountByHash(ctx context.Context, blockHash string) (string, error) {
	result, err := c.callRPC(ctx, "eth_getBlockTransactionCountByHash", []interface{}{blockHash})
	if err != nil {
		return "", err
	}

	var count string
	if err := json.Unmarshal(result, &count); err != nil {
		return "", fmt.Errorf("failed to unmarshal transaction count: %w", err)
	}

	return count, nil
}

// GetBlockTransactionCountByNumber gets tx count by block number
func (c *RPCClient) GetBlockTransactionCountByNumber(ctx context.Context, blockNumber, blockTag string) (string, error) {
	param := blockNumber
	if blockTag != "" {
		param = blockTag
	}
	result, err := c.callRPC(ctx, "eth_getBlockTransactionCountByNumber", []interface{}{param})
	if err != nil {
		return "", err
	}

	var count string
	if err := json.Unmarshal(result, &count); err != nil {
		return "", fmt.Errorf("failed to unmarshal transaction count: %w", err)
	}

	return count, nil
}

// GetLogs gets event logs
func (c *RPCClient) GetLogs(ctx context.Context, filter map[string]interface{}) ([]map[string]interface{}, error) {
	result, err := c.callRPC(ctx, "eth_getLogs", []interface{}{filter})
	if err != nil {
		return nil, err
	}

	var logs []map[string]interface{}
	if err := json.Unmarshal(result, &logs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal logs: %w", err)
	}

	return logs, nil
}

// GetPendingTransactions gets pending tx in the mempool
func (c *RPCClient) GetPendingTransactions(ctx context.Context) ([]map[string]interface{}, error) {
	result, err := c.callRPC(ctx, "eth_pendingTransactions", []interface{}{})
	if err != nil {
		return nil, err
	}

	var txs []map[string]interface{}
	if err := json.Unmarshal(result, &txs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pending transactions: %w", err)
	}

	return txs, nil
}

// GetChainId gets the chain ID
func (c *RPCClient) GetChainId(ctx context.Context) (string, error) {
	result, err := c.callRPC(ctx, "eth_chainId", []interface{}{})
	if err != nil {
		return "", err
	}

	var chainId string
	if err := json.Unmarshal(result, &chainId); err != nil {
		return "", fmt.Errorf("failed to unmarshal chain ID: %w", err)
	}

	return chainId, nil
}

// GetProtocolVersion gets Ethereum protocol version
func (c *RPCClient) GetProtocolVersion(ctx context.Context) (string, error) {
	result, err := c.callRPC(ctx, "eth_protocolVersion", []interface{}{})
	if err != nil {
		return "", err
	}

	var version string
	if err := json.Unmarshal(result, &version); err != nil {
		return "", fmt.Errorf("failed to unmarshal protocol version: %w", err)
	}

	return version, nil
}

// GetSyncing gets sync status
func (c *RPCClient) GetSyncing(ctx context.Context) (interface{}, error) {
	result, err := c.callRPC(ctx, "eth_syncing", []interface{}{})
	if err != nil {
		return nil, err
	}

	var syncing interface{}
	if err := json.Unmarshal(result, &syncing); err != nil {
		return nil, fmt.Errorf("failed to unmarshal syncing status: %w", err)
	}

	return syncing, nil
}

// GetBesuRPCClient creates an RPC client for a Besu node
func (s *NodeService) GetBesuRPCClient(ctx context.Context, nodeID int64) (*RPCClient, error) {
	node, err := s.db.GetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	if types.NodeType(node.NodeType.String) != types.NodeTypeBesuFullnode {
		return nil, errors.NewValidationError("node is not a Besu node", nil)
	}

	nodeConfig, err := utils.LoadNodeConfig([]byte(node.NodeConfig.String))
	if err != nil {
		return nil, fmt.Errorf("failed to load node config: %w", err)
	}

	besuConfig, ok := nodeConfig.(*types.BesuNodeConfig)
	if !ok {
		return nil, fmt.Errorf("invalid besu config type")
	}

	return NewRPCClient(besuConfig.RPCHost, besuConfig.RPCPort), nil
}
