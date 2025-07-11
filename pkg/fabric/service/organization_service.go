package service

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/config"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/keymanagement/models"
	keymanagement "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	gwidentity "github.com/hyperledger/fabric-gateway/pkg/identity"
)

// OrganizationDTO represents the service layer data structure
type OrganizationDTO struct {
	ID              int64          `json:"id"`
	MspID           string         `json:"mspId"`
	Description     sql.NullString `json:"description"`
	SignKeyID       sql.NullInt64  `json:"signKeyId"`
	TlsRootKeyID    sql.NullInt64  `json:"tlsRootKeyId"`
	SignPublicKey   string         `json:"signPublicKey"`
	SignCertificate string         `json:"signCertificate"`
	TlsPublicKey    string         `json:"tlsPublicKey"`
	TlsCertificate  string         `json:"tlsCertificate"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	AdminTlsKeyID   sql.NullInt64  `json:"adminTlsKeyId"`
	AdminSignKeyID  sql.NullInt64  `json:"adminSignKeyId"`
	ClientSignKeyID sql.NullInt64  `json:"clientSignKeyId"`
	ProviderID      int64          `json:"providerId"`
	ProviderName    string         `json:"providerName"`
}

// CreateOrganizationParams represents the service layer input parameters
type CreateOrganizationParams struct {
	MspID       string `validate:"required"`
	Name        string `validate:"required"`
	Description string
	ProviderID  int64

	// CA certificate properties
	CommonName    string
	Country       []string
	Province      []string
	Locality      []string
	StreetAddress []string
	PostalCode    []string
}

// UpdateOrganizationParams represents the service layer update parameters
type UpdateOrganizationParams struct {
	Description *string
}

// RevokedCertificateDTO represents a revoked certificate
type RevokedCertificateDTO struct {
	SerialNumber   string    `json:"serialNumber"`
	RevocationTime time.Time `json:"revocationTime"`
	Reason         int64     `json:"reason"`
}

// PaginationParams represents pagination input for listing organizations
// You may want to move this to a shared location if used elsewhere
type PaginationParams struct {
	Limit  int64
	Offset int64
}

type OrganizationService struct {
	queries       *db.Queries
	keyManagement *keymanagement.KeyManagementService
	configService *config.ConfigService
}

func NewOrganizationService(queries *db.Queries, keyManagement *keymanagement.KeyManagementService, configService *config.ConfigService) *OrganizationService {
	return &OrganizationService{
		queries:       queries,
		keyManagement: keyManagement,
		configService: configService,
	}
}

func mapDBOrganizationToServiceOrganization(org *db.GetFabricOrganizationByMspIDRow) *OrganizationDTO {
	providerName := ""
	if org.ProviderName.Valid {
		providerName = org.ProviderName.String
	}

	return &OrganizationDTO{
		ID:              org.ID,
		MspID:           org.MspID,
		Description:     org.Description,
		SignKeyID:       org.SignKeyID,
		TlsRootKeyID:    org.TlsRootKeyID,
		SignPublicKey:   org.SignPublicKey.String,
		SignCertificate: org.SignCertificate.String,
		TlsPublicKey:    org.TlsPublicKey.String,
		TlsCertificate:  org.TlsCertificate.String,
		CreatedAt:       org.CreatedAt,
		UpdatedAt:       org.UpdatedAt.Time,
		AdminTlsKeyID:   org.AdminTlsKeyID,
		AdminSignKeyID:  org.AdminSignKeyID,
		ClientSignKeyID: org.ClientSignKeyID,
		ProviderID:      org.ProviderID.Int64,
		ProviderName:    providerName,
	}
}

// Convert database model to DTO for single organization
func toOrganizationDTO(org *db.GetFabricOrganizationWithKeysRow) *OrganizationDTO {
	providerName := ""
	if org.ProviderName.Valid {
		providerName = org.ProviderName.String
	}

	return &OrganizationDTO{
		ID:              org.ID,
		MspID:           org.MspID,
		Description:     org.Description,
		SignKeyID:       org.SignKeyID,
		TlsRootKeyID:    org.TlsRootKeyID,
		SignPublicKey:   org.SignPublicKey.String,
		SignCertificate: org.SignCertificate.String,
		TlsPublicKey:    org.TlsPublicKey.String,
		TlsCertificate:  org.TlsCertificate.String,
		CreatedAt:       org.CreatedAt,
		UpdatedAt:       org.UpdatedAt.Time,
		AdminTlsKeyID:   org.AdminTlsKeyID,
		AdminSignKeyID:  org.AdminSignKeyID,
		ClientSignKeyID: org.ClientSignKeyID,
		ProviderID:      org.ProviderID.Int64,
		ProviderName:    providerName,
	}
}

// Convert database model to DTO for list of organizations
func toOrganizationListDTO(org *db.ListFabricOrganizationsWithKeysRow) *OrganizationDTO {
	providerName := ""
	if org.ProviderName.Valid {
		providerName = org.ProviderName.String
	}

	return &OrganizationDTO{
		ID:              org.ID,
		MspID:           org.MspID,
		Description:     org.Description,
		SignKeyID:       org.SignKeyID,
		TlsRootKeyID:    org.TlsRootKeyID,
		SignPublicKey:   org.SignPublicKey.String,
		SignCertificate: org.SignCertificate.String,
		TlsPublicKey:    org.TlsPublicKey.String,
		TlsCertificate:  org.TlsCertificate.String,
		CreatedAt:       org.CreatedAt,
		UpdatedAt:       org.UpdatedAt.Time,
		ProviderID:      org.ProviderID.Int64,
		ProviderName:    providerName,
		AdminTlsKeyID:   org.AdminTlsKeyID,
		AdminSignKeyID:  org.AdminSignKeyID,
		ClientSignKeyID: org.ClientSignKeyID,
	}
}

func (s *OrganizationService) CreateOrganization(ctx context.Context, params CreateOrganizationParams) (*OrganizationDTO, error) {
	// Uniqueness check by MSP ID
	if existing, _ := s.queries.GetFabricOrganizationByMSPID(ctx, params.MspID); existing != nil && existing.ID != 0 {
		return nil, fmt.Errorf("organization with MSP ID '%s' already exists", params.MspID)
	}

	description := fmt.Sprintf("Sign key for organization %s", params.MspID)
	curve := models.ECCurveP256
	// Create SIGN key
	providerID := int(params.ProviderID)

	_, err := s.keyManagement.GetProviderByID(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get key management provider: %w", err)
	}

	isCA := 1
	isNotCA := 0
	signKeyReq := models.CreateKeyRequest{
		Name:        fmt.Sprintf("%s-sign-ca", params.MspID),
		Description: &description,
		Algorithm:   models.KeyAlgorithmEC,
		KeySize:     nil,
		Curve:       &curve,
		ProviderID:  &providerID,
		IsCA:        &isCA,
		Certificate: &models.CertificateRequest{
			CommonName:         params.CommonName,
			Organization:       []string{params.Name},
			OrganizationalUnit: []string{"SIGN"},
			Country:            params.Country,
			Locality:           params.Locality,
			Province:           params.Province,
			StreetAddress:      params.StreetAddress,
			PostalCode:         params.PostalCode,
		},
	}
	signKey, err := s.keyManagement.CreateKey(ctx, signKeyReq, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to create SIGN key: %w", err)
	}

	// Create SIGN admin key
	signAdminKeyReq := models.CreateKeyRequest{
		Name:        fmt.Sprintf("%s-sign-admin", params.MspID),
		Description: &description,
		Algorithm:   models.KeyAlgorithmEC,
		KeySize:     nil,
		Curve:       &curve,
		ProviderID:  &providerID,
		IsCA:        &isNotCA,
	}
	signAdminKey, err := s.keyManagement.CreateKey(ctx, signAdminKeyReq, providerID)
	if err != nil {
		_ = s.keyManagement.DeleteKey(ctx, signKey.ID)
		return nil, fmt.Errorf("failed to create SIGN admin key: %w", err)
	}

	// Sign the admin key with the CA
	_, err = s.keyManagement.SignCertificate(ctx, signAdminKey.ID, signKey.ID, models.CertificateRequest{
		CommonName:         fmt.Sprintf("%s-sign-admin", params.MspID),
		Organization:       []string{params.Name},
		OrganizationalUnit: []string{"admin"},
		Country:            params.Country,
		Locality:           params.Locality,
		Province:           params.Province,
		StreetAddress:      params.StreetAddress,
		PostalCode:         params.PostalCode,
		KeyUsage:           x509.KeyUsageCertSign,
	})
	if err != nil {
		_ = s.keyManagement.DeleteKey(ctx, signKey.ID)
		return nil, fmt.Errorf("failed to sign admin certificate: %w", err)
	}

	// Create SIGN client key
	signClientKeyReq := models.CreateKeyRequest{
		Name:        fmt.Sprintf("%s-sign-client", params.MspID),
		Description: &description,
		Algorithm:   models.KeyAlgorithmEC,
		KeySize:     nil,
		Curve:       &curve,
		ProviderID:  &providerID,
		IsCA:        &isNotCA,
	}
	signClientKey, err := s.keyManagement.CreateKey(ctx, signClientKeyReq, providerID)
	if err != nil {
		_ = s.keyManagement.DeleteKey(ctx, signKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, signAdminKey.ID)
		return nil, fmt.Errorf("failed to create SIGN client key: %w", err)
	}

	// Sign the client key with the CA
	signClientKey, err = s.keyManagement.SignCertificate(ctx, signClientKey.ID, signKey.ID, models.CertificateRequest{
		CommonName:         fmt.Sprintf("%s-sign-client", params.MspID),
		Organization:       []string{params.Name},
		OrganizationalUnit: []string{"client"},
		Country:            params.Country,
		Locality:           params.Locality,
		Province:           params.Province,
		StreetAddress:      params.StreetAddress,
		PostalCode:         params.PostalCode,
		KeyUsage:           x509.KeyUsageCertSign,
	})
	if err != nil {
		_ = s.keyManagement.DeleteKey(ctx, signKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, signAdminKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, signClientKey.ID)
		return nil, fmt.Errorf("failed to sign client certificate: %w", err)
	}

	// Create TLS key
	isCA = 1
	tlsKeyReq := models.CreateKeyRequest{
		Name:        fmt.Sprintf("%s-tls-ca", params.MspID),
		Description: &description,
		Algorithm:   models.KeyAlgorithmEC,
		KeySize:     nil,
		Curve:       &curve,
		ProviderID:  &providerID,
		IsCA:        &isCA,
		Certificate: &models.CertificateRequest{
			CommonName:         fmt.Sprintf("%s-tls-ca", params.MspID),
			Organization:       []string{params.Name},
			OrganizationalUnit: []string{"TLS"},
			Country:            params.Country,
			Locality:           params.Locality,
			Province:           params.Province,
			StreetAddress:      params.StreetAddress,
			PostalCode:         params.PostalCode,
		},
	}
	tlsKey, err := s.keyManagement.CreateKey(ctx, tlsKeyReq, providerID)
	if err != nil {
		_ = s.keyManagement.DeleteKey(ctx, signKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, signAdminKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, signClientKey.ID)
		return nil, fmt.Errorf("failed to create TLS key: %w", err)
	}

	// Create TLS admin key
	isCA = 0
	tlsAdminKeyReq := models.CreateKeyRequest{
		Name:        fmt.Sprintf("%s-tls-admin", params.MspID),
		Description: &description,
		Algorithm:   models.KeyAlgorithmEC,
		KeySize:     nil,
		Curve:       &curve,
		ProviderID:  &providerID,
		IsCA:        &isNotCA,
	}
	tlsAdminKey, err := s.keyManagement.CreateKey(ctx, tlsAdminKeyReq, providerID)
	if err != nil {
		_ = s.keyManagement.DeleteKey(ctx, signKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, signAdminKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, signClientKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, tlsKey.ID)
		return nil, fmt.Errorf("failed to create TLS admin key: %w", err)
	}

	// Sign the TLS admin key with the CA
	tlsAdminKey, err = s.keyManagement.SignCertificate(ctx, tlsAdminKey.ID, tlsKey.ID, models.CertificateRequest{
		CommonName:         fmt.Sprintf("%s-tls-admin", params.MspID),
		Organization:       []string{params.Name},
		OrganizationalUnit: []string{"admin"},
		Country:            params.Country,
		Locality:           params.Locality,
		Province:           params.Province,
		StreetAddress:      params.StreetAddress,
		PostalCode:         params.PostalCode,
		KeyUsage:           x509.KeyUsageCertSign,
	})
	if err != nil {
		_ = s.keyManagement.DeleteKey(ctx, signKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, signAdminKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, signClientKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, tlsKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, tlsAdminKey.ID)
		return nil, fmt.Errorf("failed to sign TLS admin certificate: %w", err)
	}

	// Create organization
	org, err := s.queries.CreateFabricOrganization(ctx, &db.CreateFabricOrganizationParams{
		MspID:           params.MspID,
		Description:     sql.NullString{String: params.Description, Valid: params.Description != ""},
		ProviderID:      sql.NullInt64{Int64: params.ProviderID, Valid: true},
		SignKeyID:       sql.NullInt64{Int64: int64(signKey.ID), Valid: true},
		TlsRootKeyID:    sql.NullInt64{Int64: int64(tlsKey.ID), Valid: true},
		AdminTlsKeyID:   sql.NullInt64{Int64: int64(tlsAdminKey.ID), Valid: true},
		AdminSignKeyID:  sql.NullInt64{Int64: int64(signAdminKey.ID), Valid: true},
		ClientSignKeyID: sql.NullInt64{Int64: int64(signClientKey.ID), Valid: true},
	})

	if err != nil {
		_ = s.keyManagement.DeleteKey(ctx, signKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, signAdminKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, signClientKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, tlsKey.ID)
		_ = s.keyManagement.DeleteKey(ctx, tlsAdminKey.ID)
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	// After creating the organization, fetch it with the provider name
	createdOrg, err := s.queries.GetFabricOrganizationWithKeys(ctx, org.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created organization: %w", err)
	}

	return toOrganizationDTO(createdOrg), nil
}

func (s *OrganizationService) GetOrganization(ctx context.Context, id int64) (*OrganizationDTO, error) {
	org, err := s.queries.GetFabricOrganizationWithKeys(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("organization not found")
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	return toOrganizationDTO(org), nil
}

func (s *OrganizationService) GetOrganizationByMspID(ctx context.Context, mspID string) (*OrganizationDTO, error) {
	org, err := s.queries.GetFabricOrganizationByMspID(ctx, mspID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("organization not found")
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}
	return mapDBOrganizationToServiceOrganization(org), nil
}

func (s *OrganizationService) UpdateOrganization(ctx context.Context, id int64, req UpdateOrganizationParams) (*OrganizationDTO, error) {
	// Get existing organization
	org, err := s.queries.GetFabricOrganization(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("organization not found")
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	// Update fields if provided
	if req.Description != nil {
		org.Description = sql.NullString{String: *req.Description, Valid: true}
	}

	// Update organization
	_, err = s.queries.UpdateFabricOrganization(ctx, &db.UpdateFabricOrganizationParams{
		ID:          id,
		Description: org.Description,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update organization: %w", err)
	}

	// Fetch the updated organization with keys
	updatedOrg, err := s.queries.GetFabricOrganizationWithKeys(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated organization: %w", err)
	}

	return toOrganizationDTO(updatedOrg), nil
}

func (s *OrganizationService) DeleteOrganization(ctx context.Context, id int64) error {
	// Get the organization first to retrieve the MspID
	org, err := s.queries.GetFabricOrganization(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("organization not found")
		}
		return fmt.Errorf("failed to get organization: %w", err)
	}

	// Delete the organization from the database
	err = s.queries.DeleteFabricOrganization(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("organization not found")
		}
		return fmt.Errorf("failed to delete organization: %w", err)
	}

	// Delete the organization directory
	// Convert MspID to lowercase for the directory name
	mspIDLower := strings.ToLower(org.MspID)

	orgDir := filepath.Join(s.configService.GetDataPath(), "orgs", mspIDLower)
	err = os.RemoveAll(orgDir)
	if err != nil {
		// Log the error but don't fail the operation
		// The database record is already deleted
		fmt.Printf("Warning: failed to delete organization directory %s: %v\n", orgDir, err)
	}

	return nil
}

func (s *OrganizationService) ListOrganizations(ctx context.Context, params PaginationParams) ([]OrganizationDTO, error) {
	orgs, err := s.queries.ListFabricOrganizationsWithKeys(ctx, &db.ListFabricOrganizationsWithKeysParams{
		Limit:  params.Limit,
		Offset: params.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list organizations: %w", err)
	}

	dtos := make([]OrganizationDTO, len(orgs))
	for i, org := range orgs {
		dtos[i] = *toOrganizationListDTO(org)
	}
	return dtos, nil
}

// GetCRL returns the current CRL for the organization in PEM format
func (s *OrganizationService) GetCRL(ctx context.Context, orgID int64) ([]byte, error) {
	// Get organization details
	org, err := s.queries.GetFabricOrganizationWithKeys(ctx, orgID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("organization not found")
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	// Get all revoked certificates for this organization
	revokedCerts, err := s.queries.GetRevokedCertificates(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get revoked certificates: %w", err)
	}

	// Get the admin signing key for signing the CRL
	adminSignKey, err := s.keyManagement.GetKey(ctx, int(org.SignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to get admin sign key: %w", err)
	}

	// Parse the certificate
	cert, err := gwidentity.CertificateFromPEM([]byte(*adminSignKey.Certificate))
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Get private key from key management service
	if org.SignKeyID.Int64 < math.MinInt || org.SignKeyID.Int64 > math.MaxInt {
		return nil, fmt.Errorf("sign key ID %d is out of valid range for int type", org.SignKeyID.Int64)
	}
	privateKeyPEM, err := s.keyManagement.GetDecryptedPrivateKey(int(org.SignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to get private key: %w", err)
	}

	// Parse the private key
	priv, err := gwidentity.PrivateKeyFromPEM([]byte(privateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Cast private key to crypto.Signer
	signer, ok := priv.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("private key does not implement crypto.Signer")
	}

	// Create CRL
	now := time.Now()
	crl := &x509.RevocationList{
		Number:     big.NewInt(1),
		ThisUpdate: now,
		NextUpdate: now.AddDate(0, 0, 7), // Valid for 7 days
	}

	// Add all revoked certificates
	for _, rc := range revokedCerts {
		serialNumber, ok := new(big.Int).SetString(rc.SerialNumber, 16)
		if !ok {
			return nil, fmt.Errorf("invalid serial number format: %s", rc.SerialNumber)
		}

		revokedCert := pkix.RevokedCertificate{
			SerialNumber:   serialNumber,
			RevocationTime: rc.RevocationTime,
			Extensions: []pkix.Extension{
				{
					Id:    asn1.ObjectIdentifier{2, 5, 29, 21}, // CRLReason OID
					Value: []byte{byte(rc.Reason)},
				},
			},
		}
		crl.RevokedCertificates = append(crl.RevokedCertificates, revokedCert)
	}

	// Create the CRL
	crlBytes, err := x509.CreateRevocationList(rand.Reader, crl, cert, signer)
	if err != nil {
		return nil, fmt.Errorf("failed to create CRL: %w", err)
	}

	// Encode the CRL in PEM format
	pemBlock := &pem.Block{
		Type:  "X509 CRL",
		Bytes: crlBytes,
	}

	return pem.EncodeToMemory(pemBlock), nil
}

// GetRevokedCertificates returns all revoked certificates for an organization
func (s *OrganizationService) GetRevokedCertificates(ctx context.Context, orgID int64) ([]RevokedCertificateDTO, error) {
	// Get organization details to verify it exists
	_, err := s.queries.GetFabricOrganizationWithKeys(ctx, orgID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("organization not found")
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	// Get all revoked certificates for this organization
	revokedCerts, err := s.queries.GetRevokedCertificates(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get revoked certificates: %w", err)
	}

	// Convert to DTOs
	dtos := make([]RevokedCertificateDTO, len(revokedCerts))
	for i, cert := range revokedCerts {
		dtos[i] = RevokedCertificateDTO{
			SerialNumber:   cert.SerialNumber,
			RevocationTime: cert.RevocationTime,
			Reason:         cert.Reason,
		}
	}

	return dtos, nil
}

// RevokeCertificate adds a certificate to the organization's CRL
func (s *OrganizationService) RevokeCertificate(ctx context.Context, orgID int64, serialNumber *big.Int, reason int) error {
	// Get organization details
	org, err := s.queries.GetFabricOrganizationWithKeys(ctx, orgID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("organization not found")
		}
		return fmt.Errorf("failed to get organization: %w", err)
	}

	if !org.SignKeyID.Valid {
		return fmt.Errorf("organization has no admin sign key")
	}

	// Check if certificate is already revoked
	revokedCerts, err := s.queries.GetRevokedCertificates(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to check revoked certificates: %w", err)
	}

	serialNumberHex := serialNumber.Text(16)
	for _, cert := range revokedCerts {
		if cert.SerialNumber == serialNumberHex {
			return fmt.Errorf("certificate with serial number %s is already revoked", serialNumberHex)
		}
	}

	// Check if CRL is initialized (has last update time)
	if !org.CrlLastUpdate.Valid {
		// Initialize CRL timestamps
		now := time.Now()
		err = s.queries.UpdateOrganizationCRL(ctx, &db.UpdateOrganizationCRLParams{
			ID:            orgID,
			CrlLastUpdate: sql.NullTime{Time: now, Valid: true},
			CrlKeyID:      org.SignKeyID,
		})
		if err != nil {
			return fmt.Errorf("failed to initialize CRL: %w", err)
		}
	}

	// Add the certificate to the database
	err = s.queries.AddRevokedCertificate(ctx, &db.AddRevokedCertificateParams{
		FabricOrganizationID: orgID,
		SerialNumber:         serialNumberHex, // Store as hex string
		RevocationTime:       time.Now(),
		Reason:               int64(reason),
		IssuerCertificateID: sql.NullInt64{
			Int64: org.SignKeyID.Int64,
			Valid: true,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add revoked certificate: %w", err)
	}

	// Update the CRL timestamps
	now := time.Now()
	err = s.queries.UpdateOrganizationCRL(ctx, &db.UpdateOrganizationCRLParams{
		ID:            orgID,
		CrlLastUpdate: sql.NullTime{Time: now, Valid: true},
		CrlKeyID:      org.AdminSignKeyID,
	})
	if err != nil {
		return fmt.Errorf("failed to update CRL timestamps: %w", err)
	}

	return nil
}

// DeleteRevokedCertificate removes a certificate from the organization's revocation list
func (s *OrganizationService) DeleteRevokedCertificate(ctx context.Context, orgID int64, serialNumber string) error {
	err := s.queries.DeleteRevokedCertificate(ctx, &db.DeleteRevokedCertificateParams{
		FabricOrganizationID: orgID,
		SerialNumber:         serialNumber,
	})
	if err != nil {
		return err
	}

	return nil
}

// KeyRole represents the role of a key in the organization
type KeyRole string

const (
	KeyRoleAdmin  KeyRole = "admin"
	KeyRoleClient KeyRole = "client"
)

// CreateKeyParams represents parameters for creating a new key
type CreateKeyParams struct {
	OrganizationID int64   `validate:"required"`
	Role           KeyRole `validate:"required,oneof=admin client"`
	Name           string  `validate:"required"`
	Description    *string
	DNSNames       []string
	IPAddresses    []string
}

// RenewCertificateParams represents parameters for renewing a certificate
type RenewCertificateParams struct {
	OrganizationID int64   `validate:"required"`
	KeyID          int64   `validate:"required"`
	Role           KeyRole `validate:"required,oneof=admin client"`
	CAType         string  `validate:"required,oneof=tls sign"`
	DNSNames       []string
	IPAddresses    []string
	ValidFor       *time.Duration
}

// CreateKey creates a new key with a specific role (admin or client)
func (s *OrganizationService) CreateKey(ctx context.Context, params CreateKeyParams) (*models.KeyResponse, error) {
	// Get organization details
	org, err := s.queries.GetFabricOrganizationWithKeys(ctx, params.OrganizationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("organization not found")
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	// Determine the organizational unit based on the role
	var organizationalUnit string

	switch params.Role {
	case KeyRoleAdmin:
		organizationalUnit = "admin"
	case KeyRoleClient:
		organizationalUnit = "client"
	default:
		return nil, fmt.Errorf("invalid key role: %s", params.Role)
	}

	// For now, we'll create both sign and TLS keys
	// In the future, this could be made configurable
	keys := make([]*models.KeyResponse, 0, 2)

	// Create sign key
	signKey, err := s.createKeyWithRole(ctx, org, params, "sign", organizationalUnit)
	if err != nil {
		return nil, fmt.Errorf("failed to create sign key: %w", err)
	}
	keys = append(keys, signKey)

	// Create TLS key
	tlsKey, err := s.createKeyWithRole(ctx, org, params, "tls", organizationalUnit)
	if err != nil {
		// Clean up the sign key if TLS key creation fails
		_ = s.keyManagement.DeleteKey(ctx, signKey.ID)
		return nil, fmt.Errorf("failed to create TLS key: %w", err)
	}
	keys = append(keys, tlsKey)

	// Return the sign key as the primary result
	// In the future, we might want to return both keys or a composite result
	return signKey, nil
}

// createKeyWithRole is a helper function to create a key with a specific role and type
func (s *OrganizationService) createKeyWithRole(ctx context.Context, org *db.GetFabricOrganizationWithKeysRow, params CreateKeyParams, keyType, organizationalUnit string) (*models.KeyResponse, error) {
	// Get the CA key for signing
	var caKeyID int64

	switch keyType {
	case "sign":
		if !org.SignKeyID.Valid {
			return nil, fmt.Errorf("organization has no sign CA key")
		}
		caKeyID = org.SignKeyID.Int64
	case "tls":
		if !org.TlsRootKeyID.Valid {
			return nil, fmt.Errorf("organization has no TLS CA key")
		}
		caKeyID = org.TlsRootKeyID.Int64
	default:
		return nil, fmt.Errorf("invalid key type: %s", keyType)
	}

	// Create the key
	description := params.Name
	if params.Description != nil {
		description = *params.Description
	}

	curve := models.ECCurveP256
	isCA := 0
	providerID := int(org.ProviderID.Int64)

	keyReq := models.CreateKeyRequest{
		Name:        fmt.Sprintf("%s-%s-%s", params.Name, keyType, params.Role),
		Description: &description,
		Algorithm:   models.KeyAlgorithmEC,
		Curve:       &curve,
		ProviderID:  &providerID,
		IsCA:        &isCA,
	}

	key, err := s.keyManagement.CreateKey(ctx, keyReq, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s key: %w", keyType, err)
	}

	// Convert string IP addresses to net.IP
	var ipAddresses []net.IP
	for _, ipStr := range params.IPAddresses {
		if ip := net.ParseIP(ipStr); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		}
	}

	// Sign the key with the CA
	certReq := models.CertificateRequest{
		CommonName:         fmt.Sprintf("%s-%s-%s", params.Name, keyType, params.Role),
		Organization:       []string{org.MspID},
		OrganizationalUnit: []string{organizationalUnit},
		DNSNames:           params.DNSNames,
		IPAddresses:        ipAddresses,
		IsCA:               false,
		ValidFor:           models.Duration(24 * 365 * time.Hour), // 1 year
		KeyUsage:           x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}

	signedKey, err := s.keyManagement.SignCertificate(ctx, key.ID, int(caKeyID), certReq)
	if err != nil {
		// Clean up the key if signing fails
		_ = s.keyManagement.DeleteKey(ctx, key.ID)
		return nil, fmt.Errorf("failed to sign %s key: %w", keyType, err)
	}

	return signedKey, nil
}

// RenewCertificate renews a certificate for a specific key role
func (s *OrganizationService) RenewCertificate(ctx context.Context, params RenewCertificateParams) (*models.KeyResponse, error) {
	// Get organization details
	org, err := s.queries.GetFabricOrganizationWithKeys(ctx, params.OrganizationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("organization not found")
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	// Verify the key belongs to this organization by checking if it's one of the known keys
	var isOrgKey bool
	switch params.Role {
	case KeyRoleAdmin:
		if (org.AdminSignKeyID.Valid && org.AdminSignKeyID.Int64 == params.KeyID) ||
			(org.AdminTlsKeyID.Valid && org.AdminTlsKeyID.Int64 == params.KeyID) {
			isOrgKey = true
		}
	case KeyRoleClient:
		if org.ClientSignKeyID.Valid && org.ClientSignKeyID.Int64 == params.KeyID {
			isOrgKey = true
		}
	}

	if !isOrgKey {
		return nil, fmt.Errorf("key %d is not a valid %s key for this organization", params.KeyID, params.Role)
	}

	// Determine the CA key ID based on the CA type
	switch params.CAType {
	case "sign":
		if !org.SignKeyID.Valid {
			return nil, fmt.Errorf("organization has no sign CA key")
		}
	case "tls":
		if !org.TlsRootKeyID.Valid {
			return nil, fmt.Errorf("organization has no TLS CA key")
		}
	default:
		return nil, fmt.Errorf("invalid CA type: %s. Must be 'tls' or 'sign'", params.CAType)
	}

	// Set default validity period if not provided
	validFor := models.Duration(24 * 365 * time.Hour) // 1 year
	if params.ValidFor != nil {
		validFor = models.Duration(*params.ValidFor)
	}

	// Determine organizational unit based on role
	var organizationalUnit string
	switch params.Role {
	case KeyRoleAdmin:
		organizationalUnit = "admin"
	case KeyRoleClient:
		organizationalUnit = "client"
	}

	// Convert string IP addresses to net.IP
	var ipAddresses []net.IP
	for _, ipStr := range params.IPAddresses {
		if ip := net.ParseIP(ipStr); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		}
	}

	// Renew the certificate
	certReq := models.CertificateRequest{
		CommonName:         fmt.Sprintf("%s-%s", org.MspID, params.Role),
		Organization:       []string{org.MspID},
		OrganizationalUnit: []string{organizationalUnit},
		DNSNames:           params.DNSNames,
		IPAddresses:        ipAddresses,
		IsCA:               false,
		ValidFor:           validFor,
		KeyUsage:           x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}

	renewedKey, err := s.keyManagement.RenewCertificate(ctx, int(params.KeyID), certReq)
	if err != nil {
		return nil, fmt.Errorf("failed to renew certificate: %w", err)
	}

	return renewedKey, nil
}

// ListKeys returns all keys for an organization
func (s *OrganizationService) ListKeys(ctx context.Context, orgID int64) (map[string]*models.KeyResponse, error) {
	// Get organization details
	org, err := s.queries.GetFabricOrganizationWithKeys(ctx, orgID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("organization not found")
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	keys := make(map[string]*models.KeyResponse)

	// Get sign CA key
	if org.SignKeyID.Valid {
		signCAKey, err := s.keyManagement.GetKey(ctx, int(org.SignKeyID.Int64))
		if err == nil {
			keys["sign-ca"] = signCAKey
		}
	}

	// Get TLS CA key
	if org.TlsRootKeyID.Valid {
		tlsCAKey, err := s.keyManagement.GetKey(ctx, int(org.TlsRootKeyID.Int64))
		if err == nil {
			keys["tls-ca"] = tlsCAKey
		}
	}

	// Get admin sign key
	if org.AdminSignKeyID.Valid {
		adminSignKey, err := s.keyManagement.GetKey(ctx, int(org.AdminSignKeyID.Int64))
		if err == nil {
			keys["admin-sign"] = adminSignKey
		}
	}

	// Get admin TLS key
	if org.AdminTlsKeyID.Valid {
		adminTlsKey, err := s.keyManagement.GetKey(ctx, int(org.AdminTlsKeyID.Int64))
		if err == nil {
			keys["admin-tls"] = adminTlsKey
		}
	}

	// Get client sign key
	if org.ClientSignKeyID.Valid {
		clientSignKey, err := s.keyManagement.GetKey(ctx, int(org.ClientSignKeyID.Int64))
		if err == nil {
			keys["client-sign"] = clientSignKey
		}
	}

	return keys, nil
}

// GetKey returns a specific key by ID
func (s *OrganizationService) GetKey(ctx context.Context, keyID int64) (*models.KeyResponse, error) {
	if keyID < math.MinInt32 || keyID > math.MaxInt32 {
		return nil, fmt.Errorf("key ID %d is out of range for int type", keyID)
	}
	key, err := s.keyManagement.GetKey(ctx, int(keyID))
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}
	return key, nil
}

// DeleteKey deletes a key and its associated certificate
func (s *OrganizationService) DeleteKey(ctx context.Context, keyID int64) error {
	if keyID < math.MinInt32 || keyID > math.MaxInt32 {
		return fmt.Errorf("key ID %d is out of range for int type", keyID)
	}
	err := s.keyManagement.DeleteKey(ctx, int(keyID))
	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}
	return nil
}
