package besu

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"encoding/base64"
	"encoding/hex"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	keymanagement "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/google/uuid"
)

// BesuDeployer implements the NetworkDeployer interface for Hyperledger Besu
type BesuDeployer struct {
	db      *db.Queries
	logger  *logger.Logger
	nodes   *nodeservice.NodeService
	keyMgmt *keymanagement.KeyManagementService
}

// NewBesuDeployer creates a new BesuDeployer instance
func NewBesuDeployer(db *db.Queries, nodes *nodeservice.NodeService, keyMgmt *keymanagement.KeyManagementService) *BesuDeployer {
	logger := logger.NewDefault().With("component", "besu_deployer")
	return &BesuDeployer{
		db:      db,
		logger:  logger,
		nodes:   nodes,
		keyMgmt: keyMgmt,
	}
}

func (d *BesuDeployer) JoinNode(networkID int64, genesisBlock []byte, nodeID int64) error {
	return fmt.Errorf("operation not supported for Besu networks")
}

func (d *BesuDeployer) AddOrganization(networkID, organizationID int64) error {
	return fmt.Errorf("operation not supported for Besu networks")
}

func (d *BesuDeployer) RemoveOrganization(networkID, organizationID int64) error {
	return fmt.Errorf("operation not supported for Besu networks")
}

func (d *BesuDeployer) SetAnchorPeers(ctx context.Context, networkID int64, organizationID int64, anchorPeers []types.HostPort) (string, error) {
	return "", fmt.Errorf("operation not supported for Besu networks")
}

// CreateGenesisBlock generates a genesis block for a new Besu network
func (d *BesuDeployer) CreateGenesisBlock(networkID int64, config interface{}) ([]byte, error) {
	besuConfig, ok := config.(*types.BesuNetworkConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type: expected BesuNetworkConfig, got %T", config)
	}

	ctx := context.Background()
	d.logger.Info("Creating genesis block for Besu network with chain ID %d", besuConfig.ChainID)

	// Get validator nodes
	validators := make([]BesuNode, 0)
	for validatorIndex, validatorKeyId := range besuConfig.InitialValidatorKeyIds {
		key, err := d.keyMgmt.GetKey(ctx, int(validatorKeyId))
		if err != nil {
			return nil, fmt.Errorf("failed to get key: %w", err)
		}
		if key.EthereumAddress == "" {
			return nil, fmt.Errorf("key has no ethereum address")
		}
		publicKey, err := d.decodePEMPublicKey(key.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode public key: %w", err)
		}

		// Convert node to BesuNode
		besuNode := BesuNode{
			ID:             validatorKeyId,
			Type:           NodeTypeValidator,
			Address:        key.EthereumAddress,
			PublicKey:      publicKey,
			ValidatorIndex: validatorIndex,
		}
		validators = append(validators, besuNode)
	}

	// Create extraData for QBFT
	extraData, err := d.createExtraData(validators)
	if err != nil {
		return nil, fmt.Errorf("failed to create extra data: %w", err)
	}

	// Create initial allocation
	alloc := make(map[string]map[string]string)

	// Add custom allocations from config if any
	if besuConfig.Alloc != nil {
		for address, balance := range besuConfig.Alloc {
			// Remove 0x prefix if present
			addressWithoutPrefix := strings.TrimPrefix(address, "0x")
			alloc[addressWithoutPrefix] = map[string]string{
				"balance": balance.Balance,
			}
		}
	}

	// Create genesis parameters
	genesis := &GenesisParams{
		Config: Config{
			ChainID:     besuConfig.ChainID,
			BerlinBlock: 0,
			QBFT: QBFTConfig{
				BlockPeriodSeconds:    besuConfig.BlockPeriod,
				EpochLength:           besuConfig.EpochLength,
				RequestTimeoutSeconds: besuConfig.RequestTimeout,
				StartBlock:            0,
			},
		},
		Nonce:      besuConfig.Nonce,
		Timestamp:  besuConfig.Timestamp,
		GasLimit:   besuConfig.GasLimit,
		Difficulty: besuConfig.Difficulty,
		MixHash:    besuConfig.MixHash,
		Coinbase:   besuConfig.Coinbase,
		Alloc:      alloc,
		ExtraData:  extraData,
		Number:     "0x0",
		GasUsed:    "0x0",
		ParentHash: "0x0000000000000000000000000000000000000000000000000000000000000000",
	}

	// Convert genesis to JSON
	genesisJSON, err := json.Marshal(genesis)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal genesis: %w", err)
	}

	// Update network with genesis block
	_, err = d.db.UpdateNetworkGenesisBlock(context.Background(), &db.UpdateNetworkGenesisBlockParams{
		ID: networkID,
		GenesisBlockB64: sql.NullString{
			String: string(genesisJSON),
			Valid:  true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update network genesis block: %w", err)
	}

	// Create network nodes
	for _, validator := range validators {
		_, err = d.db.CreateNetworkNode(ctx, &db.CreateNetworkNodeParams{
			NetworkID: networkID,
			NodeID:    validator.ID,
			Status:    "pending",
			Role:      "validator",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create network node: %w", err)
		}
	}

	return genesisJSON, nil
}

func (d *BesuDeployer) GetCurrentConfigBlock(ctx context.Context, networkID int64) ([]byte, error) {
	return nil, fmt.Errorf("operation not supported for Besu networks")
}

// createExtraData creates the extraData field for QBFT consensus
// func (d *BesuDeployer) createExtraData(validators []BesuNode) (string, error) {
// 	// Implement QBFT extra data creation logic
// 	// This should include the validator addresses in the required format
// 	// The format depends on your specific Besu implementation
// 	validatorAddresses := make([]string, len(validators))
// 	for i, validator := range validators {
// 		validatorAddresses[i] = validator.Address
// 	}

//		// This is a simplified example - adjust according to your needs
//		extraData := "0x" + strings.Join(validatorAddresses, "")
//		return extraData, nil
//	}
const EXTRA_VANITY_LENGTH = 32

func (d *BesuDeployer) createExtraData(validators []BesuNode) (string, error) {
	// Convert validator addresses from hex strings to Address type
	validatorAddresses := make([]common.Address, len(validators))
	for i, validator := range validators {
		validatorAddresses[i] = common.HexToAddress(validator.Address)
	}
	d.logger.Info("validatorAddresses: %v", validatorAddresses)
	// First, RLP encode the main components
	rlpList := []interface{}{
		make([]byte, EXTRA_VANITY_LENGTH), // 32 bytes of zeros
		validatorAddresses,                // List of validators
		[]interface{}{},                   // Empty vote list
		uint(0),                           // Round number (0 for genesis)
		[]interface{}{},                   // Empty seals list
	}

	// RLP encode the entire structure
	extraData, err := rlp.EncodeToBytes(rlpList)
	if err != nil {
		return "", fmt.Errorf("failed to RLP encode extra data: %v", err)
	}

	return "0x" + hex.EncodeToString(extraData), nil
}

// GetStatus retrieves the current status of the Besu network
func (d *BesuDeployer) GetStatus(networkID int64) (*types.NetworkDeploymentStatus, error) {
	// ctx := context.Background()

	// network, err := d.db.GetNetwork(ctx, networkID)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to get network: %w", err)
	// }

	// nodes, err := d.db.GetNetworkNodes(ctx, networkID)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to get network nodes: %w", err)
	// }

	// status := &types.NetworkDeploymentStatus{
	// 	NetworkID: networkID,
	// 	Status:    string(network.Status),
	// 	Nodes:     make([]types.NodeStatus, len(nodes)),
	// }

	// for i, node := range nodes {
	// 	status.Nodes[i] = types.NodeStatus{
	// 		NodeID: node.NodeID,
	// 		Status: node.Status,
	// 	}
	// }

	return nil, errors.New("not implemented")
}

// decodePEMPublicKey decodes a hex-encoded public key and validates its format
func (d *BesuDeployer) decodePEMPublicKey(hexStr string) (string, error) {
	// Remove "0x" prefix if present
	hexStr = strings.TrimPrefix(hexStr, "0x")

	// Decode hex string to bytes to validate length
	publicKeyBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", fmt.Errorf("invalid hex string: %w", err)
	}

	// Validate the public key length (uncompressed secp256k1 public key is 65 bytes)
	if len(publicKeyBytes) != 65 {
		return "", fmt.Errorf("invalid public key length: expected 65 bytes, got %d", len(publicKeyBytes))
	}

	// Return the normalized hex string (without 0x prefix)
	return hexStr, nil
}

// ImportNetwork imports a Besu network from a genesis file
func (d *BesuDeployer) ImportNetwork(ctx context.Context, genesisFile []byte, name, description string) (string, error) {
	// Parse the genesis file
	var genesisConfig map[string]interface{}
	if err := json.Unmarshal(genesisFile, &genesisConfig); err != nil {
		return "", fmt.Errorf("failed to parse Besu genesis file: %w", err)
	}

	// Validate required fields
	if _, ok := genesisConfig["config"]; !ok {
		return "", fmt.Errorf("invalid Besu genesis file: missing config section")
	}

	// Validate chainId exists
	config, ok := genesisConfig["config"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid Besu genesis file: config section is not an object")
	}
	if _, ok := config["chainId"]; !ok {
		return "", fmt.Errorf("invalid Besu genesis file: missing chainId in config section")
	}

	// Generate a unique network ID
	networkID := uuid.New().String()

	// Create network in database
	_, err := d.db.CreateNetworkFull(ctx, &db.CreateNetworkFullParams{
		Name:        name,
		Platform:    "besu",
		Description: sql.NullString{String: description, Valid: description != ""},
		Status:      "genesis_block_created",
		NetworkID:   sql.NullString{String: networkID, Valid: true},
		GenesisBlockB64: sql.NullString{
			String: base64.StdEncoding.EncodeToString(genesisFile),
			Valid:  true,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create network: %w", err)
	}

	return networkID, nil
}
