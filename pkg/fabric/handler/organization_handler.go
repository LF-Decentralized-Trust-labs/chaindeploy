package handler

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/errors"
	"github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	"github.com/chainlaunch/chainlaunch/pkg/http/response"
	kmodels "github.com/chainlaunch/chainlaunch/pkg/keymanagement/models"
	"github.com/go-chi/chi/v5"
)

type OrganizationHandler struct {
	service *service.OrganizationService
}

func NewOrganizationHandler(service *service.OrganizationService) *OrganizationHandler {
	return &OrganizationHandler{
		service: service,
	}
}

// RevokeCertificateBySerialRequest represents the request to revoke a certificate by serial number
type RevokeCertificateBySerialRequest struct {
	SerialNumber     string `json:"serialNumber"` // Hex string of the serial number
	RevocationReason int    `json:"revocationReason"`
}

// RevokeCertificateByPEMRequest represents the request to revoke a certificate by PEM data
type RevokeCertificateByPEMRequest struct {
	Certificate      string `json:"certificate"` // PEM encoded certificate
	RevocationReason int    `json:"revocationReason"`
}

// DeleteRevokedCertificateRequest represents the request to delete a revoked certificate by serial number
type DeleteRevokedCertificateRequest struct {
	SerialNumber string `json:"serialNumber"` // Hex string of the serial number
}

// PaginatedOrganizationsResponse represents a paginated list of organizations for HTTP response
// swagger:model PaginatedOrganizationsResponse
type PaginatedOrganizationsResponse struct {
	Items  []*OrganizationResponse `json:"items"`
	Limit  int64                   `json:"limit"`
	Offset int64                   `json:"offset"`
	Count  int                     `json:"count"`
}

// ListOrganizationsQuery represents the query parameters for listing organizations
// swagger:model ListOrganizationsQuery
type ListOrganizationsQuery struct {
	Limit  int64 `form:"limit" json:"limit" query:"limit" example:"20"`
	Offset int64 `form:"offset" json:"offset" query:"offset" example:"0"`
}

// RegisterRoutes registers the organization routes
func (h *OrganizationHandler) RegisterRoutes(r chi.Router) {
	r.Route("/organizations", func(r chi.Router) {
		r.Post("/", response.Middleware(h.CreateOrganization))
		r.Get("/", response.Middleware(h.ListOrganizations))
		r.Get("/by-mspid/{mspid}", response.Middleware(h.GetOrganizationByMspID))
		r.Get("/{id}", response.Middleware(h.GetOrganization))
		r.Put("/{id}", response.Middleware(h.UpdateOrganization))
		r.Delete("/{id}", response.Middleware(h.DeleteOrganization))

		// Add CRL-related routes
		r.Route("/{id}/crl", func(r chi.Router) {
			r.Post("/revoke/serial", response.Middleware(h.RevokeCertificateBySerial))
			r.Post("/revoke/pem", response.Middleware(h.RevokeCertificateByPEM))
			r.Delete("/revoke/serial", response.Middleware(h.DeleteRevokedCertificate))
			r.Get("/", response.Middleware(h.GetCRL))
		})
		r.Get("/{id}/revoked-certificates", response.Middleware(h.GetRevokedCertificates))

		// Add key management routes
		r.Route("/{id}/keys", func(r chi.Router) {
			r.Post("/", response.Middleware(h.CreateKey))
			r.Post("/renew", response.Middleware(h.RenewCertificate))
			r.Get("/", response.Middleware(h.ListKeys))
			r.Get("/{keyId}", response.Middleware(h.GetKey))
			r.Delete("/{keyId}", response.Middleware(h.DeleteKey))
		})
	})
}

// @Summary Create a new Fabric organization
// @Description Create a new Fabric organization with the specified configuration
// @Tags Organizations
// @Accept json
// @Produce json
// @Param request body CreateOrganizationRequest true "Organization creation request"
// @Success 201 {object} OrganizationResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations [post]
func (h *OrganizationHandler) CreateOrganization(w http.ResponseWriter, r *http.Request) error {
	var req CreateOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_REQUEST_BODY",
		})
	}

	params := service.CreateOrganizationParams{
		MspID:       req.MspID,
		Name:        req.Name,
		Description: req.Description,
		ProviderID:  req.ProviderID,
	}

	org, err := h.service.CreateOrganization(r.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return errors.NewValidationError("organization already exists", map[string]interface{}{
				"detail": err.Error(),
				"code":   "ORGANIZATION_ALREADY_EXISTS",
			})
		}
		return errors.NewInternalError("failed to create organization", err, nil)
	}

	return response.WriteJSON(w, http.StatusCreated, toOrganizationResponse(org))
}

// @Summary Get a Fabric organization
// @Description Get a Fabric organization by ID
// @Tags Organizations
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Success 200 {object} OrganizationResponse
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id} [get]
func (h *OrganizationHandler) GetOrganization(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid organization ID", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_ID_FORMAT",
		})
	}

	org, err := h.service.GetOrganization(r.Context(), id)
	if err != nil {
		return errors.NewNotFoundError("organization not found", map[string]interface{}{
			"code":   "ORGANIZATION_NOT_FOUND",
			"detail": err.Error(),
		})
	}

	return response.WriteJSON(w, http.StatusOK, toOrganizationResponse(org))
}

// @Summary Get a Fabric organization by MSP ID
// @Description Get a Fabric organization by MSP ID
// @Tags Organizations
// @Accept json
// @Produce json
// @Param mspid path string true "MSP ID"
// @Success 200 {object} OrganizationResponse
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/by-mspid/{mspid} [get]
func (h *OrganizationHandler) GetOrganizationByMspID(w http.ResponseWriter, r *http.Request) error {
	mspid := chi.URLParam(r, "mspid")
	if mspid == "" {
		return errors.NewValidationError("invalid MSP ID", map[string]interface{}{
			"code":   "INVALID_MSPID",
			"detail": "MSP ID cannot be empty",
		})
	}

	org, err := h.service.GetOrganizationByMspID(r.Context(), mspid)
	if err != nil {
		return errors.NewNotFoundError("organization not found", map[string]interface{}{
			"code":   "ORGANIZATION_NOT_FOUND",
			"detail": err.Error(),
		})
	}

	return response.WriteJSON(w, http.StatusOK, toOrganizationResponse(org))
}

// @Summary Update a Fabric organization
// @Description Update an existing Fabric organization
// @Tags Organizations
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body UpdateOrganizationRequest true "Organization update request"
// @Success 200 {object} OrganizationResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id} [put]
func (h *OrganizationHandler) UpdateOrganization(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid organization ID", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_ID_FORMAT",
		})
	}

	var req UpdateOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_REQUEST_BODY",
		})
	}

	org, err := h.service.UpdateOrganization(r.Context(), id, service.UpdateOrganizationParams{
		Description: req.Description,
	})
	if err != nil {
		return errors.NewInternalError("failed to update organization", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, toOrganizationResponse(org))
}

// @Summary Delete a Fabric organization
// @Description Delete a Fabric organization by ID
// @Tags Organizations
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Success 204 "No Content"
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id} [delete]
func (h *OrganizationHandler) DeleteOrganization(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid organization ID", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_ID_FORMAT",
		})
	}

	if err := h.service.DeleteOrganization(r.Context(), id); err != nil {
		return errors.NewInternalError("failed to delete organization", err, nil)
	}

	return response.WriteJSON(w, http.StatusNoContent, nil)
}

// @Summary List all Fabric organizations
// @Description Get a list of all Fabric organizations
// @Tags Organizations
// @Accept json
// @Produce json
// @Param limit query int false "Maximum number of organizations to return" default(20)
// @Param offset query int false "Number of organizations to skip" default(0)
// @Success 200 {object} PaginatedOrganizationsResponse
// @Failure 500 {object} map[string]string
// @Router /organizations [get]
func (h *OrganizationHandler) ListOrganizations(w http.ResponseWriter, r *http.Request) error {
	// Parse pagination query params
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	var (
		limit  int64 = 20 // default limit
		offset int64 = 0
		err    error
	)
	if limitStr != "" {
		limit, err = strconv.ParseInt(limitStr, 10, 64)
		if err != nil || limit <= 0 {
			return errors.NewValidationError("invalid limit parameter", map[string]interface{}{
				"detail": "limit must be a positive integer",
				"code":   "INVALID_LIMIT",
			})
		}
	}
	if offsetStr != "" {
		offset, err = strconv.ParseInt(offsetStr, 10, 64)
		if err != nil || offset < 0 {
			return errors.NewValidationError("invalid offset parameter", map[string]interface{}{
				"detail": "offset must be a non-negative integer",
				"code":   "INVALID_OFFSET",
			})
		}
	}

	orgs, err := h.service.ListOrganizations(r.Context(), service.PaginationParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return errors.NewInternalError("failed to list organizations", err, nil)
	}

	orgResponses := make([]*OrganizationResponse, len(orgs))
	for i, org := range orgs {
		orgResponses[i] = toOrganizationResponse(&org)
	}

	// Optionally, you can return pagination info in the response
	resp := PaginatedOrganizationsResponse{
		Items:  orgResponses,
		Limit:  limit,
		Offset: offset,
		Count:  len(orgResponses),
	}

	return response.WriteJSON(w, http.StatusOK, resp)
}

// @Summary Revoke a certificate using its serial number
// @Description Add a certificate to the organization's CRL using its serial number
// @Tags Organizations
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body RevokeCertificateBySerialRequest true "Certificate revocation request"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/crl/revoke/serial [post]
func (h *OrganizationHandler) RevokeCertificateBySerial(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid organization ID", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_ID_FORMAT",
		})
	}

	var req RevokeCertificateBySerialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_REQUEST_BODY",
		})
	}

	serialNumber, ok := new(big.Int).SetString(req.SerialNumber, 16)
	if !ok {
		return errors.NewValidationError("invalid serial number format", map[string]interface{}{
			"code":   "INVALID_SERIAL_NUMBER_FORMAT",
			"detail": "Invalid serial number format",
		})
	}

	err = h.service.RevokeCertificate(r.Context(), id, serialNumber, req.RevocationReason)
	if err != nil {
		return errors.NewInternalError("failed to revoke certificate", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, map[string]string{"message": "Certificate revoked successfully"})
}

// @Summary Revoke a certificate using PEM data
// @Description Add a certificate to the organization's CRL using its PEM encoded data
// @Tags Organizations
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body RevokeCertificateByPEMRequest true "Certificate revocation request"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/crl/revoke/pem [post]
func (h *OrganizationHandler) RevokeCertificateByPEM(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid organization ID", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_ID_FORMAT",
		})
	}

	var req RevokeCertificateByPEMRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_REQUEST_BODY",
		})
	}

	block, _ := pem.Decode([]byte(req.Certificate))
	if block == nil || block.Type != "CERTIFICATE" {
		return errors.NewValidationError("invalid certificate PEM data", map[string]interface{}{
			"code":   "INVALID_CERTIFICATE_PEM_DATA",
			"detail": "Invalid certificate PEM data",
		})
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return errors.NewValidationError("failed to parse certificate", map[string]interface{}{
			"detail": err.Error(),
			"code":   "FAILED_TO_PARSE_CERTIFICATE",
		})
	}

	err = h.service.RevokeCertificate(r.Context(), id, cert.SerialNumber, req.RevocationReason)
	if err != nil {
		return errors.NewInternalError("failed to revoke certificate", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, map[string]string{
		"message":      "Certificate revoked successfully",
		"serialNumber": cert.SerialNumber.Text(16),
	})
}

// @Summary Get organization's CRL
// @Description Get the current Certificate Revocation List for the organization
// @Tags Organizations
// @Accept json
// @Produce application/x-pem-file
// @Param id path int true "Organization ID"
// @Success 200 {string} string "PEM encoded CRL"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/crl [get]
func (h *OrganizationHandler) GetCRL(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid organization ID", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_ID_FORMAT",
		})
	}

	crlBytes, err := h.service.GetCRL(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get CRL", err, nil)
	}

	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", "attachment; filename=crl.pem")
	_, err = w.Write(crlBytes)
	if err != nil {
		return errors.NewInternalError("failed to write response", err, nil)
	}

	return nil
}

// @Summary Get organization's revoked certificates
// @Description Get all revoked certificates for the organization
// @Tags Organizations
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Success 200 {array} RevokedCertificateResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/revoked-certificates [get]
func (h *OrganizationHandler) GetRevokedCertificates(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewInternalError("failed to parse organization ID", err, nil)
	}

	certs, err := h.service.GetRevokedCertificates(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get revoked certificates", err, nil)
	}

	certsResponse := make([]RevokedCertificateResponse, len(certs))
	for i, cert := range certs {
		certsResponse[i] = RevokedCertificateResponse{
			SerialNumber:   cert.SerialNumber,
			RevocationTime: cert.RevocationTime,
			Reason:         cert.Reason,
		}
	}

	return response.WriteJSON(w, http.StatusOK, certsResponse)
}

// @Summary Delete a revoked certificate using its serial number
// @Description Remove a certificate from the organization's CRL using its serial number
// @Tags Organizations
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body DeleteRevokedCertificateRequest true "Certificate deletion request"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/crl/revoke/serial [delete]
func (h *OrganizationHandler) DeleteRevokedCertificate(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid organization ID", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_ID_FORMAT",
		})
	}

	var req DeleteRevokedCertificateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_REQUEST_BODY",
		})
	}

	err = h.service.DeleteRevokedCertificate(r.Context(), id, req.SerialNumber)
	if err != nil {
		// Check if it's a not found error from the service
		if errors.IsType(err, errors.NotFoundError) {
			return errors.NewNotFoundError("certificate not found", map[string]interface{}{
				"code":   "CERTIFICATE_NOT_FOUND",
				"detail": "The specified certificate was not found in the revocation list",
			})
		}
		return errors.NewInternalError("failed to delete revoked certificate", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Certificate successfully removed from revocation list",
	})
}

// CreateKeyRequest represents the request to create a new key
type CreateKeyRequest struct {
	Role        string   `json:"role" validate:"required,oneof=admin client" example:"admin"`
	Name        string   `json:"name" validate:"required" example:"new-admin-key"`
	Description *string  `json:"description,omitempty" example:"New admin key for organization"`
	DNSNames    []string `json:"dnsNames,omitempty" example:"admin.example.com"`
	IPAddresses []string `json:"ipAddresses,omitempty" example:"192.168.1.100"`
}

// RenewCertificateRequest represents the request to renew a certificate
type RenewCertificateRequest struct {
	KeyID       int64    `json:"keyId" validate:"required" example:"123"`
	Role        string   `json:"role" validate:"required,oneof=admin client" example:"admin"`
	CAType      string   `json:"caType" validate:"required,oneof=tls sign" example:"sign"`
	DNSNames    []string `json:"dnsNames,omitempty" example:"admin.example.com"`
	IPAddresses []string `json:"ipAddresses,omitempty" example:"192.168.1.100"`
	ValidFor    *string  `json:"validFor,omitempty" example:"8760h"` // Duration in Go format (e.g., "8760h" for 1 year)
}

// KeyResponse represents a key in the HTTP response
type KeyResponse struct {
	ID                int        `json:"id"`
	Name              string     `json:"name"`
	Description       *string    `json:"description,omitempty"`
	Algorithm         string     `json:"algorithm"`
	PublicKey         string     `json:"publicKey"`
	Certificate       *string    `json:"certificate,omitempty"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"createdAt"`
	ExpiresAt         *time.Time `json:"expiresAt,omitempty"`
	LastRotatedAt     *time.Time `json:"lastRotatedAt,omitempty"`
	SHA256Fingerprint string     `json:"sha256Fingerprint"`
	SHA1Fingerprint   string     `json:"sha1Fingerprint"`
	Provider          struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"provider"`
	EthereumAddress string `json:"ethereumAddress"`
	SigningKeyID    *int   `json:"signingKeyId,omitempty"`
}

// ListKeysResponse represents the response for listing keys
type ListKeysResponse struct {
	Keys map[string]*KeyResponse `json:"keys"`
}

// @Summary Create a new key for an organization
// @Description Create a new key with a specific role (admin or client) for an organization
// @Tags Organizations
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body CreateKeyRequest true "Key creation request"
// @Success 201 {object} KeyResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/keys [post]
func (h *OrganizationHandler) CreateKey(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid organization ID", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_ID_FORMAT",
		})
	}

	var req CreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_REQUEST_BODY",
		})
	}

	params := service.CreateKeyParams{
		OrganizationID: id,
		Role:           service.KeyRole(req.Role),
		Name:           req.Name,
		Description:    req.Description,
		DNSNames:       req.DNSNames,
		IPAddresses:    req.IPAddresses,
	}

	key, err := h.service.CreateKey(r.Context(), params)
	if err != nil {
		return errors.NewInternalError("failed to create key", err, nil)
	}

	return response.WriteJSON(w, http.StatusCreated, toKeyResponse(key))
}

// @Summary Renew a certificate for a key
// @Description Renew a certificate for a specific key in an organization
// @Tags Organizations
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body RenewCertificateRequest true "Certificate renewal request"
// @Success 200 {object} KeyResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/keys/renew [post]
func (h *OrganizationHandler) RenewCertificate(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid organization ID", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_ID_FORMAT",
		})
	}

	var req RenewCertificateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_REQUEST_BODY",
		})
	}

	// Parse ValidFor duration if provided
	var validFor *time.Duration
	if req.ValidFor != nil {
		duration, err := time.ParseDuration(*req.ValidFor)
		if err != nil {
			return errors.NewValidationError("invalid validFor duration", map[string]interface{}{
				"detail": err.Error(),
				"code":   "INVALID_DURATION_FORMAT",
			})
		}
		validFor = &duration
	}

	params := service.RenewCertificateParams{
		OrganizationID: id,
		KeyID:          req.KeyID,
		Role:           service.KeyRole(req.Role),
		CAType:         req.CAType,
		DNSNames:       req.DNSNames,
		IPAddresses:    req.IPAddresses,
		ValidFor:       validFor,
	}

	key, err := h.service.RenewCertificate(r.Context(), params)
	if err != nil {
		return errors.NewInternalError("failed to renew certificate", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, toKeyResponse(key))
}

// @Summary List all keys for an organization
// @Description Get all keys associated with an organization
// @Tags Organizations
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Success 200 {object} ListKeysResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/keys [get]
func (h *OrganizationHandler) ListKeys(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid organization ID", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_ID_FORMAT",
		})
	}

	keys, err := h.service.ListKeys(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to list keys", err, nil)
	}

	// Convert service keys to response format
	responseKeys := make(map[string]*KeyResponse)
	for keyName, key := range keys {
		responseKeys[keyName] = toKeyResponse(key)
	}

	return response.WriteJSON(w, http.StatusOK, ListKeysResponse{Keys: responseKeys})
}

// @Summary Get a specific key by ID
// @Description Get a specific key by its ID
// @Tags Organizations
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param keyId path int true "Key ID"
// @Success 200 {object} KeyResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/keys/{keyId} [get]
func (h *OrganizationHandler) GetKey(w http.ResponseWriter, r *http.Request) error {
	keyID, err := strconv.ParseInt(chi.URLParam(r, "keyId"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid key ID", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_KEY_ID_FORMAT",
		})
	}

	key, err := h.service.GetKey(r.Context(), keyID)
	if err != nil {
		return errors.NewNotFoundError("key not found", map[string]interface{}{
			"code":   "KEY_NOT_FOUND",
			"detail": err.Error(),
		})
	}

	return response.WriteJSON(w, http.StatusOK, toKeyResponse(key))
}

// @Summary Delete a key
// @Description Delete a key and its associated certificate
// @Tags Organizations
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param keyId path int true "Key ID"
// @Success 204 "No Content"
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/keys/{keyId} [delete]
func (h *OrganizationHandler) DeleteKey(w http.ResponseWriter, r *http.Request) error {
	keyID, err := strconv.ParseInt(chi.URLParam(r, "keyId"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid key ID", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_KEY_ID_FORMAT",
		})
	}

	err = h.service.DeleteKey(r.Context(), keyID)
	if err != nil {
		return errors.NewInternalError("failed to delete key", err, nil)
	}

	return response.WriteJSON(w, http.StatusNoContent, nil)
}

// Helper function to convert service key to response format
func toKeyResponse(key *kmodels.KeyResponse) *KeyResponse {
	if key == nil {
		return nil
	}

	response := &KeyResponse{
		ID:                key.ID,
		Name:              key.Name,
		Description:       key.Description,
		Algorithm:         string(key.Algorithm),
		PublicKey:         key.PublicKey,
		Certificate:       key.Certificate,
		Status:            key.Status,
		CreatedAt:         key.CreatedAt,
		ExpiresAt:         key.ExpiresAt,
		LastRotatedAt:     key.LastRotatedAt,
		SHA256Fingerprint: key.SHA256Fingerprint,
		SHA1Fingerprint:   key.SHA1Fingerprint,
		EthereumAddress:   key.EthereumAddress,
		SigningKeyID:      key.SigningKeyID,
	}

	response.Provider.ID = key.Provider.ID
	response.Provider.Name = key.Provider.Name

	return response
}

// RevokedCertificateResponse represents the response for a revoked certificate
type RevokedCertificateResponse struct {
	SerialNumber   string    `json:"serialNumber"`
	RevocationTime time.Time `json:"revocationTime"`
	Reason         int64     `json:"reason"`
}
