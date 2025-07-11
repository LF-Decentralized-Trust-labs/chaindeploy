package xsource

import (
	"context"
	"fmt"

	"github.com/chainlaunch/chainlaunch/pkg/plugin/types"
)

// XSourceType represents the type of x-source
type XSourceType string

const (
	FabricKey  XSourceType = "fabric-key"
	FabricPeer XSourceType = "fabric-peer"
	FabricOrg  XSourceType = "fabric-org"
	KeyStore   XSourceType = "keyStore"
	File       XSourceType = "file"
)

// VolumeMount represents a volume mount configuration
type VolumeMount struct {
	Source      string
	Target      string
	Type        string // "bind" or "volume"
	ReadOnly    bool
	Description string
}

// XSourceValue represents a value that can be validated and processed
type XSourceValue interface {
	// Validate checks if the value is valid for this x-source type
	Validate(ctx context.Context) error
	// GetValue returns the processed value that can be used in templates
	GetValue(ctx context.Context, spec types.ParameterSpec) (interface{}, error)
	// GetValidationValue returns the value used for validation
	GetValidationValue() string
	// GetVolumeMounts returns the volume mounts needed for this x-source
	GetVolumeMounts(ctx context.Context) ([]VolumeMount, error)
}

// XSourceHandler defines the interface for handling x-source types
type XSourceHandler interface {
	// GetType returns the type of x-source this handler manages
	GetType() XSourceType
	// CreateValue creates a new XSourceValue from the raw input
	CreateValue(key string, rawValue interface{}) (XSourceValue, error)
	// ListOptions returns the list of valid options for this x-source type
	ListOptions(ctx context.Context) ([]OptionItem, error)
}

// OptionItem represents a selectable option for an x-source
type OptionItem struct {
	Label string
	Value string
}

// BaseXSourceValue provides common functionality for x-source values
type BaseXSourceValue struct {
	Key      string
	RawValue interface{}
}

// GetValidationValue returns the string representation of the value for validation
func (b *BaseXSourceValue) GetValidationValue() string {
	return fmt.Sprintf("%v", b.RawValue)
}

// GetVolumeMounts returns an empty slice of volume mounts
func (b *BaseXSourceValue) GetVolumeMounts(ctx context.Context) ([]VolumeMount, error) {
	return []VolumeMount{}, nil
}
