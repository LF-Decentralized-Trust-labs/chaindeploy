package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/config"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/errors"
	fabricservice "github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	keymanagement "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	metricscommon "github.com/chainlaunch/chainlaunch/pkg/metrics/common"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/utils"
	settingsservice "github.com/chainlaunch/chainlaunch/pkg/settings/service"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer/lifecycle"
)

// NodeService handles business logic for node management
type NodeService struct {
	db                   *db.Queries
	logger               *logger.Logger
	keymanagementService *keymanagement.KeyManagementService
	orgService           *fabricservice.OrganizationService
	eventService         *NodeEventService
	configService        *config.ConfigService
	settingsService      *settingsservice.SettingsService
	metricsService       metricscommon.Service
}

// CreateNodeRequest represents the service-layer request to create a node
type CreateNodeRequest struct {
	Name               string
	DeploymentMode     types.DeploymentMode
	BlockchainPlatform types.BlockchainPlatform
	FabricPeer         *types.FabricPeerConfig
	FabricOrderer      *types.FabricOrdererConfig
	BesuNode           *types.BesuNodeConfig
}

// NewNodeService creates a new NodeService instance
func NewNodeService(
	db *db.Queries,
	logger *logger.Logger,
	keymanagementService *keymanagement.KeyManagementService,
	orgService *fabricservice.OrganizationService,
	eventService *NodeEventService,
	configService *config.ConfigService,
	settingsService *settingsservice.SettingsService,
) *NodeService {
	return &NodeService{
		db:                   db,
		logger:               logger,
		keymanagementService: keymanagementService,
		orgService:           orgService,
		eventService:         eventService,
		configService:        configService,
		settingsService:      settingsService,
	}
}

func (s *NodeService) SetMetricsService(metricsService metricscommon.Service) {
	s.metricsService = metricsService
}

func (s *NodeService) validateCreateNodeRequest(req CreateNodeRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}

	switch req.BlockchainPlatform {
	case types.PlatformFabric:
		if req.FabricPeer == nil && req.FabricOrderer == nil {
			return fmt.Errorf("fabric configuration is required")
		}
		if req.FabricPeer != nil && req.FabricOrderer != nil {
			return fmt.Errorf("cannot specify both peer and orderer configurations")
		}

		// Validate Fabric peer configuration
		if req.FabricPeer != nil {
			req.FabricPeer.DomainNames = s.ensureExternalEndpointInDomains(req.FabricPeer.ExternalEndpoint, req.FabricPeer.DomainNames)
			if err := s.validateFabricPeerConfig(req.FabricPeer); err != nil {
				return fmt.Errorf("invalid fabric peer configuration: %w", err)
			}
		}

		// Validate Fabric orderer configuration
		if req.FabricOrderer != nil {
			req.FabricOrderer.DomainNames = s.ensureExternalEndpointInDomains(req.FabricOrderer.ExternalEndpoint, req.FabricOrderer.DomainNames)
			if err := s.validateFabricOrdererConfig(req.FabricOrderer); err != nil {
				return fmt.Errorf("invalid fabric orderer configuration: %w", err)
			}
		}

	case types.PlatformBesu:
		if req.BesuNode == nil {
			return fmt.Errorf("besu configuration is required")
		}
		if err := s.validateBesuNodeConfig(req.BesuNode); err != nil {
			return fmt.Errorf("invalid besu configuration: %w", err)
		}
	default:
		return fmt.Errorf("unsupported blockchain platform: %s", req.BlockchainPlatform)
	}

	return nil
}

// validateFabricPeerConfig validates Fabric peer configuration
func (s *NodeService) validateFabricPeerConfig(config *types.FabricPeerConfig) error {
	// Validate required fields
	if config.Name == "" {
		return fmt.Errorf("name is required")
	}
	if config.OrganizationID == 0 {
		return fmt.Errorf("organization ID is required")
	}
	if config.MSPID == "" {
		return fmt.Errorf("MSP ID is required")
	}

	// Validate address formats
	addresses := map[string]string{
		"listen address":            config.ListenAddress,
		"chaincode address":         config.ChaincodeAddress,
		"events address":            config.EventsAddress,
		"operations listen address": config.OperationsListenAddress,
		"external endpoint":         config.ExternalEndpoint,
	}

	for addrType, addr := range addresses {
		if addr == "" {
			return fmt.Errorf("%s is required", addrType)
		}
		if err := s.validateAddressFormat(addr); err != nil {
			return fmt.Errorf("invalid %s format: %w", addrType, err)
		}
	}

	// Validate domain names format
	for i, domain := range config.DomainNames {
		if domain == "" {
			return fmt.Errorf("domain name at index %d cannot be empty", i)
		}
		if err := s.validateDomainName(domain); err != nil {
			return fmt.Errorf("invalid domain name '%s' at index %d: %w", domain, i, err)
		}
	}

	// Validate external endpoint format (should be a valid domain:port)
	if err := s.validateExternalEndpoint(config.ExternalEndpoint); err != nil {
		return fmt.Errorf("invalid external endpoint: %w", err)
	}

	// Validate deployment mode
	if config.Mode != "service" && config.Mode != "docker" {
		return fmt.Errorf("invalid deployment mode: %s (must be 'service' or 'docker')", config.Mode)
	}

	// Check for port conflicts between addresses
	if err := s.validatePeerAddressConflicts(config); err != nil {
		return fmt.Errorf("address conflicts: %w", err)
	}

	return nil
}

// validateFabricOrdererConfig validates Fabric orderer configuration
func (s *NodeService) validateFabricOrdererConfig(config *types.FabricOrdererConfig) error {
	// Validate required fields
	if config.Name == "" {
		return fmt.Errorf("name is required")
	}
	if config.OrganizationID == 0 {
		return fmt.Errorf("organization ID is required")
	}
	if config.MSPID == "" {
		return fmt.Errorf("MSP ID is required")
	}

	// Validate address formats
	addresses := map[string]string{
		"listen address":            config.ListenAddress,
		"admin address":             config.AdminAddress,
		"operations listen address": config.OperationsListenAddress,
		"external endpoint":         config.ExternalEndpoint,
	}

	for addrType, addr := range addresses {
		if addr == "" {
			return fmt.Errorf("%s is required", addrType)
		}
		if err := s.validateAddressFormat(addr); err != nil {
			return fmt.Errorf("invalid %s format: %w", addrType, err)
		}
	}

	// Validate domain names
	if len(config.DomainNames) == 0 {
		return fmt.Errorf("at least one domain name is required")
	}

	// Validate domain names format
	for i, domain := range config.DomainNames {
		if domain == "" {
			return fmt.Errorf("domain name at index %d cannot be empty", i)
		}
		if err := s.validateDomainName(domain); err != nil {
			return fmt.Errorf("invalid domain name '%s' at index %d: %w", domain, i, err)
		}
	}

	// Validate external endpoint format (should be a valid domain:port)
	if err := s.validateExternalEndpoint(config.ExternalEndpoint); err != nil {
		return fmt.Errorf("invalid external endpoint: %w", err)
	}

	// Validate deployment mode
	if config.Mode != "service" && config.Mode != "docker" {
		return fmt.Errorf("invalid deployment mode: %s (must be 'service' or 'docker')", config.Mode)
	}

	// Check for port conflicts between addresses
	if err := s.validateOrdererAddressConflicts(config); err != nil {
		return fmt.Errorf("address conflicts: %w", err)
	}

	return nil
}

// validateBesuNodeConfig validates Besu node configuration
func (s *NodeService) validateBesuNodeConfig(config *types.BesuNodeConfig) error {
	// Validate required fields
	if config.NetworkID == 0 {
		return fmt.Errorf("network ID is required")
	}
	if config.KeyID == 0 {
		return fmt.Errorf("key ID is required")
	}
	if config.P2PPort == 0 {
		return fmt.Errorf("P2P port is required")
	}
	if config.RPCPort == 0 {
		return fmt.Errorf("RPC port is required")
	}
	if config.P2PHost == "" {
		return fmt.Errorf("P2P host is required")
	}
	if config.RPCHost == "" {
		return fmt.Errorf("RPC host is required")
	}
	if config.ExternalIP == "" {
		return fmt.Errorf("external IP is required")
	}
	if config.InternalIP == "" {
		return fmt.Errorf("internal IP is required")
	}

	// Validate port ranges
	if config.P2PPort < 1024 || config.P2PPort > 65535 {
		return fmt.Errorf("P2P port must be between 1024 and 65535")
	}
	if config.RPCPort < 1024 || config.RPCPort > 65535 {
		return fmt.Errorf("RPC port must be between 1024 and 65535")
	}

	// Validate IP addresses
	if err := s.validateIPAddress(config.ExternalIP); err != nil {
		return fmt.Errorf("invalid external IP: %w", err)
	}
	if err := s.validateIPAddress(config.InternalIP); err != nil {
		return fmt.Errorf("invalid internal IP: %w", err)
	}

	// Validate deployment mode
	if config.Mode != "service" && config.Mode != "docker" {
		return fmt.Errorf("invalid deployment mode: %s (must be 'service' or 'docker')", config.Mode)
	}

	// Check for port conflicts
	if config.P2PPort == config.RPCPort {
		return fmt.Errorf("P2P port and RPC port cannot be the same")
	}

	return nil
}

// validateAddressFormat validates that an address has the correct host:port format
func (s *NodeService) validateAddressFormat(address string) error {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid address format %s: %w", address, err)
	}

	// Validate host is not empty
	if host == "" {
		return fmt.Errorf("host cannot be empty in address: %s", address)
	}

	// Validate port
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port number %s: %w", portStr, err)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port number %d out of range (1-65535)", port)
	}

	return nil
}

// validateIPAddress validates that a string is a valid IP address
func (s *NodeService) validateIPAddress(ip string) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}
	return nil
}

// validateDomainName validates that a string is a valid domain name
func (s *NodeService) validateDomainName(domain string) error {
	// Basic domain name validation
	if len(domain) == 0 || len(domain) > 253 {
		return fmt.Errorf("domain name length must be between 1 and 253 characters")
	}

	// Check for valid characters and structure
	parts := strings.Split(domain, ".")
	if len(parts) < 1 {
		return fmt.Errorf("domain name must have at least 1 part")
	}

	for i, part := range parts {
		if len(part) == 0 || len(part) > 63 {
			return fmt.Errorf("domain part %d length must be between 1 and 63 characters", i+1)
		}

		// Check for valid characters (letters, numbers, hyphens, but not starting/ending with hyphen)
		if part[0] == '-' || part[len(part)-1] == '-' {
			return fmt.Errorf("domain part %d cannot start or end with a hyphen", i+1)
		}

		// Check for valid characters
		for _, char := range part {
			if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
				return fmt.Errorf("domain part %d contains invalid character '%c'", i+1, char)
			}
		}
	}

	return nil
}

// validateExternalEndpoint validates that an external endpoint has a valid domain:port format
func (s *NodeService) validateExternalEndpoint(endpoint string) error {
	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint format %s: %w", endpoint, err)
	}

	// Validate host is not empty
	if host == "" {
		return fmt.Errorf("host cannot be empty in endpoint: %s", endpoint)
	}

	// Validate port
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port number %s: %w", portStr, err)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port number %d out of range (1-65535)", port)
	}

	// Validate host is a valid domain name or IP address
	if net.ParseIP(host) == nil {
		// If it's not an IP address, validate as domain name
		if err := s.validateDomainName(host); err != nil {
			return fmt.Errorf("invalid host in endpoint: %w", err)
		}
	}

	return nil
}

// validatePeerAddressConflicts checks for port conflicts in peer addresses
func (s *NodeService) validatePeerAddressConflicts(config *types.FabricPeerConfig) error {
	addresses := map[string]string{
		"listen":     config.ListenAddress,
		"chaincode":  config.ChaincodeAddress,
		"events":     config.EventsAddress,
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

// validateOrdererAddressConflicts checks for port conflicts in orderer addresses
func (s *NodeService) validateOrdererAddressConflicts(config *types.FabricOrdererConfig) error {
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

func (s *NodeService) determineNodeType(req CreateNodeRequest) types.NodeType {
	switch req.BlockchainPlatform {
	case types.PlatformFabric:
		if req.FabricPeer != nil {
			return types.NodeTypeFabricPeer
		}
		return types.NodeTypeFabricOrderer
	case types.PlatformBesu:
		return types.NodeTypeBesuFullnode
	}
	return ""
}

// generateSlug creates a URL-friendly slug from a string
func (s *NodeService) generateSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)

	// Replace spaces and underscores with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")

	// Remove all characters except letters, numbers, and hyphens
	reg := regexp.MustCompile("[^a-z0-9-]")
	slug = reg.ReplaceAllString(slug, "")

	// Replace multiple hyphens with a single hyphen
	reg = regexp.MustCompile("-+")
	slug = reg.ReplaceAllString(slug, "-")

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	return slug
}

// GetAllNodes retrieves all nodes without pagination
func (s *NodeService) GetAllNodes(ctx context.Context) (*PaginatedNodes, error) {
	// Get all nodes from the database
	dbNodes, err := s.db.ListNodes(ctx, &db.ListNodesParams{
		Limit:  1000, // Use a high limit to get all nodes
		Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// Get total count
	total, err := s.db.CountNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count nodes: %w", err)
	}

	// Map database nodes to service nodes
	nodes := make([]NodeResponse, len(dbNodes))
	for i, dbNode := range dbNodes {
		_, nodeResponse := s.mapDBNodeToServiceNode(dbNode)
		nodes[i] = *nodeResponse
	}

	return &PaginatedNodes{
		Items:       nodes,
		Total:       total,
		Page:        1,
		PageCount:   len(nodes),
		HasNextPage: false,
	}, nil
}

// GetNodeByID retrieves a node by its ID
func (s *NodeService) GetNodeByID(ctx context.Context, id int64) (*NodeResponse, error) {
	node, err := s.db.GetNode(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	_, nodeResponse := s.mapDBNodeToServiceNode(node)
	return nodeResponse, nil
}

// CreateNode creates a new node
func (s *NodeService) CreateNode(ctx context.Context, req CreateNodeRequest) (*NodeResponse, error) {
	if err := s.validateCreateNodeRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Generate slug from name
	slug := s.generateSlug(req.Name)

	// Check if slug already exists
	_, err := s.db.GetNodeBySlug(ctx, slug)
	if err == nil {
		return nil, fmt.Errorf("node with slug %s already exists", slug)
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("error checking slug existence: %w", err)
	}

	// Create node config based on request
	nodeConfig, err := s.createNodeConfig(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create node config: %w", err)
	}

	// Store node config
	configBytes, err := utils.StoreNodeConfig(nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to store node config: %w", err)
	}

	nodeType := s.determineNodeType(req)

	// Determine endpoint based on node type and config
	var endpoint sql.NullString
	switch nodeConfig := nodeConfig.(type) {
	case *types.FabricPeerConfig:
		endpoint = sql.NullString{
			String: nodeConfig.ExternalEndpoint, // Use ExternalEndpoint instead of ListenAddress
			Valid:  true,
		}
	case *types.FabricOrdererConfig:
		endpoint = sql.NullString{
			String: nodeConfig.ExternalEndpoint, // Use ExternalEndpoint instead of ListenAddress
			Valid:  true,
		}
	case *types.BesuNodeConfig:
		endpoint = sql.NullString{
			String: fmt.Sprintf("%s:%d", nodeConfig.ExternalIP, nodeConfig.P2PPort), // Use ExternalIP instead of P2PHost
			Valid:  true,
		}
	}

	// Create node in database
	node, err := s.db.CreateNode(ctx, &db.CreateNodeParams{
		Name:       req.Name,
		Slug:       slug,
		Platform:   string(req.BlockchainPlatform),
		NodeType:   sql.NullString{String: string(nodeType), Valid: true},
		Status:     string(types.NodeStatusPending),
		NodeConfig: sql.NullString{String: string(configBytes), Valid: true},
		Endpoint:   endpoint, // Add endpoint here
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	// Initialize the node based on its type
	deploymentConfig, err := s.initializeNode(ctx, node, req)
	if err != nil {
		// Update node status to failed if initialization fails
		s.updateNodeStatusWithError(ctx, node.ID, types.NodeStatusError, fmt.Sprintf("Failed to initialize node: %v", err))
		return nil, fmt.Errorf("failed to initialize node: %w", err)
	}

	// Store deployment config
	deploymentConfigJSON, err := json.Marshal(deploymentConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deployment config: %w", err)
	}

	// Update node with deployment config
	node, err = s.db.UpdateNodeDeploymentConfig(ctx, &db.UpdateNodeDeploymentConfigParams{
		ID:               node.ID,
		DeploymentConfig: sql.NullString{String: string(deploymentConfigJSON), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update node deployment config: %w", err)
	}

	// Start the node
	if err := s.startNode(ctx, node); err != nil {
		return nil, fmt.Errorf("failed to start node: %w", err)
	}

	// Use default validation timeout of 60 seconds
	validationTimeout := 60 * time.Second

	// Validate node connectivity with retry and exponential backoff
	if err := s.validateNodeConnectivityWithRetry(ctx, node, validationTimeout); err != nil {
		s.logger.Error("Node connectivity validation failed after retries", "node_id", node.ID, "error", err)
		// Update node status to error if validation fails
		s.updateNodeStatusWithError(ctx, node.ID, types.NodeStatusError, fmt.Sprintf("Node connectivity validation failed after retries: %v", err))
		return nil, fmt.Errorf("node connectivity validation failed after retries: %w", err)
	}

	node, err = s.db.GetNode(ctx, node.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	// Map database node to service node
	_, nodeResponse := s.mapDBNodeToServiceNode(node)

	// Publish node created event
	s.metricsService.Reload(ctx)

	return nodeResponse, nil
}

// Add new function to create node config
func (s *NodeService) createNodeConfig(req CreateNodeRequest) (types.NodeConfig, error) {
	switch req.BlockchainPlatform {
	case types.PlatformFabric:
		if req.FabricPeer != nil {
			return &types.FabricPeerConfig{
				BaseNodeConfig: types.BaseNodeConfig{
					Type: "fabric-peer",
					Mode: req.FabricPeer.Mode,
				},
				Name:                    req.FabricPeer.Name,
				OrganizationID:          req.FabricPeer.OrganizationID,
				MSPID:                   req.FabricPeer.MSPID,
				ListenAddress:           req.FabricPeer.ListenAddress,
				ChaincodeAddress:        req.FabricPeer.ChaincodeAddress,
				EventsAddress:           req.FabricPeer.EventsAddress,
				OperationsListenAddress: req.FabricPeer.OperationsListenAddress,
				ExternalEndpoint:        req.FabricPeer.ExternalEndpoint,
				DomainNames:             req.FabricPeer.DomainNames,
				Env:                     req.FabricPeer.Env,
				Version:                 req.FabricPeer.Version,
			}, nil
		} else if req.FabricOrderer != nil {
			return &types.FabricOrdererConfig{
				BaseNodeConfig: types.BaseNodeConfig{
					Type: "fabric-orderer",
					Mode: req.FabricOrderer.Mode,
				},
				Name:                    req.FabricOrderer.Name,
				OrganizationID:          req.FabricOrderer.OrganizationID,
				MSPID:                   req.FabricOrderer.MSPID,
				ListenAddress:           req.FabricOrderer.ListenAddress,
				AdminAddress:            req.FabricOrderer.AdminAddress,
				OperationsListenAddress: req.FabricOrderer.OperationsListenAddress,
				ExternalEndpoint:        req.FabricOrderer.ExternalEndpoint,
				DomainNames:             req.FabricOrderer.DomainNames,
				Env:                     req.FabricOrderer.Env,
				Version:                 req.FabricOrderer.Version,
			}, nil
		}
	case types.PlatformBesu:
		if req.BesuNode != nil {
			return &types.BesuNodeConfig{
				BaseNodeConfig: types.BaseNodeConfig{
					Type: "besu",
					Mode: req.BesuNode.Mode,
				},
				P2PPort:         req.BesuNode.P2PPort,
				RPCPort:         req.BesuNode.RPCPort,
				NetworkID:       req.BesuNode.NetworkID,
				ExternalIP:      req.BesuNode.ExternalIP,
				Env:             req.BesuNode.Env,
				KeyID:           req.BesuNode.KeyID,
				P2PHost:         req.BesuNode.P2PHost,
				RPCHost:         req.BesuNode.RPCHost,
				InternalIP:      req.BesuNode.InternalIP,
				BootNodes:       req.BesuNode.BootNodes,
				Version:         req.BesuNode.Version,
				MetricsEnabled:  req.BesuNode.MetricsEnabled,
				MetricsPort:     req.BesuNode.MetricsPort,
				MetricsProtocol: "PROMETHEUS",
			}, nil
		}
	}
	return nil, fmt.Errorf("invalid node configuration")
}

// initializeNode initializes a node and returns its deployment config
func (s *NodeService) initializeNode(ctx context.Context, dbNode *db.Node, req CreateNodeRequest) (types.NodeDeploymentConfig, error) {
	switch types.BlockchainPlatform(dbNode.Platform) {
	case types.PlatformFabric:
		if req.FabricPeer != nil {
			config, err := s.initializeFabricPeer(ctx, dbNode, req.FabricPeer)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize fabric peer: %w", err)
			}
			return config, nil
		} else if req.FabricOrderer != nil {
			config, err := s.initializeFabricOrderer(ctx, dbNode, req.FabricOrderer)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize fabric orderer: %w", err)
			}
			return config, nil
		}
	case types.PlatformBesu:
		if req.BesuNode != nil {
			config, err := s.initializeBesuNode(ctx, dbNode, req.BesuNode)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize besu node: %w", err)
			}
			return config, nil
		}
	}
	return nil, fmt.Errorf("unsupported platform: %s", dbNode.Platform)
}

// validatePort checks if a port is valid and available
func (s *NodeService) validatePort(host string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port number %d out of range (1-65535)", port)
	}

	// Check if port is in use
	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("address %s is not available: %w", addr, err)
	}
	listener.Close()
	return nil
}

// updateNodeStatus updates the status of a node in the database
func (s *NodeService) updateNodeStatus(ctx context.Context, nodeID int64, status types.NodeStatus) error {
	_, err := s.db.UpdateNodeStatus(ctx, &db.UpdateNodeStatusParams{
		ID:     nodeID,
		Status: string(status),
	})
	if err != nil {
		return fmt.Errorf("failed to update node status: %w", err)
	}
	return nil
}

func (s *NodeService) updateNodeStatusWithError(ctx context.Context, nodeID int64, status types.NodeStatus, errorMessage string) error {
	_, err := s.db.UpdateNodeStatusWithError(ctx, &db.UpdateNodeStatusWithErrorParams{
		ID:           nodeID,
		Status:       string(status),
		ErrorMessage: sql.NullString{String: errorMessage, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to update node status with error: %w", err)
	}
	return nil
}

// GetNode retrieves a node by ID
func (s *NodeService) GetNode(ctx context.Context, id int64) (*NodeResponse, error) {
	node, err := s.db.GetNode(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("node not found", map[string]interface{}{
				"id": id,
			})
		}
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	_, nodeResponse := s.mapDBNodeToServiceNode(node)
	return nodeResponse, nil
}

// ListNodes retrieves a paginated list of nodes
func (s *NodeService) ListNodes(ctx context.Context, platform *types.BlockchainPlatform, page, limit int) (*PaginatedNodes, error) {
	var dbNodes []*db.Node
	var err error
	var total int64

	offset := (page - 1) * limit

	if platform != nil {
		// Get nodes filtered by platform
		dbNodes, err = s.db.ListNodesByPlatform(ctx, &db.ListNodesByPlatformParams{
			Platform: string(*platform),
			Limit:    int64(limit),
			Offset:   int64(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list nodes: %w", err)
		}
		total, err = s.db.CountNodesByPlatform(ctx, string(*platform))
	} else {
		// Get all nodes
		dbNodes, err = s.db.ListNodes(ctx, &db.ListNodesParams{
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list nodes: %w", err)
		}
		total, err = s.db.CountNodes(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to count nodes: %w", err)
	}

	nodes := make([]NodeResponse, len(dbNodes))
	for i, dbNode := range dbNodes {
		_, nodeResponse := s.mapDBNodeToServiceNode(dbNode)
		nodes[i] = *nodeResponse
	}

	return &PaginatedNodes{
		Items:       nodes,
		Total:       total,
		Page:        page,
		PageCount:   (int(total) + limit - 1) / limit,
		HasNextPage: (int(total)+limit-1)/limit > page,
	}, nil
}

// Update mapDBNodeToServiceNode to include deployment config and MSPID
func (s *NodeService) mapDBNodeToServiceNode(dbNode *db.Node) (*Node, *NodeResponse) {
	ctx := context.Background()
	var nodeConfig types.NodeConfig
	var deploymentConfig types.NodeDeploymentConfig

	// Load node config
	if dbNode.NodeConfig.Valid {
		var err error
		nodeConfig, err = utils.LoadNodeConfig([]byte(dbNode.NodeConfig.String))
		if err != nil {
			s.logger.Error("failed to load node config", "error", err)
		}
	}

	// Load deployment config
	if dbNode.DeploymentConfig.Valid {
		var err error
		deploymentConfig, err = utils.DeserializeDeploymentConfig(dbNode.DeploymentConfig.String)
		if err != nil {
			s.logger.Error("failed to deserialize deployment config", "error", err)
		}
	}

	// Create base node
	node := &Node{
		ID:                 dbNode.ID,
		Name:               dbNode.Name,
		BlockchainPlatform: types.BlockchainPlatform(dbNode.Platform),
		NodeType:           types.NodeType(dbNode.NodeType.String),
		Status:             types.NodeStatus(dbNode.Status),
		Endpoint:           dbNode.Endpoint.String,
		PublicEndpoint:     dbNode.PublicEndpoint.String,
		NodeConfig:         nodeConfig,
		DeploymentConfig:   deploymentConfig,
		CreatedAt:          dbNode.CreatedAt,
		UpdatedAt:          dbNode.UpdatedAt.Time,
		ErrorMessage:       dbNode.ErrorMessage.String,
	}

	// Create node response
	nodeResponse := &NodeResponse{
		ID:           dbNode.ID,
		Name:         dbNode.Name,
		Platform:     dbNode.Platform,
		Status:       dbNode.Status,
		NodeType:     types.NodeType(dbNode.NodeType.String),
		ErrorMessage: dbNode.ErrorMessage.String,
		Endpoint:     dbNode.Endpoint.String,
		CreatedAt:    dbNode.CreatedAt,
		UpdatedAt:    dbNode.UpdatedAt.Time,
	}

	// Add type-specific properties
	if nodeConfig != nil {
		switch config := nodeConfig.(type) {
		case *types.FabricPeerConfig:
			node.MSPID = config.MSPID
			nodeResponse.FabricPeer = &FabricPeerProperties{
				MSPID:             config.MSPID,
				OrganizationID:    config.OrganizationID,
				ExternalEndpoint:  config.ExternalEndpoint,
				ChaincodeAddress:  config.ChaincodeAddress,
				EventsAddress:     config.EventsAddress,
				OperationsAddress: config.OperationsListenAddress,
				ListenAddress:     config.ListenAddress,
				DomainNames:       config.DomainNames,
				Version:           config.Version,
			}
			// Enrich with deployment config if available
			if peerDeployConfig, ok := deploymentConfig.(*types.FabricPeerDeploymentConfig); ok {
				nodeResponse.FabricPeer.ExternalEndpoint = config.ExternalEndpoint
				nodeResponse.FabricPeer.ListenAddress = config.ListenAddress
				nodeResponse.FabricPeer.ChaincodeAddress = config.ChaincodeAddress
				nodeResponse.FabricPeer.EventsAddress = config.EventsAddress
				nodeResponse.FabricPeer.OperationsAddress = config.OperationsListenAddress
				nodeResponse.FabricPeer.TLSKeyID = peerDeployConfig.TLSKeyID
				nodeResponse.FabricPeer.SignKeyID = peerDeployConfig.SignKeyID
				nodeResponse.FabricPeer.Mode = config.Mode
			}
			// Add certificate information
			peerConfig, _ := nodeConfig.(*types.FabricPeerConfig)

			peerDeployConfig, ok := deploymentConfig.(*types.FabricPeerDeploymentConfig)
			if ok && peerConfig != nil {
				nodeResponse.FabricPeer.AddressOverrides = peerDeployConfig.AddressOverrides
				// Get certificates from key service
				signKey, err := s.keymanagementService.GetKey(ctx, int(peerDeployConfig.SignKeyID))
				if err == nil && signKey.Certificate != nil {
					nodeResponse.FabricPeer.SignCert = *signKey.Certificate
					nodeResponse.FabricPeer.SignKeyID = peerDeployConfig.SignKeyID
				}

				tlsKey, err := s.keymanagementService.GetKey(ctx, int(peerDeployConfig.TLSKeyID))
				if err == nil && tlsKey.Certificate != nil {
					nodeResponse.FabricPeer.TLSCert = *tlsKey.Certificate
					nodeResponse.FabricPeer.TLSKeyID = peerDeployConfig.TLSKeyID
				}

				// Get CA certificates from organization
				org, err := s.orgService.GetOrganization(ctx, peerConfig.OrganizationID)
				if err == nil {
					if org.SignKeyID.Valid {
						signCAKey, err := s.keymanagementService.GetKey(ctx, int(org.SignKeyID.Int64))
						if err == nil && signCAKey.Certificate != nil {
							nodeResponse.FabricPeer.SignCACert = *signCAKey.Certificate
						}
					}

					if org.TlsRootKeyID.Valid {
						tlsCAKey, err := s.keymanagementService.GetKey(ctx, int(org.TlsRootKeyID.Int64))
						if err == nil && tlsCAKey.Certificate != nil {
							nodeResponse.FabricPeer.TLSCACert = *tlsCAKey.Certificate
						}
					}
				}
			}

		case *types.FabricOrdererConfig:
			node.MSPID = config.MSPID
			nodeResponse.FabricOrderer = &FabricOrdererProperties{
				MSPID:             config.MSPID,
				OrganizationID:    config.OrganizationID,
				ExternalEndpoint:  config.ExternalEndpoint,
				AdminAddress:      config.AdminAddress,
				OperationsAddress: config.OperationsListenAddress,
				ListenAddress:     config.ListenAddress,
				DomainNames:       config.DomainNames,
				Version:           config.Version,
			}
			// Enrich with deployment config if available
			if ordererDeployConfig, ok := deploymentConfig.(*types.FabricOrdererDeploymentConfig); ok {
				nodeResponse.FabricOrderer.ExternalEndpoint = config.ExternalEndpoint
				nodeResponse.FabricOrderer.ListenAddress = config.ListenAddress
				nodeResponse.FabricOrderer.AdminAddress = config.AdminAddress
				nodeResponse.FabricOrderer.OperationsAddress = config.OperationsListenAddress
				nodeResponse.FabricOrderer.TLSKeyID = ordererDeployConfig.TLSKeyID
				nodeResponse.FabricOrderer.SignKeyID = ordererDeployConfig.SignKeyID
				nodeResponse.FabricOrderer.Mode = config.Mode
			}
			// Add certificate information
			ordererConfig, ok := nodeConfig.(*types.FabricOrdererConfig)
			ordererDeployConfig, ok := deploymentConfig.(*types.FabricOrdererDeploymentConfig)
			if ok && ordererConfig != nil {
				// Get certificates from key service
				signKey, err := s.keymanagementService.GetKey(ctx, int(ordererDeployConfig.SignKeyID))
				if err == nil && signKey.Certificate != nil {
					nodeResponse.FabricOrderer.SignCert = *signKey.Certificate
				}

				tlsKey, err := s.keymanagementService.GetKey(ctx, int(ordererDeployConfig.TLSKeyID))
				if err == nil && tlsKey.Certificate != nil {
					nodeResponse.FabricOrderer.TLSCert = *tlsKey.Certificate
				}

				// Get CA certificates from organization
				org, err := s.orgService.GetOrganization(ctx, ordererConfig.OrganizationID)
				if err == nil {
					if org.SignKeyID.Valid {
						signCAKey, err := s.keymanagementService.GetKey(ctx, int(org.SignKeyID.Int64))
						if err == nil && signCAKey.Certificate != nil {
							nodeResponse.FabricOrderer.SignCACert = *signCAKey.Certificate
						}
					}

					if org.TlsRootKeyID.Valid {
						tlsCAKey, err := s.keymanagementService.GetKey(ctx, int(org.TlsRootKeyID.Int64))
						if err == nil && tlsCAKey.Certificate != nil {
							nodeResponse.FabricOrderer.TLSCACert = *tlsCAKey.Certificate
						}
					}
				}
			}
		case *types.BesuNodeConfig:
			nodeResponse.BesuNode = &BesuNodeProperties{
				NetworkID:  config.NetworkID,
				P2PPort:    config.P2PPort,
				RPCPort:    config.RPCPort,
				ExternalIP: config.ExternalIP,
				InternalIP: config.InternalIP,
				P2PHost:    config.P2PHost,
				RPCHost:    config.RPCHost,
				KeyID:      config.KeyID,
				Mode:       config.Mode,
				BootNodes:  config.BootNodes,
				// Add metrics configuration
				MetricsEnabled:  config.MetricsEnabled,
				MetricsHost:     "0.0.0.0", // Default to allow metrics from any interface
				MetricsPort:     uint(config.MetricsPort),
				MetricsProtocol: config.MetricsProtocol,
			}

			// Fetch key information from key management service
			if config.KeyID > 0 {
				key, err := s.keymanagementService.GetKey(ctx, int(config.KeyID))
				if err == nil {
					nodeResponse.BesuNode.KeyAddress = key.EthereumAddress
					nodeResponse.BesuNode.PublicKey = key.PublicKey
				} else {
					s.logger.Warn("failed to get key information for Besu node", "nodeID", dbNode.ID, "keyID", config.KeyID, "error", err)
				}
			}

			deployConfig, ok := deploymentConfig.(*types.BesuNodeDeploymentConfig)
			if ok {
				nodeResponse.BesuNode.KeyID = deployConfig.KeyID
				nodeResponse.BesuNode.EnodeURL = deployConfig.EnodeURL
				// Add metrics configuration from deployment config
				nodeResponse.BesuNode.MetricsEnabled = deployConfig.MetricsEnabled
				nodeResponse.BesuNode.MetricsPort = uint(deployConfig.MetricsPort)
				nodeResponse.BesuNode.MetricsProtocol = deployConfig.MetricsProtocol
			}
		}
	}

	return node, nodeResponse
}

// StartNode starts a node by ID
func (s *NodeService) StartNode(ctx context.Context, id int64) (*NodeResponse, error) {
	node, err := s.db.GetNode(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	if err := s.startNode(ctx, node); err != nil {
		return nil, err
	}

	_, nodeResponse := s.mapDBNodeToServiceNode(node)
	return nodeResponse, nil
}

// StopNode stops a node by ID
func (s *NodeService) StopNode(ctx context.Context, id int64) (*NodeResponse, error) {
	node, err := s.db.GetNode(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	// Update status to stopping
	if err := s.updateNodeStatus(ctx, id, types.NodeStatusStopping); err != nil {
		return nil, fmt.Errorf("failed to update node status: %w", err)
	}

	// Create stopping event
	if err := s.eventService.CreateEvent(ctx, id, NodeEventStopping, map[string]interface{}{
		"node_id": id,
		"name":    node.Name,
	}); err != nil {
		s.logger.Error("Failed to create stopping event", "error", err)
	}

	var stopErr error
	switch types.NodeType(node.NodeType.String) {
	case types.NodeTypeFabricPeer:
		stopErr = s.stopFabricPeer(ctx, node)
	case types.NodeTypeFabricOrderer:
		stopErr = s.stopFabricOrderer(ctx, node)
	case types.NodeTypeBesuFullnode:
		stopErr = s.stopBesuNode(ctx, node)
	default:
		stopErr = fmt.Errorf("unsupported node type: %s", node.NodeType.String)
	}

	if stopErr != nil {
		s.logger.Error("Failed to stop node", "error", stopErr)
		// Update status to error if stop failed
		if err := s.updateNodeStatusWithError(ctx, id, types.NodeStatusError, fmt.Sprintf("Failed to stop node: %v", stopErr)); err != nil {
			s.logger.Error("Failed to update node status after stop error", "error", err)
		}
		// Create error event
		if err := s.eventService.CreateEvent(ctx, id, NodeEventError, map[string]interface{}{
			"node_id": id,
			"name":    node.Name,
			"error":   stopErr.Error(),
		}); err != nil {
			s.logger.Error("Failed to create error event", "error", err)
		}
		return nil, fmt.Errorf("failed to stop node: %w", stopErr)
	}

	// Update status to stopped if stop succeeded
	if err := s.updateNodeStatus(ctx, id, types.NodeStatusStopped); err != nil {
		return nil, fmt.Errorf("failed to update node status: %w", err)
	}

	// Create stopped event
	if err := s.eventService.CreateEvent(ctx, id, NodeEventStopped, map[string]interface{}{
		"node_id": id,
		"name":    node.Name,
	}); err != nil {
		s.logger.Error("Failed to create stopped event", "error", err)
	}

	_, nodeResponse := s.mapDBNodeToServiceNode(node)
	return nodeResponse, nil
}

// startNode starts a node based on its type and configuration
func (s *NodeService) startNode(ctx context.Context, dbNode *db.Node) error {
	// Update status to starting
	if err := s.updateNodeStatus(ctx, dbNode.ID, types.NodeStatusStarting); err != nil {
		return fmt.Errorf("failed to update node status: %w", err)
	}

	// Create starting event
	if err := s.eventService.CreateEvent(ctx, dbNode.ID, NodeEventStarting, map[string]interface{}{
		"node_id": dbNode.ID,
		"name":    dbNode.Name,
	}); err != nil {
		s.logger.Error("Failed to create starting event", "error", err)
	}

	var startErr error
	switch types.NodeType(dbNode.NodeType.String) {
	case types.NodeTypeFabricPeer:
		startErr = s.startFabricPeer(ctx, dbNode)
	case types.NodeTypeFabricOrderer:
		startErr = s.startFabricOrderer(ctx, dbNode)
	case types.NodeTypeBesuFullnode:
		startErr = s.startBesuNode(ctx, dbNode)
	default:
		startErr = fmt.Errorf("unsupported node type: %s", dbNode.NodeType.String)
	}

	if startErr != nil {
		s.logger.Error("Failed to start node", "error", startErr)
		// Update status to error if start failed
		if err := s.updateNodeStatusWithError(ctx, dbNode.ID, types.NodeStatusError, fmt.Sprintf("Failed to start node: %v", startErr)); err != nil {
			s.logger.Error("Failed to update node status after start error", "error", err)
		}
		// Create error event
		if err := s.eventService.CreateEvent(ctx, dbNode.ID, NodeEventError, map[string]interface{}{
			"node_id": dbNode.ID,
			"name":    dbNode.Name,
			"error":   startErr.Error(),
		}); err != nil {
			s.logger.Error("Failed to create error event", "error", err)
		}
		return fmt.Errorf("failed to start node: %w", startErr)
	}

	// Update status to running if start succeeded
	if err := s.updateNodeStatus(ctx, dbNode.ID, types.NodeStatusRunning); err != nil {
		return fmt.Errorf("failed to update node status: %w", err)
	}

	// Create started event
	if err := s.eventService.CreateEvent(ctx, dbNode.ID, NodeEventStarted, map[string]interface{}{
		"node_id": dbNode.ID,
		"name":    dbNode.Name,
	}); err != nil {
		s.logger.Error("Failed to create started event", "error", err)
	}

	return nil
}

// DeleteNode deletes a node by ID
func (s *NodeService) DeleteNode(ctx context.Context, id int64) error {
	// Get the node first to check its type and deployment config
	node, err := s.db.GetNode(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.Info("Node not found, already deleted", "id", id)
			return nil
		}
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Stop the node first
	if types.NodeStatus(node.Status) == types.NodeStatusRunning {
		_, err := s.StopNode(ctx, id)
		if err != nil {
			s.logger.Warn("Failed to stop node during deletion", "error", err)
			// Continue with deletion even if stop fails
		}
	}

	// Clean up node-specific resources based on type
	if err := s.cleanupNodeResources(ctx, node); err != nil {
		s.logger.Warn("Failed to cleanup node resources", "error", err)
		// Continue with deletion even if cleanup fails
	}

	// Delete the node from the database
	if err := s.db.DeleteNode(ctx, id); err != nil {
		if err == sql.ErrNoRows {
			s.logger.Info("Node not found during deletion, already deleted", "id", id)
			return nil
		}
		return fmt.Errorf("failed to delete node from database: %w", err)
	}

	// Publish node deleted event
	s.metricsService.Reload(ctx)

	return nil
}

// Update cleanupNodeResources to use the new function
func (s *NodeService) cleanupNodeResources(ctx context.Context, node *db.Node) error {
	// Get the home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	deploymentConfig, err := utils.DeserializeDeploymentConfig(node.DeploymentConfig.String)
	if err != nil {
		return fmt.Errorf("failed to deserialize deployment config: %w", err)
	}

	// Clean up service files based on platform
	switch runtime.GOOS {
	case "linux":
		// Remove systemd service file
		serviceFile := fmt.Sprintf("/etc/systemd/system/%s.service", deploymentConfig.GetServiceName())
		if err := os.Remove(serviceFile); err != nil {
			if !os.IsNotExist(err) {
				s.logger.Warn("Failed to remove systemd service file", "error", err)
			}
		}

	case "darwin":
		// Remove launchd plist file
		plistFile := filepath.Join(homeDir, "Library/LaunchAgents", fmt.Sprintf("dev.chainlaunch.%s.plist", deploymentConfig.GetServiceName()))
		if err := os.Remove(plistFile); err != nil {
			if !os.IsNotExist(err) {
				s.logger.Warn("Failed to remove launchd plist file", "error", err)
			}
		}
	}

	// Clean up node-specific resources based on type
	switch types.NodeType(node.NodeType.String) {
	case types.NodeTypeFabricPeer:
		if err := s.cleanupPeerResources(ctx, node); err != nil {
			s.logger.Warn("Failed to cleanup peer resources", "error", err)
		}
	case types.NodeTypeFabricOrderer:
		if err := s.cleanupOrdererResources(ctx, node); err != nil {
			s.logger.Warn("Failed to cleanup orderer resources", "error", err)
		}
	case types.NodeTypeBesuFullnode:
		if err := s.cleanupBesuResources(ctx, node); err != nil {
			s.logger.Warn("Failed to cleanup besu resources", "error", err)
		}
	default:
		s.logger.Warn("Unknown node type for cleanup", "type", node.NodeType.String)
	}

	return nil
}

func (s *NodeService) GetNodeLogPath(ctx context.Context, node *NodeResponse) (string, error) {
	dbNode, err := s.db.GetNode(ctx, node.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get node: %w", err)
	}

	// Get deployment config
	deploymentConfig, err := utils.DeserializeDeploymentConfig(dbNode.DeploymentConfig.String)
	if err != nil {
		return "", fmt.Errorf("failed to deserialize deployment config: %w", err)
	}

	switch types.NodeType(dbNode.NodeType.String) {
	case types.NodeTypeFabricPeer:
		nodeConfig, err := utils.LoadNodeConfig([]byte(dbNode.NodeConfig.String))
		if err != nil {
			return "", fmt.Errorf("failed to deserialize node config: %w", err)
		}
		peerNodeConfig, ok := nodeConfig.(*types.FabricPeerConfig)
		if !ok {
			return "", fmt.Errorf("failed to assert node config to FabricPeerConfig")
		}
		s.logger.Debug("Peer config", "config", peerNodeConfig, "deploymentConfig", deploymentConfig)
		// Get organization
		org, err := s.orgService.GetOrganization(ctx, peerNodeConfig.OrganizationID)
		if err != nil {
			return "", fmt.Errorf("failed to get organization: %w", err)
		}

		// Create peer instance
		localPeer := s.getPeerFromConfig(dbNode, org, peerNodeConfig)

		// Tail logs from peer
		return localPeer.GetStdOutPath(), nil
	case types.NodeTypeFabricOrderer:
		// Convert to FabricOrdererDeploymentConfig
		nodeConfig, err := utils.LoadNodeConfig([]byte(dbNode.NodeConfig.String))
		if err != nil {
			return "", fmt.Errorf("failed to deserialize node config: %w", err)
		}
		ordererNodeConfig, ok := nodeConfig.(*types.FabricOrdererConfig)
		if !ok {
			return "", fmt.Errorf("failed to assert node config to FabricOrdererConfig")
		}
		s.logger.Info("Orderer config", "config", ordererNodeConfig, "deploymentConfig", deploymentConfig)
		// Get organization
		org, err := s.orgService.GetOrganization(ctx, ordererNodeConfig.OrganizationID)
		if err != nil {
			return "", fmt.Errorf("failed to get organization: %w", err)
		}
		// Create orderer instance
		localOrderer := s.getOrdererFromConfig(dbNode, org, ordererNodeConfig)
		// Tail logs from orderer
		return localOrderer.GetStdOutPath(), nil
	case types.NodeTypeBesuFullnode:
		nodeConfig, err := utils.LoadNodeConfig([]byte(dbNode.NodeConfig.String))
		if err != nil {
			return "", fmt.Errorf("failed to deserialize node config: %w", err)
		}
		besuNodeConfig, ok := nodeConfig.(*types.BesuNodeConfig)
		if !ok {
			return "", fmt.Errorf("failed to assert node config to BesuNodeConfig")
		}
		besuDeployConfig := deploymentConfig.ToBesuNodeConfig()

		localBesu, err := s.getBesuFromConfig(ctx, dbNode, besuNodeConfig, besuDeployConfig)
		if err != nil {
			return "", fmt.Errorf("failed to get besu from config: %w", err)
		}
		return localBesu.GetStdOutPath(), nil
	default:
		return "", fmt.Errorf("unsupported node type for log tailing: %s", dbNode.NodeType.String)
	}
}

// TailLogs returns a channel that receives log lines from the specified node
func (s *NodeService) TailLogs(ctx context.Context, nodeID int64, tail int, follow bool) (<-chan string, error) {
	// Get the node first to verify it exists
	dbNode, err := s.db.GetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	// Get deployment config
	deploymentConfig, err := utils.DeserializeDeploymentConfig(dbNode.DeploymentConfig.String)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize deployment config: %w", err)
	}

	switch types.NodeType(dbNode.NodeType.String) {
	case types.NodeTypeFabricPeer:
		nodeConfig, err := utils.LoadNodeConfig([]byte(dbNode.NodeConfig.String))
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize node config: %w", err)
		}
		peerNodeConfig, ok := nodeConfig.(*types.FabricPeerConfig)
		if !ok {
			return nil, fmt.Errorf("failed to assert node config to FabricPeerConfig")
		}
		s.logger.Debug("Peer config", "config", peerNodeConfig, "deploymentConfig", deploymentConfig)
		// Get organization
		org, err := s.orgService.GetOrganization(ctx, peerNodeConfig.OrganizationID)
		if err != nil {
			return nil, fmt.Errorf("failed to get organization: %w", err)
		}

		// Create peer instance
		localPeer := s.getPeerFromConfig(dbNode, org, peerNodeConfig)

		// Tail logs from peer
		return localPeer.TailLogs(ctx, tail, follow)
	case types.NodeTypeFabricOrderer:
		// Convert to FabricOrdererDeploymentConfig
		nodeConfig, err := utils.LoadNodeConfig([]byte(dbNode.NodeConfig.String))
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize node config: %w", err)
		}
		ordererNodeConfig, ok := nodeConfig.(*types.FabricOrdererConfig)
		if !ok {
			return nil, fmt.Errorf("failed to assert node config to FabricOrdererConfig")
		}
		s.logger.Info("Orderer config", "config", ordererNodeConfig, "deploymentConfig", deploymentConfig)
		// Get organization
		org, err := s.orgService.GetOrganization(ctx, ordererNodeConfig.OrganizationID)
		if err != nil {
			return nil, fmt.Errorf("failed to get organization: %w", err)
		}
		// Create orderer instance
		localOrderer := s.getOrdererFromConfig(dbNode, org, ordererNodeConfig)
		// Tail logs from orderer
		return localOrderer.TailLogs(ctx, tail, follow)
	case types.NodeTypeBesuFullnode:
		nodeConfig, err := utils.LoadNodeConfig([]byte(dbNode.NodeConfig.String))
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize node config: %w", err)
		}
		besuNodeConfig, ok := nodeConfig.(*types.BesuNodeConfig)
		if !ok {
			return nil, fmt.Errorf("failed to assert node config to BesuNodeConfig")
		}
		besuDeployConfig := deploymentConfig.ToBesuNodeConfig()

		localBesu, err := s.getBesuFromConfig(ctx, dbNode, besuNodeConfig, besuDeployConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to get besu from config: %w", err)
		}
		return localBesu.TailLogs(ctx, tail, follow)
	default:
		return nil, fmt.Errorf("unsupported node type for log tailing: %s", dbNode.NodeType.String)
	}
}

// GetNodeEvents retrieves a paginated list of node events
func (s *NodeService) GetNodeEvents(ctx context.Context, nodeID int64, page, limit int) ([]NodeEvent, error) {
	return s.eventService.GetEvents(ctx, nodeID, page, limit)
}

// GetLatestNodeEvent retrieves the latest event for a node
func (s *NodeService) GetLatestNodeEvent(ctx context.Context, nodeID int64) (*NodeEvent, error) {
	return s.eventService.GetLatestEvent(ctx, nodeID)
}

// GetEventsByType retrieves a paginated list of node events of a specific type
func (s *NodeService) GetEventsByType(ctx context.Context, nodeID int64, eventType NodeEventType, page, limit int) ([]NodeEvent, error) {
	return s.eventService.GetEventsByType(ctx, nodeID, eventType, page, limit)
}

// Add a method to get full node details when needed
func (s *NodeService) GetNodeWithConfig(ctx context.Context, id int64) (*Node, error) {
	dbNode, err := s.db.GetNode(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	node, _ := s.mapDBNodeToServiceNode(dbNode)
	return node, nil
}

// Update the fabric deployer to use this method
func (s *NodeService) GetNodeForDeployment(ctx context.Context, id int64) (*Node, error) {
	return s.GetNodeWithConfig(ctx, id)
}

// Channel represents a Fabric channel
type Channel struct {
	Name      string    `json:"name"`
	BlockNum  int64     `json:"blockNum"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *NodeService) GetFabricChaincodes(ctx context.Context, id int64, channelID string) ([]*lifecycle.QueryChaincodeDefinitionsResult_ChaincodeDefinition, error) {
	peer, err := s.GetFabricPeer(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer: %w", err)
	}
	committedChaincodes, err := peer.GetCommittedChaincodes(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get committed chaincodes: %w", err)
	}
	return committedChaincodes, nil
}

// GetNodeChannels retrieves the list of channels for a Fabric node
func (s *NodeService) GetNodeChannels(ctx context.Context, id int64) ([]Channel, error) {
	// Get the node first
	node, err := s.db.GetNode(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("node not found", nil)
		}
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	// Verify node type
	nodeType := types.NodeType(node.NodeType.String)
	if nodeType != types.NodeTypeFabricPeer && nodeType != types.NodeTypeFabricOrderer {
		return nil, errors.NewValidationError("node is not a Fabric node", nil)
	}

	switch nodeType {
	case types.NodeTypeFabricPeer:
		// Get peer instance
		peer, err := s.GetFabricPeer(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get peer: %w", err)
		}
		peerChannels, err := peer.GetChannels(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get peer channels: %w", err)
		}
		channels := make([]Channel, len(peerChannels))
		for i, channel := range peerChannels {
			channels[i] = Channel{
				Name:      channel.Name,
				BlockNum:  channel.BlockNum,
				CreatedAt: channel.CreatedAt,
			}
		}
		return channels, nil

	case types.NodeTypeFabricOrderer:
		// Get orderer instance
		orderer, err := s.GetFabricOrderer(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get orderer: %w", err)
		}
		ordererChannels, err := orderer.GetChannels(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get orderer channels: %w", err)
		}
		channels := make([]Channel, len(ordererChannels))
		for i, channel := range ordererChannels {
			channels[i] = Channel{
				Name:      channel.Name,
				BlockNum:  channel.BlockNum,
				CreatedAt: channel.CreatedAt,
			}
		}
		return channels, nil
	}

	return nil, fmt.Errorf("unsupported node type: %s", nodeType)
}

// RenewCertificates renews the certificates for a node
func (s *NodeService) RenewCertificates(ctx context.Context, id int64) (*NodeResponse, error) {
	// Get the node from database
	node, err := s.db.GetNode(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("node not found", nil)
		}
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	// Update status to indicate certificate renewal is in progress
	if err := s.updateNodeStatus(ctx, id, types.NodeStatusUpdating); err != nil {
		return nil, fmt.Errorf("failed to update node status: %w", err)
	}

	// Get deployment config
	deploymentConfig, err := utils.DeserializeDeploymentConfig(node.DeploymentConfig.String)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize deployment config: %w", err)
	}

	var renewErr error
	switch types.NodeType(node.NodeType.String) {
	case types.NodeTypeFabricPeer:
		renewErr = s.renewPeerCertificates(ctx, node, deploymentConfig)
	case types.NodeTypeFabricOrderer:
		renewErr = s.renewOrdererCertificates(ctx, node, deploymentConfig)
	default:
		renewErr = fmt.Errorf("certificate renewal not supported for node type: %s", node.NodeType.String)
	}

	if renewErr != nil {
		// Update status to error if renewal failed
		if err := s.updateNodeStatusWithError(ctx, id, types.NodeStatusError, fmt.Sprintf("Failed to renew certificates: %v", renewErr)); err != nil {
			s.logger.Error("Failed to update node status after renewal error", "error", err)
		}
		// Create error event
		if err := s.eventService.CreateEvent(ctx, id, NodeEventError, map[string]interface{}{
			"node_id": id,
			"name":    node.Name,
			"action":  "certificate_renewal",
			"error":   renewErr.Error(),
		}); err != nil {
			s.logger.Error("Failed to create error event", "error", err)
		}
		return nil, fmt.Errorf("failed to renew certificates: %w", renewErr)
	}

	// Update status to running after successful renewal
	if err := s.updateNodeStatus(ctx, id, types.NodeStatusRunning); err != nil {
		return nil, fmt.Errorf("failed to update node status: %w", err)
	}

	// Get updated node
	updatedNode, err := s.GetNode(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated node: %w", err)
	}

	return updatedNode, nil
}

// UpdateNodeEnvironment updates the environment variables for a node
func (s *NodeService) UpdateNodeEnvironment(ctx context.Context, nodeID int64, req *types.UpdateNodeEnvRequest) (*types.UpdateNodeEnvResponse, error) {
	// Get the node from the database
	dbNode, err := s.db.GetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	// Create environment update event
	if err := s.eventService.CreateEvent(ctx, nodeID, NodeEventStarting, map[string]interface{}{
		"node_id": nodeID,
		"name":    dbNode.Name,
		"action":  "environment_update",
	}); err != nil {
		s.logger.Error("Failed to create environment update event", "error", err)
	}

	// Get the node's current configuration
	switch dbNode.NodeType.String {
	case string(types.NodeTypeFabricPeer):
		var peerConfig types.FabricPeerConfig
		if err := json.Unmarshal([]byte(dbNode.Config.String), &peerConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal peer config: %w", err)
		}
		peerConfig.Env = req.Env
		newConfig, err := json.Marshal(peerConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal updated peer config: %w", err)
		}
		if _, err := s.db.UpdateNodeConfig(ctx, &db.UpdateNodeConfigParams{
			ID:         nodeID,
			NodeConfig: sql.NullString{String: string(newConfig), Valid: true},
		}); err != nil {
			// Create error event
			if err := s.eventService.CreateEvent(ctx, nodeID, NodeEventError, map[string]interface{}{
				"node_id": nodeID,
				"name":    dbNode.Name,
				"action":  "environment_update",
				"error":   err.Error(),
			}); err != nil {
				s.logger.Error("Failed to create error event", "error", err)
			}
			return nil, fmt.Errorf("failed to update node config: %w", err)
		}

	case string(types.NodeTypeFabricOrderer):
		var ordererConfig types.FabricOrdererConfig
		if err := json.Unmarshal([]byte(dbNode.Config.String), &ordererConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal orderer config: %w", err)
		}
		ordererConfig.Env = req.Env
		newConfig, err := json.Marshal(ordererConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal updated orderer config: %w", err)
		}
		if _, err := s.db.UpdateNodeConfig(ctx, &db.UpdateNodeConfigParams{
			ID:         nodeID,
			NodeConfig: sql.NullString{String: string(newConfig), Valid: true},
		}); err != nil {
			// Create error event
			if err := s.eventService.CreateEvent(ctx, nodeID, NodeEventError, map[string]interface{}{
				"node_id": nodeID,
				"name":    dbNode.Name,
				"action":  "environment_update",
				"error":   err.Error(),
			}); err != nil {
				s.logger.Error("Failed to create error event", "error", err)
			}
			return nil, fmt.Errorf("failed to update node config: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported node type: %s", dbNode.NodeType.String)
	}

	// Create environment update completed event
	if err := s.eventService.CreateEvent(ctx, nodeID, NodeEventStarted, map[string]interface{}{
		"node_id": nodeID,
		"name":    dbNode.Name,
		"action":  "environment_update",
	}); err != nil {
		s.logger.Error("Failed to create environment update completed event", "error", err)
	}

	// Return the updated environment variables and indicate that a restart is required
	return &types.UpdateNodeEnvResponse{
		Env:             req.Env,
		RequiresRestart: true,
	}, nil
}

// GetNodeEnvironment retrieves the current environment variables for a node
func (s *NodeService) GetNodeEnvironment(ctx context.Context, nodeID int64) (map[string]string, error) {
	// Get the node from the database
	dbNode, err := s.db.GetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	// Get the node's current configuration
	switch dbNode.NodeType.String {
	case string(types.NodeTypeFabricPeer):
		var peerConfig types.FabricPeerConfig
		if err := json.Unmarshal([]byte(dbNode.Config.String), &peerConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal peer config: %w", err)
		}
		return peerConfig.Env, nil

	case string(types.NodeTypeFabricOrderer):
		var ordererConfig types.FabricOrdererConfig
		if err := json.Unmarshal([]byte(dbNode.Config.String), &ordererConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal orderer config: %w", err)
		}
		return ordererConfig.Env, nil

	default:
		return nil, fmt.Errorf("unsupported node type: %s", dbNode.NodeType.String)
	}
}

// GetExternalIP returns the external IP address of the node
func (s *NodeService) GetExternalIP() (string, error) {
	// Try to get external IP from environment variable first
	if externalIP := os.Getenv("EXTERNAL_IP"); externalIP != "" {
		return externalIP, nil
	}

	// Get local network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to get network interfaces: %w", err)
	}

	// Look for a suitable non-loopback interface with an IPv4 address
	for _, iface := range interfaces {
		// Skip loopback, down interfaces, and interfaces without addresses
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			// Check if this is an IP network address
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			// Skip loopback and IPv6 addresses
			ip := ipNet.IP.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Skip link-local addresses
			if ip[0] == 169 && ip[1] == 254 {
				continue
			}

			// Found a suitable IP address
			return ip.String(), nil
		}
	}

	// Fallback to localhost if no suitable interface is found
	return "127.0.0.1", nil
}

// validateNodeConnectivity validates that a node is working by checking its addresses
func (s *NodeService) validateNodeConnectivity(ctx context.Context, node *db.Node) error {
	s.logger.Info("Validating node connectivity", "node_id", node.ID, "name", node.Name)

	// Load node config
	nodeConfig, err := utils.LoadNodeConfig([]byte(node.NodeConfig.String))
	if err != nil {
		return fmt.Errorf("failed to load node config: %w", err)
	}

	// Load deployment config
	deploymentConfig, err := utils.DeserializeDeploymentConfig(node.DeploymentConfig.String)
	if err != nil {
		return fmt.Errorf("failed to deserialize deployment config: %w", err)
	}

	switch types.NodeType(node.NodeType.String) {
	case types.NodeTypeFabricPeer:
		return s.validateFabricPeerConnectivity(ctx, node, nodeConfig, deploymentConfig)
	case types.NodeTypeFabricOrderer:
		return s.validateFabricOrdererConnectivity(ctx, node, nodeConfig, deploymentConfig)
	case types.NodeTypeBesuFullnode:
		return s.validateBesuNodeConnectivity(ctx, node, nodeConfig, deploymentConfig)
	default:
		return fmt.Errorf("unsupported node type for connectivity validation: %s", node.NodeType.String)
	}
}

// validateFabricPeerConnectivity validates that a Fabric peer is working
func (s *NodeService) validateFabricPeerConnectivity(ctx context.Context, node *db.Node, nodeConfig types.NodeConfig, deploymentConfig types.NodeDeploymentConfig) error {
	peerConfig, ok := nodeConfig.(*types.FabricPeerConfig)
	if !ok {
		return fmt.Errorf("failed to assert node config to FabricPeerConfig")
	}

	s.logger.Info("Validating Fabric peer connectivity",
		"node_id", node.ID,
		"listen_address", peerConfig.ListenAddress,
		"operations_address", peerConfig.OperationsListenAddress)

	// Validate listen address
	if err := s.validateHTTPConnection(ctx, peerConfig.ExternalEndpoint, "peer listen"); err != nil {
		return fmt.Errorf("peer listen address validation failed: %w", err)
	}

	s.logger.Info("Fabric peer connectivity validation successful", "node_id", node.ID)
	return nil
}

// validateFabricOrdererConnectivity validates that a Fabric orderer is working
func (s *NodeService) validateFabricOrdererConnectivity(ctx context.Context, node *db.Node, nodeConfig types.NodeConfig, deploymentConfig types.NodeDeploymentConfig) error {
	ordererConfig, ok := nodeConfig.(*types.FabricOrdererConfig)
	if !ok {
		return fmt.Errorf("failed to assert node config to FabricOrdererConfig")
	}

	s.logger.Info("Validating Fabric orderer connectivity",
		"node_id", node.ID,
		"listen_address", ordererConfig.ListenAddress,
		"admin_address", ordererConfig.AdminAddress,
		"operations_address", ordererConfig.OperationsListenAddress)

	// Validate listen address
	if err := s.validateHTTPConnection(ctx, ordererConfig.ExternalEndpoint, "orderer listen"); err != nil {
		return fmt.Errorf("orderer listen address validation failed: %w", err)
	}

	s.logger.Info("Fabric orderer connectivity validation successful", "node_id", node.ID)
	return nil
}

// validateBesuNodeConnectivity validates that a Besu node is working
func (s *NodeService) validateBesuNodeConnectivity(ctx context.Context, node *db.Node, nodeConfig types.NodeConfig, deploymentConfig types.NodeDeploymentConfig) error {
	besuConfig, ok := nodeConfig.(*types.BesuNodeConfig)
	if !ok {
		return fmt.Errorf("failed to assert node config to BesuNodeConfig")
	}

	s.logger.Info("Validating Besu node connectivity",
		"node_id", node.ID,
		"p2p_host", besuConfig.P2PHost,
		"p2p_port", besuConfig.P2PPort,
		"rpc_host", besuConfig.RPCHost,
		"rpc_port", besuConfig.RPCPort)

	// Validate P2P address
	p2pAddress := fmt.Sprintf("%s:%d", besuConfig.P2PHost, besuConfig.P2PPort)
	if err := s.validateHTTPConnection(ctx, p2pAddress, "besu P2P"); err != nil {
		return fmt.Errorf("besu P2P address validation failed: %w", err)
	}

	// Validate RPC address
	rpcAddress := fmt.Sprintf("%s:%d", besuConfig.RPCHost, besuConfig.RPCPort)
	if err := s.validateHTTPConnection(ctx, rpcAddress, "besu RPC"); err != nil {
		return fmt.Errorf("besu RPC address validation failed: %w", err)
	}

	s.logger.Info("Besu node connectivity validation successful", "node_id", node.ID)
	return nil
}

// validateAddressAvailability checks if an address is available for binding
func (s *NodeService) validateAddressAvailability(address, addressType string) error {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid %s address format %s: %w", addressType, address, err)
	}

	// Replace 0.0.0.0 with localhost for binding check
	if host == "0.0.0.0" {
		host = "localhost"
	}

	// Validate port
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid %s port number %s: %w", addressType, portStr, err)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s port number %d out of range (1-65535)", addressType, port)
	}

	// Check if port is available for binding
	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("%s address %s is not available: %w", addressType, addr, err)
	}
	listener.Close()

	s.logger.Debug("Address validation successful", "address_type", addressType, "address", addr)
	return nil
}

// validateGRPCConnection attempts to establish a gRPC connection to validate the service is working
func (s *NodeService) validateGRPCConnection(ctx context.Context, address, serviceType string) error {
	// Replace 0.0.0.0 with localhost for connection check
	host, portStr, err := net.SplitHostPort(address)
	if err == nil && host == "0.0.0.0" {
		address = fmt.Sprintf("localhost:%s", portStr)
	}
	// Try to establish a TCP connection first
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return fmt.Errorf("failed to establish TCP connection to %s at %s: %w", serviceType, address, err)
	}
	defer conn.Close()

	s.logger.Debug("gRPC connection validation successful", "service_type", serviceType, "address", address)
	return nil
}

// validateHTTPConnection attempts to establish an HTTP connection to validate the service is working
func (s *NodeService) validateHTTPConnection(ctx context.Context, address, serviceType string) error {
	// Replace 0.0.0.0 with localhost for connection check
	host, portStr, err := net.SplitHostPort(address)
	if err == nil && host == "0.0.0.0" {
		address = fmt.Sprintf("localhost:%s", portStr)
	}
	// Try to establish a TCP connection
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return fmt.Errorf("failed to establish TCP connection to %s at %s: %w", serviceType, address, err)
	}
	defer conn.Close()

	s.logger.Debug("HTTP connection validation successful", "service_type", serviceType, "address", address)
	return nil
}

// ValidateNodeConnectivity validates that an existing node is working by checking its addresses
func (s *NodeService) ValidateNodeConnectivity(ctx context.Context, nodeID int64) error {
	// Get the node from database
	node, err := s.db.GetNode(ctx, nodeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.NewNotFoundError("node not found", map[string]interface{}{
				"id": nodeID,
			})
		}
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Check if node is running
	if types.NodeStatus(node.Status) != types.NodeStatusRunning {
		return fmt.Errorf("node is not running (status: %s)", node.Status)
	}

	// Validate node connectivity
	return s.validateNodeConnectivity(ctx, node)
}

// validateNodeConnectivityWithRetry validates node connectivity with exponential backoff retry
func (s *NodeService) validateNodeConnectivityWithRetry(ctx context.Context, node *db.Node, timeout time.Duration) error {
	startTime := time.Now()
	initialDelay := 500 * time.Millisecond
	maxDelay := 5 * time.Second
	currentDelay := initialDelay

	s.logger.Info("Starting node connectivity validation with retry",
		"node_id", node.ID,
		"name", node.Name,
		"timeout", timeout)

	for {
		// Check if we've exceeded the timeout
		if time.Since(startTime) > timeout {
			return fmt.Errorf("node connectivity validation timed out after %v", timeout)
		}

		// Try to validate connectivity
		if err := s.validateNodeConnectivity(ctx, node); err == nil {
			s.logger.Info("Node connectivity validation successful",
				"node_id", node.ID,
				"name", node.Name,
				"duration", time.Since(startTime))
			return nil
		} else {
			s.logger.Debug("Node connectivity validation attempt failed, retrying",
				"node_id", node.ID,
				"name", node.Name,
				"error", err,
				"delay", currentDelay)
		}

		// Wait before retry with exponential backoff
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during node connectivity validation: %w", ctx.Err())
		case <-time.After(currentDelay):
			// Continue to next iteration
		}

		// Increase delay with exponential backoff, but cap it
		currentDelay = time.Duration(float64(currentDelay) * 1.5)
		if currentDelay > maxDelay {
			currentDelay = maxDelay
		}
	}
}
