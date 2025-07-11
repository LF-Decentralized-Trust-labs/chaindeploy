package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/errors"
	fabricservice "github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/orderer"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/peer"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/utils"
	"github.com/hyperledger/fabric-admin-sdk/pkg/chaincode"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	"google.golang.org/grpc"
)

// GetFabricPeerDefaults returns default values for a Fabric peer node
func (s *NodeService) GetFabricPeerDefaults() *NodeDefaults {
	// Fetch default IP from settings
	defaultIP := "127.0.0.1"
	if setting, err := s.settingsService.GetSetting(context.Background()); err == nil {
		if setting.Config.DefaultNodeExposeIP != "" {
			defaultIP = setting.Config.DefaultNodeExposeIP
		}
	}
	loopbackAddress := "0.0.0.0"
	// Get available ports for peer services
	listen, chaincode, events, operations, err := GetPeerPorts(7051)
	if err != nil {
		// If we can't get the preferred ports, try from a higher range
		listen, chaincode, events, operations, err = GetPeerPorts(10000)
		if err != nil {
			s.logger.Error("Failed to get available ports for peer", "error", err)
			// Fall back to default ports if all attempts fail
			return &NodeDefaults{
				ListenAddress:           fmt.Sprintf("%s:%d", loopbackAddress, 7051),
				ExternalEndpoint:        fmt.Sprintf("%s:%d", defaultIP, 7051),
				ChaincodeAddress:        fmt.Sprintf("%s:%d", loopbackAddress, 7052),
				EventsAddress:           fmt.Sprintf("%s:%d", loopbackAddress, 7053),
				OperationsListenAddress: fmt.Sprintf("%s:%d", loopbackAddress, 9443),
				Mode:                    ModeService,
				ServiceName:             "fabric-peer",
				LogPath:                 "/var/log/fabric/peer.log",
				ErrorLogPath:            "/var/log/fabric/peer.err",
			}
		}
	}

	return &NodeDefaults{
		ListenAddress:           fmt.Sprintf("%s:%d", loopbackAddress, listen),
		ExternalEndpoint:        fmt.Sprintf("%s:%d", defaultIP, listen),
		ChaincodeAddress:        fmt.Sprintf("%s:%d", loopbackAddress, chaincode),
		EventsAddress:           fmt.Sprintf("%s:%d", loopbackAddress, events),
		OperationsListenAddress: fmt.Sprintf("%s:%d", loopbackAddress, operations),
		Mode:                    ModeService,
		ServiceName:             "fabric-peer",
		LogPath:                 "/var/log/fabric/peer.log",
		ErrorLogPath:            "/var/log/fabric/peer.err",
	}
}

// GetFabricOrdererDefaults returns default values for a Fabric orderer node
func (s *NodeService) GetFabricOrdererDefaults() *NodeDefaults {
	// Fetch default IP from settings
	defaultIP := "127.0.0.1"
	if setting, err := s.settingsService.GetSetting(context.Background()); err == nil {
		if setting.Config.DefaultNodeExposeIP != "" {
			defaultIP = setting.Config.DefaultNodeExposeIP
		}
	}
	loopbackAddress := "0.0.0.0"
	// Get available ports for orderer services
	listen, admin, operations, err := GetOrdererPorts(7050)
	if err != nil {
		// If we can't get the preferred ports, try from a higher range
		listen, admin, operations, err = GetOrdererPorts(10000)
		if err != nil {
			s.logger.Error("Failed to get available ports for orderer", "error", err)
			// Fall back to default ports if all attempts fail
			return &NodeDefaults{
				ListenAddress:           fmt.Sprintf("%s:%d", loopbackAddress, 7050),
				ExternalEndpoint:        fmt.Sprintf("%s:%d", defaultIP, 7050),
				AdminAddress:            fmt.Sprintf("%s:%d", loopbackAddress, 7053),
				OperationsListenAddress: fmt.Sprintf("%s:%d", loopbackAddress, 8443),
				Mode:                    ModeService,
				ServiceName:             "fabric-orderer",
				LogPath:                 "/var/log/fabric/orderer.log",
				ErrorLogPath:            "/var/log/fabric/orderer.err",
			}
		}
	}

	return &NodeDefaults{
		ListenAddress:           fmt.Sprintf("%s:%d", loopbackAddress, listen),
		ExternalEndpoint:        fmt.Sprintf("%s:%d", defaultIP, listen),
		AdminAddress:            fmt.Sprintf("%s:%d", loopbackAddress, admin),
		OperationsListenAddress: fmt.Sprintf("%s:%d", loopbackAddress, operations),
		Mode:                    ModeService,
		ServiceName:             "fabric-orderer",
		LogPath:                 "/var/log/fabric/orderer.log",
		ErrorLogPath:            "/var/log/fabric/orderer.err",
	}
}

// Update the port offsets and base ports to prevent overlap
const (
	// Base ports for peers and orderers with sufficient spacing
	peerBasePort    = 7000 // Starting port for peers
	ordererBasePort = 9000 // Starting port for orderers with 2000 port gap

	// Port offsets to ensure no overlap within node types
	peerPortOffset    = 100 // Each peer gets a 100 port range
	ordererPortOffset = 100 // Each orderer gets a 100 port range

	maxPortAttempts = 100 // Maximum attempts to find available ports
)

// GetFabricNodesDefaults returns default values for multiple nodes with guaranteed non-overlapping ports
func (s *NodeService) GetFabricNodesDefaults(params NodesDefaultsParams) (*NodesDefaultsResult, error) {
	// Fetch default IP from settings
	defaultIP := "127.0.0.1"
	loopbackAddress := "0.0.0.0"
	if setting, err := s.settingsService.GetSetting(context.Background()); err == nil {
		if setting.Config.DefaultNodeExposeIP != "" {
			defaultIP = setting.Config.DefaultNodeExposeIP
		}
	}
	// Validate node counts
	if params.PeerCount > 15 {
		return nil, fmt.Errorf("peer count exceeds maximum supported nodes (15)")
	}
	if params.OrdererCount > 15 {
		return nil, fmt.Errorf("orderer count exceeds maximum supported nodes (15)")
	}

	result := &NodesDefaultsResult{
		Peers:              make([]NodeDefaults, params.PeerCount),
		Orderers:           make([]NodeDefaults, params.OrdererCount),
		AvailableAddresses: []string{"localhost", "0.0.0.0"},
	}

	// Generate peer defaults with incremental ports
	// Each peer needs 4 ports (listen, chaincode, events, operations)
	for i := 0; i < params.PeerCount; i++ {
		basePort := peerBasePort + (i * peerPortOffset)
		listen, chaincode, events, operations, err := GetPeerPorts(basePort)
		if err != nil {
			// Try with a higher range if initial attempt fails
			listen, chaincode, events, operations, err = GetPeerPorts(10000 + (i * peerPortOffset))
			if err != nil {
				return nil, fmt.Errorf("failed to get peer ports: %w", err)
			}
		}

		// Validate that ports don't overlap with orderer range
		if listen >= ordererBasePort || chaincode >= ordererBasePort ||
			events >= ordererBasePort || operations >= ordererBasePort {
			return nil, fmt.Errorf("peer ports would overlap with orderer port range")
		}

		result.Peers[i] = NodeDefaults{
			ListenAddress:           fmt.Sprintf("%s:%d", loopbackAddress, listen),
			ExternalEndpoint:        fmt.Sprintf("%s:%d", defaultIP, listen),
			ChaincodeAddress:        fmt.Sprintf("%s:%d", loopbackAddress, chaincode),
			EventsAddress:           fmt.Sprintf("%s:%d", loopbackAddress, events),
			OperationsListenAddress: fmt.Sprintf("%s:%d", loopbackAddress, operations),
			Mode:                    params.Mode,
			ServiceName:             fmt.Sprintf("fabric-peer-%d", i+1),
			LogPath:                 fmt.Sprintf("/var/log/fabric/peer%d.log", i+1),
			ErrorLogPath:            fmt.Sprintf("/var/log/fabric/peer%d.err", i+1),
		}
	}

	// Generate orderer defaults with incremental ports
	// Each orderer needs 3 ports (listen, admin, operations)
	for i := 0; i < params.OrdererCount; i++ {
		basePort := ordererBasePort + (i * ordererPortOffset)
		listen, admin, operations, err := GetOrdererPorts(basePort)
		if err != nil {
			// Try with a higher range if initial attempt fails
			listen, admin, operations, err = GetOrdererPorts(11000 + (i * ordererPortOffset))
			if err != nil {
				return nil, fmt.Errorf("failed to get orderer ports: %w", err)
			}
		}

		// Validate that ports don't overlap with peer range
		maxPeerPort := peerBasePort + (15 * peerPortOffset) // Account for maximum possible peers
		if listen <= maxPeerPort ||
			admin <= maxPeerPort ||
			operations <= maxPeerPort {
			return nil, fmt.Errorf("orderer ports would overlap with peer port range")
		}

		result.Orderers[i] = NodeDefaults{
			ListenAddress:           fmt.Sprintf("%s:%d", loopbackAddress, listen),
			ExternalEndpoint:        fmt.Sprintf("%s:%d", defaultIP, listen),
			AdminAddress:            fmt.Sprintf("%s:%d", loopbackAddress, admin),
			OperationsListenAddress: fmt.Sprintf("%s:%d", loopbackAddress, operations),
			Mode:                    params.Mode,
			ServiceName:             fmt.Sprintf("fabric-orderer-%d", i+1),
			LogPath:                 fmt.Sprintf("/var/log/fabric/orderer%d.log", i+1),
			ErrorLogPath:            fmt.Sprintf("/var/log/fabric/orderer%d.err", i+1),
		}
	}

	return result, nil
}

// GetFabricPeer gets a Fabric peer node configuration
func (s *NodeService) GetFabricPeer(ctx context.Context, id int64) (*peer.LocalPeer, error) {
	// Get the node from database
	node, err := s.db.GetNode(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("peer node not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get peer node: %w", err)
	}

	// Verify node type
	if types.NodeType(node.NodeType.String) != types.NodeTypeFabricPeer {
		return nil, fmt.Errorf("node %d is not a Fabric peer", id)
	}

	// Load node config
	nodeConfig, err := utils.LoadNodeConfig([]byte(node.NodeConfig.String))
	if err != nil {
		return nil, fmt.Errorf("failed to load peer config: %w", err)
	}

	// Type assert to FabricPeerConfig
	peerConfig, ok := nodeConfig.(*types.FabricPeerConfig)
	if !ok {
		return nil, fmt.Errorf("invalid peer config type")
	}

	// Get deployment config if available
	if node.DeploymentConfig.Valid {
		deploymentConfig, err := utils.DeserializeDeploymentConfig(node.DeploymentConfig.String)
		if err != nil {
			s.logger.Warn("Failed to deserialize deployment config", "error", err)
		} else {
			// Update config with deployment values
			if deployConfig, ok := deploymentConfig.(*types.FabricPeerDeploymentConfig); ok {
				peerConfig.ExternalEndpoint = deployConfig.ExternalEndpoint
				// Add any other deployment-specific fields that should be included
			}
		}
	}

	// Get organization
	org, err := s.orgService.GetOrganization(ctx, peerConfig.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	// Create and return local peer
	localPeer := s.getPeerFromConfig(node, org, peerConfig)
	return localPeer, nil
}

// GetFabricOrderer gets a Fabric orderer node configuration
func (s *NodeService) GetFabricOrderer(ctx context.Context, id int64) (*orderer.LocalOrderer, error) {
	// Get the node from database
	node, err := s.db.GetNode(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("orderer node not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get orderer node: %w", err)
	}

	// Verify node type
	if types.NodeType(node.NodeType.String) != types.NodeTypeFabricOrderer {
		return nil, fmt.Errorf("node %d is not a Fabric orderer", id)
	}

	// Load node config
	nodeConfig, err := utils.LoadNodeConfig([]byte(node.NodeConfig.String))
	if err != nil {
		return nil, fmt.Errorf("failed to load orderer config: %w", err)
	}

	// Type assert to FabricOrdererConfig
	ordererConfig, ok := nodeConfig.(*types.FabricOrdererConfig)
	if !ok {
		return nil, fmt.Errorf("invalid orderer config type")
	}

	// Get deployment config if available
	if node.DeploymentConfig.Valid {
		deploymentConfig, err := utils.DeserializeDeploymentConfig(node.DeploymentConfig.String)
		if err != nil {
			s.logger.Warn("Failed to deserialize deployment config", "error", err)
		} else {
			// Update config with deployment values
			if deployConfig, ok := deploymentConfig.(*types.FabricOrdererDeploymentConfig); ok {
				ordererConfig.ExternalEndpoint = deployConfig.ExternalEndpoint
				// Add any other deployment-specific fields that should be included
			}
		}
	}

	// Get organization
	org, err := s.orgService.GetOrganization(ctx, ordererConfig.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	// Create and return local orderer
	localOrderer := s.getOrdererFromConfig(node, org, ordererConfig)
	return localOrderer, nil
}

// GetFabricNodesByOrganization gets all Fabric nodes (peers and orderers) for an organization
func (s *NodeService) GetFabricNodesByOrganization(ctx context.Context, orgID int64) ([]NodeResponse, error) {
	// Get all nodes
	nodes, err := s.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	// Filter nodes by organization
	var orgNodes []NodeResponse
	for _, node := range nodes.Items {
		// Check node type and config
		switch node.NodeType {
		case types.NodeTypeFabricPeer:
			if node.FabricPeer != nil {
				if node.FabricPeer.OrganizationID == orgID {
					orgNodes = append(orgNodes, node)
				}
			}
		case types.NodeTypeFabricOrderer:
			if node.FabricOrderer != nil {
				if node.FabricOrderer.OrganizationID == orgID {
					orgNodes = append(orgNodes, node)
				}
			}
		}
	}

	return orgNodes, nil
}

// startFabricPeer starts a Fabric peer node
func (s *NodeService) startFabricPeer(ctx context.Context, dbNode *db.Node) error {

	nodeConfig, err := utils.LoadNodeConfig([]byte(dbNode.NodeConfig.String))
	if err != nil {
		return fmt.Errorf("failed to deserialize node config: %w", err)
	}
	peerNodeConfig, ok := nodeConfig.(*types.FabricPeerConfig)
	if !ok {
		return fmt.Errorf("failed to assert node config to FabricPeerConfig")
	}

	deploymentConfig, err := utils.DeserializeDeploymentConfig(dbNode.DeploymentConfig.String)
	if err != nil {
		return fmt.Errorf("failed to deserialize deployment config: %w", err)
	}
	s.logger.Info("Starting fabric peer", "deploymentConfig", deploymentConfig)

	peerConfig := deploymentConfig.ToFabricPeerConfig()

	org, err := s.orgService.GetOrganization(ctx, peerConfig.OrganizationID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	localPeer := s.getPeerFromConfig(dbNode, org, peerNodeConfig)

	_, err = localPeer.Start()
	if err != nil {
		return fmt.Errorf("failed to start peer: %w", err)
	}

	return nil
}

// stopFabricPeer stops a Fabric peer node
func (s *NodeService) stopFabricPeer(ctx context.Context, dbNode *db.Node) error {
	deploymentConfig, err := utils.DeserializeDeploymentConfig(dbNode.NodeConfig.String)
	if err != nil {
		return fmt.Errorf("failed to deserialize deployment config: %w", err)
	}
	nodeConfig, err := utils.LoadNodeConfig([]byte(dbNode.NodeConfig.String))
	if err != nil {
		return fmt.Errorf("failed to deserialize node config: %w", err)
	}
	peerNodeConfig, ok := nodeConfig.(*types.FabricPeerConfig)
	if !ok {
		return fmt.Errorf("failed to assert node config to FabricPeerConfig")
	}
	s.logger.Debug("peerNodeConfig", "peerNodeConfig", peerNodeConfig)
	peerConfig := deploymentConfig.ToFabricPeerConfig()
	s.logger.Debug("peerConfig", "peerConfig", peerConfig)
	org, err := s.orgService.GetOrganization(ctx, peerNodeConfig.OrganizationID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	localPeer := s.getPeerFromConfig(dbNode, org, peerNodeConfig)

	err = localPeer.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop peer: %w", err)
	}

	return nil
}

// startFabricOrderer starts a Fabric orderer node
func (s *NodeService) startFabricOrderer(ctx context.Context, dbNode *db.Node) error {
	nodeConfig, err := utils.LoadNodeConfig([]byte(dbNode.NodeConfig.String))
	if err != nil {
		return fmt.Errorf("failed to deserialize node config: %w", err)
	}
	ordererNodeConfig, ok := nodeConfig.(*types.FabricOrdererConfig)
	if !ok {
		return fmt.Errorf("failed to assert node config to FabricOrdererConfig")
	}

	org, err := s.orgService.GetOrganization(ctx, ordererNodeConfig.OrganizationID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	localOrderer := s.getOrdererFromConfig(dbNode, org, ordererNodeConfig)

	_, err = localOrderer.Start()
	if err != nil {
		return fmt.Errorf("failed to start orderer: %w", err)
	}

	return nil
}

// stopFabricOrderer stops a Fabric orderer node
func (s *NodeService) stopFabricOrderer(ctx context.Context, dbNode *db.Node) error {
	nodeConfig, err := utils.LoadNodeConfig([]byte(dbNode.NodeConfig.String))
	if err != nil {
		return fmt.Errorf("failed to deserialize node config: %w", err)
	}
	ordererNodeConfig, ok := nodeConfig.(*types.FabricOrdererConfig)
	if !ok {
		return fmt.Errorf("failed to assert node config to FabricOrdererConfig")
	}

	org, err := s.orgService.GetOrganization(ctx, ordererNodeConfig.OrganizationID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	localOrderer := s.getOrdererFromConfig(dbNode, org, ordererNodeConfig)

	err = localOrderer.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop orderer: %w", err)
	}

	return nil
}

// UpdateFabricPeer updates a Fabric peer node configuration
func (s *NodeService) UpdateFabricPeer(ctx context.Context, opts UpdateFabricPeerOpts) (*NodeResponse, error) {
	// Get the node from database
	node, err := s.db.GetNode(ctx, opts.NodeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("peer node not found", nil)
		}
		return nil, fmt.Errorf("failed to get peer node: %w", err)
	}

	// Verify node type
	if types.NodeType(node.NodeType.String) != types.NodeTypeFabricPeer {
		return nil, fmt.Errorf("node %d is not a Fabric peer", opts.NodeID)
	}

	// Load current config
	nodeConfig, err := utils.LoadNodeConfig([]byte(node.NodeConfig.String))
	if err != nil {
		return nil, fmt.Errorf("failed to load peer config: %w", err)
	}

	peerConfig, ok := nodeConfig.(*types.FabricPeerConfig)
	if !ok {
		return nil, fmt.Errorf("invalid peer config type")
	}

	deployConfig, err := utils.DeserializeDeploymentConfig(node.DeploymentConfig.String)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize deployment config: %w", err)
	}
	deployPeerConfig, ok := deployConfig.(*types.FabricPeerDeploymentConfig)
	if !ok {
		return nil, fmt.Errorf("invalid deployment config type")
	}

	// --- MODE CHANGE LOGIC ---
	modeChanged := false
	var newMode string
	if opts.Mode != "" {
		if opts.Mode != peerConfig.Mode {
			modeChanged = true
			newMode = opts.Mode
		}
	}
	// If mode is changing, stop the node first
	if modeChanged {
		if err := s.stopFabricPeer(ctx, node); err != nil {
			s.logger.Warn("Failed to stop peer before mode change", "error", err)
		}
		peerConfig.Mode = newMode
		deployPeerConfig.Mode = newMode
	}

	// Update configuration fields if provided
	if opts.ExternalEndpoint != "" && opts.ExternalEndpoint != peerConfig.ExternalEndpoint {
		peerConfig.ExternalEndpoint = opts.ExternalEndpoint
		deployPeerConfig.ExternalEndpoint = opts.ExternalEndpoint
	}
	if opts.ListenAddress != "" && opts.ListenAddress != peerConfig.ListenAddress {
		peerConfig.ListenAddress = opts.ListenAddress
		deployPeerConfig.ListenAddress = opts.ListenAddress
	}
	if opts.EventsAddress != "" && opts.EventsAddress != peerConfig.EventsAddress {
		peerConfig.EventsAddress = opts.EventsAddress
		deployPeerConfig.EventsAddress = opts.EventsAddress
	}
	if opts.OperationsListenAddress != "" && opts.OperationsListenAddress != peerConfig.OperationsListenAddress {
		peerConfig.OperationsListenAddress = opts.OperationsListenAddress
		deployPeerConfig.OperationsListenAddress = opts.OperationsListenAddress
	}
	if opts.ChaincodeAddress != "" && opts.ChaincodeAddress != peerConfig.ChaincodeAddress {
		peerConfig.ChaincodeAddress = opts.ChaincodeAddress
		deployPeerConfig.ChaincodeAddress = opts.ChaincodeAddress
	}
	if opts.DomainNames != nil {
		peerConfig.DomainNames = opts.DomainNames
		deployPeerConfig.DomainNames = opts.DomainNames
	}
	if opts.Env != nil {
		peerConfig.Env = opts.Env
	}
	if opts.AddressOverrides != nil {
		peerConfig.AddressOverrides = opts.AddressOverrides
		deployPeerConfig.AddressOverrides = opts.AddressOverrides
	}
	if opts.Version != "" {
		peerConfig.Version = opts.Version
		deployPeerConfig.Version = opts.Version
	}
	if opts.AddressOverrides != nil {
		peerConfig.AddressOverrides = opts.AddressOverrides
		deployPeerConfig.AddressOverrides = opts.AddressOverrides
	}

	// Validate the updated configuration
	if err := s.validateFabricPeerConfig(peerConfig); err != nil {
		return nil, fmt.Errorf("invalid peer configuration: %w", err)
	}

	// In UpdateFabricPeer, before validation
	peerConfig.DomainNames = s.ensureExternalEndpointInDomains(peerConfig.ExternalEndpoint, peerConfig.DomainNames)

	configBytes, err := utils.StoreNodeConfig(nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to store node config: %w", err)
	}
	_, err = s.db.UpdateNodeConfig(ctx, &db.UpdateNodeConfigParams{
		ID: opts.NodeID,
		NodeConfig: sql.NullString{
			String: string(configBytes),
			Valid:  true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update node config: %w", err)
	}
	// Update the deployment config in the database
	deploymentConfigBytes, err := json.Marshal(deployPeerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated deployment config: %w", err)
	}

	_, err = s.db.UpdateDeploymentConfig(ctx, &db.UpdateDeploymentConfigParams{
		ID: opts.NodeID,
		DeploymentConfig: sql.NullString{
			String: string(deploymentConfigBytes),
			Valid:  true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment config: %w", err)
	}

	// Synchronize the peer config
	if err := s.SynchronizePeerConfig(ctx, opts.NodeID); err != nil {
		return nil, fmt.Errorf("failed to synchronize peer config: %w", err)
	}

	// If mode changed, start the node with the new mode
	if modeChanged {
		if err := s.startFabricPeer(ctx, node); err != nil {
			s.logger.Warn("Failed to start peer after mode change", "error", err)
		}
	}

	// Return updated node response
	_, nodeResponse := s.mapDBNodeToServiceNode(node)
	return nodeResponse, nil
}

// UpdateFabricOrderer updates a Fabric orderer node configuration
func (s *NodeService) UpdateFabricOrderer(ctx context.Context, opts UpdateFabricOrdererOpts) (*NodeResponse, error) {
	// Get the node from database
	node, err := s.db.GetNode(ctx, opts.NodeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("orderer node not found", nil)
		}
		return nil, fmt.Errorf("failed to get orderer node: %w", err)
	}

	// Verify node type
	if types.NodeType(node.NodeType.String) != types.NodeTypeFabricOrderer {
		return nil, fmt.Errorf("node %d is not a Fabric orderer", opts.NodeID)
	}

	// Load current config
	nodeConfig, err := utils.LoadNodeConfig([]byte(node.NodeConfig.String))
	if err != nil {
		return nil, fmt.Errorf("failed to load orderer config: %w", err)
	}

	ordererConfig, ok := nodeConfig.(*types.FabricOrdererConfig)
	if !ok {
		return nil, fmt.Errorf("invalid orderer config type")
	}

	// Load deployment config
	deployOrdererConfig := &types.FabricOrdererDeploymentConfig{}
	if node.DeploymentConfig.Valid {
		deploymentConfig, err := utils.DeserializeDeploymentConfig(node.DeploymentConfig.String)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize deployment config: %w", err)
		}
		var ok bool
		deployOrdererConfig, ok = deploymentConfig.(*types.FabricOrdererDeploymentConfig)
		if !ok {
			return nil, fmt.Errorf("invalid orderer deployment config type")
		}
	}

	// --- MODE CHANGE LOGIC ---
	modeChanged := false
	var newMode string
	if opts.Mode != "" {
		if opts.Mode != ordererConfig.Mode {
			modeChanged = true
			newMode = opts.Mode
		}
	}
	if modeChanged {
		if err := s.stopFabricOrderer(ctx, node); err != nil {
			s.logger.Warn("Failed to stop orderer before mode change", "error", err)
		}
		ordererConfig.Mode = newMode
		deployOrdererConfig.Mode = newMode
	}

	// Update configuration fields if provided
	if opts.ExternalEndpoint != "" && opts.ExternalEndpoint != ordererConfig.ExternalEndpoint {
		ordererConfig.ExternalEndpoint = opts.ExternalEndpoint
		deployOrdererConfig.ExternalEndpoint = opts.ExternalEndpoint
	}
	if opts.ListenAddress != "" && opts.ListenAddress != ordererConfig.ListenAddress {
		ordererConfig.ListenAddress = opts.ListenAddress
		deployOrdererConfig.ListenAddress = opts.ListenAddress
	}
	if opts.AdminAddress != "" && opts.AdminAddress != ordererConfig.AdminAddress {
		ordererConfig.AdminAddress = opts.AdminAddress
		deployOrdererConfig.AdminAddress = opts.AdminAddress
	}
	if opts.OperationsListenAddress != "" && opts.OperationsListenAddress != ordererConfig.OperationsListenAddress {
		ordererConfig.OperationsListenAddress = opts.OperationsListenAddress
		deployOrdererConfig.OperationsListenAddress = opts.OperationsListenAddress
	}
	if opts.DomainNames != nil {
		ordererConfig.DomainNames = opts.DomainNames
		deployOrdererConfig.DomainNames = opts.DomainNames
	}
	if opts.Env != nil {
		ordererConfig.Env = opts.Env
	}
	if opts.Version != "" {
		ordererConfig.Version = opts.Version
		deployOrdererConfig.Version = opts.Version
	}

	// Validate the updated configuration
	if err := s.validateFabricOrdererConfig(ordererConfig); err != nil {
		return nil, fmt.Errorf("invalid orderer configuration: %w", err)
	}

	// In UpdateFabricOrderer, before validation
	ordererConfig.DomainNames = s.ensureExternalEndpointInDomains(ordererConfig.ExternalEndpoint, ordererConfig.DomainNames)

	configBytes, err := utils.StoreNodeConfig(nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to store node config: %w", err)
	}
	node, err = s.db.UpdateNodeConfig(ctx, &db.UpdateNodeConfigParams{
		ID: opts.NodeID,
		NodeConfig: sql.NullString{
			String: string(configBytes),
			Valid:  true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update node config: %w", err)
	}

	// Update the deployment config in the database
	deploymentConfigBytes, err := json.Marshal(deployOrdererConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated deployment config: %w", err)
	}

	node, err = s.db.UpdateDeploymentConfig(ctx, &db.UpdateDeploymentConfigParams{
		ID: opts.NodeID,
		DeploymentConfig: sql.NullString{
			String: string(deploymentConfigBytes),
			Valid:  true,
		},
	})

	// If mode changed, start the node with the new mode
	if modeChanged {
		if err := s.startFabricOrderer(ctx, node); err != nil {
			s.logger.Warn("Failed to start orderer after mode change", "error", err)
		}
	}

	// Return updated node response
	_, nodeResponse := s.mapDBNodeToServiceNode(node)
	return nodeResponse, nil
}

// SynchronizePeerConfig synchronizes the peer's configuration files and service
func (s *NodeService) SynchronizePeerConfig(ctx context.Context, nodeID int64) error {
	// Get the node from database
	node, err := s.db.GetNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Verify node type
	if types.NodeType(node.NodeType.String) != types.NodeTypeFabricPeer {
		return fmt.Errorf("node %d is not a Fabric peer", nodeID)
	}

	// Load node config
	nodeConfig, err := utils.LoadNodeConfig([]byte(node.NodeConfig.String))
	if err != nil {
		return fmt.Errorf("failed to load node config: %w", err)
	}

	peerConfig, ok := nodeConfig.(*types.FabricPeerConfig)
	if !ok {
		return fmt.Errorf("invalid peer config type")
	}

	// Get organization
	org, err := s.orgService.GetOrganization(ctx, peerConfig.OrganizationID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	// Get local peer instance
	localPeer := s.getPeerFromConfig(node, org, peerConfig)

	// Get deployment config
	deploymentConfig, err := utils.DeserializeDeploymentConfig(node.DeploymentConfig.String)
	if err != nil {
		return fmt.Errorf("failed to deserialize deployment config: %w", err)
	}

	peerDeployConfig, ok := deploymentConfig.(*types.FabricPeerDeploymentConfig)
	if !ok {
		return fmt.Errorf("invalid peer deployment config type")
	}

	// Synchronize configuration
	if err := localPeer.SynchronizeConfig(peerDeployConfig); err != nil {
		return fmt.Errorf("failed to synchronize peer config: %w", err)
	}

	return nil
}

// validateFabricPeerAddresses validates all addresses used by a Fabric peer
func (s *NodeService) validateFabricPeerAddresses(config *types.FabricPeerConfig) error {
	// Get current addresses to compare against
	currentAddresses := map[string]string{
		"listen":     config.ListenAddress,
		"chaincode":  config.ChaincodeAddress,
		"events":     config.EventsAddress,
		"operations": config.OperationsListenAddress,
	}

	// Check for port conflicts between addresses
	usedPorts := make(map[string]string)
	for addrType, addr := range currentAddresses {
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			return fmt.Errorf("invalid %s address format: %w", addrType, err)
		}

		if existingType, exists := usedPorts[port]; exists {
			// If the port is already used by the same address type, it's okay
			if existingType == addrType {
				continue
			}
			return fmt.Errorf("port conflict: %s and %s addresses use the same port %s", existingType, addrType, port)
		}
		usedPorts[port] = addrType

		// Only validate port availability if it's not already in use by this peer
		if err := s.validateAddress(addr); err != nil {
			// Check if the error is due to the port being in use by this peer
			if strings.Contains(err.Error(), "address already in use") {
				continue
			}
			return fmt.Errorf("invalid %s address: %w", addrType, err)
		}
	}

	return nil
}

// validateFabricOrdererAddresses validates all addresses used by a Fabric orderer
func (s *NodeService) validateFabricOrdererAddresses(config *types.FabricOrdererConfig) error {
	// Validate listen address
	if err := s.validateAddress(config.ListenAddress); err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}

	// Validate admin address
	if err := s.validateAddress(config.AdminAddress); err != nil {
		return fmt.Errorf("invalid admin address: %w", err)
	}

	// Validate operations listen address
	if err := s.validateAddress(config.OperationsListenAddress); err != nil {
		return fmt.Errorf("invalid operations listen address: %w", err)
	}

	// Check for port conflicts between addresses
	addresses := map[string]string{
		"listen":     config.ListenAddress,
		"admin":      config.AdminAddress,
		"operations": config.OperationsListenAddress,
	}

	usedPorts := make(map[string]string)
	for addrType, addr := range addresses {
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			return fmt.Errorf("invalid %s address format: %w", addrType, err)
		}

		if existingType, exists := usedPorts[port]; exists {
			return fmt.Errorf("port conflict: %s and %s addresses use the same port %s", existingType, addrType, port)
		}
		usedPorts[port] = addrType
	}

	return nil
}

// initializeFabricPeer initializes a Fabric peer node
func (s *NodeService) initializeFabricPeer(ctx context.Context, dbNode *db.Node, req *types.FabricPeerConfig) (types.NodeDeploymentConfig, error) {
	org, err := s.orgService.GetOrganization(ctx, req.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	localPeer := s.getPeerFromConfig(dbNode, org, req)

	// Get deployment config from initialization
	peerConfig, err := localPeer.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize peer: %w", err)
	}

	return peerConfig, nil
}

// getOrdererFromConfig creates a LocalOrderer instance from configuration
func (s *NodeService) getOrdererFromConfig(dbNode *db.Node, org *fabricservice.OrganizationDTO, config *types.FabricOrdererConfig) *orderer.LocalOrderer {
	return orderer.NewLocalOrderer(
		org.MspID,
		s.db,
		orderer.StartOrdererOpts{
			ID:                      dbNode.Name,
			ListenAddress:           config.ListenAddress,
			OperationsListenAddress: config.OperationsListenAddress,
			AdminListenAddress:      config.AdminAddress,
			ExternalEndpoint:        config.ExternalEndpoint,
			DomainNames:             config.DomainNames,
			Env:                     config.Env,
			Version:                 config.Version,
			AddressOverrides:        config.AddressOverrides,
		},
		config.Mode,
		org,
		config.OrganizationID,
		s.orgService,
		s.keymanagementService,
		dbNode.ID,
		s.logger,
		s.configService,
		s.settingsService,
	)
}

// initializeFabricOrderer initializes a Fabric orderer node
func (s *NodeService) initializeFabricOrderer(ctx context.Context, dbNode *db.Node, req *types.FabricOrdererConfig) (*types.FabricOrdererDeploymentConfig, error) {
	org, err := s.orgService.GetOrganization(ctx, req.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	localOrderer := s.getOrdererFromConfig(dbNode, org, req)

	// Get deployment config from initialization
	config, err := localOrderer.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize orderer: %w", err)
	}

	// Type assert the config
	ordererConfig, ok := config.(*types.FabricOrdererDeploymentConfig)
	if !ok {
		return nil, fmt.Errorf("invalid orderer config type")
	}

	return ordererConfig, nil
}

// getPeerFromConfig creates a peer instance from the given configuration and database node
func (s *NodeService) getPeerFromConfig(dbNode *db.Node, org *fabricservice.OrganizationDTO, config *types.FabricPeerConfig) *peer.LocalPeer {
	return peer.NewLocalPeer(
		org.MspID,
		s.db,
		peer.StartPeerOpts{
			ID:                      dbNode.Slug,
			ListenAddress:           config.ListenAddress,
			ChaincodeAddress:        config.ChaincodeAddress,
			EventsAddress:           config.EventsAddress,
			OperationsListenAddress: config.OperationsListenAddress,
			ExternalEndpoint:        config.ExternalEndpoint,
			DomainNames:             config.DomainNames,
			Env:                     config.Env,
			Version:                 config.Version,
			AddressOverrides:        config.AddressOverrides,
		},
		config.Mode,
		org,
		org.ID,
		s.orgService,
		s.keymanagementService,
		dbNode.ID,
		s.logger,
		s.configService,
		s.settingsService,
	)
}

// renewPeerCertificates handles certificate renewal for a Fabric peer
func (s *NodeService) renewPeerCertificates(ctx context.Context, dbNode *db.Node, deploymentConfig types.NodeDeploymentConfig) error {
	// Create certificate renewal starting event
	if err := s.eventService.CreateEvent(ctx, dbNode.ID, NodeEventRenewingCertificates, map[string]interface{}{
		"node_id": dbNode.ID,
		"name":    dbNode.Name,
		"action":  "renewing_certificates",
		"type":    "peer",
	}); err != nil {
		s.logger.Error("Failed to create certificate renewal starting event", "error", err)
	}

	nodeConfig, err := utils.LoadNodeConfig([]byte(dbNode.NodeConfig.String))
	if err != nil {
		return fmt.Errorf("failed to load node config: %w", err)
	}

	peerConfig, ok := nodeConfig.(*types.FabricPeerConfig)
	if !ok {
		return fmt.Errorf("invalid peer config type")
	}

	peerDeployConfig, ok := deploymentConfig.(*types.FabricPeerDeploymentConfig)
	if !ok {
		return fmt.Errorf("invalid peer deployment config type")
	}

	org, err := s.orgService.GetOrganization(ctx, peerConfig.OrganizationID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	localPeer := s.getPeerFromConfig(dbNode, org, peerConfig)
	err = localPeer.RenewCertificates(peerDeployConfig)
	if err != nil {
		return fmt.Errorf("failed to renew peer certificates: %w", err)
	}

	// Create certificate renewal completed event
	if err := s.eventService.CreateEvent(ctx, dbNode.ID, NodeEventRenewedCertificates, map[string]interface{}{
		"node_id": dbNode.ID,
		"name":    dbNode.Name,
		"action":  "renewing_certificates",
		"type":    "peer",
	}); err != nil {
		s.logger.Error("Failed to create certificate renewal completed event", "error", err)
	}

	return nil
}

// renewOrdererCertificates handles certificate renewal for a Fabric orderer
func (s *NodeService) renewOrdererCertificates(ctx context.Context, dbNode *db.Node, deploymentConfig types.NodeDeploymentConfig) error {
	// Create certificate renewal starting event
	if err := s.eventService.CreateEvent(ctx, dbNode.ID, NodeEventRenewingCertificates, map[string]interface{}{
		"node_id": dbNode.ID,
		"name":    dbNode.Name,
		"action":  "renewing_certificates",
		"type":    "orderer",
	}); err != nil {
		s.logger.Error("Failed to create certificate renewal starting event", "error", err)
	}

	nodeConfig, err := utils.LoadNodeConfig([]byte(dbNode.NodeConfig.String))
	if err != nil {
		return fmt.Errorf("failed to load node config: %w", err)
	}

	ordererConfig, ok := nodeConfig.(*types.FabricOrdererConfig)
	if !ok {
		return fmt.Errorf("invalid orderer config type")
	}

	ordererDeployConfig, ok := deploymentConfig.(*types.FabricOrdererDeploymentConfig)
	if !ok {
		return fmt.Errorf("invalid orderer deployment config type")
	}

	org, err := s.orgService.GetOrganization(ctx, ordererConfig.OrganizationID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	localOrderer := s.getOrdererFromConfig(dbNode, org, ordererConfig)
	err = localOrderer.RenewCertificates(ordererDeployConfig)
	if err != nil {
		return fmt.Errorf("failed to renew orderer certificates: %w", err)
	}

	// Create certificate renewal completed event
	if err := s.eventService.CreateEvent(ctx, dbNode.ID, NodeEventRenewedCertificates, map[string]interface{}{
		"node_id": dbNode.ID,
		"name":    dbNode.Name,
		"action":  "renewing_certificates",
		"type":    "orderer",
	}); err != nil {
		s.logger.Error("Failed to create certificate renewal completed event", "error", err)
	}

	return nil
}

// cleanupPeerResources cleans up resources specific to a Fabric peer node
func (s *NodeService) cleanupPeerResources(ctx context.Context, node *db.Node) error {
	// Clean up peer-specific directories
	dirsToClean := []string{
		filepath.Join(s.configService.GetDataPath(), "nodes", node.Slug),
		filepath.Join(s.configService.GetDataPath(), "peers", node.Slug),
		filepath.Join(s.configService.GetDataPath(), "fabric", "peers", node.Slug),
	}

	for _, dir := range dirsToClean {
		if err := os.RemoveAll(dir); err != nil {
			if !os.IsNotExist(err) {
				s.logger.Warn("Failed to remove peer directory",
					"path", dir,
					"error", err)
			}
		} else {
			s.logger.Info("Successfully removed peer directory",
				"path", dir)
		}
	}

	return nil
}

// cleanupOrdererResources cleans up resources specific to a Fabric orderer node
func (s *NodeService) cleanupOrdererResources(ctx context.Context, node *db.Node) error {

	// Clean up orderer-specific directories
	dirsToClean := []string{
		filepath.Join(s.configService.GetDataPath(), "nodes", node.Slug),
		filepath.Join(s.configService.GetDataPath(), "orderers", node.Slug),
		filepath.Join(s.configService.GetDataPath(), "fabric", "orderers", node.Slug),
	}

	for _, dir := range dirsToClean {
		if err := os.RemoveAll(dir); err != nil {
			if !os.IsNotExist(err) {
				s.logger.Warn("Failed to remove orderer directory",
					"path", dir,
					"error", err)
			}
		} else {
			s.logger.Info("Successfully removed orderer directory",
				"path", dir)
		}
	}

	return nil
}

// GetPeerGateway returns a chaincode.Gateway for a peer
func (s *NodeService) GetFabricPeerGateway(ctx context.Context, peerID int64) (*chaincode.Gateway, *grpc.ClientConn, error) {
	// Get the peer node from database
	node, err := s.db.GetNode(ctx, peerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, errors.NewNotFoundError("peer not found", map[string]interface{}{
				"id": peerID,
			})
		}
		return nil, nil, fmt.Errorf("failed to get peer: %w", err)
	}

	// Validate node is a Fabric peer
	if node.Platform != string(types.PlatformFabric) || node.NodeType.String != string(types.NodeTypeFabricPeer) {
		return nil, nil, errors.NewValidationError("invalid node type", map[string]interface{}{
			"detail": fmt.Sprintf("Node must be a Fabric peer, got %s", node.NodeType.String),
			"code":   "INVALID_NODE_TYPE",
		})
	}

	localPeer, err := s.GetFabricPeer(ctx, node.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get peer: %w", err)
	}

	gateway, peerConn, err := localPeer.GetGateway(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get gateway: %w", err)
	}

	return gateway, peerConn, nil
}

func (s *NodeService) GetFabricClientIdentityForOrganization(ctx context.Context, orgID int64) (int64, error) {
	org, err := s.orgService.GetOrganization(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("failed to get organization: %w", err)
	}

	return org.ClientSignKeyID.Int64, nil
}

// GetPeerGateway returns a chaincode.Gateway for a peer
func (s *NodeService) GetFabricPeerClientGateway(ctx context.Context, peerID int64, keyID int64) (*client.Gateway, *grpc.ClientConn, error) {
	// Get the peer node from database
	node, err := s.db.GetNode(ctx, peerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, errors.NewNotFoundError("peer not found", map[string]interface{}{
				"id": peerID,
			})
		}
		return nil, nil, fmt.Errorf("failed to get peer: %w", err)
	}

	// Validate node is a Fabric peer
	if node.Platform != string(types.PlatformFabric) || node.NodeType.String != string(types.NodeTypeFabricPeer) {
		return nil, nil, errors.NewValidationError("invalid node type", map[string]interface{}{
			"detail": fmt.Sprintf("Node must be a Fabric peer, got %s", node.NodeType.String),
			"code":   "INVALID_NODE_TYPE",
		})
	}

	localPeer, err := s.GetFabricPeer(ctx, node.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get peer: %w", err)
	}

	gateway, peerConn, err := localPeer.GetGatewayClient(ctx, keyID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get gateway: %w", err)
	}

	return gateway, peerConn, nil
}

// GetPeerGateway returns a chaincode.Gateway for a peer
func (s *NodeService) GetFabricPeerService(ctx context.Context, peerID int64) (*chaincode.Peer, *grpc.ClientConn, error) {
	// Get the peer node from database
	node, err := s.db.GetNode(ctx, peerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, errors.NewNotFoundError("peer not found", map[string]interface{}{
				"id": peerID,
			})
		}
		return nil, nil, fmt.Errorf("failed to get peer: %w", err)
	}

	// Validate node is a Fabric peer
	if node.Platform != string(types.PlatformFabric) || node.NodeType.String != string(types.NodeTypeFabricPeer) {
		return nil, nil, errors.NewValidationError("invalid node type", map[string]interface{}{
			"detail": fmt.Sprintf("Node must be a Fabric peer, got %s", node.NodeType.String),
			"code":   "INVALID_NODE_TYPE",
		})
	}

	localPeer, err := s.GetFabricPeer(ctx, node.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get peer: %w", err)
	}

	gateway, peerConn, err := localPeer.GetPeerClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get gateway: %w", err)
	}

	return gateway, peerConn, nil
}

// ensureExternalEndpointInDomains ensures that the external endpoint is included in the domain names
func (s *NodeService) ensureExternalEndpointInDomains(externalEndpoint string, domainNames []string) []string {
	// Split host:port to get just the host part
	host, _, err := net.SplitHostPort(externalEndpoint)
	if err != nil {
		// If SplitHostPort fails, assume the entire string is the host
		host = externalEndpoint
	}

	for _, domain := range domainNames {
		if domain == host {
			return domainNames
		}
	}
	return append(domainNames, host)
}
