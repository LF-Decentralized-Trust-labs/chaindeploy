package template

import (
	"context"
	"fmt"

	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// Validation error codes
const (
	ErrCodeInvalidVersion         = "INVALID_VERSION"
	ErrCodeMissingFabricConfig    = "MISSING_FABRIC_CONFIG"
	ErrCodeMissingBesuConfig      = "MISSING_BESU_CONFIG"
	ErrCodeOrgNotFound            = "ORG_NOT_FOUND"
	ErrCodeNodeNotFound           = "NODE_NOT_FOUND"
	ErrCodeMissingRequiredField   = "MISSING_REQUIRED_FIELD"
	ErrCodeMissingVariableBinding = "MISSING_VARIABLE_BINDING"
	ErrCodeInvalidVariableBinding = "INVALID_VARIABLE_BINDING"
	ErrCodeInvalidVariableType    = "INVALID_VARIABLE_TYPE"
)

const (
	WarnCodeVersionNewer = "VERSION_NEWER"
)

// ValidateTemplateImport validates an import request without making changes
func (s *TemplateService) ValidateTemplateImport(ctx context.Context, req *ValidateTemplateRequest) (*ValidateTemplateResponse, error) {
	response := &ValidateTemplateResponse{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
	}

	if err := s.validateTemplateStructure(req, response); err != nil {
		return response, nil
	}

	bindingMap := make(map[string]VariableBinding)
	for _, b := range req.VariableBindings {
		bindingMap[b.VariableName] = b
	}

	for _, varDef := range req.Template.Variables {
		binding, hasBinding := bindingMap[varDef.Name]
		if !hasBinding && varDef.Required {
			response.Valid = false
			response.Errors = append(response.Errors, ValidationError{
				Code:    ErrCodeMissingVariableBinding,
				Message: fmt.Sprintf("Missing required binding for variable '%s'", varDef.Name),
				Field:   fmt.Sprintf("variableBindings[%s]", varDef.Name),
			})
			continue
		}

		if hasBinding {
			s.validateVariableBinding(ctx, varDef, binding, response)
		}
	}

	if response.Valid {
		preview, err := s.buildImportPreview(ctx, req)
		if err != nil {
			response.Valid = false
			response.Errors = append(response.Errors, ValidationError{
				Code:    "PREVIEW_FAILED",
				Message: fmt.Sprintf("Failed to build preview: %v", err),
			})
		} else {
			response.Preview = preview
		}
	}

	return response, nil
}

func (s *TemplateService) validateVariableBinding(ctx context.Context, varDef TemplateVariable, binding VariableBinding, response *ValidateTemplateResponse) {
	switch varDef.Type {
	case TypeOrganization:
		if binding.ExistingOrgID != nil {
			_, err := s.orgService.GetOrganization(ctx, *binding.ExistingOrgID)
			if err != nil {
				response.Valid = false
				response.Errors = append(response.Errors, ValidationError{
					Code:    ErrCodeOrgNotFound,
					Message: fmt.Sprintf("Organization %d not found for variable '%s'", *binding.ExistingOrgID, varDef.Name),
					Field:   fmt.Sprintf("variableBindings[%s].existingOrgId", varDef.Name),
				})
			}
		} else if binding.Properties == nil || binding.Properties["mspId"] == nil {
			response.Valid = false
			response.Errors = append(response.Errors, ValidationError{
				Code:    ErrCodeInvalidVariableBinding,
				Message: fmt.Sprintf("Organization variable '%s' requires existingOrgId or properties.mspId", varDef.Name),
				Field:   fmt.Sprintf("variableBindings[%s]", varDef.Name),
			})
		}

	case TypeNode:
		if binding.ExistingNodeID != nil {
			_, err := s.nodeService.GetNode(ctx, *binding.ExistingNodeID)
			if err != nil {
				response.Valid = false
				response.Errors = append(response.Errors, ValidationError{
					Code:    ErrCodeNodeNotFound,
					Message: fmt.Sprintf("Node %d not found for variable '%s'", *binding.ExistingNodeID, varDef.Name),
					Field:   fmt.Sprintf("variableBindings[%s].existingNodeId", varDef.Name),
				})
			}
		} else if varDef.Required {
			response.Valid = false
			response.Errors = append(response.Errors, ValidationError{
				Code:    ErrCodeInvalidVariableBinding,
				Message: fmt.Sprintf("Node variable '%s' requires existingNodeId", varDef.Name),
				Field:   fmt.Sprintf("variableBindings[%s]", varDef.Name),
			})
		}

	case TypeKey:
		if binding.ExistingKeyID != nil {
			_, err := s.keyMgmt.GetKey(ctx, int(*binding.ExistingKeyID))
			if err != nil {
				response.Valid = false
				response.Errors = append(response.Errors, ValidationError{
					Code:    "KEY_NOT_FOUND",
					Message: fmt.Sprintf("Key %d not found for variable '%s'", *binding.ExistingKeyID, varDef.Name),
					Field:   fmt.Sprintf("variableBindings[%s].existingKeyId", varDef.Name),
				})
			}
		} else if varDef.Required {
			response.Valid = false
			response.Errors = append(response.Errors, ValidationError{
				Code:    ErrCodeInvalidVariableBinding,
				Message: fmt.Sprintf("Key variable '%s' requires existingKeyId", varDef.Name),
				Field:   fmt.Sprintf("variableBindings[%s]", varDef.Name),
			})
		}

	case TypeMspId:
		if binding.Value != nil {
			if mspId, ok := binding.Value.(string); ok {
				if err := ValidateMspId(mspId); err != nil {
					response.Valid = false
					response.Errors = append(response.Errors, ValidationError{
						Code:    ErrCodeInvalidVariableType,
						Message: fmt.Sprintf("Invalid MSP ID for variable '%s': %v", varDef.Name, err),
						Field:   fmt.Sprintf("variableBindings[%s].value", varDef.Name),
					})
				}
			}
		}

	case TypeEthereumAddress:
		if binding.Value != nil {
			if addr, ok := binding.Value.(string); ok {
				if err := ValidateEthereumAddress(addr); err != nil {
					response.Valid = false
					response.Errors = append(response.Errors, ValidationError{
						Code:    ErrCodeInvalidVariableType,
						Message: fmt.Sprintf("Invalid Ethereum address for variable '%s': %v", varDef.Name, err),
						Field:   fmt.Sprintf("variableBindings[%s].value", varDef.Name),
					})
				}
			}
		}
	}
}

func (s *TemplateService) buildImportPreview(ctx context.Context, req *ValidateTemplateRequest) (*ImportPreview, error) {
	preview := &ImportPreview{
		OrganizationsToCreate: []OrgPreview{},
		NodesToCreate:         []NodePreview{},
		ExistingOrgsUsed:      []ExistingOrgPreview{},
		ExistingNodesUsed:     []ExistingNodePreview{},
	}

	networkName := req.Template.Network.Name
	if req.Overrides.NetworkName != nil && *req.Overrides.NetworkName != "" {
		networkName = *req.Overrides.NetworkName
	}

	channelName := ""
	if req.Template.Network.Fabric != nil {
		channelName = req.Template.Network.Fabric.ChannelName
	}
	if req.Overrides.ChannelName != nil && *req.Overrides.ChannelName != "" {
		channelName = *req.Overrides.ChannelName
	}

	description := req.Template.Network.Description
	if req.Overrides.Description != nil {
		description = *req.Overrides.Description
	}

	preview.Network = NetworkPreview{
		Name:        networkName,
		ChannelName: channelName,
		Description: description,
		Platform:    req.Template.Network.Platform,
	}

	bindingMap := make(map[string]VariableBinding)
	for _, b := range req.VariableBindings {
		bindingMap[b.VariableName] = b
	}

	for _, varDef := range req.Template.Variables {
		binding, ok := bindingMap[varDef.Name]
		if !ok {
			continue
		}

		switch varDef.Type {
		case TypeOrganization:
			if binding.ExistingOrgID != nil {
				org, err := s.orgService.GetOrganization(ctx, *binding.ExistingOrgID)
				if err == nil {
					preview.ExistingOrgsUsed = append(preview.ExistingOrgsUsed, ExistingOrgPreview{
						TemplateID: varDef.Name,
						OrgID:      org.ID,
						MspID:      org.MspID,
					})
				}
			}

		case TypeNode:
			if binding.ExistingNodeID != nil {
				node, err := s.nodeService.GetNode(ctx, *binding.ExistingNodeID)
				if err == nil {
					nodeType := ""
					switch node.NodeType {
					case nodetypes.NodeTypeFabricPeer:
						nodeType = "peer"
					case nodetypes.NodeTypeFabricOrderer:
						nodeType = "orderer"
					case nodetypes.NodeTypeBesuFullnode:
						nodeType = "besu"
					}
					preview.ExistingNodesUsed = append(preview.ExistingNodesUsed, ExistingNodePreview{
						TemplateID: varDef.Name,
						NodeID:     node.ID,
						Name:       node.Name,
						Type:       nodeType,
					})
				}
			}
		}
	}

	// Add chaincode previews
	for _, cc := range req.Template.Chaincodes {
		cp := ChaincodePreview{
			Name:     cc.Name,
			Platform: cc.Platform,
		}
		if cc.Fabric != nil {
			cp.Version = cc.Fabric.Version
		}
		preview.Chaincodes = append(preview.Chaincodes, cp)
	}

	return preview, nil
}

func (s *TemplateService) validateTemplateStructure(req *ValidateTemplateRequest, response *ValidateTemplateResponse) error {
	if req.Template.Version == "" {
		response.Valid = false
		response.Errors = append(response.Errors, ValidationError{
			Code:    ErrCodeInvalidVersion,
			Message: "Template version is required",
			Field:   "template.version",
		})
	} else if req.Template.Version > TemplateVersion {
		response.Warnings = append(response.Warnings, ValidationWarning{
			Code:       WarnCodeVersionNewer,
			Message:    fmt.Sprintf("Template version %s is newer than supported version %s", req.Template.Version, TemplateVersion),
			Suggestion: "Some features may not be fully supported",
		})
	}

	switch req.Template.Network.Platform {
	case "fabric":
		if req.Template.Network.Fabric == nil {
			response.Valid = false
			response.Errors = append(response.Errors, ValidationError{
				Code:    ErrCodeMissingFabricConfig,
				Message: "Fabric configuration is required for Fabric networks",
				Field:   "template.network.fabric",
			})
			return nil
		}
		if req.Template.Network.Fabric.ChannelName == "" && (req.Overrides.ChannelName == nil || *req.Overrides.ChannelName == "") {
			response.Valid = false
			response.Errors = append(response.Errors, ValidationError{
				Code:    ErrCodeMissingRequiredField,
				Message: "Channel name is required (either in template or overrides)",
				Field:   "channelName",
			})
		}

	case "besu":
		if req.Template.Network.Besu == nil {
			response.Valid = false
			response.Errors = append(response.Errors, ValidationError{
				Code:    ErrCodeMissingBesuConfig,
				Message: "Besu configuration is required for Besu networks",
				Field:   "template.network.besu",
			})
			return nil
		}
		if req.Template.Network.Besu.ChainID == 0 {
			response.Valid = false
			response.Errors = append(response.Errors, ValidationError{
				Code:    ErrCodeMissingRequiredField,
				Message: "Chain ID is required for Besu networks",
				Field:   "template.network.besu.chainId",
			})
		}

	default:
		response.Valid = false
		response.Errors = append(response.Errors, ValidationError{
			Code:    ErrCodeMissingFabricConfig,
			Message: fmt.Sprintf("Unsupported platform: %s (supported: 'fabric', 'besu')", req.Template.Network.Platform),
			Field:   "template.network.platform",
		})
	}

	return nil
}
