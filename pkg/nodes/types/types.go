package types

import "encoding/json"

type DeploymentMode string

const (
	DeploymentModeService DeploymentMode = "SERVICE"
	DeploymentModeDocker  DeploymentMode = "DOCKER"
)

// BlockchainPlatform represents the type of blockchain platform
type BlockchainPlatform string

const (
	PlatformFabric  BlockchainPlatform = "FABRIC"
	PlatformBesu    BlockchainPlatform = "BESU"
	PlatformFabricX BlockchainPlatform = "FABRICX"
)

// NodeType represents the type of node
type NodeType string

const (
	// Fabric node types
	NodeTypeFabricPeer    NodeType = "FABRIC_PEER"
	NodeTypeFabricOrderer NodeType = "FABRIC_ORDERER"

	// Besu node types
	NodeTypeBesuFullnode NodeType = "BESU_FULLNODE"

	// Fabric X node types.
	// Endorsement in Fabric X is handled by token-sdk-x, not by
	// chaindeploy-managed node types, so there is no FabricXEndorser.
	NodeTypeFabricXOrdererGroup NodeType = "FABRICX_ORDERER_GROUP"
	NodeTypeFabricXCommitter    NodeType = "FABRICX_COMMITTER"

	// Fabric X per-role child node types (members of a node_group).
	// Each child is exactly one container — restoring the
	// "1 nodes row = 1 runnable unit" invariant. See ADR 0001.
	NodeTypeFabricXOrdererRouter           NodeType = "FABRICX_ORDERER_ROUTER"
	NodeTypeFabricXOrdererBatcher          NodeType = "FABRICX_ORDERER_BATCHER"
	NodeTypeFabricXOrdererConsenter        NodeType = "FABRICX_ORDERER_CONSENTER"
	NodeTypeFabricXOrdererAssembler        NodeType = "FABRICX_ORDERER_ASSEMBLER"
	NodeTypeFabricXCommitterSidecar        NodeType = "FABRICX_COMMITTER_SIDECAR"
	NodeTypeFabricXCommitterCoordinator    NodeType = "FABRICX_COMMITTER_COORDINATOR"
	NodeTypeFabricXCommitterValidator      NodeType = "FABRICX_COMMITTER_VALIDATOR"
	NodeTypeFabricXCommitterVerifier       NodeType = "FABRICX_COMMITTER_VERIFIER"
	NodeTypeFabricXCommitterQueryService   NodeType = "FABRICX_COMMITTER_QUERY_SERVICE"
)

// FabricX component types for sub-container tracking
type FabricXComponentType string

const (
	FabricXComponentRouter       FabricXComponentType = "router"
	FabricXComponentBatcher      FabricXComponentType = "batcher"
	FabricXComponentConsenter    FabricXComponentType = "consenter"
	FabricXComponentAssembler    FabricXComponentType = "assembler"
	FabricXComponentSidecar      FabricXComponentType = "sidecar"
	FabricXComponentCoordinator  FabricXComponentType = "coordinator"
	FabricXComponentValidator    FabricXComponentType = "validator"
	FabricXComponentVerifier     FabricXComponentType = "verifier"
	FabricXComponentQueryService FabricXComponentType = "query-service"
)

// NodeStatus represents the status of a node
type NodeStatus string

const (
	NodeStatusPending  NodeStatus = "PENDING"
	NodeStatusRunning  NodeStatus = "RUNNING"
	NodeStatusStopped  NodeStatus = "STOPPED"
	NodeStatusStopping NodeStatus = "STOPPING"
	NodeStatusStarting NodeStatus = "STARTING"
	NodeStatusUpdating NodeStatus = "UPDATING"
	NodeStatusError    NodeStatus = "ERROR"
	// Aggregated status used for node_groups and services whose children
	// are partially running. Added by migration 0022.
	NodeStatusDegraded NodeStatus = "DEGRADED"
	// CREATED is the default DB status for newly-persisted node_groups
	// and services before any lifecycle action has run.
	NodeStatusCreated NodeStatus = "CREATED"
)

// StoredConfig represents the stored configuration with type information
type StoredConfig struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

type AddressOverride struct {
	From      string `json:"from"`
	To        string `json:"to"`
	TLSCACert string `json:"tlsCACert"`
}
