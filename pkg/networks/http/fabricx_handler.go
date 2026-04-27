package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chainlaunch/chainlaunch/pkg/networks/service"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
)

// CreateFabricXNetworkRequest is the HTTP payload for creating a FabricX network.
//
// Preconditions: each referenced orderer group / committer node must already
// exist and be in the initialized (created-but-not-started) state. This is the
// first of the two FabricX stages: the nodes are created first (generates
// certs/config), then the network is created referencing them (generates
// genesis), then each node is joined to the network (writes genesis + starts).
type CreateFabricXNetworkRequest struct {
	Name        string               `json:"name" validate:"required"`
	Description string               `json:"description"`
	Config      FabricXNetworkConfig `json:"config" validate:"required"`
}

// FabricXChannelID is the only channel ID supported by FabricX (Arma consensus).
// Enforced at request-validation time — any other value is a 400.
const FabricXChannelID = "arma"

// FabricXNetworkConfig is the on-wire representation of types.FabricXNetworkConfig.
// ChannelName must equal FabricXChannelID ("arma"). Empty is tolerated and
// auto-filled; any other value is rejected.
type FabricXNetworkConfig struct {
	ChannelName   string                `json:"channelName"`
	Organizations []FabricXOrganization `json:"organizations" validate:"required,min=1"`
	// LocalDev enables Docker Desktop (macOS/Windows) compatibility mode for
	// this network. Set to true when ChainLaunch itself runs on macOS/Windows
	// with Docker Desktop so containers can reach each other via
	// host.docker.internal and the host can dial published ports on 127.0.0.1.
	LocalDev bool `json:"localDev,omitempty"`
}

// FabricXOrganization binds an org to the specific orderer-group and committer
// node IDs that represent that org in this network. Exactly one of
// OrdererNodeGroupID (new ADR-0001 path) or OrdererNodeID (legacy monolithic
// path) must be provided. Committer may use CommitterNodeGroupID (new path)
// or the legacy CommitterNodeID; exactly one, or neither if the org has no
// committer-side role in this network.
type FabricXOrganization struct {
	ID                   int64 `json:"id" validate:"required"`
	OrdererNodeGroupID   int64 `json:"ordererNodeGroupId,omitempty"`
	OrdererNodeID        int64 `json:"ordererNodeId,omitempty"`
	CommitterNodeGroupID int64 `json:"committerNodeGroupId,omitempty"`
	CommitterNodeID      int64 `json:"committerNodeId,omitempty"`
}

// FabricXNetworkResponse is the HTTP response shape for a FabricX network.
type FabricXNetworkResponse struct {
	ID           int64           `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	Platform     string          `json:"platform"`
	Status       string          `json:"status"`
	ChannelName  string          `json:"channelName,omitempty"`
	Config       json.RawMessage `json:"config,omitempty"`
	GenesisBlock string          `json:"genesisBlock,omitempty"`
	CreatedAt    string          `json:"createdAt"`
	UpdatedAt    string          `json:"updatedAt,omitempty"`
}

// FabricXJoinNodeResponse is returned after a successful node-join (stage 2).
type FabricXJoinNodeResponse struct {
	NetworkID int64  `json:"networkId"`
	NodeID    int64  `json:"nodeId"`
	Status    string `json:"status"`
}

// @Summary Create a FabricX network
// @Description Creates a FabricX network and generates the Arma-consensus genesis block from the referenced orderer-group nodes. The nodes must already be created (certs/config generated) but not yet started.
// @Tags FabricX Networks
// @Accept json
// @Produce json
// @Param request body CreateFabricXNetworkRequest true "Network creation request"
// @Success 201 {object} FabricXNetworkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabricx [post]
func (h *Handler) FabricXNetworkCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateFabricXNetworkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}

	// FabricX supports exactly one channel ID: "arma". Accept empty (auto-fill)
	// and reject any other explicit value. This matches the Arma-consensus
	// constraint enforced by the deployer when generating the genesis block.
	if req.Config.ChannelName == "" {
		req.Config.ChannelName = FabricXChannelID
	} else if req.Config.ChannelName != FabricXChannelID {
		writeError(w, http.StatusBadRequest, "invalid_channel_name",
			"FabricX requires channelName=\""+FabricXChannelID+"\"; got \""+req.Config.ChannelName+"\"")
		return
	}

	orgs := make([]types.FabricXOrganization, 0, len(req.Config.Organizations))
	for i, o := range req.Config.Organizations {
		if o.OrdererNodeGroupID == 0 && o.OrdererNodeID == 0 {
			writeError(w, http.StatusBadRequest, "invalid_organization",
				"organization["+strconv.Itoa(i)+"] requires ordererNodeGroupId or ordererNodeId")
			return
		}
		if o.OrdererNodeGroupID != 0 && o.OrdererNodeID != 0 {
			writeError(w, http.StatusBadRequest, "invalid_organization",
				"organization["+strconv.Itoa(i)+"] must set only one of ordererNodeGroupId / ordererNodeId")
			return
		}
		if o.CommitterNodeGroupID != 0 && o.CommitterNodeID != 0 {
			writeError(w, http.StatusBadRequest, "invalid_organization",
				"organization["+strconv.Itoa(i)+"] must set only one of committerNodeGroupId / committerNodeId")
			return
		}
		orgs = append(orgs, types.FabricXOrganization{
			ID:                   o.ID,
			OrdererNodeGroupID:   o.OrdererNodeGroupID,
			OrdererNodeID:        o.OrdererNodeID,
			CommitterNodeGroupID: o.CommitterNodeGroupID,
			CommitterNodeID:      o.CommitterNodeID,
		})
	}

	cfg := &types.FabricXNetworkConfig{
		BaseNetworkConfig: types.BaseNetworkConfig{Type: types.NetworkTypeFabricX},
		ChannelName:       req.Config.ChannelName,
		Organizations:     orgs,
		LocalDev:          req.Config.LocalDev,
	}

	configBytes, err := json.Marshal(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "marshal_config_failed", err.Error())
		return
	}

	network, err := h.networkService.CreateNetwork(r.Context(), req.Name, req.Description, configBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_network_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, mapFabricXNetworkToResponse(*network))
}

// @Summary List FabricX networks
// @Tags FabricX Networks
// @Produce json
// @Success 200 {object} ListNetworksResponse
// @Router /networks/fabricx [get]
func (h *Handler) FabricXNetworkList(w http.ResponseWriter, r *http.Request) {
	limit := int32(50)
	offset := int32(0)
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 32); err == nil {
			limit = int32(parsed)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 32); err == nil {
			offset = int32(parsed)
		}
	}

	result, err := h.networkService.ListNetworks(r.Context(), service.ListNetworksParams{
		Limit:    limit,
		Offset:   offset,
		Platform: service.BlockchainTypeFabricX,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_networks_failed", err.Error())
		return
	}

	out := make([]FabricXNetworkResponse, len(result.Networks))
	for i, n := range result.Networks {
		out[i] = mapFabricXNetworkToResponse(n)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"networks": out,
		"total":    result.Total,
	})
}

// @Summary Get a FabricX network by ID
// @Tags FabricX Networks
// @Produce json
// @Param id path int true "Network ID"
// @Success 200 {object} FabricXNetworkResponse
// @Router /networks/fabricx/{id} [get]
func (h *Handler) FabricXNetworkGet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	network, err := h.networkService.GetNetwork(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_network_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mapFabricXNetworkToResponse(*network))
}

// @Summary Delete a FabricX network
// @Tags FabricX Networks
// @Param id path int true "Network ID"
// @Success 204
// @Router /networks/fabricx/{id} [delete]
func (h *Handler) FabricXNetworkDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	if err := h.networkService.DeleteNetwork(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_network_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary Get nodes of a FabricX network
// @Tags FabricX Networks
// @Produce json
// @Param id path int true "Network ID"
// @Success 200 {object} GetNetworkNodesResponse
// @Router /networks/fabricx/{id}/nodes [get]
func (h *Handler) FabricXNetworkGetNodes(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	nodes, err := h.networkService.GetNetworkNodes(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_network_nodes_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, GetNetworkNodesResponse{Nodes: nodes})
}

// @Summary Join a FabricX node to a network (stage 2)
// @Description Writes the network genesis block into the node's config dirs and starts the node. The node must already be created and initialized (stage 1), and the network must already have its genesis block.
// @Tags FabricX Networks
// @Param id path int true "Network ID"
// @Param nodeId path int true "Node ID (orderer group or committer)"
// @Success 200 {object} FabricXJoinNodeResponse
// @Router /networks/fabricx/{id}/nodes/{nodeId}/join [post]
func (h *Handler) FabricXNetworkJoinNode(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	nodeID, err := strconv.ParseInt(chi.URLParam(r, "nodeId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_node_id", "Invalid node ID")
		return
	}
	if err := h.networkService.JoinFabricXNodeToNetwork(r.Context(), networkID, nodeID); err != nil {
		writeError(w, http.StatusInternalServerError, "join_node_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, FabricXJoinNodeResponse{
		NetworkID: networkID,
		NodeID:    nodeID,
		Status:    "joined",
	})
}

func mapFabricXNetworkToResponse(n service.Network) FabricXNetworkResponse {
	var channelName string
	if n.Config != nil {
		var parsed types.FabricXNetworkConfig
		if err := json.Unmarshal(n.Config, &parsed); err == nil {
			channelName = parsed.ChannelName
		}
	}
	var updatedAt string
	if n.UpdatedAt != nil {
		updatedAt = n.UpdatedAt.Format(time.RFC3339)
	}
	return FabricXNetworkResponse{
		ID:           n.ID,
		Name:         n.Name,
		Description:  n.Description,
		Platform:     n.Platform,
		Status:       string(n.Status),
		ChannelName:  channelName,
		Config:       n.Config,
		GenesisBlock: n.GenesisBlock,
		CreatedAt:    n.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    updatedAt,
	}
}
