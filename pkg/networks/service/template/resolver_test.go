package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMspId(t *testing.T) {
	tests := []struct {
		name    string
		mspId   string
		wantErr bool
	}{
		{"valid", "Org1MSP", false},
		{"valid long", "MyOrganizationMSP", false},
		{"empty", "", true},
		{"no suffix", "Org1", true},
		{"wrong suffix", "Org1msp", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMspId(tt.mspId)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEthereumAddress(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"valid", "0xfe3b557e8fb62b89f4916b721be55ceb828dbd73", false},
		{"valid uppercase", "0xFE3B557E8FB62B89F4916B721BE55CEB828DBD73", false},
		{"empty", "", true},
		{"no prefix", "fe3b557e8fb62b89f4916b721be55ceb828dbd73", true},
		{"too short", "0xfe3b557e8fb62b89f4916b721be55ceb828dbd7", true},
		{"too long", "0xfe3b557e8fb62b89f4916b721be55ceb828dbd733", true},
		{"invalid char", "0xge3b557e8fb62b89f4916b721be55ceb828dbd73", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEthereumAddress(tt.addr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSubstituteVariables(t *testing.T) {
	resolver := &VariableResolver{}
	vc := NewVariableContext()
	vc.Set("peerOrg1", &ResolvedVariable{
		Name:       "peerOrg1",
		Type:       TypeOrganization,
		Properties: map[string]interface{}{"mspId": "Org1MSP"},
	})
	vc.Set("ordererOrg1", &ResolvedVariable{
		Name:       "ordererOrg1",
		Type:       TypeOrganization,
		Properties: map[string]interface{}{"mspId": "OrdererMSP"},
	})

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			"simple substitution",
			"OR('${peerOrg1.mspId}.member')",
			"OR('Org1MSP.member')",
			false,
		},
		{
			"multiple substitutions",
			"OR('${peerOrg1.mspId}.member', '${ordererOrg1.mspId}.admin')",
			"OR('Org1MSP.member', 'OrdererMSP.admin')",
			false,
		},
		{
			"no variables",
			"ANY Readers",
			"ANY Readers",
			false,
		},
		{
			"undefined variable",
			"${nonexistent.mspId}",
			"${nonexistent.mspId}",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.SubstituteVariables(tt.input, vc)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSubstituteInPolicies(t *testing.T) {
	resolver := &VariableResolver{}
	vc := NewVariableContext()
	vc.Set("peerOrg1", &ResolvedVariable{
		Name:       "peerOrg1",
		Type:       TypeOrganization,
		Properties: map[string]interface{}{"mspId": "Org1MSP"},
	})

	policies := map[string]PolicyTemplate{
		"Endorsement": {Type: "Signature", Rule: "OR('${peerOrg1.mspId}.member')"},
		"Readers":     {Type: "ImplicitMeta", Rule: "ANY Readers"},
	}

	result, err := resolver.SubstituteInPolicies(policies, vc)
	require.NoError(t, err)

	assert.Equal(t, "OR('Org1MSP.member')", result["Endorsement"].Rule)
	assert.Equal(t, "Signature", result["Endorsement"].Type)
	assert.Equal(t, "ANY Readers", result["Readers"].Rule)
}

func TestSubstituteInPolicies_Nil(t *testing.T) {
	resolver := &VariableResolver{}
	vc := NewVariableContext()

	result, err := resolver.SubstituteInPolicies(nil, vc)
	assert.NoError(t, err)
	assert.Nil(t, result)
}
