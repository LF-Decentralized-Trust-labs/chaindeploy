package chainlaunchdeploy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/chainlaunch/chainlaunch/pkg/audit"
	"github.com/chainlaunch/chainlaunch/pkg/errors"
	"github.com/chainlaunch/chainlaunch/pkg/http/response"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	nodeService "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// Handler handles HTTP requests for smart contract deployment
type Handler struct {
	auditService     *audit.AuditService
	logger           *logger.Logger
	besuDeployer     DeployerWithAudit
	validate         *validator.Validate
	nodeService      *nodeService.NodeService
	chaincodeService *ChaincodeService
}

// NewHandler creates a new smart contract deploy handler
func NewHandler(auditService *audit.AuditService, logger *logger.Logger, besuDeployer DeployerWithAudit, nodeService *nodeService.NodeService, chaincodeService *ChaincodeService) *Handler {
	SetFabricAuditService(auditService)
	if besuDeployer != nil {
		besuDeployer.SetAuditService(auditService)
	}
	return &Handler{
		auditService:     auditService,
		logger:           logger,
		besuDeployer:     besuDeployer,
		validate:         validator.New(),
		nodeService:      nodeService,
		chaincodeService: chaincodeService,
	}
}

// RegisterRoutes registers the smart contract deploy routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/sc/fabric", func(r chi.Router) {
		r.Post("/deploy", response.Middleware(h.DeployFabricChaincode))
		r.Post("/peer/{peerId}/chaincode/install", response.Middleware(h.InstallFabricChaincode))
		r.Post("/peer/{peerId}/chaincode/approve", response.Middleware(h.ApproveFabricChaincode))
		r.Post("/peer/{peerId}/chaincode/commit", response.Middleware(h.CommitFabricChaincode))
		r.Get("/chaincodes", response.Middleware(h.ListFabricChaincodes))
		r.Get("/chaincodes/{id}", response.Middleware(h.GetFabricChaincodeDetailByID))

		r.Post("/chaincodes", response.Middleware(h.CreateChaincode))
		r.Post("/chaincodes/{chaincodeId}/definitions", response.Middleware(h.CreateChaincodeDefinition))
		r.Get("/chaincodes/{chaincodeId}/definitions", response.Middleware(h.ListChaincodeDefinitions))
		r.Get("/chaincodes/{chaincodeId}/definitions/{definitionId}", response.Middleware(h.GetChaincodeDefinitionDetailByID))
	})

	r.Route("/sc/fabric/definitions", func(r chi.Router) {
		r.Post("/{definitionId}/install", response.Middleware(h.InstallChaincodeByDefinition))
		r.Post("/{definitionId}/approve", response.Middleware(h.ApproveChaincodeByDefinition))
		r.Post("/{definitionId}/commit", response.Middleware(h.CommitChaincodeByDefinition))
		r.Post("/{definitionId}/deploy", response.Middleware(h.DeployChaincodeByDefinition))
		r.Put("/{definitionId}", response.Middleware(h.UpdateChaincodeDefinition))
		r.Get("/{definitionId}/timeline", response.Middleware(h.GetChaincodeDefinitionTimeline))
		r.Delete("/{definitionId}", response.Middleware(h.DeleteChaincodeDefinition))
	})

	r.Route("/sc/besu", func(r chi.Router) {
		r.Post("/deploy", response.Middleware(h.DeployBesuContract))
	})
}

// FabricDeployRequest represents the request body for Fabric chaincode deployment
type FabricDeployRequest FabricChaincodeDeployParams

// FabricDeployResponse represents the response for Fabric chaincode deployment
type FabricDeployResponse struct {
	Status  string           `json:"status"`
	Message string           `json:"message"`
	Result  DeploymentResult `json:"result"`
}

// FabricInstallRequest represents the request body for Fabric chaincode install
// (separate from service struct for HTTP layer)
type FabricInstallRequest struct {
	PackageBytes []byte `json:"package_bytes" validate:"required"`
	Label        string `json:"label" validate:"required"`
}

type FabricInstallResponse struct {
	Status  string           `json:"status"`
	Message string           `json:"message"`
	Result  DeploymentResult `json:"result"`
}

// FabricApproveRequest represents the request body for Fabric chaincode approve
type FabricApproveRequest struct {
	Name              string
	Version           string
	Sequence          int64
	PackageID         string
	ChannelID         string
	EndorsementPolicy string
	InitRequired      bool
}

type FabricApproveResponse struct {
	Status  string           `json:"status"`
	Message string           `json:"message"`
	Result  DeploymentResult `json:"result"`
}

// FabricCommitRequest represents the request body for Fabric chaincode commit
type FabricCommitRequest struct {
	Name              string
	Version           string
	Sequence          int64
	ChannelID         string
	EndorsementPolicy string
	InitRequired      bool
}

type FabricCommitResponse struct {
	Status  string           `json:"status"`
	Message string           `json:"message"`
	Result  DeploymentResult `json:"result"`
}

// BesuDeployRequest represents the request body for Besu contract deployment
type BesuDeployRequest EVMParams

// BesuDeployResponse represents the response for Besu contract deployment
type BesuDeployResponse struct {
	Status  string           `json:"status"`
	Message string           `json:"message"`
	Result  DeploymentResult `json:"result"`
}

// FabricChaincodeDockerDeployRequest represents the request body for Fabric chaincode Docker deployment
// (separate from service struct for HTTP layer)
type FabricChaincodeDockerDeployRequest struct {
	Name          string `json:"name" validate:"required"`
	Slug          string `json:"slug"` // optional, for updates
	DockerImage   string `json:"docker_image" validate:"required"`
	PackageID     string `json:"package_id" validate:"required"`
	HostPort      int64  `json:"host_port"`      // optional, if 0 a free port is chosen
	ContainerPort int64  `json:"container_port"` // optional, defaults to 7052
}

type FabricChaincodeDockerDeployResponse struct {
	Status  string           `json:"status"`
	Message string           `json:"message"`
	Slug    string           `json:"slug"`
	Result  DeploymentResult `json:"result"`
}

type ChaincodeResponse struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	NetworkID       int64  `json:"network_id"`
	NetworkName     string `json:"network_name"`
	NetworkPlatform string `json:"network_platform"`
	CreatedAt       string `json:"created_at"`
}

type ListChaincodesResponse struct {
	Chaincodes []ChaincodeResponse `json:"chaincodes"`
}

// Mapping function from service-layer Chaincode to HTTP response struct
func mapChaincodeToResponse(cc *Chaincode) ChaincodeResponse {
	return ChaincodeResponse{
		ID:              cc.ID,
		Name:            cc.Name,
		NetworkID:       cc.NetworkID,
		NetworkName:     cc.NetworkName,
		NetworkPlatform: cc.NetworkPlatform,
		CreatedAt:       cc.CreatedAt,
	}
}

// DeployFabricChaincode handles Fabric chaincode deployment requests
// @Summary Deploy Fabric chaincode
// @Description Deploy a chaincode to a Fabric network (install, approve, commit)
// @Tags SmartContracts
// @Accept json
// @Produce json
// @Param request body FabricDeployRequest true "Fabric chaincode deployment parameters"
// @Success 200 {object} FabricDeployResponse
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/deploy [post]
func (h *Handler) DeployFabricChaincode(w http.ResponseWriter, r *http.Request) error {
	var req FabricDeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Invalid Fabric request body", "error", err)
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_REQUEST_BODY",
		})
	}

	if err := h.validate.Struct(req); err != nil {
		validationErrors := make(map[string]string)
		for _, err := range err.(validator.ValidationErrors) {
			validationErrors[err.Field()] = err.Tag()
		}
		return errors.NewValidationError("validation failed", map[string]interface{}{
			"detail": "Request validation failed",
			"code":   "VALIDATION_ERROR",
			"errors": validationErrors,
		})
	}

	reporter := NewInMemoryDeploymentStatusReporter()
	result, err := DeployChaincode(FabricChaincodeDeployParams(req), reporter)
	if err != nil {
		h.logger.Error("Fabric chaincode deployment failed", "error", err)
		return errors.NewInternalError("deployment failed", err, nil)
	}

	resp := FabricDeployResponse{
		Status:  "success",
		Message: "Chaincode deployed successfully",
		Result:  result,
	}
	return response.WriteJSON(w, http.StatusOK, resp)
}

// InstallFabricChaincode handles Fabric chaincode install requests
// @Summary Install Fabric chaincode
// @Description Install a chaincode package on a Fabric peer
// @Tags SmartContracts
// @Accept json
// @Produce json
// @Param peerId path string true "Peer ID"
// @Param request body FabricInstallRequest true "Fabric chaincode install parameters"
// @Success 200 {object} FabricInstallResponse
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/peer/{peerId}/chaincode/install [post]
func (h *Handler) InstallFabricChaincode(w http.ResponseWriter, r *http.Request) error {
	peerId := chi.URLParam(r, "peerId")
	peerIdInt, err := strconv.ParseInt(peerId, 10, 64)
	if err != nil {
		h.logger.Error("Invalid peer ID", "peerId", peerId)
		return errors.NewValidationError("invalid peer ID", map[string]interface{}{
			"detail": "Invalid peer ID",
			"code":   "INVALID_PEER_ID",
		})
	}
	var req FabricInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Invalid Fabric install request body", "error", err)
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_REQUEST_BODY",
		})
	}
	if err := h.validate.Struct(req); err != nil {
		validationErrors := make(map[string]string)
		for _, err := range err.(validator.ValidationErrors) {
			validationErrors[err.Field()] = err.Tag()
		}
		return errors.NewValidationError("validation failed", map[string]interface{}{
			"detail": "Request validation failed",
			"code":   "VALIDATION_ERROR",
			"errors": validationErrors,
		})
	}
	peerService, peerConn, err := h.nodeService.GetFabricPeerService(r.Context(), peerIdInt)
	if err != nil {
		h.logger.Error("Node not found", "peerId", peerId)
		return errors.NewValidationError("node not found", map[string]interface{}{
			"detail": "Node not found",
			"code":   "NODE_NOT_FOUND",
		})
	}
	defer peerConn.Close()

	params := FabricChaincodeInstallParams{
		Peer:         peerService,
		PackageBytes: req.PackageBytes, Label: req.Label,
	}
	reporter := NewInMemoryDeploymentStatusReporter()
	result, err := InstallChaincode(params, reporter)
	if err != nil {
		h.logger.Error("Fabric chaincode install failed", "error", err)
		return errors.NewInternalError("install failed", err, nil)
	}
	resp := FabricInstallResponse{
		Status:  "success",
		Message: "Chaincode installed successfully",
		Result:  result,
	}
	return response.WriteJSON(w, http.StatusOK, resp)
}

// ApproveFabricChaincode handles Fabric chaincode approve requests
// @Summary Approve Fabric chaincode
// @Description Approve a chaincode definition for an organization
// @Tags SmartContracts
// @Accept json
// @Produce json
// @Param peerId path string true "Peer ID"
// @Param request body FabricApproveRequest true "Fabric chaincode approve parameters"
// @Success 200 {object} FabricApproveResponse
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/peer/{peerId}/chaincode/approve [post]
func (h *Handler) ApproveFabricChaincode(w http.ResponseWriter, r *http.Request) error {
	peerId := chi.URLParam(r, "peerId")
	var req FabricApproveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Invalid Fabric approve request body", "error", err)
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_REQUEST_BODY",
		})
	}
	if err := h.validate.Struct(req); err != nil {
		validationErrors := make(map[string]string)
		for _, err := range err.(validator.ValidationErrors) {
			validationErrors[err.Field()] = err.Tag()
		}
		return errors.NewValidationError("validation failed", map[string]interface{}{
			"detail": "Request validation failed",
			"code":   "VALIDATION_ERROR",
			"errors": validationErrors,
		})
	}
	peerIdInt, err := strconv.ParseInt(peerId, 10, 64)
	if err != nil {
		h.logger.Error("Invalid peer ID", "peerId", peerId)
		return errors.NewValidationError("invalid peer ID", map[string]interface{}{
			"detail": "Invalid peer ID",
			"code":   "INVALID_PEER_ID",
		})
	}
	peerGateway, peerConn, err := h.nodeService.GetFabricPeerGateway(r.Context(), peerIdInt)
	if err != nil {
		h.logger.Error("Node not found", "peerId", peerId)
		return errors.NewValidationError("node not found", map[string]interface{}{
			"detail": "Node not found",
			"code":   "NODE_NOT_FOUND",
		})
	}
	defer peerConn.Close()

	params := FabricChaincodeApproveParams{
		Gateway:           peerGateway,
		Name:              req.Name,
		Version:           req.Version,
		Sequence:          req.Sequence,
		PackageID:         req.PackageID,
		ChannelID:         req.ChannelID,
		EndorsementPolicy: req.EndorsementPolicy,
		InitRequired:      req.InitRequired,
	}
	reporter := NewInMemoryDeploymentStatusReporter()
	result, err := ApproveChaincode(params, reporter)
	if err != nil {
		h.logger.Error("Fabric chaincode approve failed", "error", err)
		return errors.NewInternalError("approve failed", err, nil)
	}
	resp := FabricApproveResponse{
		Status:  "success",
		Message: "Chaincode approved successfully",
		Result:  result,
	}
	return response.WriteJSON(w, http.StatusOK, resp)
}

// CommitFabricChaincode handles Fabric chaincode commit requests
// @Summary Commit Fabric chaincode
// @Description Commit a chaincode definition to the channel
// @Tags SmartContracts
// @Accept json
// @Produce json
// @Param peerId path string true "Peer ID"
// @Param request body FabricCommitRequest true "Fabric chaincode commit parameters"
// @Success 200 {object} FabricCommitResponse
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/peer/{peerId}/chaincode/commit [post]
func (h *Handler) CommitFabricChaincode(w http.ResponseWriter, r *http.Request) error {
	peerId := chi.URLParam(r, "peerId")
	var req FabricCommitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Invalid Fabric commit request body", "error", err)
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_REQUEST_BODY",
		})
	}
	if err := h.validate.Struct(req); err != nil {
		validationErrors := make(map[string]string)
		for _, err := range err.(validator.ValidationErrors) {
			validationErrors[err.Field()] = err.Tag()
		}
		return errors.NewValidationError("validation failed", map[string]interface{}{
			"detail": "Request validation failed",
			"code":   "VALIDATION_ERROR",
			"errors": validationErrors,
		})
	}
	peerIdInt, err := strconv.ParseInt(peerId, 10, 64)
	if err != nil {
		h.logger.Error("Invalid peer ID", "peerId", peerId)
		return errors.NewValidationError("invalid peer ID", map[string]interface{}{
			"detail": "Invalid peer ID",
			"code":   "INVALID_PEER_ID",
		})
	}
	peerGateway, peerConn, err := h.nodeService.GetFabricPeerGateway(r.Context(), peerIdInt)
	if err != nil {
		h.logger.Error("Node not found", "peerId", peerId)
		return errors.NewValidationError("node not found", map[string]interface{}{
			"detail": "Node not found",
			"code":   "NODE_NOT_FOUND",
		})
	}
	defer peerConn.Close()
	params := FabricChaincodeCommitParams{
		Gateway:           peerGateway,
		Name:              req.Name,
		Version:           req.Version,
		Sequence:          req.Sequence,
		ChannelID:         req.ChannelID,
		EndorsementPolicy: req.EndorsementPolicy,
		InitRequired:      req.InitRequired,
	}
	reporter := NewInMemoryDeploymentStatusReporter()
	result, err := CommitChaincode(params, reporter)
	if err != nil {
		h.logger.Error("Fabric chaincode commit failed", "error", err)
		return errors.NewInternalError("commit failed", err, nil)
	}
	resp := FabricCommitResponse{
		Status:  "success",
		Message: "Chaincode committed successfully",
		Result:  result,
	}
	return response.WriteJSON(w, http.StatusOK, resp)
}

// DeployBesuContract handles Besu contract deployment requests
// @Summary Deploy Besu smart contract
// @Description Deploy a smart contract to a Besu (EVM) network
// @Tags SmartContracts
// @Accept json
// @Produce json
// @Param request body BesuDeployRequest true "Besu contract deployment parameters"
// @Success 200 {object} BesuDeployResponse
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/besu/deploy [post]
func (h *Handler) DeployBesuContract(w http.ResponseWriter, r *http.Request) error {
	var req BesuDeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Invalid Besu request body", "error", err)
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_REQUEST_BODY",
		})
	}

	if h.besuDeployer == nil {
		return errors.NewInternalError("Besu deployer not configured", nil, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		validationErrors := make(map[string]string)
		for _, err := range err.(validator.ValidationErrors) {
			validationErrors[err.Field()] = err.Tag()
		}
		return errors.NewValidationError("validation failed", map[string]interface{}{
			"detail": "Request validation failed",
			"code":   "VALIDATION_ERROR",
			"errors": validationErrors,
		})
	}

	reporter := NewInMemoryDeploymentStatusReporter()
	result, err := h.besuDeployer.DeployEVMContract(EVMParams(req), reporter)
	if err != nil {
		h.logger.Error("Besu contract deployment failed", "error", err)
		return errors.NewInternalError("deployment failed", err, nil)
	}

	resp := BesuDeployResponse{
		Status:  "success",
		Message: "Contract deployed successfully",
		Result:  result,
	}
	return response.WriteJSON(w, http.StatusOK, resp)
}

// ListFabricChaincodes lists all deployed Fabric chaincodes
// @Summary List deployed Fabric chaincodes
// @Description List all Fabric chaincodes deployed via Docker
// @Tags SmartContracts
// @Accept json
// @Produce json
// @Success 200 {object} ListChaincodesResponse
// @Failure 500 {object} response.Response
// @Router /sc/fabric/chaincodes [get]
func (h *Handler) ListFabricChaincodes(w http.ResponseWriter, r *http.Request) error {
	chaincodes, err := h.chaincodeService.ListChaincodes(r.Context())
	if err != nil {
		h.logger.Error("Failed to list fabric chaincodes", "error", err)
		return errors.NewInternalError("failed to list chaincodes", err, nil)
	}
	resp := ListChaincodesResponse{Chaincodes: make([]ChaincodeResponse, 0, len(chaincodes))}
	for _, cc := range chaincodes {
		if cc != nil {
			resp.Chaincodes = append(resp.Chaincodes, mapChaincodeToResponse(cc))
		}
	}
	return response.WriteJSON(w, http.StatusOK, resp)
}

// generateUniqueSlug creates a slug from the name and a random suffix if needed
func generateUniqueSlug(name string) string {
	base := strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(name, "-"))
	return fmt.Sprintf("%s-%s", strings.Trim(base, "-"), uuid.New().String()[:8])
}

// @Summary Get Fabric chaincode details by ID
// @Description Get a specific Fabric chaincode and its Docker/runtime info by ID
// @Tags SmartContracts
// @Accept json
// @Produce json
// @Param id path int true "Chaincode ID"
// @Success 200 {object} FabricChaincodeDetail
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/chaincodes/{id} [get]
func (h *Handler) GetFabricChaincodeDetailByID(w http.ResponseWriter, r *http.Request) error {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid chaincode ID", "id", idStr)
		return errors.NewValidationError("invalid chaincode ID", map[string]interface{}{"detail": "Invalid chaincode ID"})
	}
	detail, err := h.chaincodeService.GetChaincodeDetail(r.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get chaincode detail", "error", err)
		return errors.NewInternalError("failed to get chaincode detail", err, nil)
	}
	if detail == nil {
		return response.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "Chaincode not found"})
	}
	resp := FabricChaincodeDetail{
		Chaincode:   detail.Chaincode,
		Definitions: detail.Definitions,
		DockerInfo:  detail.DockerInfo,
	}
	return response.WriteJSON(w, http.StatusOK, resp)
}

// --- HTTP structs for new endpoints ---

// swagger:parameters createChaincode
type CreateChaincodeRequest struct {
	// Name of the chaincode
	// required: true
	Name string `json:"name"`
	// Network ID
	// required: true
	NetworkID int64 `json:"network_id"`
}

type CreateChaincodeResponse struct {
	Chaincode ChaincodeResponse `json:"chaincode"`
}

// swagger:parameters createChaincodeDefinition
type CreateChaincodeDefinitionRequest struct {
	// Chaincode ID
	// required: true
	ChaincodeID int64 `json:"chaincode_id"`
	// Version
	// required: true
	Version string `json:"version"`
	// Sequence
	// required: true
	Sequence int64 `json:"sequence"`
	// Docker image
	// required: true
	DockerImage string `json:"docker_image"`
	// Endorsement policy
	EndorsementPolicy string `json:"endorsement_policy"`
	// Chaincode address
	ChaincodeAddress string `json:"chaincode_address"`
}

type CreateChaincodeDefinitionResponse struct {
	Definition ChaincodeDefinitionResponse `json:"definition"`
}

type ChaincodeDefinitionResponse struct {
	ID                int64  `json:"id"`
	ChaincodeID       int64  `json:"chaincode_id"`
	Version           string `json:"version"`
	Sequence          int64  `json:"sequence"`
	DockerImage       string `json:"docker_image"`
	EndorsementPolicy string `json:"endorsement_policy"`
	ChaincodeAddress  string `json:"chaincode_address"`
	CreatedAt         string `json:"created_at"`
}

type ListChaincodeDefinitionsResponse struct {
	Definitions []ChaincodeDefinitionResponse `json:"definitions"`
}

// --- Mapping functions ---
func mapChaincodeDefinitionToResponse(def *ChaincodeDefinition) ChaincodeDefinitionResponse {
	return ChaincodeDefinitionResponse{
		ID:                def.ID,
		ChaincodeID:       def.ChaincodeID,
		Version:           def.Version,
		Sequence:          def.Sequence,
		DockerImage:       def.DockerImage,
		EndorsementPolicy: def.EndorsementPolicy,
		ChaincodeAddress:  def.ChaincodeAddress,
		CreatedAt:         def.CreatedAt,
	}
}

// --- Handler methods for new endpoints ---

// @Summary Create a chaincode
// @Description Create a new chaincode
// @Tags Chaincode
// @Accept json
// @Produce json
// @Param request body CreateChaincodeRequest true "Chaincode info"
// @Success 200 {object} CreateChaincodeResponse
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/chaincodes [post]
func (h *Handler) CreateChaincode(w http.ResponseWriter, r *http.Request) error {
	var req CreateChaincodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Invalid create chaincode request body", "error", err)
		return errors.NewValidationError("invalid request body", map[string]interface{}{"detail": err.Error()})
	}
	cc, err := h.chaincodeService.CreateChaincode(r.Context(), req.Name, req.NetworkID)
	if err != nil {
		h.logger.Error("Failed to create chaincode", "error", err)
		return errors.NewInternalError("failed to create chaincode", err, nil)
	}
	resp := CreateChaincodeResponse{Chaincode: mapChaincodeToResponse(cc)}
	return response.WriteJSON(w, http.StatusOK, resp)
}

// @Summary Create a chaincode definition
// @Description Create a new chaincode definition for a chaincode
// @Tags Chaincode
// @Accept json
// @Produce json
// @Param request body CreateChaincodeDefinitionRequest true "Chaincode definition info"
// @Success 200 {object} CreateChaincodeDefinitionResponse
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/chaincodes/{chaincodeId}/definitions [post]
func (h *Handler) CreateChaincodeDefinition(w http.ResponseWriter, r *http.Request) error {
	var req CreateChaincodeDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Invalid create chaincode definition request body", "error", err)
		return errors.NewValidationError("invalid request body", map[string]interface{}{"detail": err.Error()})
	}
	def, err := h.chaincodeService.CreateChaincodeDefinition(r.Context(), req.ChaincodeID, req.Version, req.Sequence, req.DockerImage, req.EndorsementPolicy, req.ChaincodeAddress)
	if err != nil {
		h.logger.Error("Failed to create chaincode definition", "error", err)
		return errors.NewInternalError("failed to create chaincode definition", err, nil)
	}
	resp := CreateChaincodeDefinitionResponse{Definition: mapChaincodeDefinitionToResponse(def)}
	return response.WriteJSON(w, http.StatusOK, resp)
}

// @Summary List chaincode definitions for a chaincode
// @Description List all definitions for a given chaincode
// @Tags Chaincode
// @Accept json
// @Produce json
// @Param chaincodeId path int true "Chaincode ID"
// @Success 200 {object} ListChaincodeDefinitionsResponse
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/chaincodes/{chaincodeId}/definitions [get]
func (h *Handler) ListChaincodeDefinitions(w http.ResponseWriter, r *http.Request) error {
	chaincodeIdStr := chi.URLParam(r, "chaincodeId")
	chaincodeId, err := strconv.ParseInt(chaincodeIdStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid chaincode ID", "chaincodeId", chaincodeIdStr)
		return errors.NewValidationError("invalid chaincode ID", map[string]interface{}{"detail": "Invalid chaincode ID"})
	}
	defs, err := h.chaincodeService.ListChaincodeDefinitions(r.Context(), chaincodeId)
	if err != nil {
		h.logger.Error("Failed to list chaincode definitions", "error", err)
		return errors.NewInternalError("failed to list chaincode definitions", err, nil)
	}
	resp := ListChaincodeDefinitionsResponse{Definitions: make([]ChaincodeDefinitionResponse, 0, len(defs))}
	for _, def := range defs {
		resp.Definitions = append(resp.Definitions, mapChaincodeDefinitionToResponse(def))
	}
	return response.WriteJSON(w, http.StatusOK, resp)
}

// Request struct for installing chaincode by definition
// swagger:parameters installChaincodeByDefinition
type InstallChaincodeByDefinitionRequest struct {
	// Peer IDs to install the chaincode on
	// required: true
	PeerIDs []int64 `json:"peer_ids"`
}

// @Summary Install chaincode based on chaincode definition
// @Description Install chaincode on peers for a given definition
// @Tags Chaincode
// @Accept json
// @Produce json
// @Param definitionId path int true "Chaincode Definition ID"
// @Param request body InstallChaincodeByDefinitionRequest true "Peer IDs to install on"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/definitions/{definitionId}/install [post]
func (h *Handler) InstallChaincodeByDefinition(w http.ResponseWriter, r *http.Request) error {
	definitionIdStr := chi.URLParam(r, "definitionId")
	definitionId, err := strconv.ParseInt(definitionIdStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid definition ID", map[string]interface{}{"detail": "Invalid definition ID"})
	}
	var req InstallChaincodeByDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Invalid install chaincode request body", "error", err)
		return errors.NewValidationError("invalid request body", map[string]interface{}{"detail": err.Error()})
	}
	if len(req.PeerIDs) == 0 {
		return errors.NewValidationError("peer_ids required", map[string]interface{}{"detail": "peer_ids must not be empty"})
	}
	// Call service layer to install chaincode on the given peers
	err = h.chaincodeService.InstallChaincodeByDefinition(r.Context(), definitionId, req.PeerIDs)
	if err != nil {
		h.logger.Error("Failed to install chaincode by definition", "error", err)
		return errors.NewInternalError("failed to install chaincode by definition", err, nil)
	}
	return response.WriteJSON(w, http.StatusOK, map[string]string{"status": "install success", "definitionId": definitionIdStr})
}

// Request struct for approving chaincode by definition
// swagger:parameters approveChaincodeByDefinition
type ApproveChaincodeByDefinitionRequest struct {
	// Peer ID to use for approval
	// required: true
	PeerID int64 `json:"peer_id"`
}

// Request struct for committing chaincode by definition
// swagger:parameters commitChaincodeByDefinition
type CommitChaincodeByDefinitionRequest struct {
	// Peer ID to use for commit
	// required: true
	PeerID int64 `json:"peer_id"`
}

// @Summary Approve chaincode based on chaincode definition
// @Description Approve chaincode for a given definition
// @Tags Chaincode
// @Accept json
// @Produce json
// @Param definitionId path int true "Chaincode Definition ID"
// @Param request body ApproveChaincodeByDefinitionRequest true "Peer ID to use for approval"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/definitions/{definitionId}/approve [post]
func (h *Handler) ApproveChaincodeByDefinition(w http.ResponseWriter, r *http.Request) error {
	definitionIdStr := chi.URLParam(r, "definitionId")
	definitionId, err := strconv.ParseInt(definitionIdStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid definition ID", map[string]interface{}{"detail": "Invalid definition ID"})
	}
	var req ApproveChaincodeByDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Invalid approve chaincode request body", "error", err)
		return errors.NewValidationError("invalid request body", map[string]interface{}{"detail": err.Error()})
	}
	if req.PeerID == 0 {
		return errors.NewValidationError("peer_id required", map[string]interface{}{"detail": "peer_id must not be zero"})
	}
	// Call service layer to approve chaincode using the given peer
	err = h.chaincodeService.ApproveChaincodeByDefinition(r.Context(), definitionId, req.PeerID)
	if err != nil {
		h.logger.Error("Failed to approve chaincode by definition", "error", err)
		return errors.NewInternalError("failed to approve chaincode by definition", err, nil)
	}
	return response.WriteJSON(w, http.StatusOK, map[string]string{"status": "approve success", "definitionId": definitionIdStr})
}

// @Summary Commit chaincode based on chaincode definition
// @Description Commit chaincode for a given definition
// @Tags Chaincode
// @Accept json
// @Produce json
// @Param definitionId path int true "Chaincode Definition ID"
// @Param request body CommitChaincodeByDefinitionRequest true "Peer ID to use for commit"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/definitions/{definitionId}/commit [post]
func (h *Handler) CommitChaincodeByDefinition(w http.ResponseWriter, r *http.Request) error {
	definitionIdStr := chi.URLParam(r, "definitionId")
	definitionId, err := strconv.ParseInt(definitionIdStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid definition ID", map[string]interface{}{"detail": "Invalid definition ID"})
	}
	var req CommitChaincodeByDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Invalid commit chaincode request body", "error", err)
		return errors.NewValidationError("invalid request body", map[string]interface{}{"detail": err.Error()})
	}
	if req.PeerID == 0 {
		return errors.NewValidationError("peer_id required", map[string]interface{}{"detail": "peer_id must not be zero"})
	}
	// Call service layer to commit chaincode using the given peer
	err = h.chaincodeService.CommitChaincodeByDefinition(r.Context(), definitionId, req.PeerID)
	if err != nil {
		h.logger.Error("Failed to commit chaincode by definition", "error", err)
		return errors.NewInternalError("failed to commit chaincode by definition", err, nil)
	}
	return response.WriteJSON(w, http.StatusOK, map[string]string{"status": "commit success", "definitionId": definitionIdStr})
}

// Request struct for deploying chaincode by definition using Docker image
// swagger:parameters deployChaincodeByDefinition
type DeployChaincodeByDefinitionRequest struct {
}

// @Summary Deploy chaincode based on chaincode definition (Docker)
// @Description Deploy chaincode for a given definition using Docker image
// @Tags Chaincode
// @Accept json
// @Produce json
// @Param definitionId path int true "Chaincode Definition ID"
// @Param request body DeployChaincodeByDefinitionRequest true "Docker deploy params"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/definitions/{definitionId}/deploy [post]
func (h *Handler) DeployChaincodeByDefinition(w http.ResponseWriter, r *http.Request) error {
	definitionIdStr := chi.URLParam(r, "definitionId")
	definitionId, err := strconv.ParseInt(definitionIdStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid definition ID", map[string]interface{}{"detail": "Invalid definition ID"})
	}
	var req DeployChaincodeByDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Invalid deploy chaincode request body", "error", err)
		return errors.NewValidationError("invalid request body", map[string]interface{}{"detail": err.Error()})
	}
	err = h.chaincodeService.DeployChaincodeByDefinition(r.Context(), definitionId)
	if err != nil {
		h.logger.Error("Failed to deploy chaincode by definition", "error", err)
		return errors.NewInternalError("failed to deploy chaincode by definition", err, nil)
	}
	return response.WriteJSON(w, http.StatusOK, map[string]string{"status": "deploy success", "definitionId": definitionIdStr})
}

// swagger:parameters updateChaincodeDefinition
// UpdateChaincodeDefinitionRequest is the request body for updating a chaincode definition
type UpdateChaincodeDefinitionRequest struct {
	// Version
	// required: true
	Version string `json:"version"`
	// Sequence
	// required: true
	Sequence int64 `json:"sequence"`
	// Docker image
	// required: true
	DockerImage string `json:"docker_image"`
	// Endorsement policy
	EndorsementPolicy string `json:"endorsement_policy"`
	// Chaincode address
	ChaincodeAddress string `json:"chaincode_address"`
}

// @Summary Update a chaincode definition
// @Description Update an existing chaincode definition by ID
// @Tags Chaincode
// @Accept json
// @Produce json
// @Param definitionId path int true "Chaincode Definition ID"
// @Param request body UpdateChaincodeDefinitionRequest true "Chaincode definition update info"
// @Success 200 {object} ChaincodeDefinitionResponse
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/definitions/{definitionId} [put]
func (h *Handler) UpdateChaincodeDefinition(w http.ResponseWriter, r *http.Request) error {
	definitionIdStr := chi.URLParam(r, "definitionId")
	definitionId, err := strconv.ParseInt(definitionIdStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid definition ID", "definitionId", definitionIdStr)
		return errors.NewValidationError("invalid definition ID", map[string]interface{}{"detail": "Invalid definition ID"})
	}
	var req UpdateChaincodeDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Invalid update chaincode definition request body", "error", err)
		return errors.NewValidationError("invalid request body", map[string]interface{}{"detail": err.Error()})
	}
	def, err := h.chaincodeService.UpdateChaincodeDefinition(r.Context(), definitionId, req.Version, req.Sequence, req.DockerImage, req.EndorsementPolicy, req.ChaincodeAddress)
	if err != nil {
		h.logger.Error("Failed to update chaincode definition", "error", err)
		return errors.NewInternalError("failed to update chaincode definition", err, nil)
	}
	return response.WriteJSON(w, http.StatusOK, mapChaincodeDefinitionToResponse(def))
}

// @Summary Get timeline of events for a chaincode definition
// @Description Get the timeline of install/approve/commit/deploy events for a chaincode definition
// @Tags Chaincode
// @Accept json
// @Produce json
// @Param definitionId path int true "Chaincode Definition ID"
// @Success 200 {array} ChaincodeDefinitionEvent
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/definitions/{definitionId}/timeline [get]
func (h *Handler) GetChaincodeDefinitionTimeline(w http.ResponseWriter, r *http.Request) error {
	definitionIdStr := chi.URLParam(r, "definitionId")
	definitionId, err := strconv.ParseInt(definitionIdStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid definition ID", "definitionId", definitionIdStr)
		return errors.NewValidationError("invalid definition ID", map[string]interface{}{"detail": "Invalid definition ID"})
	}
	events, err := h.chaincodeService.ListChaincodeDefinitionEvents(r.Context(), definitionId)
	if err != nil {
		h.logger.Error("Failed to get chaincode definition timeline", "error", err)
		return errors.NewInternalError("failed to get chaincode definition timeline", err, nil)
	}
	return response.WriteJSON(w, http.StatusOK, events)
}

// @Summary Delete a chaincode definition
// @Description Delete a chaincode definition by ID
// @Tags Chaincode
// @Accept json
// @Produce json
// @Param definitionId path int true "Chaincode Definition ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/definitions/{definitionId} [delete]
func (h *Handler) DeleteChaincodeDefinition(w http.ResponseWriter, r *http.Request) error {
	definitionIdStr := chi.URLParam(r, "definitionId")
	definitionId, err := strconv.ParseInt(definitionIdStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid definition ID", "definitionId", definitionIdStr)
		return errors.NewValidationError("invalid definition ID", map[string]interface{}{"detail": "Invalid definition ID"})
	}
	err = h.chaincodeService.DeleteChaincodeDefinition(r.Context(), definitionId)
	if err != nil {
		h.logger.Error("Failed to delete chaincode definition", "error", err)
		return errors.NewInternalError("failed to delete chaincode definition", err, nil)
	}
	return response.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted", "definitionId": definitionIdStr})
}

// ChaincodeDefinitionDetailResponse represents the response for getting chaincode definition details
type ChaincodeDefinitionDetailResponse struct {
	Definition ChaincodeDefinitionResponse `json:"definition"`
	Events     []*ChaincodeDefinitionEvent `json:"events"`
}

// @Summary Get chaincode definition details by ID
// @Description Get detailed information about a specific chaincode definition including its events
// @Tags Chaincode
// @Accept json
// @Produce json
// @Param chaincodeId path int true "Chaincode ID"
// @Param definitionId path int true "Chaincode Definition ID"
// @Success 200 {object} ChaincodeDefinitionDetailResponse
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sc/fabric/chaincodes/{chaincodeId}/definitions/{definitionId} [get]
func (h *Handler) GetChaincodeDefinitionDetailByID(w http.ResponseWriter, r *http.Request) error {
	chaincodeIdStr := chi.URLParam(r, "chaincodeId")
	definitionIdStr := chi.URLParam(r, "definitionId")

	chaincodeId, err := strconv.ParseInt(chaincodeIdStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid chaincode ID", "chaincodeId", chaincodeIdStr)
		return errors.NewValidationError("invalid chaincode ID", map[string]interface{}{"detail": "Invalid chaincode ID"})
	}

	definitionId, err := strconv.ParseInt(definitionIdStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid definition ID", "definitionId", definitionIdStr)
		return errors.NewValidationError("invalid definition ID", map[string]interface{}{"detail": "Invalid definition ID"})
	}

	// Get the definition details
	def, err := h.chaincodeService.GetChaincodeDefinition(r.Context(), definitionId)
	if err != nil {
		h.logger.Error("Failed to get chaincode definition", "error", err)
		return errors.NewInternalError("failed to get chaincode definition", err, nil)
	}
	if def == nil {
		return response.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "Chaincode definition not found"})
	}

	// Verify the definition belongs to the specified chaincode
	if def.ChaincodeID != chaincodeId {
		return response.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "Chaincode definition not found for the specified chaincode"})
	}

	// Get the events for this definition
	events, err := h.chaincodeService.ListChaincodeDefinitionEvents(r.Context(), definitionId)
	if err != nil {
		h.logger.Error("Failed to get chaincode definition events", "error", err)
		return errors.NewInternalError("failed to get chaincode definition events", err, nil)
	}

	resp := ChaincodeDefinitionDetailResponse{
		Definition: mapChaincodeDefinitionToResponse(def),
		Events:     events,
	}

	return response.WriteJSON(w, http.StatusOK, resp)
}
