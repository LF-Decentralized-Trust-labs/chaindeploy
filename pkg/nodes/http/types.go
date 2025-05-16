package http

import (
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// NodeType represents the type of node
type NodeType string

const (
	NodeTypeFabricPeer    NodeType = "fabric-peer"
	NodeTypeFabricOrderer NodeType = "fabric-orderer"
	NodeTypeBesuFullNode  NodeType = "besu-fullnode"
	NodeTypeBesuValidator NodeType = "besu-validator"
	NodeTypeBesuBootNode  NodeType = "besu-bootnode"
)

// NodeMode represents the deployment mode
type NodeMode string

const (
	NodeModeService NodeMode = "service"
	NodeModeDocker  NodeMode = "docker"
)

type SuccessResponse struct {
	Success bool `json:"success"`
}

// BaseNodeConfig contains common fields for all node configurations
type BaseNodeConfig struct {
	Type NodeType `json:"type" validate:"required"`
	Mode NodeMode `json:"mode" validate:"required,oneof=service docker"`
}

// FabricPeerConfig represents the configuration for a Fabric peer node
type FabricPeerConfig struct {
	BaseNodeConfig
	Name                    string                  `json:"name" validate:"required"`
	OrganizationID          int64                   `json:"organizationId" validate:"required"`
	MSPID                   string                  `json:"mspId" validate:"required"`
	SignKeyID               int64                   `json:"signKeyId" validate:"required"`
	TLSKeyID                int64                   `json:"tlsKeyId" validate:"required"`
	ExternalEndpoint        string                  `json:"externalEndpoint" validate:"required"`
	ListenAddress           string                  `json:"listenAddress" validate:"required"`
	EventsAddress           string                  `json:"eventsAddress" validate:"required"`
	OperationsListenAddress string                  `json:"operationsListenAddress" validate:"required"`
	ChaincodeAddress        string                  `json:"chaincodeAddress" validate:"required"`
	DomainNames             []string                `json:"domainNames"`
	Env                     map[string]string       `json:"env"`
	Version                 string                  `json:"version"` // Fabric version to use
	AddressOverrides        []types.AddressOverride `json:"addressOverrides,omitempty"`
}

// FabricOrdererConfig represents the configuration for a Fabric orderer node
type FabricOrdererConfig struct {
	BaseNodeConfig
	Name                    string            `json:"name" validate:"required"`
	OrganizationID          int64             `json:"organizationId" validate:"required"`
	MSPID                   string            `json:"mspId" validate:"required"`
	SignKeyID               int64             `json:"signKeyId" validate:"required"`
	TLSKeyID                int64             `json:"tlsKeyId" validate:"required"`
	ExternalEndpoint        string            `json:"externalEndpoint" validate:"required"`
	ListenAddress           string            `json:"listenAddress" validate:"required"`
	AdminAddress            string            `json:"adminAddress" validate:"required"`
	OperationsListenAddress string            `json:"operationsListenAddress" validate:"required"`
	DomainNames             []string          `json:"domainNames"`
	Env                     map[string]string `json:"env"`
	Version                 string            `json:"version"` // Fabric version to use
}

// BesuNodeConfig represents the configuration for a Besu node
type BesuNodeConfig struct {
	BaseNodeConfig
	NetworkID   uint              `json:"networkId" validate:"required"`
	P2PPort     uint              `json:"p2pPort" validate:"required"`
	RPCPort     uint              `json:"rpcPort" validate:"required"`
	WSPort      uint              `json:"wsPort" validate:"required"`
	NodePrivKey string            `json:"nodePrivKey,omitempty"`
	Bootnodes   []string          `json:"bootnodes,omitempty"`
	ExternalIP  string            `json:"externalIp,omitempty"`
	IsBootnode  bool              `json:"isBootnode"`
	IsValidator bool              `json:"isValidator"`
	StaticNodes []string          `json:"staticNodes,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

// NodeConfigResponse represents the response for node configuration
type NodeConfigResponse struct {
	Type          NodeType             `json:"type"`
	Mode          NodeMode             `json:"mode"`
	FabricPeer    *FabricPeerConfig    `json:"fabricPeer,omitempty"`
	FabricOrderer *FabricOrdererConfig `json:"fabricOrderer,omitempty"`
	BesuNode      *BesuNodeConfig      `json:"besuNode,omitempty"`
}

// BesuNodeRequest represents the HTTP request for creating a Besu node
type BesuNodeRequest struct {
	NetworkID      uint              `json:"networkId" validate:"required"`
	P2PPort        uint              `json:"p2pPort" validate:"required"`
	RPCPort        uint              `json:"rpcPort" validate:"required"`
	WSPort         uint              `json:"wsPort" validate:"required"`
	NodePrivKey    string            `json:"nodePrivKey,omitempty"`
	Bootnodes      []string          `json:"bootnodes,omitempty"`
	ExternalIP     string            `json:"externalIp,omitempty"`
	IsBootnode     bool              `json:"isBootnode"`
	IsValidator    bool              `json:"isValidator"`
	StaticNodes    []string          `json:"staticNodes,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	MetricsEnabled bool              `json:"metricsEnabled"`
	MetricsPort    int64             `json:"metricsPort"`
}

// FabricPeerRequest represents the HTTP request for creating a Fabric peer node
type FabricPeerRequest struct {
	Name                    string            `json:"name" validate:"required"`
	OrganizationID          int64             `json:"organizationId" validate:"required"`
	Mode                    string            `json:"mode" validate:"required,oneof=service docker"`
	ExternalEndpoint        string            `json:"externalEndpoint" validate:"required"`
	ListenAddress           string            `json:"listenAddress" validate:"required"`
	EventsAddress           string            `json:"eventsAddress" validate:"required"`
	OperationsListenAddress string            `json:"operationsListenAddress" validate:"required"`
	ChaincodeAddress        string            `json:"chaincodeAddress" validate:"required"`
	DomainNames             []string          `json:"domainNames" validate:"required"`
	Env                     map[string]string `json:"env,omitempty"`
	MSPID                   string            `json:"mspId" validate:"required"`
	SignKeyID               int64             `json:"signKeyId" validate:"required"`
	TLSKeyID                int64             `json:"tlsKeyId" validate:"required"`
}

// FabricOrdererRequest represents the HTTP request for creating a Fabric orderer node
type FabricOrdererRequest struct {
	Name                    string            `json:"name" validate:"required"`
	OrganizationID          int64             `json:"organizationId" validate:"required"`
	Mode                    string            `json:"mode" validate:"required,oneof=service docker"`
	ExternalEndpoint        string            `json:"externalEndpoint" validate:"required"`
	ListenAddress           string            `json:"listenAddress" validate:"required"`
	AdminAddress            string            `json:"adminAddress" validate:"required"`
	OperationsListenAddress string            `json:"operationsListenAddress" validate:"required"`
	DomainNames             []string          `json:"domainNames" validate:"required"`
	Env                     map[string]string `json:"env,omitempty"`
	MSPID                   string            `json:"mspId" validate:"required"`
	SignKeyID               int64             `json:"signKeyId" validate:"required"`
	TLSKeyID                int64             `json:"tlsKeyId" validate:"required"`
}

// NodeResponse represents the HTTP response for any node type
type NodeResponse struct {
	ID                 int64                            `json:"id"`
	Name               string                           `json:"name"`
	BlockchainPlatform string                           `json:"platform"`
	NodeType           string                           `json:"nodeType"`
	Status             string                           `json:"status"`
	ErrorMessage       string                           `json:"errorMessage"`
	Endpoint           string                           `json:"endpoint"`
	CreatedAt          time.Time                        `json:"createdAt"`
	UpdatedAt          time.Time                        `json:"updatedAt"`
	FabricPeer         *service.FabricPeerProperties    `json:"fabricPeer,omitempty"`
	FabricOrderer      *service.FabricOrdererProperties `json:"fabricOrderer,omitempty"`
	BesuNode           *service.BesuNodeProperties      `json:"besuNode,omitempty"`
}

// ListNodesResponse represents the paginated response for listing nodes
type ListNodesResponse struct {
	Items       []NodeResponse `json:"items"`
	Total       int64          `json:"total"`
	Page        int            `json:"page"`
	PageCount   int            `json:"pageCount"`
	HasNextPage bool           `json:"hasNextPage"`
}

// AddressOverride represents an address override configuration for Fabric nodes
type AddressOverride struct {
	From      string `json:"from"`
	To        string `json:"to"`
	TLSCACert string `json:"tlsCACert"`
}

// UpdateNodeRequest represents the request body for updating a node
type UpdateNodeRequest struct {
	// Common fields
	Name               *string                   `json:"name,omitempty"`
	BlockchainPlatform *types.BlockchainPlatform `json:"blockchainPlatform,omitempty"`

	// Platform-specific configurations
	FabricPeer    *UpdateFabricPeerRequest    `json:"fabricPeer,omitempty"`
	FabricOrderer *UpdateFabricOrdererRequest `json:"fabricOrderer,omitempty"`
	BesuNode      *UpdateBesuNodeRequest      `json:"besuNode,omitempty"`
}

// UpdateFabricPeerRequest represents the configuration for updating a Fabric peer node
type UpdateFabricPeerRequest struct {
	ExternalEndpoint        *string                 `json:"externalEndpoint,omitempty"`
	ListenAddress           *string                 `json:"listenAddress,omitempty"`
	EventsAddress           *string                 `json:"eventsAddress,omitempty"`
	OperationsListenAddress *string                 `json:"operationsListenAddress,omitempty"`
	ChaincodeAddress        *string                 `json:"chaincodeAddress,omitempty"`
	DomainNames             []string                `json:"domainNames,omitempty"`
	Env                     map[string]string       `json:"env,omitempty"`
	AddressOverrides        []types.AddressOverride `json:"addressOverrides,omitempty"`
	Version                 *string                 `json:"version,omitempty"`
}

// UpdateFabricOrdererRequest represents the configuration for updating a Fabric orderer node
type UpdateFabricOrdererRequest struct {
	ExternalEndpoint        *string           `json:"externalEndpoint,omitempty"`
	ListenAddress           *string           `json:"listenAddress,omitempty"`
	AdminAddress            *string           `json:"adminAddress,omitempty"`
	OperationsListenAddress *string           `json:"operationsListenAddress,omitempty"`
	DomainNames             []string          `json:"domainNames,omitempty"`
	Env                     map[string]string `json:"env,omitempty"`
	Version                 *string           `json:"version,omitempty"`
}

// UpdateBesuNodeRequest represents the configuration for updating a Besu node
type UpdateBesuNodeRequest struct {
	NetworkID      uint              `json:"networkId" validate:"required"`
	P2PHost        string            `json:"p2pHost" validate:"required"`
	P2PPort        uint              `json:"p2pPort" validate:"required"`
	RPCHost        string            `json:"rpcHost" validate:"required"`
	RPCPort        uint              `json:"rpcPort" validate:"required"`
	Bootnodes      []string          `json:"bootnodes,omitempty"`
	ExternalIP     string            `json:"externalIp,omitempty"`
	InternalIP     string            `json:"internalIp,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	MetricsEnabled bool              `json:"metricsEnabled"`
	MetricsPort    int64             `json:"metricsPort"`
}

type BesuNodeDefaultsResponse struct {
	NodeCount int                        `json:"nodeCount"`
	Defaults  []service.BesuNodeDefaults `json:"defaults"`
}
