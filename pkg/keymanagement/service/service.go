package service

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/keymanagement/models"
	"github.com/chainlaunch/chainlaunch/pkg/keymanagement/providers"
	"github.com/chainlaunch/chainlaunch/pkg/keymanagement/providers/types"
)

type KeyManagementService struct {
	queries         *db.Queries
	providerFactory *providers.ProviderFactory
}

func NewKeyManagementService(queries *db.Queries) (*KeyManagementService, error) {
	factory, err := providers.NewProviderFactory(queries)
	if err != nil {
		return nil, err
	}

	return &KeyManagementService{
		queries:         queries,
		providerFactory: factory,
	}, nil
}

func (s *KeyManagementService) InitializeKeyProviders(ctx context.Context) error {
	// Check if default provider exists
	_, err := s.queries.GetKeyProviderByDefault(ctx)

	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		// Create default provider
		_, err = s.queries.CreateKeyProvider(ctx, &db.CreateKeyProviderParams{
			Name:      "Default Database Provider",
			Type:      "DATABASE",
			IsDefault: 1,
			Config:    "{}",
		})
		if err != nil {
			return err
		}
		return err
	}

	return nil
}

func (s *KeyManagementService) CreateKey(ctx context.Context, req models.CreateKeyRequest, userID int) (*models.KeyResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}
	// Get provider
	provider, err := s.providerFactory.GetProvider(providers.ProviderTypeDatabase)
	if err != nil {
		return nil, err
	}

	// Generate key pair
	generateKeyReq := types.GenerateKeyRequest{
		Name:        req.Name,
		Description: req.Description,
		Algorithm:   types.KeyAlgorithm(req.Algorithm),
		KeySize:     req.KeySize,
		Format:      "PEM",
		Status:      "active",
		ProviderID:  req.ProviderID,
		UserID:      userID,
		IsCA:        req.IsCA,
		Certificate: ToProviderCertRequest(req.Certificate),
	}
	if req.Algorithm == models.KeyAlgorithmEC {
		curve := types.ECCurve(*req.Curve)
		generateKeyReq.Curve = &curve
	}
	key, err := provider.GenerateKey(ctx, generateKeyReq)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	return key, nil
}

// ToProviderCertRequest converts a models.CertificateRequest to types.CertificateRequest
func ToProviderCertRequest(r *models.CertificateRequest) *types.CertificateRequest {
	if r == nil {
		return nil
	}

	return &types.CertificateRequest{
		CommonName:         r.CommonName,
		Organization:       r.Organization,
		OrganizationalUnit: r.OrganizationalUnit,
		Country:            r.Country,
		Province:           r.Province,
		Locality:           r.Locality,
		StreetAddress:      r.StreetAddress,
		PostalCode:         r.PostalCode,
		DNSNames:           r.DNSNames,
		EmailAddresses:     r.EmailAddresses,
		IPAddresses:        r.IPAddresses,
		URIs:               r.URIs,
		ValidFrom:          time.Now(), // Always use current time as ValidFrom
		ValidFor:           time.Duration(r.ValidFor),
		IsCA:               r.IsCA,
		KeyUsage:           r.KeyUsage,
		ExtKeyUsage:        r.ExtKeyUsage,
	}
}

func (s *KeyManagementService) GetKeys(ctx context.Context, page, pageSize int) (*models.PaginatedResponse, error) {
	// Get total count
	count, err := s.queries.GetKeysCount(ctx)
	if err != nil {
		return nil, err
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get keys with pagination
	keys, err := s.queries.ListKeys(ctx, &db.ListKeysParams{
		Limit:  int64(pageSize),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}

	// Map to response objects
	keyResponses := make([]models.KeyResponse, len(keys))
	for i, key := range keys {
		keySize := int(key.KeySize.Int64)
		curve := models.ECCurve(key.Curve.String)
		keyResponses[i] = models.KeyResponse{
			ID:                int(key.ID),
			Name:              key.Name,
			Description:       &key.Description.String,
			Algorithm:         models.KeyAlgorithm(key.Algorithm),
			KeySize:           &keySize,
			Curve:             &curve,
			Format:            key.Format,
			PublicKey:         key.PublicKey,
			Certificate:       &key.Certificate.String,
			Status:            key.Status,
			CreatedAt:         key.CreatedAt,
			ExpiresAt:         &key.ExpiresAt.Time,
			LastRotatedAt:     &key.LastRotatedAt.Time,
			SHA256Fingerprint: key.Sha256Fingerprint,
			SHA1Fingerprint:   key.Sha1Fingerprint,
			Provider: models.KeyProviderInfo{
				ID:   int(key.ProviderID),
				Name: key.ProviderName,
			},
			EthereumAddress: key.EthereumAddress.String,
		}
	}

	return &models.PaginatedResponse{
		Items:      keyResponses,
		TotalItems: int64(count),
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

func (s *KeyManagementService) GetKey(ctx context.Context, id int) (*models.KeyResponse, error) {
	key, err := s.queries.GetKey(ctx, int64(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("key not found")
		}
		return nil, err
	}
	keySize := int(key.KeySize.Int64)
	curve := models.ECCurve(key.Curve.String)
	signingKeyID := int(key.SigningKeyID.Int64)
	return &models.KeyResponse{
		ID:                int(key.ID),
		Name:              key.Name,
		Description:       &key.Description.String,
		Algorithm:         models.KeyAlgorithm(key.Algorithm),
		KeySize:           &keySize,
		Curve:             &curve,
		Format:            key.Format,
		PublicKey:         key.PublicKey,
		Certificate:       &key.Certificate.String,
		Status:            key.Status,
		CreatedAt:         key.CreatedAt,
		ExpiresAt:         &key.ExpiresAt.Time,
		LastRotatedAt:     &key.LastRotatedAt.Time,
		SHA256Fingerprint: key.Sha256Fingerprint,
		SHA1Fingerprint:   key.Sha1Fingerprint,
		Provider: models.KeyProviderInfo{
			ID:   int(key.ProviderID),
			Name: key.ProviderName,
		},
		EthereumAddress: key.EthereumAddress.String,
		SigningKeyID:    &signingKeyID,
	}, err
}

func (s *KeyManagementService) DeleteKey(ctx context.Context, id int) error {
	err := s.queries.DeleteKey(ctx, int64(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("key not found")
		}
		return err
	}
	return nil
}

func (s *KeyManagementService) CreateProvider(ctx context.Context, req models.CreateProviderRequest) (*models.ProviderResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// If this provider is set as default, unset existing default
	if req.IsDefault == 1 {
		err := s.queries.UnsetDefaultProvider(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Convert config to JSON string
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, err
	}

	provider, err := s.queries.CreateKeyProvider(ctx, &db.CreateKeyProviderParams{
		Name:      req.Name,
		Type:      string(req.Type),
		IsDefault: int64(req.IsDefault),
		Config:    string(configJSON),
	})
	if err != nil {
		return nil, err
	}

	return mapProviderToResponse(provider), nil
}

func (s *KeyManagementService) ListProviders(ctx context.Context) ([]models.ProviderResponse, error) {
	providers, err := s.queries.ListKeyProviders(ctx)
	if err != nil {
		return nil, err
	}

	// Map DB models to service models
	providerResponses := make([]models.ProviderResponse, len(providers))
	for i, provider := range providers {
		providerResponses[i] = models.ProviderResponse{
			ID:        int(provider.ID),
			Name:      provider.Name,
			Type:      models.KeyProviderType(provider.Type),
			IsDefault: int(provider.IsDefault),
			Config:    json.RawMessage(provider.Config),
			CreatedAt: provider.CreatedAt,
		}
	}

	return providerResponses, nil
}

func (s *KeyManagementService) GetProviderByID(ctx context.Context, id int) (*models.ProviderResponse, error) {
	provider, err := s.queries.GetKeyProvider(ctx, int64(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("provider not found")
		}
		return nil, err
	}

	return mapProviderToResponse(provider), nil
}

func (s *KeyManagementService) DeleteProvider(ctx context.Context, id int) error {
	// Check if provider has any keys
	count, err := s.queries.GetKeyCountByProvider(ctx, int64(id))
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("cannot delete provider with existing keys")
	}

	err = s.queries.DeleteKeyProvider(ctx, int64(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("provider not found")
		}
		return err
	}
	return nil
}

func mapProviderToResponse(provider *db.KeyProvider) *models.ProviderResponse {
	return &models.ProviderResponse{
		ID:        int(provider.ID),
		Name:      provider.Name,
		Type:      models.KeyProviderType(provider.Type),
		IsDefault: int(provider.IsDefault),
		Config:    json.RawMessage(provider.Config),
		CreatedAt: provider.CreatedAt,
	}
}

// KeyPair represents a public/private key pair
type KeyPair struct {
	PublicKey         string
	PrivateKey        string
	SHA256Fingerprint string
	SHA1Fingerprint   string
}

func (s *KeyManagementService) generateKeyPair(req models.CreateKeyRequest) (*KeyPair, error) {
	var keyPair *KeyPair
	var err error

	switch req.Algorithm {
	case models.KeyAlgorithmRSA:
		keyPair, err = s.generateRSAKeyPair(req)
	case models.KeyAlgorithmEC:
		keyPair, err = s.generateECKeyPair(req)
	case models.KeyAlgorithmED25519:
		keyPair, err = s.generateED25519KeyPair()
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", req.Algorithm)
	}

	if err != nil {
		return nil, err
	}
	// Calculate fingerprints from public key
	var publicKeyBytes []byte
	if req.Curve != nil && *req.Curve == models.ECCurveSECP256K1 {
		// For secp256k1, public key is already hex encoded
		var err error
		publicKeyBytes, err = hex.DecodeString(keyPair.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode hex public key: %w", err)
		}
	} else {
		// For other curves, public key is PEM encoded
		block, _ := pem.Decode([]byte(keyPair.PublicKey))
		if block == nil {
			return nil, fmt.Errorf("failed to decode public key PEM")
		}
		publicKeyBytes = block.Bytes
	}

	sha256Sum := sha256.Sum256(publicKeyBytes)
	sha1Sum := sha1.Sum(publicKeyBytes)

	keyPair.SHA256Fingerprint = hex.EncodeToString(sha256Sum[:])
	keyPair.SHA1Fingerprint = hex.EncodeToString(sha1Sum[:])

	return keyPair, nil
}

func (s *KeyManagementService) generateRSAKeyPair(req models.CreateKeyRequest) (*KeyPair, error) {
	keySize := 2048
	if req.KeySize != nil {
		keySize = *req.KeySize
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Encode private key
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Encode public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return &KeyPair{
		PublicKey:  string(publicPEM),
		PrivateKey: string(privatePEM),
	}, nil
}

func (s *KeyManagementService) generateECKeyPair(req models.CreateKeyRequest) (*KeyPair, error) {
	if req.Curve == nil {
		return nil, fmt.Errorf("curve must be specified for EC keys")
	}

	var curve elliptic.Curve
	switch *req.Curve {
	case "P-224":
		curve = elliptic.P224()
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported curve: %s", *req.Curve)
	}

	privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate EC key: %w", err)
	}

	// Encode private key
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Encode public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return &KeyPair{
		PublicKey:  string(publicPEM),
		PrivateKey: string(privatePEM),
	}, nil
}

func (s *KeyManagementService) generateED25519KeyPair() (*KeyPair, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ED25519 key: %w", err)
	}

	// Encode private key
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Encode public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return &KeyPair{
		PublicKey:  string(publicPEM),
		PrivateKey: string(privatePEM),
	}, nil
}

func (s *KeyManagementService) SignCertificate(ctx context.Context, keyID int, caKeyID int, certReq models.CertificateRequest) (*models.KeyResponse, error) {
	// Validate that the CA key exists and is a CA
	caKey, err := s.queries.GetKey(ctx, int64(caKeyID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("CA key not found")
		}
		return nil, err
	}

	// Check if the key is marked as CA
	if caKey.IsCa != 1 {
		return nil, fmt.Errorf("key %d is not a CA, value: %d", caKeyID, caKey.IsCa)
	}

	// Get provider
	provider, err := s.providerFactory.GetProvider(providers.ProviderTypeDatabase)
	if err != nil {
		return nil, err
	}

	// Sign the certificate
	return provider.SignCertificate(ctx, types.SignCertificateRequest{
		KeyID:              keyID,
		CAKeyID:            caKeyID,
		CertificateRequest: *ToProviderCertRequest(&certReq),
	})
}

// GetDecryptedPrivateKey retrieves and decrypts the private key for a given key ID
func (s *KeyManagementService) GetDecryptedPrivateKey(id int) (string, error) {
	// Get provider
	provider, err := s.providerFactory.GetProvider(providers.ProviderTypeDatabase)
	if err != nil {
		return "", fmt.Errorf("failed to get provider: %w", err)
	}

	// Use provider to decrypt the private key
	pk, err := provider.GetDecryptedPrivateKey(id)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt private key: %w", err)
	}

	return pk, nil
}

// FilterKeys returns keys filtered by algorithm and/or curve
func (s *KeyManagementService) FilterKeys(ctx context.Context, algorithm, curve string, page, pageSize int) (*models.PaginatedResponse, error) {
	var keys []*db.GetKeysByFilterRow
	var err error

	if curve != "" {
		// If curve is specified, use GetKeysByProviderAndCurve
		// TODO: Get provider ID from context or configuration
		providerID := int64(1)
		keys, err = s.queries.GetKeysByFilter(ctx, &db.GetKeysByFilterParams{
			AlgorithmFilter:  algorithm,
			Algorithm:        algorithm,
			ProviderIDFilter: 0,
			ProviderID:       providerID,
			CurveFilter:      curve,
			Curve:            sql.NullString{String: curve, Valid: true},
		})
	} else if algorithm != "" {
		// If only algorithm is specified, use GetKeysByAlgorithm
		keys, err = s.queries.GetKeysByFilter(ctx, &db.GetKeysByFilterParams{
			AlgorithmFilter:  algorithm,
			Algorithm:        algorithm,
			ProviderIDFilter: 0,
			ProviderID:       0,
			CurveFilter:      "",
			Curve:            sql.NullString{String: "", Valid: false},
		})
	} else {
		// If no filters, get all keys
		return nil, fmt.Errorf("no filters provided")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query keys: %w", err)
	}

	// Calculate pagination
	total := len(keys)
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > total {
		end = total
	}
	if start > total {
		start = total
	}

	// Convert db.Key to models.KeyResponse
	keyResponses := make([]models.KeyResponse, 0)
	for _, key := range keys[start:end] {
		keySize := int(key.KeySize.Int64)
		curve := models.ECCurve(key.Curve.String)
		keyResponses = append(keyResponses, models.KeyResponse{
			ID:                int(key.ID),
			Name:              key.Name,
			Description:       &key.Description.String,
			Algorithm:         models.KeyAlgorithm(key.Algorithm),
			KeySize:           &keySize,
			Curve:             &curve,
			Format:            key.Format,
			PublicKey:         key.PublicKey,
			Certificate:       &key.Certificate.String,
			Status:            key.Status,
			CreatedAt:         key.CreatedAt,
			ExpiresAt:         &key.ExpiresAt.Time,
			LastRotatedAt:     &key.LastRotatedAt.Time,
			SHA256Fingerprint: key.Sha256Fingerprint,
			SHA1Fingerprint:   key.Sha1Fingerprint,
			Provider: models.KeyProviderInfo{
				ID:   int(key.ProviderID),
				Name: key.ProviderName,
			},
			EthereumAddress: key.EthereumAddress.String,
		})
	}

	return &models.PaginatedResponse{
		Items:      keyResponses,
		Page:       page,
		PageSize:   pageSize,
		TotalItems: int64(total),
	}, nil
}

type KeyInfo struct {
	DID       string
	KeyType   string
	PublicKey string
}

// SetSigningKeyIDForKey updates a key with the ID of the key that signed its certificate
func (s *KeyManagementService) SetSigningKeyIDForKey(ctx context.Context, keyID, signingKeyID int) error {
	// Validate that both keys exist
	key, err := s.queries.GetKey(ctx, int64(keyID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("key not found")
		}
		return fmt.Errorf("failed to get key: %w", err)
	}

	signingKey, err := s.queries.GetKey(ctx, int64(signingKeyID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("signing key not found")
		}
		return fmt.Errorf("failed to get signing key: %w", err)
	}

	// Verify that the signing key is a CA
	if signingKey.IsCa != 1 {
		return fmt.Errorf("signing key %d is not a CA", signingKeyID)
	}

	// Verify that the key has a certificate
	if !key.Certificate.Valid {
		return fmt.Errorf("key %d does not have a certificate", keyID)
	}

	// Update the key with the signing key ID
	params := &db.UpdateKeyParams{
		ID:                key.ID,
		Name:              key.Name,
		Description:       key.Description,
		Algorithm:         key.Algorithm,
		KeySize:           key.KeySize,
		Curve:             key.Curve,
		Format:            key.Format,
		PublicKey:         key.PublicKey,
		PrivateKey:        key.PrivateKey,
		Certificate:       key.Certificate,
		Status:            key.Status,
		ExpiresAt:         key.ExpiresAt,
		Sha256Fingerprint: key.Sha256Fingerprint,
		Sha1Fingerprint:   key.Sha1Fingerprint,
		ProviderID:        key.ProviderID,
		UserID:            key.UserID,
		EthereumAddress:   key.EthereumAddress,
		SigningKeyID:      sql.NullInt64{Int64: int64(signingKeyID), Valid: true},
	}

	_, err = s.queries.UpdateKey(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to update key with signing key ID: %w", err)
	}

	return nil
}

// RenewCertificate renews a certificate using the same keypair and CA that was used to generate it
func (s *KeyManagementService) RenewCertificate(ctx context.Context, keyID int, certReq models.CertificateRequest) (*models.KeyResponse, error) {
	// Get the key details
	key, err := s.queries.GetKey(ctx, int64(keyID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("key not found")
		}
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	// Check if the key has a certificate
	if !key.Certificate.Valid {
		return nil, fmt.Errorf("key does not have a certificate to renew")
	}

	// Get the CA key ID that was used to sign this certificate
	if !key.SigningKeyID.Valid {
		return nil, fmt.Errorf("key does not have an associated CA key")
	}
	caKeyID := int(key.SigningKeyID.Int64)

	// Validate that the CA key exists and is a CA
	caKey, err := s.queries.GetKey(ctx, int64(caKeyID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("CA key not found")
		}
		return nil, fmt.Errorf("failed to get CA key: %w", err)
	}

	// Check if the CA key is marked as CA
	if caKey.IsCa != 1 {
		return nil, fmt.Errorf("key %d is not a CA", caKeyID)
	}

	// Get provider
	provider, err := s.providerFactory.GetProvider(providers.ProviderTypeDatabase)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	// If no certificate request is provided, use the existing certificate's details
	if certReq.CommonName == "" {
		existingCert, err := parseCertificate(key.Certificate.String)
		if err != nil {
			return nil, fmt.Errorf("failed to parse existing certificate: %w", err)
		}

		certReq = models.CertificateRequest{
			CommonName:         existingCert.Subject.CommonName,
			Organization:       existingCert.Subject.Organization,
			OrganizationalUnit: existingCert.Subject.OrganizationalUnit,
			Country:            existingCert.Subject.Country,
			Province:           existingCert.Subject.Province,
			Locality:           existingCert.Subject.Locality,
			StreetAddress:      existingCert.Subject.StreetAddress,
			PostalCode:         existingCert.Subject.PostalCode,
			DNSNames:           existingCert.DNSNames,
			EmailAddresses:     existingCert.EmailAddresses,
			IPAddresses:        existingCert.IPAddresses,
			URIs:               existingCert.URIs,
			ValidFor:           models.Duration(365 * 24 * time.Hour),
			IsCA:               existingCert.IsCA,
			KeyUsage:           x509.KeyUsage(existingCert.KeyUsage),
			ExtKeyUsage:        existingCert.ExtKeyUsage,
		}
	}

	// Sign the certificate with the same CA
	return provider.SignCertificate(ctx, types.SignCertificateRequest{
		KeyID:              keyID,
		CAKeyID:            caKeyID,
		CertificateRequest: *ToProviderCertRequest(&certReq),
	})
}

// Helper function to parse PEM certificate
func parseCertificate(certPEM string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}

// Exported wrapper for parseCertificate
func ParseCertificate(certPEM string) (*x509.Certificate, error) {
	return parseCertificate(certPEM)
}

// SignData signs data using a key with the specified parameters
func (s *KeyManagementService) SignData(ctx context.Context, keyID int, req models.SignRequest) (*models.SignResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid sign request: %w", err)
	}

	// Get the key
	key, err := s.queries.GetKey(ctx, int64(keyID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("key not found")
		}
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	// Check if key is active
	if key.Status != "active" {
		return nil, fmt.Errorf("key is not active (status: %s)", key.Status)
	}
	provider, err := s.providerFactory.GetProvider(providers.ProviderTypeDatabase)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	// Get the private key

	// Sign the data
	signature, err := provider.SignData(ctx, keyID, req)
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}

	return &models.SignResponse{
		Signature:  signature.Signature,
		KeyVersion: 1, // For now, we only support version 1
		Reference:  req.Reference,
	}, nil
}
