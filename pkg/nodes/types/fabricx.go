package types

import (
	"fmt"
)

// FabricXOrdererGroupConfig represents user input to create a Fabric X orderer group
type FabricXOrdererGroupConfig struct {
	BaseNodeConfig
	Name           string            `json:"name" validate:"required"`
	OrganizationID int64             `json:"organizationId" validate:"required"`
	MSPID          string            `json:"mspId" validate:"required"`
	PartyID        int               `json:"partyId" validate:"required"`
	ExternalIP     string            `json:"externalIp"`
	DomainNames    []string          `json:"domainNames,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Version        string            `json:"version"` // fabric-x-orderer image tag

	// Ports (auto-allocated if zero)
	RouterPort    int `json:"routerPort,omitempty"`
	BatcherPort   int `json:"batcherPort,omitempty"`
	ConsenterPort int `json:"consenterPort,omitempty"`
	AssemblerPort int `json:"assemblerPort,omitempty"`

	// Monitoring (Prometheus /metrics) host ports per role. Zero means
	// "auto-allocate during Init" (router GRPC port + 100 by default).
	RouterMonitoringPort    int `json:"routerMonitoringPort,omitempty"`
	BatcherMonitoringPort   int `json:"batcherMonitoringPort,omitempty"`
	ConsenterMonitoringPort int `json:"consenterMonitoringPort,omitempty"`
	AssemblerMonitoringPort int `json:"assemblerMonitoringPort,omitempty"`

	// Tuning
	ConsenterType string `json:"consenterType,omitempty"` // "raft" or "pbft", default "pbft"
}

func (c *FabricXOrdererGroupConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if c.OrganizationID == 0 {
		return fmt.Errorf("organization ID is required")
	}
	if c.MSPID == "" {
		return fmt.Errorf("MSPID is required")
	}
	if c.PartyID < 1 || c.PartyID > 10 {
		return fmt.Errorf("partyId must be between 1 and 10")
	}
	return nil
}

// FabricXCommitterConfig represents user input to create a Fabric X committer
type FabricXCommitterConfig struct {
	BaseNodeConfig
	Name           string            `json:"name" validate:"required"`
	OrganizationID int64             `json:"organizationId" validate:"required"`
	MSPID          string            `json:"mspId" validate:"required"`
	ExternalIP     string            `json:"externalIp"`
	DomainNames    []string          `json:"domainNames,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Version        string            `json:"version"` // fabric-x-committer image tag

	// NodeGroupID points at the parent FABRICX_COMMITTER node-group that
	// owns this committer node. Required: a network has one shared
	// committer node-group that all per-party committer nodes hang off
	// (each carries its own MSP identity, but they're listed together
	// under one group so the network can be reasoned about as a whole).
	NodeGroupID int64 `json:"nodeGroupId" validate:"required"`

	// Ports (auto-allocated if zero)
	SidecarPort      int `json:"sidecarPort,omitempty"`
	CoordinatorPort  int `json:"coordinatorPort,omitempty"`
	ValidatorPort    int `json:"validatorPort,omitempty"`
	VerifierPort     int `json:"verifierPort,omitempty"`
	QueryServicePort int `json:"queryServicePort,omitempty"`

	// Monitoring (Prometheus /metrics) host ports per role. Zero means
	// "auto-allocate during Init" using the upstream defaults
	// (sidecar 2114, verifier 2115, validator 2116, queryservice 2117,
	// coordinator 2119) — see fabric-x-committer service/*/config.go.
	SidecarMonitoringPort      int `json:"sidecarMonitoringPort,omitempty"`
	CoordinatorMonitoringPort  int `json:"coordinatorMonitoringPort,omitempty"`
	ValidatorMonitoringPort    int `json:"validatorMonitoringPort,omitempty"`
	VerifierMonitoringPort     int `json:"verifierMonitoringPort,omitempty"`
	QueryServiceMonitoringPort int `json:"queryServiceMonitoringPort,omitempty"`

	// Orderer endpoints (assembler addresses) for the sidecar to connect to
	OrdererEndpoints []string `json:"ordererEndpoints" validate:"required"`

	// PostgreSQL config for validator and query-service
	PostgresHost     string `json:"postgresHost" validate:"required"`
	PostgresPort     int    `json:"postgresPort,omitempty"`
	PostgresDB       string `json:"postgresDb,omitempty"`
	PostgresUser     string `json:"postgresUser,omitempty"`
	PostgresPassword string `json:"postgresPassword,omitempty"`

	// Channel ID for the sidecar
	ChannelID string `json:"channelId,omitempty"`
}

func (c *FabricXCommitterConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if c.OrganizationID == 0 {
		return fmt.Errorf("organization ID is required")
	}
	if c.MSPID == "" {
		return fmt.Errorf("MSPID is required")
	}
	if c.PostgresHost == "" {
		return fmt.Errorf("postgresHost is required")
	}
	if len(c.OrdererEndpoints) == 0 {
		return fmt.Errorf("ordererEndpoints is required")
	}
	return nil
}

// --- Deployment Configs (generated after Init) ---

// FabricXOrdererGroupDeploymentConfig is the computed config after cert generation
type FabricXOrdererGroupDeploymentConfig struct {
	BaseDeploymentConfig
	OrganizationID int64    `json:"organizationId"`
	MSPID          string   `json:"mspId"`
	PartyID        int      `json:"partyId"`
	ExternalIP     string   `json:"externalIp"`
	DomainNames    []string `json:"domainNames,omitempty"`
	Version        string   `json:"version"`

	// Generated keys/certs
	SignKeyID int64  `json:"signKeyId"`
	TLSKeyID  int64  `json:"tlsKeyId"`
	SignCert  string `json:"signCert"`
	TLSCert   string `json:"tlsCert"`
	CACert    string `json:"caCert"`
	TLSCACert string `json:"tlsCaCert"`

	// Allocated ports
	RouterPort    int `json:"routerPort"`
	BatcherPort   int `json:"batcherPort"`
	ConsenterPort int `json:"consenterPort"`
	AssemblerPort int `json:"assemblerPort"`

	// Allocated monitoring (Prometheus /metrics) host ports per role.
	RouterMonitoringPort    int `json:"routerMonitoringPort"`
	BatcherMonitoringPort   int `json:"batcherMonitoringPort"`
	ConsenterMonitoringPort int `json:"consenterMonitoringPort"`
	AssemblerMonitoringPort int `json:"assemblerMonitoringPort"`

	// Container names
	RouterContainer    string `json:"routerContainer"`
	BatcherContainer   string `json:"batcherContainer"`
	ConsenterContainer string `json:"consenterContainer"`
	AssemblerContainer string `json:"assemblerContainer"`

	ConsenterType string `json:"consenterType"`
}

func (c *FabricXOrdererGroupDeploymentConfig) Validate() error {
	if c.Mode != "docker" {
		return fmt.Errorf("fabricx orderer group only supports docker mode, got: %s", c.Mode)
	}
	return nil
}
func (c *FabricXOrdererGroupDeploymentConfig) GetServiceName() string   { return c.ServiceName }
func (c *FabricXOrdererGroupDeploymentConfig) GetOrganizationID() int64 { return c.OrganizationID }
func (c *FabricXOrdererGroupDeploymentConfig) ToFabricPeerConfig() *FabricPeerDeploymentConfig {
	return nil
}
func (c *FabricXOrdererGroupDeploymentConfig) ToFabricOrdererConfig() *FabricOrdererDeploymentConfig {
	return nil
}
func (c *FabricXOrdererGroupDeploymentConfig) ToBesuNodeConfig() *BesuNodeDeploymentConfig {
	return nil
}

// FabricXCommitterDeploymentConfig is the computed config after cert generation
type FabricXCommitterDeploymentConfig struct {
	BaseDeploymentConfig
	OrganizationID int64    `json:"organizationId"`
	MSPID          string   `json:"mspId"`
	ExternalIP     string   `json:"externalIp"`
	DomainNames    []string `json:"domainNames,omitempty"`
	Version        string   `json:"version"`

	// Generated keys/certs
	SignKeyID int64  `json:"signKeyId"`
	TLSKeyID  int64  `json:"tlsKeyId"`
	SignCert  string `json:"signCert"`
	TLSCert   string `json:"tlsCert"`
	CACert    string `json:"caCert"`
	TLSCACert string `json:"tlsCaCert"`

	// Allocated ports
	SidecarPort      int `json:"sidecarPort"`
	CoordinatorPort  int `json:"coordinatorPort"`
	ValidatorPort    int `json:"validatorPort"`
	VerifierPort     int `json:"verifierPort"`
	QueryServicePort int `json:"queryServicePort"`

	// Allocated monitoring (Prometheus /metrics) host ports per role.
	SidecarMonitoringPort      int `json:"sidecarMonitoringPort"`
	CoordinatorMonitoringPort  int `json:"coordinatorMonitoringPort"`
	ValidatorMonitoringPort    int `json:"validatorMonitoringPort"`
	VerifierMonitoringPort     int `json:"verifierMonitoringPort"`
	QueryServiceMonitoringPort int `json:"queryServiceMonitoringPort"`

	// Container names
	SidecarContainer      string `json:"sidecarContainer"`
	CoordinatorContainer  string `json:"coordinatorContainer"`
	ValidatorContainer    string `json:"validatorContainer"`
	VerifierContainer     string `json:"verifierContainer"`
	QueryServiceContainer string `json:"queryServiceContainer"`
	PostgresContainer     string `json:"postgresContainer,omitempty"`

	// Connection info
	OrdererEndpoints []string `json:"ordererEndpoints"`
	PostgresHost     string   `json:"postgresHost"`
	PostgresPort     int      `json:"postgresPort"`
	PostgresDB       string   `json:"postgresDb"`
	PostgresUser     string   `json:"postgresUser"`
	PostgresPassword string   `json:"postgresPassword"`
	ChannelID        string   `json:"channelId"`
}

func (c *FabricXCommitterDeploymentConfig) Validate() error {
	if c.Mode != "docker" {
		return fmt.Errorf("fabricx committer only supports docker mode, got: %s", c.Mode)
	}
	return nil
}
func (c *FabricXCommitterDeploymentConfig) GetServiceName() string   { return c.ServiceName }
func (c *FabricXCommitterDeploymentConfig) GetOrganizationID() int64 { return c.OrganizationID }
func (c *FabricXCommitterDeploymentConfig) ToFabricPeerConfig() *FabricPeerDeploymentConfig {
	return nil
}
func (c *FabricXCommitterDeploymentConfig) ToFabricOrdererConfig() *FabricOrdererDeploymentConfig {
	return nil
}
func (c *FabricXCommitterDeploymentConfig) ToBesuNodeConfig() *BesuNodeDeploymentConfig {
	return nil
}

// FabricXRole identifies one of the per-container roles owned by a node
// group. Each child `nodes` row has exactly one role.
type FabricXRole string

const (
	FabricXRoleOrdererRouter         FabricXRole = "router"
	FabricXRoleOrdererBatcher        FabricXRole = "batcher"
	FabricXRoleOrdererConsenter      FabricXRole = "consenter"
	FabricXRoleOrdererAssembler      FabricXRole = "assembler"
	FabricXRoleCommitterSidecar      FabricXRole = "sidecar"
	FabricXRoleCommitterCoordinator  FabricXRole = "coordinator"
	FabricXRoleCommitterValidator    FabricXRole = "validator"
	FabricXRoleCommitterVerifier     FabricXRole = "verifier"
	FabricXRoleCommitterQueryService FabricXRole = "query-service"
)

// FabricXChildConfig is the stored config for a single child `nodes` row
// (one container) owned by a node_group. It is intentionally thin: the
// heavy state (keys, certs, externalIP, shared ports) lives on the group's
// `node_groups.deployment_config`. The child carries just what's needed
// to dispatch StartNode → correct role-specific startup.
type FabricXChildConfig struct {
	BaseNodeConfig
	// NodeGroupID points back to the owning node_groups row. Required.
	NodeGroupID int64 `json:"nodeGroupId"`
	// Role selects which container this child represents.
	Role FabricXRole `json:"role"`
	// Name is the child's human-readable name (usually groupName-role).
	Name string `json:"name"`
}

func (c *FabricXChildConfig) Validate() error {
	if c.NodeGroupID == 0 {
		return fmt.Errorf("nodeGroupId is required")
	}
	if c.Role == "" {
		return fmt.Errorf("role is required")
	}
	return nil
}

// FabricXChildDeploymentConfig is the deployment config persisted on a
// child `nodes` row. Like the child node config it stays thin — the
// running container name and host port are included so StartNode can
// start exactly this container without touching siblings.
type FabricXChildDeploymentConfig struct {
	BaseDeploymentConfig
	NodeGroupID    int64       `json:"nodeGroupId"`
	Role           FabricXRole `json:"role"`
	ContainerName  string      `json:"containerName"`
	HostPort       int         `json:"hostPort"`
	MonitoringPort int         `json:"monitoringPort,omitempty"`
}

func (c *FabricXChildDeploymentConfig) Validate() error {
	if c.Mode != "docker" {
		return fmt.Errorf("fabricx child only supports docker mode, got: %s", c.Mode)
	}
	if c.NodeGroupID == 0 {
		return fmt.Errorf("nodeGroupId is required")
	}
	if c.Role == "" {
		return fmt.Errorf("role is required")
	}
	return nil
}
func (c *FabricXChildDeploymentConfig) GetServiceName() string   { return c.ServiceName }
func (c *FabricXChildDeploymentConfig) GetOrganizationID() int64 { return 0 }
func (c *FabricXChildDeploymentConfig) ToFabricPeerConfig() *FabricPeerDeploymentConfig {
	return nil
}
func (c *FabricXChildDeploymentConfig) ToFabricOrdererConfig() *FabricOrdererDeploymentConfig {
	return nil
}
func (c *FabricXChildDeploymentConfig) ToBesuNodeConfig() *BesuNodeDeploymentConfig {
	return nil
}
