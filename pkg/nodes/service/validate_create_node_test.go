package service

import (
	"testing"

	"github.com/chainlaunch/chainlaunch/pkg/errors"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// newTestNodeService creates a minimal NodeService for testing validation methods
func newTestNodeService() *NodeService {
	return &NodeService{}
}

func TestValidateCreateNodeRequest_EmptyName(t *testing.T) {
	s := newTestNodeService()
	req := CreateNodeRequest{
		BlockchainPlatform: types.PlatformFabric,
	}

	err := s.validateCreateNodeRequest(req)
	if err == nil {
		t.Fatal("expected validation error for empty name")
	}

	mve, ok := errors.GetMultiValidationError(err)
	if !ok {
		t.Fatalf("expected MultiValidationError, got %T: %v", err, err)
	}

	if !mve.HasErrors() {
		t.Error("expected HasErrors() to be true")
	}

	// Should contain the name error
	found := false
	for _, fe := range mve.Errors {
		if fe.Field == "name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'name' field error in MultiValidationError")
	}
}

func TestValidateCreateNodeRequest_EmptyPlatform(t *testing.T) {
	s := newTestNodeService()
	req := CreateNodeRequest{
		Name: "test-node",
	}

	err := s.validateCreateNodeRequest(req)
	if err == nil {
		t.Fatal("expected validation error for empty platform")
	}

	mve, ok := errors.GetMultiValidationError(err)
	if !ok {
		t.Fatalf("expected MultiValidationError, got %T: %v", err, err)
	}

	found := false
	for _, fe := range mve.Errors {
		if fe.Field == "blockchainPlatform" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'blockchainPlatform' field error in MultiValidationError")
	}
}

func TestValidateCreateNodeRequest_UnsupportedPlatform(t *testing.T) {
	s := newTestNodeService()
	req := CreateNodeRequest{
		Name:               "test-node",
		BlockchainPlatform: "UNKNOWN",
	}

	err := s.validateCreateNodeRequest(req)
	if err == nil {
		t.Fatal("expected validation error for unsupported platform")
	}

	mve, ok := errors.GetMultiValidationError(err)
	if !ok {
		t.Fatalf("expected MultiValidationError, got %T: %v", err, err)
	}

	found := false
	for _, fe := range mve.Errors {
		if fe.Field == "blockchainPlatform" && fe.Value == "UNKNOWN" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'blockchainPlatform' field error with value 'UNKNOWN'")
	}
}

func TestValidateCreateNodeRequest_FabricMissingConfig(t *testing.T) {
	s := newTestNodeService()
	req := CreateNodeRequest{
		Name:               "test-node",
		BlockchainPlatform: types.PlatformFabric,
	}

	err := s.validateCreateNodeRequest(req)
	if err == nil {
		t.Fatal("expected validation error for missing fabric config")
	}

	mve, ok := errors.GetMultiValidationError(err)
	if !ok {
		t.Fatalf("expected MultiValidationError, got %T: %v", err, err)
	}

	found := false
	for _, fe := range mve.Errors {
		if fe.Field == "fabricPeer/fabricOrderer" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'fabricPeer/fabricOrderer' field error")
	}
}

func TestValidateCreateNodeRequest_FabricBothPeerAndOrderer(t *testing.T) {
	s := newTestNodeService()
	req := CreateNodeRequest{
		Name:               "test-node",
		BlockchainPlatform: types.PlatformFabric,
		FabricPeer:         &types.FabricPeerConfig{},
		FabricOrderer:      &types.FabricOrdererConfig{},
	}

	err := s.validateCreateNodeRequest(req)
	if err == nil {
		t.Fatal("expected validation error for both peer and orderer")
	}

	mve, ok := errors.GetMultiValidationError(err)
	if !ok {
		t.Fatalf("expected MultiValidationError, got %T: %v", err, err)
	}

	found := false
	for _, fe := range mve.Errors {
		if fe.Field == "fabricPeer/fabricOrderer" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'fabricPeer/fabricOrderer' field error for both specified")
	}
}

func TestValidateCreateNodeRequest_BesuMissingConfig(t *testing.T) {
	s := newTestNodeService()
	req := CreateNodeRequest{
		Name:               "test-node",
		BlockchainPlatform: types.PlatformBesu,
	}

	err := s.validateCreateNodeRequest(req)
	if err == nil {
		t.Fatal("expected validation error for missing besu config")
	}

	mve, ok := errors.GetMultiValidationError(err)
	if !ok {
		t.Fatalf("expected MultiValidationError, got %T: %v", err, err)
	}

	found := false
	for _, fe := range mve.Errors {
		if fe.Field == "besuNode" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'besuNode' field error")
	}
}

func TestValidateCreateNodeRequest_MultipleErrors(t *testing.T) {
	s := newTestNodeService()
	// Empty name AND empty platform should produce multiple errors
	req := CreateNodeRequest{}

	err := s.validateCreateNodeRequest(req)
	if err == nil {
		t.Fatal("expected validation error")
	}

	mve, ok := errors.GetMultiValidationError(err)
	if !ok {
		t.Fatalf("expected MultiValidationError, got %T: %v", err, err)
	}

	if len(mve.Errors) < 2 {
		t.Errorf("expected at least 2 validation errors, got %d: %v", len(mve.Errors), mve.Errors)
	}

	// Should have both name and platform errors
	hasName := false
	hasPlatform := false
	for _, fe := range mve.Errors {
		if fe.Field == "name" {
			hasName = true
		}
		if fe.Field == "blockchainPlatform" {
			hasPlatform = true
		}
	}
	if !hasName {
		t.Error("expected 'name' field error")
	}
	if !hasPlatform {
		t.Error("expected 'blockchainPlatform' field error")
	}
}

func TestValidateCreateNodeRequest_FabricPeerWithInvalidConfig(t *testing.T) {
	s := newTestNodeService()
	req := CreateNodeRequest{
		Name:               "test-node",
		BlockchainPlatform: types.PlatformFabric,
		FabricPeer:         &types.FabricPeerConfig{
			// Missing Name, OrganizationID, etc. - will fail validation
		},
	}

	err := s.validateCreateNodeRequest(req)
	if err == nil {
		t.Fatal("expected validation error for invalid peer config")
	}

	mve, ok := errors.GetMultiValidationError(err)
	if !ok {
		t.Fatalf("expected MultiValidationError, got %T: %v", err, err)
	}

	found := false
	for _, fe := range mve.Errors {
		if fe.Field == "fabricPeer" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'fabricPeer' field error for invalid config")
	}
}
