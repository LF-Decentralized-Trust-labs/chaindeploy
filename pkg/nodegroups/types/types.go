// Package types defines the public domain model for node groups.
//
// A node group is a logical parent that owns shared identity/crypto material
// for a set of child nodes (e.g. a FabricX orderer group owns the router,
// batcher, consenter and assembler children). The group itself is not a
// runnable container — it is metadata plus a lifecycle coordinator. See
// ADR 0001 for the full rationale.
package types

import (
	"encoding/json"
	"time"

	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// GroupType identifies the kind of node group and determines which child
// node types the group coordinates.
type GroupType string

const (
	GroupTypeFabricXOrderer   GroupType = "FABRICX_ORDERER_GROUP"
	GroupTypeFabricXCommitter GroupType = "FABRICX_COMMITTER"
)

// GroupStatus is the aggregated status of a node_group derived from its
// children. Persisted values are constrained by the node_statuses FK.
type GroupStatus = nodetypes.NodeStatus

// NodeGroup is the domain representation of a row in node_groups, with
// JSON-encoded columns hydrated into typed fields.
type NodeGroup struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Platform       string    `json:"platform"`
	GroupType      GroupType `json:"groupType"`
	MSPID          string    `json:"mspId,omitempty"`
	OrganizationID *int64    `json:"organizationId,omitempty"`
	PartyID        *int64    `json:"partyId,omitempty"`
	Version        string    `json:"version,omitempty"`
	ExternalIP     string    `json:"externalIp,omitempty"`
	// DomainNames is a parsed JSON array of SANs/SNI hostnames shared by
	// all children. Stored as a JSON string column.
	DomainNames []string `json:"domainNames,omitempty"`
	SignKeyID   *int64   `json:"signKeyId,omitempty"`
	TLSKeyID    *int64   `json:"tlsKeyId,omitempty"`
	SignCert    string   `json:"signCert,omitempty"`
	TLSCert     string   `json:"tlsCert,omitempty"`
	CACert      string   `json:"caCert,omitempty"`
	TLSCACert   string   `json:"tlsCaCert,omitempty"`
	// Config is group-shared logical configuration (JSON payload whose
	// concrete shape depends on GroupType).
	Config json.RawMessage `json:"config,omitempty"`
	// DeploymentConfig is group-shared deployment configuration (network
	// name, shared mounts, etc.).
	DeploymentConfig json.RawMessage `json:"deploymentConfig,omitempty"`
	Status           GroupStatus     `json:"status"`
	ErrorMessage     string          `json:"errorMessage,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        *time.Time      `json:"updatedAt,omitempty"`
}

// ServiceType identifies the kind of managed supporting service attached
// to a node_group. First-class value is POSTGRES (for FabricX committer
// groups).
type ServiceType string

const (
	ServiceTypePostgres ServiceType = "POSTGRES"
)

// NodeGroupService is the domain representation of a services row, with JSON
// columns hydrated into typed fields.
type NodeGroupService struct {
	ID          int64       `json:"id"`
	NodeGroupID *int64      `json:"nodeGroupId,omitempty"`
	Name        string      `json:"name"`
	ServiceType ServiceType `json:"serviceType"`
	Version     string      `json:"version,omitempty"`
	Status      GroupStatus `json:"status"`
	// Config is service-type-specific logical configuration supplied at
	// creation time. For POSTGRES this carries db/user/password/hostPort.
	Config json.RawMessage `json:"config,omitempty"`
	// DeploymentConfig is resolved at runtime (host/port the coordinator
	// dials). Empty until the service has been started at least once.
	DeploymentConfig json.RawMessage `json:"deploymentConfig,omitempty"`
	ErrorMessage     string          `json:"errorMessage,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        *time.Time      `json:"updatedAt,omitempty"`
}

// PostgresServiceConfig is the JSON payload stored in services.config
// for POSTGRES services. Decoupled from the DB row so the coordinator
// and HTTP layer share one canonical shape.
type PostgresServiceConfig struct {
	Version  string `json:"version,omitempty"`
	DB       string `json:"db"`
	User     string `json:"user"`
	Password string `json:"password"`
	HostPort int    `json:"hostPort,omitempty"`
}

// PostgresServiceDeployment is the JSON payload stored in
// services.deployment_config once the postgres container is running.
// resolveManagedPostgres reads this to dial the managed instance.
type PostgresServiceDeployment struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	ContainerName string `json:"containerName,omitempty"`
	NetworkName   string `json:"networkName,omitempty"`
}

// ChildRoles returns the FabricX per-role NodeType values that belong to a
// group of the given type, in the canonical dependency order used by the
// lifecycle coordinator when starting children.
func ChildRoles(gt GroupType) []nodetypes.NodeType {
	switch gt {
	case GroupTypeFabricXOrderer:
		return []nodetypes.NodeType{
			nodetypes.NodeTypeFabricXOrdererConsenter,
			nodetypes.NodeTypeFabricXOrdererBatcher,
			nodetypes.NodeTypeFabricXOrdererAssembler,
			nodetypes.NodeTypeFabricXOrdererRouter,
		}
	case GroupTypeFabricXCommitter:
		return []nodetypes.NodeType{
			nodetypes.NodeTypeFabricXCommitterVerifier,
			nodetypes.NodeTypeFabricXCommitterValidator,
			nodetypes.NodeTypeFabricXCommitterCoordinator,
			nodetypes.NodeTypeFabricXCommitterSidecar,
			nodetypes.NodeTypeFabricXCommitterQueryService,
		}
	default:
		return nil
	}
}
