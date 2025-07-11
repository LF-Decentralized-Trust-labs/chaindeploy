package xsource

import (
	"context"
	"fmt"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	key "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	ptypes "github.com/chainlaunch/chainlaunch/pkg/plugin/types"
)

// Registry manages x-source handlers
type Registry struct {
	handlers map[XSourceType]XSourceHandler
}

// NewRegistry creates a new registry with the default handlers
func NewRegistry(queries *db.Queries, nodeService *nodeservice.NodeService, keyManagement *key.KeyManagementService) *Registry {
	r := &Registry{
		handlers: make(map[XSourceType]XSourceHandler),
	}

	// Register default handlers
	r.Register(NewFabricKeyHandler(queries, nodeService, keyManagement))
	r.Register(NewFabricPeerHandler(queries, nodeService))
	r.Register(NewFileHandler())

	return r
}

// Register adds a new handler to the registry
func (r *Registry) Register(handler XSourceHandler) {
	r.handlers[handler.GetType()] = handler
}

// GetHandler returns the handler for the specified x-source type
func (r *Registry) GetHandler(xSourceType XSourceType) (XSourceHandler, error) {
	handler, ok := r.handlers[xSourceType]
	if !ok {
		return nil, fmt.Errorf("no handler registered for x-source type: %s", xSourceType)
	}
	return handler, nil
}

// ValidateAndProcess validates and processes an x-source value
func (r *Registry) ValidateAndProcess(ctx context.Context, xSourceType XSourceType, key string, value interface{}) (interface{}, error) {
	handler, err := r.GetHandler(xSourceType)
	if err != nil {
		return nil, err
	}

	xSourceValue, err := handler.CreateValue(key, value)
	if err != nil {
		return nil, err
	}

	if err := xSourceValue.Validate(ctx); err != nil {
		return nil, err
	}

	return xSourceValue.GetValue(ctx, ptypes.ParameterSpec{})
}

// ListOptions returns the valid options for the specified x-source type
func (r *Registry) ListOptions(ctx context.Context, xSourceType XSourceType) ([]OptionItem, error) {
	handler, err := r.GetHandler(xSourceType)
	if err != nil {
		return nil, err
	}

	return handler.ListOptions(ctx)
}
