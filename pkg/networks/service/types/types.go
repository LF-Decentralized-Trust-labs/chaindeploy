package types

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-config/configtx"
)

// NetworkDeploymentStatus represents the current status of a network deployment
type NetworkDeploymentStatus struct {
	NetworkID int64  `json:"networkId"`
	Status    string `json:"status"` // creating, running, stopped, error
	Endpoint  string `json:"endpoint,omitempty"`
}

// NetworkConfig is an interface that all network configurations must implement
type NetworkConfig interface {
	Validate() error
	Type() string
}

// NetworkConfigType represents the type of network configuration
type NetworkConfigType string

const (
	NetworkTypeFabric NetworkConfigType = "fabric"
	NetworkTypeBesu   NetworkConfigType = "besu"
)

// BaseNetworkConfig contains common fields for all network configurations
type BaseNetworkConfig struct {
	Type NetworkConfigType `json:"type"`
}

// ConsenterRef represents a reference to a consenter node
type ConsenterRef struct {
	ID      string `json:"id"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	TLSCert string `json:"tlsCert"`
}

// ExternalNodeRef represents a reference to an external node
type ExternalNodeRef struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

// FabricNetworkConfig represents the configuration for a Fabric network

// FabricNetworkConfig represents the configuration for a Fabric network
type FabricNetworkConfig struct {
	BaseNetworkConfig    `json:",inline"`
	ChannelName          string                     `json:"channel_name"`
	PeerOrganizations    []Organization             `json:"peer_organizations"`
	OrdererOrganizations []Organization             `json:"orderer_organizations"`
	ApplicationPolicies  map[string]configtx.Policy `json:"application_policies,omitempty"`
	OrdererPolicies      map[string]configtx.Policy `json:"orderer_policies,omitempty"`
	ChannelPolicies      map[string]configtx.Policy `json:"channel_policies,omitempty"`

	// Consensus configuration
	ConsensusType      string              `json:"consensus_type,omitempty"` // "etcdraft" or "smartbft"
	SmartBFTConsenters []SmartBFTConsenter `json:"smartbft_consenters,omitempty"`
	SmartBFTOptions    *SmartBFTOptions    `json:"smartbft_options,omitempty"`
	EtcdRaftOptions    *EtcdRaftOptions    `json:"etcdraft_options,omitempty"`
	// Batch configuration
	BatchSize    *BatchSize `json:"batch_size,omitempty"`
	BatchTimeout string     `json:"batch_timeout,omitempty"` // e.g., "2s"

}

// Organization represents a Fabric organization configuration
type Organization struct {
	ID      int64   `json:"id"`
	NodeIDs []int64 `json:"nodeIds"`
}

// HostPort represents a network endpoint
type HostPort struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// Consenter represents a Fabric consenter configuration
type Consenter struct {
	Address       HostPort `json:"address"`
	ClientTLSCert string   `json:"clientTLSCert"`
	ServerTLSCert string   `json:"serverTLSCert"`
}

type BesuConsensusType string

const (
	BesuConsensusTypeQBFT BesuConsensusType = "qbft"
)

// AccountBalance represents the balance configuration for an account
type AccountBalance struct {
	Balance string `json:"balance"`
}

// BesuNetworkConfig represents the configuration for a Besu network
type BesuNetworkConfig struct {
	BaseNetworkConfig
	NetworkID              int64                     `json:"networkId"`
	ChainID                int64                     `json:"chainId"`
	Consensus              BesuConsensusType         `json:"consensus"`
	InitialValidatorKeyIds []int64                   `json:"initialValidators"`
	ExternalNodes          []ExternalNodeRef         `json:"externalNodes,omitempty"`
	BlockPeriod            int                       `json:"blockPeriod"`
	EpochLength            int                       `json:"epochLength"`
	RequestTimeout         int                       `json:"requestTimeout"`
	Nonce                  string                    `json:"nonce"`
	Timestamp              string                    `json:"timestamp"`
	GasLimit               string                    `json:"gasLimit"`
	Difficulty             string                    `json:"difficulty"`
	MixHash                string                    `json:"mixHash"`
	Coinbase               string                    `json:"coinbase"`
	Alloc                  map[string]AccountBalance `json:"alloc,omitempty"`
	// Metrics configuration
	MetricsEnabled  bool   `json:"metricsEnabled"`
	MetricsHost     string `json:"metricsHost"`
	MetricsPort     int    `json:"metricsPort"`
	MetricsProtocol string `json:"metricsProtocol"`
}

// UnmarshalNetworkConfig unmarshals network configuration based on its type
func UnmarshalNetworkConfig(data []byte) (NetworkConfig, error) {
	var base BaseNetworkConfig
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, fmt.Errorf("failed to unmarshal base config: %w", err)
	}

	switch base.Type {
	case NetworkTypeFabric:
		var config FabricNetworkConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Fabric config: %w", err)
		}
		return &config, nil
	case NetworkTypeBesu:
		var config BesuNetworkConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Besu config: %w", err)
		}
		return &config, nil
	default:
		return nil, fmt.Errorf("unsupported network type: %s", base.Type)
	}
}

// Validate implements NetworkConfig interface for FabricNetworkConfig
func (c *FabricNetworkConfig) Validate() error {
	if c.ChannelName == "" {
		return fmt.Errorf("channel name is required")
	}
	peerOrgLen := len(c.PeerOrganizations)
	if peerOrgLen == 0 {
		return fmt.Errorf("at least one peer organization is required")
	}
	ordererOrgLen := len(c.OrdererOrganizations)
	if ordererOrgLen == 0 {
		return fmt.Errorf("at least one orderer organization is required")
	}
	return nil
}

// Type implements NetworkConfig interface for FabricNetworkConfig
func (c *FabricNetworkConfig) Type() string {
	return string(NetworkTypeFabric)
}

// Validate implements NetworkConfig interface for BesuNetworkConfig
func (c *BesuNetworkConfig) Validate() error {
	if c.ChainID == 0 {
		return fmt.Errorf("chain ID is required")
	}
	if c.Consensus == "" {
		return fmt.Errorf("consensus mechanism is required")
	}
	return nil
}

// Type implements NetworkConfig interface for BesuNetworkConfig
func (c *BesuNetworkConfig) Type() string {
	return string(NetworkTypeBesu)
}

// NetworkDeployer defines the interface for network deployment operations
type NetworkDeployer interface {
	CreateGenesisBlock(networkID int64, config interface{}) ([]byte, error)
	JoinNode(networkID int64, genesisBlock []byte, nodeID int64) error
	GetStatus(networkID int64) (*NetworkDeploymentStatus, error)
}

// SmartBFTConsenter represents a SmartBFT consenter with additional fields
type SmartBFTConsenter struct {
	Address       HostPort `json:"address"`
	ClientTLSCert string   `json:"clientTLSCert"`
	ServerTLSCert string   `json:"serverTLSCert"`
	Identity      string   `json:"identity"`
	ID            uint64   `json:"id"`
	MSPID         string   `json:"mspId"`
}

// SmartBFTOptions represents SmartBFT configuration options
type SmartBFTOptions struct {
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

// EtcdRaftOptions represents etcdraft configuration options
type EtcdRaftOptions struct {
	TickInterval         string `json:"tickInterval"`
	ElectionTick         uint32 `json:"electionTick"`
	HeartbeatTick        uint32 `json:"heartbeatTick"`
	MaxInflightBlocks    uint32 `json:"maxInflightBlocks"`
	SnapshotIntervalSize uint32 `json:"snapshotIntervalSize"`
}

// BatchSize represents batch size configuration
type BatchSize struct {
	MaxMessageCount   uint32 `json:"maxMessageCount"`
	AbsoluteMaxBytes  uint32 `json:"absoluteMaxBytes"`
	PreferredMaxBytes uint32 `json:"preferredMaxBytes"`
}
