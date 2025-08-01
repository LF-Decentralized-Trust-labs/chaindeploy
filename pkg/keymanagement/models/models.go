package models

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"crypto/x509"
)

type KeyProviderType string
type KeyAlgorithm string
type ECCurve string

const (
	KeyProviderTypeDatabase KeyProviderType = "DATABASE"
	KeyProviderTypeVault    KeyProviderType = "VAULT"
	KeyProviderTypeHSM      KeyProviderType = "HSM"

	KeyAlgorithmRSA     KeyAlgorithm = "RSA"
	KeyAlgorithmEC      KeyAlgorithm = "EC"
	KeyAlgorithmED25519 KeyAlgorithm = "ED25519"

	ECCurveP256      ECCurve = "P-256"
	ECCurveP384      ECCurve = "P-384"
	ECCurveP521      ECCurve = "P-521"
	ECCurveSECP256K1 ECCurve = "secp256k1"
)

// KeyAlgorithm represents the supported key algorithms
// @Description Supported key algorithms
type CreateKeyRequest struct {
	// Name of the key
	// @Required
	Name string `json:"name" validate:"required" example:"my-key"`

	// Optional description
	Description *string `json:"description,omitempty" example:"Key for signing certificates"`

	// Key algorithm (RSA, EC, ED25519)
	// @Required
	Algorithm KeyAlgorithm `json:"algorithm" validate:"required,oneof=RSA EC ED25519" example:"RSA"`

	// Key size in bits (for RSA)
	KeySize *int `json:"keySize,omitempty" validate:"omitempty,min=2048,max=8192" example:"2048"`

	// Elliptic curve name (for EC keys)
	Curve *ECCurve `json:"curve,omitempty" example:"P-256"`

	// Optional provider ID
	ProviderID *int `json:"providerId,omitempty" example:"1"`

	// Whether this key is a CA
	IsCA *int `json:"isCA,omitempty" example:"0"`

	// Optional: configure CA certificate properties
	Certificate *CertificateRequest `json:"certificate,omitempty"`
}

func (r *CreateKeyRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}

	// Validate algorithm
	validAlgorithms := map[KeyAlgorithm]bool{
		KeyAlgorithmRSA:     true,
		KeyAlgorithmEC:      true,
		KeyAlgorithmED25519: true,
	}
	if !validAlgorithms[r.Algorithm] {
		return fmt.Errorf("invalid algorithm: must be one of RSA, EC, ED25519")
	}

	// Validate RSA key size
	if r.Algorithm == KeyAlgorithmRSA {
		if r.KeySize == nil {
			return fmt.Errorf("key size is required for RSA keys")
		}
		if *r.KeySize < 2048 || *r.KeySize > 8192 {
			return fmt.Errorf("RSA key size must be between 2048 and 8192 bits")
		}
	}

	// Validate EC curve
	if r.Algorithm == KeyAlgorithmEC {
		if r.Curve == nil {
			return fmt.Errorf("curve is required for EC keys")
		}
		validCurves := map[ECCurve]bool{
			ECCurveP256:      true,
			ECCurveP384:      true,
			ECCurveP521:      true,
			ECCurveSECP256K1: true,
		}
		if !validCurves[*r.Curve] {
			return fmt.Errorf("invalid curve: must be one of P-256, P-384, P-521, secp256k1")
		}
	}

	return nil
}

type KeyResponse struct {
	ID                int             `json:"id"`
	Name              string          `json:"name"`
	Description       *string         `json:"description,omitempty"`
	Algorithm         KeyAlgorithm    `json:"algorithm"`
	KeySize           *int            `json:"keySize,omitempty"`
	Curve             *ECCurve        `json:"curve,omitempty"`
	Format            string          `json:"format"`
	PublicKey         string          `json:"publicKey"`
	Certificate       *string         `json:"certificate,omitempty"`
	Status            string          `json:"status"`
	CreatedAt         time.Time       `json:"createdAt"`
	ExpiresAt         *time.Time      `json:"expiresAt,omitempty"`
	LastRotatedAt     *time.Time      `json:"lastRotatedAt,omitempty"`
	SHA256Fingerprint string          `json:"sha256Fingerprint"`
	SHA1Fingerprint   string          `json:"sha1Fingerprint"`
	Provider          KeyProviderInfo `json:"provider"`
	EthereumAddress   string          `json:"ethereumAddress"`
	SigningKeyID      *int            `json:"signingKeyID,omitempty"`
}

type KeyProviderInfo struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type CreateProviderRequest struct {
	Name      string          `json:"name" validate:"required"`
	Type      KeyProviderType `json:"type" validate:"required,oneof=DATABASE VAULT HSM"`
	Config    json.RawMessage `json:"config,omitempty"`
	IsDefault int             `json:"isDefault" validate:"required,oneof=0 1"`
}

func (r *CreateProviderRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}

	validTypes := map[KeyProviderType]bool{
		KeyProviderTypeDatabase: true,
		KeyProviderTypeVault:    true,
		KeyProviderTypeHSM:      true,
	}

	if !validTypes[r.Type] {
		return fmt.Errorf("invalid provider type: must be one of DATABASE, VAULT, HSM")
	}

	return nil
}

type ProviderResponse struct {
	ID        int             `json:"id"`
	Name      string          `json:"name"`
	Type      KeyProviderType `json:"type"`
	IsDefault int             `json:"isDefault"`
	Config    json.RawMessage `json:"config,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

type PaginatedResponse struct {
	Items      []KeyResponse `json:"items"`
	TotalItems int64         `json:"totalItems"`
	Page       int           `json:"page"`
	PageSize   int           `json:"pageSize"`
}

// Add CertificateRequest model
type CertificateRequest struct {
	CommonName         string             `json:"commonName" validate:"required"`
	Organization       []string           `json:"organization,omitempty"`
	OrganizationalUnit []string           `json:"organizationalUnit,omitempty"`
	Country            []string           `json:"country,omitempty"`
	Province           []string           `json:"province,omitempty"`
	Locality           []string           `json:"locality,omitempty"`
	StreetAddress      []string           `json:"streetAddress,omitempty"`
	PostalCode         []string           `json:"postalCode,omitempty"`
	DNSNames           []string           `json:"dnsNames,omitempty"`
	EmailAddresses     []string           `json:"emailAddresses,omitempty"`
	IPAddresses        []net.IP           `json:"ipAddresses,omitempty"`
	URIs               []*url.URL         `json:"uris,omitempty"`
	ValidFor           Duration           `json:"validFor" validate:"required"`
	IsCA               bool               `json:"isCA"`
	KeyUsage           x509.KeyUsage      `json:"keyUsage"`
	ExtKeyUsage        []x509.ExtKeyUsage `json:"extKeyUsage,omitempty"`
}

// Add Duration type for JSON marshaling
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(tmp)
		return nil
	default:
		return fmt.Errorf("invalid duration")
	}
}

// Note: parseCertificate function is defined in service/service.go

// SignRequest represents a request to sign data using a key
// @Description Request to sign data using a key
type SignRequest struct {
	// Key version to use for signing (0 means latest)
	KeyVersion *int `json:"key_version,omitempty" example:"0"`

	// Hash algorithm to use for signing
	HashAlgorithm *string `json:"hash_algorithm,omitempty" example:"sha2-256"`

	// Base64 encoded input data to sign
	Input string `json:"input" validate:"required" example:"SGVsbG8gV29ybGQ="`

	// Reference string for batch operations
	Reference *string `json:"reference,omitempty" example:"batch-ref-1"`

	// Context for key derivation (base64 encoded)
	Context *string `json:"context,omitempty" example:"Y29udGV4dA=="`

	// Signature context for Ed25519ctx and Ed25519ph signatures
	SignatureContext *string `json:"signature_context,omitempty" example:"c2lnbmF0dXJlLWNvbnRleHQ="`

	// Whether input is already hashed
	Prehashed *bool `json:"prehashed,omitempty" example:"false"`

	// RSA signature algorithm (pss or pkcs1v15)
	SignatureAlgorithm *string `json:"signature_algorithm,omitempty" example:"pss"`

	// Marshaling algorithm for ECDSA (asn1 or jws)
	MarshalingAlgorithm *string `json:"marshaling_algorithm,omitempty" example:"asn1"`

	// Salt length for RSA PSS (auto, hash, or integer)
	SaltLength *string `json:"salt_length,omitempty" example:"auto"`
}

// BatchSignRequest represents a batch signing request
// @Description Batch signing request
type BatchSignRequest struct {
	BatchInput []SignRequest `json:"batch_input" validate:"required,min=1,max=100"`
}

// SignResponse represents the response from a signing operation
// @Description Response from a signing operation
type SignResponse struct {
	// Base64 encoded signature
	Signature string `json:"signature" example:"c2lnbmF0dXJlLWJhc2U2NA=="`

	// Key version used for signing
	KeyVersion int `json:"key_version" example:"1"`

	// Reference from the request (for batch operations)
	Reference *string `json:"reference,omitempty" example:"batch-ref-1"`
}

// BatchSignResponse represents the response from a batch signing operation
// @Description Response from a batch signing operation
type BatchSignResponse struct {
	BatchResults []BatchSignResult `json:"batch_results"`
}

// BatchSignResult represents a single result in a batch signing operation
// @Description Single result in a batch signing operation
type BatchSignResult struct {
	// Base64 encoded signature
	Signature *string `json:"signature,omitempty" example:"c2lnbmF0dXJlLWJhc2U2NA=="`

	// Key version used for signing
	KeyVersion *int `json:"key_version,omitempty" example:"1"`

	// Reference from the request
	Reference *string `json:"reference,omitempty" example:"batch-ref-1"`

	// Error message if signing failed
	Error *string `json:"error,omitempty" example:"invalid input data"`
}

// Supported hash algorithms
const (
	HashAlgorithmSHA1     = "sha1"
	HashAlgorithmSHA2_224 = "sha2-224"
	HashAlgorithmSHA2_256 = "sha2-256"
	HashAlgorithmSHA2_384 = "sha2-384"
	HashAlgorithmSHA2_512 = "sha2-512"
	HashAlgorithmSHA3_224 = "sha3-224"
	HashAlgorithmSHA3_256 = "sha3-256"
	HashAlgorithmSHA3_384 = "sha3-384"
	HashAlgorithmSHA3_512 = "sha3-512"
	HashAlgorithmNone     = "none"
)

// Supported signature algorithms
const (
	SignatureAlgorithmPSS      = "pss"
	SignatureAlgorithmPKCS1V15 = "pkcs1v15"
)

// Supported marshaling algorithms
const (
	MarshalingAlgorithmASN1 = "asn1"
	MarshalingAlgorithmJWS  = "jws"
)

// Supported salt length values
const (
	SaltLengthAuto = "auto"
	SaltLengthHash = "hash"
)

// ValidateSignRequest validates a sign request
func (r *SignRequest) Validate() error {
	if r.Input == "" {
		return fmt.Errorf("input is required")
	}

	// Validate hash algorithm if provided
	if r.HashAlgorithm != nil {
		validHashAlgorithms := map[string]bool{
			HashAlgorithmSHA1:     true,
			HashAlgorithmSHA2_224: true,
			HashAlgorithmSHA2_256: true,
			HashAlgorithmSHA2_384: true,
			HashAlgorithmSHA2_512: true,
			HashAlgorithmSHA3_224: true,
			HashAlgorithmSHA3_256: true,
			HashAlgorithmSHA3_384: true,
			HashAlgorithmSHA3_512: true,
			HashAlgorithmNone:     true,
		}
		if !validHashAlgorithms[*r.HashAlgorithm] {
			return fmt.Errorf("invalid hash algorithm: %s", *r.HashAlgorithm)
		}
	}

	// Validate signature algorithm if provided
	if r.SignatureAlgorithm != nil {
		validSigAlgorithms := map[string]bool{
			SignatureAlgorithmPSS:      true,
			SignatureAlgorithmPKCS1V15: true,
		}
		if !validSigAlgorithms[*r.SignatureAlgorithm] {
			return fmt.Errorf("invalid signature algorithm: %s", *r.SignatureAlgorithm)
		}
	}

	// Validate marshaling algorithm if provided
	if r.MarshalingAlgorithm != nil {
		validMarshalingAlgorithms := map[string]bool{
			MarshalingAlgorithmASN1: true,
			MarshalingAlgorithmJWS:  true,
		}
		if !validMarshalingAlgorithms[*r.MarshalingAlgorithm] {
			return fmt.Errorf("invalid marshaling algorithm: %s", *r.MarshalingAlgorithm)
		}
	}

	// Validate salt length if provided
	if r.SaltLength != nil {
		if *r.SaltLength != SaltLengthAuto && *r.SaltLength != SaltLengthHash {
			// Check if it's a valid integer
			if _, err := strconv.Atoi(*r.SaltLength); err != nil {
				return fmt.Errorf("invalid salt length: %s", *r.SaltLength)
			}
		}
	}

	return nil
}

// ValidateBatchSignRequest validates a batch sign request
func (r *BatchSignRequest) Validate() error {
	if len(r.BatchInput) == 0 {
		return fmt.Errorf("batch_input is required and cannot be empty")
	}

	if len(r.BatchInput) > 100 {
		return fmt.Errorf("batch_input cannot contain more than 100 items")
	}

	for i, item := range r.BatchInput {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("batch_input[%d]: %w", i, err)
		}
	}

	return nil
}
