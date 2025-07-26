package providers

import (
	"context"

	"github.com/chainlaunch/chainlaunch/pkg/keymanagement/models"
	"github.com/chainlaunch/chainlaunch/pkg/keymanagement/providers/types"
)

// Provider defines the interface that all key providers must implement
type Provider interface {
	// GenerateKey generates a new key pair
	GenerateKey(ctx context.Context, req types.GenerateKeyRequest) (*models.KeyResponse, error)
	// StoreKey stores a key pair
	StoreKey(ctx context.Context, req types.StoreKeyRequest) (*models.KeyResponse, error)
	// RetrieveKey retrieves a key pair by ID
	RetrieveKey(ctx context.Context, id int) (*models.KeyResponse, error)
	// DeleteKey deletes a key by ID
	DeleteKey(ctx context.Context, id int) error
	// SignCertificate signs a certificate for a key using a CA key
	SignCertificate(ctx context.Context, req types.SignCertificateRequest) (*models.KeyResponse, error)
	// GetDecryptedPrivateKey retrieves and decrypts the private key for a given key ID
	GetDecryptedPrivateKey(id int) (string, error)
	// SignData signs data using a key with the specified parameters
	SignData(ctx context.Context, keyID int, req models.SignRequest) (*models.SignResponse, error)
	// BatchSignData signs multiple data items in a batch using a key
	BatchSignData(ctx context.Context, keyID int, req models.BatchSignRequest) (*models.BatchSignResponse, error)
}
