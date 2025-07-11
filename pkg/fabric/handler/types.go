package handler

import (
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/fabric/service"
)

// HTTP layer request/response structs
type CreateOrganizationRequest struct {
	MspID       string `json:"mspId" validate:"required"`
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
	ProviderID  int64  `json:"providerId"`

	// CA certificate properties
	CommonName    string   `json:"commonName"`
	Country       []string `json:"country"`
	Province      []string `json:"province"`
	Locality      []string `json:"locality"`
	StreetAddress []string `json:"streetAddress"`
	PostalCode    []string `json:"postalCode"`
}

type UpdateOrganizationRequest struct {
	Description *string `json:"description"`
}

// OrganizationResponse represents the HTTP response structure
type OrganizationResponse struct {
	ID              int64     `json:"id"`
	MspID           string    `json:"mspId"`
	Description     string    `json:"description,omitempty"`
	SignPublicKey   string    `json:"signPublicKey"`
	SignCertificate string    `json:"signCertificate"`
	TlsPublicKey    string    `json:"tlsPublicKey"`
	TlsCertificate  string    `json:"tlsCertificate"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	ProviderID      int64     `json:"providerId"`
	ProviderName    string    `json:"providerName,omitempty"`
	AdminTlsKeyID   int64     `json:"adminTlsKeyId,omitempty"`
	AdminSignKeyID  int64     `json:"adminSignKeyId,omitempty"`
	ClientSignKeyID int64     `json:"clientSignKeyId,omitempty"`
}

// Convert service DTO to HTTP response
func toOrganizationResponse(dto *service.OrganizationDTO) *OrganizationResponse {
	resp := &OrganizationResponse{
		ID:              dto.ID,
		MspID:           dto.MspID,
		Description:     dto.Description.String,
		SignPublicKey:   dto.SignPublicKey,
		SignCertificate: dto.SignCertificate,
		TlsPublicKey:    dto.TlsPublicKey,
		TlsCertificate:  dto.TlsCertificate,
		CreatedAt:       dto.CreatedAt,
		UpdatedAt:       dto.UpdatedAt,
		ProviderID:      dto.ProviderID,
		ProviderName:    dto.ProviderName,
	}

	if dto.AdminTlsKeyID.Valid {
		resp.AdminTlsKeyID = dto.AdminTlsKeyID.Int64
	}
	if dto.AdminSignKeyID.Valid {
		resp.AdminSignKeyID = dto.AdminSignKeyID.Int64
	}
	if dto.ClientSignKeyID.Valid {
		resp.ClientSignKeyID = dto.ClientSignKeyID.Int64
	}

	return resp
}
