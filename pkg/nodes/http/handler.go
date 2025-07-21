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
		NetworkID:  req.NetworkID,
		P2PHost:    req.P2PHost,
		P2PPort:    req.P2PPort,
		RPCHost:    req.RPCHost,
		RPCPort:    req.RPCPort,
		Bootnodes:  req.Bootnodes,
		ExternalIP: req.ExternalIP,
		InternalIP: req.InternalIP,
		Env:        req.Env,
		Mode:       req.Mode,
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
