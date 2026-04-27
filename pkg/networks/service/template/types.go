package template

import (
	"time"
)

// Template version
const (
	TemplateVersion = "2.0.0"
)

// NetworkTemplate is the exported network configuration template
type NetworkTemplate struct {
	Version      string             `json:"version"`                // Schema version
	ExportedAt   string             `json:"exportedAt"`             // ISO timestamp
	ExportedFrom string             `json:"exportedFrom,omitempty"` // Source instance identifier
	Variables    []TemplateVariable `json:"variables,omitempty"`    // Variable declarations
	Network      NetworkDefinition  `json:"network"`
	Chaincodes   []ChaincodeTemplate `json:"chaincodes,omitempty"` // Chaincode/smart contract definitions
}

// NetworkDefinition contains the network configuration
type NetworkDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Platform    string                 `json:"platform"` // "fabric" or "besu"
	Fabric      *FabricNetworkTemplate `json:"fabric,omitempty"`
	Besu        *BesuNetworkTemplate   `json:"besu,omitempty"`
}

// ChaincodeTemplate represents a chaincode/smart contract definition in a template
type ChaincodeTemplate struct {
	Name              string            `json:"name"`
	Platform          string            `json:"platform"` // "fabric" or "besu"
	Fabric            *FabricChaincodeTemplate `json:"fabric,omitempty"`
	Besu              *BesuContractTemplate    `json:"besu,omitempty"`
}

// FabricChaincodeTemplate represents a Fabric chaincode definition
type FabricChaincodeTemplate struct {
	Version           string `json:"version"`
	Sequence          int64  `json:"sequence"`
	DockerImage       string `json:"dockerImage"`
	EndorsementPolicy string `json:"endorsementPolicy,omitempty"`
	ChaincodeAddress  string `json:"chaincodeAddress,omitempty"`
}

// BesuContractTemplate represents a Besu smart contract definition
type BesuContractTemplate struct {
	ABI      string `json:"abi"`               // Contract ABI JSON
	Bytecode string `json:"bytecode"`          // Compiled bytecode (hex)
	// Constructor args are left for the user to provide at import time
}

// FabricNetworkTemplate contains Fabric-specific network configuration
type FabricNetworkTemplate struct {
	ChannelName             string                    `json:"channelName"`
	ConsensusType           string                    `json:"consensusType"` // "etcdraft" | "smartbft"
	BatchSize               *BatchSizeTemplate        `json:"batchSize,omitempty"`
	BatchTimeout            string                    `json:"batchTimeout,omitempty"`
	EtcdRaftOptions         *EtcdRaftOptionsTemplate  `json:"etcdRaftOptions,omitempty"`
	SmartBFTOptions         *SmartBFTOptionsTemplate  `json:"smartBFTOptions,omitempty"`
	ChannelCapabilities     []string                  `json:"channelCapabilities,omitempty"`
	ApplicationCapabilities []string                  `json:"applicationCapabilities,omitempty"`
	OrdererCapabilities     []string                  `json:"ordererCapabilities,omitempty"`
	ApplicationPolicies     map[string]PolicyTemplate `json:"applicationPolicies,omitempty"`
	OrdererPolicies         map[string]PolicyTemplate `json:"ordererPolicies,omitempty"`
	ChannelPolicies         map[string]PolicyTemplate `json:"channelPolicies,omitempty"`
	PeerOrgRefs             []OrganizationRef         `json:"peerOrgRefs,omitempty"`
	OrdererOrgRefs          []OrganizationRef         `json:"ordererOrgRefs,omitempty"`
}

// BesuNetworkTemplate contains Besu-specific network configuration
type BesuNetworkTemplate struct {
	Consensus      string                    `json:"consensus"`      // "qbft"
	ChainID        int64                     `json:"chainId"`
	BlockPeriod    int                       `json:"blockPeriod"`
	EpochLength    int                       `json:"epochLength"`
	RequestTimeout int                       `json:"requestTimeout"`
	GasLimit       string                    `json:"gasLimit,omitempty"`
	Difficulty     string                    `json:"difficulty,omitempty"`
	Alloc          map[string]AllocTemplate  `json:"alloc,omitempty"`
	ValidatorRefs  []ValidatorRef            `json:"validatorRefs,omitempty"`
}

// AllocTemplate represents an initial account balance
type AllocTemplate struct {
	Balance string `json:"balance"`
}

// ValidatorRef references a validator key by variable name
type ValidatorRef struct {
	VariableRef string `json:"variableRef"`
}

// OrganizationRef references an organization by variable name
type OrganizationRef struct {
	VariableRef string    `json:"variableRef"`
	NodeRefs    []NodeRef `json:"nodeRefs,omitempty"`
}

// NodeRef references a node by variable name
type NodeRef struct {
	VariableRef string `json:"variableRef"`
}

// PolicyTemplate represents a policy in the template
type PolicyTemplate struct {
	Type string `json:"type"` // "ImplicitMeta" | "Signature"
	Rule string `json:"rule"`
}

// BatchSizeTemplate represents batch size configuration
type BatchSizeTemplate struct {
	MaxMessageCount   uint32 `json:"maxMessageCount"`
	AbsoluteMaxBytes  uint32 `json:"absoluteMaxBytes"`
	PreferredMaxBytes uint32 `json:"preferredMaxBytes"`
}

// EtcdRaftOptionsTemplate represents etcdraft configuration options
type EtcdRaftOptionsTemplate struct {
	TickInterval         string `json:"tickInterval"`
	ElectionTick         uint32 `json:"electionTick"`
	HeartbeatTick        uint32 `json:"heartbeatTick"`
	MaxInflightBlocks    uint32 `json:"maxInflightBlocks"`
	SnapshotIntervalSize uint32 `json:"snapshotIntervalSize"`
}

// SmartBFTOptionsTemplate represents SmartBFT configuration options
type SmartBFTOptionsTemplate struct {
	RequestBatchMaxCount      uint64 `json:"requestBatchMaxCount"`
	RequestBatchMaxBytes      uint64 `json:"requestBatchMaxBytes"`
	RequestBatchMaxInterval   string `json:"requestBatchMaxInterval"`
	IncomingMessageBufferSize uint64 `json:"incomingMessageBufferSize"`
	RequestPoolSize           uint64 `json:"requestPoolSize"`
	RequestForwardTimeout     string `json:"requestForwardTimeout"`
	RequestComplainTimeout    string `json:"requestComplainTimeout"`
	RequestAutoRemoveTimeout  string `json:"requestAutoRemoveTimeout"`
	RequestMaxBytes           uint64 `json:"requestMaxBytes"`
	ViewChangeResendInterval  string `json:"viewChangeResendInterval"`
	ViewChangeTimeout         string `json:"viewChangeTimeout"`
	LeaderHeartbeatTimeout    string `json:"leaderHeartbeatTimeout"`
	LeaderHeartbeatCount      uint64 `json:"leaderHeartbeatCount"`
	CollectTimeout            string `json:"collectTimeout"`
	SyncOnStart               bool   `json:"syncOnStart"`
	SpeedUpViewChange         bool   `json:"speedUpViewChange"`
	LeaderRotation            string `json:"leaderRotation"`
	DecisionsPerLeader        uint64 `json:"decisionsPerLeader"`
}

// --- Import Request Types ---

// ImportOverrides contains overridable parameters for import
type ImportOverrides struct {
	NetworkName *string `json:"networkName,omitempty"`
	ChannelName *string `json:"channelName,omitempty"`
	Description *string `json:"description,omitempty"`
}

// ValidateTemplateRequest is the request to validate a template import
type ValidateTemplateRequest struct {
	Template         NetworkTemplate   `json:"template"`
	Overrides        ImportOverrides   `json:"overrides,omitempty"`
	VariableBindings []VariableBinding `json:"variableBindings,omitempty"`
}

// ImportTemplateRequest is the request to import a network from a template
type ImportTemplateRequest struct {
	Template         NetworkTemplate   `json:"template"`
	Overrides        ImportOverrides   `json:"overrides,omitempty"`
	VariableBindings []VariableBinding `json:"variableBindings,omitempty"`
}

// --- Validation Response Types ---

// ValidationError represents an error during validation
type ValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// ValidationWarning represents a warning during validation
type ValidationWarning struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

// ValidateTemplateResponse is the response from validating a template import
type ValidateTemplateResponse struct {
	Valid    bool                `json:"valid"`
	Errors   []ValidationError   `json:"errors"`
	Warnings []ValidationWarning `json:"warnings"`
	Preview  *ImportPreview      `json:"preview,omitempty"`
}

// ImportPreview shows what would be created/used during import
type ImportPreview struct {
	Network               NetworkPreview        `json:"network"`
	OrganizationsToCreate []OrgPreview          `json:"organizationsToCreate"`
	NodesToCreate         []NodePreview         `json:"nodesToCreate"`
	ExistingOrgsUsed      []ExistingOrgPreview  `json:"existingOrgsUsed"`
	ExistingNodesUsed     []ExistingNodePreview `json:"existingNodesUsed"`
	Chaincodes            []ChaincodePreview    `json:"chaincodes,omitempty"`
}

// NetworkPreview shows the network that would be created
type NetworkPreview struct {
	Name        string `json:"name"`
	ChannelName string `json:"channelName,omitempty"`
	Description string `json:"description,omitempty"`
	Platform    string `json:"platform"`
}

// OrgPreview shows an organization that would be used
type OrgPreview struct {
	TemplateID   string `json:"templateId"`
	MspID        string `json:"mspId"`
	Name         string `json:"name"`
	ProviderID   int64  `json:"providerId"`
	ProviderName string `json:"providerName,omitempty"`
}

// NodePreview shows a node that would be created
type NodePreview struct {
	TemplateID string `json:"templateId"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	OrgMspID   string `json:"orgMspId"`
}

// ExistingOrgPreview shows an existing organization that would be used
type ExistingOrgPreview struct {
	TemplateID string `json:"templateId"`
	OrgID      int64  `json:"orgId"`
	MspID      string `json:"mspId"`
}

// ExistingNodePreview shows an existing node that would be used
type ExistingNodePreview struct {
	TemplateID string `json:"templateId"`
	NodeID     int64  `json:"nodeId"`
	Name       string `json:"name"`
	Type       string `json:"type"`
}

// ChaincodePreview shows a chaincode that would be deployed
type ChaincodePreview struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Version  string `json:"version,omitempty"`
}

// --- Import Response Types ---

// ImportTemplateResponse is the response from importing a network from a template
type ImportTemplateResponse struct {
	NetworkID            int64         `json:"networkId"`
	NetworkName          string        `json:"networkName"`
	CreatedOrganizations []CreatedOrg  `json:"createdOrganizations"`
	CreatedNodes         []CreatedNode `json:"createdNodes"`
	CreatedChaincodes    []CreatedChaincode `json:"createdChaincodes,omitempty"`
	Message              string        `json:"message"`
}

// CreatedOrg represents an organization that was created during import
type CreatedOrg struct {
	TemplateID string `json:"templateId"`
	OrgID      int64  `json:"orgId"`
	MspID      string `json:"mspId"`
}

// CreatedNode represents a node that was created during import
type CreatedNode struct {
	TemplateID string `json:"templateId"`
	NodeID     int64  `json:"nodeId"`
	Name       string `json:"name"`
	Type       string `json:"type"`
}

// CreatedChaincode represents a chaincode created during import
type CreatedChaincode struct {
	Name        string `json:"name"`
	ChaincodeID int64  `json:"chaincodeId"`
	Platform    string `json:"platform"`
}

// --- Helper Functions ---

// NewNetworkTemplate creates a new network template with default values
func NewNetworkTemplate() *NetworkTemplate {
	return &NetworkTemplate{
		Version:    TemplateVersion,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// ExportTemplateResponse is the HTTP response for exporting a template
type ExportTemplateResponse struct {
	Template NetworkTemplate `json:"template"`
}
