package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/errors"
	"github.com/chainlaunch/chainlaunch/pkg/http/response"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	"github.com/go-chi/chi/v5"

	"github.com/golang/protobuf/proto"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	msp "github.com/hyperledger/fabric-protos-go-apiv2/msp"
)

type NodeHandler struct {
	service *service.NodeService
	logger  *logger.Logger
}

func NewNodeHandler(service *service.NodeService, logger *logger.Logger) *NodeHandler {
	return &NodeHandler{
		service: service,
		logger:  logger,
	}
}

// Add these types for the response structures
type NodeEventResponse struct {
	ID        int64       `json:"id"`
	NodeID    int64       `json:"node_id"`
	Type      string      `json:"type"`
	Data      interface{} `json:"data,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}

type PaginatedNodeEventsResponse struct {
	Items []NodeEventResponse `json:"items"`
	Total int64               `json:"total"`
	Page  int                 `json:"page"`
}

// RegisterRoutes registers the node routes
func (h *NodeHandler) RegisterRoutes(r chi.Router) {
	r.Route("/nodes", func(r chi.Router) {
		r.Post("/", response.Middleware(h.CreateNode))
		r.Get("/", response.Middleware(h.ListNodes))
		r.Get("/platform/{platform}", response.Middleware(h.ListNodesByPlatform))
		r.Get("/defaults/fabric-peer", response.Middleware(h.GetFabricPeerDefaults))
		r.Get("/defaults/fabric-orderer", response.Middleware(h.GetFabricOrdererDefaults))
		r.Get("/defaults/fabric", response.Middleware(h.GetFabricNodesDefaults))
		r.Get("/defaults/besu-node", response.Middleware(h.GetBesuNodeDefaults))
		r.Get("/readiness/besu", response.Middleware(h.CheckBesuReadiness))
		r.Get("/{id}", response.Middleware(h.GetNode))
		r.Post("/{id}/start", response.Middleware(h.StartNode))
		r.Post("/{id}/stop", response.Middleware(h.StopNode))
		r.Post("/{id}/restart", response.Middleware(h.RestartNode))
		r.Delete("/{id}", response.Middleware(h.DeleteNode))
		r.Get("/{id}/logs", h.TailLogs)
		r.Get("/{id}/events", response.Middleware(h.GetNodeEvents))
		r.Get("/{id}/channels", response.Middleware(h.GetNodeChannels))
		r.Get("/{id}/channels/{channelID}/chaincodes", response.Middleware(h.GetNodeChaincodes))
		r.Post("/{id}/certificates/renew", response.Middleware(h.RenewCertificates))
		r.Put("/{id}", response.Middleware(h.UpdateNode))

		// Besu RPC endpoints
		r.Route("/{id}/rpc", func(r chi.Router) {
			r.Get("/accounts", response.Middleware(h.GetBesuAccounts))
			r.Get("/balance", response.Middleware(h.GetBesuBalance))
			r.Get("/code", response.Middleware(h.GetBesuCode))
			r.Get("/storage", response.Middleware(h.GetBesuStorageAt))
			r.Get("/transaction-count", response.Middleware(h.GetBesuTransactionCount))
			r.Get("/block-number", response.Middleware(h.GetBesuBlockNumber))
			r.Get("/block-by-hash", response.Middleware(h.GetBesuBlockByHash))
			r.Get("/block-by-number", response.Middleware(h.GetBesuBlockByNumber))
			r.Get("/block-transaction-count-by-hash", response.Middleware(h.GetBesuBlockTransactionCountByHash))
			r.Get("/block-transaction-count-by-number", response.Middleware(h.GetBesuBlockTransactionCountByNumber))
			r.Get("/transaction-by-hash", response.Middleware(h.GetBesuTransactionByHash))
			r.Get("/transaction-by-block-hash-and-index", response.Middleware(h.GetBesuTransactionByBlockHashAndIndex))
			r.Get("/transaction-by-block-number-and-index", response.Middleware(h.GetBesuTransactionByBlockNumberAndIndex))
			r.Get("/transaction-receipt", response.Middleware(h.GetBesuTransactionReceipt))
			r.Get("/fee-history", response.Middleware(h.GetBesuFeeHistory))
			r.Post("/logs", response.Middleware(h.GetBesuLogs))
			r.Get("/pending-transactions", response.Middleware(h.GetBesuPendingTransactions))
			r.Get("/chain-id", response.Middleware(h.GetBesuChainId))
			r.Get("/protocol-version", response.Middleware(h.GetBesuProtocolVersion))
			r.Get("/syncing", response.Middleware(h.GetBesuSyncing))
			r.Get("/qbft-signer-metrics", response.Middleware(h.GetBesuQbftSignerMetrics))
			r.Get("/qbft-request-timeout", response.Middleware(h.GetBesuQbftRequestTimeoutSeconds))
			r.Post("/qbft-discard-validator-vote", response.Middleware(h.QbftDiscardValidatorVote))
			r.Get("/qbft-pending-votes", response.Middleware(h.QbftGetPendingVotes))
			r.Post("/qbft-propose-validator-vote", response.Middleware(h.QbftProposeValidatorVote))
			r.Get("/qbft-validators-by-block-hash", response.Middleware(h.QbftGetValidatorsByBlockHash))
			r.Get("/qbft-validators-by-block-number", response.Middleware(h.QbftGetValidatorsByBlockNumber))
		})
	})
}

// CreateNode godoc
// @Summary Create a new node
// @Description Create a new node with the specified configuration
// @Tags Nodes
// @Accept json
// @Produce json
// @Param request body CreateNodeRequest true "Node creation request"
// @Success 201 {object} NodeResponse
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes [post]
func (h *NodeHandler) CreateNode(w http.ResponseWriter, r *http.Request) error {
	var req CreateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Validate request
	if req.Name == "" {
		return errors.NewValidationError("name is required", nil)
	}

	if req.BlockchainPlatform == "" {
		return errors.NewValidationError("blockchain platform is required", nil)
	}

	if !isValidPlatform(types.BlockchainPlatform(req.BlockchainPlatform)) {
		return errors.NewValidationError("invalid blockchain platform", map[string]interface{}{
			"valid_platforms": []string{string(types.PlatformFabric), string(types.PlatformBesu)},
		})
	}

	serviceReq := service.CreateNodeRequest{
		Name:               req.Name,
		BlockchainPlatform: req.BlockchainPlatform,
		FabricPeer:         req.FabricPeer,
		FabricOrderer:      req.FabricOrderer,
		BesuNode:           req.BesuNode,
	}

	node, err := h.service.CreateNode(r.Context(), serviceReq)
	if err != nil {
		return errors.NewInternalError("failed to create node", err, nil)
	}

	return response.WriteJSON(w, http.StatusCreated, toNodeResponse(node))
}

// GetNode godoc
// @Summary Get a node
// @Description Get a node by ID
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 200 {object} NodeResponse
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id} [get]
func (h *NodeHandler) GetNode(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	node, err := h.service.GetNode(r.Context(), id)
	if err != nil {
		if errors.IsType(err, errors.NotFoundError) {
			return errors.NewNotFoundError("node not found", nil)
		}
		return errors.NewInternalError("failed to get node", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, toNodeResponse(node))
}

// ListNodes godoc
// @Summary List all nodes
// @Description Get a paginated list of nodes with optional platform filter
// @Tags Nodes
// @Accept json
// @Produce json
// @Param platform query string false "Filter by blockchain platform"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} PaginatedNodesResponse
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes [get]
func (h *NodeHandler) ListNodes(w http.ResponseWriter, r *http.Request) error {
	var platform *types.BlockchainPlatform
	if platformStr := r.URL.Query().Get("platform"); platformStr != "" {
		p := types.BlockchainPlatform(platformStr)
		if !isValidPlatform(p) {
			return errors.NewValidationError("invalid platform", map[string]interface{}{
				"valid_platforms": []string{string(types.PlatformFabric), string(types.PlatformBesu)},
			})
		}
		platform = &p
	}

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	nodes, err := h.service.ListNodes(r.Context(), platform, page, limit)
	if err != nil {
		return errors.NewInternalError("failed to list nodes", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, toPaginatedNodesResponse(nodes))
}

// ListNodesByPlatform godoc
// @Summary List nodes by platform
// @Description Get a paginated list of nodes filtered by blockchain platform
// @Tags Nodes
// @Accept json
// @Produce json
// @Param platform path string true "Blockchain platform (FABRIC/BESU)" Enums(FABRIC,BESU)
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} PaginatedNodesResponse
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/platform/{platform} [get]
func (h *NodeHandler) ListNodesByPlatform(w http.ResponseWriter, r *http.Request) error {
	platform := types.BlockchainPlatform(chi.URLParam(r, "platform"))

	// Validate platform
	if !isValidPlatform(platform) {
		return errors.NewValidationError("invalid platform", map[string]interface{}{
			"valid_platforms": []string{string(types.PlatformFabric), string(types.PlatformBesu)},
		})
	}

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	nodes, err := h.service.ListNodes(r.Context(), &platform, page, limit)
	if err != nil {
		return errors.NewInternalError("failed to list nodes", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, toPaginatedNodesResponse(nodes))
}

// StartNode godoc
// @Summary Start a node
// @Description Start a node by ID
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 200 {object} NodeResponse
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/start [post]
func (h *NodeHandler) StartNode(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	node, err := h.service.StartNode(r.Context(), id)
	if err != nil {
		if err == service.ErrNotFound {
			return errors.NewNotFoundError("node not found", nil)
		}
		return errors.NewInternalError("failed to start node", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, toNodeResponse(node))
}

// StopNode godoc
// @Summary Stop a node
// @Description Stop a node by ID
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 200 {object} NodeResponse
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/stop [post]
func (h *NodeHandler) StopNode(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	node, err := h.service.StopNode(r.Context(), id)
	if err != nil {
		if err == service.ErrNotFound {
			return errors.NewNotFoundError("node not found", nil)
		}
		return errors.NewInternalError("failed to stop node", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, toNodeResponse(node))
}

// RestartNode godoc
// @Summary Restart a node
// @Description Restart a node by ID (stops and starts the node)
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 200 {object} NodeResponse
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/restart [post]
func (h *NodeHandler) RestartNode(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// First stop the node
	_, err = h.service.StopNode(r.Context(), id)
	if err != nil {
		if err == service.ErrNotFound {
			return errors.NewNotFoundError("node not found", nil)
		}
		return errors.NewInternalError("failed to stop node", err, nil)
	}

	// Then start it again
	node, err := h.service.StartNode(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to start node", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, toNodeResponse(node))
}

// DeleteNode godoc
// @Summary Delete a node
// @Description Delete a node by ID
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 204 "No Content"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id} [delete]
func (h *NodeHandler) DeleteNode(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	if err := h.service.DeleteNode(r.Context(), id); err != nil {
		if err == service.ErrNotFound {
			return errors.NewNotFoundError("node not found", nil)
		}
		return errors.NewInternalError("failed to delete node", err, nil)
	}

	return response.WriteJSON(w, http.StatusNoContent, nil)
}

// GetFabricPeerDefaults godoc
// @Summary Get default values for Fabric peer node
// @Description Get default configuration values for a Fabric peer node
// @Tags Nodes
// @Produce json
// @Success 200 {object} service.NodeDefaults
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/defaults/fabric-peer [get]
func (h *NodeHandler) GetFabricPeerDefaults(w http.ResponseWriter, r *http.Request) error {
	defaults := h.service.GetFabricPeerDefaults()
	return response.WriteJSON(w, http.StatusOK, defaults)
}

// GetFabricOrdererDefaults godoc
// @Summary Get default values for Fabric orderer node
// @Description Get default configuration values for a Fabric orderer node
// @Tags Nodes
// @Produce json
// @Success 200 {object} service.NodeDefaults
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/defaults/fabric-orderer [get]
func (h *NodeHandler) GetFabricOrdererDefaults(w http.ResponseWriter, r *http.Request) error {
	defaults := h.service.GetFabricOrdererDefaults()
	return response.WriteJSON(w, http.StatusOK, defaults)
}

// GetFabricNodesDefaults godoc
// @Summary Get default values for multiple Fabric nodes
// @Description Get default configuration values for multiple Fabric nodes
// @Tags Nodes
// @Produce json
// @Param peerCount query int false "Number of peer nodes" default(1) minimum(0)
// @Param ordererCount query int false "Number of orderer nodes" default(1) minimum(0)
// @Param mode query string false "Deployment mode" Enums(service, docker) default(service)
// @Success 200 {object} service.NodesDefaultsResult
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/defaults/fabric [get]
func (h *NodeHandler) GetFabricNodesDefaults(w http.ResponseWriter, r *http.Request) error {
	// Parse query parameters
	peerCount := 1
	if countStr := r.URL.Query().Get("peerCount"); countStr != "" {
		if count, err := strconv.Atoi(countStr); err == nil && count >= 0 {
			peerCount = count
		}
	}

	ordererCount := 1
	if countStr := r.URL.Query().Get("ordererCount"); countStr != "" {
		if count, err := strconv.Atoi(countStr); err == nil && count >= 0 {
			ordererCount = count
		}
	}

	mode := service.ModeService
	if modeStr := r.URL.Query().Get("mode"); modeStr != "" {
		mode = service.Mode(modeStr)
	}

	// Validate mode
	if mode != service.ModeService && mode != service.ModeDocker {
		return errors.NewValidationError("invalid mode", map[string]interface{}{
			"valid_modes": []string{string(service.ModeService), string(service.ModeDocker)},
		})
	}

	result, err := h.service.GetFabricNodesDefaults(service.NodesDefaultsParams{
		PeerCount:    peerCount,
		OrdererCount: ordererCount,
		Mode:         mode,
	})
	if err != nil {
		return errors.NewInternalError("failed to get node defaults", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, result)
}

// GetBesuNodeDefaults godoc
// @Summary Get default values for Besu node
// @Description Get default configuration values for a Besu node
// @Tags Nodes
// @Produce json
// @Param besuNodes query int false "Number of Besu nodes" default(1) minimum(0)
// @Success 200 {object} BesuNodeDefaultsResponse
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/defaults/besu-node [get]
func (h *NodeHandler) GetBesuNodeDefaults(w http.ResponseWriter, r *http.Request) error {
	// Parse besuNodes parameter
	besuNodes := 1
	if countStr := r.URL.Query().Get("besuNodes"); countStr != "" {
		if count, err := strconv.Atoi(countStr); err == nil && count >= 0 {
			besuNodes = count
		}
	}

	defaults, err := h.service.GetBesuNodeDefaults(besuNodes)
	if err != nil {
		return errors.NewInternalError("failed to get Besu node defaults", err, nil)
	}

	res := BesuNodeDefaultsResponse{
		NodeCount: besuNodes,
		Defaults:  defaults,
	}

	return response.WriteJSON(w, http.StatusOK, res)
}

// TailLogs godoc
// @Summary Tail node logs
// @Description Stream logs from a specific node
// @Tags Nodes
// @Accept json
// @Produce text/event-stream
// @Param id path int true "Node ID"
// @Param follow query bool false "Follow logs" default(false)
// @Param tail query int false "Number of lines to show from the end" default(100)
// @Success 200 {string} string "Log stream"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/logs [get]
func (h *NodeHandler) TailLogs(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	// Parse query parameters
	follow := false
	if followStr := r.URL.Query().Get("follow"); followStr == "true" {
		follow = true
	}

	tail := 100 // default to last 100 lines
	if tailStr := r.URL.Query().Get("tail"); tailStr != "" {
		if t, err := strconv.Atoi(tailStr); err == nil && t > 0 {
			tail = t
		}
	}

	// Set headers for streaming response
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create a context that's canceled when the client disconnects
	ctx := r.Context()

	// Create channel for logs
	logChan, err := h.service.TailLogs(ctx, id, tail, follow)
	if err != nil {
		if err == service.ErrNotFound {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to tail logs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Stream logs to client
	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			return
		case logLine, ok := <-logChan:
			if !ok {
				// Channel closed
				return
			}
			// Write log line to response
			fmt.Fprintf(w, "data: %s\n\n", logLine)
			flusher.Flush()
		}
	}
}

// GetNodeEvents godoc
// @Summary Get node events
// @Description Get a paginated list of events for a specific node
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} PaginatedNodeEventsResponse
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/events [get]
func (h *NodeHandler) GetNodeEvents(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	events, err := h.service.GetNodeEvents(r.Context(), id, page, limit)
	if err != nil {
		if err == service.ErrNotFound {
			return errors.NewNotFoundError("node not found", nil)
		}
		return errors.NewInternalError("failed to get node events", err, nil)
	}

	eventsResponse := PaginatedNodeEventsResponse{
		Items: make([]NodeEventResponse, len(events)),
		Page:  page,
	}

	for i, event := range events {
		eventsResponse.Items[i] = toNodeEventResponse(event)
	}

	return response.WriteJSON(w, http.StatusOK, eventsResponse)
}

// GetNodeChannels godoc
// @Summary Get channels for a Fabric node
// @Description Retrieves all channels for a specific Fabric node
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 200 {object} NodeChannelsResponse
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/channels [get]
func (h *NodeHandler) GetNodeChannels(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	channels, err := h.service.GetNodeChannels(r.Context(), id)
	if err != nil {
		if err == service.ErrNotFound {
			return errors.NewNotFoundError("node not found", nil)
		}
		if err == service.ErrInvalidNodeType {
			return errors.NewValidationError("node is not a Fabric node", nil)
		}
		return errors.NewInternalError("failed to get node channels", err, nil)
	}

	channelsResponse := NodeChannelsResponse{
		NodeID:   id,
		Channels: make([]ChannelResponse, len(channels)),
	}

	for i, channel := range channels {
		channelsResponse.Channels[i] = toChannelResponse(channel)
	}

	return response.WriteJSON(w, http.StatusOK, channelsResponse)
}

// NodeChannelsResponse represents the response for node channels
type NodeChannelsResponse struct {
	NodeID   int64             `json:"nodeId"`
	Channels []ChannelResponse `json:"channels"`
}

// ChannelResponse represents a Fabric channel in the response
type ChannelResponse struct {
	Name      string    `json:"name"`
	BlockNum  int64     `json:"blockNum"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

// Helper function to convert service channel to response channel
func toChannelResponse(channel service.Channel) ChannelResponse {
	return ChannelResponse{
		Name:      channel.Name,
		BlockNum:  channel.BlockNum,
		CreatedAt: channel.CreatedAt,
	}
}

func toNodeResponse(node *service.NodeResponse) NodeResponse {
	return NodeResponse{
		ID:                 node.ID,
		Name:               node.Name,
		BlockchainPlatform: node.Platform,
		NodeType:           string(node.NodeType),
		Status:             string(node.Status),
		ErrorMessage:       node.ErrorMessage,
		Endpoint:           node.Endpoint,
		CreatedAt:          node.CreatedAt,
		UpdatedAt:          node.UpdatedAt,
		FabricPeer:         node.FabricPeer,
		FabricOrderer:      node.FabricOrderer,
		BesuNode:           node.BesuNode,
	}
}

// Helper function to validate platform
func isValidPlatform(platform types.BlockchainPlatform) bool {
	switch platform {
	case types.PlatformFabric, types.PlatformBesu:
		return true
	}
	return false
}

func toNodeEventResponse(event service.NodeEvent) NodeEventResponse {
	return NodeEventResponse{
		ID:        event.ID,
		NodeID:    event.NodeID,
		Type:      string(event.Type),
		Data:      event.Data,
		CreatedAt: event.CreatedAt,
	}
}

// RenewCertificates godoc
// @Summary Renew node certificates
// @Description Renews the TLS and signing certificates for a Fabric node
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 200 {object} NodeResponse
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/certificates/renew [post]
func (h *NodeHandler) RenewCertificates(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	node, err := h.service.RenewCertificates(r.Context(), id)
	if err != nil {
		if errors.IsType(err, errors.NotFoundError) {
			return errors.NewNotFoundError("node not found", nil)
		}
		return errors.NewInternalError("failed to renew certificates", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, toNodeResponse(node))
}

// UpdateNode godoc
// @Summary Update a node
// @Description Updates an existing node's configuration based on its type
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param request body UpdateNodeRequest true "Update node request"
// @Success 200 {object} NodeResponse
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id} [put]
func (h *NodeHandler) UpdateNode(w http.ResponseWriter, r *http.Request) error {
	nodeID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	var req UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Get the node to determine its type
	node, err := h.service.GetNode(r.Context(), nodeID)
	if err != nil {
		if errors.IsType(err, errors.NotFoundError) {
			return errors.NewNotFoundError("node not found", nil)
		}
		return errors.NewInternalError("failed to get node", err, nil)
	}

	switch node.NodeType {
	case types.NodeTypeFabricPeer:
		if req.FabricPeer == nil {
			return errors.NewValidationError("fabricPeer configuration is required for Fabric peer nodes", nil)
		}
		return h.updateFabricPeer(w, r, nodeID, req.FabricPeer)
	case types.NodeTypeFabricOrderer:
		if req.FabricOrderer == nil {
			return errors.NewValidationError("fabricOrderer configuration is required for Fabric orderer nodes", nil)
		}
		return h.updateFabricOrderer(w, r, nodeID, req.FabricOrderer)
	case types.NodeTypeBesuFullnode:
		if req.BesuNode == nil {
			return errors.NewValidationError("besuNode configuration is required for Besu nodes", nil)
		}
		return h.updateBesuNode(w, r, nodeID, req.BesuNode)
	default:
		return errors.NewValidationError("unsupported node type", map[string]interface{}{
			"nodeType": node.NodeType,
		})
	}
}

// updateBesuNode handles updating a Besu node
func (h *NodeHandler) updateBesuNode(w http.ResponseWriter, r *http.Request, nodeID int64, req *UpdateBesuNodeRequest) error {
	// Convert HTTP layer request to service layer request
	serviceReq := service.UpdateBesuNodeRequest{
		NetworkID:      req.NetworkID,
		P2PHost:        req.P2PHost,
		P2PPort:        req.P2PPort,
		RPCHost:        req.RPCHost,
		RPCPort:        req.RPCPort,
		Bootnodes:      req.Bootnodes,
		ExternalIP:     req.ExternalIP,
		InternalIP:     req.InternalIP,
		Env:            req.Env,
		Mode:           req.Mode,
		MetricsEnabled: req.MetricsEnabled,
		MetricsPort:    req.MetricsPort,
	}

	// Call service layer to update the Besu node
	updatedNode, err := h.service.UpdateBesuNode(r.Context(), nodeID, serviceReq)
	if err != nil {
		if errors.IsType(err, errors.ValidationError) {
			return errors.NewValidationError("invalid besu node configuration", map[string]interface{}{
				"error": err.Error(),
			})
		}
		if errors.IsType(err, errors.NotFoundError) {
			return errors.NewNotFoundError("node not found", nil)
		}
		return errors.NewInternalError("failed to update besu node", err, nil)
	}

	// Return the updated node as response
	return response.WriteJSON(w, http.StatusOK, toNodeResponse(updatedNode))
}

// updateFabricPeer handles updating a Fabric peer node
func (h *NodeHandler) updateFabricPeer(w http.ResponseWriter, r *http.Request, nodeID int64, req *UpdateFabricPeerRequest) error {
	opts := service.UpdateFabricPeerOpts{
		NodeID: nodeID,
	}

	if req.ExternalEndpoint != nil {
		opts.ExternalEndpoint = *req.ExternalEndpoint
	}
	if req.ListenAddress != nil {
		opts.ListenAddress = *req.ListenAddress
	}
	if req.EventsAddress != nil {
		opts.EventsAddress = *req.EventsAddress
	}
	if req.OperationsListenAddress != nil {
		opts.OperationsListenAddress = *req.OperationsListenAddress
	}
	if req.ChaincodeAddress != nil {
		opts.ChaincodeAddress = *req.ChaincodeAddress
	}
	if req.DomainNames != nil {
		opts.DomainNames = req.DomainNames
	}
	if req.Env != nil {
		opts.Env = req.Env
	}
	if req.AddressOverrides != nil {
		opts.AddressOverrides = req.AddressOverrides
	}
	if req.Version != nil {
		opts.Version = *req.Version
	}
	opts.Mode = req.Mode

	updatedNode, err := h.service.UpdateFabricPeer(r.Context(), opts)
	if err != nil {
		return errors.NewInternalError("failed to update peer", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, toNodeResponse(updatedNode))
}

// updateFabricOrderer handles updating a Fabric orderer node
func (h *NodeHandler) updateFabricOrderer(w http.ResponseWriter, r *http.Request, nodeID int64, req *UpdateFabricOrdererRequest) error {
	opts := service.UpdateFabricOrdererOpts{
		NodeID: nodeID,
	}

	if req.ExternalEndpoint != nil {
		opts.ExternalEndpoint = *req.ExternalEndpoint
	}
	if req.ListenAddress != nil {
		opts.ListenAddress = *req.ListenAddress
	}
	if req.AdminAddress != nil {
		opts.AdminAddress = *req.AdminAddress
	}
	if req.OperationsListenAddress != nil {
		opts.OperationsListenAddress = *req.OperationsListenAddress
	}
	if req.DomainNames != nil {
		opts.DomainNames = req.DomainNames
	}
	if req.Env != nil {
		opts.Env = req.Env
	}
	if req.Version != nil {
		opts.Version = *req.Version
	}
	opts.Mode = req.Mode

	updatedNode, err := h.service.UpdateFabricOrderer(r.Context(), opts)
	if err != nil {
		return errors.NewInternalError("failed to update orderer", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, toNodeResponse(updatedNode))
}

// ChaincodeResponse represents a committed chaincode in the response
type ChaincodeResponse struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	Sequence          int64  `json:"sequence"`
	EndorsementPlugin string `json:"endorsementPlugin"`
	ValidationPlugin  string `json:"validationPlugin"`
	InitRequired      bool   `json:"initRequired"`
	EndorsementPolicy string `json:"endorsementPolicy,omitempty"`
}

// GetNodeChaincodes godoc
// @Summary Get committed chaincodes for a Fabric peer
// @Description Retrieves all committed chaincodes for a specific channel on a Fabric peer node
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param channelID path string true "Channel ID"
// @Success 200 {array} ChaincodeResponse
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/channels/{channelID}/chaincodes [get]
func (h *NodeHandler) GetNodeChaincodes(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	channelID := chi.URLParam(r, "channelID")
	if channelID == "" {
		return errors.NewValidationError("channel ID is required", nil)
	}

	chaincodes, err := h.service.GetFabricChaincodes(r.Context(), id, channelID)
	if err != nil {
		if err == service.ErrNotFound {
			return errors.NewNotFoundError("node not found", nil)
		}
		if err == service.ErrInvalidNodeType {
			return errors.NewValidationError("node is not a Fabric peer", nil)
		}
		return errors.NewInternalError("failed to get chaincodes", err, nil)
	}

	// Convert chaincodes to response format
	chaincodeResponses := make([]ChaincodeResponse, len(chaincodes))
	for i, cc := range chaincodes {
		chaincodeResponses[i] = ChaincodeResponse{
			Name:              cc.Name,
			Version:           cc.Version,
			Sequence:          cc.Sequence,
			EndorsementPlugin: cc.EndorsementPlugin,
			ValidationPlugin:  cc.ValidationPlugin,
			InitRequired:      cc.InitRequired,
		}

		// Convert endorsement policy to string if it exists
		if len(cc.ValidationParameter) > 0 {
			policy, _, err := UnmarshalApplicationPolicy(cc.ValidationParameter)
			if err != nil {
				return errors.NewInternalError("failed to unmarshal endorsement policy", err, nil)
			}
			policyStr, err := SignaturePolicyToString(policy)
			if err != nil {
				return errors.NewInternalError("failed to convert endorsement policy to string", err, nil)
			}
			chaincodeResponses[i].EndorsementPolicy = policyStr
		}
	}

	response.JSON(w, http.StatusOK, chaincodeResponses)
	return nil
}

// GetBesuAccounts godoc
// @Summary Get accounts managed by the Besu node
// @Description Lists accounts managed by the node
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 200 {array} string "Array of account addresses"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/accounts [get]
func (h *NodeHandler) GetBesuAccounts(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	accounts, err := rpcClient.GetAccounts(r.Context())
	if err != nil {
		return errors.NewInternalError("failed to get accounts", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, accounts)
}

// GetBesuBalance godoc
// @Summary Get balance of an address
// @Description Gets balance of an address in Wei
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param address query string true "Account address"
// @Param blockTag query string false "Block tag (default: latest)"
// @Success 200 {string} string "Balance in hex"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/balance [get]
func (h *NodeHandler) GetBesuBalance(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	address := r.URL.Query().Get("address")
	if address == "" {
		return errors.NewValidationError("address parameter is required", nil)
	}

	blockTag := r.URL.Query().Get("blockTag")
	if blockTag == "" {
		blockTag = "latest"
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	balance, err := rpcClient.GetBalance(r.Context(), address, blockTag)
	if err != nil {
		return errors.NewInternalError("failed to get balance", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, balance)
}

// GetBesuCode godoc
// @Summary Get bytecode at an address
// @Description Gets bytecode at an address (e.g., contract code)
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param address query string true "Contract address"
// @Param blockTag query string false "Block tag (default: latest)"
// @Success 200 {string} string "Bytecode in hex"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/code [get]
func (h *NodeHandler) GetBesuCode(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	address := r.URL.Query().Get("address")
	if address == "" {
		return errors.NewValidationError("address parameter is required", nil)
	}

	blockTag := r.URL.Query().Get("blockTag")
	if blockTag == "" {
		blockTag = "latest"
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	code, err := rpcClient.GetCode(r.Context(), address, blockTag)
	if err != nil {
		return errors.NewInternalError("failed to get code", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, code)
}

// GetBesuStorageAt godoc
// @Summary Get storage value at a position
// @Description Gets storage value at a position for an address
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param address query string true "Contract address"
// @Param position query string true "Storage position"
// @Param blockTag query string false "Block tag (default: latest)"
// @Success 200 {string} string "Storage value in hex"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/storage [get]
func (h *NodeHandler) GetBesuStorageAt(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	address := r.URL.Query().Get("address")
	if address == "" {
		return errors.NewValidationError("address parameter is required", nil)
	}

	position := r.URL.Query().Get("position")
	if position == "" {
		return errors.NewValidationError("position parameter is required", nil)
	}

	blockTag := r.URL.Query().Get("blockTag")
	if blockTag == "" {
		blockTag = "latest"
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	value, err := rpcClient.GetStorageAt(r.Context(), address, position, blockTag)
	if err != nil {
		return errors.NewInternalError("failed to get storage value", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, value)
}

// GetBesuTransactionCount godoc
// @Summary Get transaction count for an address
// @Description Gets nonce (tx count) for an address
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param address query string true "Account address"
// @Param blockTag query string false "Block tag (default: latest)"
// @Success 200 {string} string "Transaction count in hex"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/transaction-count [get]
func (h *NodeHandler) GetBesuTransactionCount(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	address := r.URL.Query().Get("address")
	if address == "" {
		return errors.NewValidationError("address parameter is required", nil)
	}

	blockTag := r.URL.Query().Get("blockTag")
	if blockTag == "" {
		blockTag = "latest"
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	count, err := rpcClient.GetTransactionCount(r.Context(), address, blockTag)
	if err != nil {
		return errors.NewInternalError("failed to get transaction count", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, count)
}

// GetBesuBlockNumber godoc
// @Summary Get latest block number
// @Description Gets the latest block number
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 200 {string} string "Block number in hex"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/block-number [get]
func (h *NodeHandler) GetBesuBlockNumber(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	blockNumber, err := rpcClient.GetBlockNumber(r.Context())
	if err != nil {
		return errors.NewInternalError("failed to get block number", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, blockNumber)
}

// GetBesuBlockByHash godoc
// @Summary Get block by hash
// @Description Gets block details by hash
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param hash query string true "Block hash"
// @Param fullTx query bool false "Include full transaction objects" default(false)
// @Success 200 {object} map[string]interface{} "Block object"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/block-by-hash [get]
func (h *NodeHandler) GetBesuBlockByHash(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	hash := r.URL.Query().Get("hash")
	if hash == "" {
		return errors.NewValidationError("hash parameter is required", nil)
	}

	fullTx := r.URL.Query().Get("fullTx") == "true"

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	block, err := rpcClient.GetBlockByHash(r.Context(), hash, fullTx)
	if err != nil {
		return errors.NewInternalError("failed to get block", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, block)
}

// GetBesuBlockByNumber godoc
// @Summary Get block by number
// @Description Gets block by number or tag
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param number query string false "Block number"
// @Param tag query string false "Block tag (latest, earliest, pending)"
// @Param fullTx query bool false "Include full transaction objects" default(false)
// @Success 200 {object} map[string]interface{} "Block object"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/block-by-number [get]
func (h *NodeHandler) GetBesuBlockByNumber(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	number := r.URL.Query().Get("number")
	tag := r.URL.Query().Get("tag")
	if number == "" && tag == "" {
		tag = "latest"
	}

	fullTx := r.URL.Query().Get("fullTx") == "true"

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	block, err := rpcClient.GetBlockByNumber(r.Context(), number, tag, fullTx)
	if err != nil {
		return errors.NewInternalError("failed to get block", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, block)
}

// GetBesuTransactionByHash godoc
// @Summary Get transaction by hash
// @Description Gets transaction details by hash
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param hash query string true "Transaction hash"
// @Success 200 {object} map[string]interface{} "Transaction object"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/transaction-by-hash [get]
func (h *NodeHandler) GetBesuTransactionByHash(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	hash := r.URL.Query().Get("hash")
	if hash == "" {
		return errors.NewValidationError("hash parameter is required", nil)
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	tx, err := rpcClient.GetTransactionByHash(r.Context(), hash)
	if err != nil {
		return errors.NewInternalError("failed to get transaction", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, tx)
}

// GetBesuTransactionReceipt godoc
// @Summary Get transaction receipt
// @Description Gets receipt for a transaction
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param hash query string true "Transaction hash"
// @Success 200 {object} map[string]interface{} "Transaction receipt"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/transaction-receipt [get]
func (h *NodeHandler) GetBesuTransactionReceipt(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	hash := r.URL.Query().Get("hash")
	if hash == "" {
		return errors.NewValidationError("hash parameter is required", nil)
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	receipt, err := rpcClient.GetTransactionReceipt(r.Context(), hash)
	if err != nil {
		return errors.NewInternalError("failed to get transaction receipt", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, receipt)
}

// GetBesuLogs godoc
// @Summary Get event logs
// @Description Gets event logs based on filter criteria
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param request body map[string]interface{} true "Log filter object"
// @Success 200 {array} map[string]interface{} "Array of log objects"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/logs [post]
func (h *NodeHandler) GetBesuLogs(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	var filter map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&filter); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"error": err.Error(),
		})
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	logs, err := rpcClient.GetLogs(r.Context(), filter)
	if err != nil {
		return errors.NewInternalError("failed to get logs", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, logs)
}

// GetBesuPendingTransactions godoc
// @Summary Get pending transactions
// @Description Gets pending transactions in the mempool
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 200 {array} map[string]interface{} "Array of transaction objects"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/pending-transactions [get]
func (h *NodeHandler) GetBesuPendingTransactions(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	txs, err := rpcClient.GetPendingTransactions(r.Context())
	if err != nil {
		return errors.NewInternalError("failed to get pending transactions", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, txs)
}

// GetBesuChainId godoc
// @Summary Get chain ID
// @Description Gets the chain ID
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 200 {string} string "Chain ID in hex"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/chain-id [get]
func (h *NodeHandler) GetBesuChainId(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	chainId, err := rpcClient.GetChainId(r.Context())
	if err != nil {
		return errors.NewInternalError("failed to get chain ID", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, chainId)
}

// GetBesuProtocolVersion godoc
// @Summary Get protocol version
// @Description Gets Ethereum protocol version
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 200 {string} string "Protocol version in hex"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/protocol-version [get]
func (h *NodeHandler) GetBesuProtocolVersion(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	version, err := rpcClient.GetProtocolVersion(r.Context())
	if err != nil {
		return errors.NewInternalError("failed to get protocol version", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, version)
}

// GetBesuSyncing godoc
// @Summary Get sync status
// @Description Gets sync status
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 200 {object} interface{} "Sync object or false"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/syncing [get]
func (h *NodeHandler) GetBesuSyncing(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	syncing, err := rpcClient.GetSyncing(r.Context())
	if err != nil {
		return errors.NewInternalError("failed to get syncing status", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, syncing)
}

// GetBesuBlockTransactionCountByHash godoc
// @Summary Get transaction count in block by hash
// @Description Gets transaction count in a block by hash
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param hash query string true "Block hash"
// @Success 200 {string} string "Transaction count in hex"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/block-transaction-count-by-hash [get]
func (h *NodeHandler) GetBesuBlockTransactionCountByHash(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	hash := r.URL.Query().Get("hash")
	if hash == "" {
		return errors.NewValidationError("hash parameter is required", nil)
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	count, err := rpcClient.GetBlockTransactionCountByHash(r.Context(), hash)
	if err != nil {
		return errors.NewInternalError("failed to get block transaction count", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, count)
}

// GetBesuBlockTransactionCountByNumber godoc
// @Summary Get transaction count in block by number
// @Description Gets transaction count by block number
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param number query string false "Block number"
// @Param tag query string false "Block tag (latest, earliest, pending)"
// @Success 200 {string} string "Transaction count in hex"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/block-transaction-count-by-number [get]
func (h *NodeHandler) GetBesuBlockTransactionCountByNumber(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	number := r.URL.Query().Get("number")
	tag := r.URL.Query().Get("tag")
	if number == "" && tag == "" {
		tag = "latest"
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	count, err := rpcClient.GetBlockTransactionCountByNumber(r.Context(), number, tag)
	if err != nil {
		return errors.NewInternalError("failed to get block transaction count", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, count)
}

// GetBesuTransactionByBlockHashAndIndex godoc
// @Summary Get transaction by block hash and index
// @Description Gets transaction by block hash and index
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param hash query string true "Block hash"
// @Param index query string true "Transaction index"
// @Success 200 {object} map[string]interface{} "Transaction object"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/transaction-by-block-hash-and-index [get]
func (h *NodeHandler) GetBesuTransactionByBlockHashAndIndex(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	hash := r.URL.Query().Get("hash")
	if hash == "" {
		return errors.NewValidationError("hash parameter is required", nil)
	}

	index := r.URL.Query().Get("index")
	if index == "" {
		return errors.NewValidationError("index parameter is required", nil)
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	tx, err := rpcClient.GetTransactionByBlockHashAndIndex(r.Context(), hash, index)
	if err != nil {
		return errors.NewInternalError("failed to get transaction", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, tx)
}

// GetBesuTransactionByBlockNumberAndIndex godoc
// @Summary Get transaction by block number and index
// @Description Gets transaction by block number and index
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param number query string false "Block number"
// @Param tag query string false "Block tag (latest, earliest, pending)"
// @Param index query string true "Transaction index"
// @Success 200 {object} map[string]interface{} "Transaction object"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/transaction-by-block-number-and-index [get]
func (h *NodeHandler) GetBesuTransactionByBlockNumberAndIndex(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	number := r.URL.Query().Get("number")
	tag := r.URL.Query().Get("tag")
	if number == "" && tag == "" {
		tag = "latest"
	}

	index := r.URL.Query().Get("index")
	if index == "" {
		return errors.NewValidationError("index parameter is required", nil)
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	tx, err := rpcClient.GetTransactionByBlockNumberAndIndex(r.Context(), number, tag, index)
	if err != nil {
		return errors.NewInternalError("failed to get transaction", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, tx)
}

// GetBesuFeeHistory godoc
// @Summary Get fee history
// @Description Gets historical gas fees
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param blockCount query string true "Number of blocks"
// @Param newestBlock query string true "Newest block"
// @Param rewardPercentiles query string true "Reward percentiles"
// @Success 200 {object} map[string]interface{} "Fee history object"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/fee-history [get]
func (h *NodeHandler) GetBesuFeeHistory(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	blockCount := r.URL.Query().Get("blockCount")
	if blockCount == "" {
		return errors.NewValidationError("blockCount parameter is required", nil)
	}

	newestBlock := r.URL.Query().Get("newestBlock")
	if newestBlock == "" {
		return errors.NewValidationError("newestBlock parameter is required", nil)
	}

	rewardPercentiles := r.URL.Query().Get("rewardPercentiles")
	if rewardPercentiles == "" {
		return errors.NewValidationError("rewardPercentiles parameter is required", nil)
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	feeHistory, err := rpcClient.GetFeeHistory(r.Context(), blockCount, newestBlock, rewardPercentiles)
	if err != nil {
		return errors.NewInternalError("failed to get fee history", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, feeHistory)
}

// GetBesuQbftSignerMetrics godoc
// @Summary Get QBFT signer metrics
// @Description Gets QBFT signer metrics
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 200 {array} service.QbftSignerMetric "QBFT signer metrics"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/qbft-signer-metrics [get]
func (h *NodeHandler) GetBesuQbftSignerMetrics(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	metrics, err := rpcClient.GetQbftSignerMetrics(r.Context())
	if err != nil {
		return errors.NewInternalError("failed to get QBFT signer metrics", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, metrics)
}

// GetBesuQbftRequestTimeoutSeconds godoc
// @Summary Get QBFT request timeout
// @Description Gets QBFT request timeout in seconds
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 200 {int64} int64 "Request timeout in seconds"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/qbft-request-timeout [get]
func (h *NodeHandler) GetBesuQbftRequestTimeoutSeconds(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	timeout, err := rpcClient.GetQbftRequestTimeoutSeconds(r.Context())
	if err != nil {
		return errors.NewInternalError("failed to get QBFT request timeout", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, timeout)
}

// SignaturePolicyToString converts a SignaturePolicyEnvelope to a human-readable string format
func SignaturePolicyToString(policy *cb.SignaturePolicyEnvelope) (string, error) {
	if policy == nil {
		return "", fmt.Errorf("policy is nil")
	}

	rule := policy.GetRule()
	if rule == nil {
		return "", fmt.Errorf("policy rule is nil")
	}

	switch t := rule.Type.(type) {
	case *cb.SignaturePolicy_SignedBy:
		// Get the identity index
		idx := t.SignedBy
		if idx < 0 || int(idx) >= len(policy.Identities) {
			return "", fmt.Errorf("invalid identity index: %d", idx)
		}
		// Get the identity
		identity := policy.Identities[idx]
		if identity == nil {
			return "", fmt.Errorf("identity at index %d is nil", idx)
		}
		// Get the MSP ID and role
		mspID := strings.TrimSpace(string(identity.Principal))
		role := "member" // Default role
		if identity.PrincipalClassification == msp.MSPPrincipal_ROLE {
			role = "admin"
		}
		return fmt.Sprintf("'%s.%s'", mspID, role), nil

	case *cb.SignaturePolicy_NOutOf_:
		nOutOf := t.NOutOf
		if nOutOf == nil {
			return "", fmt.Errorf("n_out_of policy is nil")
		}
		// Convert sub-rules
		var rules []string
		for _, r := range nOutOf.Rules {
			ruleStr, err := SignaturePolicyToString(&cb.SignaturePolicyEnvelope{
				Version:    policy.Version,
				Rule:       r,
				Identities: policy.Identities,
			})
			if err != nil {
				return "", fmt.Errorf("error converting sub-rule: %w", err)
			}
			rules = append(rules, ruleStr)
		}
		// Format based on N value
		if nOutOf.N == 1 {
			return fmt.Sprintf("OR(%s)", strings.Join(rules, ", ")), nil
		} else if nOutOf.N == int32(len(rules)) {
			return fmt.Sprintf("AND(%s)", strings.Join(rules, ", ")), nil
		} else {
			return fmt.Sprintf("OutOf(%s, %d)", strings.Join(rules, ", "), nOutOf.N), nil
		}

	default:
		return "", fmt.Errorf("unsupported policy type: %T", t)
	}
}

// UnmarshalApplicationPolicy unmarshals the policy baytes and returns either a signature policy or a channel config policy.
func UnmarshalApplicationPolicy(policyBytes []byte) (*cb.SignaturePolicyEnvelope, string, error) {
	applicationPolicy := &cb.ApplicationPolicy{}
	err := proto.Unmarshal(policyBytes, applicationPolicy)
	if err != nil {
		return nil, "", errors.NewInternalError("failed to unmarshal application policy", err, nil)
	}

	switch policy := applicationPolicy.Type.(type) {
	case *cb.ApplicationPolicy_SignaturePolicy:
		return policy.SignaturePolicy, "", nil
	case *cb.ApplicationPolicy_ChannelConfigPolicyReference:
		return nil, policy.ChannelConfigPolicyReference, nil
	default:
		return nil, "", errors.NewInternalError("unsupported policy type", nil, map[string]interface{}{
			"policyType": fmt.Sprintf("%T", policy),
		})
	}
}

// QbftDiscardValidatorVoteRequest represents the request to discard a validator vote
type QbftDiscardValidatorVoteRequest struct {
	ValidatorAddress string `json:"validatorAddress" validate:"required"`
}

// QbftDiscardValidatorVote godoc
// @Summary Discard QBFT validator vote
// @Description Discards a pending vote for a validator proposal
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param request body QbftDiscardValidatorVoteRequest true "Discard vote request"
// @Success 200 {boolean} bool "Success status"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/qbft-discard-validator-vote [post]
func (h *NodeHandler) QbftDiscardValidatorVote(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	var req QbftDiscardValidatorVoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"error": err.Error(),
		})
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	success, err := rpcClient.QbftDiscardValidatorVote(r.Context(), req.ValidatorAddress)
	if err != nil {
		return errors.NewInternalError("failed to discard QBFT validator vote", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, success)
}

// QbftGetPendingVotes godoc
// @Summary Get QBFT pending votes
// @Description Retrieves a map of pending validator proposals where keys are validator addresses and values are boolean (true indicates pending vote)
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Success 200 {object} service.QbftPendingVotes "Map of validator addresses to boolean values indicating pending votes"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/qbft-pending-votes [get]
func (h *NodeHandler) QbftGetPendingVotes(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	pendingVotes, err := rpcClient.QbftGetPendingVotes(r.Context())
	if err != nil {
		return errors.NewInternalError("failed to get QBFT pending votes", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, pendingVotes)
}

// QbftProposeValidatorVoteRequest represents the request to propose a validator vote
type QbftProposeValidatorVoteRequest struct {
	ValidatorAddress string `json:"validatorAddress" validate:"required"`
	Vote             bool   `json:"vote"`
}

// QbftProposeValidatorVote godoc
// @Summary Propose QBFT validator vote
// @Description Proposes a vote to add (true) or remove (false) a validator
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param request body QbftProposeValidatorVoteRequest true "Propose vote request"
// @Success 200 {boolean} bool "Success status"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/qbft-propose-validator-vote [post]
func (h *NodeHandler) QbftProposeValidatorVote(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	var req QbftProposeValidatorVoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"error": err.Error(),
		})
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	success, err := rpcClient.QbftProposeValidatorVote(r.Context(), req.ValidatorAddress, req.Vote)
	if err != nil {
		return errors.NewInternalError("failed to propose QBFT validator vote", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, success)
}

// QbftGetValidatorsByBlockHash godoc
// @Summary Get QBFT validators by block hash
// @Description Retrieves the list of validators for a specific block by its hash
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param blockHash query string true "Block hash"
// @Success 200 {array} string "List of validator addresses"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/qbft-validators-by-block-hash [get]
func (h *NodeHandler) QbftGetValidatorsByBlockHash(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	blockHash := r.URL.Query().Get("blockHash")
	if blockHash == "" {
		return errors.NewValidationError("block hash is required", map[string]interface{}{
			"error": "blockHash query parameter is required",
		})
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	validators, err := rpcClient.QbftGetValidatorsByBlockHash(r.Context(), blockHash)
	if err != nil {
		return errors.NewInternalError("failed to get QBFT validators by block hash", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, validators)
}

// QbftGetValidatorsByBlockNumber godoc
// @Summary Get QBFT validators by block number
// @Description Retrieves the list of validators for a specific block by its number
// @Tags Nodes
// @Accept json
// @Produce json
// @Param id path int true "Node ID"
// @Param blockNumber query string true "Block number (hex string, 'latest', 'earliest', or 'pending')"
// @Success 200 {array} string "List of validator addresses"
// @Failure 400 {object} response.ErrorResponse "Validation error"
// @Failure 404 {object} response.ErrorResponse "Node not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /nodes/{id}/rpc/qbft-validators-by-block-number [get]
func (h *NodeHandler) QbftGetValidatorsByBlockNumber(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid node ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	blockNumber := r.URL.Query().Get("blockNumber")
	if blockNumber == "" {
		return errors.NewValidationError("block number is required", map[string]interface{}{
			"error": "blockNumber query parameter is required",
		})
	}

	rpcClient, err := h.service.GetBesuRPCClient(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get RPC client", err, nil)
	}

	validators, err := rpcClient.QbftGetValidatorsByBlockNumber(r.Context(), blockNumber)
	if err != nil {
		return errors.NewInternalError("failed to get QBFT validators by block number", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, validators)
}

// CheckBesuReadiness checks if the system is ready for Besu node deployment
// @Summary Check Besu readiness
// @Description Check if Java and Besu are installed and ready for deployment
// @Tags nodes
// @Accept json
// @Produce json
// @Success 200 {object} service.BesuReadinessResponse
// @Router /nodes/readiness/besu [get]
func (h *NodeHandler) CheckBesuReadiness(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	readiness, err := h.service.CheckBesuReadiness(ctx)
	if err != nil {
		return fmt.Errorf("failed to check Besu readiness: %w", err)
	}

	response.JSON(w, http.StatusOK, readiness)
	return nil
}
