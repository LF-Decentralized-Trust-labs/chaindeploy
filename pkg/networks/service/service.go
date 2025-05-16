package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	orgservicefabric "github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	keymanagement "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	"github.com/google/uuid"
)

// BlockchainType represents the type of blockchain network
type BlockchainType string

const (
	BlockchainTypeFabric BlockchainType = "fabric"
	BlockchainTypeBesu   BlockchainType = "besu"
	// Add other blockchain types as needed
)

// NetworkStatus represents the status of a network
type NetworkStatus string

const (
	NetworkStatusCreating            NetworkStatus = "creating"
	NetworkStatusGenesisBlockCreated NetworkStatus = "genesis_block_created"
	NetworkStatusRunning             NetworkStatus = "running"
	NetworkStatusStopped             NetworkStatus = "stopped"
	NetworkStatusError               NetworkStatus = "error"
)

// Network represents a blockchain network
type Network struct {
	ID                 int64           `json:"id"`
	Name               string          `json:"name"`
	Platform           string          `json:"platform"`
	Status             NetworkStatus   `json:"status"`
	Description        string          `json:"description"`
	Config             json.RawMessage `json:"config,omitempty"`
	DeploymentConfig   json.RawMessage `json:"deploymentConfig,omitempty"`
	ExposedPorts       json.RawMessage `json:"exposedPorts,omitempty"`
	GenesisBlock       string          `json:"genesisBlock,omitempty"`
	CurrentConfigBlock string          `json:"currentConfigBlock,omitempty"`
	Domain             string          `json:"domain,omitempty"`
	CreatedAt          time.Time       `json:"createdAt"`
	CreatedBy          *int64          `json:"createdBy,omitempty"`
	UpdatedAt          *time.Time      `json:"updatedAt,omitempty"`
}

// ListNetworksParams represents the parameters for listing networks
type ListNetworksParams struct {
	Limit    int32
	Offset   int32
	Platform BlockchainType
}

// ListNetworksResult represents the result of listing networks
type ListNetworksResult struct {
	Networks []Network
	Total    int64
}

// ConfigUpdateOperationRequest represents a configuration update operation
type ConfigUpdateOperationRequest struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Proposal represents a configuration update proposal
type Proposal struct {
	ID                string                         `json:"id"`
	NetworkID         int64                          `json:"network_id"`
	ChannelName       string                         `json:"channel_name"`
	Status            string                         `json:"status"`
	CreatedAt         time.Time                      `json:"created_at"`
	CreatedBy         string                         `json:"created_by"`
	Operations        []ConfigUpdateOperationRequest `json:"operations"`
	PreviewJSON       string                         `json:"preview_json,omitempty"`
	ConfigUpdateBytes []byte                         `json:"config_update_bytes,omitempty"`
}

// ProposalSignature represents a signature on a proposal
type ProposalSignature struct {
	ID       int64     `json:"id"`
	MSPID    string    `json:"msp_id"`
	SignedBy string    `json:"signed_by"`
	SignedAt time.Time `json:"signed_at"`
}

// FabricNetworkService handles network operations
type NetworkService struct {
	db              *db.Queries
	deployerFactory *DeployerFactory
	nodeService     *nodeservice.NodeService
	keyMgmt         *keymanagement.KeyManagementService
	logger          *logger.Logger
	orgService      *orgservicefabric.OrganizationService
}

// NewNetworkService creates a new NetworkService
func NewNetworkService(db *db.Queries, nodes *nodeservice.NodeService, keyMgmt *keymanagement.KeyManagementService, logger *logger.Logger, orgService *orgservicefabric.OrganizationService) *NetworkService {
	return &NetworkService{
		db:              db,
		deployerFactory: NewDeployerFactory(db, nodes, keyMgmt, orgService),
		nodeService:     nodes,
		keyMgmt:         keyMgmt,
		logger:          logger,
		orgService:      orgService,
	}
}

// GetNetworkByName retrieves a network by its name
func (s *NetworkService) GetNetworkByName(ctx context.Context, name string) (*Network, error) {
	network, err := s.db.GetNetworkByName(ctx, name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("network with name %s not found", name)
		}
		return nil, fmt.Errorf("failed to get network by name: %w", err)
	}

	return s.mapDBNetworkToServiceNetwork(network), nil
}

// CreateNetwork creates a new blockchain network
func (s *NetworkService) CreateNetwork(ctx context.Context, name, description string, configData []byte) (*Network, error) {
	// Parse base config to determine type
	var baseConfig types.BaseNetworkConfig
	if err := json.Unmarshal(configData, &baseConfig); err != nil {
		return nil, fmt.Errorf("failed to parse base config: %w", err)
	}

	var config types.NetworkConfig
	switch baseConfig.Type {
	case types.NetworkTypeFabric:
		var fabricConfig types.FabricNetworkConfig
		if err := json.Unmarshal(configData, &fabricConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Fabric config: %w", err)
		}
		config = &fabricConfig

	case types.NetworkTypeBesu:
		var besuConfig types.BesuNetworkConfig
		if err := json.Unmarshal(configData, &besuConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Besu config: %w", err)
		}
		config = &besuConfig

	default:
		return nil, fmt.Errorf("unsupported network type: %s", baseConfig.Type)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid network configuration: %w", err)
	}

	// Validate external nodes exist and are of correct type

	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Generate a random network ID
	networkID := fmt.Sprintf("net_%s_%s", name, uuid.New().String())
	// Create network in database
	network, err := s.db.CreateNetwork(ctx, &db.CreateNetworkParams{
		Name:        name,
		Platform:    string(baseConfig.Type),
		Description: sql.NullString{String: description, Valid: description != ""},
		Config:      sql.NullString{String: string(configJSON), Valid: true},
		Status:      string(NetworkStatusCreating),
		NetworkID:   sql.NullString{String: networkID, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %w", err)
	}

	deployer, err := s.deployerFactory.GetDeployer(string(baseConfig.Type))
	if err != nil {
		return nil, fmt.Errorf("failed to get deployer: %w", err)
	}

	genesisBlock, err := deployer.CreateGenesisBlock(network.ID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create genesis block: %w", err)
	}
	genesisBlockB64 := base64.StdEncoding.EncodeToString(genesisBlock)
	network.GenesisBlockB64 = sql.NullString{String: genesisBlockB64, Valid: true}

	// Update network status to running after successful genesis block creation
	if err := s.UpdateNetworkStatus(ctx, network.ID, NetworkStatusGenesisBlockCreated); err != nil {
		return nil, fmt.Errorf("failed to update network status: %w", err)
	}

	return s.mapDBNetworkToServiceNetwork(network), nil
}

// ListNetworks retrieves a list of networks with pagination
func (s *NetworkService) ListNetworks(ctx context.Context, params ListNetworksParams) (*ListNetworksResult, error) {
	networks, err := s.db.ListNetworks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	result := &ListNetworksResult{
		Networks: make([]Network, len(networks)),
		Total:    int64(len(networks)), // TODO: Implement proper counting
	}

	for i, n := range networks {
		result.Networks[i] = *s.mapDBNetworkToServiceNetwork(n)
	}

	return result, nil
}

// GetNetwork retrieves a network by ID
func (s *NetworkService) GetNetwork(ctx context.Context, networkID int64) (*Network, error) {
	network, err := s.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	return s.mapDBNetworkToServiceNetwork(network), nil
}

// DeleteNetwork deletes a network and all associated resources
func (s *NetworkService) DeleteNetwork(ctx context.Context, networkID int64) error {

	// Delete network record
	if err := s.db.DeleteNetwork(ctx, networkID); err != nil {
		return fmt.Errorf("failed to delete network record: %w", err)
	}

	return nil
}

// Helper function to map db.Network to service.Network
func (s *NetworkService) mapDBNetworkToServiceNetwork(n *db.Network) *Network {
	var config, deploymentConfig, exposedPorts json.RawMessage
	if n.Config.Valid {
		config = json.RawMessage(n.Config.String)
	}
	if n.DeploymentConfig.Valid {
		deploymentConfig = json.RawMessage(n.DeploymentConfig.String)
	}
	if n.ExposedPorts.Valid {
		exposedPorts = json.RawMessage(n.ExposedPorts.String)
	}

	network := &Network{
		ID:               n.ID,
		Name:             n.Name,
		Platform:         n.Platform,
		Status:           NetworkStatus(n.Status),
		Config:           config,
		DeploymentConfig: deploymentConfig,
		ExposedPorts:     exposedPorts,
		CreatedAt:        n.CreatedAt,
		CreatedBy:        nil,
	}

	if n.Description.Valid {
		network.Description = n.Description.String
	}
	if n.Domain.Valid {
		network.Domain = n.Domain.String
	}
	if n.CreatedBy.Valid {
		network.CreatedBy = &n.CreatedBy.Int64
	}
	if n.UpdatedAt.Valid {
		updatedAt := n.UpdatedAt.Time
		network.UpdatedAt = &updatedAt
	}
	if n.GenesisBlockB64.Valid {
		network.GenesisBlock = n.GenesisBlockB64.String
	}
	if n.CurrentConfigBlockB64.Valid {
		network.CurrentConfigBlock = n.CurrentConfigBlockB64.String
	}

	return network
}

// UpdateNetworkStatus updates the status of a network
func (s *NetworkService) UpdateNetworkStatus(ctx context.Context, networkID int64, status NetworkStatus) error {
	err := s.db.UpdateNetworkStatus(ctx, &db.UpdateNetworkStatusParams{
		ID:     networkID,
		Status: string(status),
	})
	if err != nil {
		return fmt.Errorf("failed to update network status: %w", err)
	}
	return nil
}

// GetNetworkNodes retrieves all nodes associated with a network
func (s *NetworkService) GetNetworkNodes(ctx context.Context, networkID int64) ([]NetworkNode, error) {
	// Get network nodes from database
	dbNodes, err := s.db.GetNetworkNodes(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network nodes: %w", err)
	}

	nodes := make([]NetworkNode, len(dbNodes))
	for i, dbNode := range dbNodes {
		node, err := s.nodeService.GetNode(ctx, dbNode.NodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to get node: %w", err)
		}

		nodes[i] = NetworkNode{
			ID:        dbNode.ID,
			NetworkID: dbNode.NetworkID,
			NodeID:    dbNode.NodeID,
			Status:    dbNode.Status,
			Role:      dbNode.Role,
			CreatedAt: dbNode.CreatedAt,
			UpdatedAt: dbNode.UpdatedAt,
			Node:      node,
		}
	}

	return nodes, nil
}

// NetworkNode represents a node in a network with its full details
type NetworkNode struct {
	ID        int64                     `json:"id"`
	NetworkID int64                     `json:"networkId"`
	NodeID    int64                     `json:"nodeId"`
	Status    string                    `json:"status"`
	Role      string                    `json:"role"`
	CreatedAt time.Time                 `json:"createdAt"`
	UpdatedAt time.Time                 `json:"updatedAt"`
	Node      *nodeservice.NodeResponse `json:"node"`
}

// AddNodeToNetwork adds a node to the network with the specified role
func (s *NetworkService) AddNodeToNetwork(ctx context.Context, networkID, nodeID int64, role string) error {
	// Get the network
	network, err := s.db.GetNetwork(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}

	// Get the node
	node, err := s.nodeService.GetNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Validate node type matches role
	switch role {
	case "peer":
		if node.NodeType != nodetypes.NodeTypeFabricPeer {
			return fmt.Errorf("node %d is not a peer", nodeID)
		}
	case "orderer":
		if node.NodeType != nodetypes.NodeTypeFabricOrderer {
			return fmt.Errorf("node %d is not an orderer", nodeID)
		}
	default:
		return fmt.Errorf("invalid role: %s", role)
	}

	// Check if node is already in network
	exists, err := s.db.CheckNetworkNodeExists(ctx, &db.CheckNetworkNodeExistsParams{
		NetworkID: networkID,
		NodeID:    nodeID,
	})
	if err != nil {
		return fmt.Errorf("failed to check if node exists in network: %w", err)
	}
	if exists == 1 {
		return fmt.Errorf("node %d is already in network %d", nodeID, networkID)
	}

	// Create network node entry
	_, err = s.db.CreateNetworkNode(ctx, &db.CreateNetworkNodeParams{
		NetworkID: networkID,
		NodeID:    nodeID,
		Status:    "pending",
		Role:      role,
	})
	if err != nil {
		return fmt.Errorf("failed to create network node: %w", err)
	}

	// Get genesis block
	if !network.GenesisBlockB64.Valid {
		return fmt.Errorf("network %d has no genesis block", networkID)
	}

	return nil
}

// GetGenesisBlock retrieves the genesis block for a network
func (s *NetworkService) GetGenesisBlock(ctx context.Context, networkID int64) ([]byte, error) {
	network, err := s.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}
	genesisBlockB64 := network.GenesisBlockB64.String
	if genesisBlockB64 == "" {
		return nil, fmt.Errorf("no genesis block found for network")
	}
	genesisBlock, err := base64.StdEncoding.DecodeString(genesisBlockB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode genesis block: %w", err)
	}

	return genesisBlock, nil
}

func (s *NetworkService) ImportNetwork(ctx context.Context, params ImportNetworkParams) (*ImportNetworkResult, error) {
	switch params.NetworkType {
	case "fabric":
		return s.importFabricNetwork(ctx, params)
	case "besu":
		return s.importBesuNetwork(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported network type: %s", params.NetworkType)
	}
}
