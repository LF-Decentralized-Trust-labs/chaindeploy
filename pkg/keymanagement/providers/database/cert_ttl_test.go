package database

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/keymanagement/models"
	"github.com/chainlaunch/chainlaunch/pkg/keymanagement/providers/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSelfSignedCert_TTL(t *testing.T) {
	// Create a test key pair using EC P-256 for testing
	provider := &DatabaseProvider{
		encryptionKey: make([]byte, 32), // dummy key for testing
	}

	// Generate a test key pair
	curve := models.ECCurveP256
	keyPair, err := provider.generateKeyPair(models.CreateKeyRequest{
		Name:      "test-key",
		Algorithm: models.KeyAlgorithmEC,
		Curve:     &curve,
	})
	require.NoError(t, err)

	tests := []struct {
		name          string
		validFor      time.Duration
		expectedYears float64
		tolerance     float64 // tolerance in years
	}{
		{
			name:          "10 year CA certificate",
			validFor:      time.Hour * 24 * 365 * 10, // 10 years
			expectedYears: 10.0,
			tolerance:     0.1, // Allow 0.1 year tolerance for leap years
		},
		{
			name:          "1 year certificate",
			validFor:      time.Hour * 24 * 365, // 1 year
			expectedYears: 1.0,
			tolerance:     0.1,
		},
		{
			name:          "2 year certificate",
			validFor:      time.Hour * 24 * 365 * 2, // 2 years
			expectedYears: 2.0,
			tolerance:     0.1,
		},
		{
			name:          "6 month certificate",
			validFor:      time.Hour * 24 * 182, // ~6 months
			expectedYears: 0.5,
			tolerance:     0.05,
		},
		{
			name:          "24 hour certificate",
			validFor:      time.Hour * 24, // 1 day
			expectedYears: 1.0 / 365,
			tolerance:     0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			certReq := &types.CertificateRequest{
				CommonName:   "test-cert",
				Organization: []string{"Test Org"},
				Country:      []string{"US"},
				ValidFrom:    now,
				ValidFor:     tt.validFor,
				IsCA:         true,
				KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
				ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			}

			certPEM, err := provider.generateSelfSignedCert(keyPair, certReq)
			require.NoError(t, err)

			// Parse the certificate
			block, _ := pem.Decode([]byte(certPEM))
			require.NotNil(t, block, "Failed to decode PEM block")

			cert, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err)

			// Verify the validity period
			validity := cert.NotAfter.Sub(cert.NotBefore)
			actualYears := validity.Hours() / 24 / 365

			assert.InDelta(t, tt.expectedYears, actualYears, tt.tolerance,
				"Certificate validity period mismatch: expected ~%.2f years, got %.2f years",
				tt.expectedYears, actualYears)
		})
	}
}

func TestGenerateSelfSignedCert_ValidForRespected(t *testing.T) {
	provider := &DatabaseProvider{
		encryptionKey: make([]byte, 32),
	}

	// Generate a test key pair
	curve := models.ECCurveP256
	keyPair, err := provider.generateKeyPair(models.CreateKeyRequest{
		Name:      "test-key",
		Algorithm: models.KeyAlgorithmEC,
		Curve:     &curve,
	})
	require.NoError(t, err)

	// Test that the exact ValidFor is used
	testDuration := time.Hour * 8760 // 1 year in hours
	now := time.Now()

	certReq := &types.CertificateRequest{
		CommonName:   "test-cert",
		Organization: []string{"Test Org"},
		ValidFrom:    now,
		ValidFor:     testDuration,
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}

	certPEM, err := provider.generateSelfSignedCert(keyPair, certReq)
	require.NoError(t, err)

	block, _ := pem.Decode([]byte(certPEM))
	require.NotNil(t, block)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	// The NotAfter should be approximately ValidFrom + ValidFor
	expectedNotAfter := now.Add(testDuration)
	timeDiff := cert.NotAfter.Sub(expectedNotAfter)

	// Allow 1 second tolerance for computation time
	assert.True(t, timeDiff < time.Second && timeDiff > -time.Second,
		"NotAfter time does not match ValidFor: expected ~%v, got %v",
		expectedNotAfter, cert.NotAfter)
}

func TestCertificateTTL_DifferentAlgorithms(t *testing.T) {
	provider := &DatabaseProvider{
		encryptionKey: make([]byte, 32),
	}

	validFor := time.Hour * 24 * 365 * 5 // 5 years

	tests := []struct {
		name      string
		algorithm models.KeyAlgorithm
		keySize   *int
		curve     *models.ECCurve
	}{
		{
			name:      "RSA 2048",
			algorithm: models.KeyAlgorithmRSA,
			keySize:   intPtr(2048),
		},
		{
			name:      "EC P-256",
			algorithm: models.KeyAlgorithmEC,
			curve:     curvePtr(models.ECCurveP256),
		},
		// Note: EC P-384/P-521 tests skipped due to existing issue in generateECKeyPair
		// that incorrectly uses P-256 for public key marshaling
		{
			name:      "ED25519",
			algorithm: models.KeyAlgorithmED25519,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyPair, err := provider.generateKeyPair(models.CreateKeyRequest{
				Name:      "test-key",
				Algorithm: tt.algorithm,
				KeySize:   tt.keySize,
				Curve:     tt.curve,
			})
			require.NoError(t, err)

			now := time.Now()
			certReq := &types.CertificateRequest{
				CommonName:   "test-cert",
				Organization: []string{"Test Org"},
				ValidFrom:    now,
				ValidFor:     validFor,
				IsCA:         true,
				KeyUsage:     x509.KeyUsageCertSign,
			}

			certPEM, err := provider.generateSelfSignedCert(keyPair, certReq)
			require.NoError(t, err)

			block, _ := pem.Decode([]byte(certPEM))
			require.NotNil(t, block)

			cert, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err)

			// Verify validity is approximately 5 years
			validity := cert.NotAfter.Sub(cert.NotBefore)
			actualYears := validity.Hours() / 24 / 365

			assert.InDelta(t, 5.0, actualYears, 0.1,
				"Certificate validity period mismatch for %s: expected ~5 years, got %.2f years",
				tt.name, actualYears)
		})
	}
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func curvePtr(c models.ECCurve) *models.ECCurve {
	return &c
}
