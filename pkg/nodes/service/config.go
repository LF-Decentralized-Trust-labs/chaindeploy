package service

import (
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// Node represents a node with its full configuration
type Node struct {
	ID                 int64                      `json:"id"`
	Name               string                     `json:"name"`
	BlockchainPlatform types.BlockchainPlatform   `json:"platform"`
	NodeType           types.NodeType             `json:"nodeType"`
	Status             types.NodeStatus           `json:"status"`
	ErrorMessage       string                     `json:"errorMessage"`
	Endpoint           string                     `json:"endpoint"`
	PublicEndpoint     string                     `json:"publicEndpoint"`
	NodeConfig         types.NodeConfig           `json:"nodeConfig"`
	DeploymentConfig   types.NodeDeploymentConfig `json:"deploymentConfig"`
	MSPID              string                     `json:"mspId"`
	CreatedAt          time.Time                  `json:"createdAt"`
	UpdatedAt          time.Time                  `json:"updatedAt"`
}

// Add PaginatedNodes type
type PaginatedNodes struct {
	Items       []NodeResponse
	Total       int64
	Page        int
	PageCount   int
	HasNextPage bool
}

// NodeResponse represents the response for node configuration
type NodeResponse struct {
	ID           int64          `json:"id"`
	Name         string         `json:"name"`
	Platform     string         `json:"platform"`
	Status       string         `json:"status"`
	ErrorMessage string         `json:"errorMessage"`
	NodeType     types.NodeType `json:"nodeType"`
	Endpoint     string         `json:"endpoint"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`

	// Type-specific fields
	FabricPeer    *FabricPeerProperties    `json:"fabricPeer,omitempty"`
	FabricOrderer *FabricOrdererProperties `json:"fabricOrderer,omitempty"`
	BesuNode      *BesuNodeProperties      `json:"besuNode,omitempty"`
}

// FabricPeerProperties represents the properties specific to a Fabric peer node
type FabricPeerProperties struct {
	MSPID             string `json:"mspId"`
	OrganizationID    int64  `json:"organizationId"`
	ExternalEndpoint  string `json:"externalEndpoint"`
	ChaincodeAddress  string `json:"chaincodeAddress"`
	EventsAddress     string `json:"eventsAddress"`
	OperationsAddress string `json:"operationsAddress"`
	// Add deployment config fields
	SignKeyID     int64    `json:"signKeyId"`
	TLSKeyID      int64    `json:"tlsKeyId"`
	ListenAddress string   `json:"listenAddress"`
	DomainNames   []string `json:"domainNames"`
	Mode          string   `json:"mode"`
	// Add certificate information
	SignCert   string `json:"signCert,omitempty"`
	TLSCert    string `json:"tlsCert,omitempty"`
	SignCACert string `json:"signCaCert,omitempty"`
	TLSCACert  string `json:"tlsCaCert,omitempty"`

	AddressOverrides []types.AddressOverride `json:"addressOverrides,omitempty"`
	Version          string                  `json:"version"`
}

// FabricOrdererProperties represents the properties specific to a Fabric orderer node
type FabricOrdererProperties struct {
	MSPID             string `json:"mspId"`
	OrganizationID    int64  `json:"organizationId"`
	ExternalEndpoint  string `json:"externalEndpoint"`
	AdminAddress      string `json:"adminAddress"`
	OperationsAddress string `json:"operationsAddress"`
	// Add deployment config fields
	SignKeyID     int64    `json:"signKeyId"`
	TLSKeyID      int64    `json:"tlsKeyId"`
	ListenAddress string   `json:"listenAddress"`
	DomainNames   []string `json:"domainNames"`
	Mode          string   `json:"mode"`
	// Add certificate information
	SignCert   string `json:"signCert,omitempty"`
	TLSCert    string `json:"tlsCert,omitempty"`
	SignCACert string `json:"signCaCert,omitempty"`
	TLSCACert  string `json:"tlsCaCert,omitempty"`
	Version    string `json:"version"`
}

// BesuNodeProperties represents the properties specific to a Besu node
type BesuNodeProperties struct {
	NetworkID  int64  `json:"networkId"`
	P2PPort    uint   `json:"p2pPort"`
	RPCPort    uint   `json:"rpcPort"`
	ExternalIP string `json:"externalIp"`
	InternalIP string `json:"internalIp"`
	EnodeURL   string `json:"enodeUrl"`
	// Add deployment config fields
	P2PHost   string   `json:"p2pHost"`
	RPCHost   string   `json:"rpcHost"`
	KeyID     int64    `json:"keyId"`
	Mode      string   `json:"mode"`
	Version   string   `json:"version"`
	BootNodes []string `json:"bootNodes"`
	// Metrics configuration
	MetricsEnabled  bool   `json:"metricsEnabled"`
	MetricsHost     string `json:"metricsHost"`
	MetricsPort     uint   `json:"metricsPort"`
	MetricsProtocol string `json:"metricsProtocol"`
}
