package template

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	fabricservice "github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	keymanagement "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
)

// VariableResolver handles resolution of template variables
type VariableResolver struct {
	orgService  *fabricservice.OrganizationService
	nodeService *nodeservice.NodeService
	keyMgmt     *keymanagement.KeyManagementService
}

// NewVariableResolver creates a new VariableResolver
func NewVariableResolver(
	orgService *fabricservice.OrganizationService,
	nodeService *nodeservice.NodeService,
	keyMgmt *keymanagement.KeyManagementService,
) *VariableResolver {
	return &VariableResolver{
		orgService:  orgService,
		nodeService: nodeService,
		keyMgmt:     keyMgmt,
	}
}

// variablePattern matches ${variableName} or ${variableName.property}
var variablePattern = regexp.MustCompile(`\$\{([a-zA-Z][a-zA-Z0-9_]*)(?:\.([a-zA-Z][a-zA-Z0-9_]*))?\}`)

// ResolveBindings resolves all variable bindings and creates a VariableContext
func (r *VariableResolver) ResolveBindings(
	ctx context.Context,
	variables []TemplateVariable,
	bindings []VariableBinding,
) (*VariableContext, error) {
	vc := NewVariableContext()

	varDefs := make(map[string]TemplateVariable)
	for _, v := range variables {
		varDefs[v.Name] = v
	}

	bindingMap := make(map[string]VariableBinding)
	for _, b := range bindings {
		bindingMap[b.VariableName] = b
	}

	for _, varDef := range variables {
		binding, hasBinding := bindingMap[varDef.Name]
		if !hasBinding && varDef.Required {
			return nil, fmt.Errorf("missing required binding for variable '%s'", varDef.Name)
		}

		if !hasBinding {
			if varDef.Default != nil {
				vc.Set(varDef.Name, &ResolvedVariable{
					Name:  varDef.Name,
					Type:  varDef.Type,
					Value: varDef.Default,
				})
			}
			continue
		}

		resolved, err := r.resolveBinding(ctx, varDef, binding)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve variable '%s': %w", varDef.Name, err)
		}

		vc.Set(varDef.Name, resolved)
	}

	return vc, nil
}

func (r *VariableResolver) resolveBinding(
	ctx context.Context,
	varDef TemplateVariable,
	binding VariableBinding,
) (*ResolvedVariable, error) {
	resolved := &ResolvedVariable{
		Name: varDef.Name,
		Type: varDef.Type,
	}

	switch varDef.Type {
	case TypeOrganization:
		return r.resolveOrganizationBinding(ctx, varDef, binding)
	case TypeNode:
		return r.resolveNodeBinding(ctx, varDef, binding)
	case TypeKey:
		return r.resolveKeyBinding(ctx, varDef, binding)
	case TypeString, TypeMspId, TypeEthereumAddress, TypePublicKey, TypeHostPort:
		if binding.Value != nil {
			resolved.Value = binding.Value
		} else if binding.Properties != nil && len(binding.Properties) > 0 {
			for _, v := range binding.Properties {
				resolved.Value = v
				break
			}
		}
		return resolved, nil
	case TypeInteger:
		if binding.Value != nil {
			resolved.Value = binding.Value
		}
		return resolved, nil
	default:
		return nil, fmt.Errorf("unsupported variable type: %s", varDef.Type)
	}
}

func (r *VariableResolver) resolveOrganizationBinding(
	ctx context.Context,
	varDef TemplateVariable,
	binding VariableBinding,
) (*ResolvedVariable, error) {
	resolved := &ResolvedVariable{
		Name:       varDef.Name,
		Type:       varDef.Type,
		Properties: make(map[string]interface{}),
	}

	if binding.ExistingOrgID != nil {
		org, err := r.orgService.GetOrganization(ctx, *binding.ExistingOrgID)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing organization %d: %w", *binding.ExistingOrgID, err)
		}

		resolved.ResolvedOrgID = binding.ExistingOrgID
		resolved.Properties["mspId"] = org.MspID
		resolved.Properties["description"] = org.Description.String

		return resolved, nil
	}

	if binding.Properties != nil {
		resolved.Properties = binding.Properties
	}

	for _, prop := range varDef.Properties {
		if prop.Required {
			if _, ok := resolved.Properties[prop.Name]; !ok {
				return nil, fmt.Errorf("missing required property '%s' for organization variable '%s'", prop.Name, varDef.Name)
			}
		}
	}

	return resolved, nil
}

func (r *VariableResolver) resolveNodeBinding(
	ctx context.Context,
	varDef TemplateVariable,
	binding VariableBinding,
) (*ResolvedVariable, error) {
	resolved := &ResolvedVariable{
		Name:       varDef.Name,
		Type:       varDef.Type,
		Properties: make(map[string]interface{}),
	}

	if binding.ExistingNodeID != nil {
		node, err := r.nodeService.GetNode(ctx, *binding.ExistingNodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing node %d: %w", *binding.ExistingNodeID, err)
		}

		resolved.ResolvedNodeID = binding.ExistingNodeID
		resolved.Properties["name"] = node.Name
		resolved.Properties["endpoint"] = node.Endpoint

		return resolved, nil
	}

	if binding.Properties != nil {
		resolved.Properties = binding.Properties
	}

	return resolved, nil
}

func (r *VariableResolver) resolveKeyBinding(
	ctx context.Context,
	varDef TemplateVariable,
	binding VariableBinding,
) (*ResolvedVariable, error) {
	resolved := &ResolvedVariable{
		Name:       varDef.Name,
		Type:       varDef.Type,
		Properties: make(map[string]interface{}),
	}

	if binding.ExistingKeyID != nil {
		key, err := r.keyMgmt.GetKey(ctx, int(*binding.ExistingKeyID))
		if err != nil {
			return nil, fmt.Errorf("failed to get existing key %d: %w", *binding.ExistingKeyID, err)
		}

		resolved.ResolvedKeyID = binding.ExistingKeyID
		resolved.Properties["publicKey"] = key.PublicKey

		return resolved, nil
	}

	if binding.Properties != nil {
		resolved.Properties = binding.Properties
	}

	return resolved, nil
}

// SubstituteVariables replaces all ${var} and ${var.prop} placeholders in a string
func (r *VariableResolver) SubstituteVariables(input string, vc *VariableContext) (string, error) {
	var lastErr error

	result := variablePattern.ReplaceAllStringFunc(input, func(match string) string {
		parts := variablePattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			lastErr = fmt.Errorf("invalid variable reference: %s", match)
			return match
		}

		varName := parts[1]
		propName := ""
		if len(parts) > 2 {
			propName = parts[2]
		}

		resolved, ok := vc.Get(varName)
		if !ok {
			lastErr = fmt.Errorf("undefined variable: %s", varName)
			return match
		}

		if propName != "" {
			if resolved.Properties == nil {
				lastErr = fmt.Errorf("variable '%s' has no properties", varName)
				return match
			}
			val, ok := resolved.Properties[propName]
			if !ok {
				lastErr = fmt.Errorf("variable '%s' has no property '%s'", varName, propName)
				return match
			}
			return fmt.Sprintf("%v", val)
		}

		if resolved.Value != nil {
			return fmt.Sprintf("%v", resolved.Value)
		}

		lastErr = fmt.Errorf("variable '%s' has no value", varName)
		return match
	})

	return result, lastErr
}

// SubstituteInPolicies substitutes variables in a policy map
func (r *VariableResolver) SubstituteInPolicies(policies map[string]PolicyTemplate, vc *VariableContext) (map[string]PolicyTemplate, error) {
	if policies == nil {
		return nil, nil
	}

	result := make(map[string]PolicyTemplate)
	for name, policy := range policies {
		newRule, err := r.SubstituteVariables(policy.Rule, vc)
		if err != nil {
			return nil, fmt.Errorf("failed to substitute variables in policy '%s': %w", name, err)
		}
		result[name] = PolicyTemplate{
			Type: policy.Type,
			Rule: newRule,
		}
	}

	return result, nil
}

// ValidateMspId validates that a string is a valid MSP ID
func ValidateMspId(mspId string) error {
	if mspId == "" {
		return fmt.Errorf("MSP ID cannot be empty")
	}
	if !strings.HasSuffix(mspId, "MSP") {
		return fmt.Errorf("MSP ID must end with 'MSP', got: %s", mspId)
	}
	return nil
}

// ValidateEthereumAddress validates an Ethereum address format
func ValidateEthereumAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("Ethereum address cannot be empty")
	}
	if !strings.HasPrefix(addr, "0x") {
		return fmt.Errorf("Ethereum address must start with '0x'")
	}
	if len(addr) != 42 {
		return fmt.Errorf("Ethereum address must be 42 characters (0x + 40 hex), got %d", len(addr))
	}
	for _, c := range addr[2:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return fmt.Errorf("Ethereum address contains invalid character: %c", c)
		}
	}
	return nil
}
