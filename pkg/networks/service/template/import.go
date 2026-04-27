package template

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
	"github.com/hyperledger/fabric-config/configtx"
)

// NetworkCreator interface for creating networks
type NetworkCreator interface {
	CreateNetwork(ctx context.Context, name, description string, configData []byte) (interface{}, error)
}

// ChaincodeCreator interface for creating chaincodes after network import
type ChaincodeCreator interface {
	CreateChaincode(ctx context.Context, name string, networkID int64) (int64, error)
	CreateChaincodeDefinition(ctx context.Context, chaincodeID int64, version string, sequence int64, dockerImage string, endorsementPolicy string, chaincodeAddress string) (int64, error)
}

// ImportFromTemplate imports a network from a template
func (s *TemplateService) ImportFromTemplate(ctx context.Context, req *ImportTemplateRequest, networkCreator NetworkCreator, chaincodeCreator ChaincodeCreator) (*ImportTemplateResponse, error) {
	resolver := NewVariableResolver(s.orgService, s.nodeService, s.keyMgmt)

	varCtx, err := resolver.ResolveBindings(ctx, req.Template.Variables, req.VariableBindings)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve variable bindings: %w", err)
	}

	response := &ImportTemplateResponse{
		CreatedOrganizations: []CreatedOrg{},
		CreatedNodes:         []CreatedNode{},
	}

	var networkID int64

	switch req.Template.Network.Platform {
	case "fabric":
		networkID, err = s.importFabricNetwork(ctx, req, varCtx, resolver, networkCreator, response)
	case "besu":
		networkID, err = s.importBesuNetwork(ctx, req, varCtx, networkCreator, response)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", req.Template.Network.Platform)
	}

	if err != nil {
		return nil, err
	}

	response.NetworkID = networkID

	// Create chaincodes if template includes them
	if chaincodeCreator != nil && len(req.Template.Chaincodes) > 0 {
		for _, ccTmpl := range req.Template.Chaincodes {
			if ccTmpl.Platform != req.Template.Network.Platform {
				continue
			}

			ccID, err := chaincodeCreator.CreateChaincode(ctx, ccTmpl.Name, networkID)
			if err != nil {
				s.logger.Warnf("Failed to create chaincode '%s': %v", ccTmpl.Name, err)
				continue
			}

			created := CreatedChaincode{
				Name:        ccTmpl.Name,
				ChaincodeID: ccID,
				Platform:    ccTmpl.Platform,
			}

			// Create chaincode definition if available
			if ccTmpl.Fabric != nil {
				_, err := chaincodeCreator.CreateChaincodeDefinition(
					ctx, ccID,
					ccTmpl.Fabric.Version,
					ccTmpl.Fabric.Sequence,
					ccTmpl.Fabric.DockerImage,
					ccTmpl.Fabric.EndorsementPolicy,
					ccTmpl.Fabric.ChaincodeAddress,
				)
				if err != nil {
					s.logger.Warnf("Failed to create chaincode definition for '%s': %v", ccTmpl.Name, err)
				}
			}

			response.CreatedChaincodes = append(response.CreatedChaincodes, created)
		}
	}

	return response, nil
}

func (s *TemplateService) importFabricNetwork(ctx context.Context, req *ImportTemplateRequest, varCtx *VariableContext, resolver *VariableResolver, networkCreator NetworkCreator, response *ImportTemplateResponse) (int64, error) {
	fabricConfig, err := s.buildFabricNetworkConfig(ctx, req, varCtx, resolver)
	if err != nil {
		return 0, fmt.Errorf("failed to build network config: %w", err)
	}

	configData, err := json.Marshal(fabricConfig)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal network config: %w", err)
	}

	networkName := req.Template.Network.Name
	if req.Overrides.NetworkName != nil && *req.Overrides.NetworkName != "" {
		networkName = *req.Overrides.NetworkName
	}

	description := req.Template.Network.Description
	if req.Overrides.Description != nil {
		description = *req.Overrides.Description
	}

	network, err := networkCreator.CreateNetwork(ctx, networkName, description, configData)
	if err != nil {
		return 0, fmt.Errorf("failed to create network: %w", err)
	}

	response.NetworkName = networkName

	// Extract network ID
	var networkID int64
	if networkBytes, err := json.Marshal(network); err == nil {
		var netData struct {
			ID int64 `json:"id"`
		}
		if json.Unmarshal(networkBytes, &netData) == nil {
			networkID = netData.ID
		}
	}

	totalOrgs := len(fabricConfig.PeerOrganizations) + len(fabricConfig.OrdererOrganizations)
	totalNodes := 0
	for _, org := range fabricConfig.PeerOrganizations {
		totalNodes += len(org.NodeIDs)
	}
	for _, org := range fabricConfig.OrdererOrganizations {
		totalNodes += len(org.NodeIDs)
	}

	response.Message = fmt.Sprintf("Successfully created Fabric network '%s' using %d organizations and %d nodes",
		networkName, totalOrgs, totalNodes)

	return networkID, nil
}

func (s *TemplateService) importBesuNetwork(ctx context.Context, req *ImportTemplateRequest, varCtx *VariableContext, networkCreator NetworkCreator, response *ImportTemplateResponse) (int64, error) {
	besuConfig, err := s.buildBesuNetworkConfig(ctx, req, varCtx)
	if err != nil {
		return 0, fmt.Errorf("failed to build besu network config: %w", err)
	}

	configData, err := json.Marshal(besuConfig)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal besu network config: %w", err)
	}

	networkName := req.Template.Network.Name
	if req.Overrides.NetworkName != nil && *req.Overrides.NetworkName != "" {
		networkName = *req.Overrides.NetworkName
	}

	description := req.Template.Network.Description
	if req.Overrides.Description != nil {
		description = *req.Overrides.Description
	}

	network, err := networkCreator.CreateNetwork(ctx, networkName, description, configData)
	if err != nil {
		return 0, fmt.Errorf("failed to create besu network: %w", err)
	}

	response.NetworkName = networkName

	var networkID int64
	if networkBytes, err := json.Marshal(network); err == nil {
		var netData struct {
			ID int64 `json:"id"`
		}
		if json.Unmarshal(networkBytes, &netData) == nil {
			networkID = netData.ID
		}
	}

	validatorCount := len(req.Template.Network.Besu.ValidatorRefs)
	response.Message = fmt.Sprintf("Successfully created Besu network '%s' with %d validators",
		networkName, validatorCount)

	return networkID, nil
}

// buildFabricNetworkConfig builds the FabricNetworkConfig from a template with variable bindings
func (s *TemplateService) buildFabricNetworkConfig(ctx context.Context, req *ImportTemplateRequest, varCtx *VariableContext, resolver *VariableResolver) (*types.FabricNetworkConfig, error) {
	if req.Template.Network.Fabric == nil {
		return nil, fmt.Errorf("fabric configuration is required")
	}

	templateFabric := req.Template.Network.Fabric

	varDefs := make(map[string]TemplateVariable)
	for _, v := range req.Template.Variables {
		varDefs[v.Name] = v
	}

	channelName := templateFabric.ChannelName
	if req.Overrides.ChannelName != nil && *req.Overrides.ChannelName != "" {
		channelName = *req.Overrides.ChannelName
	}

	config := &types.FabricNetworkConfig{
		BaseNetworkConfig: types.BaseNetworkConfig{
			Type: types.NetworkTypeFabric,
		},
		ChannelName:             channelName,
		ConsensusType:           templateFabric.ConsensusType,
		ChannelCapabilities:     templateFabric.ChannelCapabilities,
		ApplicationCapabilities: templateFabric.ApplicationCapabilities,
		OrdererCapabilities:     templateFabric.OrdererCapabilities,
	}

	if templateFabric.BatchSize != nil {
		config.BatchSize = &types.BatchSize{
			MaxMessageCount:   templateFabric.BatchSize.MaxMessageCount,
			AbsoluteMaxBytes:  templateFabric.BatchSize.AbsoluteMaxBytes,
			PreferredMaxBytes: templateFabric.BatchSize.PreferredMaxBytes,
		}
	}
	config.BatchTimeout = templateFabric.BatchTimeout

	if templateFabric.EtcdRaftOptions != nil {
		config.EtcdRaftOptions = &types.EtcdRaftOptions{
			TickInterval:         templateFabric.EtcdRaftOptions.TickInterval,
			ElectionTick:         templateFabric.EtcdRaftOptions.ElectionTick,
			HeartbeatTick:        templateFabric.EtcdRaftOptions.HeartbeatTick,
			MaxInflightBlocks:    templateFabric.EtcdRaftOptions.MaxInflightBlocks,
			SnapshotIntervalSize: templateFabric.EtcdRaftOptions.SnapshotIntervalSize,
		}
	}

	if templateFabric.SmartBFTOptions != nil {
		config.SmartBFTOptions = &types.SmartBFTOptions{
			RequestBatchMaxCount:      templateFabric.SmartBFTOptions.RequestBatchMaxCount,
			RequestBatchMaxBytes:      templateFabric.SmartBFTOptions.RequestBatchMaxBytes,
			RequestBatchMaxInterval:   templateFabric.SmartBFTOptions.RequestBatchMaxInterval,
			IncomingMessageBufferSize: templateFabric.SmartBFTOptions.IncomingMessageBufferSize,
			RequestPoolSize:           templateFabric.SmartBFTOptions.RequestPoolSize,
			RequestForwardTimeout:     templateFabric.SmartBFTOptions.RequestForwardTimeout,
			RequestComplainTimeout:    templateFabric.SmartBFTOptions.RequestComplainTimeout,
			RequestAutoRemoveTimeout:  templateFabric.SmartBFTOptions.RequestAutoRemoveTimeout,
			RequestMaxBytes:           templateFabric.SmartBFTOptions.RequestMaxBytes,
			ViewChangeResendInterval:  templateFabric.SmartBFTOptions.ViewChangeResendInterval,
			ViewChangeTimeout:         templateFabric.SmartBFTOptions.ViewChangeTimeout,
			LeaderHeartbeatTimeout:    templateFabric.SmartBFTOptions.LeaderHeartbeatTimeout,
			LeaderHeartbeatCount:      templateFabric.SmartBFTOptions.LeaderHeartbeatCount,
			CollectTimeout:            templateFabric.SmartBFTOptions.CollectTimeout,
			SyncOnStart:               templateFabric.SmartBFTOptions.SyncOnStart,
			SpeedUpViewChange:         templateFabric.SmartBFTOptions.SpeedUpViewChange,
			LeaderRotation:            templateFabric.SmartBFTOptions.LeaderRotation,
			DecisionsPerLeader:        templateFabric.SmartBFTOptions.DecisionsPerLeader,
		}
	}

	// Substitute variables in policies
	appPolicies, err := resolver.SubstituteInPolicies(templateFabric.ApplicationPolicies, varCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in application policies: %w", err)
	}
	config.ApplicationPolicies = convertTemplatePolicies(appPolicies)

	ordererPolicies, err := resolver.SubstituteInPolicies(templateFabric.OrdererPolicies, varCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in orderer policies: %w", err)
	}
	config.OrdererPolicies = convertTemplatePolicies(ordererPolicies)

	channelPolicies, err := resolver.SubstituteInPolicies(templateFabric.ChannelPolicies, varCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in channel policies: %w", err)
	}
	config.ChannelPolicies = convertTemplatePolicies(channelPolicies)

	// Build peer organizations
	config.PeerOrganizations = make([]types.Organization, 0)
	for _, orgRef := range templateFabric.PeerOrgRefs {
		resolved, ok := varCtx.Get(orgRef.VariableRef)
		if !ok || resolved.ResolvedOrgID == nil {
			return nil, fmt.Errorf("peer organization variable '%s' not resolved or has no org ID", orgRef.VariableRef)
		}

		org := types.Organization{
			ID:      *resolved.ResolvedOrgID,
			NodeIDs: []int64{},
		}

		for _, nodeRef := range orgRef.NodeRefs {
			nodeResolved, ok := varCtx.Get(nodeRef.VariableRef)
			if !ok || nodeResolved.ResolvedNodeID == nil {
				varDef, hasVarDef := varDefs[nodeRef.VariableRef]
				if hasVarDef && !varDef.Required {
					continue
				}
				return nil, fmt.Errorf("node variable '%s' not resolved or has no node ID", nodeRef.VariableRef)
			}
			org.NodeIDs = append(org.NodeIDs, *nodeResolved.ResolvedNodeID)
		}

		config.PeerOrganizations = append(config.PeerOrganizations, org)
	}

	// Build orderer organizations
	config.OrdererOrganizations = make([]types.Organization, 0)
	for _, orgRef := range templateFabric.OrdererOrgRefs {
		resolved, ok := varCtx.Get(orgRef.VariableRef)
		if !ok || resolved.ResolvedOrgID == nil {
			return nil, fmt.Errorf("orderer organization variable '%s' not resolved or has no org ID", orgRef.VariableRef)
		}

		org := types.Organization{
			ID:      *resolved.ResolvedOrgID,
			NodeIDs: []int64{},
		}

		for _, nodeRef := range orgRef.NodeRefs {
			nodeResolved, ok := varCtx.Get(nodeRef.VariableRef)
			if !ok || nodeResolved.ResolvedNodeID == nil {
				varDef, hasVarDef := varDefs[nodeRef.VariableRef]
				if hasVarDef && !varDef.Required {
					continue
				}
				return nil, fmt.Errorf("node variable '%s' not resolved or has no node ID", nodeRef.VariableRef)
			}
			org.NodeIDs = append(org.NodeIDs, *nodeResolved.ResolvedNodeID)
		}

		config.OrdererOrganizations = append(config.OrdererOrganizations, org)
	}

	return config, nil
}

// buildBesuNetworkConfig builds the BesuNetworkConfig from a template with variable bindings
func (s *TemplateService) buildBesuNetworkConfig(ctx context.Context, req *ImportTemplateRequest, varCtx *VariableContext) (*types.BesuNetworkConfig, error) {
	if req.Template.Network.Besu == nil {
		return nil, fmt.Errorf("besu configuration is required")
	}

	tmplBesu := req.Template.Network.Besu

	config := &types.BesuNetworkConfig{
		BaseNetworkConfig: types.BaseNetworkConfig{
			Type: types.NetworkTypeBesu,
		},
		ChainID:        tmplBesu.ChainID,
		Consensus:      types.BesuConsensusType(tmplBesu.Consensus),
		BlockPeriod:    tmplBesu.BlockPeriod,
		EpochLength:    tmplBesu.EpochLength,
		RequestTimeout: tmplBesu.RequestTimeout,
		GasLimit:       tmplBesu.GasLimit,
		Difficulty:     tmplBesu.Difficulty,
	}

	// Copy alloc
	if tmplBesu.Alloc != nil {
		config.Alloc = make(map[string]types.AccountBalance)
		for addr, alloc := range tmplBesu.Alloc {
			config.Alloc[addr] = types.AccountBalance{Balance: alloc.Balance}
		}
	}

	// Resolve validator key IDs from variable bindings
	for _, valRef := range tmplBesu.ValidatorRefs {
		resolved, ok := varCtx.Get(valRef.VariableRef)
		if !ok || resolved.ResolvedKeyID == nil {
			return nil, fmt.Errorf("validator variable '%s' not resolved or has no key ID", valRef.VariableRef)
		}
		config.InitialValidatorKeyIds = append(config.InitialValidatorKeyIds, *resolved.ResolvedKeyID)
	}

	return config, nil
}

// convertTemplatePolicies converts PolicyTemplate map back to configtx.Policy map
func convertTemplatePolicies(policies map[string]PolicyTemplate) map[string]configtx.Policy {
	if policies == nil {
		return nil
	}

	result := make(map[string]configtx.Policy)
	for name, policy := range policies {
		result[name] = configtx.Policy{
			Type: policy.Type,
			Rule: policy.Rule,
		}
	}

	return result
}
