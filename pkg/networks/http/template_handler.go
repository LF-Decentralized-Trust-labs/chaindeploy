package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/chainlaunch/chainlaunch/pkg/networks/service/template"
)

// TemplateHandler handles HTTP requests for network template operations
type TemplateHandler struct {
	templateService  *template.TemplateService
	networkCreator   template.NetworkCreator
	chaincodeCreator template.ChaincodeCreator
}

// NewTemplateHandler creates a new template handler
func NewTemplateHandler(
	templateService *template.TemplateService,
	networkCreator template.NetworkCreator,
	chaincodeCreator template.ChaincodeCreator,
) *TemplateHandler {
	return &TemplateHandler{
		templateService:  templateService,
		networkCreator:   networkCreator,
		chaincodeCreator: chaincodeCreator,
	}
}

// RegisterTemplateRoutes registers the template routes on both fabric and besu network routers
func (h *TemplateHandler) RegisterTemplateRoutes(r chi.Router) {
	r.Route("/networks/templates", func(r chi.Router) {
		// Export a network as template (any platform)
		r.Get("/export/{id}", h.ExportTemplate)
		// Validate a template import
		r.Post("/validate", h.ValidateTemplateImport)
		// Import from a template
		r.Post("/import", h.ImportFromTemplate)
	})
}

// ExportTemplate exports a network configuration as a reusable template
// @Summary Export network as template
// @Description Export a network's configuration as a reusable template that can be imported to create similar networks. Supports both Fabric and Besu platforms.
// @Tags Network Templates
// @Produce json
// @Param id path int true "Network ID"
// @Success 200 {object} template.ExportTemplateResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BasicAuth
// @Security CookieAuth
// @Router /networks/templates/export/{id} [get]
// @ID exportNetworkTemplate
func (h *TemplateHandler) ExportTemplate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	networkID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "Invalid network ID")
		return
	}

	tmpl, err := h.templateService.ExportNetworkTemplate(r.Context(), networkID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export_failed", err.Error())
		return
	}

	response := template.ExportTemplateResponse{
		Template: *tmpl,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ValidateTemplateImport validates a template import request without making changes
// @Summary Validate template import
// @Description Validate a template import request and get a preview of what would be created. Supports both Fabric and Besu templates.
// @Tags Network Templates
// @Accept json
// @Produce json
// @Param request body template.ValidateTemplateRequest true "Template validation request"
// @Success 200 {object} template.ValidateTemplateResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BasicAuth
// @Security CookieAuth
// @Router /networks/templates/validate [post]
// @ID validateTemplateImport
func (h *TemplateHandler) ValidateTemplateImport(w http.ResponseWriter, r *http.Request) {
	var req template.ValidateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body: "+err.Error())
		return
	}

	response, err := h.templateService.ValidateTemplateImport(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "validation_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ImportFromTemplate imports a network from a template
// @Summary Import network from template
// @Description Create a new network from a template with organization and node mappings. Supports both Fabric and Besu templates, with optional chaincode/smart contract definitions.
// @Tags Network Templates
// @Accept json
// @Produce json
// @Param request body template.ImportTemplateRequest true "Template import request"
// @Success 201 {object} template.ImportTemplateResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BasicAuth
// @Security CookieAuth
// @Router /networks/templates/import [post]
// @ID importNetworkFromTemplate
func (h *TemplateHandler) ImportFromTemplate(w http.ResponseWriter, r *http.Request) {
	var req template.ImportTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body: "+err.Error())
		return
	}

	response, err := h.templateService.ImportFromTemplate(r.Context(), &req, h.networkCreator, h.chaincodeCreator)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "import_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// NetworkServiceAdapter adapts NetworkService to implement NetworkCreator interface
type NetworkServiceAdapter struct {
	createFunc func(ctx context.Context, name, description string, configData []byte) (interface{}, error)
}

func NewNetworkServiceAdapter(createFunc func(ctx context.Context, name, description string, configData []byte) (interface{}, error)) *NetworkServiceAdapter {
	return &NetworkServiceAdapter{createFunc: createFunc}
}

func (a *NetworkServiceAdapter) CreateNetwork(ctx context.Context, name, description string, configData []byte) (interface{}, error) {
	return a.createFunc(ctx, name, description, configData)
}

// ChaincodeServiceAdapter adapts chaincode service for template import
type ChaincodeServiceAdapter struct {
	createChaincodeFunc    func(ctx context.Context, name string, networkID int64) (int64, error)
	createDefinitionFunc   func(ctx context.Context, chaincodeID int64, version string, sequence int64, dockerImage string, endorsementPolicy string, chaincodeAddress string) (int64, error)
}

func NewChaincodeServiceAdapter(
	createChaincodeFunc func(ctx context.Context, name string, networkID int64) (int64, error),
	createDefinitionFunc func(ctx context.Context, chaincodeID int64, version string, sequence int64, dockerImage string, endorsementPolicy string, chaincodeAddress string) (int64, error),
) *ChaincodeServiceAdapter {
	return &ChaincodeServiceAdapter{
		createChaincodeFunc:  createChaincodeFunc,
		createDefinitionFunc: createDefinitionFunc,
	}
}

func (a *ChaincodeServiceAdapter) CreateChaincode(ctx context.Context, name string, networkID int64) (int64, error) {
	return a.createChaincodeFunc(ctx, name, networkID)
}

func (a *ChaincodeServiceAdapter) CreateChaincodeDefinition(ctx context.Context, chaincodeID int64, version string, sequence int64, dockerImage string, endorsementPolicy string, chaincodeAddress string) (int64, error) {
	return a.createDefinitionFunc(ctx, chaincodeID, version, sequence, dockerImage, endorsementPolicy, chaincodeAddress)
}
