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
	FabricPeer          *FabricPeerProperties          `json:"fabricPeer,omitempty"`
	FabricOrderer       *FabricOrdererProperties       `json:"fabricOrderer,omitempty"`
	BesuNode            *BesuNodeProperties             `json:"besuNode,omitempty"`
	FabricXOrdererGroup *FabricXOrdererGroupProperties  `json:"fabricXOrdererGroup,omitempty"`
	FabricXCommitter    *FabricXCommitterProperties     `json:"fabricXCommitter,omitempty"`
	// FabricXChild is set on per-role child node rows (router/batcher/...
	// and sidecar/coordinator/...) so the UI can render a single
	// metricsUrl + container info directly off the leaf node.
	FabricXChild *FabricXChildProperties `json:"fabricXChild,omitempty"`
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
	// Key information
	KeyAddress string `json:"keyAddress,omitempty"`
	PublicKey  string `json:"publicKey,omitempty"`
}

// FabricXOrdererGroupProperties represents the properties specific to a Fabric X orderer group
type FabricXOrdererGroupProperties struct {
	MSPID          string `json:"mspId"`
	OrganizationID int64  `json:"organizationId"`
	PartyID        int    `json:"partyId"`
	ExternalIP     string `json:"externalIp"`
	RouterPort     int    `json:"routerPort"`
	BatcherPort    int    `json:"batcherPort"`
	ConsenterPort  int    `json:"consenterPort"`
	AssemblerPort  int    `json:"assemblerPort"`
	// Per-role Prometheus /metrics host ports. Zero means "not allocated"
	// (legacy nodes that predate monitoring support).
	RouterMonitoringPort    int `json:"routerMonitoringPort,omitempty"`
	BatcherMonitoringPort   int `json:"batcherMonitoringPort,omitempty"`
	ConsenterMonitoringPort int `json:"consenterMonitoringPort,omitempty"`
	AssemblerMonitoringPort int `json:"assemblerMonitoringPort,omitempty"`
	// Pre-rendered URLs for the UI: http://<externalIp>:<port>/metrics
	RouterMetricsUrl    string `json:"routerMetricsUrl,omitempty"`
	BatcherMetricsUrl   string `json:"batcherMetricsUrl,omitempty"`
	ConsenterMetricsUrl string `json:"consenterMetricsUrl,omitempty"`
	AssemblerMetricsUrl string `json:"assemblerMetricsUrl,omitempty"`
	Version             string `json:"version"`
	SignCert            string `json:"signCert,omitempty"`
	TLSCert             string `json:"tlsCert,omitempty"`
	CACert              string `json:"caCert,omitempty"`
	TLSCACert           string `json:"tlsCaCert,omitempty"`
}

// FabricXCommitterProperties represents the properties specific to a Fabric X committer
type FabricXCommitterProperties struct {
	MSPID            string `json:"mspId"`
	OrganizationID   int64  `json:"organizationId"`
	PartyID          int    `json:"partyId"`
	ExternalIP       string `json:"externalIp"`
	SidecarPort      int    `json:"sidecarPort"`
	CoordinatorPort  int    `json:"coordinatorPort"`
	ValidatorPort    int    `json:"validatorPort"`
	VerifierPort     int    `json:"verifierPort"`
	QueryServicePort int    `json:"queryServicePort"`
	// Per-role Prometheus /metrics host ports.
	SidecarMonitoringPort      int `json:"sidecarMonitoringPort,omitempty"`
	CoordinatorMonitoringPort  int `json:"coordinatorMonitoringPort,omitempty"`
	ValidatorMonitoringPort    int `json:"validatorMonitoringPort,omitempty"`
	VerifierMonitoringPort     int `json:"verifierMonitoringPort,omitempty"`
	QueryServiceMonitoringPort int `json:"queryServiceMonitoringPort,omitempty"`
	// Pre-rendered URLs.
	SidecarMetricsUrl      string `json:"sidecarMetricsUrl,omitempty"`
	CoordinatorMetricsUrl  string `json:"coordinatorMetricsUrl,omitempty"`
	ValidatorMetricsUrl    string `json:"validatorMetricsUrl,omitempty"`
	VerifierMetricsUrl     string `json:"verifierMetricsUrl,omitempty"`
	QueryServiceMetricsUrl string `json:"queryServiceMetricsUrl,omitempty"`
	Version                string `json:"version"`
}

// FabricXChildProperties is set on per-role child node rows
// (FABRICX_*_ROUTER/BATCHER/.../QUERY_SERVICE) so the UI can render a
// single metricsUrl directly off the leaf node without picking from
// the parent group.
type FabricXChildProperties struct {
	Role           string `json:"role"`
	ContainerName  string `json:"containerName,omitempty"`
	HostPort       int    `json:"hostPort,omitempty"`
	MonitoringPort int    `json:"monitoringPort,omitempty"`
	MetricsUrl     string `json:"metricsUrl,omitempty"`
}
