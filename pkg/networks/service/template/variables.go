package template

// VariableType defines the type of a template variable
type VariableType string

const (
	TypeString  VariableType = "string"
	TypeInteger VariableType = "integer"

	// Fabric-specific types
	TypeMspId VariableType = "mspId"

	// Besu-specific types
	TypeEthereumAddress VariableType = "ethereumAddress"
	TypePublicKey       VariableType = "publicKey"

	// Network types
	TypeHostPort VariableType = "hostPort"

	// Composite types
	TypeOrganization VariableType = "organization"
	TypeKey          VariableType = "key"
	TypeNode         VariableType = "node"
)

// VariableScope defines where a variable applies
type VariableScope string

const (
	ScopeNetwork      VariableScope = "network"
	ScopeOrganization VariableScope = "organization"
	ScopeNode         VariableScope = "node"
	ScopeValidator    VariableScope = "validator"
)

// VariableValidation defines validation rules for a variable
type VariableValidation struct {
	MinLength *int    `json:"minLength,omitempty"`
	MaxLength *int    `json:"maxLength,omitempty"`
	Pattern   *string `json:"pattern,omitempty"`
	MinValue  *int    `json:"minValue,omitempty"`
	MaxValue  *int    `json:"maxValue,omitempty"`
}

// TemplateVariable defines a variable in a template
type TemplateVariable struct {
	Name        string              `json:"name"`
	Type        VariableType        `json:"type"`
	Description string              `json:"description,omitempty"`
	Required    bool                `json:"required"`
	Default     interface{}         `json:"default,omitempty"`
	Scope       VariableScope       `json:"scope"`
	Platform    []string            `json:"platform,omitempty"`
	Properties  []TemplateVariable  `json:"properties,omitempty"`
	Validation  *VariableValidation `json:"validation,omitempty"`
}

// VariableBinding represents a user-provided binding for a template variable
type VariableBinding struct {
	VariableName string `json:"variableName"`

	// Direct value binding (for simple types)
	Value interface{} `json:"value,omitempty"`

	// Property bindings (for composite types like organization)
	Properties map[string]interface{} `json:"properties,omitempty"`

	// Reference to existing resources
	ExistingOrgID  *int64 `json:"existingOrgId,omitempty"`
	ExistingNodeID *int64 `json:"existingNodeId,omitempty"`
	ExistingKeyID  *int64 `json:"existingKeyId,omitempty"`
}

// ResolvedVariable represents a variable after resolution
type ResolvedVariable struct {
	Name       string                 `json:"name"`
	Type       VariableType           `json:"type"`
	Value      interface{}            `json:"value,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`

	ResolvedOrgID  *int64 `json:"resolvedOrgId,omitempty"`
	ResolvedNodeID *int64 `json:"resolvedNodeId,omitempty"`
	ResolvedKeyID  *int64 `json:"resolvedKeyId,omitempty"`
}

// VariableContext holds resolved variables for use during import
type VariableContext struct {
	Variables map[string]*ResolvedVariable
}

// NewVariableContext creates a new variable context
func NewVariableContext() *VariableContext {
	return &VariableContext{
		Variables: make(map[string]*ResolvedVariable),
	}
}

// Get retrieves a resolved variable by name
func (vc *VariableContext) Get(name string) (*ResolvedVariable, bool) {
	v, ok := vc.Variables[name]
	return v, ok
}

// Set stores a resolved variable
func (vc *VariableContext) Set(name string, v *ResolvedVariable) {
	vc.Variables[name] = v
}

// GetProperty retrieves a specific property from a variable
func (vc *VariableContext) GetProperty(varName, propName string) (interface{}, bool) {
	v, ok := vc.Variables[varName]
	if !ok {
		return nil, false
	}
	if v.Properties == nil {
		return nil, false
	}
	val, ok := v.Properties[propName]
	return val, ok
}

// GetMspID is a convenience method for getting the mspId property
func (vc *VariableContext) GetMspID(varName string) (string, bool) {
	val, ok := vc.GetProperty(varName, "mspId")
	if !ok {
		return "", false
	}
	mspID, ok := val.(string)
	return mspID, ok
}

// CreateOrganizationVariable creates a standard organization variable definition
func CreateOrganizationVariable(name, description string, platform []string) TemplateVariable {
	return TemplateVariable{
		Name:        name,
		Type:        TypeOrganization,
		Description: description,
		Required:    true,
		Scope:       ScopeOrganization,
		Platform:    platform,
		Properties: []TemplateVariable{
			{
				Name:     "mspId",
				Type:     TypeMspId,
				Required: true,
			},
		},
	}
}

// CreateNodeVariable creates a standard node variable definition
func CreateNodeVariable(name, description string, platform []string, required bool) TemplateVariable {
	return TemplateVariable{
		Name:        name,
		Type:        TypeNode,
		Description: description,
		Required:    required,
		Scope:       ScopeNode,
		Platform:    platform,
	}
}

// CreateValidatorVariable creates a standard validator variable definition (for Besu)
func CreateValidatorVariable(name, description string) TemplateVariable {
	return TemplateVariable{
		Name:        name,
		Type:        TypeKey,
		Description: description,
		Required:    true,
		Scope:       ScopeValidator,
		Platform:    []string{"besu"},
		Properties: []TemplateVariable{
			{
				Name:     "ethereumAddress",
				Type:     TypeEthereumAddress,
				Required: true,
			},
			{
				Name:     "publicKey",
				Type:     TypePublicKey,
				Required: false,
			},
		},
	}
}
