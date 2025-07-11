package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// Plugin represents a plugin definition
type Plugin struct {
	APIVersion       string            `json:"apiVersion" yaml:"apiVersion"`
	Kind             string            `json:"kind" yaml:"kind"`
	Metadata         Metadata          `json:"metadata" yaml:"metadata"`
	Spec             Spec              `json:"spec" yaml:"spec"`
	DeploymentStatus *DeploymentStatus `json:"deploymentStatus,omitempty" yaml:"deploymentStatus,omitempty"`
}

// Metadata contains plugin metadata
type Metadata struct {
	Name        string   `json:"name" yaml:"name"`
	Version     string   `json:"version" yaml:"version"`
	Description string   `json:"description" yaml:"description"`
	Author      string   `json:"author" yaml:"author"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Repository  string   `json:"repository,omitempty" yaml:"repository,omitempty"`
	License     string   `json:"license,omitempty" yaml:"license,omitempty"`
}

// Spec contains the plugin specification
type Spec struct {
	DockerCompose DockerCompose `json:"dockerCompose" yaml:"dockerCompose"`
	Parameters    Parameters    `json:"parameters" yaml:"parameters"`
	Documentation Documentation `json:"documentation" yaml:"documentation"`
}

// DockerCompose contains the docker-compose configuration
type DockerCompose struct {
	Contents string `json:"contents" yaml:"contents"`
}

// XSourceType defines the possible values for x-source
type XSourceType string

const (
	XSourceFabricPeer    XSourceType = "fabric-peer"
	XSourceKey           XSourceType = "key"
	XSourceFabricOrg     XSourceType = "fabric-org"
	XSourceFabricNetwork XSourceType = "fabric-network"
	XSourceFabricKey     XSourceType = "fabric-key"
)

// Parameters defines the plugin parameters schema
type Parameters struct {
	Schema     string                   `json:"$schema" yaml:"$schema"`
	Type       string                   `json:"type" yaml:"type"`
	Properties map[string]ParameterSpec `json:"properties" yaml:"properties"`
	Required   []string                 `json:"required" yaml:"required"`
	// XSource defines the source type for plugin parameters
	// Can be one of: fabric-peer, key, fabric-org, fabric-network
}

// ParameterSpec defines a single parameter specification
type ParameterSpec struct {
	Type        string      `json:"type" yaml:"type"`
	Description string      `json:"description" yaml:"description"`
	Default     string      `json:"default,omitempty" yaml:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty" yaml:"enum,omitempty"`
	XSource     XSourceType `json:"x-source,omitempty" yaml:"x-source,omitempty"`
}

// DeploymentStatus represents the status of a plugin deployment
type DeploymentStatus struct {
	Status      string                 `json:"status" yaml:"status"`
	StartedAt   time.Time              `json:"startedAt" yaml:"startedAt"`
	StoppedAt   time.Time              `json:"stoppedAt,omitempty" yaml:"stoppedAt,omitempty"`
	Error       string                 `json:"error,omitempty" yaml:"error,omitempty"`
	Services    []Service              `json:"services" yaml:"services"`
	ProjectName string                 `json:"projectName" yaml:"projectName"`
	Parameters  map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// Service represents a docker-compose service status
type Service struct {
	Name      string `json:"name" yaml:"name"`
	Status    string `json:"status" yaml:"status"`
	Image     string `json:"image" yaml:"image"`
	Ports     []Port `json:"ports" yaml:"ports"`
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
}

// Port represents a port mapping
type Port struct {
	HostPort      string `json:"hostPort" yaml:"hostPort"`
	ContainerPort string `json:"containerPort" yaml:"containerPort"`
	Protocol      string `json:"protocol" yaml:"protocol"`
}

// GetPluginParameters returns the plugin parameters as a JSON string
func (p *Plugin) GetPluginParameters() (string, error) {
	data, err := json.Marshal(p.Spec.Parameters)
	if err != nil {
		return "", fmt.Errorf("failed to marshal parameters: %w", err)
	}
	return string(data), nil
}

// SetPluginParameters sets the plugin parameters from a JSON string
func (p *Plugin) SetPluginParameters(parameters string) error {
	if err := json.Unmarshal([]byte(parameters), &p.Spec.Parameters); err != nil {
		return fmt.Errorf("failed to unmarshal parameters: %w", err)
	}
	return nil
}

// Validate validates the plugin structure
func (p *Plugin) Validate() error {
	if p.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}
	if p.Kind == "" {
		return fmt.Errorf("kind is required")
	}
	if p.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if p.Metadata.Version == "" {
		return fmt.Errorf("metadata.version is required")
	}
	if p.Spec.DockerCompose.Contents == "" {
		return fmt.Errorf("spec.dockerCompose.contents is required")
	}
	if p.Spec.Parameters.Schema == "" {
		return fmt.Errorf("spec.parameters.$schema is required")
	}
	if p.Spec.Parameters.Type == "" {
		return fmt.Errorf("spec.parameters.type is required")
	}
	if len(p.Spec.Parameters.Properties) == 0 {
		return fmt.Errorf("spec.parameters.properties is required")
	}

	// Validate that all required parameters have corresponding properties
	for _, required := range p.Spec.Parameters.Required {
		if _, ok := p.Spec.Parameters.Properties[required]; !ok {
			return fmt.Errorf("required parameter %s is not defined in properties", required)
		}
	}

	return nil
}

// FabricPeerDetails represents the details of a Fabric peer
type FabricPeerDetails struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	ExternalEndpoint string `json:"externalEndpoint"`
	TLSCert          string `json:"tlsCert"`
	TLSCACert        string `json:"tlsCACert"`
	MspID            string `json:"mspId"`
	OrgID            int64  `json:"orgId"`
}

// FabricOrgDetails represents the details of a Fabric organization
type FabricOrgDetails struct {
	ID          int64  `json:"id"`
	MspID       string `json:"mspId"`
	Description string `json:"description"`
}

// KeyDetails represents the details of a key
type KeyDetails struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// FabricKeyDetails represents the details of a Fabric key
type FabricKeyDetails struct {
	KeyID       int64  `json:"keyId"`
	OrgID       int64  `json:"orgId"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	MspID       string `json:"mspId"`
	Certificate string `json:"certificate"`
}

// Documentation contains plugin documentation information
type Documentation struct {
	// README contains the main documentation for the plugin
	README string `json:"readme" yaml:"readme"`
	// Examples contains example configurations and usage
	Examples []Example `json:"examples,omitempty" yaml:"examples,omitempty"`
	// Troubleshooting contains common issues and their solutions
	Troubleshooting []TroubleshootingItem `json:"troubleshooting,omitempty" yaml:"troubleshooting,omitempty"`
}

// Example represents a usage example for the plugin
type Example struct {
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description" yaml:"description"`
	Parameters  map[string]interface{} `json:"parameters" yaml:"parameters"`
}

// TroubleshootingItem represents a common issue and its solution
type TroubleshootingItem struct {
	Problem     string `json:"problem" yaml:"problem"`
	Solution    string `json:"solution" yaml:"solution"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}
