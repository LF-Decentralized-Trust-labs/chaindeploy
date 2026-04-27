package template

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	fabricservice "github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	keymanagement "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	"github.com/hyperledger/fabric-config/configtx"
)

// TemplateService handles network template operations
type TemplateService struct {
	db          *db.Queries
	nodeService *nodeservice.NodeService
	orgService  *fabricservice.OrganizationService
	keyMgmt     *keymanagement.KeyManagementService
	logger      *logger.Logger
	instanceID  string
}

// NewTemplateService creates a new TemplateService
func NewTemplateService(
	db *db.Queries,
	nodeService *nodeservice.NodeService,
	orgService *fabricservice.OrganizationService,
	keyMgmt *keymanagement.KeyManagementService,
	logger *logger.Logger,
) *TemplateService {
	return &TemplateService{
		db:          db,
		nodeService: nodeService,
		orgService:  orgService,
		keyMgmt:     keyMgmt,
		logger:      logger,
	}
}

// SetInstanceID sets the instance identifier for exports
func (s *TemplateService) SetInstanceID(instanceID string) {
	s.instanceID = instanceID
}

// ExportNetworkTemplate exports a network configuration as a reusable V2 template
func (s *TemplateService) ExportNetworkTemplate(ctx context.Context, networkID int64) (*NetworkTemplate, error) {
	network, err := s.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	tmpl := &NetworkTemplate{
		Version:      TemplateVersion,
		ExportedAt:   time.Now().UTC().Format(time.RFC3339),
		ExportedFrom: s.instanceID,
		Network: NetworkDefinition{
			Name:        network.Name,
			Description: network.Description.String,
			Platform:    network.Platform,
		},
	}

	if !network.Config.Valid {
		return nil, fmt.Errorf("network has no configuration stored")
	}

	switch network.Platform {
	case "fabric":
		var fabricConfig types.FabricNetworkConfig
		if err := json.Unmarshal([]byte(network.Config.String), &fabricConfig); err != nil {
			return nil, fmt.Errorf("failed to parse network config: %w", err)
		}
		fabricTemplate, variables, err := s.buildFabricTemplate(ctx, &fabricConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build fabric template: %w", err)
		}
		tmpl.Variables = variables
		tmpl.Network.Fabric = fabricTemplate

	case "besu":
		var besuConfig types.BesuNetworkConfig
		if err := json.Unmarshal([]byte(network.Config.String), &besuConfig); err != nil {
			return nil, fmt.Errorf("failed to parse network config: %w", err)
		}
		besuTemplate, variables, err := s.buildBesuTemplate(ctx, &besuConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build besu template: %w", err)
		}
		tmpl.Variables = variables
		tmpl.Network.Besu = besuTemplate

	default:
		return nil, fmt.Errorf("unsupported platform: %s", network.Platform)
	}

	return tmpl, nil
}

// buildFabricTemplate builds the Fabric-specific network template with variables
func (s *TemplateService) buildFabricTemplate(ctx context.Context, config *types.FabricNetworkConfig) (*FabricNetworkTemplate, []TemplateVariable, error) {
	tmpl := &FabricNetworkTemplate{
		ChannelName:             config.ChannelName,
		ConsensusType:           config.ConsensusType,
		ChannelCapabilities:     config.ChannelCapabilities,
		ApplicationCapabilities: config.ApplicationCapabilities,
		OrdererCapabilities:     config.OrdererCapabilities,
	}

	if config.BatchSize != nil {
		tmpl.BatchSize = &BatchSizeTemplate{
			MaxMessageCount:   config.BatchSize.MaxMessageCount,
			AbsoluteMaxBytes:  config.BatchSize.AbsoluteMaxBytes,
			PreferredMaxBytes: config.BatchSize.PreferredMaxBytes,
		}
	}
	tmpl.BatchTimeout = config.BatchTimeout

	if config.EtcdRaftOptions != nil {
		tmpl.EtcdRaftOptions = &EtcdRaftOptionsTemplate{
			TickInterval:         config.EtcdRaftOptions.TickInterval,
			ElectionTick:         config.EtcdRaftOptions.ElectionTick,
			HeartbeatTick:        config.EtcdRaftOptions.HeartbeatTick,
			MaxInflightBlocks:    config.EtcdRaftOptions.MaxInflightBlocks,
			SnapshotIntervalSize: config.EtcdRaftOptions.SnapshotIntervalSize,
		}
	}

	if config.SmartBFTOptions != nil {
		tmpl.SmartBFTOptions = &SmartBFTOptionsTemplate{
			RequestBatchMaxCount:      config.SmartBFTOptions.RequestBatchMaxCount,
			RequestBatchMaxBytes:      config.SmartBFTOptions.RequestBatchMaxBytes,
			RequestBatchMaxInterval:   config.SmartBFTOptions.RequestBatchMaxInterval,
			IncomingMessageBufferSize: config.SmartBFTOptions.IncomingMessageBufferSize,
			RequestPoolSize:           config.SmartBFTOptions.RequestPoolSize,
			RequestForwardTimeout:     config.SmartBFTOptions.RequestForwardTimeout,
			RequestComplainTimeout:    config.SmartBFTOptions.RequestComplainTimeout,
			RequestAutoRemoveTimeout:  config.SmartBFTOptions.RequestAutoRemoveTimeout,
			RequestMaxBytes:           config.SmartBFTOptions.RequestMaxBytes,
			ViewChangeResendInterval:  config.SmartBFTOptions.ViewChangeResendInterval,
			ViewChangeTimeout:         config.SmartBFTOptions.ViewChangeTimeout,
			LeaderHeartbeatTimeout:    config.SmartBFTOptions.LeaderHeartbeatTimeout,
			LeaderHeartbeatCount:      config.SmartBFTOptions.LeaderHeartbeatCount,
			CollectTimeout:            config.SmartBFTOptions.CollectTimeout,
			SyncOnStart:               config.SmartBFTOptions.SyncOnStart,
			SpeedUpViewChange:         config.SmartBFTOptions.SpeedUpViewChange,
			LeaderRotation:            config.SmartBFTOptions.LeaderRotation,
			DecisionsPerLeader:        config.SmartBFTOptions.DecisionsPerLeader,
		}
	}

	// Fetch all Fabric nodes once
	fabricPlatform := nodetypes.PlatformFabric
	allNodesResult, err := s.nodeService.ListNodes(ctx, &fabricPlatform, 1, 1000)
	if err != nil {
		s.logger.Warnf("Failed to list nodes: %v", err)
		allNodesResult = &nodeservice.PaginatedNodes{Items: []nodeservice.NodeResponse{}}
	}

	var variables []TemplateVariable
	mspIDToVar := make(map[string]string)

	// Build peer organization variables and refs
	peerOrgRefs, peerVars, peerMspMap, err := s.buildOrgVariablesAndRefs(ctx, config.PeerOrganizations, "peerOrg", "peer", allNodesResult.Items)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build peer org variables: %w", err)
	}
	tmpl.PeerOrgRefs = peerOrgRefs
	variables = append(variables, peerVars...)
	for k, v := range peerMspMap {
		mspIDToVar[k] = v
	}

	// Build orderer organization variables and refs
	ordererOrgRefs, ordererVars, ordererMspMap, err := s.buildOrgVariablesAndRefs(ctx, config.OrdererOrganizations, "ordererOrg", "orderer", allNodesResult.Items)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build orderer org variables: %w", err)
	}
	tmpl.OrdererOrgRefs = ordererOrgRefs
	variables = append(variables, ordererVars...)
	for k, v := range ordererMspMap {
		mspIDToVar[k] = v
	}

	// Convert policies with variable placeholders
	tmpl.ApplicationPolicies = convertConfigtxPoliciesToV2(config.ApplicationPolicies, mspIDToVar)
	tmpl.OrdererPolicies = convertConfigtxPoliciesToV2(config.OrdererPolicies, mspIDToVar)
	tmpl.ChannelPolicies = convertConfigtxPoliciesToV2(config.ChannelPolicies, mspIDToVar)

	return tmpl, variables, nil
}

// buildBesuTemplate builds the Besu-specific network template with variables
func (s *TemplateService) buildBesuTemplate(ctx context.Context, config *types.BesuNetworkConfig) (*BesuNetworkTemplate, []TemplateVariable, error) {
	tmpl := &BesuNetworkTemplate{
		Consensus:      string(config.Consensus),
		ChainID:        config.ChainID,
		BlockPeriod:    config.BlockPeriod,
		EpochLength:    config.EpochLength,
		RequestTimeout: config.RequestTimeout,
		GasLimit:       config.GasLimit,
		Difficulty:     config.Difficulty,
	}

	// Copy alloc
	if config.Alloc != nil {
		tmpl.Alloc = make(map[string]AllocTemplate)
		for addr, balance := range config.Alloc {
			tmpl.Alloc[addr] = AllocTemplate{Balance: balance.Balance}
		}
	}

	// Build validator variables
	var variables []TemplateVariable
	for i, keyID := range config.InitialValidatorKeyIds {
		varName := fmt.Sprintf("validator%d", i+1)

		key, err := s.keyMgmt.GetKey(ctx, int(keyID))
		if err != nil {
			s.logger.Warnf("Failed to get key %d: %v", keyID, err)
			continue
		}

		desc := fmt.Sprintf("Validator key %d", i+1)
		if key != nil {
			desc = fmt.Sprintf("Validator key (original: %s)", key.Name)
		}

		validatorVar := CreateValidatorVariable(varName, desc)
		variables = append(variables, validatorVar)

		tmpl.ValidatorRefs = append(tmpl.ValidatorRefs, ValidatorRef{
			VariableRef: varName,
		})
	}

	return tmpl, variables, nil
}

// buildOrgVariablesAndRefs builds organization variables and references for templates
func (s *TemplateService) buildOrgVariablesAndRefs(
	ctx context.Context,
	orgs []types.Organization,
	varPrefix string,
	orgType string,
	allNodes []nodeservice.NodeResponse,
) ([]OrganizationRef, []TemplateVariable, map[string]string, error) {
	refs := make([]OrganizationRef, 0, len(orgs))
	vars := make([]TemplateVariable, 0)
	mspIDToVar := make(map[string]string)

	for i, org := range orgs {
		orgDTO, err := s.orgService.GetOrganization(ctx, org.ID)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get organization %d: %w", org.ID, err)
		}

		varName := fmt.Sprintf("%s%d", varPrefix, i+1)

		orgVar := CreateOrganizationVariable(
			varName,
			fmt.Sprintf("%s organization (original: %s)", orgType, orgDTO.MspID),
			[]string{"fabric"},
		)
		vars = append(vars, orgVar)

		mspIDToVar[orgDTO.MspID] = varName

		nodeRefs := make([]NodeRef, 0)
		nodeCount := 0

		for _, node := range allNodes {
			var nodeOrgID int64
			if node.FabricPeer != nil {
				nodeOrgID = node.FabricPeer.OrganizationID
			} else if node.FabricOrderer != nil {
				nodeOrgID = node.FabricOrderer.OrganizationID
			}

			if nodeOrgID != org.ID {
				continue
			}

			if orgType == "peer" && node.NodeType != nodetypes.NodeTypeFabricPeer {
				continue
			}
			if orgType == "orderer" && node.NodeType != nodetypes.NodeTypeFabricOrderer {
				continue
			}

			nodeCount++
			nodeVarName := fmt.Sprintf("%s_%s%d", varName, orgType, nodeCount)

			nodeRequired := orgType == "orderer"
			nodeVar := CreateNodeVariable(
				nodeVarName,
				fmt.Sprintf("%s node (original: %s)", orgType, node.Name),
				[]string{"fabric"},
				nodeRequired,
			)
			vars = append(vars, nodeVar)

			nodeRefs = append(nodeRefs, NodeRef{
				VariableRef: nodeVarName,
			})
		}

		refs = append(refs, OrganizationRef{
			VariableRef: varName,
			NodeRefs:    nodeRefs,
		})
	}

	return refs, vars, mspIDToVar, nil
}

// convertConfigtxPoliciesToV2 converts configtx.Policy map to PolicyTemplate map with variable placeholders
func convertConfigtxPoliciesToV2(policies map[string]configtx.Policy, mspIDToVar map[string]string) map[string]PolicyTemplate {
	if policies == nil {
		return nil
	}

	result := make(map[string]PolicyTemplate)
	for name, policy := range policies {
		rule := policy.Rule
		for mspID, varName := range mspIDToVar {
			rule = replaceMspIDWithVariable(rule, mspID, varName)
		}
		result[name] = PolicyTemplate{
			Type: policy.Type,
			Rule: rule,
		}
	}

	return result
}

func replaceMspIDWithVariable(rule, mspID, varName string) string {
	suffixes := []string{".member", ".admin", ".peer", ".client"}
	for _, suffix := range suffixes {
		old := "'" + mspID + suffix + "'"
		newStr := "'${" + varName + ".mspId}" + suffix + "'"
		rule = strings.ReplaceAll(rule, old, newStr)
	}
	return rule
}
