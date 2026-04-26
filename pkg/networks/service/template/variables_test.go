package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateOrganizationVariable(t *testing.T) {
	v := CreateOrganizationVariable("peerOrg1", "Peer org 1", []string{"fabric"})

	assert.Equal(t, "peerOrg1", v.Name)
	assert.Equal(t, TypeOrganization, v.Type)
	assert.True(t, v.Required)
	assert.Equal(t, ScopeOrganization, v.Scope)
	assert.Equal(t, []string{"fabric"}, v.Platform)
	assert.Len(t, v.Properties, 1)
	assert.Equal(t, "mspId", v.Properties[0].Name)
	assert.Equal(t, TypeMspId, v.Properties[0].Type)
	assert.True(t, v.Properties[0].Required)
}

func TestCreateNodeVariable(t *testing.T) {
	v := CreateNodeVariable("peer1", "Peer node 1", []string{"fabric"}, true)

	assert.Equal(t, "peer1", v.Name)
	assert.Equal(t, TypeNode, v.Type)
	assert.True(t, v.Required)
	assert.Equal(t, ScopeNode, v.Scope)

	v2 := CreateNodeVariable("peer2", "Peer node 2", []string{"fabric"}, false)
	assert.False(t, v2.Required)
}

func TestCreateValidatorVariable(t *testing.T) {
	v := CreateValidatorVariable("validator1", "Validator 1 key")

	assert.Equal(t, "validator1", v.Name)
	assert.Equal(t, TypeKey, v.Type)
	assert.True(t, v.Required)
	assert.Equal(t, ScopeValidator, v.Scope)
	assert.Equal(t, []string{"besu"}, v.Platform)
	assert.Len(t, v.Properties, 2)
	assert.Equal(t, "ethereumAddress", v.Properties[0].Name)
	assert.Equal(t, "publicKey", v.Properties[1].Name)
}

func TestVariableContext(t *testing.T) {
	vc := NewVariableContext()

	// Test Set and Get
	resolved := &ResolvedVariable{
		Name:       "org1",
		Type:       TypeOrganization,
		Properties: map[string]interface{}{"mspId": "Org1MSP"},
	}
	vc.Set("org1", resolved)

	got, ok := vc.Get("org1")
	assert.True(t, ok)
	assert.Equal(t, "org1", got.Name)

	// Test missing
	_, ok = vc.Get("nonexistent")
	assert.False(t, ok)

	// Test GetProperty
	val, ok := vc.GetProperty("org1", "mspId")
	assert.True(t, ok)
	assert.Equal(t, "Org1MSP", val)

	_, ok = vc.GetProperty("org1", "missing")
	assert.False(t, ok)

	_, ok = vc.GetProperty("nonexistent", "mspId")
	assert.False(t, ok)

	// Test GetMspID
	mspId, ok := vc.GetMspID("org1")
	assert.True(t, ok)
	assert.Equal(t, "Org1MSP", mspId)
}

func TestVariableContext_NilProperties(t *testing.T) {
	vc := NewVariableContext()
	vc.Set("simple", &ResolvedVariable{
		Name:  "simple",
		Type:  TypeString,
		Value: "hello",
	})

	_, ok := vc.GetProperty("simple", "anything")
	assert.False(t, ok)

	_, ok = vc.GetMspID("simple")
	assert.False(t, ok)
}
