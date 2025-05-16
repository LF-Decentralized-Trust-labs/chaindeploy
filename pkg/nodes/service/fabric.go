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
)

// GetFabricPeerDefaults returns default values for a Fabric peer node
func (s *NodeService) GetFabricPeerDefaults() *NodeDefaults {
	// Get available ports for peer services
	listen, chaincode, events, operations, err := GetPeerPorts(7051)
	if err != nil {
		// If we can't get the preferred ports, try from a higher range
		listen, chaincode, events, operations, err = GetPeerPorts(10000)
		if err != nil {
			s.logger.Error("Failed to get available ports for peer", "error", err)
			// Fall back to default ports if all attempts fail
			return &NodeDefaults{
				ListenAddress:           "0.0.0.0:7051",
				ExternalEndpoint:        "localhost:7051",
				ChaincodeAddress:        "0.0.0.0:7052",
				EventsAddress:           "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:9443",
				Mode:                    ModeService,
				ServiceName:             "fabric-peer",
				LogPath:                 "/var/log/fabric/peer.log",
				ErrorLogPath:            "/var/log/fabric/peer.err",
			}
		}
	}

	return &NodeDefaults{
		ListenAddress:           fmt.Sprintf("0.0.0.0:%d", listen),
		ExternalEndpoint:        fmt.Sprintf("localhost:%d", listen),
		ChaincodeAddress:        fmt.Sprintf("0.0.0.0:%d", chaincode),
		EventsAddress:           fmt.Sprintf("0.0.0.0:%d", events),
		OperationsListenAddress: fmt.Sprintf("0.0.0.0:%d", operations),
		Mode:                    ModeService,
		ServiceName:             "fabric-peer",
		LogPath:                 "/var/log/fabric/peer.log",
		ErrorLogPath:            "/var/log/fabric/peer.err",
	}
}

// GetFabricOrdererDefaults returns default values for a Fabric orderer node
func (s *NodeService) GetFabricOrdererDefaults() *NodeDefaults {
	// Get available ports for orderer services
	listen, admin, operations, err := GetOrdererPorts(7050)
	if err != nil {
		// If we can't get the preferred ports, try from a higher range
		listen, admin, operations, err = GetOrdererPorts(10000)
		if err != nil {
			s.logger.Error("Failed to get available ports for orderer", "error", err)
			// Fall back to default ports if all attempts fail
			return &NodeDefaults{
				ListenAddress:           "0.0.0.0:7050",
				ExternalEndpoint:        "localhost:7050",
				AdminAddress:            "0.0.0.0:7053",
				OperationsListenAddress: "0.0.0.0:8443",
				Mode:                    ModeService,
				ServiceName:             "fabric-orderer",
				LogPath:                 "/var/log/fabric/orderer.log",
				ErrorLogPath:            "/var/log/fabric/orderer.err",
			}
		}
	}

	return &NodeDefaults{
		ListenAddress:           fmt.Sprintf("0.0.0.0:%d", listen),
		ExternalEndpoint:        fmt.Sprintf("localhost:%d", listen),
		AdminAddress:            fmt.Sprintf("0.0.0.0:%d", admin),
		OperationsListenAddress: fmt.Sprintf("0.0.0.0:%d", operations),
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
			ListenAddress:           fmt.Sprintf("0.0.0.0:%d", listen),
			ExternalEndpoint:        fmt.Sprintf("localhost:%d", listen),
			ChaincodeAddress:        fmt.Sprintf("0.0.0.0:%d", chaincode),
			EventsAddress:           fmt.Sprintf("0.0.0.0:%d", events),
			OperationsListenAddress: fmt.Sprintf("0.0.0.0:%d", operations),
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
			ListenAddress:           fmt.Sprintf("0.0.0.0:%d", listen),
			ExternalEndpoint:        fmt.Sprintf("localhost:%d", listen),
			AdminAddress:            fmt.Sprintf("0.0.0.0:%d", admin),
			OperationsListenAddress: fmt.Sprintf("0.0.0.0:%d", operations),
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

	// Update configuration fields if provided
	if opts.ExternalEndpoint != "" && opts.ExternalEndpoint != peerConfig.ExternalEndpoint {
		peerConfig.ExternalEndpoint = opts.ExternalEndpoint
	}
	if opts.ListenAddress != "" && opts.ListenAddress != peerConfig.ListenAddress {
		if err := s.validateAddress(opts.ListenAddress); err != nil {
			return nil, fmt.Errorf("invalid listen address: %w", err)
		}
		peerConfig.ListenAddress = opts.ListenAddress
	}
	if opts.EventsAddress != "" && opts.EventsAddress != peerConfig.EventsAddress {
		if err := s.validateAddress(opts.EventsAddress); err != nil {
			return nil, fmt.Errorf("invalid events address: %w", err)
		}
		peerConfig.EventsAddress = opts.EventsAddress
	}
	if opts.OperationsListenAddress != "" && opts.OperationsListenAddress != peerConfig.OperationsListenAddress {
		if err := s.validateAddress(opts.OperationsListenAddress); err != nil {
			return nil, fmt.Errorf("invalid operations listen address: %w", err)
		}
		peerConfig.OperationsListenAddress = opts.OperationsListenAddress
	}
	if opts.ChaincodeAddress != "" && opts.ChaincodeAddress != peerConfig.ChaincodeAddress {
		if err := s.validateAddress(opts.ChaincodeAddress); err != nil {
			return nil, fmt.Errorf("invalid chaincode address: %w", err)
		}
		peerConfig.ChaincodeAddress = opts.ChaincodeAddress
	}
	if opts.DomainNames != nil {
		peerConfig.DomainNames = opts.DomainNames
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

	// Validate all addresses together for port conflicts
	if err := s.validateFabricPeerAddresses(peerConfig); err != nil {
		return nil, err
	}

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
	deploymentConfigBytes, err := json.Marshal(deployPeerConfig)
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

	// Synchronize the peer config
	if err := s.SynchronizePeerConfig(ctx, opts.NodeID); err != nil {
		return nil, fmt.Errorf("failed to synchronize peer config: %w", err)
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

	// Update configuration fields if provided
	if opts.ExternalEndpoint != "" && opts.ExternalEndpoint != ordererConfig.ExternalEndpoint {
		ordererConfig.ExternalEndpoint = opts.ExternalEndpoint
	}
	if opts.ListenAddress != "" && opts.ListenAddress != ordererConfig.ListenAddress {
		if err := s.validateAddress(opts.ListenAddress); err != nil {
			return nil, fmt.Errorf("invalid listen address: %w", err)
		}
		ordererConfig.ListenAddress = opts.ListenAddress
	}
	if opts.AdminAddress != "" && opts.AdminAddress != ordererConfig.AdminAddress {
		if err := s.validateAddress(opts.AdminAddress); err != nil {
			return nil, fmt.Errorf("invalid admin address: %w", err)
		}
		ordererConfig.AdminAddress = opts.AdminAddress
	}
	if opts.OperationsListenAddress != "" && opts.OperationsListenAddress != ordererConfig.OperationsListenAddress {
		if err := s.validateAddress(opts.OperationsListenAddress); err != nil {
			return nil, fmt.Errorf("invalid operations listen address: %w", err)
		}
		ordererConfig.OperationsListenAddress = opts.OperationsListenAddress
	}
	if opts.DomainNames != nil {
		ordererConfig.DomainNames = opts.DomainNames
	}
	if opts.Env != nil {
		ordererConfig.Env = opts.Env
	}
	if opts.Version != "" {
		ordererConfig.Version = opts.Version
		deployOrdererConfig.Version = opts.Version
	}

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
