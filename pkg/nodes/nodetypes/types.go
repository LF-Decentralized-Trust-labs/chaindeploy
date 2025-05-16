package nodetypes

import (
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// Node represents a node with its full configuration
type Node struct {
	ID                 int64                      `json:"id"`
	Name               string                     `json:"name"`
	BlockchainPlatform types.BlockchainPlatform   `json:"platform"`
	NodeType           types.NodeType             `json:"nodeType"`
	Status             types.NodeStatus           `json:"status"`
	ErrorMessage       string                     `json:"errorMessage"`
	Endpoint           string                     `json:"endpoint"`
	PublicEndpoint     string                     `json:"publicEndpoint"`
	NodeConfig         types.NodeConfig           `json:"nodeConfig"`
	DeploymentConfig   types.NodeDeploymentConfig `json:"deploymentConfig"`
	MSPID              string                     `json:"mspId"`
	CreatedAt          time.Time                  `json:"createdAt"`
	UpdatedAt          time.Time                  `json:"updatedAt"`
}
