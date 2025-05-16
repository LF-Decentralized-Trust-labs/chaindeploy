package types

import (
	"encoding/json"
	"fmt"
)

// NodeDeploymentConfig represents the deployment configuration for different types of nodes
// @Description Node deployment configuration interface that can be one of: FabricPeerDeploymentConfig, FabricOrdererDeploymentConfig, or BesuNodeDeploymentConfig
// @discriminator type
// @discriminatorMapping fabric-peer FabricPeerDeploymentConfig
// @discriminatorMapping fabric-orderer FabricOrdererDeploymentConfig
// @discriminatorMapping besu BesuNodeDeploymentConfig
// @model NodeDeploymentConfig
type NodeDeploymentConfig interface {
	// @name NodeDeploymentConfig
	// @Description Node deployment configuration
	GetType() string
	GetMode() string
	Validate() error
	GetServiceName() string
	GetOrganizationID() int64
	ToFabricPeerConfig() *FabricPeerDeploymentConfig
	ToFabricOrdererConfig() *FabricOrdererDeploymentConfig
	ToBesuNodeConfig() *BesuNodeDeploymentConfig
}

// BaseDeploymentConfig contains common deployment fields
// @Description Base configuration fields shared by all node deployment types
type BaseDeploymentConfig struct {
	// @Description The type of the node deployment (fabric-peer, fabric-orderer, besu)
	Type string `json:"type" example:"fabric-peer"`
	// @Description The deployment mode (service or docker)
	Mode string `json:"mode" example:"service"`
	// @Description Optional service name for the deployment
	ServiceName string `json:"serviceName,omitempty" example:"peer0-org1"`
	// @Description Optional container name for the deployment
	ContainerName string `json:"containerName,omitempty" example:"peer0.org1.example.com"`
	// @Description Optional environment variables for the deployment
	Env map[string]string `json:"env,omitempty"`
}

func (c BaseDeploymentConfig) GetType() string { return c.Type }
func (c BaseDeploymentConfig) GetMode() string { return c.Mode }

// FabricPeerDeploymentConfig represents the computed deployment configuration for a Fabric peer
// @Description Deployment configuration specific to Fabric peer nodes
// @model FabricPeerDeploymentConfig
type FabricPeerDeploymentConfig struct {
	BaseDeploymentConfig
	// @Description Organization ID that owns this peer
	OrganizationID int64 `json:"organizationId" validate:"required" example:"1"`
	// @Description MSP ID for the organization
	MSPID string `json:"mspId" validate:"required" example:"Org1MSP"`
	// Identity and security
	// @Description ID of the signing key
	SignKeyID int64 `json:"signKeyId" example:"1"`
	// @Description ID of the TLS key
	TLSKeyID int64 `json:"tlsKeyId" example:"2"`
	// @Description PEM encoded signing certificate
	SignCert string `json:"signCert"`
	// @Description PEM encoded TLS certificate
	TLSCert string `json:"tlsCert"`
	// @Description PEM encoded CA certificate
	CACert string `json:"caCert"`
	// @Description PEM encoded TLS CA certificate
	TLSCACert string `json:"tlsCaCert"`

	// Network configuration
	// @Description Listen address for the peer
	ListenAddress string `json:"listenAddress" example:"0.0.0.0:7051"`
	// @Description Chaincode listen address
	ChaincodeAddress string `json:"chaincodeAddress" example:"0.0.0.0:7052"`
	// @Description Events listen address
	EventsAddress string `json:"eventsAddress" example:"0.0.0.0:7053"`
	// @Description Operations listen address
	OperationsListenAddress string `json:"operationsListenAddress" example:"0.0.0.0:9443"`
	// @Description External endpoint for the peer
	ExternalEndpoint string `json:"externalEndpoint" example:"peer0.org1.example.com:7051"`
	// @Description Domain names for the peer
	DomainNames []string `json:"domainNames,omitempty"`

	// @Description Address overrides for the peer
	AddressOverrides []AddressOverride `json:"addressOverrides,omitempty"`
	// @Description Fabric version to use
	Version string `json:"version" example:"2.5.0"`
}

func (c *FabricPeerDeploymentConfig) GetMode() string { return c.Mode }
func (c *FabricPeerDeploymentConfig) Validate() error {
	if c.Mode != "service" && c.Mode != "docker" {
		return fmt.Errorf("invalid mode: %s", c.Mode)
	}
	return nil
}

func (c *FabricPeerDeploymentConfig) GetServiceName() string   { return c.ServiceName }
func (c *FabricPeerDeploymentConfig) GetOrganizationID() int64 { return c.OrganizationID }
func (c *FabricPeerDeploymentConfig) ToFabricPeerConfig() *FabricPeerDeploymentConfig {
	return &FabricPeerDeploymentConfig{
		BaseDeploymentConfig:    BaseDeploymentConfig{Type: "fabric-peer", Mode: c.Mode},
		OrganizationID:          c.OrganizationID,
		MSPID:                   c.MSPID,
		SignKeyID:               c.SignKeyID,
		TLSKeyID:                c.TLSKeyID,
		ListenAddress:           c.ListenAddress,
		ChaincodeAddress:        c.ChaincodeAddress,
		EventsAddress:           c.EventsAddress,
		OperationsListenAddress: c.OperationsListenAddress,
		ExternalEndpoint:        c.ExternalEndpoint,
		DomainNames:             c.DomainNames,
		SignCert:                c.SignCert,
		TLSCert:                 c.TLSCert,
		CACert:                  c.CACert,
		TLSCACert:               c.TLSCACert,
		Version:                 c.Version,
	}
}
func (c *FabricPeerDeploymentConfig) ToFabricOrdererConfig() *FabricOrdererDeploymentConfig {
	return nil
}
func (c *FabricPeerDeploymentConfig) ToBesuNodeConfig() *BesuNodeDeploymentConfig {
	return nil
}

// FabricOrdererDeploymentConfig represents the computed deployment configuration for a Fabric orderer
// @Description Deployment configuration specific to Fabric orderer nodes
// @model FabricOrdererDeploymentConfig
type FabricOrdererDeploymentConfig struct {
	BaseDeploymentConfig
	// @Description Organization ID that owns this orderer
	OrganizationID int64 `json:"organizationId" validate:"required" example:"1"`
	// @Description MSP ID for the organization
	MSPID string `json:"mspId" validate:"required" example:"OrdererMSP"`
	// Identity and security
	// @Description ID of the signing key
	SignKeyID int64 `json:"signKeyId" example:"1"`
	// @Description ID of the TLS key
	TLSKeyID int64 `json:"tlsKeyId" example:"2"`
	// @Description PEM encoded signing certificate
	SignCert string `json:"signCert"`
	// @Description PEM encoded TLS certificate
	TLSCert string `json:"tlsCert"`
	// @Description PEM encoded CA certificate
	CACert string `json:"caCert"`
	// @Description PEM encoded TLS CA certificate
	TLSCACert string `json:"tlsCaCert"`

	// Network configuration
	// @Description Listen address for the orderer
	ListenAddress string `json:"listenAddress" example:"0.0.0.0:7050"`
	// @Description Admin listen address
	AdminAddress string `json:"adminAddress" example:"0.0.0.0:7053"`
	// @Description Operations listen address
	OperationsListenAddress string `json:"operationsListenAddress" example:"0.0.0.0:9443"`
	// @Description External endpoint for the orderer
	ExternalEndpoint string `json:"externalEndpoint" example:"orderer.example.com:7050"`
	// @Description Domain names for the orderer
	DomainNames []string `json:"domainNames,omitempty"`
	// @Description Fabric version to use
	Version string `json:"version" example:"2.5.0"`
}

func (c *FabricOrdererDeploymentConfig) GetURL() string {
	return fmt.Sprintf("grpcs://%s", c.ExternalEndpoint)
}

func (c *FabricOrdererDeploymentConfig) GetAddress() string {
	return c.ExternalEndpoint
}

func (c *FabricOrdererDeploymentConfig) GetMode() string { return c.Mode }
func (c *FabricOrdererDeploymentConfig) Validate() error {
	if c.Mode != "service" && c.Mode != "docker" {
		return fmt.Errorf("invalid mode: %s", c.Mode)
	}
	return nil
}

func (c *FabricOrdererDeploymentConfig) GetServiceName() string                          { return c.ServiceName }
func (c *FabricOrdererDeploymentConfig) GetOrganizationID() int64                        { return c.OrganizationID }
func (c *FabricOrdererDeploymentConfig) ToFabricPeerConfig() *FabricPeerDeploymentConfig { return nil }
func (c *FabricOrdererDeploymentConfig) ToBesuNodeConfig() *BesuNodeDeploymentConfig {
	return nil
}
func (c *FabricOrdererDeploymentConfig) ToFabricOrdererConfig() *FabricOrdererDeploymentConfig {
	return &FabricOrdererDeploymentConfig{
		BaseDeploymentConfig:    BaseDeploymentConfig{Type: "fabric-orderer", Mode: c.Mode},
		OrganizationID:          c.OrganizationID,
		MSPID:                   c.MSPID,
		ListenAddress:           c.ListenAddress,
		AdminAddress:            c.AdminAddress,
		OperationsListenAddress: c.OperationsListenAddress,
		ExternalEndpoint:        c.ExternalEndpoint,
		DomainNames:             c.DomainNames,
		SignCert:                c.SignCert,
		TLSCert:                 c.TLSCert,
		CACert:                  c.CACert,
		TLSCACert:               c.TLSCACert,
		SignKeyID:               c.SignKeyID,
		TLSKeyID:                c.TLSKeyID,
		Version:                 c.Version,
	}
}

// BesuNodeDeploymentConfig represents the computed deployment configuration for a Besu node
// @Description Deployment configuration specific to Besu nodes
// @model BesuNodeDeploymentConfig
type BesuNodeDeploymentConfig struct {
	BaseDeploymentConfig
	// @Description ID of the node key
	KeyID int64 `json:"keyId" validate:"required" example:"1"`
	// @Description P2P port for node communication
	P2PPort uint `json:"p2pPort" validate:"required" example:"30303"`
	// @Description RPC port for API access
	RPCPort uint `json:"rpcPort" validate:"required" example:"8545"`
	// @Description P2P host address
	P2PHost string `json:"p2pHost" validate:"required" example:"0.0.0.0"`
	// @Description RPC host address
	RPCHost string `json:"rpcHost" validate:"required" example:"0.0.0.0"`
	// @Description External IP address of the node
	ExternalIP string `json:"externalIp" validate:"required" example:"172.16.1.10"`
	// @Description Internal IP address of the node
	InternalIP string `json:"internalIp" validate:"required" example:"10.0.0.10"`
	// @Description Network ID of the blockchain
	NetworkID int64 `json:"networkId" validate:"required" example:"1337"`
	// @Description Enode URL for node discovery
	EnodeURL string `json:"enodeUrl" example:"enode://pubkey@172.16.1.10:30303"`
	// @Description Metrics port for Prometheus metrics
	MetricsPort int64 `json:"metricsPort" validate:"required" example:"9545"`
	// @Description Whether metrics are enabled
	MetricsEnabled bool `json:"metricsEnabled" example:"true"`
	// @Description Metrics protocol (e.g. PROMETHEUS)
	MetricsProtocol string `json:"metricsProtocol" validate:"required" example:"PROMETHEUS"`
}

func (c *BesuNodeDeploymentConfig) GetMode() string { return c.Mode }
func (c *BesuNodeDeploymentConfig) Validate() error {
	if c.Mode != "service" && c.Mode != "docker" {
		return fmt.Errorf("invalid mode: %s", c.Mode)
	}
	return nil
}

func (c *BesuNodeDeploymentConfig) GetServiceName() string                                { return c.ServiceName }
func (c *BesuNodeDeploymentConfig) GetOrganizationID() int64                              { return 0 }
func (c *BesuNodeDeploymentConfig) ToFabricPeerConfig() *FabricPeerDeploymentConfig       { return nil }
func (c *BesuNodeDeploymentConfig) ToFabricOrdererConfig() *FabricOrdererDeploymentConfig { return nil }
func (c *BesuNodeDeploymentConfig) ToBesuNodeConfig() *BesuNodeDeploymentConfig {
	return &BesuNodeDeploymentConfig{
		BaseDeploymentConfig: BaseDeploymentConfig{Type: "besu", Mode: c.Mode},
		KeyID:                c.KeyID,
		P2PPort:              c.P2PPort,
		RPCPort:              c.RPCPort,
		P2PHost:              c.P2PHost,
		RPCHost:              c.RPCHost,
		ExternalIP:           c.ExternalIP,
		InternalIP:           c.InternalIP,
		NetworkID:            c.NetworkID,
	}
}

// NodeConfig is the interface that all node configurations must implement
// @Description Base interface for all node configurations
type NodeConfig interface {
	GetType() string
	Validate() error
}

// BaseNodeConfig contains common fields for all node configurations
// @Description Base configuration shared by all node types
type BaseNodeConfig struct {
	// @Description The type of node (fabric-peer, fabric-orderer, besu)
	Type string `json:"type" example:"fabric-peer"`
	// @Description The deployment mode (service or docker)
	Mode string `json:"mode" example:"service"`
}

func (c BaseNodeConfig) GetType() string { return c.Type }

// FabricPeerConfig represents the parameters needed to create a Fabric peer node
// @Description Configuration for creating a new Fabric peer node
type FabricPeerConfig struct {
	BaseNodeConfig
	// @Description Name of the peer node
	Name string `json:"name" validate:"required" example:"peer0-org1"`
	// @Description Organization ID that owns this peer
	OrganizationID int64 `json:"organizationId" validate:"required" example:"1"`
	// @Description MSP ID for the organization
	MSPID string `json:"mspId" validate:"required" example:"Org1MSP"`
	// @Description External endpoint for the peer
	ExternalEndpoint string `json:"externalEndpoint" example:"peer0.org1.example.com:7051"`
	// @Description Listen address for the peer
	ListenAddress string `json:"listenAddress" example:"0.0.0.0:7051"`
	// @Description Events listen address
	EventsAddress string `json:"eventsAddress" example:"0.0.0.0:7053"`
	// @Description Chaincode listen address
	ChaincodeAddress string `json:"chaincodeAddress" example:"0.0.0.0:7052"`
	// @Description Operations listen address
	OperationsListenAddress string `json:"operationsListenAddress" example:"0.0.0.0:9443"`
	// @Description Domain names for the peer
	DomainNames []string `json:"domainNames,omitempty"`
	// @Description Environment variables for the peer
	Env map[string]string `json:"env,omitempty"`
	// @Description Fabric version to use
	Version string `json:"version" example:"2.2.0"`
	// @Description Orderer address overrides for the peer
	OrdererAddressOverrides []OrdererAddressOverride `json:"ordererAddressOverrides,omitempty"`
	// @Description Address overrides for the peer
	AddressOverrides []AddressOverride `json:"addressOverrides,omitempty"`
}

// FabricOrdererConfig represents the parameters needed to create a Fabric orderer node
type FabricOrdererConfig struct {
	BaseNodeConfig
	Name                    string            `json:"name" validate:"required"`
	OrganizationID          int64             `json:"organizationId" validate:"required"`
	MSPID                   string            `json:"mspId" validate:"required"`
	ExternalEndpoint        string            `json:"externalEndpoint"`
	ListenAddress           string            `json:"listenAddress"`
	AdminAddress            string            `json:"adminAddress"`
	OperationsListenAddress string            `json:"operationsListenAddress"`
	DomainNames             []string          `json:"domainNames,omitempty"`
	Env                     map[string]string `json:"env,omitempty"`
	Version                 string            `json:"version"` // Fabric version to use
	// @Description Address overrides for the orderer
	AddressOverrides []AddressOverride `json:"addressOverrides,omitempty"`
}

// BesuNodeConfig represents the parameters needed to create a Besu node
type BesuNodeConfig struct {
	BaseNodeConfig
	NetworkID       int64             `json:"networkId" validate:"required"`
	KeyID           int64             `json:"keyId" validate:"required"`
	P2PPort         uint              `json:"p2pPort" validate:"required"`
	RPCPort         uint              `json:"rpcPort" validate:"required"`
	P2PHost         string            `json:"p2pHost" validate:"required"`
	RPCHost         string            `json:"rpcHost" validate:"required"`
	ExternalIP      string            `json:"externalIp" validate:"required"`
	InternalIP      string            `json:"internalIp" validate:"required"`
	Env             map[string]string `json:"env,omitempty"`
	BootNodes       []string          `json:"bootNodes,omitempty"`
	MetricsEnabled  bool              `json:"metricsEnabled"`
	MetricsPort     int64             `json:"metricsPort"`
	MetricsProtocol string            `json:"metricsProtocol"`
}

// Add this new type for storage
// StoredNodeConfig represents the configuration as stored in the database
// @Description Node configuration as stored in the database
type StoredNodeConfig struct {
	// @Description Type of the node (fabric-peer, fabric-orderer, besu)
	Type string `json:"type" example:"fabric-peer"`
	// @Description Raw JSON configuration data
	Config json.RawMessage `json:"config"`
}

// Add helper functions for serialization/deserialization
func SerializeNodeConfig(config NodeDeploymentConfig) (*StoredNodeConfig, error) {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	return &StoredNodeConfig{
		Type:   config.GetType(),
		Config: configBytes,
	}, nil
}

func DeserializeNodeConfig(stored *StoredNodeConfig) (NodeConfig, error) {
	var config NodeConfig

	switch stored.Type {
	case "fabric-peer":
		var c FabricPeerConfig
		if err := json.Unmarshal(stored.Config, &c); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fabric peer config: %w", err)
		}
		config = &c
	case "fabric-orderer":
		var c FabricOrdererConfig
		if err := json.Unmarshal(stored.Config, &c); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fabric orderer config: %w", err)
		}
		config = &c
	case "besu":
		var c BesuNodeConfig
		if err := json.Unmarshal(stored.Config, &c); err != nil {
			return nil, fmt.Errorf("failed to unmarshal besu config: %w", err)
		}
		config = &c
	default:
		return nil, fmt.Errorf("unknown node type: %s", stored.Type)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// Validation methods
func (c *FabricPeerConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if c.OrganizationID == 0 {
		return fmt.Errorf("organization ID is required")
	}
	if c.MSPID == "" {
		return fmt.Errorf("MSPID is required")
	}
	return nil
}

func (c *FabricOrdererConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if c.OrganizationID == 0 {
		return fmt.Errorf("organization ID is required")
	}
	if c.MSPID == "" {
		return fmt.Errorf("MSPID is required")
	}
	return nil
}

func (c *BesuNodeConfig) Validate() error {

	return nil
}

// MapToNodeConfig maps a deployment config to a node config based on type
func MapToNodeConfig(deploymentConfig NodeDeploymentConfig) (NodeConfig, error) {
	switch deploymentConfig.GetType() {
	case "fabric-peer":
		if peerConfig := deploymentConfig.ToFabricPeerConfig(); peerConfig != nil {
			return peerConfig, nil
		}
		return nil, fmt.Errorf("failed to convert to fabric peer config")

	case "fabric-orderer":
		if ordererConfig := deploymentConfig.ToFabricOrdererConfig(); ordererConfig != nil {
			return ordererConfig, nil
		}
		return nil, fmt.Errorf("failed to convert to fabric orderer config")

	case "besu":
		if besuConfig, ok := deploymentConfig.(*BesuNodeDeploymentConfig); ok {
			return &BesuNodeConfig{
				BaseNodeConfig: BaseNodeConfig{
					Type: "besu",
					Mode: besuConfig.Mode,
				},
				P2PPort:    besuConfig.P2PPort,
				RPCPort:    besuConfig.RPCPort,
				ExternalIP: besuConfig.ExternalIP,
				KeyID:      besuConfig.KeyID,
				P2PHost:    besuConfig.P2PHost,
				RPCHost:    besuConfig.RPCHost,
				InternalIP: besuConfig.InternalIP,
			}, nil
		}
		return nil, fmt.Errorf("failed to convert to besu config")

	default:
		return nil, fmt.Errorf("unsupported node type: %s", deploymentConfig.GetType())
	}
}

// UpdateNodeEnvRequest represents a request to update a node's environment variables
type UpdateNodeEnvRequest struct {
	// @Description Environment variables to update
	Env map[string]string `json:"env" validate:"required"`
}

// UpdateNodeEnvResponse represents the response after updating a node's environment variables
type UpdateNodeEnvResponse struct {
	// @Description Updated environment variables
	Env map[string]string `json:"env"`
	// @Description Whether the node needs to be restarted for changes to take effect
	RequiresRestart bool `json:"requiresRestart"`
}

// UpdateNodeConfigRequest represents a request to update a node's configuration
type UpdateNodeConfigRequest struct {
	// Common fields
	// @Description Environment variables to update
	Env map[string]string `json:"env,omitempty"`
	// @Description Domain names for the node
	DomainNames []string `json:"domainNames,omitempty"`
	// @Description The deployment mode (service or docker)
	Mode string `json:"mode,omitempty" validate:"omitempty,oneof=service docker"`

	// Fabric peer specific fields
	// @Description Listen address for the peer
	ListenAddress string `json:"listenAddress,omitempty"`
	// @Description Chaincode listen address
	ChaincodeAddress string `json:"chaincodeAddress,omitempty"`
	// @Description Events listen address
	EventsAddress string `json:"eventsAddress,omitempty"`
	// @Description Operations listen address
	OperationsListenAddress string `json:"operationsListenAddress,omitempty"`
	// @Description External endpoint for the peer
	ExternalEndpoint string `json:"externalEndpoint,omitempty"`

	// Fabric orderer specific fields
	// @Description Admin listen address for orderer
	AdminAddress string `json:"adminAddress,omitempty"`

	// Besu specific fields
	// @Description P2P port for Besu node
	P2PPort uint `json:"p2pPort,omitempty"`
	// @Description RPC port for Besu node
	RPCPort uint `json:"rpcPort,omitempty"`
	// @Description P2P host address
	P2PHost string `json:"p2pHost,omitempty"`
	// @Description RPC host address
	RPCHost string `json:"rpcHost,omitempty"`
	// @Description External IP address
	ExternalIP string `json:"externalIp,omitempty"`
	// @Description Internal IP address
	InternalIP string `json:"internalIp,omitempty"`
}

// UpdateNodeConfigResponse represents the response after updating a node's configuration
type UpdateNodeConfigResponse struct {
	// @Description Updated node configuration
	Config NodeConfig `json:"config"`
	// @Description Whether the node needs to be restarted for changes to take effect
	RequiresRestart bool `json:"requiresRestart"`
}

// OrdererAddressOverride represents an orderer address override configuration
type OrdererAddressOverride struct {
	// @Description Original orderer address
	From string `json:"from" validate:"required"`
	// @Description New orderer address to use
	To string `json:"to" validate:"required"`
	// @Description TLS CA certificate in PEM format
	TLSCACert string `json:"tlsCACert" validate:"required"`
}

// UpdatePeerOrdererOverridesRequest represents a request to update a peer's orderer address overrides
type UpdatePeerOrdererOverridesRequest struct {
	// @Description List of orderer address overrides
	Overrides []OrdererAddressOverride `json:"overrides" validate:"required,dive"`
}

// UpdatePeerOrdererOverridesResponse represents the response after updating orderer address overrides
type UpdatePeerOrdererOverridesResponse struct {
	// @Description Updated orderer address overrides
	Overrides []OrdererAddressOverride `json:"overrides"`
	// @Description Whether the node needs to be restarted for changes to take effect
	RequiresRestart bool `json:"requiresRestart"`
}

// UpdateNodeAddressOverridesRequest represents a request to update a node's address overrides
type UpdateNodeAddressOverridesRequest struct {
	// @Description List of address overrides
	Overrides []AddressOverride `json:"overrides" validate:"required,dive"`
}

// UpdateNodeAddressOverridesResponse represents the response after updating address overrides
type UpdateNodeAddressOverridesResponse struct {
	// @Description Updated address overrides
	Overrides []AddressOverride `json:"overrides"`
	// @Description Whether the node needs to be restarted for changes to take effect
	RequiresRestart bool `json:"requiresRestart"`
}
