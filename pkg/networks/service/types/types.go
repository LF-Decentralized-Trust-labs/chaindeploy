package types

import (
	"encoding/json"
	"fmt"
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
type FabricNetworkConfig struct {
	BaseNetworkConfig
	ChannelName          string         `json:"channelName"`
	PeerOrganizations    []Organization `json:"peerOrganizations"`
	OrdererOrganizations []Organization `json:"ordererOrganizations"`
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
