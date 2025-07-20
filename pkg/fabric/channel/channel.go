package channel

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"

	"crypto/x509"
	"crypto/x509/pkix"
	"time"

	"github.com/hyperledger/fabric-config/configtx"
	"github.com/hyperledger/fabric-config/configtx/membership"
	"github.com/hyperledger/fabric-config/configtx/orderer"
	"github.com/hyperledger/fabric-config/protolator"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	sb "github.com/hyperledger/fabric-protos-go-apiv2/orderer/smartbft"

	"github.com/chainlaunch/chainlaunch/internal/protoutil"
	"google.golang.org/protobuf/proto"
)

// ConsensusType represents the type of consensus algorithm
type ConsensusType string

const (
	ConsensusTypeEtcdRaft ConsensusType = "etcdraft"
	ConsensusTypeSmartBFT ConsensusType = "smartbft"
)

// SmartBFTConsenter represents a SmartBFT consenter with additional fields
type SmartBFTConsenter struct {
	Address       HostPort `json:"address"`
	ClientTLSCert string   `json:"clientTLSCert"`
	ServerTLSCert string   `json:"serverTLSCert"`
	Identity      string   `json:"identity"`
	ID            uint64   `json:"id"`
	MSPID         string   `json:"mspId"`
}

// SmartBFTOptions represents SmartBFT configuration options
type SmartBFTOptions struct {
	RequestBatchMaxCount      uint64 `json:"requestBatchMaxCount"`
	RequestBatchMaxBytes      uint64 `json:"requestBatchMaxBytes"`
	RequestBatchMaxInterval   string `json:"requestBatchMaxInterval"`
	IncomingMessageBufferSize uint64 `json:"incomingMessageBufferSize"`
	RequestPoolSize           uint64 `json:"requestPoolSize"`
	RequestForwardTimeout     string `json:"requestForwardTimeout"`
	RequestComplainTimeout    string `json:"requestComplainTimeout"`
	RequestAutoRemoveTimeout  string `json:"requestAutoRemoveTimeout"`
	RequestMaxBytes           uint64 `json:"requestMaxBytes"`
	ViewChangeResendInterval  string `json:"viewChangeResendInterval"`
	ViewChangeTimeout         string `json:"viewChangeTimeout"`
	LeaderHeartbeatTimeout    string `json:"leaderHeartbeatTimeout"`
	LeaderHeartbeatCount      uint64 `json:"leaderHeartbeatCount"`
	CollectTimeout            string `json:"collectTimeout"`
	SyncOnStart               bool   `json:"syncOnStart"`
	SpeedUpViewChange         bool   `json:"speedUpViewChange"`
	LeaderRotation            string `json:"leaderRotation"`
	DecisionsPerLeader        uint64 `json:"decisionsPerLeader"`
}

// EtcdRaftOptions represents etcdraft configuration options
type EtcdRaftOptions struct {
	TickInterval         string `json:"tickInterval"`
	ElectionTick         uint32 `json:"electionTick"`
	HeartbeatTick        uint32 `json:"heartbeatTick"`
	MaxInflightBlocks    uint32 `json:"maxInflightBlocks"`
	SnapshotIntervalSize uint32 `json:"snapshotIntervalSize"`
}

// BatchSize represents batch size configuration
type BatchSize struct {
	MaxMessageCount   uint32 `json:"maxMessageCount"`
	AbsoluteMaxBytes  uint32 `json:"absoluteMaxBytes"`
	PreferredMaxBytes uint32 `json:"preferredMaxBytes"`
}

// ChannelService handles channel operations
type ChannelService struct {
	// Add any dependencies here
}

// NewChannelService creates a new channel service
func NewChannelService() *ChannelService {
	return &ChannelService{}
}

// HostPort represents a network host and port
type HostPort struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// Organization represents a blockchain organization
type Organization struct {
	Name             string     `json:"name"`
	AnchorPeers      []HostPort `json:"anchorPeers"`
	OrdererEndpoints []string   `json:"ordererEndpoints"`
	SignCACert       string     `json:"signCACert"`
	TLSCACert        string     `json:"tlsCACert"`
}

// AddressWithCerts represents a network address with TLS certificates
type AddressWithCerts struct {
	Address       HostPort `json:"address"`
	ClientTLSCert string   `json:"clientTLSCert"`
	ServerTLSCert string   `json:"serverTLSCert"`
}

// CreateChannelInput represents the input for creating a new channel
type CreateChannelInput struct {
	Name        string             `json:"name"`
	PeerOrgs    []Organization     `json:"peerOrgs"`
	OrdererOrgs []Organization     `json:"ordererOrgs"`
	Consenters  []AddressWithCerts `json:"consenters"`
	// SmartBFT specific fields
	SmartBFTConsenters []SmartBFTConsenter `json:"smartBFTConsenters,omitempty"`
	SmartBFTOptions    *SmartBFTOptions    `json:"smartBFTOptions,omitempty"`
	// EtcdRaft specific fields
	EtcdRaftOptions *EtcdRaftOptions `json:"etcdRaftOptions,omitempty"`
	// Consensus type - defaults to etcdraft if not specified
	ConsensusType ConsensusType `json:"consensusType,omitempty"`
	// Batch configuration
	BatchSize    *BatchSize `json:"batchSize,omitempty"`
	BatchTimeout string     `json:"batchTimeout,omitempty"` // e.g., "2s"
	// Optional policies
	ChannelPolicies     map[string]configtx.Policy `json:"channelPolicies,omitempty"`
	ApplicationPolicies map[string]configtx.Policy `json:"applicationPolicies,omitempty"`
	OrdererPolicies     map[string]configtx.Policy `json:"ordererPolicies,omitempty"`
}

// SetAnchorPeersInput represents the input for setting anchor peers
type SetAnchorPeersInput struct {
	CurrentConfig *cb.Config
	AnchorPeers   []HostPort
	MSPID         string
	ChannelName   string
}

// CreateChannelResponse represents the response from creating a channel
type CreateChannelResponse struct {
	ChannelID  string `json:"channelId"`
	ConfigData string `json:"configData"`
}

// CreateChannel creates a new channel with the given configuration
func (s *ChannelService) CreateChannel(input CreateChannelInput) (*CreateChannelResponse, error) {
	channelConfig, err := s.parseAndCreateChannel(input)
	if err != nil {
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	return &CreateChannelResponse{
		ChannelID:  input.Name,
		ConfigData: base64.StdEncoding.EncodeToString(channelConfig),
	}, nil
}

// SetCRLInput represents the input for setting CRL
type SetCRLInput struct {
	CurrentConfig *cb.Config
	CRL           []byte
	MSPID         string
	ChannelName   string
}

// SetCRL updates the CRL for an organization in a channel
func (s *ChannelService) SetCRL(input *SetCRLInput) (*cb.Envelope, error) {
	// Create config manager and update CRL
	cftxGen := configtx.New(input.CurrentConfig)
	org, err := cftxGen.Application().Organization(input.MSPID).Configuration()
	if err != nil {
		return nil, fmt.Errorf("failed to get organization configuration: %w", err)
	}

	crl, err := ParseCRL(input.CRL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CRL: %w", err)
	}
	org.MSP.RevocationList = []*pkix.CertificateList{crl}
	err = cftxGen.Application().SetOrganization(org)
	if err != nil {
		return nil, fmt.Errorf("failed to set organization configuration: %w", err)
	}

	// Compute update
	configUpdateBytes, err := cftxGen.ComputeMarshaledUpdate(input.ChannelName)
	if err != nil {
		return nil, fmt.Errorf("failed to compute update: %w", err)
	}

	configUpdate := &cb.ConfigUpdate{}
	if err := proto.Unmarshal(configUpdateBytes, configUpdate); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config update: %w", err)
	}

	// Create envelope
	configEnvelope, err := s.createConfigUpdateEnvelope(input.ChannelName, configUpdate)
	if err != nil {
		return nil, fmt.Errorf("failed to create config update envelope: %w", err)
	}

	return configEnvelope, nil
}

// SetAnchorPeers updates the anchor peers for an organization in a channel
func (s *ChannelService) SetAnchorPeers(input *SetAnchorPeersInput) (*cb.Envelope, error) {
	// Create config manager and update anchor peers
	cftxGen := configtx.New(input.CurrentConfig)
	app := cftxGen.Application().Organization(input.MSPID)

	// Remove existing anchor peers
	currentAnchorPeers, err := app.AnchorPeers()
	if err != nil {
		return nil, fmt.Errorf("failed to get current anchor peers: %w", err)
	}

	for _, ap := range currentAnchorPeers {
		if err := app.RemoveAnchorPeer(configtx.Address{
			Host: ap.Host,
			Port: ap.Port,
		}); err != nil {
			continue
		}
	}

	// Add new anchor peers
	for _, ap := range input.AnchorPeers {
		if err := app.AddAnchorPeer(configtx.Address{
			Host: ap.Host,
			Port: ap.Port,
		}); err != nil {
			return nil, fmt.Errorf("failed to add anchor peer: %w", err)
		}
	}

	// Compute update
	configUpdateBytes, err := cftxGen.ComputeMarshaledUpdate(input.ChannelName)
	if err != nil {
		return nil, fmt.Errorf("failed to compute update: %w", err)
	}

	configUpdate := &cb.ConfigUpdate{}
	if err := proto.Unmarshal(configUpdateBytes, configUpdate); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config update: %w", err)
	}

	// Create envelope
	configEnvelope, err := s.createConfigUpdateEnvelope(input.ChannelName, configUpdate)
	if err != nil {
		return nil, fmt.Errorf("failed to create config update envelope: %w", err)
	}

	return configEnvelope, nil
}
func (s *ChannelService) createConfigUpdateEnvelope(channelID string, configUpdate *cb.ConfigUpdate) (*cb.Envelope, error) {
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

	return envelope, nil
}

// DecodeBlock decodes a base64 encoded block into JSON
func (s *ChannelService) DecodeBlock(blockB64 string) (map[string]interface{}, error) {
	blockBytes, err := base64.StdEncoding.DecodeString(blockB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode block: %w", err)
	}

	block := &cb.Block{}
	if err := proto.Unmarshal(blockBytes, block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	var buf bytes.Buffer
	if err := protolator.DeepMarshalJSON(&buf, block); err != nil {
		return nil, fmt.Errorf("failed to marshal block to JSON: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return data, nil
}

// Helper functions below...

func (s *ChannelService) parseAndCreateChannel(input CreateChannelInput) ([]byte, error) {
	// Parse organizations
	peerOrgs := []configtx.Organization{}
	for _, org := range input.PeerOrgs {
		// Parse certificates
		signCACert, err := parseCertificate(org.SignCACert)
		if err != nil {
			return nil, fmt.Errorf("failed to parse signing CA cert for org %s: %w", org.Name, err)
		}

		tlsCACert, err := parseCertificate(org.TLSCACert)
		if err != nil {
			return nil, fmt.Errorf("failed to parse TLS CA cert for org %s: %w", org.Name, err)
		}

		// Convert anchor peers
		anchorPeers := make([]configtx.Address, len(org.AnchorPeers))
		for i, ap := range org.AnchorPeers {
			anchorPeers[i] = configtx.Address{
				Host: ap.Host,
				Port: ap.Port,
			}
		}

		// Create organization config
		peerOrg := configtx.Organization{
			Name: org.Name,
			MSP: configtx.MSP{
				Name:         org.Name,
				RootCerts:    []*x509.Certificate{signCACert},
				TLSRootCerts: []*x509.Certificate{tlsCACert},
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
				Admins:                        []*x509.Certificate{},
				IntermediateCerts:             []*x509.Certificate{},
				RevocationList:                []*pkix.CertificateList{},
				OrganizationalUnitIdentifiers: []membership.OUIdentifier{},
				CryptoConfig:                  membership.CryptoConfig{},
				TLSIntermediateCerts:          []*x509.Certificate{},
			},
			Policies: map[string]configtx.Policy{
				"Admins": {
					Type: "Signature",
					Rule: fmt.Sprintf("OR('%s.admin')", org.Name),
				},
				"Readers": {
					Type: "Signature",
					Rule: fmt.Sprintf("OR('%s.member')", org.Name),
				},
				"Writers": {
					Type: "Signature",
					Rule: fmt.Sprintf("OR('%s.member')", org.Name),
				},
				"Endorsement": {
					Type: "Signature",
					Rule: fmt.Sprintf("OR('%s.member')", org.Name),
				},
			},
			AnchorPeers:      anchorPeers,
			OrdererEndpoints: org.OrdererEndpoints,
			ModPolicy:        "",
		}

		peerOrgs = append(peerOrgs, peerOrg)
	}

	// Parse orderer organizations
	ordererOrgs := []configtx.Organization{}
	for _, org := range input.OrdererOrgs {
		signCACert, err := parseCertificate(org.SignCACert)
		if err != nil {
			return nil, fmt.Errorf("failed to parse signing CA cert for orderer org %s: %w", org.Name, err)
		}

		tlsCACert, err := parseCertificate(org.TLSCACert)
		if err != nil {
			return nil, fmt.Errorf("failed to parse TLS CA cert for orderer org %s: %w", org.Name, err)
		}

		ordererOrg := configtx.Organization{
			Name: org.Name,
			MSP: configtx.MSP{
				Name:         org.Name,
				RootCerts:    []*x509.Certificate{signCACert},
				TLSRootCerts: []*x509.Certificate{tlsCACert},
				NodeOUs: membership.NodeOUs{
					Enable: true,
					ClientOUIdentifier: membership.OUIdentifier{
						Certificate:                  signCACert,
						OrganizationalUnitIdentifier: "client",
					},
					OrdererOUIdentifier: membership.OUIdentifier{
						Certificate:                  signCACert,
						OrganizationalUnitIdentifier: "orderer",
					},
					AdminOUIdentifier: membership.OUIdentifier{
						Certificate:                  signCACert,
						OrganizationalUnitIdentifier: "admin",
					},
					PeerOUIdentifier: membership.OUIdentifier{
						Certificate:                  signCACert,
						OrganizationalUnitIdentifier: "peer",
					},
				},
				Admins:                        []*x509.Certificate{},
				IntermediateCerts:             []*x509.Certificate{},
				RevocationList:                []*pkix.CertificateList{},
				OrganizationalUnitIdentifiers: []membership.OUIdentifier{},
				CryptoConfig:                  membership.CryptoConfig{},
				TLSIntermediateCerts:          []*x509.Certificate{},
			},
			Policies: map[string]configtx.Policy{
				"Admins": {
					Type: "Signature",
					Rule: fmt.Sprintf("OR('%s.admin')", org.Name),
				},
				"Readers": {
					Type: "Signature",
					Rule: fmt.Sprintf("OR('%s.member')", org.Name),
				},
				"Writers": {
					Type: "Signature",
					Rule: fmt.Sprintf("OR('%s.member')", org.Name),
				},
				"Endorsement": {
					Type: "Signature",
					Rule: fmt.Sprintf("OR('%s.member')", org.Name),
				},
			},
			OrdererEndpoints: org.OrdererEndpoints,
			ModPolicy:        "",
		}

		ordererOrgs = append(ordererOrgs, ordererOrg)
	}

	// Parse consenters
	consenters := []orderer.Consenter{}
	for _, cons := range input.Consenters {
		clientTLSCert, err := parseCertificate(cons.ClientTLSCert)
		if err != nil {
			return nil, fmt.Errorf("failed to parse client TLS cert for consenter %s: %w", cons.Address.Host, err)
		}

		serverTLSCert, err := parseCertificate(cons.ServerTLSCert)
		if err != nil {
			return nil, fmt.Errorf("failed to parse server TLS cert for consenter %s: %w", cons.Address.Host, err)
		}

		consenters = append(consenters, orderer.Consenter{
			Address: orderer.EtcdAddress{
				Host: cons.Address.Host,
				Port: cons.Address.Port,
			},
			ClientTLSCert: clientTLSCert,
			ServerTLSCert: serverTLSCert,
		})
	}

	// Create channel configuration
	appPolicies := input.ApplicationPolicies
	if appPolicies == nil {
		appPolicies = map[string]configtx.Policy{
			"Readers": {
				Type: "ImplicitMeta",
				Rule: "ANY Readers",
			},
			"Writers": {
				Type: "ImplicitMeta",
				Rule: "ANY Writers",
			},
			"Admins": {
				Type: "ImplicitMeta",
				Rule: "MAJORITY Admins",
			},
			"LifecycleEndorsement": {
				Type: "ImplicitMeta",
				Rule: "MAJORITY Endorsement",
			},
			"Endorsement": {
				Type: "ImplicitMeta",
				Rule: "MAJORITY Endorsement",
			},
		}
	}
	ordererPolicies := input.OrdererPolicies
	if ordererPolicies == nil {
		ordererPolicies = map[string]configtx.Policy{
			"Readers": {
				Type: "ImplicitMeta",
				Rule: "ANY Readers",
			},
			"Writers": {
				Type: "ImplicitMeta",
				Rule: "ANY Writers",
			},
			"Admins": {
				Type: "ImplicitMeta",
				Rule: "MAJORITY Admins",
			},
			"BlockValidation": {
				Type: "ImplicitMeta",
				Rule: "ANY Writers",
			},
		}
	}
	channelPolicies := input.ChannelPolicies
	if channelPolicies == nil {
		channelPolicies = map[string]configtx.Policy{
			"Readers": {
				Type: "ImplicitMeta",
				Rule: "ANY Readers",
			},
			"Writers": {
				Type: "ImplicitMeta",
				Rule: "ANY Writers",
			},
			"Admins": {
				Type: "ImplicitMeta",
				Rule: "MAJORITY Admins",
			},
		}
	}

	// Determine consensus type - default to etcdraft if not specified
	consensusType := input.ConsensusType
	if consensusType == "" {
		consensusType = ConsensusTypeEtcdRaft
	}

	// Build orderer configuration based on consensus type
	var ordererConfig configtx.Orderer

	if consensusType == ConsensusTypeSmartBFT {
		// Validate SmartBFT requirements
		if len(input.SmartBFTConsenters) < 4 {
			return nil, fmt.Errorf("SmartBFT requires at least 4 consenters, got %d", len(input.SmartBFTConsenters))
		}

		// Parse SmartBFT consenters
		consenterMapping := []cb.Consenter{}
		for _, cons := range input.SmartBFTConsenters {
			identityCert, err := parseCertificate(cons.Identity)
			if err != nil {
				return nil, fmt.Errorf("failed to parse identity cert for SmartBFT consenter %s: %w", cons.Address.Host, err)
			}
			clientTLSCert, err := parseCertificate(cons.ClientTLSCert)
			if err != nil {
				return nil, fmt.Errorf("failed to parse client TLS cert for SmartBFT consenter %s: %w", cons.Address.Host, err)
			}
			serverTLSCert, err := parseCertificate(cons.ServerTLSCert)
			if err != nil {
				return nil, fmt.Errorf("failed to parse server TLS cert for SmartBFT consenter %s: %w", cons.Address.Host, err)
			}

			consenterMapping = append(consenterMapping, cb.Consenter{
				Id:            uint32(cons.ID),
				Host:          cons.Address.Host,
				Port:          uint32(cons.Address.Port),
				MspId:         cons.MSPID,
				Identity:      encodeX509Certificate(identityCert),
				ClientTlsCert: encodeX509Certificate(clientTLSCert),
				ServerTlsCert: encodeX509Certificate(serverTLSCert),
			})
		}

		// Build SmartBFT options
		var smartBFTOptions *sb.Options
		if input.SmartBFTOptions != nil {
			leaderRotation := sb.Options_ROTATION_ON
			switch input.SmartBFTOptions.LeaderRotation {
			case "ROTATION_ON":
				leaderRotation = sb.Options_ROTATION_ON
			case "ROTATION_OFF":
				leaderRotation = sb.Options_ROTATION_OFF
			default:
				leaderRotation = sb.Options_ROTATION_UNSPECIFIED
			}

			smartBFTOptions = &sb.Options{
				RequestBatchMaxCount:      input.SmartBFTOptions.RequestBatchMaxCount,
				RequestBatchMaxBytes:      input.SmartBFTOptions.RequestBatchMaxBytes,
				RequestBatchMaxInterval:   input.SmartBFTOptions.RequestBatchMaxInterval,
				IncomingMessageBufferSize: input.SmartBFTOptions.IncomingMessageBufferSize,
				RequestPoolSize:           input.SmartBFTOptions.RequestPoolSize,
				RequestForwardTimeout:     input.SmartBFTOptions.RequestForwardTimeout,
				RequestComplainTimeout:    input.SmartBFTOptions.RequestComplainTimeout,
				RequestAutoRemoveTimeout:  input.SmartBFTOptions.RequestAutoRemoveTimeout,
				RequestMaxBytes:           input.SmartBFTOptions.RequestMaxBytes,
				ViewChangeResendInterval:  input.SmartBFTOptions.ViewChangeResendInterval,
				ViewChangeTimeout:         input.SmartBFTOptions.ViewChangeTimeout,
				LeaderHeartbeatTimeout:    input.SmartBFTOptions.LeaderHeartbeatTimeout,
				LeaderHeartbeatCount:      input.SmartBFTOptions.LeaderHeartbeatCount,
				CollectTimeout:            input.SmartBFTOptions.CollectTimeout,
				SyncOnStart:               input.SmartBFTOptions.SyncOnStart,
				SpeedUpViewChange:         input.SmartBFTOptions.SpeedUpViewChange,
				LeaderRotation:            leaderRotation,
				DecisionsPerLeader:        input.SmartBFTOptions.DecisionsPerLeader,
			}
		}

		// Parse batch timeout for SmartBFT
		batchTimeout := 2 * time.Second
		if input.BatchTimeout != "" {
			if parsedTimeout, err := time.ParseDuration(input.BatchTimeout); err == nil {
				batchTimeout = parsedTimeout
			}
		}

		// Set batch size for SmartBFT
		batchSize := orderer.BatchSize{
			MaxMessageCount:   500,
			AbsoluteMaxBytes:  10 * 1024 * 1024,
			PreferredMaxBytes: 2 * 1024 * 1024,
		}
		if input.BatchSize != nil {
			batchSize.MaxMessageCount = input.BatchSize.MaxMessageCount
			batchSize.AbsoluteMaxBytes = input.BatchSize.AbsoluteMaxBytes
			batchSize.PreferredMaxBytes = input.BatchSize.PreferredMaxBytes
		}

		ordererConfig = configtx.Orderer{
			OrdererType:      string(orderer.ConsensusTypeBFT),
			BatchTimeout:     batchTimeout,
			State:            orderer.ConsensusStateNormal,
			ConsenterMapping: consenterMapping,
			SmartBFT:         smartBFTOptions,
			Organizations:    ordererOrgs,
			Capabilities:     []string{"V2_0"},
			Policies:         ordererPolicies,
			BatchSize:        batchSize,
		}
	} else {
		// Default to etcdraft
		// Parse batch timeout
		batchTimeout := 2 * time.Second
		if input.BatchTimeout != "" {
			if parsedTimeout, err := time.ParseDuration(input.BatchTimeout); err == nil {
				batchTimeout = parsedTimeout
			}
		}

		// Set batch size
		batchSize := orderer.BatchSize{
			MaxMessageCount:   500,
			AbsoluteMaxBytes:  10 * 1024 * 1024,
			PreferredMaxBytes: 2 * 1024 * 1024,
		}
		if input.BatchSize != nil {
			batchSize.MaxMessageCount = input.BatchSize.MaxMessageCount
			batchSize.AbsoluteMaxBytes = input.BatchSize.AbsoluteMaxBytes
			batchSize.PreferredMaxBytes = input.BatchSize.PreferredMaxBytes
		}

		// Set etcdraft options
		etcdRaftOptions := orderer.EtcdRaftOptions{
			TickInterval:         "500ms",
			ElectionTick:         10,
			HeartbeatTick:        1,
			MaxInflightBlocks:    5,
			SnapshotIntervalSize: 16 * 1024 * 1024, // 16 MB
		}
		if input.EtcdRaftOptions != nil {
			if input.EtcdRaftOptions.TickInterval != "" {
				etcdRaftOptions.TickInterval = input.EtcdRaftOptions.TickInterval
			}
			if input.EtcdRaftOptions.ElectionTick > 0 {
				etcdRaftOptions.ElectionTick = input.EtcdRaftOptions.ElectionTick
			}
			if input.EtcdRaftOptions.HeartbeatTick > 0 {
				etcdRaftOptions.HeartbeatTick = input.EtcdRaftOptions.HeartbeatTick
			}
			if input.EtcdRaftOptions.MaxInflightBlocks > 0 {
				etcdRaftOptions.MaxInflightBlocks = input.EtcdRaftOptions.MaxInflightBlocks
			}
			if input.EtcdRaftOptions.SnapshotIntervalSize > 0 {
				etcdRaftOptions.SnapshotIntervalSize = input.EtcdRaftOptions.SnapshotIntervalSize
			}
		}

		ordererConfig = configtx.Orderer{
			OrdererType:  orderer.ConsensusTypeEtcdRaft,
			BatchTimeout: batchTimeout,
			State:        orderer.ConsensusStateNormal,
			BatchSize:    batchSize,
			EtcdRaft: orderer.EtcdRaft{
				Consenters: consenters,
				Options:    etcdRaftOptions,
			},
			Organizations: ordererOrgs,
			Capabilities:  []string{"V2_0"},
			Policies:      ordererPolicies,
		}
	}

	channelConfig := configtx.Channel{
		Consortiums: nil, // Not needed for application channels
		Application: configtx.Application{
			Organizations: peerOrgs,
			Capabilities:  []string{"V2_0"},
			ACLs:          defaultACLs(),
			Policies:      appPolicies,
		},
		Orderer:      ordererConfig,
		Capabilities: []string{"V2_0"},
		Policies:     channelPolicies,
	}

	// Create genesis block
	block, err := configtx.NewApplicationChannelGenesisBlock(channelConfig, input.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create genesis block: %w", err)
	}

	// Marshal the block
	blockBytes, err := proto.Marshal(block)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal genesis block: %w", err)
	}

	return blockBytes, nil
}

// Helper function to parse PEM certificates
func parseCertificate(certPEM string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}

// Helper function to encode X509 certificate to PEM format
func encodeX509Certificate(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

func defaultACLs() map[string]string {
	return map[string]string{
		"_lifecycle/CheckCommitReadiness": "/Channel/Application/Writers",

		//  ACL policy for _lifecycle's "CommitChaincodeDefinition" function
		"_lifecycle/CommitChaincodeDefinition": "/Channel/Application/Writers",

		//  ACL policy for _lifecycle's "QueryChaincodeDefinition" function
		"_lifecycle/QueryChaincodeDefinition": "/Channel/Application/Writers",

		//  ACL policy for _lifecycle's "QueryChaincodeDefinitions" function
		"_lifecycle/QueryChaincodeDefinitions": "/Channel/Application/Writers",

		// ---Lifecycle System Chaincode (lscc) function to policy mapping for access control---//

		//  ACL policy for lscc's "getid" function
		"lscc/ChaincodeExists": "/Channel/Application/Readers",

		//  ACL policy for lscc's "getdepspec" function
		"lscc/GetDeploymentSpec": "/Channel/Application/Readers",

		//  ACL policy for lscc's "getccdata" function
		"lscc/GetChaincodeData": "/Channel/Application/Readers",

		//  ACL Policy for lscc's "getchaincodes" function
		"lscc/GetInstantiatedChaincodes": "/Channel/Application/Readers",

		// ---Query System Chaincode (qscc) function to policy mapping for access control---//

		//  ACL policy for qscc's "GetChainInfo" function
		"qscc/GetChainInfo": "/Channel/Application/Readers",

		//  ACL policy for qscc's "GetBlockByNumber" function
		"qscc/GetBlockByNumber": "/Channel/Application/Readers",

		//  ACL policy for qscc's  "GetBlockByHash" function
		"qscc/GetBlockByHash": "/Channel/Application/Readers",

		//  ACL policy for qscc's "GetTransactionByID" function
		"qscc/GetTransactionByID": "/Channel/Application/Readers",

		//  ACL policy for qscc's "GetBlockByTxID" function
		"qscc/GetBlockByTxID": "/Channel/Application/Readers",

		// ---Configuration System Chaincode (cscc) function to policy mapping for access control---//

		//  ACL policy for cscc's "GetConfigBlock" function
		"cscc/GetConfigBlock": "/Channel/Application/Readers",

		//  ACL policy for cscc's "GetChannelConfig" function
		"cscc/GetChannelConfig": "/Channel/Application/Readers",

		// ---Miscellaneous peer function to policy mapping for access control---//

		//  ACL policy for invoking chaincodes on peer
		"peer/Propose": "/Channel/Application/Writers",

		//  ACL policy for chaincode to chaincode invocation
		"peer/ChaincodeToChaincode": "/Channel/Application/Writers",

		// ---Events resource to policy mapping for access control// // // ---//

		//  ACL policy for sending block events
		"event/Block": "/Channel/Application/Readers",

		//  ACL policy for sending filtered block events
		"event/FilteredBlock": "/Channel/Application/Readers",
	}
}

func ParseCRL(crlBytes []byte) (*pkix.CertificateList, error) {
	block, _ := pem.Decode(crlBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing CRL")
	}

	crl, err := x509.ParseCRL(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CRL: %v", err)
	}

	return crl, nil
}
