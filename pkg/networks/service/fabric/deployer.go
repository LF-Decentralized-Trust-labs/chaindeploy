package fabric

import (
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"bytes"

	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/chainlaunch/chainlaunch/internal/protoutil"
	"github.com/chainlaunch/chainlaunch/pkg/certutils"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/fabric/channel"
	orgservicefabric "github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	keymanagement "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/fabric/block"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/fabric/org"
	fabricorg "github.com/chainlaunch/chainlaunch/pkg/networks/service/fabric/org"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"

	"github.com/google/uuid"
	"github.com/hyperledger/fabric-admin-sdk/pkg/network"
	"github.com/hyperledger/fabric-config/configtx"
	"github.com/hyperledger/fabric-config/configtx/membership"
	"github.com/hyperledger/fabric-config/configtx/orderer"
	ordererapi "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"google.golang.org/grpc"

	"github.com/hyperledger/fabric-config/protolator"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
)

// ConfigUpdateOperationType represents the type of configuration update operation
type ConfigUpdateOperationType string

const (
	// Application config update operations
	OpAddOrg                ConfigUpdateOperationType = "add_org"
	OpRemoveOrg             ConfigUpdateOperationType = "remove_org"
	OpUpdateOrgMSP          ConfigUpdateOperationType = "update_org_msp"
	OpSetAnchorPeers        ConfigUpdateOperationType = "set_anchor_peers"
	OpUpdateEtcdRaftOptions ConfigUpdateOperationType = "update_etcd_raft_options"
	// Orderer config update operations
	OpAddConsenter       ConfigUpdateOperationType = "add_consenter"
	OpRemoveConsenter    ConfigUpdateOperationType = "remove_consenter"
	OpUpdateConsenter    ConfigUpdateOperationType = "update_consenter"
	OpUpdateBatchSize    ConfigUpdateOperationType = "update_batch_size"
	OpUpdateBatchTimeout ConfigUpdateOperationType = "update_batch_timeout"
)

// ConfigUpdateOperation represents a configuration update operation with its associated data
type ConfigUpdateOperation struct {
	Type    ConfigUpdateOperationType `json:"type"`
	Payload json.RawMessage           `json:"payload"`
}

// ConfigModifier is an interface for modifying a Fabric configuration block
type ConfigModifier interface {
	// Type returns the type of the operation
	Type() ConfigUpdateOperationType

	// Modify applies the operation to the given config
	Modify(ctx context.Context, c *configtx.ConfigTx) error

	// Validate validates the operation
	Validate() error
}

// ConfigUpdateProposal represents a proposed update to the channel configuration
type ConfigUpdateProposal struct {
	ID                   string                  `json:"id"`
	NetworkID            int64                   `json:"network_id"`
	ChannelName          string                  `json:"channel_name"`
	Operations           []ConfigUpdateOperation `json:"operations"`
	ConfigUpdateEnvelope []byte                  `json:"config_update_envelope"`
	CreatedBy            string                  `json:"created_by"`
	CreatedAt            time.Time               `json:"created_at"`
	Status               string                  `json:"status"`
	Signatures           []ConfigSignature       `json:"signatures"`
}

// ConfigSignature represents a signature on a config update
type ConfigSignature struct {
	MSPID     string    `json:"msp_id"`
	Signature []byte    `json:"signature"`
	SignedAt  time.Time `json:"signed_at"`
	SignedBy  string    `json:"signed_by"`
}

// AddOrgOperation represents an operation to add an organization
type AddOrgOperation struct {
	MSPID        string   `json:"msp_id"`
	TLSRootCerts []string `json:"tls_root_certs"`
	RootCerts    []string `json:"root_certs"`
}

// Type returns the type of the operation
func (op *AddOrgOperation) Type() ConfigUpdateOperationType {
	return OpAddOrg
}

// Validate validates the operation
func (op *AddOrgOperation) Validate() error {
	if op.MSPID == "" {
		return fmt.Errorf("invalid MSPID: %s", op.MSPID)
	}
	if len(op.TLSRootCerts) == 0 {
		return fmt.Errorf("TLS root certificates cannot be empty")
	}
	if len(op.RootCerts) == 0 {
		return fmt.Errorf("root certificates cannot be empty")
	}
	return nil
}

// Modify applies the operation to the given config
func (op *AddOrgOperation) Modify(ctx context.Context, c *configtx.ConfigTx) error {
	// Get organization details from the database or service
	// This would typically involve fetching the organization details using op.OrgID
	// For this example, we'll assume we have a method to get these details from the deployer
	// We'll need to pass the deployer as a parameter in a real implementation

	// Create MSP configuration
	// In a real implementation, we would fetch the certificates and MSP details
	// For now, we'll use placeholder values
	mspID := op.MSPID

	// Create a new organization in the application group

	var rootCerts []*x509.Certificate
	for _, rootCertStr := range op.RootCerts {
		rootCert, err := certutils.ParseX509Certificate([]byte(rootCertStr))
		if err != nil {
			return fmt.Errorf("failed to parse root certificate: %w", err)
		}
		rootCerts = append(rootCerts, rootCert)
	}

	var tlsRootCerts []*x509.Certificate
	for _, tlsRootCertStr := range op.TLSRootCerts {
		tlsRootCert, err := certutils.ParseX509Certificate([]byte(tlsRootCertStr))
		if err != nil {
			return fmt.Errorf("failed to parse TLS root certificate: %w", err)
		}
		tlsRootCerts = append(tlsRootCerts, tlsRootCert)
	}
	signCACert := rootCerts[0]

	// Set MSP configuration
	err := c.Application().SetOrganization(configtx.Organization{
		Name: mspID,
		MSP: configtx.MSP{
			Name:         mspID,
			RootCerts:    rootCerts,
			TLSRootCerts: tlsRootCerts,
			Admins:       []*x509.Certificate{},
			NodeOUs: membership.NodeOUs{
				Enable: true,
				ClientOUIdentifier: membership.OUIdentifier{
					Certificate:                  signCACert,
					OrganizationalUnitIdentifier: "client",
				},
				PeerOUIdentifier: membership.OUIdentifier{
					Certificate:                  signCACert,
					OrganizationalUnitIdentifier: "peer",
				},
				AdminOUIdentifier: membership.OUIdentifier{
					Certificate:                  signCACert,
					OrganizationalUnitIdentifier: "admin",
				},
				OrdererOUIdentifier: membership.OUIdentifier{
					Certificate:                  signCACert,
					OrganizationalUnitIdentifier: "orderer",
				},
			},
		},
		AnchorPeers:      []configtx.Address{},
		OrdererEndpoints: []string{},
		Policies: map[string]configtx.Policy{
			"Admins": {
				Type: "Signature",
				Rule: fmt.Sprintf("OR('%s.admin')", mspID),
			},
			"Readers": {
				Type: "Signature",
				Rule: fmt.Sprintf("OR('%s.member')", mspID),
			},
			"Writers": {
				Type: "Signature",
				Rule: fmt.Sprintf("OR('%s.member')", mspID),
			},
			"Endorsement": {
				Type: "Signature",
				Rule: fmt.Sprintf("OR('%s.member')", mspID),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to set MSP configuration: %w", err)
	}

	return nil
}

// RemoveOrgOperation represents an operation to remove an organization
type RemoveOrgOperation struct {
	MSPID string `json:"msp_id"`
}

// Type returns the type of the operation
func (op *RemoveOrgOperation) Type() ConfigUpdateOperationType {
	return OpRemoveOrg
}

// Validate validates the operation
func (op *RemoveOrgOperation) Validate() error {
	if op.MSPID == "" {
		return fmt.Errorf("MSPID cannot be empty")
	}
	return nil
}

// Modify applies the operation to the given config
func (op *RemoveOrgOperation) Modify(ctx context.Context, c *configtx.ConfigTx) error {

	// Get the application organization
	c.Application().RemoveOrganization(op.MSPID)

	// Compute the updated config
	return nil
}

// AddConsenterOperation represents an operation to add a consenter
type AddConsenterOperation struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	ClientTLSCert string `json:"client_tls_cert"`
	ServerTLSCert string `json:"server_tls_cert"`
}

// Type returns the type of the operation
func (op *AddConsenterOperation) Type() ConfigUpdateOperationType {
	return OpAddConsenter
}

// Validate validates the operation
func (op *AddConsenterOperation) Validate() error {
	if op.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if op.Port <= 0 {
		return fmt.Errorf("invalid port: %d", op.Port)
	}
	if op.ClientTLSCert == "" {
		return fmt.Errorf("client TLS certificate cannot be empty")
	}
	if op.ServerTLSCert == "" {
		return fmt.Errorf("server TLS certificate cannot be empty")
	}
	return nil
}

// Modify applies the operation to the given config
func (op *AddConsenterOperation) Modify(ctx context.Context, c *configtx.ConfigTx) error {

	// Parse TLS certificates
	clientTLSCert, err := certutils.ParseX509Certificate([]byte(op.ClientTLSCert))
	if err != nil {
		return fmt.Errorf("failed to parse client TLS certificate: %w", err)
	}

	serverTLSCert, err := certutils.ParseX509Certificate([]byte(op.ServerTLSCert))
	if err != nil {
		return fmt.Errorf("failed to parse server TLS certificate: %w", err)
	}

	// Add new consenter
	newConsenter := orderer.Consenter{
		Address: orderer.EtcdAddress{
			Host: op.Host,
			Port: op.Port,
		},
		ClientTLSCert: clientTLSCert,
		ServerTLSCert: serverTLSCert,
	}

	// Update consenters
	err = c.Orderer().AddConsenter(newConsenter)
	if err != nil {
		return fmt.Errorf("failed to set consenters: %w", err)
	}

	return nil
}

// RemoveConsenterOperation represents an operation to remove a consenter
type RemoveConsenterOperation struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// Type returns the type of the operation
func (op *RemoveConsenterOperation) Type() ConfigUpdateOperationType {
	return OpRemoveConsenter
}

// Validate validates the operation
func (op *RemoveConsenterOperation) Validate() error {
	if op.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if op.Port <= 0 {
		return fmt.Errorf("invalid port: %d", op.Port)
	}
	return nil
}

// Modify applies the operation to the given config
func (op *RemoveConsenterOperation) Modify(ctx context.Context, c *configtx.ConfigTx) error {
	// Get orderer group
	ordConfig, err := c.Orderer().Configuration()
	if err != nil {
		return fmt.Errorf("failed to get orderer configuration: %w", err)
	}
	consenters := ordConfig.EtcdRaft.Consenters

	// Remove consenter
	var consenterToRemove *orderer.Consenter
	for _, consenter := range consenters {
		if consenter.Address.Host == op.Host && consenter.Address.Port == op.Port {
			consenterToRemove = &consenter
		}
	}
	if consenterToRemove == nil {
		return fmt.Errorf("consenter not found")
	}
	// Update consenters
	err = c.Orderer().RemoveConsenter(*consenterToRemove)
	if err != nil {
		return fmt.Errorf("failed to remove consenters: %w", err)
	}

	return nil
}

// UpdateConsenterOperation represents an operation to update a consenter
type UpdateConsenterOperation struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	NewHost       string `json:"new_host"`
	NewPort       int    `json:"new_port"`
	ClientTLSCert string `json:"client_tls_cert"`
	ServerTLSCert string `json:"server_tls_cert"`
}

// Type returns the type of the operation
func (op *UpdateConsenterOperation) Type() ConfigUpdateOperationType {
	return OpUpdateConsenter
}

// Validate validates the operation
func (op *UpdateConsenterOperation) Validate() error {
	if op.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if op.Port <= 0 {
		return fmt.Errorf("invalid port: %d", op.Port)
	}
	if op.NewHost == "" {
		return fmt.Errorf("new host cannot be empty")
	}
	if op.NewPort <= 0 {
		return fmt.Errorf("invalid new port: %d", op.NewPort)
	}
	return nil
}

// Modify applies the operation to the given config
func (op *UpdateConsenterOperation) Modify(ctx context.Context, c *configtx.ConfigTx) error {
	// Get orderer group
	ordConfig, err := c.Orderer().Configuration()
	if err != nil {
		return fmt.Errorf("failed to get orderer configuration: %w", err)
	}
	consenters := ordConfig.EtcdRaft.Consenters

	// Update consenter
	found := false
	for i := range consenters {
		if consenters[i].Address.Host == op.Host && consenters[i].Address.Port == op.Port {
			clientTLSCert, err := certutils.ParseX509Certificate([]byte(op.ClientTLSCert))
			if err != nil {
				return fmt.Errorf("failed to parse client TLS cert: %w", err)
			}

			serverTLSCert, err := certutils.ParseX509Certificate([]byte(op.ServerTLSCert))
			if err != nil {
				return fmt.Errorf("failed to parse server TLS cert: %w", err)
			}

			consenters[i].Address.Host = op.NewHost
			consenters[i].Address.Port = op.NewPort
			consenters[i].ClientTLSCert = clientTLSCert
			consenters[i].ServerTLSCert = serverTLSCert
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("consenter not found")
	}

	// Update orderer configuration
	err = c.Orderer().SetConfiguration(ordConfig)
	if err != nil {
		return fmt.Errorf("failed to update orderer configuration: %w", err)
	}

	return nil
}

// UpdateEtcdRaftOptionsOperation represents an operation to update etcd raft options
type UpdateEtcdRaftOptionsOperation struct {
	TickInterval         string `json:"tick_interval"`
	ElectionTick         uint32 `json:"election_tick"`
	HeartbeatTick        uint32 `json:"heartbeat_tick"`
	MaxInflightBlocks    uint32 `json:"max_inflight_blocks"`
	SnapshotIntervalSize uint32 `json:"snapshot_interval_size"`
}

// Type returns the type of the operation
func (op *UpdateEtcdRaftOptionsOperation) Type() ConfigUpdateOperationType {
	return OpUpdateEtcdRaftOptions
}

// Validate validates the operation
func (op *UpdateEtcdRaftOptionsOperation) Validate() error {
	if op.TickInterval == "" {
		return fmt.Errorf("tick interval cannot be empty")
	}
	if op.ElectionTick == 0 {
		return fmt.Errorf("election tick cannot be zero")
	}
	if op.HeartbeatTick == 0 {
		return fmt.Errorf("heartbeat tick cannot be zero")
	}
	if op.MaxInflightBlocks == 0 {
		return fmt.Errorf("max inflight blocks cannot be zero")
	}
	if op.SnapshotIntervalSize == 0 {
		return fmt.Errorf("snapshot interval size cannot be zero")
	}
	return nil
}

// Modify applies the operation to the given config
func (op *UpdateEtcdRaftOptionsOperation) Modify(ctx context.Context, c *configtx.ConfigTx) error {
	// Get orderer configuration
	ordConfig, err := c.Orderer().Configuration()
	if err != nil {
		return fmt.Errorf("failed to get orderer configuration: %w", err)
	}

	// Update etcd raft options
	ordConfig.EtcdRaft.Options = orderer.EtcdRaftOptions{
		TickInterval:         op.TickInterval,
		ElectionTick:         op.ElectionTick,
		HeartbeatTick:        op.HeartbeatTick,
		MaxInflightBlocks:    op.MaxInflightBlocks,
		SnapshotIntervalSize: op.SnapshotIntervalSize,
	}

	// Set updated configuration
	err = c.Orderer().SetConfiguration(ordConfig)
	if err != nil {
		return fmt.Errorf("failed to update orderer configuration: %w", err)
	}

	return nil
}

type UpdateBatchSizeOperation struct {
	AbsoluteMaxBytes  int `json:"absolute_max_bytes"`
	MaxMessageCount   int `json:"max_message_count"`
	PreferredMaxBytes int `json:"preferred_max_bytes"`
}

// Type returns the type of the operation
func (op *UpdateBatchSizeOperation) Type() ConfigUpdateOperationType {
	return OpUpdateBatchSize
}

// Validate validates the operation
func (op *UpdateBatchSizeOperation) Validate() error {
	if op.AbsoluteMaxBytes <= 0 {
		return fmt.Errorf("absolute max bytes must be greater than 0")
	}
	if op.MaxMessageCount <= 0 {
		return fmt.Errorf("max message count must be greater than 0")
	}
	if op.PreferredMaxBytes <= 0 {
		return fmt.Errorf("preferred max bytes must be greater than 0")
	}
	return nil
}

// Modify applies the operation to the given config
func (op *UpdateBatchSizeOperation) Modify(ctx context.Context, c *configtx.ConfigTx) error {
	// Get orderer configuration
	ordConfig, err := c.Orderer().Configuration()
	if err != nil {
		return fmt.Errorf("failed to get orderer configuration: %w", err)
	}

	// Update batch size configuration
	ordConfig.BatchSize = orderer.BatchSize{
		AbsoluteMaxBytes:  uint32(op.AbsoluteMaxBytes),
		MaxMessageCount:   uint32(op.MaxMessageCount),
		PreferredMaxBytes: uint32(op.PreferredMaxBytes),
	}

	// Set updated configuration
	err = c.Orderer().SetConfiguration(ordConfig)
	if err != nil {
		return fmt.Errorf("failed to update orderer configuration: %w", err)
	}

	return nil
}

// UpdateBatchTimeoutOperation represents an operation to update batch timeout
type UpdateBatchTimeoutOperation struct {
	Timeout string `json:"timeout"` // e.g., "2s"
}

// Type returns the type of the operation
func (op *UpdateBatchTimeoutOperation) Type() ConfigUpdateOperationType {
	return OpUpdateBatchTimeout
}

// Validate validates the operation
func (op *UpdateBatchTimeoutOperation) Validate() error {
	if op.Timeout == "" {
		return fmt.Errorf("timeout cannot be empty")
	}
	// Validate that the timeout is a valid duration
	_, err := time.ParseDuration(op.Timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}
	return nil
}

// Modify applies the operation to the given config
func (op *UpdateBatchTimeoutOperation) Modify(ctx context.Context, c *configtx.ConfigTx) error {
	// Parse the timeout duration
	batchTimeout, err := time.ParseDuration(op.Timeout)
	if err != nil {
		return fmt.Errorf("failed to parse timeout duration: %w", err)
	}

	// Set updated configuration
	err = c.Orderer().SetBatchTimeout(batchTimeout)
	if err != nil {
		return fmt.Errorf("failed to update orderer configuration: %w", err)
	}

	return nil
}

// CreateConfigModifier creates a ConfigModifier from a ConfigUpdateOperation
func CreateConfigModifier(operation ConfigUpdateOperation) (ConfigModifier, error) {
	var modifier ConfigModifier

	switch operation.Type {
	case OpAddOrg:
		var op AddOrgOperation
		if err := json.Unmarshal(operation.Payload, &op); err != nil {
			return nil, fmt.Errorf("failed to unmarshal add org payload: %w", err)
		}
		modifier = &op
	case OpRemoveOrg:
		var op RemoveOrgOperation
		if err := json.Unmarshal(operation.Payload, &op); err != nil {
			return nil, fmt.Errorf("failed to unmarshal remove org payload: %w", err)
		}
		modifier = &op
	case OpAddConsenter:
		var op AddConsenterOperation
		if err := json.Unmarshal(operation.Payload, &op); err != nil {
			return nil, fmt.Errorf("failed to unmarshal add consenter payload: %w", err)
		}
		modifier = &op
	case OpRemoveConsenter:
		var op RemoveConsenterOperation
		if err := json.Unmarshal(operation.Payload, &op); err != nil {
			return nil, fmt.Errorf("failed to unmarshal remove consenter payload: %w", err)
		}
		modifier = &op
	case OpUpdateConsenter:
		var op UpdateConsenterOperation
		if err := json.Unmarshal(operation.Payload, &op); err != nil {
			return nil, fmt.Errorf("failed to unmarshal update consenter payload: %w", err)
		}
		modifier = &op
	case OpUpdateEtcdRaftOptions:
		var op UpdateEtcdRaftOptionsOperation
		if err := json.Unmarshal(operation.Payload, &op); err != nil {
			return nil, fmt.Errorf("failed to unmarshal update etcd raft options payload: %w", err)
		}
		modifier = &op
	case OpUpdateBatchSize:
		var op UpdateBatchSizeOperation
		if err := json.Unmarshal(operation.Payload, &op); err != nil {
			return nil, fmt.Errorf("failed to unmarshal update batch size payload: %w", err)
		}
		modifier = &op
	case OpUpdateBatchTimeout:
		var op UpdateBatchTimeoutOperation
		if err := json.Unmarshal(operation.Payload, &op); err != nil {
			return nil, fmt.Errorf("failed to unmarshal update batch timeout payload: %w", err)
		}
		modifier = &op
	default:
		return nil, fmt.Errorf("unsupported operation type: %s", operation.Type)
	}

	// Validate the operation
	if err := modifier.Validate(); err != nil {
		return nil, fmt.Errorf("invalid operation: %w", err)
	}

	return modifier, nil
}

// UpdateChannelConfig updates the channel configuration with the provided config update envelope and signatures
func (d *FabricDeployer) UpdateChannelConfig(ctx context.Context, networkID int64, configUpdateEnvelope []byte, signingOrgIDs []string, ordererAddress string, ordererTLSCert string) (string, error) {
	// Get network details
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to get network: %w", err)
	}

	// Unmarshal the config update envelope
	envelope := &cb.Envelope{}
	if err := proto.Unmarshal(configUpdateEnvelope, envelope); err != nil {
		return "", fmt.Errorf("failed to unmarshal config update envelope: %w", err)
	}

	// Collect signatures from the specified organizations
	for _, orgID := range signingOrgIDs {
		// Get organization details and MSP
		orgService := org.NewOrganizationService(d.orgService, d.keyMgmt, d.logger, orgID, d.db)

		// Sign the config update
		envelope, err = orgService.CreateConfigSignature(ctx, network.Name, envelope)
		if err != nil {
			return "", fmt.Errorf("failed to sign config update for org %s: %w", orgID, err)
		}
	}

	ordererConn, err := d.createOrdererConnection(ordererAddress, ordererTLSCert)
	if err != nil {
		return "", fmt.Errorf("failed to create orderer connection: %w", err)
	}
	defer ordererConn.Close()
	ordererClient, err := ordererapi.NewAtomicBroadcastClient(ordererConn).Broadcast(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to create orderer client: %w", err)
	}
	err = ordererClient.Send(envelope)
	if err != nil {
		return "", fmt.Errorf("failed to send envelope: %w", err)
	}
	response, err := ordererClient.Recv()
	if err != nil {
		return "", fmt.Errorf("failed to receive response: %w", err)
	}
	return response.String(), nil

}

// CreateOrdererConnection establishes a gRPC connection to an orderer
func (d *FabricDeployer) createOrdererConnection(ordererURL string, ordererTLSCACert string) (*grpc.ClientConn, error) {
	d.logger.Info("Creating orderer connection",
		"ordererURL", ordererURL)

	// Create a network node with the orderer details
	networkNode := network.Node{
		Addr:          ordererURL,
		TLSCACertByte: []byte(ordererTLSCACert),
	}

	// Establish connection to the orderer
	ordererConn, err := network.DialConnection(networkNode)
	if err != nil {
		return nil, fmt.Errorf("failed to dial orderer connection: %w", err)
	}

	return ordererConn, nil
}

// PrepareConfigUpdate prepares a config update for the given operations
func (d *FabricDeployer) PrepareConfigUpdate(ctx context.Context, networkID int64, operations []ConfigUpdateOperation) (*ConfigUpdateProposal, error) {
	// Get network details
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	// Get current channel config
	configBlock, err := d.FetchCurrentChannelConfig(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current channel config: %w", err)
	}

	// Extract config from block
	block := &cb.Block{}
	if err := proto.Unmarshal(configBlock, block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config block: %w", err)
	}

	config, err := ExtractConfigFromBlock(block)
	if err != nil {
		return nil, fmt.Errorf("failed to extract config from block: %w", err)
	}

	// Create a copy of the config to modify
	c := configtx.New(config)
	// Apply each operation to the config
	for _, operation := range operations {
		// Create a config modifier for the operation
		modifier, err := CreateConfigModifier(operation)
		if err != nil {
			return nil, fmt.Errorf("failed to create config modifier: %w", err)
		}

		// Apply the operation to the config
		if err := modifier.Modify(ctx, &c); err != nil {
			return nil, fmt.Errorf("failed to apply operation %s: %w", operation.Type, err)
		}
	}

	// Compute config update
	configUpdateBytes, err := c.ComputeMarshaledUpdate(network.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to compute config update: %w", err)
	}
	configUpdate := &cb.ConfigUpdate{}
	err = proto.Unmarshal(configUpdateBytes, configUpdate)
	if err != nil {
		return nil, err
	}
	channelConfigBytes, err := CreateConfigUpdateEnvelope(network.Name, configUpdate)
	if err != nil {
		return nil, err
	}
	// Create config update proposal
	proposal := &ConfigUpdateProposal{
		ID:                   uuid.New().String(),
		NetworkID:            networkID,
		ChannelName:          network.Name,
		Operations:           operations,
		ConfigUpdateEnvelope: channelConfigBytes,
		CreatedAt:            time.Now(),
		Status:               "proposed",
	}

	return proposal, nil
}

// Add this near the top of the file with other constants
const networkConfigTemplate = `
name: {{.Name}}
version: 1.0.0

client:
  organization: {{.Organization}}
  credentialStore:
    path: /tmp/state-store
    cryptoStore:
      path: /tmp/msp

organizations:
  {{.Organization}}:
    mspid: {{.Organization}}
    certificateAuthorities:
    - {{.Organization}}-ca
    peers:
      {{- range $name, $peer := .Peers}}
      - {{$name}}
      {{- end}}

    users:
       admin: 
          cert: 
            pem: |
{{.AdminKey.Certificate | indent 16}}
          key:
            pem: |
{{.AdminKey.PrivateKey | indent 16}}

orderers:
  {{- range $name, $orderer := .Orderers}}
  {{$name}}:
    url: {{$orderer.URL}}
    tlsCACerts:
      pem: |
{{$orderer.TLSCert|indent 10}}
  {{- end}}

peers:
  {{- range $name, $peer := .Peers}}
  {{$name}}:
    url: {{$peer.URL}}
    tlsCACerts:
      pem: |
{{$peer.TLSCert|indent 10}}
  {{- end}}

channels:
  {{.ChannelName}}:
    orderers:
    {{- range $name, $_ := .Orderers}}
    - {{$name}}
    {{- end}}
    peers:
    {{- range $name, $_ := .Peers}}
      {{$name}}:
         discover: true
         endorsingPeer: true
         chaincodeQuery: true
         ledgerQuery: true
         eventSource: true
    {{- end}}

`

// NetworkConfigData holds the data for the network config template
type NetworkConfigData struct {
	Name         string
	Organization string
	ChannelName  string
	AdminKey     struct {
		PrivateKey  string
		Certificate string
	}
	Orderers map[string]struct {
		URL     string
		TLSCert string
	}
	Peers map[string]struct {
		URL     string
		TLSCert string
	}
}

// OrdererInfo holds the information for an orderer
type OrdererInfo struct {
	URL     string
	TLSCert string
}

// FabricDeployer implements the NetworkDeployer interface for Hyperledger Fabric
type FabricDeployer struct {
	db             *db.Queries
	channelService *channel.ChannelService
	logger         *logger.Logger
	nodes          *nodeservice.NodeService
	keyMgmt        *keymanagement.KeyManagementService
	orgService     *orgservicefabric.OrganizationService
}

// NewFabricDeployer creates a new FabricDeployer instance
func NewFabricDeployer(db *db.Queries, nodes *nodeservice.NodeService, keyMgmt *keymanagement.KeyManagementService, orgService *orgservicefabric.OrganizationService) *FabricDeployer {
	channelService := channel.NewChannelService()
	logger := logger.NewDefault().With("component", "fabric_deployer")
	return &FabricDeployer{
		db:             db,
		channelService: channelService,
		logger:         logger,
		nodes:          nodes,
		keyMgmt:        keyMgmt,
		orgService:     orgService,
	}
}

// CreateGenesisBlock generates a genesis block for a new Fabric network
func (d *FabricDeployer) CreateGenesisBlock(networkID int64, config interface{}) ([]byte, error) {
	fabricConfig, ok := config.(*types.FabricNetworkConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type: expected FabricNetworkConfig")
	}
	ctx := context.Background()
	d.logger.Info("Creating genesis block for channel %s with %d organizations", fabricConfig.ChannelName, len(fabricConfig.PeerOrganizations))
	peerOrgs := []channel.Organization{}
	ordererOrgs := []channel.Organization{}
	consenters := []channel.AddressWithCerts{}
	nodes, err := d.nodes.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes by organization ID: %w", err)
	}
	listCreateNetworkNodes := []*db.CreateNetworkNodeParams{}

	// Handle internal peer organizations
	for _, org := range fabricConfig.PeerOrganizations {
		org, err := d.db.GetFabricOrganizationByID(ctx, org.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get organization by MSPID: %w", err)
		}

		signKey, err := d.keyMgmt.GetKey(ctx, int(org.SignKeyID.Int64))
		if err != nil {
			return nil, fmt.Errorf("failed to get sign key: %w", err)
		}
		tlsKey, err := d.keyMgmt.GetKey(ctx, int(org.TlsRootKeyID.Int64))
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS root key: %w", err)
		}
		if signKey == nil || tlsKey == nil {
			return nil, fmt.Errorf("failed to get sign key or TLS root key")
		}
		if signKey.Certificate == nil || tlsKey.Certificate == nil {
			return nil, fmt.Errorf("failed to get sign certificate or TLS root certificate")
		}
		signCACert := *signKey.Certificate
		tlsCACert := *tlsKey.Certificate

		orgNodes := []nodeservice.NodeResponse{}
		for _, node := range nodes.Items {
			if node.FabricPeer != nil {
				if node.FabricPeer.MSPID == org.MspID {
					orgNodes = append(orgNodes, node)
				}
			}
		}
		peerNodes := []nodeservice.NodeResponse{}
		for _, node := range orgNodes {

			peerNodes = append(peerNodes, node)
			listCreateNetworkNodes = append(listCreateNetworkNodes, &db.CreateNetworkNodeParams{
				NetworkID: networkID,
				NodeID:    node.ID,
				Status:    "pending",
				Role:      "peer",
			})
		}
		if len(peerNodes) > 0 {
			anchorPeers := []channel.HostPort{}

			for _, peerNode := range peerNodes {
				externalEndpoint := peerNode.FabricPeer.ExternalEndpoint
				if externalEndpoint == "" {
					externalEndpoint = peerNode.Endpoint
				}
				host, portStr, err := net.SplitHostPort(externalEndpoint)
				if err != nil {
					return nil, fmt.Errorf("failed to split host/port from endpoint %s: %w", externalEndpoint, err)
				}
				port, err := strconv.Atoi(portStr)
				if err != nil {
					return nil, fmt.Errorf("failed to parse port number %s: %w", portStr, err)
				}
				anchorPeers = append(anchorPeers, channel.HostPort{
					Host: host,
					Port: port,
				})
			}

			peerOrgs = append(peerOrgs, channel.Organization{
				Name:             org.MspID,
				AnchorPeers:      anchorPeers,
				SignCACert:       signCACert,
				TLSCACert:        tlsCACert,
				OrdererEndpoints: []string{},
			})
		}

	}

	// Handle internal orderer organizations
	for _, org := range fabricConfig.OrdererOrganizations {
		fabricOrgDB, err := d.db.GetFabricOrganizationByID(ctx, org.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get organization by MSPID: %w", err)
		}

		signKey, err := d.keyMgmt.GetKey(ctx, int(fabricOrgDB.SignKeyID.Int64))
		if err != nil {
			return nil, fmt.Errorf("failed to get sign key: %w", err)
		}
		tlsKey, err := d.keyMgmt.GetKey(ctx, int(fabricOrgDB.TlsRootKeyID.Int64))
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS root key: %w", err)
		}
		if signKey == nil || tlsKey == nil {
			return nil, fmt.Errorf("failed to get sign key or TLS root key")
		}
		if signKey.Certificate == nil || tlsKey.Certificate == nil {
			return nil, fmt.Errorf("failed to get sign certificate or TLS root certificate")
		}
		signCACert := *signKey.Certificate
		tlsCACert := *tlsKey.Certificate

		// Get orderer nodes for this organization
		ordererNodes := []*nodeservice.NodeResponse{}
		for _, nodeID := range org.NodeIDs {
			node, err := d.nodes.GetNode(ctx, nodeID)
			if err != nil {
				return nil, fmt.Errorf("failed to get node: %w", err)
			}
			if node.NodeType == nodetypes.NodeTypeFabricOrderer {
				ordererNodes = append(ordererNodes, node)
				listCreateNetworkNodes = append(listCreateNetworkNodes, &db.CreateNetworkNodeParams{
					NetworkID: networkID,
					NodeID:    node.ID,
					Status:    "pending",
					Role:      "orderer",
				})
			}
		}

		if len(ordererNodes) > 0 {
			ordererEndpoints := []string{}
			for _, ordererNode := range ordererNodes {
				ordererEndpoint := ordererNode.FabricOrderer.ExternalEndpoint
				if strings.Trim(ordererEndpoint, " ") == "" {
					return nil, fmt.Errorf("orderer node %s has no endpoint", ordererNode.Name)
				}
				ordererEndpoints = append(ordererEndpoints, ordererEndpoint)
			}

			ordererOrgs = append(ordererOrgs, channel.Organization{
				Name:             fabricOrgDB.MspID,
				AnchorPeers:      []channel.HostPort{},
				OrdererEndpoints: ordererEndpoints,
				SignCACert:       signCACert,
				TLSCACert:        tlsCACert,
			})

			// Add consenters for each orderer node
			for _, ordererNode := range ordererNodes {
				nodeTlsKey, err := d.keyMgmt.GetKey(ctx, int(ordererNode.FabricOrderer.TLSKeyID))
				if err != nil {
					return nil, fmt.Errorf("failed to get orderer TLS key: %w", err)
				}
				if nodeTlsKey == nil {
					return nil, fmt.Errorf("failed to get orderer TLS root key")
				}
				if nodeTlsKey.Certificate == nil {
					return nil, fmt.Errorf("failed to get orderer TLS certificate")
				}
				nodeTlsCert := *nodeTlsKey.Certificate
				externalEndpoint := ordererNode.FabricOrderer.ExternalEndpoint
				if externalEndpoint == "" {
					externalEndpoint = ordererNode.Endpoint
				}
				host, portStr, err := net.SplitHostPort(externalEndpoint)
				if err != nil {
					return nil, fmt.Errorf("failed to split host/port from endpoint %s: %w", externalEndpoint, err)
				}
				port, err := strconv.Atoi(portStr)
				if err != nil {
					return nil, fmt.Errorf("failed to parse port number %s: %w", portStr, err)
				}
				consenters = append(consenters, channel.AddressWithCerts{
					Address: channel.HostPort{
						Host: host,
						Port: port,
					},
					ClientTLSCert: nodeTlsCert,
					ServerTLSCert: nodeTlsCert,
				})
			}
		}
	}

	createReq := channel.CreateChannelInput{
		Name:        fabricConfig.ChannelName,
		Consenters:  consenters,
		PeerOrgs:    peerOrgs,
		OrdererOrgs: ordererOrgs,
	}
	d.logger.Debug("Creating channel with request: %+v", createReq)
	channel, err := d.channelService.CreateChannel(createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}
	channelConfig, err := base64.StdEncoding.DecodeString(channel.ConfigData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode channel config: %w", err)
	}

	// Update network with genesis block
	_, err = d.db.UpdateNetworkGenesisBlock(context.Background(), &db.UpdateNetworkGenesisBlockParams{
		ID: networkID,
		GenesisBlockB64: sql.NullString{
			String: channel.ConfigData, // Store base64 encoded genesis block
			Valid:  true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update network genesis block: %w", err)
	}

	// After the channel creation, create all network nodes
	for _, networkNode := range listCreateNetworkNodes {
		_, err = d.db.CreateNetworkNode(ctx, networkNode)
		if err != nil {
			return nil, fmt.Errorf("failed to create network node: %w", err)
		}
	}

	return channelConfig, nil // Return the decoded genesis block
}

// JoinNode joins a node (peer or orderer) to the Fabric network
func (d *FabricDeployer) JoinNode(networkId int64, genesisBlock []byte, nodeID int64) error {
	ctx := context.Background()
	node, err := d.nodes.GetNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}
	if node.NodeType != nodetypes.NodeTypeFabricPeer && node.NodeType != nodetypes.NodeTypeFabricOrderer {
		return fmt.Errorf("node type %s is not supported", node.NodeType)
	}

	// Get network ID from genesis block
	block := &cb.Block{}
	if err := proto.Unmarshal(genesisBlock, block); err != nil {
		return fmt.Errorf("failed to unmarshal genesis block: %w", err)
	}

	// Get network from genesis block
	network, err := d.db.GetNetwork(ctx, networkId)
	if err != nil {
		return fmt.Errorf("failed to get network from genesis block: %w", err)
	}

	// Join the node based on its type
	switch node.NodeType {
	case nodetypes.NodeTypeFabricPeer:
		peer, err := d.nodes.GetFabricPeer(ctx, nodeID)
		if err != nil {
			return fmt.Errorf("failed to get peer: %w", err)
		}
		if err = peer.JoinChannel(genesisBlock); err != nil {
			return fmt.Errorf("failed to join peer to channel: %w", err)
		}
	case nodetypes.NodeTypeFabricOrderer:
		orderer, err := d.nodes.GetFabricOrderer(ctx, nodeID)
		if err != nil {
			return fmt.Errorf("failed to get orderer: %w", err)
		}
		if err = orderer.JoinChannel(genesisBlock); err != nil {
			return fmt.Errorf("failed to join orderer to channel: %w", err)
		}
	default:
		return fmt.Errorf("unsupported node type: %s", node.NodeType)
	}

	// Update network node status to "joined"
	_, err = d.db.UpdateNetworkNodeStatus(ctx, &db.UpdateNetworkNodeStatusParams{
		NetworkID: network.ID,
		NodeID:    nodeID,
		Status:    "joined",
	})
	if err != nil {
		return fmt.Errorf("failed to update network node status: %w", err)
	}

	return nil
}

// GetStatus retrieves the current status of the Fabric network
func (d *FabricDeployer) GetStatus(networkID int64) (*types.NetworkDeploymentStatus, error) {
	// TODO: Implement status check
	// 1. Query network details from database
	// 2. Check node statuses
	// 3. Aggregate network status
	return nil, fmt.Errorf("not implemented")
}

// GetCurrentChannelConfig retrieves and decodes the current channel configuration
func (d *FabricDeployer) GetCurrentChannelConfig(networkID int64) ([]byte, error) {
	ctx := context.Background()

	// Get network from database
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	// Decode base64 config block
	blockBytes, err := base64.StdEncoding.DecodeString(network.CurrentConfigBlockB64.String)
	if err != nil {
		return nil, fmt.Errorf("failed to decode current config block: %w", err)
	}

	// Unmarshal to Block
	block := &cb.Block{}
	err = proto.Unmarshal(blockBytes, block)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal current config block: %w", err)
	}

	return blockBytes, nil
}

func (d *FabricDeployer) GetCurrentChannelConfigAsMap(networkID int64) (map[string]interface{}, error) {
	ctx := context.Background()
	configBlock, err := d.FetchCurrentChannelConfig(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current channel config: %w", err)
	}

	block := &cb.Block{}
	err = proto.Unmarshal(configBlock, block)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal current config block: %w", err)
	}
	buffer := &bytes.Buffer{}
	err = protolator.DeepMarshalJSON(buffer, block)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal block to JSON: %w", err)
	}

	// Parse JSON into map
	var configMap map[string]interface{}
	err = json.Unmarshal(buffer.Bytes(), &configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to map: %w", err)
	}

	return configMap, nil
}

// GetChannelConfig retrieves and decodes the channel configuration from the genesis block
func (d *FabricDeployer) GetChannelConfig(networkID int64) (map[string]interface{}, error) {
	ctx := context.Background()

	// Get network from database
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	// Check if genesis block exists
	if !network.GenesisBlockB64.Valid {
		return nil, fmt.Errorf("genesis block not found for network %d", networkID)
	}

	// Decode base64 genesis block
	blockBytes, err := base64.StdEncoding.DecodeString(network.GenesisBlockB64.String)
	if err != nil {
		return nil, fmt.Errorf("failed to decode genesis block: %w", err)
	}

	// Unmarshal to Block
	block := &cb.Block{}
	err = proto.Unmarshal(blockBytes, block)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal genesis block: %w", err)
	}

	// Use protolator to convert block to JSON
	buffer := &bytes.Buffer{}
	err = protolator.DeepMarshalJSON(buffer, block)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal block to JSON: %w", err)
	}

	// Parse JSON into map
	var configMap map[string]interface{}
	err = json.Unmarshal(buffer.Bytes(), &configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to map: %w", err)
	}

	return configMap, nil
}

// RemoveNode removes a node (peer or orderer) from the Fabric network
func (d *FabricDeployer) RemoveNode(networkID int64, nodeID int64) error {
	ctx := context.Background()

	// Get network details
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}

	// Get node details
	node, err := d.nodes.GetNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Get channel name from network config
	var fabricConfig types.FabricNetworkConfig
	if err := json.Unmarshal([]byte(network.Config.String), &fabricConfig); err != nil {
		return fmt.Errorf("failed to unmarshal network config: %w", err)
	}

	// Remove the node based on its type
	switch node.NodeType {
	case nodetypes.NodeTypeFabricPeer:
		peer, err := d.nodes.GetFabricPeer(ctx, nodeID)
		if err != nil {
			return fmt.Errorf("failed to get peer: %w", err)
		}
		if err = peer.LeaveChannel(fabricConfig.ChannelName); err != nil {
			return fmt.Errorf("failed to remove peer from channel: %w", err)
		}
	case nodetypes.NodeTypeFabricOrderer:
		orderer, err := d.nodes.GetFabricOrderer(ctx, nodeID)
		if err != nil {
			return fmt.Errorf("failed to get orderer: %w", err)
		}
		if err = orderer.LeaveChannel(fabricConfig.ChannelName); err != nil {
			return fmt.Errorf("failed to remove orderer from channel: %w", err)
		}
	default:
		return fmt.Errorf("unsupported node type: %s", node.NodeType)
	}

	// Update network node status to "removed"
	_, err = d.db.UpdateNetworkNodeStatus(ctx, &db.UpdateNetworkNodeStatusParams{
		NetworkID: network.ID,
		NodeID:    nodeID,
		Status:    "removed",
	})
	if err != nil {
		return fmt.Errorf("failed to update network node status: %w", err)
	}

	return nil
}

// UnjoinNode removes a node from the channel but keeps it in the network
func (d *FabricDeployer) UnjoinNode(networkID int64, nodeID int64) error {
	ctx := context.Background()

	// Get network details
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}

	// Get node details
	node, err := d.nodes.GetNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Get channel name from network config
	var fabricConfig types.FabricNetworkConfig
	if err := json.Unmarshal([]byte(network.Config.String), &fabricConfig); err != nil {
		return fmt.Errorf("failed to unmarshal network config: %w", err)
	}

	// Remove the node based on its type
	switch node.NodeType {
	case nodetypes.NodeTypeFabricPeer:
		peer, err := d.nodes.GetFabricPeer(ctx, nodeID)
		if err != nil {
			return fmt.Errorf("failed to get peer: %w", err)
		}
		if err = peer.LeaveChannel(fabricConfig.ChannelName); err != nil {
			return fmt.Errorf("failed to unjoin peer from channel: %w", err)
		}
	case nodetypes.NodeTypeFabricOrderer:
		orderer, err := d.nodes.GetFabricOrderer(ctx, nodeID)
		if err != nil {
			return fmt.Errorf("failed to get orderer: %w", err)
		}
		if err = orderer.LeaveChannel(fabricConfig.ChannelName); err != nil {
			return fmt.Errorf("failed to unjoin orderer from channel: %w", err)
		}
	default:
		return fmt.Errorf("unsupported node type: %s", node.NodeType)
	}

	// Update network node status to "unjoined"
	_, err = d.db.UpdateNetworkNodeStatus(ctx, &db.UpdateNetworkNodeStatusParams{
		NetworkID: network.ID,
		NodeID:    nodeID,
		Status:    "unjoined",
	})
	if err != nil {
		return fmt.Errorf("failed to update network node status: %w", err)
	}

	return nil
}

type UpdateOrganizationCRLInput struct {
	OrganizationID int64 `json:"organizationId" validate:"required"`
}

func (d *FabricDeployer) UpdateOrganizationCRL(ctx context.Context, networkID int64, input UpdateOrganizationCRLInput) (string, error) {
	// Get network details
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to get network: %w", err)
	}

	// Get organization details
	org, err := d.db.GetFabricOrganizationByID(ctx, input.OrganizationID)
	if err != nil {
		return "", fmt.Errorf("failed to get organization: %w", err)
	}

	// Get the CRL from organization service
	crl, err := d.orgService.GetCRL(ctx, input.OrganizationID)
	if err != nil {
		return "", fmt.Errorf("failed to get CRL: %w", err)
	}

	// Get a peer from the organization to submit the update
	nodes, err := d.nodes.GetFabricNodesByOrganization(ctx, input.OrganizationID)
	if err != nil {
		return "", fmt.Errorf("failed to get organization nodes: %w", err)
	}

	var peer *nodeservice.NodeResponse
	for _, node := range nodes {
		if node.NodeType == nodetypes.NodeTypeFabricPeer {
			peer = &node
			break
		}
	}
	if peer == nil {
		return "", fmt.Errorf("no peer found for organization %d", input.OrganizationID)
	}

	// Get an orderer node from the network
	networkNodes, err := d.db.GetNetworkNodes(ctx, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to get network nodes: %w", err)
	}

	var orderer *db.GetNetworkNodesRow
	for _, node := range networkNodes {
		if node.NodeType.String == string(nodetypes.NodeTypeFabricOrderer) {
			orderer = node
			break
		}
	}
	if orderer == nil {
		return "", fmt.Errorf("no orderer found in network %d", networkID)
	}

	// Get orderer TLS CA cert
	ordererConfig := &nodetypes.FabricOrdererDeploymentConfig{}
	if err := json.Unmarshal([]byte(orderer.DeploymentConfig.String), ordererConfig); err != nil {
		return "", fmt.Errorf("failed to unmarshal orderer config: %w", err)
	}

	ordererTLSKey, err := d.keyMgmt.GetKey(ctx, int(ordererConfig.TLSKeyID))
	if err != nil {
		return "", fmt.Errorf("failed to get orderer TLS key: %w", err)
	}
	if ordererTLSKey.Certificate == nil {
		return "", fmt.Errorf("orderer TLS certificate not found")
	}
	ordererURL := ordererConfig.GetAddress()
	ordererCert := *ordererTLSKey.Certificate

	p, err := d.nodes.GetFabricPeer(ctx, peer.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get fabric peer: %w", err)
	}
	channelConfig, err := p.GetChannelConfig(ctx, network.Name, ordererURL, ordererCert)
	if err != nil {
		return "", fmt.Errorf("failed to get channel config: %w", err)
	}

	// Get channel name from network config
	var fabricConfig types.FabricNetworkConfig
	if err := json.Unmarshal([]byte(network.Config.String), &fabricConfig); err != nil {
		return "", fmt.Errorf("failed to unmarshal network config: %w", err)
	}

	// Generate channel update
	channelUpdate, err := d.channelService.SetCRL(&channel.SetCRLInput{
		CurrentConfig: channelConfig.ChannelGroup,
		ChannelName:   fabricConfig.ChannelName,
		MSPID:         org.MspID,
		CRL:           crl,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create anchor peer update: %w", err)
	}

	// Get peer instance
	fabricPeer, err := d.nodes.GetFabricPeer(ctx, peer.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get fabric peer: %w", err)
	}

	// Save channel config
	resp, err := fabricPeer.SaveChannelConfig(ctx,
		fabricConfig.ChannelName,
		ordererURL,
		*ordererTLSKey.Certificate,
		channelUpdate,
	)
	if err != nil {
		return "", fmt.Errorf("failed to save channel config: %w", err)
	}

	return resp.TransactionID, nil

}

type CreateAnchorPeerUpdateInput struct {
	ChannelName string
	OrgMSPID    string
	AnchorPeers []types.HostPort
}

// SetAnchorPeers sets the anchor peers for an organization in the channel
func (d *FabricDeployer) SetAnchorPeers(ctx context.Context, networkID int64, organizationID int64, anchorPeers []types.HostPort) (string, error) {
	// Get network details
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to get network: %w", err)
	}

	// Get organization details
	org, err := d.db.GetFabricOrganizationByID(ctx, organizationID)
	if err != nil {
		return "", fmt.Errorf("failed to get organization: %w", err)
	}

	// Get a peer from the organization to submit the update
	nodes, err := d.nodes.GetFabricNodesByOrganization(ctx, organizationID)
	if err != nil {
		return "", fmt.Errorf("failed to get organization nodes: %w", err)
	}

	var peer *nodeservice.NodeResponse
	for _, node := range nodes {
		if node.NodeType == nodetypes.NodeTypeFabricPeer {
			peer = &node
			break
		}
	}
	if peer == nil {
		return "", fmt.Errorf("no peer found for organization %d", organizationID)
	}

	// Get an orderer node from the network
	networkNodes, err := d.db.GetNetworkNodes(ctx, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to get network nodes: %w", err)
	}

	var orderer *db.GetNetworkNodesRow
	for _, node := range networkNodes {
		if node.NodeType.String == string(nodetypes.NodeTypeFabricOrderer) {
			orderer = node
			break
		}
	}
	if orderer == nil {
		return "", fmt.Errorf("no orderer found in network %d", networkID)
	}

	// Get orderer TLS CA cert
	ordererConfig := &nodetypes.FabricOrdererDeploymentConfig{}
	if err := json.Unmarshal([]byte(orderer.DeploymentConfig.String), ordererConfig); err != nil {
		return "", fmt.Errorf("failed to unmarshal orderer config: %w", err)
	}

	ordererTLSKey, err := d.keyMgmt.GetKey(ctx, int(ordererConfig.TLSKeyID))
	if err != nil {
		return "", fmt.Errorf("failed to get orderer TLS key: %w", err)
	}
	if ordererTLSKey.Certificate == nil {
		return "", fmt.Errorf("orderer TLS certificate not found")
	}
	ordererURL := ordererConfig.GetAddress()
	ordererCert := *ordererTLSKey.Certificate

	p, err := d.nodes.GetFabricPeer(ctx, peer.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get fabric peer: %w", err)
	}
	channelConfig, err := p.GetChannelConfig(ctx, network.Name, ordererURL, ordererCert)
	if err != nil {
		return "", fmt.Errorf("failed to get channel config: %w", err)
	}

	// Convert anchor peers to channel format
	channelAnchorPeers := make([]channel.HostPort, len(anchorPeers))
	for i, ap := range anchorPeers {
		channelAnchorPeers[i] = channel.HostPort{
			Host: ap.Host,
			Port: ap.Port,
		}
	}

	// Get channel name from network config
	var fabricConfig types.FabricNetworkConfig
	if err := json.Unmarshal([]byte(network.Config.String), &fabricConfig); err != nil {
		return "", fmt.Errorf("failed to unmarshal network config: %w", err)
	}

	// Generate channel update
	channelUpdate, err := d.channelService.SetAnchorPeers(&channel.SetAnchorPeersInput{
		ChannelName:   fabricConfig.ChannelName,
		MSPID:         org.MspID,
		AnchorPeers:   channelAnchorPeers,
		CurrentConfig: channelConfig.ChannelGroup,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create anchor peer update: %w", err)
	}

	// Get peer instance
	fabricPeer, err := d.nodes.GetFabricPeer(ctx, peer.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get fabric peer: %w", err)
	}

	// Save channel config
	resp, err := fabricPeer.SaveChannelConfig(ctx,
		fabricConfig.ChannelName,
		ordererURL,
		*ordererTLSKey.Certificate,
		channelUpdate,
	)
	if err != nil {
		return "", fmt.Errorf("failed to save channel config: %w", err)
	}

	return resp.TransactionID, nil
}

// GenerateNetworkConfig generates a network configuration YAML for a specific organization
func (d *FabricDeployer) GenerateNetworkConfig(ctx context.Context, networkID int64, orgID int64) (string, error) {

	// Get network details
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to get network: %w", err)
	}

	// Get organization details
	org, err := d.db.GetFabricOrganizationByID(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to get organization: %w", err)
	}

	// Get admin key
	adminKey, err := d.keyMgmt.GetKey(ctx, int(org.AdminSignKeyID.Int64))
	if err != nil {
		return "", fmt.Errorf("failed to get admin key: %w", err)
	}
	adminPrivateKey, err := d.keyMgmt.GetDecryptedPrivateKey(int(org.AdminSignKeyID.Int64))
	if err != nil {
		return "", fmt.Errorf("failed to get admin private key: %w", err)
	}
	// Get network nodes
	networkNodes, err := d.db.GetNetworkNodes(ctx, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to get network nodes: %w", err)
	}

	// Prepare template data
	data := NetworkConfigData{
		Name:         network.Name,
		Organization: org.MspID,
		ChannelName:  network.Name,
		AdminKey: struct {
			PrivateKey  string
			Certificate string
		}{
			PrivateKey:  string(adminPrivateKey),
			Certificate: string(*adminKey.Certificate),
		},
		Orderers: make(map[string]struct {
			URL     string
			TLSCert string
		}),
		Peers: make(map[string]struct {
			URL     string
			TLSCert string
		}),
	}

	// Process all nodes
	for _, node := range networkNodes {
		nodeDetails, err := d.nodes.GetNode(ctx, node.NodeID)
		if err != nil {
			return "", fmt.Errorf("failed to get node details: %w", err)
		}

		switch nodeDetails.NodeType {
		case nodetypes.NodeTypeFabricPeer:
			peerConfig := nodeDetails.FabricPeer
			peerTLSKey, err := d.keyMgmt.GetKey(ctx, int(peerConfig.TLSKeyID))
			if err != nil {
				return "", fmt.Errorf("failed to get peer TLS key: %w", err)
			}

			data.Peers[nodeDetails.Name] = struct {
				URL     string
				TLSCert string
			}{
				URL:     fmt.Sprintf("grpcs://%s", peerConfig.ExternalEndpoint),
				TLSCert: string(*peerTLSKey.Certificate),
			}

		case nodetypes.NodeTypeFabricOrderer:
			ordererConfig := nodeDetails.FabricOrderer
			ordererTLSKey, err := d.keyMgmt.GetKey(ctx, int(ordererConfig.TLSKeyID))
			if err != nil {
				return "", fmt.Errorf("failed to get orderer TLS key: %w", err)
			}

			data.Orderers[nodeDetails.Name] = struct {
				URL     string
				TLSCert string
			}{
				URL:     fmt.Sprintf("grpcs://%s", ordererConfig.ExternalEndpoint),
				TLSCert: string(*ordererTLSKey.Certificate),
			}
		}
	}

	// Create template with sprig functions
	tmpl, err := template.New("networkConfig").
		Funcs(sprig.TxtFuncMap()).
		Parse(networkConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse network config template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute network config template: %w", err)
	}

	return buf.String(), nil
}

// Update GetAllNodes call to handle both return values
func (d *FabricDeployer) GetNetworkNodes(ctx context.Context) ([]nodeservice.Node, error) {
	// Use GetNodeWithConfig instead of GetAllNodes since we need the full Node type
	dbNodes, err := d.db.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	nodes := make([]nodeservice.Node, len(dbNodes))
	for i, dbNode := range dbNodes {
		node, err := d.nodes.GetNodeWithConfig(ctx, dbNode.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get node config: %w", err)
		}
		nodes[i] = *node
	}
	return nodes, nil
}

// Update node handling to use GetNodeWithConfig
func (d *FabricDeployer) deployNode(ctx context.Context, nodeID int64) error {
	node, err := d.nodes.GetNodeWithConfig(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	if node.DeploymentConfig == nil {
		return fmt.Errorf("node %d has no deployment config", node.ID)
	}

	// Rest of the deployment logic...
	return nil
}

// Update nodeDetails handling to use GetNodeWithConfig
func (d *FabricDeployer) handleNodeDetails(ctx context.Context, nodeID int64) error {
	nodeDetails, err := d.nodes.GetNodeWithConfig(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node details: %w", err)
	}

	if nodeDetails.DeploymentConfig == nil {
		return fmt.Errorf("node has no deployment config")
	}

	// Use nodeDetails.DeploymentConfig as before
	return nil
}

func (d *FabricDeployer) GetOrderersForNetwork(ctx context.Context, networkID int64) ([]*OrdererInfo, error) {

	// First try to get orderers from active network nodes
	networkNodes, err := d.db.GetNetworkNodes(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network nodes: %w", err)
	}

	var orderers []*OrdererInfo
	for _, node := range networkNodes {
		if node.NodeType.String == string(nodetypes.NodeTypeFabricOrderer) && node.Status == "joined" {
			ordererNode, err := d.nodes.GetNodeByID(ctx, node.NodeID)
			if err != nil {
				continue
			}
			ordererConfig := ordererNode.FabricOrderer
			orderers = append(orderers, &OrdererInfo{
				URL:     fmt.Sprintf("grpcs://%s", ordererConfig.ExternalEndpoint),
				TLSCert: ordererConfig.TLSCACert,
			})
		}
	}

	// If no active orderers found, try to get from genesis block
	if len(orderers) == 0 {
		genesisOrderers, err := d.GetOrderersFromGenesisBlock(ctx, networkID)
		if err != nil {
			return nil, fmt.Errorf("failed to get orderers from genesis block: %w", err)
		}
		orderers = genesisOrderers
	}

	if len(orderers) == 0 {
		return nil, fmt.Errorf("no orderer nodes found for network %d", networkID)
	}

	return orderers, nil
}

// FetchCurrentChannelConfig retrieves the current channel configuration directly from the network
func (d *FabricDeployer) FetchCurrentChannelConfig(ctx context.Context, networkID int64) ([]byte, error) {
	// Get network details
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	// Find a peer node to query
	networkNodes, err := d.db.GetNetworkNodes(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network nodes: %w", err)
	}
	firstNode := networkNodes[0]
	nodeResponse, err := d.nodes.GetNode(ctx, firstNode.NodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	// Get organization MSPID based on node type
	var mspID string
	switch nodeResponse.NodeType {
	case nodetypes.NodeTypeFabricPeer:
		if nodeResponse.FabricPeer == nil {
			return nil, fmt.Errorf("node %d is a peer but has no peer config", firstNode.NodeID)
		}
		mspID = nodeResponse.FabricPeer.MSPID
	case nodetypes.NodeTypeFabricOrderer:
		if nodeResponse.FabricOrderer == nil {
			return nil, fmt.Errorf("node %d is an orderer but has no orderer config", firstNode.NodeID)
		}
		mspID = nodeResponse.FabricOrderer.MSPID
	default:
		return nil, fmt.Errorf("unsupported node type %s for getting MSPID", nodeResponse.NodeType)
	}
	if mspID == "" {
		return nil, fmt.Errorf("mspID not found for node %d", firstNode.NodeID)
	}
	fabricOrg, err := d.orgService.GetOrganizationByMspID(ctx, mspID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}
	fabricOrgItem := fabricorg.NewOrganizationService(d.orgService, d.keyMgmt, d.logger, fabricOrg.MspID, d.db)

	// Get all available orderers - first from active nodes
	var orderersList []struct {
		address string
		tlsCert string
	}

	// First try to get orderers from active nodes
	for _, node := range networkNodes {
		if node.NodeType.String == string(nodetypes.NodeTypeFabricOrderer) && node.Status == "joined" {
			ordererNode, err := d.nodes.GetNodeByID(ctx, node.NodeID)
			if err != nil {
				d.logger.Warn("Failed to get orderer node", "nodeID", node.NodeID, "error", err)
				continue
			}
			ordererConfig := ordererNode.FabricOrderer
			// Get orderer TLS cert
			ordererTLSKey, err := d.keyMgmt.GetKey(ctx, int(ordererConfig.TLSKeyID))
			if err != nil || ordererTLSKey.Certificate == nil {
				d.logger.Warn("Failed to get orderer TLS cert", "nodeID", node.NodeID, "error", err)
				continue
			}
			orderersList = append(orderersList, struct {
				address string
				tlsCert string
			}{
				address: ordererConfig.ExternalEndpoint,
				tlsCert: *ordererTLSKey.Certificate,
			})
		}
	}

	// If no active orderers found, try to get from genesis block
	if len(orderersList) == 0 {
		orderers, err := d.GetOrderersFromGenesisBlock(ctx, networkID)
		if err != nil {
			return nil, fmt.Errorf("failed to get orderers from genesis block: %w", err)
		}
		for _, orderer := range orderers {
			// Remove the grpcs:// prefix if present
			address := orderer.URL
			if strings.HasPrefix(address, "grpcs://") {
				address = strings.TrimPrefix(address, "grpcs://")
			}
			orderersList = append(orderersList, struct {
				address string
				tlsCert string
			}{
				address: address,
				tlsCert: orderer.TLSCert,
			})
		}
	}

	if len(orderersList) == 0 {
		return nil, fmt.Errorf("no orderers found in network or genesis block")
	}

	// Try each orderer until one succeeds
	var lastErr error
	for _, orderer := range orderersList {
		d.logger.Info("Attempting to fetch channel config from orderer", "address", orderer.address)

		// Fetch channel config from orderer
		channelConfig, err := fabricOrgItem.GetConfigBlockWithNetworkConfig(ctx, network.Name, orderer.address, orderer.tlsCert)
		if err != nil {
			d.logger.Warn("Failed to get channel config from orderer", "address", orderer.address, "error", err)
			lastErr = err
			continue
		}

		// Marshal the config block
		configBytes, err := proto.Marshal(channelConfig)
		if err != nil {
			lastErr = fmt.Errorf("failed to marshal config block: %w", err)
			continue
		}

		// Update the current config block in the database
		configBase64 := base64.StdEncoding.EncodeToString(configBytes)
		err = d.db.UpdateNetworkCurrentConfigBlock(ctx, &db.UpdateNetworkCurrentConfigBlockParams{
			ID: networkID,
			CurrentConfigBlockB64: sql.NullString{
				String: configBase64,
				Valid:  true,
			},
		})
		if err != nil {
			lastErr = fmt.Errorf("failed to update network current config block: %w", err)
			continue
		}

		// Successfully fetched and stored config
		d.logger.Info("Successfully fetched channel config from orderer", "address", orderer.address)
		return configBytes, nil
	}

	// If we get here, all orderers failed
	return nil, fmt.Errorf("failed to fetch channel config from any orderer: %w", lastErr)
}

// GetOrdererInfoFromConfig extracts orderer information from a channel config
func (d *FabricDeployer) GetOrdererInfoFromConfig(configBlock map[string]interface{}) (*OrdererInfo, error) {
	// Extract orderer section from config block
	channelGroup, ok := configBlock["channel_group"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("channel_group not found in config block")
	}

	groups, ok := channelGroup["groups"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("groups not found in channel_group")
	}

	ordererSection, ok := groups["Orderer"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("orderer section not found in config block")
	}

	values, ok := ordererSection["values"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("values not found in orderer section")
	}

	// Extract orderer addresses
	ordererAddresses, ok := values["OrdererAddresses"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("OrdererAddresses not found in values")
	}

	addressesValue, ok := ordererAddresses["value"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value not found in OrdererAddresses")
	}

	addresses, ok := addressesValue["addresses"].([]interface{})
	if !ok || len(addresses) == 0 {
		return nil, fmt.Errorf("no orderer addresses found in config block")
	}

	// Extract TLS cert from orderer org
	msp, ok := values["MSP"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("MSP not found in values")
	}

	mspValue, ok := msp["value"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value not found in MSP")
	}

	tlsRootCerts, ok := mspValue["tls_root_certs"].([]interface{})
	if !ok || len(tlsRootCerts) == 0 {
		return nil, fmt.Errorf("tls_root_certs not found in MSP value")
	}

	tlsCert, ok := tlsRootCerts[0].(string)
	if !ok {
		return nil, fmt.Errorf("orderer TLS cert not found in config block")
	}

	return &OrdererInfo{
		URL:     fmt.Sprintf("grpcs://%s", addresses[0].(string)),
		TLSCert: tlsCert,
	}, nil
}

// SetAnchorPeersWithOrderer sets anchor peers using specified orderer information
func (d *FabricDeployer) SetAnchorPeersWithOrderer(ctx context.Context, networkID, organizationID int64, anchorPeers []types.HostPort, ordererURL, ordererTLSCert string) (string, error) {
	// Get network details
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to get network: %w", err)
	}

	// Get organization details
	org, err := d.db.GetFabricOrganizationByID(ctx, organizationID)
	if err != nil {
		return "", fmt.Errorf("failed to get organization: %w", err)
	}

	// Get a peer from the organization to submit the update
	nodes, err := d.nodes.GetFabricNodesByOrganization(ctx, organizationID)
	if err != nil {
		return "", fmt.Errorf("failed to get organization nodes: %w", err)
	}

	var peer *nodeservice.NodeResponse
	for _, node := range nodes {
		if node.NodeType == nodetypes.NodeTypeFabricPeer {
			peer = &node
			break
		}
	}
	if peer == nil {
		return "", fmt.Errorf("no peer found for organization %d", organizationID)
	}

	// Get peer instance
	p, err := d.nodes.GetFabricPeer(ctx, peer.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get fabric peer: %w", err)
	}

	// Get channel config from peer using provided orderer info
	channelConfig, err := p.GetChannelConfig(ctx, network.Name, ordererURL, ordererTLSCert)
	if err != nil {
		return "", fmt.Errorf("failed to get channel config: %w", err)
	}

	// Create config update envelope
	channelAnchorPeers := make([]channel.HostPort, len(anchorPeers))
	for i, ap := range anchorPeers {
		channelAnchorPeers[i] = channel.HostPort{
			Host: ap.Host,
			Port: ap.Port,
		}
	}
	// Generate channel update
	configUpdate, err := d.channelService.SetAnchorPeers(&channel.SetAnchorPeersInput{
		ChannelName:   network.Name,
		MSPID:         org.MspID,
		AnchorPeers:   channelAnchorPeers,
		CurrentConfig: channelConfig.ChannelGroup,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create anchor peer update: %w", err)
	}

	// Get peer instance
	fabricPeer, err := d.nodes.GetFabricPeer(ctx, peer.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get fabric peer: %w", err)
	}

	// Save channel config
	resp, err := fabricPeer.SaveChannelConfig(ctx,
		network.Name,
		ordererURL,
		ordererTLSCert,
		configUpdate,
	)
	if err != nil {
		return "", fmt.Errorf("failed to save channel config: %w", err)
	}

	return resp.TransactionID, nil
}

func (d *FabricDeployer) GetOrderersFromConfigBlock(ctx context.Context, blockBytes []byte) ([]*OrdererInfo, error) {

	block := &cb.Block{}
	err := proto.Unmarshal(blockBytes, block)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	cmnConfig, err := ExtractConfigFromBlock(block)
	if err != nil {
		return nil, fmt.Errorf("failed to extract config from block: %w", err)
	}

	cfgtx := configtx.New(cmnConfig)
	ordererConf, err := cfgtx.Orderer().Configuration()
	if err != nil {
		return nil, fmt.Errorf("failed to get orderer configuration: %w", err)
	}

	consenters := ordererConf.EtcdRaft.Consenters

	var orderers []*OrdererInfo
	for _, consent := range consenters {
		pemPk := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: consent.ServerTLSCert.Raw,
		})
		ordererAddress := fmt.Sprintf("%s:%d", consent.Address.Host, consent.Address.Port)
		orderers = append(orderers, &OrdererInfo{
			URL:     fmt.Sprintf("grpcs://%s", ordererAddress),
			TLSCert: string(pemPk),
		})
	}

	return orderers, nil
}

// GetOrderersFromGenesisBlock extracts orderer information from the genesis block
func (d *FabricDeployer) GetOrderersFromGenesisBlock(ctx context.Context, networkID int64) ([]*OrdererInfo, error) {
	// Get network details to access genesis block
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	if !network.GenesisBlockB64.Valid {
		return nil, fmt.Errorf("genesis block not found for network %d", networkID)
	}

	// Decode base64 genesis block
	blockBytes, err := base64.StdEncoding.DecodeString(network.GenesisBlockB64.String)
	if err != nil {
		return nil, fmt.Errorf("failed to decode genesis block: %w", err)
	}

	// Unmarshal to Block
	block := &cb.Block{}
	err = proto.Unmarshal(blockBytes, block)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal genesis block: %w", err)
	}

	cmnConfig, err := ExtractConfigFromBlock(block)
	if err != nil {
		return nil, fmt.Errorf("failed to extract config from block: %w", err)
	}

	cfgtx := configtx.New(cmnConfig)
	ordererConf, err := cfgtx.Orderer().Configuration()
	if err != nil {
		return nil, fmt.Errorf("failed to get orderer configuration: %w", err)
	}

	consenters := ordererConf.EtcdRaft.Consenters

	var orderers []*OrdererInfo
	for _, consent := range consenters {
		pemPk := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: consent.ServerTLSCert.Raw,
		})
		ordererAddress := fmt.Sprintf("%s:%d", consent.Address.Host, consent.Address.Port)
		orderers = append(orderers, &OrdererInfo{
			URL:     fmt.Sprintf("grpcs://%s", ordererAddress),
			TLSCert: string(pemPk),
		})
	}

	if len(orderers) == 0 {
		return nil, fmt.Errorf("no valid orderers found in genesis block")
	}

	return orderers, nil
}

// ImportNetworkWithOrg imports a Fabric network using organization details and orderer information
func (d *FabricDeployer) ImportNetworkWithOrg(ctx context.Context, channelID string, orgID int64, ordererURL string, ordererTLSCert []byte, description string) (string, error) {
	// Get organization details

	// Validate orderer URL format
	if !strings.HasPrefix(ordererURL, "grpcs://") {
		return "", fmt.Errorf("invalid orderer URL format: must start with grpcs://")
	}
	org, err := d.orgService.GetOrganization(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to get organization: %w", err)
	}
	orgService := fabricorg.NewOrganizationService(d.orgService, d.keyMgmt, d.logger, org.MspID, d.db)

	// Validate TLS certificate
	block, _ := pem.Decode(ordererTLSCert)
	if block == nil {
		return "", fmt.Errorf("failed to decode TLS certificate PEM")
	}

	_, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse TLS certificate: %w", err)
	}
	genesisBlock, err := orgService.GetGenesisBlock(ctx, channelID, ordererURL, ordererTLSCert)
	if err != nil {
		return "", fmt.Errorf("failed to get genesis block: %w", err)
	}
	// Create network config
	networkConfig := types.FabricNetworkConfig{
		BaseNetworkConfig: types.BaseNetworkConfig{
			Type: types.NetworkTypeFabric,
		},
		ChannelName: channelID,
		PeerOrganizations: []types.Organization{
			{
				ID:      orgID,
				NodeIDs: []int64{},
			},
		},
		OrdererOrganizations: []types.Organization{},
	}

	configBytes, err := json.Marshal(networkConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal network config: %w", err)
	}

	// Create network in database
	_, err = d.db.CreateNetworkFull(ctx, &db.CreateNetworkFullParams{
		Name:        channelID,
		Platform:    "fabric",
		Description: sql.NullString{String: description, Valid: description != ""},
		Status:      "imported",
		GenesisBlockB64: sql.NullString{
			String: base64.StdEncoding.EncodeToString(genesisBlock),
			Valid:  true,
		},
		NetworkID: sql.NullString{String: channelID, Valid: true},
		Config:    sql.NullString{String: string(configBytes), Valid: true},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create network in database: %w", err)
	}

	return channelID, nil
}

// ImportNetwork imports a Fabric network from a genesis block
func (d *FabricDeployer) ImportNetwork(ctx context.Context, genesisFile []byte, description string) (string, error) {
	// Parse and validate the genesis block
	block := &cb.Block{}
	if err := proto.Unmarshal(genesisFile, block); err != nil {
		return "", fmt.Errorf("failed to unmarshal genesis block: %w", err)
	}

	// Validate the block header
	if block.Header == nil {
		return "", fmt.Errorf("invalid genesis block: missing header")
	}

	// Additional Fabric-specific validations
	if block.Data == nil || len(block.Data.Data) == 0 {
		return "", fmt.Errorf("invalid genesis block: missing data")
	}

	// Extract channel name from block
	envelope := &cb.Envelope{}
	if err := proto.Unmarshal(block.Data.Data[0], envelope); err != nil {
		return "", fmt.Errorf("failed to unmarshal envelope: %w", err)
	}

	payload := &cb.Payload{}
	if err := proto.Unmarshal(envelope.Payload, payload); err != nil {
		return "", fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	header := &cb.ChannelHeader{}
	if err := proto.Unmarshal(payload.Header.ChannelHeader, header); err != nil {
		return "", fmt.Errorf("failed to unmarshal channel header: %w", err)
	}

	channelName := header.ChannelId
	if channelName == "" {
		return "", fmt.Errorf("invalid genesis block: missing channel name")
	}

	// Generate a unique network ID
	networkID := uuid.New().String()

	// Create network in database
	_, err := d.db.CreateNetworkFull(ctx, &db.CreateNetworkFullParams{
		Name:        channelName,
		Platform:    "fabric",
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

func CreateConfigUpdateEnvelope(channelID string, configUpdate *cb.ConfigUpdate) ([]byte, error) {
	configUpdate.ChannelId = channelID
	configUpdateData, err := proto.Marshal(configUpdate)
	if err != nil {
		return nil, err
	}
	configUpdateEnvelope := &cb.ConfigUpdateEnvelope{}
	configUpdateEnvelope.ConfigUpdate = configUpdateData
	envelope, err := protoutil.CreateSignedEnvelope(cb.HeaderType_CONFIG_UPDATE, channelID, nil, configUpdateEnvelope, 0, 0)
	if err != nil {
		return nil, err
	}
	envelopeData, err := proto.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return envelopeData, nil
}

// ExtractConfigFromBlock extracts channel configuration from block
func ExtractConfigFromBlock(block *cb.Block) (*cb.Config, error) {
	if block == nil || block.Data == nil || len(block.Data.Data) == 0 {
		return nil, errors.New("invalid block")
	}
	blockPayload := block.Data.Data[0]

	envelope := &cb.Envelope{}
	if err := proto.Unmarshal(blockPayload, envelope); err != nil {
		return nil, err
	}
	payload := &cb.Payload{}
	if err := proto.Unmarshal(envelope.Payload, payload); err != nil {
		return nil, err
	}

	cfgEnv := &cb.ConfigEnvelope{}
	if err := proto.Unmarshal(payload.Data, cfgEnv); err != nil {
		return nil, err
	}
	return cfgEnv.Config, nil
}

type ChainInfo struct {
	Height            uint64
	CurrentBlockHash  string
	PreviousBlockHash string
}

func (d *FabricDeployer) GetChainInfo(ctx context.Context, networkID int64) (*ChainInfo, error) {
	// Get network details
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	// Get a peer from the network
	networkNodes, err := d.db.GetNetworkNodes(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network nodes: %w", err)
	}

	var peerNode *db.GetNetworkNodesRow
	for _, node := range networkNodes {
		if node.Role == "peer" && node.Status == "joined" {
			peerNode = node
			break
		}
	}

	if peerNode == nil {
		return nil, fmt.Errorf("no active peer found in network")
	}

	// Get peer instance
	peer, err := d.nodes.GetFabricPeer(ctx, peerNode.NodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer: %w", err)
	}

	chainInfo, err := peer.GetChannelInfoOnPeer(ctx, network.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain info: %w", err)
	}

	return &ChainInfo{
		Height:            uint64(chainInfo.Height),
		CurrentBlockHash:  fmt.Sprintf("%x", chainInfo.CurrentBlockHash),
		PreviousBlockHash: fmt.Sprintf("%x", chainInfo.PreviousBlockHash),
	}, nil
}
func (d *FabricDeployer) GetBlocks(ctx context.Context, networkID int64, limit, offset int32, reverse bool) ([]block.Block, int64, error) {
	// Get network details
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get network: %w", err)
	}

	// Get a peer from the network
	networkNodes, err := d.db.GetNetworkNodes(ctx, networkID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get network nodes: %w", err)
	}

	var peerNode *db.GetNetworkNodesRow
	for _, node := range networkNodes {
		if node.Role == "peer" && node.Status == "joined" {
			peerNode = node
			break
		}
	}

	if peerNode == nil {
		return nil, 0, fmt.Errorf("no active peer found in network")
	}

	// Get peer instance
	peer, err := d.nodes.GetFabricPeer(ctx, peerNode.NodeID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get peer: %w", err)
	}

	// Get channel info to get total blocks
	channelInfo, err := peer.GetChannelBlockInfo(ctx, network.Name)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get channel info: %w", err)
	}

	total := int64(channelInfo.Height)
	// Calculate start and end blocks based on reverse flag
	var startBlock, endBlock int64

	// Check if offset is greater than channel height
	if int64(offset) >= int64(channelInfo.Height) {
		return []block.Block{}, total, nil // Return empty slice if offset exceeds height
	}

	if reverse {
		// If reverse is true, we start from the newest blocks
		endBlock = int64(channelInfo.Height) - 1 - int64(offset)
		if endBlock < 0 {
			endBlock = 0
		}
		// Calculate endBlock based on startBlock and limit
		startBlock = endBlock - int64(limit)
		if startBlock > endBlock { // Handle underflow
			startBlock = 0
		}
		if startBlock < 0 {
			startBlock = 0
		}

		// Example: if height is 31, limit is 10, offset is 0
		// endBlock = 30, startBlock = 21
	} else {
		// Normal order (oldest first)
		startBlock = int64(offset)
		endBlock = int64(offset + limit - 1)
		if endBlock >= int64(channelInfo.Height) {
			endBlock = int64(channelInfo.Height) - 1
		}
	}

	// Get blocks in range
	blocks, err := peer.GetBlocksInRange(ctx, network.Name, uint64(startBlock), uint64(endBlock))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get blocks: %w", err)
	}

	// Convert blocks to response type
	result := make([]block.Block, len(blocks))
	for i, block := range blocks {
		blck, err := d.MapBlock(block)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to map block: %w", err)
		}
		result[i] = *blck
	}

	return result, total, nil
}

// GetBlocks retrieves blocks from a specific network
func (d *FabricDeployer) MapBlock(blk *cb.Block) (*block.Block, error) {
	blockResponse, err := block.MapBlock(blk)
	if err != nil {
		return nil, fmt.Errorf("failed to map block: %w", err)
	}
	return blockResponse, nil
}

// GetBlock retrieves all transactions from a specific block
func (d *FabricDeployer) GetBlock(ctx context.Context, networkID int64, blockNum uint64) (*block.Block, error) {
	// Get network details
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	// Get a peer from the network
	networkNodes, err := d.db.GetNetworkNodes(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network nodes: %w", err)
	}

	var peerNode *db.GetNetworkNodesRow
	for _, node := range networkNodes {
		if node.Role == "peer" && node.Status == "joined" {
			peerNode = node
			break
		}
	}

	if peerNode == nil {
		return nil, fmt.Errorf("no active peer found in network")
	}

	// Get peer instance
	peer, err := d.nodes.GetFabricPeer(ctx, peerNode.NodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer: %w", err)
	}

	// Get transactions from block
	blk, err := peer.GetBlock(ctx, network.Name, blockNum)
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %w", err)
	}
	blockReponse, err := block.MapBlock(blk)
	if err != nil {
		return nil, fmt.Errorf("failed to map block: %w", err)
	}
	return blockReponse, nil
}

// GetTransaction retrieves a specific transaction by its ID
func (d *FabricDeployer) GetBlockByTransaction(ctx context.Context, networkID int64, txID string) (*block.Block, error) {
	// Get network details
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	// Get a peer from the network
	networkNodes, err := d.db.GetNetworkNodes(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network nodes: %w", err)
	}

	var peerNode *db.GetNetworkNodesRow
	for _, node := range networkNodes {
		if node.Role == "peer" && node.Status == "joined" {
			peerNode = node
			break
		}
	}

	if peerNode == nil {
		return nil, fmt.Errorf("no active peer found in network")
	}

	// Get peer instance
	peer, err := d.nodes.GetFabricPeer(ctx, peerNode.NodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer: %w", err)
	}

	// Get channel info
	blk, err := peer.GetBlockByTxID(ctx, network.Name, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel info: %w", err)
	}
	blockResponse, err := block.MapBlock(blk)
	if err != nil {
		return nil, fmt.Errorf("failed to map block: %w", err)
	}

	return blockResponse, nil
}
