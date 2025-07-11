package projects

import (
	"context"
	"fmt"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	fabricService "github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	keyMgmtService "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service"
	"go.uber.org/zap"
)

// ProjectLifecycleParams contains common parameters for all lifecycle hooks
type ProjectLifecycleParams struct {
	ProjectID         int64
	ProjectName       string
	ProjectSlug       string
	NetworkID         int64
	NetworkName       string
	Platform          string
	Boilerplate       string
	EndorsementPolicy string
}

// PreStartParams contains parameters for the PreStart lifecycle hook
type PreStartParams struct {
	ProjectLifecycleParams
	Image       string
	Port        int
	Command     string
	Args        []string
	Environment map[string]string
	HostIP      string
}

// PreStartResult contains the result of the PreStart lifecycle hook
type PreStartResult struct {
	Environment map[string]string
}

// PostStartParams contains parameters for the PostStart hook
type PostStartParams struct {
	ProjectLifecycleParams
	ContainerID string
	Image       string
	Port        int
	StartedAt   time.Time
	Status      string
	HostIP      string // IP address where the smart contract will be deployed
}

// PreStopParams contains parameters for the PreStop hook
type PreStopParams struct {
	ProjectLifecycleParams
	ContainerID string
	StartedAt   time.Time
}

// PostStopParams contains parameters for the PostStop hook
type PostStopParams struct {
	ProjectLifecycleParams
	ContainerID string
	StartedAt   time.Time
	StoppedAt   time.Time
}

// PlatformLifecycle defines the interface for platform-specific project lifecycle hooks
type PlatformLifecycle interface {
	// PreStart is called before starting the project container
	// It can be used to prepare the environment, validate configuration, etc.
	PreStart(ctx context.Context, params PreStartParams) (*PreStartResult, error)

	// PostStart is called after the project container has started
	// It can be used to perform platform-specific setup, like installing chaincode
	PostStart(ctx context.Context, params PostStartParams) error

	// PreStop is called before stopping the project container
	// It can be used to perform cleanup or save state
	PreStop(ctx context.Context, params PreStopParams) error

	// PostStop is called after the project container has stopped
	// It can be used to perform final cleanup or state updates
	PostStop(ctx context.Context, params PostStopParams) error
}

// GetPlatformLifecycle returns the appropriate lifecycle implementation for the given platform
func GetPlatformLifecycle(platform string, queries *db.Queries, orgService *fabricService.OrganizationService, keyMgmtService *keyMgmtService.KeyManagementService, networkService *service.NetworkService, logger *zap.Logger) (PlatformLifecycle, error) {
	switch platform {
	case "fabric":
		return NewFabricLifecycle(queries, logger, orgService, keyMgmtService, networkService), nil
	// Add more platform cases here as needed
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}
