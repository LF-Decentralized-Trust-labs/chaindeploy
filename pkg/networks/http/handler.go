package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	"encoding/base64"

	httpchainlaunch "github.com/chainlaunch/chainlaunch/pkg/http"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/fabric"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// Handler handles HTTP requests for network operations
type Handler struct {
	networkService *service.NetworkService
	nodeService    *nodeservice.NodeService
	validate       *validator.Validate
}

// NewHandler creates a new network handler
func NewHandler(networkService *service.NetworkService, nodeService *nodeservice.NodeService) *Handler {
	return &Handler{
		networkService: networkService,
		nodeService:    nodeService,
		validate:       validator.New(),
	}
}

// RegisterRoutes registers the network routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Fabric network routes with resource middleware
	r.Route("/networks/fabric", func(r chi.Router) {
		// Add resource middleware for all Fabric network routes
		r.Use(httpchainlaunch.ResourceMiddleware("fabric_network"))

		r.Get("/", h.FabricNetworkList)
		r.Post("/", h.FabricNetworkCreate)
		r.Delete("/{id}", h.FabricNetworkDelete)
		r.Post("/{id}/peers/{peerId}/join", h.FabricNetworkJoinPeer)
		r.Post("/{id}/orderers/{ordererId}/join", h.FabricNetworkJoinOrderer)
		r.Delete("/{id}/peers/{peerId}", h.FabricNetworkRemovePeer)
		r.Delete("/{id}/orderers/{ordererId}", h.FabricNetworkRemoveOrderer)
		r.Get("/{id}/channel-config", h.FabricNetworkGetChannelConfig)
		r.Get("/{id}/current-channel-config", h.FabricNetworkGetCurrentChannelConfig)
		r.Get("/{id}", h.FabricNetworkGet)
		r.Post("/{id}/reload-block", h.ReloadNetworkBlock)
		r.Get("/{id}/nodes", h.FabricNetworkGetNodes)
		r.Post("/{id}/nodes", h.FabricNetworkAddNode)
		r.Post("/{id}/peers/{peerId}/unjoin", h.FabricNetworkUnjoinPeer)
		r.Post("/{id}/orderers/{ordererId}/unjoin", h.FabricNetworkUnjoinOrderer)
		r.Post("/{id}/anchor-peers", h.FabricNetworkSetAnchorPeers)
		r.Get("/{id}/organizations/{orgId}/network-config", h.FabricNetworkGetOrganizationConfig)
		r.Get("/by-name/{name}", h.FabricNetworkGetByName)
		r.Post("/import", h.ImportFabricNetwork)
		r.Post("/import-with-org", h.ImportFabricNetworkWithOrg)
		r.Post("/{id}/update-config", h.FabricUpdateChannelConfig)
		r.Get("/{id}/blocks", h.FabricGetBlocks)
		r.Get("/{id}/blocks/{blockNum}", h.FabricGetBlock)
		r.Get("/{id}/info", h.GetChainInfo)
		r.Get("/{id}/transactions/{txId}", h.FabricGetTransaction)
		r.Post("/{id}/organization-crl", h.UpdateOrganizationCRL)
	})

	// Besu network routes with resource middleware
	r.Route("/networks/besu", func(r chi.Router) {
		// Add resource middleware for all Besu network routes
		r.Use(httpchainlaunch.ResourceMiddleware("besu_network"))

		r.Get("/", h.BesuNetworkList)
		r.Post("/", h.BesuNetworkCreate)
		r.Post("/import", h.ImportBesuNetwork)
		r.Get("/{id}", h.BesuNetworkGet)
		r.Delete("/{id}", h.BesuNetworkDelete)
	})
}

// @Summary List Fabric networks
// @Description Get a paginated list of Fabric networks
// @Tags Fabric Networks
// @Produce json
// @Param limit query int false "Number of items to return (default: 10)"
// @Param offset query int false "Number of items to skip (default: 0)"
// @Success 200 {object} ListNetworksResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric [get]
func (h *Handler) FabricNetworkList(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters
	limit := int32(10) // Default limit
	offset := int32(0) // Default offset

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limitInt, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_limit", "Invalid limit parameter")
			return
		}
		limit = int32(limitInt)
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offsetInt, err := strconv.ParseInt(offsetStr, 10, 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_offset", "Invalid offset parameter")
			return
		}
		offset = int32(offsetInt)
	}

	// Get networks from service
	result, err := h.networkService.ListNetworks(r.Context(), service.ListNetworksParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_networks_failed", err.Error())
		return
	}

	// Convert to response type
	networks := make([]NetworkResponse, len(result.Networks))
	for i, network := range result.Networks {
		networks[i] = mapNetworkToResponse(network)
	}

	resp := ListNetworksResponse{
		Networks: networks,
		Total:    result.Total,
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Create a new Fabric network
// @Description Create a new Hyperledger Fabric network with the specified configuration
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param request body CreateFabricNetworkRequest true "Network creation request"
// @Success 201 {object} NetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric [post]
func (h *Handler) FabricNetworkCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateFabricNetworkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}

	// Validate that at least 3 orderer nodes are specified
	ordererCount := 0

	// Count orderer nodes in internal organizations
	for _, org := range req.Config.OrdererOrganizations {
		// Count orderer nodes in each organization
		for _, nodeID := range org.NodeIDs {
			node, err := h.nodeService.GetNode(r.Context(), nodeID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "node_check_failed", err.Error())
				return
			}
			if node.NodeType == nodetypes.NodeTypeFabricOrderer {
				ordererCount++
			}
		}
	}

	// Count orderer nodes in external organizations
	for _, extOrg := range req.Config.ExternalOrdererOrgs {
		ordererCount += len(extOrg.Consenters)
	}

	if ordererCount < 3 {
		writeError(w, http.StatusBadRequest, "insufficient_orderers", "At least 3 orderer nodes are required for a Fabric network")
		return
	}

	// Create the Fabric network config
	fabricConfig := types.FabricNetworkConfig{
		BaseNetworkConfig: types.BaseNetworkConfig{
			Type: types.NetworkTypeFabric,
		},
		ChannelName:          req.Name,
		PeerOrganizations:    make([]types.Organization, len(req.Config.PeerOrganizations)),
		OrdererOrganizations: make([]types.Organization, len(req.Config.OrdererOrganizations)),
	}

	// Convert peer organizations
	for i, org := range req.Config.PeerOrganizations {
		fabricConfig.PeerOrganizations[i] = types.Organization{
			ID:      org.ID,
			NodeIDs: []int64{},
		}
	}

	// Convert orderer organizations
	for i, org := range req.Config.OrdererOrganizations {
		fabricConfig.OrdererOrganizations[i] = types.Organization{
			ID:      org.ID,
			NodeIDs: org.NodeIDs,
		}
	}

	// Marshal the config to bytes
	configBytes, err := json.Marshal(fabricConfig)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "marshal_config_failed", err.Error())
		return
	}

	// Create network using service
	network, err := h.networkService.CreateNetwork(r.Context(), req.Name, req.Description, configBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_network_failed", err.Error())
		return
	}

	// Return network response
	resp := mapNetworkToResponse(*network)
	writeJSON(w, http.StatusCreated, resp)
}

// @Summary Join peer to Fabric network
// @Description Join a peer node to an existing Fabric network
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param id path int true "Network ID"
// @Param peerId path int true "Peer ID"
// @Success 200 {object} NetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/peers/{peerId}/join [post]
func (h *Handler) FabricNetworkJoinPeer(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	peerID, err := strconv.ParseInt(chi.URLParam(r, "peerId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_peer_id", "Invalid peer ID")
		return
	}

	if err := h.networkService.JoinPeerToNetwork(networkID, peerID); err != nil {
		writeError(w, http.StatusInternalServerError, "join_peer_failed", err.Error())
		return
	}

	// Return network response
	resp := NetworkResponse{
		ID:     networkID,
		Status: "updated",
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Join orderer to Fabric network
// @Description Join an orderer node to an existing Fabric network
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param id path int true "Network ID"
// @Param ordererId path int true "Orderer ID"
// @Success 200 {object} NetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/orderers/{ordererId}/join [post]
func (h *Handler) FabricNetworkJoinOrderer(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	ordererID, err := strconv.ParseInt(chi.URLParam(r, "ordererId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_orderer_id", "Invalid orderer ID")
		return
	}

	if err := h.networkService.JoinOrdererToNetwork(networkID, ordererID); err != nil {
		writeError(w, http.StatusInternalServerError, "join_orderer_failed", err.Error())
		return
	}

	// Return network response
	resp := NetworkResponse{
		ID:     networkID,
		Status: "updated",
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Remove peer from Fabric network
// @Description Remove a peer node from an existing Fabric network
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param id path int true "Network ID"
// @Param peerId path int true "Peer ID"
// @Success 200 {object} NetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/peers/{peerId} [delete]
func (h *Handler) FabricNetworkRemovePeer(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	peerID, err := strconv.ParseInt(chi.URLParam(r, "peerId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_peer_id", "Invalid peer ID")
		return
	}

	if err := h.networkService.RemovePeerFromNetwork(networkID, peerID); err != nil {
		writeError(w, http.StatusInternalServerError, "remove_peer_failed", err.Error())
		return
	}

	resp := NetworkResponse{
		ID:     networkID,
		Status: "updated",
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Remove orderer from Fabric network
// @Description Remove an orderer node from an existing Fabric network
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param id path int true "Network ID"
// @Param ordererId path int true "Orderer ID"
// @Success 200 {object} NetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/orderers/{ordererId} [delete]
func (h *Handler) FabricNetworkRemoveOrderer(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	ordererID, err := strconv.ParseInt(chi.URLParam(r, "ordererId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_orderer_id", "Invalid orderer ID")
		return
	}

	if err := h.networkService.RemoveOrdererFromNetwork(networkID, ordererID); err != nil {
		writeError(w, http.StatusInternalServerError, "remove_orderer_failed", err.Error())
		return
	}

	resp := NetworkResponse{
		ID:     networkID,
		Status: "updated",
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Get Fabric network channel configuration
// @Description Retrieve the channel configuration for a Fabric network
// @Tags Fabric Networks
// @Produce json
// @Param id path int true "Network ID"
// @Success 200 {object} ChannelConfigResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/channel-config [get]
func (h *Handler) FabricNetworkGetChannelConfig(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	network, err := h.networkService.GetNetwork(r.Context(), networkID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_network_failed", err.Error())
		return
	}
	config, err := h.networkService.GetFabricChannelConfig(networkID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_config_failed", err.Error())
		return
	}

	resp := ChannelConfigResponse{
		Name:          network.Name,
		ChannelConfig: config,
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Get Fabric network current channel configuration
// @Description Retrieve the current channel configuration for a Fabric network
// @Tags Fabric Networks
// @Produce json
// @Param id path int true "Network ID"
// @Success 200 {object} ChannelConfigResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/current-channel-config [get]
func (h *Handler) FabricNetworkGetCurrentChannelConfig(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	network, err := h.networkService.GetNetwork(r.Context(), networkID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_network_failed", err.Error())
		return
	}
	config, err := h.networkService.GetFabricCurrentChannelConfig(networkID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_current_config_failed", err.Error())
		return
	}

	resp := ChannelConfigResponse{
		Name:          network.Name,
		ChannelConfig: config,
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Delete a Fabric network
// @Description Delete an existing Fabric network and all its resources
// @Tags Fabric Networks
// @Produce json
// @Param id path int true "Network ID"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id} [delete]
func (h *Handler) FabricNetworkDelete(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	if err := h.networkService.DeleteNetwork(r.Context(), networkID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_network_failed", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary Get a Fabric network by ID
// @Description Get details of a specific Fabric network
// @Tags Fabric Networks
// @Produce json
// @Param id path int true "Network ID"
// @Success 200 {object} NetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id} [get]
func (h *Handler) FabricNetworkGet(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	network, err := h.networkService.GetNetwork(r.Context(), networkID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			writeError(w, http.StatusNotFound, "network_not_found", "Network not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_network_failed", err.Error())
		return
	}

	resp := NetworkResponse{
		ID:        network.ID,
		Name:      network.Name,
		Platform:  network.Platform,
		Status:    string(network.Status),
		CreatedAt: network.CreatedAt.Format(time.RFC3339),
		Config:    network.Config,
	}

	writeJSON(w, http.StatusOK, resp)
}

// @Summary Get network nodes
// @Description Get all nodes associated with a network
// @Tags Fabric Networks
// @Produce json
// @Param id path int true "Network ID"
// @Success 200 {object} GetNetworkNodesResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/nodes [get]
func (h *Handler) FabricNetworkGetNodes(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	nodes, err := h.networkService.GetNetworkNodes(r.Context(), networkID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			writeError(w, http.StatusNotFound, "network_not_found", "Network not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_network_nodes_failed", err.Error())
		return
	}

	resp := GetNetworkNodesResponse{
		Nodes: nodes,
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Add node to network
// @Description Add a node (peer or orderer) to an existing network
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param id path int true "Network ID"
// @Param request body AddNodeToNetworkRequest true "Node addition request"
// @Success 200 {object} NetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/nodes [post]
func (h *Handler) FabricNetworkAddNode(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	var req AddNodeToNetworkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}

	err = h.networkService.AddNodeToNetwork(r.Context(), networkID, req.NodeID, req.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "add_node_failed", err.Error())
		return
	}

	resp := NetworkResponse{
		ID:     networkID,
		Status: "updated",
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Unjoin peer from Fabric network
// @Description Remove a peer node from a channel but keep it in the network
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param id path int true "Network ID"
// @Param peerId path int true "Peer ID"
// @Success 200 {object} NetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/peers/{peerId}/unjoin [post]
func (h *Handler) FabricNetworkUnjoinPeer(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	peerID, err := strconv.ParseInt(chi.URLParam(r, "peerId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_peer_id", "Invalid peer ID")
		return
	}

	if err := h.networkService.UnjoinPeerFromNetwork(networkID, peerID); err != nil {
		writeError(w, http.StatusInternalServerError, "unjoin_peer_failed", err.Error())
		return
	}

	resp := NetworkResponse{
		ID:     networkID,
		Status: "updated",
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Unjoin orderer from Fabric network
// @Description Remove an orderer node from a channel but keep it in the network
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param id path int true "Network ID"
// @Param ordererId path int true "Orderer ID"
// @Success 200 {object} NetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/orderers/{ordererId}/unjoin [post]
func (h *Handler) FabricNetworkUnjoinOrderer(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	ordererID, err := strconv.ParseInt(chi.URLParam(r, "ordererId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_orderer_id", "Invalid orderer ID")
		return
	}

	if err := h.networkService.UnjoinOrdererFromNetwork(networkID, ordererID); err != nil {
		writeError(w, http.StatusInternalServerError, "unjoin_orderer_failed", err.Error())
		return
	}

	resp := NetworkResponse{
		ID:     networkID,
		Status: "updated",
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Set anchor peers for an organization
// @Description Set the anchor peers for an organization in a Fabric network
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param id path int true "Network ID"
// @Param request body SetAnchorPeersRequest true "Anchor peers configuration"
// @Success 200 {object} SetAnchorPeersResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/anchor-peers [post]
func (h *Handler) FabricNetworkSetAnchorPeers(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	var req SetAnchorPeersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	anchorPeers := make([]service.AnchorPeer, len(req.AnchorPeers))
	for i, peer := range req.AnchorPeers {
		anchorPeers[i] = service.AnchorPeer{
			Host: peer.Host,
			Port: peer.Port,
		}
	}

	txID, err := h.networkService.SetAnchorPeers(r.Context(), networkID, req.OrganizationID, anchorPeers)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "set_anchor_peers_failed", err.Error())
		return
	}

	resp := SetAnchorPeersResponse{
		TransactionID: txID,
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Get network configuration
// @Description Get the network configuration as YAML
// @Tags Fabric Networks
// @Produce text/yaml
// @Param id path int true "Network ID"
// @Param orgId path int true "Organization ID"
// @Success 200 {string} string "Network configuration YAML"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/organizations/{orgId}/config [get]
func (h *Handler) FabricNetworkGetOrganizationConfig(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	orgID, err := strconv.ParseInt(chi.URLParam(r, "orgId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_org_id", "Invalid organization ID")
		return
	}

	configYAML, err := h.networkService.GetNetworkConfig(r.Context(), networkID, orgID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			writeError(w, http.StatusNotFound, "network_not_found", "Network or organization not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_network_config_failed", err.Error())
		return
	}

	// Set content type to YAML
	w.Header().Set("Content-Type", "text/yaml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(configYAML))
}

// @Summary Get a Fabric network by slug
// @Description Get details of a specific Fabric network using its slug
// @Tags Fabric Networks
// @Produce json
// @Param slug path string true "Network Slug"
// @Success 200 {object} NetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/by-name/{name} [get]
func (h *Handler) FabricNetworkGetByName(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid_name", "Invalid network name")
		return
	}

	network, err := h.networkService.GetNetworkByName(r.Context(), name)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			writeError(w, http.StatusNotFound, "network_not_found", "Network not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_network_failed", err.Error())
		return
	}

	resp := mapNetworkToResponse(*network)
	writeJSON(w, http.StatusOK, resp)
}

// Update mapNetworkToResponse
func mapNetworkToResponse(n service.Network) NetworkResponse {
	var updatedAt *string
	if n.UpdatedAt != nil {
		timeStr := n.UpdatedAt.Format(time.RFC3339)
		updatedAt = &timeStr
	}

	return NetworkResponse{
		ID:                 n.ID,
		Name:               n.Name,
		Platform:           n.Platform,
		Status:             string(n.Status),
		Description:        n.Description,
		Config:             n.Config,
		DeploymentConfig:   n.DeploymentConfig,
		ExposedPorts:       n.ExposedPorts,
		GenesisBlock:       n.GenesisBlock,
		CurrentConfigBlock: n.CurrentConfigBlock,
		Domain:             n.Domain,
		CreatedAt:          n.CreatedAt.Format(time.RFC3339),
		CreatedBy:          n.CreatedBy,
		UpdatedAt:          updatedAt,
	}
}

// Helper functions for writing responses
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, error string, message string) {
	resp := ErrorResponse{
		Error:   error,
		Code:    code,
		Message: message,
	}
	writeJSON(w, code, resp)
}

// @Summary List Besu networks
// @Description Get a paginated list of Besu networks
// @Tags Besu Networks
// @Produce json
// @Param limit query int false "Number of items to return (default: 10)"
// @Param offset query int false "Number of items to skip (default: 0)"
// @Success 200 {object} ListBesuNetworksResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/besu [get]
func (h *Handler) BesuNetworkList(w http.ResponseWriter, r *http.Request) {
	limit := int32(10) // Default limit
	offset := int32(0) // Default offset

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limitInt, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_limit", "Invalid limit parameter")
			return
		}
		limit = int32(limitInt)
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offsetInt, err := strconv.ParseInt(offsetStr, 10, 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_offset", "Invalid offset parameter")
			return
		}
		offset = int32(offsetInt)
	}

	result, err := h.networkService.ListNetworks(r.Context(), service.ListNetworksParams{
		Limit:    limit,
		Offset:   offset,
		Platform: service.BlockchainTypeBesu,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_networks_failed", err.Error())
		return
	}

	networks := make([]BesuNetworkResponse, len(result.Networks))
	for i, network := range result.Networks {
		networks[i] = mapBesuNetworkToResponse(network)
	}

	resp := ListBesuNetworksResponse{
		Networks: networks,
		Total:    result.Total,
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Create a new Besu network
// @Description Create a new Besu network with the specified configuration
// @Tags Besu Networks
// @Accept json
// @Produce json
// @Param request body CreateBesuNetworkRequest true "Network creation request"
// @Success 200 {object} BesuNetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/besu [post]
func (h *Handler) BesuNetworkCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateBesuNetworkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}

	// Create the Besu network config
	besuConfig := &types.BesuNetworkConfig{
		BaseNetworkConfig: types.BaseNetworkConfig{
			Type: types.NetworkTypeBesu,
		},
		ChainID:                req.Config.ChainID,
		Consensus:              types.BesuConsensusType(req.Config.Consensus),
		InitialValidatorKeyIds: req.Config.InitialValidatorKeyIds,
		BlockPeriod:            req.Config.BlockPeriod,
		EpochLength:            req.Config.EpochLength,
		RequestTimeout:         req.Config.RequestTimeout,
		Nonce:                  req.Config.Nonce,
		Timestamp:              req.Config.Timestamp,
		GasLimit:               req.Config.GasLimit,
		Difficulty:             req.Config.Difficulty,
		MixHash:                req.Config.MixHash,
		Coinbase:               req.Config.Coinbase,
	}

	// Add allocation if provided
	if req.Config.Alloc != nil {
		besuConfig.Alloc = make(map[string]types.AccountBalance)
		for address, balance := range req.Config.Alloc {
			besuConfig.Alloc[address] = types.AccountBalance{
				Balance: balance.Balance,
			}
		}
	}

	// Marshal the config to bytes
	configBytes, err := json.Marshal(besuConfig)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "marshal_config_failed", err.Error())
		return
	}

	// Create the network
	network, err := h.networkService.CreateNetwork(r.Context(), req.Name, req.Description, configBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_network_failed", err.Error())
		return
	}

	// Convert to response type and return
	resp := mapBesuNetworkToResponse(*network)
	writeJSON(w, http.StatusCreated, resp)
}

// @Summary Get a Besu network by ID
// @Description Get details of a specific Besu network
// @Tags Besu Networks
// @Produce json
// @Param id path int true "Network ID"
// @Success 200 {object} BesuNetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/besu/{id} [get]
func (h *Handler) BesuNetworkGet(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	network, err := h.networkService.GetNetwork(r.Context(), networkID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			writeError(w, http.StatusNotFound, "network_not_found", "Network not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_network_failed", err.Error())
		return
	}

	resp := mapBesuNetworkToResponse(*network)
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Delete a Besu network
// @Description Delete an existing Besu network and all its resources
// @Tags Besu Networks
// @Produce json
// @Param id path int true "Network ID"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/besu/{id} [delete]
func (h *Handler) BesuNetworkDelete(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	if err := h.networkService.DeleteNetwork(r.Context(), networkID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_network_failed", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Helper function to map network to Besu response
func mapBesuNetworkToResponse(n service.Network) BesuNetworkResponse {
	var updatedAt string
	if n.UpdatedAt != nil {
		updatedAt = n.UpdatedAt.Format(time.RFC3339)
	}

	var chainID int64
	if n.Config != nil {
		var config types.BesuNetworkConfig
		if err := json.Unmarshal(n.Config, &config); err == nil {
			chainID = config.ChainID
		}
	} else {
		chainID = 0
	}
	return BesuNetworkResponse{
		ID:            n.ID,
		Name:          n.Name,
		Description:   n.Description,
		Status:        string(n.Status),
		ChainID:       chainID,
		CreatedAt:     n.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     updatedAt,
		Config:        n.Config,
		GenesisConfig: n.GenesisBlock,
		Platform:      n.Platform,
	}
}

// @Summary Reload network config block
// @Description Reloads the current config block for a network
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param id path int true "Network ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /networks/fabric/{id}/reload-block [post]
func (h *Handler) ReloadNetworkBlock(w http.ResponseWriter, r *http.Request) {
	// Get network ID from path
	networkID := chi.URLParam(r, "id")
	if networkID == "" {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	networkIDInt, err := strconv.ParseInt(networkID, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	// Call service method to reload block
	err = h.networkService.ReloadFabricNetworkBlock(r.Context(), networkIDInt)
	if err != nil {
		// Handle different types of errors
		if err.Error() == "network not found" {
			writeError(w, http.StatusNotFound, "network_not_found", "Network not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "reload_failed", fmt.Sprintf("Failed to reload network block: %v", err))
		return
	}

	// Return success response
	resp := map[string]string{
		"message": "Network block reloaded successfully",
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Import a Fabric network
// @Description Import an existing Fabric network using its genesis block
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param request body ImportFabricNetworkRequest true "Import network request"
// @Success 200 {object} ImportNetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/import [post]
func (h *Handler) ImportFabricNetwork(w http.ResponseWriter, r *http.Request) {
	var req ImportFabricNetworkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate request
	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}

	// Decode base64 genesis block
	genesisBlockStr := string(req.GenesisFile)
	genesisBlock, err := base64.StdEncoding.DecodeString(genesisBlockStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_genesis_block", "Invalid base64-encoded genesis block")
		return
	}

	result, err := h.networkService.ImportNetwork(r.Context(), service.ImportNetworkParams{
		NetworkType: "fabric",
		GenesisFile: genesisBlock,
		Description: req.Description,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "import_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ImportNetworkResponse{
		NetworkID: result.NetworkID,
		Message:   result.Message,
	})
}

// @Summary Import a Fabric network with organization
// @Description Import an existing Fabric network using organization details
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param request body ImportFabricNetworkWithOrgRequest true "Import network with org request"
// @Success 200 {object} ImportNetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/import-with-org [post]
func (h *Handler) ImportFabricNetworkWithOrg(w http.ResponseWriter, r *http.Request) {
	var req ImportFabricNetworkWithOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate request
	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}

	// Decode base64 TLS cert
	tlsCertStr := string(req.OrdererTLSCert)
	tlsCert, err := base64.StdEncoding.DecodeString(tlsCertStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_tls_cert", "Invalid base64-encoded TLS certificate")
		return
	}

	result, err := h.networkService.ImportFabricNetworkWithOrg(r.Context(), service.ImportNetworkWithOrgParams{
		ChannelID:      req.ChannelID,
		OrganizationID: req.OrganizationID,
		OrdererURL:     req.OrdererURL,
		OrdererTLSCert: tlsCert,
		Description:    req.Description,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "import_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ImportNetworkResponse{
		NetworkID: result.NetworkID,
		Message:   result.Message,
	})
}

// @Summary Import a Besu network
// @Description Import an existing Besu network using its genesis file
// @Tags Besu Networks
// @Accept json
// @Produce json
// @Param request body ImportBesuNetworkRequest true "Import network request"
// @Success 200 {object} ImportNetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/besu/import [post]
func (h *Handler) ImportBesuNetwork(w http.ResponseWriter, r *http.Request) {
	var req ImportBesuNetworkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate request
	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}

	// Decode base64 genesis block
	genesisBlockStr := string(req.GenesisFile)
	genesisBlock, err := base64.StdEncoding.DecodeString(genesisBlockStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_genesis_block", "Invalid base64-encoded genesis block")
		return
	}

	result, err := h.networkService.ImportNetwork(r.Context(), service.ImportNetworkParams{
		NetworkType: "besu",
		GenesisFile: genesisBlock,
		Name:        req.Name,
		Description: req.Description,
		ChainID:     &req.ChainID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "import_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ImportNetworkResponse{
		NetworkID: result.NetworkID,
		Message:   result.Message,
	})
}

// ConfigUpdateOperationRequest represents a configuration update operation
// @Description A single configuration update operation
type ConfigUpdateOperationRequest struct {
	// Type is the type of configuration update operation
	// enum: add_org,remove_org,update_org_msp,set_anchor_peers,add_consenter,remove_consenter,update_consenter,update_etcd_raft_options,update_batch_size,update_batch_timeout
	Type string `json:"type" validate:"required,oneof=add_org remove_org update_org_msp set_anchor_peers add_consenter remove_consenter update_consenter update_etcd_raft_options update_batch_size update_batch_timeout"`

	// Payload contains the operation-specific data
	// The structure depends on the operation type:
	// - add_org: AddOrgPayload
	// - remove_org: RemoveOrgPayload
	// - update_org_msp: UpdateOrgMSPPayload
	// - set_anchor_peers: SetAnchorPeersPayload
	// - add_consenter: AddConsenterPayload
	// - remove_consenter: RemoveConsenterPayload
	// - update_consenter: UpdateConsenterPayload
	// - update_etcd_raft_options: UpdateEtcdRaftOptionsPayload
	// - update_batch_size: UpdateBatchSizePayload
	// - update_batch_timeout: UpdateBatchTimeoutPayload
	// @Description The payload for the configuration update operation
	// @Description Can be one of:
	// @Description - AddOrgPayload when type is "add_org"
	// @Description - RemoveOrgPayload when type is "remove_org"
	// @Description - UpdateOrgMSPPayload when type is "update_org_msp"
	// @Description - SetAnchorPeersPayload when type is "set_anchor_peers"
	// @Description - AddConsenterPayload when type is "add_consenter"
	// @Description - RemoveConsenterPayload when type is "remove_consenter"
	// @Description - UpdateConsenterPayload when type is "update_consenter"
	// @Description - UpdateEtcdRaftOptionsPayload when type is "update_etcd_raft_options"
	// @Description - UpdateBatchSizePayload when type is "update_batch_size"
	// @Description - UpdateBatchTimeoutPayload when type is "update_batch_timeout"
	Payload json.RawMessage `json:"payload" validate:"required"`
}

// Example:
//
//	{
//	  "msp_id": "Org3MSP",
//	  "tls_root_certs": ["-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"],
//	  "root_certs": ["-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"]
//	}
//
// AddOrgPayload represents the payload for adding an organization
type AddOrgPayload struct {
	MSPID        string   `json:"msp_id" validate:"required"`
	TLSRootCerts []string `json:"tls_root_certs" validate:"required,min=1"`
	RootCerts    []string `json:"root_certs" validate:"required,min=1"`
}

// Example:
//
//	{
//	  "msp_id": "Org2MSP"
//	}
//
// RemoveOrgPayload represents the payload for removing an organization
type RemoveOrgPayload struct {
	MSPID string `json:"msp_id" validate:"required"`
}

// Example:
//
//	{
//	  "msp_id": "Org1MSP",
//	  "tls_root_certs": ["-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"],
//	  "root_certs": ["-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"]
//	}
//
// UpdateOrgMSPPayload represents the payload for updating an organization's MSP
type UpdateOrgMSPPayload struct {
	MSPID        string   `json:"msp_id" validate:"required"`
	TLSRootCerts []string `json:"tls_root_certs" validate:"required,min=1"`
	RootCerts    []string `json:"root_certs" validate:"required,min=1"`
}

// Example:
//
//	{
//	  "msp_id": "Org1MSP",
//	  "anchor_peers": [
//	    {
//	      "host": "peer0.org1.example.com",
//	      "port": 7051
//	    }
//	  ]
//	}
//
// SetAnchorPeersPayload represents the payload for setting anchor peers
type SetAnchorPeersPayload struct {
	MSPID       string `json:"msp_id" validate:"required"`
	AnchorPeers []struct {
		Host string `json:"host" validate:"required"`
		Port int    `json:"port" validate:"required,min=1,max=65535"`
	} `json:"anchor_peers" validate:"required,min=1"`
}

// Example:
//
//	{
//	  "host": "orderer3.example.com",
//	  "port": 7050,
//	  "client_tls_cert": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
//	  "server_tls_cert": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"
//	}
//
// AddConsenterPayload represents the payload for adding a consenter
type AddConsenterPayload struct {
	Host          string `json:"host" validate:"required"`
	Port          int    `json:"port" validate:"required,min=1,max=65535"`
	ClientTLSCert string `json:"client_tls_cert" validate:"required"`
	ServerTLSCert string `json:"server_tls_cert" validate:"required"`
}

// Example:
//
//	{
//	  "host": "orderer2.example.com",
//	  "port": 7050
//	}
//
// RemoveConsenterPayload represents the payload for removing a consenter
type RemoveConsenterPayload struct {
	Host string `json:"host" validate:"required"`
	Port int    `json:"port" validate:"required,min=1,max=65535"`
}

// Example:
//
//	{
//	  "host": "orderer1.example.com",
//	  "port": 7050,
//	  "new_host": "orderer1-new.example.com",
//	  "new_port": 7050,
//	  "client_tls_cert": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
//	  "server_tls_cert": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"
//	}
//
// UpdateConsenterPayload represents the payload for updating a consenter
type UpdateConsenterPayload struct {
	Host          string `json:"host" validate:"required"`
	Port          int    `json:"port" validate:"required,min=1,max=65535"`
	NewHost       string `json:"new_host" validate:"required"`
	NewPort       int    `json:"new_port" validate:"required,min=1,max=65535"`
	ClientTLSCert string `json:"client_tls_cert" validate:"required"`
	ServerTLSCert string `json:"server_tls_cert" validate:"required"`
}

// Example:
//
//	{
//	  "tick_interval": "500ms",
//	  "election_tick": 10,
//	  "heartbeat_tick": 1,
//	  "max_inflight_blocks": 5,
//	  "snapshot_interval_size": 16777216
//	}
//
// UpdateEtcdRaftOptionsPayload represents the payload for updating etcd raft options
type UpdateEtcdRaftOptionsPayload struct {
	TickInterval         string `json:"tick_interval" validate:"required"`
	ElectionTick         uint32 `json:"election_tick" validate:"required,min=1"`
	HeartbeatTick        uint32 `json:"heartbeat_tick" validate:"required,min=1"`
	MaxInflightBlocks    uint32 `json:"max_inflight_blocks" validate:"required,min=1"`
	SnapshotIntervalSize uint32 `json:"snapshot_interval_size" validate:"required,min=1"`
}

// Example:
//
//	{
//	  "max_message_count": 100,
//	  "absolute_max_bytes": 10485760,
//	  "preferred_max_bytes": 2097152
//	}
//
// UpdateBatchSizePayload represents the payload for updating batch size
type UpdateBatchSizePayload struct {
	MaxMessageCount   uint32 `json:"max_message_count" validate:"required,min=1"`
	AbsoluteMaxBytes  uint32 `json:"absolute_max_bytes" validate:"required,min=1"`
	PreferredMaxBytes uint32 `json:"preferred_max_bytes" validate:"required,min=1"`
}

// Example:
//
//	{
//	  "timeout": "2s"
//	}
//
// UpdateBatchTimeoutPayload represents the payload for updating batch timeout
type UpdateBatchTimeoutPayload struct {
	Timeout string `json:"timeout" validate:"required"` // e.g., "2s"
}

// UpdateFabricNetworkRequest represents a request to update a Fabric network
type UpdateFabricNetworkRequest struct {
	Operations []ConfigUpdateOperationRequest `json:"operations" validate:"required,min=1,dive"`
}

// @Summary Submit config update proposal
// @Description Submit a signed config update proposal for execution
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param id path int true "Network ID"
// @Param proposalId path string true "Proposal ID"
// @Success 200 {object} AddOrgPayload
// @Success 200 {object} RemoveOrgPayload
// @Success 201 {object} UpdateOrgMSPPayload
// @Success 202 {object} SetAnchorPeersPayload
// @Success 203 {object} AddConsenterPayload
// @Success 204 {object} RemoveConsenterPayload
// @Success 205 {object} UpdateConsenterPayload
// @Success 206 {object} UpdateEtcdRaftOptionsPayload
// @Success 207 {object} UpdateBatchSizePayload
// @Success 208 {object} UpdateBatchTimeoutPayload
// @Router /dummy [post]
func (h *Handler) DummyHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusBadRequest, "dummy_error", "Dummy error")
}

// @Summary Prepare a config update for a Fabric network
// @Description Prepare a config update proposal for a Fabric network using the provided operations.
// @Description The following operation types are supported:
// @Description - add_org: Add a new organization to the channel
// @Description - remove_org: Remove an organization from the channel
// @Description - update_org_msp: Update an organization's MSP configuration
// @Description - set_anchor_peers: Set anchor peers for an organization
// @Description - add_consenter: Add a new consenter to the orderer
// @Description - remove_consenter: Remove a consenter from the orderer
// @Description - update_consenter: Update a consenter in the orderer
// @Description - update_etcd_raft_options: Update etcd raft options for the orderer
// @Description - update_batch_size: Update batch size for the orderer
// @Description - update_batch_timeout: Update batch timeout for the orderer
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param id path int true "Network ID"
// @Param request body UpdateFabricNetworkRequest true "Config update operations"
// @Success 200 {object} ConfigUpdateResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/update-config [post]
func (h *Handler) FabricUpdateChannelConfig(w http.ResponseWriter, r *http.Request) {
	// Parse network ID from URL
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	// Parse request body
	var req UpdateFabricNetworkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate request
	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	// Validate each operation's payload
	for i, op := range req.Operations {
		switch op.Type {
		case "add_org":
			var payload AddOrgPayload
			if err := json.Unmarshal(op.Payload, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_payload", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
			if err := h.validate.Struct(payload); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
		case "remove_org":
			var payload RemoveOrgPayload
			if err := json.Unmarshal(op.Payload, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_payload", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
			if err := h.validate.Struct(payload); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
		case "update_org_msp":
			var payload UpdateOrgMSPPayload
			if err := json.Unmarshal(op.Payload, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_payload", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
			if err := h.validate.Struct(payload); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
		case "set_anchor_peers":
			var payload SetAnchorPeersPayload
			if err := json.Unmarshal(op.Payload, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_payload", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
			if err := h.validate.Struct(payload); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
		case "add_consenter":
			var payload AddConsenterPayload
			if err := json.Unmarshal(op.Payload, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_payload", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
			if err := h.validate.Struct(payload); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
		case "remove_consenter":
			var payload RemoveConsenterPayload
			if err := json.Unmarshal(op.Payload, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_payload", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
			if err := h.validate.Struct(payload); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
		case "update_consenter":
			var payload UpdateConsenterPayload
			if err := json.Unmarshal(op.Payload, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_payload", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
			if err := h.validate.Struct(payload); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
		case "update_etcd_raft_options":
			var payload UpdateEtcdRaftOptionsPayload
			if err := json.Unmarshal(op.Payload, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_payload", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
			if err := h.validate.Struct(payload); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
		case "update_batch_size":
			var payload UpdateBatchSizePayload
			if err := json.Unmarshal(op.Payload, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_payload", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
			if err := h.validate.Struct(payload); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
		case "update_batch_timeout":
			var payload UpdateBatchTimeoutPayload
			if err := json.Unmarshal(op.Payload, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_payload", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
			if err := h.validate.Struct(payload); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Invalid payload for operation %d: %s", i, err.Error()))
				return
			}
			// Validate that the timeout is a valid duration
			if _, err := time.ParseDuration(payload.Timeout); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Invalid timeout for operation %d: %s", i, err.Error()))
				return
			}
		default:
			writeError(w, http.StatusBadRequest, "invalid_operation_type", fmt.Sprintf("Unsupported operation type: %s", op.Type))
			return
		}
	}

	// Convert operations to fabric.ConfigUpdateOperation
	operations := make([]fabric.ConfigUpdateOperation, len(req.Operations))
	for i, op := range req.Operations {
		operations[i] = fabric.ConfigUpdateOperation{
			Type:    fabric.ConfigUpdateOperationType(op.Type),
			Payload: op.Payload,
		}
	}

	// Call service to prepare config update
	proposal, err := h.networkService.UpdateFabricNetwork(r.Context(), networkID, operations)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "prepare_config_update_failed", err.Error())
		return
	}

	// Create response
	resp := ConfigUpdateResponse{
		ID:          proposal.ID,
		NetworkID:   proposal.NetworkID,
		ChannelName: proposal.ChannelName,
		Status:      proposal.Status,
		CreatedAt:   proposal.CreatedAt,
		CreatedBy:   proposal.CreatedBy,
		Operations:  req.Operations,
	}

	// Return response
	writeJSON(w, http.StatusOK, resp)
}

// ConfigUpdateResponse represents the response from preparing a config update
type ConfigUpdateResponse struct {
	ID          string                         `json:"id"`
	NetworkID   int64                          `json:"network_id"`
	ChannelName string                         `json:"channel_name"`
	Status      string                         `json:"status"`
	CreatedAt   time.Time                      `json:"created_at"`
	CreatedBy   string                         `json:"created_by"`
	Operations  []ConfigUpdateOperationRequest `json:"operations"`
	PreviewJSON string                         `json:"preview_json,omitempty"`
}

// @Summary Get list of blocks from Fabric network
// @Description Get a paginated list of blocks from a Fabric network
// @Tags Fabric Networks
// @Produce json
// @Param id path int true "Network ID"
// @Param limit query int false "Number of blocks to return (default: 10)"
// @Param offset query int false "Number of blocks to skip (default: 0)"
// @Param reverse query bool false "Get blocks in reverse order (default: false)"
// @Success 200 {object} BlockListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/blocks [get]
func (h *Handler) FabricGetBlocks(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	// Parse pagination parameters
	limit := int32(10) // Default limit
	offset := int32(0) // Default offset
	reverse := false   // Default order

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limitInt, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_limit", "Invalid limit parameter")
			return
		}
		limit = int32(limitInt)
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offsetInt, err := strconv.ParseInt(offsetStr, 10, 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_offset", "Invalid offset parameter")
			return
		}
		offset = int32(offsetInt)
	}

	if reverseStr := r.URL.Query().Get("reverse"); reverseStr != "" {
		reverseBool, err := strconv.ParseBool(reverseStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_reverse", "Invalid reverse parameter")
			return
		}
		reverse = reverseBool
	}

	blocks, total, err := h.networkService.GetFabricBlocks(r.Context(), networkID, limit, offset, reverse)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_blocks_failed", err.Error())
		return
	}

	resp := BlockListResponse{
		Blocks: blocks,
		Total:  total,
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Get transactions from a specific block
// @Description Get all transactions from a specific block in a Fabric network
// @Tags Fabric Networks
// @Produce json
// @Param id path int true "Network ID"
// @Param blockNum path int true "Block Number"
// @Success 200 {object} BlockTransactionsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/blocks/{blockNum} [get]
func (h *Handler) FabricGetBlock(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	blockNum, err := strconv.ParseUint(chi.URLParam(r, "blockNum"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_block_number", "Invalid block number")
		return
	}

	blck, err := h.networkService.GetFabricBlock(r.Context(), networkID, blockNum)
	if err != nil {
		if err.Error() == "block not found" {
			writeError(w, http.StatusNotFound, "block_not_found", "Block not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_transactions_failed", err.Error())
		return
	}

	resp := BlockTransactionsResponse{
		Block: blck,
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Get transaction details by transaction ID
// @Description Get detailed information about a specific transaction in a Fabric network
// @Tags Fabric Networks
// @Produce json
// @Param id path int true "Network ID"
// @Param txId path string true "Transaction ID"
// @Success 200 {object} TransactionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/transactions/{txId} [get]
func (h *Handler) FabricGetTransaction(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	txID := chi.URLParam(r, "txId")
	if txID == "" {
		writeError(w, http.StatusBadRequest, "invalid_transaction_id", "Invalid transaction ID")
		return
	}

	blck, err := h.networkService.GetFabricBlockByTransaction(r.Context(), networkID, txID)
	if err != nil {
		if err.Error() == "transaction not found" {
			writeError(w, http.StatusNotFound, "transaction_not_found", "Transaction not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_transaction_failed", err.Error())
		return
	}

	resp := TransactionResponse{
		Block: blck,
	}
	writeJSON(w, http.StatusOK, resp)
}

// UpdateOrganizationCRLRequest represents the request to update an organization's CRL
type UpdateOrganizationCRLRequest struct {
	OrganizationID int64 `json:"organizationId" validate:"required"`
}

// UpdateOrganizationCRLResponse represents the response from updating an organization's CRL
type UpdateOrganizationCRLResponse struct {
	TransactionID string `json:"transactionId"`
}

// @Summary Update organization CRL
// @Description Update the Certificate Revocation List (CRL) for an organization in the network
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param id path int true "Network ID"
// @Param request body UpdateOrganizationCRLRequest true "Organization CRL update request"
// @Success 200 {object} UpdateOrganizationCRLResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/organization-crl [post]
func (h *Handler) UpdateOrganizationCRL(w http.ResponseWriter, r *http.Request) {
	// Parse network ID from URL
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	// Parse request body
	var req UpdateOrganizationCRLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate request
	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}

	// Update CRL using network service
	txID, err := h.networkService.UpdateOrganizationCRL(r.Context(), networkID, req.OrganizationID)
	if err != nil {
		if err.Error() == "organization not found" {
			writeError(w, http.StatusNotFound, "org_not_found", "Organization not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_crl_failed", err.Error())
		return
	}

	// Return response
	resp := UpdateOrganizationCRLResponse{
		TransactionID: txID,
	}
	writeJSON(w, http.StatusOK, resp)
}

// @Summary Get Fabric chain information
// @Description Retrieve detailed information about the Fabric blockchain including height and block hashes
// @Tags Fabric Networks
// @Accept json
// @Produce json
// @Param id path int true "Network ID"
// @Success 200 {object} ChainInfoResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabric/{id}/info [get]
func (h *Handler) GetChainInfo(w http.ResponseWriter, r *http.Request) {
	// Parse network ID from URL
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}

	// Get chain info from service layer
	chainInfo, err := h.networkService.GetFabricChainInfo(r.Context(), networkID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_chain_info_failed", err.Error())
		return
	}

	// Return response
	resp := ChainInfoResponse{
		Height:            chainInfo.Height,
		CurrentBlockHash:  chainInfo.CurrentBlockHash,
		PreviousBlockHash: chainInfo.PreviousBlockHash,
	}
	writeJSON(w, http.StatusOK, resp)
}

type ChainInfoResponse struct {
	Height            uint64 `json:"height"`
	CurrentBlockHash  string `json:"currentBlockHash"`
	PreviousBlockHash string `json:"previousBlockHash"`
}
