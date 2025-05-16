package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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

	// Get external IP for p2p communication
	externalIP, err := s.GetExternalIP()
	if err != nil {
		return nil, fmt.Errorf("failed to get external IP: %w", err)
	}

	// Use localhost for internal IP
	internalIP := "127.0.0.1"

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
			P2PHost:    externalIP, // Use external IP for p2p host
			P2PPort:    p2pPort,
			RPCHost:    "0.0.0.0", // Allow RPC from any interface
			RPCPort:    rpcPort,
			ExternalIP: externalIP,
			InternalIP: internalIP,
			Mode:       ModeService,
			Env: map[string]string{
				"JAVA_OPTS": "-Xmx4g",
			},
			// Set metrics configuration
			MetricsEnabled:  true,
			MetricsHost:     "0.0.0.0", // Allow metrics from any interface
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
			Version:         "25.4.1", // TODO: get version from network
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
