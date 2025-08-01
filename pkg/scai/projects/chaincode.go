package projects

import (
	"context"
	"fmt"
	"strings"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	networksservice "github.com/chainlaunch/chainlaunch/pkg/networks/service"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-protos-go-apiv2/gateway"

	"google.golang.org/grpc/status"
)

// TransactionRequest represents the request structure for both invoke and query operations
type TransactionRequest struct {
	ProjectID int64    `json:"projectId"`
	Function  string   `json:"function"`
	Args      []string `json:"args"`
	OrgID     int64    `json:"orgId"`
	KeyID     int64    `json:"keyId"`
}

// InvokeTransactionResponse represents the response structure for invoke operations
type InvokeTransactionResponse struct {
	Status        string      `json:"status"`
	Message       string      `json:"message"`
	Project       string      `json:"project"`
	Function      string      `json:"function"`
	Args          []string    `json:"args"`
	Result        interface{} `json:"result"`
	Channel       string      `json:"channel"`
	Chaincode     string      `json:"chaincode"`
	BlockNumber   int64       `json:"blockNumber"`
	TransactionID string      `json:"transactionId"`
	Code          int32       `json:"code"`
}

// QueryTransactionResponse represents the response structure for query operations
type QueryTransactionResponse struct {
	Status    string      `json:"status"`
	Message   string      `json:"message"`
	Project   string      `json:"project"`
	Function  string      `json:"function"`
	Args      []string    `json:"args"`
	Result    interface{} `json:"result"`
	Channel   string      `json:"channel"`
	Chaincode string      `json:"chaincode"`
}

// ChaincodeService handles chaincode-related operations
type ChaincodeService struct {
	queries      *db.Queries
	logger       *logger.Logger
	projects     *ProjectsService
	networks     *networksservice.NetworkService
	nodesService *nodeservice.NodeService
}

// NewChaincodeService creates a new instance of ChaincodeService
func NewChaincodeService(
	queries *db.Queries,
	logger *logger.Logger,
	projects *ProjectsService,
	networks *networksservice.NetworkService,
	nodesService *nodeservice.NodeService,
) *ChaincodeService {
	return &ChaincodeService{
		queries:      queries,
		logger:       logger,
		projects:     projects,
		networks:     networks,
		nodesService: nodesService,
	}
}

// InvokeTransaction invokes a transaction on the specified chaincode project
func (s *ChaincodeService) InvokeTransaction(ctx context.Context, req TransactionRequest) (*InvokeTransactionResponse, error) {
	project, err := s.projects.GetProject(ctx, req.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	if project.NetworkID == nil {
		return nil, fmt.Errorf("project has no network ID")
	}

	// Get network details
	network, err := s.networks.GetNetwork(ctx, *project.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	allNodes, err := s.nodesService.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer node: %w", err)
	}
	var peerNode *nodeservice.NodeResponse
	for _, node := range allNodes.Items {
		if node.FabricPeer != nil && node.FabricPeer.OrganizationID == req.OrgID {
			peerNode = &node
			break
		}
	}
	if peerNode == nil {
		return nil, fmt.Errorf("no peer node found for organization %d", req.OrgID)
	}
	// Get peer node for the specified organization
	peer, err := s.nodesService.GetFabricPeer(ctx, peerNode.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer: %w", err)
	}

	// Get gateway and channel
	gatewayClient, peerConn, err := peer.GetGatewayClient(ctx, req.KeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	defer peerConn.Close()

	nw := gatewayClient.GetNetwork(network.Name)
	contract := nw.GetContract(project.Name)
	result, commit, err := contract.SubmitAsync(req.Function, client.WithArguments(req.Args...))
	// Prepare and submit transaction
	if err != nil {
		endorseError, ok := err.(*client.EndorseError)
		if ok {
			detailsStr := []string{}
			for _, detail := range status.Convert(err).Details() {
				switch detail := detail.(type) {
				case *gateway.ErrorDetail:
					detailsStr = append(detailsStr, fmt.Sprintf("- address: %s; mspId: %s; message: %s\n", detail.GetAddress(), detail.GetMspId(), detail.GetMessage()))

				}
			}
			return nil, fmt.Errorf("failed to submit transaction: %s (gRPC status: %s)",
				endorseError.TransactionError.Error(),
				strings.Join(detailsStr, "\n"))
		}
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	txStatus, err := commit.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction status: %w", err)
	}

	blockNumber := txStatus.BlockNumber
	transactionID := txStatus.TransactionID
	code := txStatus.Code
	return &InvokeTransactionResponse{
		Status:        "success",
		Message:       "Transaction submitted successfully",
		Project:       project.Name,
		Function:      req.Function,
		Args:          req.Args,
		Result:        string(result),
		Channel:       network.Name,
		Chaincode:     project.Name,
		BlockNumber:   int64(blockNumber),
		TransactionID: transactionID,
		Code:          int32(code),
	}, nil
}

// QueryTransaction queries the state of the specified chaincode project
func (s *ChaincodeService) QueryTransaction(ctx context.Context, req TransactionRequest) (*QueryTransactionResponse, error) {
	project, err := s.projects.GetProject(ctx, req.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	if project.NetworkID == nil {
		return nil, fmt.Errorf("project has no network ID")
	}

	// Get network details
	network, err := s.networks.GetNetwork(ctx, *project.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}
	allNodes, err := s.nodesService.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer node: %w", err)
	}
	var peerNode *nodeservice.NodeResponse
	for _, node := range allNodes.Items {
		if node.FabricPeer != nil && node.FabricPeer.OrganizationID == req.OrgID {
			peerNode = &node
			break
		}
	}
	// Get peer node for the specified organization
	peer, err := s.nodesService.GetFabricPeer(ctx, peerNode.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer: %w", err)
	}

	// Get gateway and channel
	gatewayClient, peerConn, err := peer.GetGatewayClient(ctx, req.KeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	defer peerConn.Close()

	nw := gatewayClient.GetNetwork(network.Name)
	contract := nw.GetContract(project.Name)

	// Prepare and evaluate transaction
	result, err := contract.EvaluateTransaction(req.Function, req.Args...)

	if err != nil {
		endorseError, ok := err.(*client.EndorseError)
		if ok {
			detailsStr := []string{}
			for _, detail := range status.Convert(err).Details() {
				switch detail := detail.(type) {
				case *gateway.ErrorDetail:
					detailsStr = append(detailsStr, fmt.Sprintf("- address: %s; mspId: %s; message: %s\n", detail.GetAddress(), detail.GetMspId(), detail.GetMessage()))

				}
			}
			return nil, fmt.Errorf("failed to submit transaction: %s (gRPC status: %s)",
				endorseError.TransactionError.Error(),
				strings.Join(detailsStr, "\n"))
		}
		statusError := status.Convert(err)
		if statusError != nil {
			detailsStr := []string{}
			for _, detail := range statusError.Details() {
				switch detail := detail.(type) {
				case *gateway.ErrorDetail:
					detailsStr = append(detailsStr, fmt.Sprintf("- address: %s; mspId: %s; message: %s",
						detail.GetAddress(),
						detail.GetMspId(),
						detail.GetMessage()))
				}
			}
			return nil, fmt.Errorf("failed to submit transaction: %s (gRPC status details: %s)",
				statusError.Message(),
				strings.Join(detailsStr, "\n"))
		}
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}

	return &QueryTransactionResponse{
		Status:    "success",
		Message:   "Query executed successfully",
		Project:   project.Name,
		Function:  req.Function,
		Args:      req.Args,
		Result:    string(result),
		Channel:   network.Name,
		Chaincode: project.Name,
	}, nil
}

type GetProjectMetadataResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	ProjectId int64  `json:"projectId"`
	Metadata  string `json:"metadata"`
	Channel   string `json:"channel"`
}

func (s *ChaincodeService) GetProjectMetadata(ctx context.Context, projectID int64) (*QueryTransactionResponse, error) {

	// Use the chaincode's network name as the channel
	project, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chaincode: %w", err)
	}
	if project == nil {
		return nil, fmt.Errorf("project not found")
	}
	networkID := project.NetworkID
	if !networkID.Valid {
		return nil, fmt.Errorf("project has no network ID")
	}
	// Get all nodes for the network and select a peer
	networkNodeResp, err := s.networks.GetNetworkNodes(ctx, networkID.Int64)
	if err != nil {
		s.logger.Error("Failed to get nodes for network", "error", err)
		return nil, fmt.Errorf("failed to get nodes for network: %w", err)
	}
	var keyID int64
	var orgID int64
	foundPeer := false
	for _, node := range networkNodeResp {
		if node.Node.NodeType == "FABRIC_PEER" && node.Node.Platform == "FABRIC" && node.Node.FabricPeer != nil {
			orgID = node.Node.FabricPeer.OrganizationID
			keyID, err = s.nodesService.GetFabricClientIdentityForOrganization(ctx, orgID)
			if err != nil {
				s.logger.Error("Failed to get organization", "error", err)
				return nil, fmt.Errorf("failed to get organization: %w", err)
			}
			foundPeer = true
			break
		}
	}
	if !foundPeer {
		return nil, fmt.Errorf("no peer found for network")
	}
	// Get network and contract
	metadataResult, err := s.QueryTransaction(ctx, TransactionRequest{
		ProjectID: projectID,
		Function:  "org.hyperledger.fabric:GetMetadata",
		Args:      []string{},
		OrgID:     orgID,
		KeyID:     keyID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	return metadataResult, nil
}
