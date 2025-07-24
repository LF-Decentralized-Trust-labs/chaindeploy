package database

import (
	"testing"

	"github.com/chainlaunch/chainlaunch/pkg/keymanagement/models"
	"github.com/stretchr/testify/assert"
)

func TestSignRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		request models.SignRequest
		wantErr bool
	}{
		{
			name: "valid request",
			request: models.SignRequest{
				Input: "SGVsbG8gV29ybGQ=",
			},
			wantErr: false,
		},
		{
			name: "missing input",
			request: models.SignRequest{
				Input: "",
			},
			wantErr: true,
		},
		{
			name: "invalid hash algorithm",
			request: models.SignRequest{
				Input:         "SGVsbG8gV29ybGQ=",
				HashAlgorithm: stringPtr("invalid-hash"),
			},
			wantErr: true,
		},
		{
			name: "valid hash algorithm",
			request: models.SignRequest{
				Input:         "SGVsbG8gV29ybGQ=",
				HashAlgorithm: stringPtr(models.HashAlgorithmSHA2_256),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBatchSignRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		request models.BatchSignRequest
		wantErr bool
	}{
		{
			name: "valid batch request",
			request: models.BatchSignRequest{
				BatchInput: []models.SignRequest{
					{Input: "SGVsbG8gV29ybGQ="},
					{Input: "R29vZGJ5ZSBXb3JsZA=="},
				},
			},
			wantErr: false,
		},
		{
			name: "empty batch input",
			request: models.BatchSignRequest{
				BatchInput: []models.SignRequest{},
			},
			wantErr: true,
		},
		{
			name: "too many batch items",
			request: models.BatchSignRequest{
				BatchInput: make([]models.SignRequest, 101),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHashData(t *testing.T) {
	provider := &DatabaseProvider{}
	testData := []byte("Hello World")

	tests := []struct {
		name      string
		algorithm string
		wantErr   bool
	}{
		{"sha1", models.HashAlgorithmSHA1, false},
		{"sha2-224", models.HashAlgorithmSHA2_224, false},
		{"sha2-256", models.HashAlgorithmSHA2_256, false},
		{"sha2-384", models.HashAlgorithmSHA2_384, false},
		{"sha2-512", models.HashAlgorithmSHA2_512, false},
		{"none", models.HashAlgorithmNone, false},
		{"invalid", "invalid-algorithm", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.hashData(testData, tt.algorithm)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.algorithm != models.HashAlgorithmNone {
					assert.NotEqual(t, testData, result)
				} else {
					assert.Equal(t, testData, result)
				}
			}
		})
	}
}

func TestParsePrivateKey(t *testing.T) {
	provider := &DatabaseProvider{}

	t.Run("unsupported algorithm", func(t *testing.T) {
		_, err := provider.parsePrivateKey("dummy-key", "UNSUPPORTED")
		assert.Error(t, err)
		// The error could be either "failed to decode PEM block" or "unsupported algorithm"
		assert.True(t, err.Error() == "failed to decode PEM block" || err.Error() == "unsupported algorithm")
	})
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// TestSignDataIntegration would require a mock database and actual keys
// This is a placeholder for integration tests
func TestSignDataIntegration(t *testing.T) {
	t.Skip("Integration test - requires database and keys")

	// This test would:
	// 1. Create a test key in the database
	// 2. Call SignData with various parameters
	// 3. Verify the signatures are valid
	// 4. Test error conditions (invalid key, wrong algorithm, etc.)
}

// TestBatchSignDataIntegration would test batch signing functionality
func TestBatchSignDataIntegration(t *testing.T) {
	t.Skip("Integration test - requires database and keys")

	// This test would:
	// 1. Create a test key in the database
	// 2. Call BatchSignData with multiple items
	// 3. Verify all signatures are valid
	// 4. Test partial failures in batch
}
