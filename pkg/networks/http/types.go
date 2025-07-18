package http

import (
	"encoding/json"

	networksservice "github.com/chainlaunch/chainlaunch/pkg/networks/service"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/fabric/block"
)

// ListNetworksResponse represents the response for listing networks
type ListNetworksResponse struct {
	Networks []NetworkResponse `json:"networks"`
	Total    int64             `json:"total"`
}

// NetworkResponse represents a network in HTTP responses

// Update NetworkResponse to include genesis block
type NetworkResponse struct {
	ID                 int64           `json:"id"`
	Name               string          `json:"name"`
	Platform           string          `json:"platform"`
	Status             string          `json:"status"`
	Description        string          `json:"description,omitempty"`
	Config             json.RawMessage `json:"config,omitempty"`
	DeploymentConfig   json.RawMessage `json:"deploymentConfig,omitempty"`
	ExposedPorts       json.RawMessage `json:"exposedPorts,omitempty"`
	GenesisBlock       string          `json:"genesisBlock,omitempty"`
	CurrentConfigBlock string          `json:"currentConfigBlock,omitempty"`
	Domain             string          `json:"domain,omitempty"`
	CreatedAt          string          `json:"createdAt"`
	CreatedBy          *int64          `json:"createdBy,omitempty"`
	UpdatedAt          *string         `json:"updatedAt,omitempty"`
}

// CreateFabricNetworkRequest represents the request to create a new Fabric network
type CreateFabricNetworkRequest struct {
	Name        string              `json:"name" validate:"required"`
	Description string              `json:"description"`
	Config      FabricNetworkConfig `json:"config" validate:"required"`
}

// ChannelConfigResponse represents the response for channel configuration
type ChannelConfigResponse struct {
	Name          string                 `json:"name"`
	ChannelConfig map[string]interface{} `json:"config"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// FabricPolicy represents a policy for the HTTP API
// This matches configtx.Policy (Type, Rule)
type FabricPolicy struct {
	Type string `json:"type"`
	Rule string `json:"rule"`
}

// FabricNetworkConfig represents the configuration for a Fabric network
type FabricNetworkConfig struct {
	PeerOrganizations    []OrganizationConfig `json:"peerOrganizations"`
	OrdererOrganizations []OrganizationConfig `json:"ordererOrganizations"`
	// Optional policies
	ApplicationPolicies map[string]FabricPolicy `json:"applicationPolicies,omitempty"`
	OrdererPolicies     map[string]FabricPolicy `json:"ordererPolicies,omitempty"`
	ChannelPolicies     map[string]FabricPolicy `json:"channelPolicies,omitempty"`
}

// OrganizationConfig represents an organization in the network
type OrganizationConfig struct {
	ID      int64   `json:"id" validate:"required"`
	NodeIDs []int64 `json:"nodeIds" validate:"required,min=1"`
}

// ConsenterConfig represents a consenter node configuration
type ConsenterConfig struct {
	ID string `json:"id" validate:"required"`
}

// GetNetworkNodesResponse represents the response for getting network nodes
type GetNetworkNodesResponse struct {
	Nodes []networksservice.NetworkNode `json:"nodes"`
}

// AddNodeToNetworkRequest represents the request to add a node to a network
type AddNodeToNetworkRequest struct {
	NodeID int64  `json:"nodeId" validate:"required"`
	Role   string `json:"role" validate:"required,oneof=peer orderer"`
}

// AnchorPeer represents a peer that will be set as anchor for an organization
type AnchorPeer struct {
	Host string `json:"host" validate:"required"`
	Port int    `json:"port" validate:"required"`
}

// SetAnchorPeersRequest represents the request to set anchor peers for an organization
type SetAnchorPeersRequest struct {
	OrganizationID int64        `json:"organizationId" validate:"required"`
	AnchorPeers    []AnchorPeer `json:"anchorPeers" validate:"required,min=0"`
}

// SetAnchorPeersResponse represents the response after setting anchor peers
type SetAnchorPeersResponse struct {
	TransactionID string `json:"transactionId"`
}

type ImportNetworkRequest struct {
	NetworkType string          `json:"networkType" validate:"required,oneof=fabric besu"`
	GenesisFile json.RawMessage `json:"genesisFile" validate:"required"`
}

type ImportNetworkResponse struct {
	NetworkID string `json:"networkId"`
	Message   string `json:"message"`
}

// ImportFabricNetworkRequest represents the request to import a Fabric network
type ImportFabricNetworkRequest struct {
	GenesisFile string `json:"genesisFile" validate:"required"`
	Description string `json:"description"`
}

// ImportFabricNetworkRequest represents the request to import a Fabric network
type ImportFabricNetworkWithOrgRequest struct {
	ChannelID      string `json:"channelId" validate:"required"`
	OrganizationID int64  `json:"organizationId" validate:"required"`
	OrdererURL     string `json:"ordererUrl" validate:"required"`
	OrdererTLSCert string `json:"ordererTlsCert" validate:"required"`
	Description    string `json:"description"`
}

// ImportBesuNetworkRequest represents the request to import a Besu network
type ImportBesuNetworkRequest struct {
	GenesisFile string `json:"genesisFile" validate:"required"`
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
	ChainID     int64  `json:"chainId" validate:"required"`
}

// BlockListResponse represents the response for listing blocks
type BlockListResponse struct {
	Blocks []block.Block `json:"blocks"`
	Total  int64         `json:"total"`
}

// BlockTransactionsResponse represents the response for listing transactions in a block
type BlockTransactionsResponse struct {
	Block *block.Block `json:"block"`
}

// TransactionResponse represents the response for getting a single transaction
type TransactionResponse struct {
	Block *block.Block `json:"block"`
}
