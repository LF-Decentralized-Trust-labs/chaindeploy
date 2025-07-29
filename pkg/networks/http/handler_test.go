package http

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

// TestUpdateGenesisBlockRequestValidation tests the request validation
func TestUpdateGenesisBlockRequestValidation(t *testing.T) {
	// Create validator
	validate := validator.New()

	// Test valid request
	validReq := UpdateGenesisBlockRequest{
		GenesisBlock: base64.StdEncoding.EncodeToString([]byte("valid genesis block")),
		Reason:       "Updated configuration",
	}
	err := validate.Struct(validReq)
	assert.NoError(t, err)

	// Test invalid request - missing genesis block
	invalidReq1 := UpdateGenesisBlockRequest{
		GenesisBlock: "",
		Reason:       "Updated configuration",
	}
	err = validate.Struct(invalidReq1)
	assert.Error(t, err)

	// Test invalid request - missing reason
	invalidReq2 := UpdateGenesisBlockRequest{
		GenesisBlock: base64.StdEncoding.EncodeToString([]byte("valid genesis block")),
		Reason:       "",
	}
	err = validate.Struct(invalidReq2)
	assert.Error(t, err)
}

// TestUpdateGenesisBlockResponseStructure tests the response structure
func TestUpdateGenesisBlockResponseStructure(t *testing.T) {
	// Create response
	resp := UpdateGenesisBlockResponse{
		NetworkID: 123,
		Message:   "Genesis block updated successfully",
	}

	// Marshal to JSON
	respBytes, err := json.Marshal(resp)
	assert.NoError(t, err)

	// Unmarshal back
	var unmarshaledResp UpdateGenesisBlockResponse
	err = json.Unmarshal(respBytes, &unmarshaledResp)
	assert.NoError(t, err)

	// Assert fields
	assert.Equal(t, int64(123), unmarshaledResp.NetworkID)
	assert.Equal(t, "Genesis block updated successfully", unmarshaledResp.Message)
}

// TestNetworkResponseWithGenesisTracking tests the network response with genesis tracking fields
func TestNetworkResponseWithGenesisTracking(t *testing.T) {
	// Create network response with genesis tracking
	resp := NetworkResponse{
		ID:                  123,
		Name:                "Test Network",
		Platform:            "besu",
		Status:              "running",
		GenesisBlock:        "base64-encoded-genesis",
		GenesisChangedAt:    stringPtr("2024-01-15T10:30:00Z"),
		GenesisChangedBy:    int64Ptr(1),
		GenesisChangeReason: stringPtr("Updated configuration"),
	}

	// Marshal to JSON
	respBytes, err := json.Marshal(resp)
	assert.NoError(t, err)

	// Unmarshal back
	var unmarshaledResp NetworkResponse
	err = json.Unmarshal(respBytes, &unmarshaledResp)
	assert.NoError(t, err)

	// Assert fields
	assert.Equal(t, int64(123), unmarshaledResp.ID)
	assert.Equal(t, "besu", unmarshaledResp.Platform)
	assert.Equal(t, "base64-encoded-genesis", unmarshaledResp.GenesisBlock)
	assert.Equal(t, "2024-01-15T10:30:00Z", *unmarshaledResp.GenesisChangedAt)
	assert.Equal(t, int64(1), *unmarshaledResp.GenesisChangedBy)
	assert.Equal(t, "Updated configuration", *unmarshaledResp.GenesisChangeReason)
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}
