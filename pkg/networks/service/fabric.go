package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/fabric"
	fabricblock "github.com/chainlaunch/chainlaunch/pkg/networks/service/fabric/block"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	"github.com/hyperledger/fabric-config/configtx"
	"github.com/sirupsen/logrus"
)

type AnchorPeer struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// UpdateOrganizationCRL updates the CRL for an organization in the network
func (s *NetworkService) UpdateOrganizationCRL(ctx context.Context, networkID, organizationID int64) (string, error) {
	// Get network details
	network, err := s.db.GetNetwork(ctx, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to get network: %w", err)
	}

	// Get deployer
	deployer, err := s.deployerFactory.GetDeployer(network.Platform)
	if err != nil {
		return "", fmt.Errorf("failed to get deployer: %w", err)
	}

	fabricDeployer, ok := deployer.(*fabric.FabricDeployer)
	if !ok {
		return "", fmt.Errorf("network %d is not a Fabric network", networkID)
	}

	// Update the CRL in the network
	txID, err := fabricDeployer.UpdateOrganizationCRL(ctx, networkID, fabric.UpdateOrganizationCRLInput{
		OrganizationID: organizationID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to update CRL: %w", err)
	}

	logrus.Info("Reloading network block after updating CRL, waiting 500ms")
	time.Sleep(500 * time.Millisecond)

	// Reload network block
	if err := s.ReloadFabricNetworkBlock(ctx, networkID); err != nil {
		logrus.Errorf("Failed to reload network block after updating CRL: %v", err)
	}

	return txID, nil
}

// UpdateFabricNetwork prepares a config update proposal for a Fabric network
func (s *NetworkService) UpdateFabricNetwork(ctx context.Context, networkID int64, operations []fabric.ConfigUpdateOperation) (*fabric.ConfigUpdateProposal, error) {
	// Get deployer for the network
	deployer, err := s.deployerFactory.GetDeployer(string(BlockchainTypeFabric))
	if err != nil {
		return nil, fmt.Errorf("failed to get deployer: %w", err)
	}

	// Assert that it's a Fabric deployer
	fabricDeployer, ok := deployer.(*fabric.FabricDeployer)
	if !ok {
		return nil, fmt.Errorf("network %d is not a Fabric network", networkID)
	}

	// Prepare the config update
	proposal, err := fabricDeployer.PrepareConfigUpdate(ctx, networkID, operations)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare config update: %w", err)
	}

	// Get organizations managed by us that can sign the config update
	orgs, err := s.db.ListFabricOrganizations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get network organizations: %w", err)
	}
	var signingOrgIDs []string
	for _, org := range orgs {
		signingOrgIDs = append(signingOrgIDs, org.MspID)
	}

	// Collect all available orderers for failover
	type ordererEntry struct {
		address string
		tlsCert string
	}
	var orderers []ordererEntry

	// Get orderers from network nodes (registry)
	networkNodes, err := s.GetNetworkNodes(ctx, networkID)
	if err != nil {
		s.logger.Warn("UpdateFabricNetwork: Failed to get network nodes", "error", err)
	} else {
		for _, node := range networkNodes {
			if node.Node.NodeType == nodetypes.NodeTypeFabricOrderer && node.Node.FabricOrderer != nil {
				orderers = append(orderers, ordererEntry{
					address: node.Node.FabricOrderer.ExternalEndpoint,
					tlsCert: node.Node.FabricOrderer.TLSCACert,
				})
			}
		}
	}

	// Get orderers from config block
	configBlock, err := fabricDeployer.GetCurrentChannelConfig(networkID)
	if err != nil {
		s.logger.Warn("UpdateFabricNetwork: Failed to get config block", "error", err)
	} else {
		configOrderers, err := fabricDeployer.GetOrderersFromConfigBlock(ctx, configBlock)
		if err != nil {
			s.logger.Warn("UpdateFabricNetwork: Failed to get orderers from config block", "error", err)
		} else {
			for _, o := range configOrderers {
				found := false
				for _, existing := range orderers {
					if existing.address == o.URL {
						found = true
						break
					}
				}
				if !found {
					orderers = append(orderers, ordererEntry{address: o.URL, tlsCert: o.TLSCert})
				}
			}
		}
	}

	if len(orderers) == 0 {
		return nil, fmt.Errorf("no orderers found for network %d", networkID)
	}

	s.logger.Info("UpdateFabricNetwork: Attempting to update channel config",
		"networkID", networkID,
		"availableOrderers", len(orderers),
	)

	// Try each orderer until one succeeds
	var lastErr error
	for i, orderer := range orderers {
		s.logger.Info("UpdateFabricNetwork: Trying orderer",
			"ordererIndex", i+1,
			"totalOrderers", len(orderers),
			"ordererURL", orderer.address,
		)

		res, err := fabricDeployer.UpdateChannelConfig(ctx, networkID, proposal.ConfigUpdateEnvelope, signingOrgIDs, orderer.address, orderer.tlsCert)
		if err == nil {
			s.logger.Info("UpdateFabricNetwork: Successfully updated channel config",
				"txID", res,
				"ordererURL", orderer.address,
			)
			return proposal, nil
		}

		lastErr = err
		s.logger.Warn("UpdateFabricNetwork: Failed with orderer, trying next",
			"ordererIndex", i+1,
			"ordererURL", orderer.address,
			"error", err,
		)
	}

	return nil, fmt.Errorf("failed to update channel config after trying all %d orderers: %w", len(orderers), lastErr)
}

func (s *NetworkService) GetFabricChainInfo(ctx context.Context, networkID int64) (*ChainInfo, error) {
	fabricDeployer, err := s.getFabricDeployerForNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get fabric deployer: %w", err)
	}
	chainInfo, err := fabricDeployer.GetChainInfo(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain info: %w", err)
	}
	return &ChainInfo{
		Height:            chainInfo.Height,
		CurrentBlockHash:  chainInfo.CurrentBlockHash,
		PreviousBlockHash: chainInfo.PreviousBlockHash,
	}, nil
}

// GetFabricBlocks retrieves a paginated list of blocks from the network
func (s *NetworkService) GetFabricBlocks(ctx context.Context, networkID int64, limit, offset int32, reverse bool) ([]fabricblock.Block, int64, error) {
	// Get the fabric deployer for this network
	fabricDeployer, err := s.getFabricDeployerForNetwork(ctx, networkID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get fabric deployer: %w", err)
	}

	// Use the fabric deployer to get blocks
	fabricBlocks, total, err := fabricDeployer.GetBlocks(ctx, networkID, limit, offset, reverse)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get blocks: %w", err)
	}

	return fabricBlocks, total, nil
}

// GetFabricBlock retrieves all transactions from a specific block
func (s *NetworkService) GetFabricBlock(ctx context.Context, networkID int64, blockNum uint64) (*fabricblock.Block, error) {
	// Get the fabric deployer for this network
	fabricDeployer, err := s.getFabricDeployerForNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get fabric deployer: %w", err)
	}

	// Use the fabric deployer to get block transactions
	fabricTransactions, err := fabricDeployer.GetBlock(ctx, networkID, blockNum)
	if err != nil {
		return nil, fmt.Errorf("failed to get block transactions: %w", err)
	}

	return fabricTransactions, nil
}

// GetTransaction retrieves a specific transaction by its ID
func (s *NetworkService) GetFabricBlockByTransaction(ctx context.Context, networkID int64, txID string) (*fabricblock.Block, error) {
	// Get the fabric deployer for this network
	fabricDeployer, err := s.getFabricDeployerForNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get fabric deployer: %w", err)
	}

	// Use the fabric deployer to get transaction
	block, err := fabricDeployer.GetBlockByTransaction(ctx, networkID, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %w", err)
	}

	return block, nil
}

// getFabricDeployerForNetwork creates and returns a fabric deployer for the specified network
func (s *NetworkService) getFabricDeployerForNetwork(ctx context.Context, networkID int64) (*fabric.FabricDeployer, error) {
	// Get network details to verify it exists and is a Fabric network
	network, err := s.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}
	deployer, err := s.deployerFactory.GetDeployer(network.Platform)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployer: %w", err)
	}

	fabricDeployer, ok := deployer.(*fabric.FabricDeployer)
	if !ok {
		return nil, fmt.Errorf("network %d is not a Fabric network", networkID)
	}

	return fabricDeployer, nil
}

// ImportNetworkWithOrgParams contains parameters for importing a network with organization details
type ImportNetworkWithOrgParams struct {
	ChannelID      string
	OrganizationID int64
	OrdererURL     string
	OrdererTLSCert []byte
	Description    string
}

// ImportFabricNetworkWithOrg imports a Fabric network using organization details
func (s *NetworkService) ImportFabricNetworkWithOrg(ctx context.Context, params ImportNetworkWithOrgParams) (*ImportNetworkResult, error) {
	// Get the Fabric deployer
	deployer, err := s.deployerFactory.GetDeployer("fabric")
	if err != nil {
		return nil, fmt.Errorf("failed to get Fabric deployer: %w", err)
	}
	fabricDeployer, ok := deployer.(*fabric.FabricDeployer)
	if !ok {
		return nil, fmt.Errorf("invalid deployer type")
	}

	// Import the network using the Fabric deployer
	network, err := fabricDeployer.ImportNetworkWithOrg(ctx, params.ChannelID, params.OrganizationID, params.OrdererURL, params.OrdererTLSCert, params.Description)
	if err != nil {
		return nil, fmt.Errorf("failed to import Fabric network with org: %w", err)
	}

	return &ImportNetworkResult{
		NetworkID: network.ID,
		Message:   "Fabric network imported successfully with organization",
	}, nil
}

func (s *NetworkService) importFabricNetwork(ctx context.Context, params ImportNetworkParams) (*ImportNetworkResult, error) {
	// Get the Fabric deployer
	deployer, err := s.deployerFactory.GetDeployer("fabric")
	if err != nil {
		return nil, fmt.Errorf("failed to get Fabric deployer: %w", err)
	}

	fabricDeployer, ok := deployer.(*fabric.FabricDeployer)
	if !ok {
		return nil, fmt.Errorf("invalid deployer type")
	}

	// Import the network using the Fabric deployer
	network, err := fabricDeployer.ImportNetwork(ctx, params.GenesisFile, params.Description)
	if err != nil {
		return nil, fmt.Errorf("failed to import Fabric network: %w", err)
	}

	return &ImportNetworkResult{
		NetworkID: network.ID,
		Message:   "Fabric network imported successfully",
	}, nil
}

// SetAnchorPeers sets the anchor peers for an organization in a Fabric network
func (s *NetworkService) SetAnchorPeers(ctx context.Context, networkID, organizationID int64, anchorPeers []AnchorPeer) (string, error) {
	// Get network details
	network, err := s.db.GetNetwork(ctx, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to get network: %w", err)
	}

	// Get deployer
	deployer, err := s.deployerFactory.GetDeployer(network.Platform)
	if err != nil {
		return "", fmt.Errorf("failed to get deployer: %w", err)
	}

	fabricDeployer, ok := deployer.(*fabric.FabricDeployer)
	if !ok {
		return "", fmt.Errorf("network %d is not a Fabric network", networkID)
	}

	// Convert anchor peers to deployer format
	deployerAnchorPeers := make([]types.HostPort, len(anchorPeers))
	for i, ap := range anchorPeers {
		deployerAnchorPeers[i] = types.HostPort{
			Host: ap.Host,
			Port: ap.Port,
		}
	}

	// Collect all available orderers to try (failover support)
	type ordererInfo struct {
		address string
		tlsCert string
		source  string // "registry", "config_block", or "genesis_block"
	}
	var orderers []ordererInfo

	// Try to get orderer info from network nodes first
	networkNodes, err := s.GetNetworkNodes(ctx, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to get network nodes: %w", err)
	}

	// Look for orderers in our registry
	for _, node := range networkNodes {
		if node.Node.NodeType == nodetypes.NodeTypeFabricOrderer {
			if node.Node.FabricOrderer == nil {
				continue
			}
			orderers = append(orderers, ordererInfo{
				address: node.Node.FabricOrderer.ExternalEndpoint,
				tlsCert: node.Node.FabricOrderer.TLSCACert,
				source:  "registry",
			})
		}
	}

	// Also get orderers from config block to ensure we have all available orderers
	configBlock, err := fabricDeployer.GetCurrentChannelConfig(networkID)
	if err != nil {
		logrus.Warnf("SetAnchorPeers: Failed to get current config block, continuing with registry orderers only: %v", err)
	} else {
		configOrderers, err := fabricDeployer.GetOrderersFromConfigBlock(ctx, configBlock)
		if err != nil {
			logrus.Warnf("SetAnchorPeers: Failed to get orderers from config block: %v", err)
		} else {
			for _, o := range configOrderers {
				// Check if this orderer is already in our list
				found := false
				for _, existing := range orderers {
					if existing.address == o.URL {
						found = true
						break
					}
				}
				if !found {
					orderers = append(orderers, ordererInfo{
						address: o.URL,
						tlsCert: o.TLSCert,
						source:  "config_block",
					})
				}
			}
		}
	}

	// Also try to get orderers from genesis block as additional fallback
	genesisOrderers, err := fabricDeployer.GetOrderersFromGenesisBlock(ctx, networkID)
	if err != nil {
		logrus.Debugf("SetAnchorPeers: Failed to get orderers from genesis block (may not exist): %v", err)
	} else {
		for _, o := range genesisOrderers {
			found := false
			for _, existing := range orderers {
				if existing.address == o.URL {
					found = true
					break
				}
			}
			if !found {
				orderers = append(orderers, ordererInfo{
					address: o.URL,
					tlsCert: o.TLSCert,
					source:  "genesis_block",
				})
			}
		}
	}

	if len(orderers) == 0 {
		return "", fmt.Errorf("no orderer found in network or config block")
	}

	logrus.Infof("SetAnchorPeers: Found %d orderers, will try each until success", len(orderers))

	// Try each orderer until one succeeds
	var lastErr error
	for i, orderer := range orderers {
		logrus.Infof("SetAnchorPeers: Attempting orderer %d/%d (%s, source=%s)", i+1, len(orderers), orderer.address, orderer.source)

		txID, err := fabricDeployer.SetAnchorPeersWithOrderer(ctx, networkID, organizationID, deployerAnchorPeers, orderer.address, orderer.tlsCert)
		if err == nil {
			logrus.Infof("SetAnchorPeers: Successfully set anchor peers via orderer %s, txID=%s", orderer.address, txID)

			logrus.Info("Reloading network block after setting anchor peers, waiting 500ms")
			time.Sleep(500 * time.Millisecond)

			// Reload network block
			if err := s.ReloadFabricNetworkBlock(ctx, networkID); err != nil {
				logrus.Errorf("Failed to reload network block after setting anchor peers: %v", err)
			}

			return txID, nil
		}

		logrus.Warnf("SetAnchorPeers: Failed with orderer %s: %v", orderer.address, err)
		lastErr = err
	}

	// All orderers failed
	return "", fmt.Errorf("failed to set anchor peers after trying all %d orderers, last error: %w", len(orderers), lastErr)
}

// ReloadFabricNetworkBlock reloads the network block for a given network ID
func (s *NetworkService) ReloadFabricNetworkBlock(ctx context.Context, networkID int64) error {
	// Get the network
	network, err := s.db.GetNetwork(ctx, networkID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("network with id %d not found", networkID)
		}
		return fmt.Errorf("failed to get network: %w", err)
	}

	// Get the deployer for this network type
	deployer, err := s.deployerFactory.GetDeployer(network.Platform)
	if err != nil {
		return fmt.Errorf("failed to get deployer: %w", err)
	}
	fabricDeployer, ok := deployer.(*fabric.FabricDeployer)
	if !ok {
		return fmt.Errorf("network %d is not a Fabric network", networkID)
	}

	// Get the current config block
	configBlock, err := fabricDeployer.FetchCurrentChannelConfig(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to get current config block: %w", err)
	}
	configBlockB64 := base64.StdEncoding.EncodeToString(configBlock)

	err = s.db.UpdateNetworkCurrentConfigBlock(ctx, &db.UpdateNetworkCurrentConfigBlockParams{
		ID:                    networkID,
		CurrentConfigBlockB64: sql.NullString{String: configBlockB64, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to update network config block: %w", err)
	}

	return nil
}

// GetNetworkConfig retrieves the network configuration as YAML
func (s *NetworkService) GetNetworkConfig(ctx context.Context, networkID, orgID int64) (string, error) {
	// Get the network
	network, err := s.db.GetNetwork(ctx, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to get network: %w", err)
	}

	// Get the deployer
	deployer, err := s.deployerFactory.GetDeployer(network.Platform)
	if err != nil {
		return "", fmt.Errorf("failed to get deployer: %w", err)
	}

	fabricDeployer, ok := deployer.(*fabric.FabricDeployer)
	if !ok {
		return "", fmt.Errorf("network %d is not a Fabric network", networkID)
	}

	// Generate network config YAML
	configYAML, err := fabricDeployer.GenerateNetworkConfig(ctx, networkID, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to generate network config: %w", err)
	}

	return configYAML, nil
}

// UnjoinPeerFromNetwork removes a peer from a channel but keeps it in the network
func (s *NetworkService) UnjoinPeerFromNetwork(networkID, peerID int64) error {
	network, err := s.db.GetNetwork(context.Background(), networkID)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}
	deployer, err := s.deployerFactory.GetDeployer(network.Platform)
	if err != nil {
		return fmt.Errorf("failed to get deployer: %w", err)
	}

	fabricDeployer, ok := deployer.(*fabric.FabricDeployer)
	if !ok {
		return fmt.Errorf("network %d is not a Fabric network", networkID)
	}

	if err := fabricDeployer.UnjoinNode(networkID, peerID); err != nil {
		return fmt.Errorf("failed to unjoin peer: %w", err)
	}

	logrus.Infof("unjoined peer %d from network %d", peerID, networkID)
	return nil
}

// UnjoinOrdererFromNetwork removes an orderer from a channel but keeps it in the network
func (s *NetworkService) UnjoinOrdererFromNetwork(networkID, ordererID int64) error {
	network, err := s.db.GetNetwork(context.Background(), networkID)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}
	deployer, err := s.deployerFactory.GetDeployer(network.Platform)
	if err != nil {
		return fmt.Errorf("failed to get deployer: %w", err)
	}

	fabricDeployer, ok := deployer.(*fabric.FabricDeployer)
	if !ok {
		return fmt.Errorf("network %d is not a Fabric network", networkID)
	}

	if err := fabricDeployer.UnjoinNode(networkID, ordererID); err != nil {
		return fmt.Errorf("failed to unjoin orderer: %w", err)
	}

	logrus.Infof("unjoined orderer %d from network %d", ordererID, networkID)
	return nil
}

// JoinPeerToNetwork joins a peer to a Fabric network
func (s *NetworkService) JoinPeerToNetwork(networkID, peerID int64) error {
	network, err := s.db.GetNetwork(context.Background(), networkID)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}
	deployer, err := s.deployerFactory.GetDeployer(network.Platform)
	if err != nil {
		return fmt.Errorf("failed to get deployer: %w", err)
	}
	if !network.GenesisBlockB64.Valid {
		return fmt.Errorf("genesis block is not set for network %d", networkID)
	}
	genesisBlockBytes, err := base64.StdEncoding.DecodeString(network.GenesisBlockB64.String)
	if err != nil {
		return fmt.Errorf("failed to decode genesis block: %w", err)
	}
	err = deployer.JoinNode(network.ID, genesisBlockBytes, peerID)
	if err != nil {
		return fmt.Errorf("failed to join node: %w", err)
	}
	logrus.Infof("joined peer %d to network %d", peerID, networkID)

	return nil
}

// JoinOrdererToNetwork joins an orderer to a Fabric network
func (s *NetworkService) JoinOrdererToNetwork(networkID, ordererID int64) error {
	network, err := s.db.GetNetwork(context.Background(), networkID)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}
	deployer, err := s.deployerFactory.GetDeployer(network.Platform)
	if err != nil {
		return fmt.Errorf("failed to get deployer: %w", err)
	}
	if !network.GenesisBlockB64.Valid {
		return fmt.Errorf("genesis block is not set for network %d", networkID)
	}
	genesisBlockBytes, err := base64.StdEncoding.DecodeString(network.GenesisBlockB64.String)
	if err != nil {
		return fmt.Errorf("failed to decode genesis block: %w", err)
	}
	err = deployer.JoinNode(network.ID, genesisBlockBytes, ordererID)
	if err != nil {
		return fmt.Errorf("failed to join node: %w", err)
	}
	logrus.Infof("joined orderer %d to network %d", ordererID, networkID)

	return nil
}

// RemovePeerFromNetwork removes a peer from a Fabric network
func (s *NetworkService) RemovePeerFromNetwork(networkID, peerID int64) error {
	// Get the appropriate deployer
	network, err := s.db.GetNetwork(context.Background(), networkID)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}

	deployer, err := s.deployerFactory.GetDeployer(network.Platform)
	if err != nil {
		return fmt.Errorf("failed to get deployer: %w", err)
	}

	fabricDeployer, ok := deployer.(*fabric.FabricDeployer)
	if !ok {
		return fmt.Errorf("network %d is not a Fabric network", networkID)
	}

	if err := fabricDeployer.RemoveNode(networkID, peerID); err != nil {
		return fmt.Errorf("failed to remove peer: %w", err)
	}

	logrus.Infof("removed peer %d from network %d", peerID, networkID)
	return nil
}

// RemoveOrdererFromNetwork removes an orderer from a Fabric network
func (s *NetworkService) RemoveOrdererFromNetwork(networkID, ordererID int64) error {
	// Get the appropriate deployer
	network, err := s.db.GetNetwork(context.Background(), networkID)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}

	deployer, err := s.deployerFactory.GetDeployer(network.Platform)
	if err != nil {
		return fmt.Errorf("failed to get deployer: %w", err)
	}

	fabricDeployer, ok := deployer.(*fabric.FabricDeployer)
	if !ok {
		return fmt.Errorf("network %d is not a Fabric network", networkID)
	}

	if err := fabricDeployer.RemoveNode(networkID, ordererID); err != nil {
		return fmt.Errorf("failed to remove orderer: %w", err)
	}

	logrus.Infof("removed orderer %d from network %d", ordererID, networkID)
	return nil
}

// GetFabricCurrentChannelConfig retrieves the current channel configuration for a network
func (s *NetworkService) GetFabricCurrentChannelConfig(networkID int64) (map[string]interface{}, error) {
	// Get the appropriate deployer
	network, err := s.db.GetNetwork(context.Background(), networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	deployer, err := s.deployerFactory.GetDeployer(network.Platform)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployer: %w", err)
	}

	fabricDeployer, ok := deployer.(*fabric.FabricDeployer)
	if !ok {
		return nil, fmt.Errorf("network %d is not a Fabric network", networkID)
	}

	return fabricDeployer.GetCurrentChannelConfigAsMap(networkID)
}

// GetFabricCurrentChannelConfig retrieves the current channel configuration for a network
func (s *NetworkService) GetFabricNetworkConfigTX(networkID int64) (*configtx.ConfigTx, error) {
	// Get the appropriate deployer
	network, err := s.db.GetNetwork(context.Background(), networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	deployer, err := s.deployerFactory.GetDeployer(network.Platform)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployer: %w", err)
	}

	fabricDeployer, ok := deployer.(*fabric.FabricDeployer)
	if !ok {
		return nil, fmt.Errorf("network %d is not a Fabric network", networkID)
	}

	return fabricDeployer.GetCurrentChannelConfigTX(networkID)
}

// GetFabricChannelConfig retrieves the channel configuration for a network
func (s *NetworkService) GetFabricChannelConfig(networkID int64) (map[string]interface{}, error) {
	// Get the appropriate deployer
	network, err := s.db.GetNetwork(context.Background(), networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	deployer, err := s.deployerFactory.GetDeployer(network.Platform)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployer: %w", err)
	}

	fabricDeployer, ok := deployer.(*fabric.FabricDeployer)
	if !ok {
		return nil, fmt.Errorf("network %d is not a Fabric network", networkID)
	}

	return fabricDeployer.GetChannelConfig(networkID)
}
