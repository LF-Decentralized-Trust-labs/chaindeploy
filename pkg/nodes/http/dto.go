package http

import (
	"github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// CreateNodeRequest represents the HTTP request to create a node
// @Description Request payload for creating a new node
type CreateNodeRequest struct {
	// @Description Name of the node
	Name string `json:"name" validate:"required" example:"peer0-org1"`
	// @Description Blockchain platform (fabric or besu)
	BlockchainPlatform types.BlockchainPlatform `json:"blockchainPlatform" validate:"required" example:"fabric"`
	// @Description Fabric peer configuration, required when creating a Fabric peer node
	FabricPeer *types.FabricPeerConfig `json:"fabricPeer,omitempty"`
	// @Description Fabric orderer configuration, required when creating a Fabric orderer node
	FabricOrderer *types.FabricOrdererConfig `json:"fabricOrderer,omitempty"`
	// @Description Besu node configuration, required when creating a Besu node
	BesuNode *types.BesuNodeConfig `json:"besuNode,omitempty"`
	// @Description Fabric X orderer group configuration, required when creating a FabricX orderer group node
	FabricXOrdererGroup *types.FabricXOrdererGroupConfig `json:"fabricXOrdererGroup,omitempty"`
	// @Description Fabric X committer configuration, required when creating a FabricX committer node
	FabricXCommitter *types.FabricXCommitterConfig `json:"fabricXCommitter,omitempty"`
}

// PaginatedNodesResponse represents the HTTP response for a paginated list of nodes
type PaginatedNodesResponse struct {
	Items       []NodeResponse `json:"items"`
	Total       int64          `json:"total"`
	Page        int            `json:"page"`
	PageCount   int            `json:"pageCount"`
	HasNextPage bool           `json:"hasNextPage"`
}

func toPaginatedNodesResponse(paginated *service.PaginatedNodes) *PaginatedNodesResponse {
	response := &PaginatedNodesResponse{
		Items:       make([]NodeResponse, len(paginated.Items)),
		Total:       paginated.Total,
		Page:        paginated.Page,
		PageCount:   paginated.PageCount,
		HasNextPage: paginated.HasNextPage,
	}

	for i, node := range paginated.Items {
		response.Items[i] = toNodeResponse(&node)
	}

	return response
}
