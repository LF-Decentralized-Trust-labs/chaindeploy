package fabricx

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hyperledger/fabric-x-common/api/types"
	"github.com/hyperledger/fabric-x-common/tools/configtxgen"
	"github.com/hyperledger/fabric-x-orderer/config/protos"
	"google.golang.org/protobuf/proto"
)

// CertFingerprint returns a short summary of a PEM cert for diagnostic logging:
// SHA-256 prefix, NotBefore, and the first SAN entries. Empty string if parse fails.
func CertFingerprint(pemBytes []byte) string {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return fmt.Sprintf("sha256=%x[no-pem len=%d]", sha256.Sum256(pemBytes), len(pemBytes))
	}
	sum := sha256.Sum256(block.Bytes)
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Sprintf("sha256=%x[parse-err: %v]", sum[:4], err)
	}
	return fmt.Sprintf("sha256=%x nb=%s dns=%v ips=%v", sum[:4], c.NotBefore.Format(time.RFC3339), c.DNSNames, c.IPAddresses)
}

// GenesisConfig holds the parameters needed to generate a Fabric X genesis block
type GenesisConfig struct {
	ChannelID string
	Parties   []GenesisParty
}

// GenesisParty represents one orderer party for genesis generation
type GenesisParty struct {
	PartyID       int
	MSPID         string
	SignCACert    string // PEM
	TLSCACert     string // PEM
	AdminCert     string // PEM (not used directly in genesis, kept for compatibility)

	// Component certs for SharedConfig
	RouterHost    string
	RouterPort    int
	RouterTLSCert string // PEM

	BatcherHost     string
	BatcherPort     int
	BatcherSignCert string // PEM
	BatcherTLSCert  string // PEM

	ConsenterHost     string
	ConsenterPort     int
	ConsenterSignCert string // PEM
	ConsenterTLSCert  string // PEM

	AssemblerHost    string
	AssemblerPort    int
	AssemblerTLSCert string // PEM

	// Identity cert for consenter mapping (the orderer sign cert)
	IdentityCert string // PEM
}

// GenerateGenesisBlock creates a protobuf-encoded genesis block for a Fabric X network
// using the fabric-x-common configtxgen with "arma" consensus type
func GenerateGenesisBlock(cfg GenesisConfig) ([]byte, error) {
	if len(cfg.Parties) == 0 {
		return nil, fmt.Errorf("at least one party is required")
	}
	if cfg.ChannelID == "" {
		cfg.ChannelID = "testchannel"
	}

	// Create temporary directory for all cert files
	tempDir, err := os.MkdirTemp("", "fabricx-genesis-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Build organizations and SharedConfig
	var ordererOrgs []*configtxgen.Organization
	var appOrgs []*configtxgen.Organization
	var partiesConfig []*protos.PartyConfig
	var consenterMapping []*configtxgen.Consenter

	for i, party := range cfg.Parties {
		partyID := party.PartyID
		if partyID == 0 {
			partyID = i + 1
		}

		// Provision MSP directory for this org
		mspDir, err := provisionMSPDirectory(tempDir, party.MSPID, party.SignCACert, party.TLSCACert)
		if err != nil {
			return nil, fmt.Errorf("failed to provision MSP for party %d: %w", i, err)
		}

		// Build orderer endpoints
		var ordererEndpoints []*types.OrdererEndpoint
		ordererEndpoints = append(ordererEndpoints, &types.OrdererEndpoint{
			ID:   uint32(partyID),
			Host: party.RouterHost,
			Port: party.RouterPort,
			API:  []string{types.Broadcast},
		})
		ordererEndpoints = append(ordererEndpoints, &types.OrdererEndpoint{
			ID:   uint32(partyID),
			Host: party.AssemblerHost,
			Port: party.AssemblerPort,
			API:  []string{types.Deliver},
		})

		// Create orderer organization
		ordererOrg := &configtxgen.Organization{
			Name:             party.MSPID,
			ID:               party.MSPID,
			MSPDir:           mspDir,
			MSPType:          "bccsp",
			OrdererEndpoints: ordererEndpoints,
			Policies: map[string]*configtxgen.Policy{
				"Readers":         {Type: "ImplicitMeta", Rule: "ANY Readers"},
				"Writers":         {Type: "ImplicitMeta", Rule: "ANY Writers"},
				"Admins":          {Type: "ImplicitMeta", Rule: "MAJORITY Admins"},
				"BlockValidation": {Type: "ImplicitMeta", Rule: "ANY Writers"},
				"Endorsement":     {Type: "Signature", Rule: fmt.Sprintf("OR('%s.member')", party.MSPID)},
			},
		}
		ordererOrgs = append(ordererOrgs, ordererOrg)

		// Create application organization (same MSP, different policies)
		appOrg := &configtxgen.Organization{
			Name:    party.MSPID,
			ID:      party.MSPID,
			MSPDir:  mspDir,
			MSPType: "bccsp",
			Policies: map[string]*configtxgen.Policy{
				"Readers":     {Type: "ImplicitMeta", Rule: "ANY Readers"},
				"Writers":     {Type: "ImplicitMeta", Rule: "ANY Writers"},
				"Admins":      {Type: "ImplicitMeta", Rule: "MAJORITY Admins"},
				"Endorsement": {Type: "Signature", Rule: fmt.Sprintf("OR('%s.member')", party.MSPID)},
			},
		}
		appOrgs = append(appOrgs, appOrg)

		// Build PartyConfig for SharedConfig
		partyConfig, err := buildPartyConfig(party, partyID)
		if err != nil {
			return nil, fmt.Errorf("failed to build party config for party %d: %w", i, err)
		}
		partiesConfig = append(partiesConfig, partyConfig)

		// Write consenter certs to disk for consenter mapping
		consenterDir := filepath.Join(tempDir, fmt.Sprintf("consenter-%d", partyID))
		if err := os.MkdirAll(consenterDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create consenter dir: %w", err)
		}

		identityPath := filepath.Join(consenterDir, "identity.pem")
		clientTLSPath := filepath.Join(consenterDir, "client-tls.pem")
		serverTLSPath := filepath.Join(consenterDir, "server-tls.pem")

		if err := os.WriteFile(identityPath, []byte(party.IdentityCert), 0644); err != nil {
			return nil, fmt.Errorf("failed to write identity cert: %w", err)
		}
		if err := os.WriteFile(clientTLSPath, []byte(party.ConsenterTLSCert), 0644); err != nil {
			return nil, fmt.Errorf("failed to write client TLS cert: %w", err)
		}
		if err := os.WriteFile(serverTLSPath, []byte(party.ConsenterTLSCert), 0644); err != nil {
			return nil, fmt.Errorf("failed to write server TLS cert: %w", err)
		}

		consenterMapping = append(consenterMapping, &configtxgen.Consenter{
			ID:            uint32(partyID),
			Host:          party.ConsenterHost,
			Port:          uint32(party.ConsenterPort),
			MSPID:         party.MSPID,
			Identity:      identityPath,
			ClientTLSCert: clientTLSPath,
			ServerTLSCert: serverTLSPath,
		})
	}

	// Build SharedConfig and write to disk
	sharedConfig := buildSharedConfig(partiesConfig)
	sharedConfigBytes, err := proto.Marshal(sharedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal shared config: %w", err)
	}

	sharedConfigPath := filepath.Join(tempDir, "shared-config.pb")
	if err := os.WriteFile(sharedConfigPath, sharedConfigBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to write shared config: %w", err)
	}

	// Merge all orgs for application section
	allOrgs := append(ordererOrgs, appOrgs...)
	// Deduplicate by ID (orderer and app orgs share the same MSP)
	seen := make(map[string]bool)
	var dedupOrgs []*configtxgen.Organization
	for _, org := range allOrgs {
		if !seen[org.ID] {
			seen[org.ID] = true
			dedupOrgs = append(dedupOrgs, org)
		}
	}

	// Build the profile
	profile := &configtxgen.Profile{
		Orderer: &configtxgen.Orderer{
			OrdererType: configtxgen.Arma,
			Arma: &configtxgen.ConsensusMetadata{
				Path: sharedConfigPath,
			},
			Addresses:        []string{},
			BatchTimeout:     2 * time.Second,
			BatchSize: configtxgen.BatchSize{
				MaxMessageCount:   500,
				AbsoluteMaxBytes:  10 * 1024 * 1024,
				PreferredMaxBytes: 2 * 1024 * 1024,
			},
			Organizations:    ordererOrgs,
			ConsenterMapping: consenterMapping,
			Policies: map[string]*configtxgen.Policy{
				"Readers":         {Type: "ImplicitMeta", Rule: "ANY Readers"},
				"Writers":         {Type: "ImplicitMeta", Rule: "ANY Writers"},
				"Admins":          {Type: "ImplicitMeta", Rule: "ANY Admins"},
				"BlockValidation": {Type: "ImplicitMeta", Rule: "ANY Writers"},
			},
			Capabilities: map[string]bool{"V2_0": true},
		},
		Application: &configtxgen.Application{
			Organizations: dedupOrgs,
			Capabilities:  map[string]bool{"V2_5": true},
			Policies: map[string]*configtxgen.Policy{
				"Readers":               {Type: "ImplicitMeta", Rule: "ANY Readers"},
				"Writers":               {Type: "ImplicitMeta", Rule: "ANY Writers"},
				"Admins":                {Type: "ImplicitMeta", Rule: "MAJORITY Admins"},
				"LifecycleEndorsement":  {Type: "ImplicitMeta", Rule: "ANY Endorsement"},
				"Endorsement":           {Type: "ImplicitMeta", Rule: "ANY Endorsement"},
			},
		},
		Capabilities: map[string]bool{"V3_0": true},
		Policies: map[string]*configtxgen.Policy{
			"Readers": {Type: "ImplicitMeta", Rule: "ANY Readers"},
			"Writers": {Type: "ImplicitMeta", Rule: "ANY Writers"},
			"Admins":  {Type: "ImplicitMeta", Rule: "MAJORITY Admins"},
		},
	}

	// Generate the genesis block
	block, err := configtxgen.GetOutputBlock(profile, cfg.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("failed to create genesis block: %w", err)
	}

	blockBytes, err := proto.Marshal(block)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal genesis block: %w", err)
	}

	return blockBytes, nil
}

// provisionMSPDirectory creates a standard MSP directory structure with NodeOU config
func provisionMSPDirectory(baseDir, mspID, signCACert, tlsCACert string) (string, error) {
	mspDir := filepath.Join(baseDir, fmt.Sprintf("msp-%s", mspID))

	dirs := []string{
		filepath.Join(mspDir, "admincerts"),
		filepath.Join(mspDir, "cacerts"),
		filepath.Join(mspDir, "intermediatecerts"),
		filepath.Join(mspDir, "keystore"),
		filepath.Join(mspDir, "signcerts"),
		filepath.Join(mspDir, "tlscacerts"),
		filepath.Join(mspDir, "tlsintermediatecerts"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return "", fmt.Errorf("failed to create dir %s: %w", d, err)
		}
	}

	if err := os.WriteFile(filepath.Join(mspDir, "cacerts", "ca.crt"), []byte(signCACert), 0644); err != nil {
		return "", fmt.Errorf("failed to write sign CA cert: %w", err)
	}
	if err := os.WriteFile(filepath.Join(mspDir, "tlscacerts", "ca.crt"), []byte(tlsCACert), 0644); err != nil {
		return "", fmt.Errorf("failed to write TLS CA cert: %w", err)
	}

	configYAML := `NodeOUs:
  Enable: true
  ClientOUIdentifier:
    Certificate: cacerts/ca.crt
    OrganizationalUnitIdentifier: client
  PeerOUIdentifier:
    Certificate: cacerts/ca.crt
    OrganizationalUnitIdentifier: peer
  AdminOUIdentifier:
    Certificate: cacerts/ca.crt
    OrganizationalUnitIdentifier: admin
  OrdererOUIdentifier:
    Certificate: cacerts/ca.crt
    OrganizationalUnitIdentifier: orderer
`
	if err := os.WriteFile(filepath.Join(mspDir, "config.yaml"), []byte(configYAML), 0644); err != nil {
		return "", fmt.Errorf("failed to write config.yaml: %w", err)
	}

	return mspDir, nil
}

// buildPartyConfig creates a protos.PartyConfig from a GenesisParty
func buildPartyConfig(party GenesisParty, partyID int) (*protos.PartyConfig, error) {
	log.Printf("[fabricx-genesis] party=%d msp=%s router=%s:%d tls=%s",
		partyID, party.MSPID, party.RouterHost, party.RouterPort, CertFingerprint([]byte(party.RouterTLSCert)))
	log.Printf("[fabricx-genesis] party=%d consenter=%s:%d tls=%s",
		partyID, party.ConsenterHost, party.ConsenterPort, CertFingerprint([]byte(party.ConsenterTLSCert)))
	return &protos.PartyConfig{
		PartyID:    uint32(partyID),
		CACerts:    [][]byte{[]byte(party.SignCACert)},
		TLSCACerts: [][]byte{[]byte(party.TLSCACert)},
		RouterConfig: &protos.RouterNodeConfig{
			Host:    party.RouterHost,
			Port:    uint32(party.RouterPort),
			TlsCert: []byte(party.RouterTLSCert),
		},
		BatchersConfig: []*protos.BatcherNodeConfig{
			{
				ShardID:  0,
				Host:     party.BatcherHost,
				Port:     uint32(party.BatcherPort),
				SignCert: []byte(party.BatcherSignCert),
				TlsCert:  []byte(party.BatcherTLSCert),
			},
		},
		ConsenterConfig: &protos.ConsenterNodeConfig{
			Host:     party.ConsenterHost,
			Port:     uint32(party.ConsenterPort),
			SignCert: []byte(party.ConsenterSignCert),
			TlsCert:  []byte(party.ConsenterTLSCert),
		},
		AssemblerConfig: &protos.AssemblerNodeConfig{
			Host:    party.AssemblerHost,
			Port:    uint32(party.AssemblerPort),
			TlsCert: []byte(party.AssemblerTLSCert),
		},
	}, nil
}

// buildSharedConfig creates a SharedConfig with default SmartBFT consensus settings
func buildSharedConfig(partiesConfig []*protos.PartyConfig) *protos.SharedConfig {
	maxPartyID := uint32(0)
	for _, p := range partiesConfig {
		if p.PartyID > maxPartyID {
			maxPartyID = p.PartyID
		}
	}

	return &protos.SharedConfig{
		PartiesConfig: partiesConfig,
		ConsensusConfig: &protos.ConsensusConfig{
			SmartBFTConfig: &protos.SmartBFTConfig{
				RequestBatchMaxCount:          500,
				RequestBatchMaxBytes:          10 * 1024 * 1024,
				RequestBatchMaxInterval:       "2s",
				IncomingMessageBufferSize:     200,
				RequestPoolSize:               1000,
				RequestForwardTimeout:         "3s",
				RequestComplainTimeout:        "10s",
				RequestAutoRemoveTimeout:      "60s",
				ViewChangeResendInterval:      "5s",
				ViewChangeTimeout:             "20s",
				LeaderHeartbeatTimeout:        "10s",
				LeaderHeartbeatCount:          10,
				NumOfTicksBehindBeforeSyncing: 10,
				CollectTimeout:                "10s",
				SyncOnStart:                   false,
				SpeedUpViewChange:             false,
				LeaderRotation:                true,
				DecisionsPerLeader:            1000,
				RequestMaxBytes:               1024 * 1024,
				RequestPoolSubmitTimeout:      "5s",
			},
		},
		BatchingConfig: &protos.BatchingConfig{
			BatchTimeouts: &protos.BatchTimeouts{
				BatchCreationTimeout:  "2s",
				FirstStrikeThreshold:  "5s",
				SecondStrikeThreshold: "10s",
				AutoRemoveTimeout:     "60s",
			},
			BatchSize: &protos.BatchSize{
				MaxMessageCount:   500,
				AbsoluteMaxBytes:  10 * 1024 * 1024,
				PreferredMaxBytes: 2 * 1024 * 1024,
			},
			RequestMaxBytes: 100 * 1024 * 1024,
		},
		MaxPartyID: maxPartyID,
	}
}
