package fabricx

import (
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/config"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	fabricservice "github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	kmodels "github.com/chainlaunch/chainlaunch/pkg/keymanagement/models"
	keymanagement "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// OrdererGroup manages the 4 sub-containers of a Fabric X orderer group:
// router, batcher, consenter, assembler
type OrdererGroup struct {
	db             *db.Queries
	orgService     *fabricservice.OrganizationService
	keyService     *keymanagement.KeyManagementService
	configService  *config.ConfigService
	logger         *logger.Logger
	nodeID         int64
	organizationID int64
	mspID          string
	opts           nodetypes.FabricXOrdererGroupConfig
}

func NewOrdererGroup(
	db *db.Queries,
	orgService *fabricservice.OrganizationService,
	keyService *keymanagement.KeyManagementService,
	configService *config.ConfigService,
	logger *logger.Logger,
	nodeID int64,
	opts nodetypes.FabricXOrdererGroupConfig,
) *OrdererGroup {
	return &OrdererGroup{
		db:             db,
		orgService:     orgService,
		keyService:     keyService,
		configService:  configService,
		logger:         logger,
		nodeID:         nodeID,
		organizationID: opts.OrganizationID,
		mspID:          opts.MSPID,
		opts:           opts,
	}
}

func (og *OrdererGroup) baseDir() string {
	slug := slugify(og.opts.Name)
	return filepath.Join(og.configService.GetDataPath(), "fabricx-orderers", slug)
}

func (og *OrdererGroup) prefix() string {
	return containerNamePrefix(og.mspID, og.opts.Name)
}

// Init generates certificates, writes config files, and returns the deployment config
func (og *OrdererGroup) Init() (*nodetypes.FabricXOrdererGroupDeploymentConfig, error) {
	ctx := context.Background()

	// Get organization
	org, err := og.orgService.GetOrganization(ctx, og.organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	signCAKeyDB, err := og.keyService.GetKey(ctx, int(org.SignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to get sign CA key: %w", err)
	}
	tlsCAKeyDB, err := og.keyService.GetKey(ctx, int(org.TlsRootKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS CA key: %w", err)
	}

	isCA := 0
	description := fmt.Sprintf("FabricX orderer group key for %s", og.opts.Name)
	curveP256 := kmodels.ECCurveP256
	providerID := int(org.ProviderID)

	// Create sign key
	signKeyDB, err := og.keyService.CreateKey(ctx, kmodels.CreateKeyRequest{
		Algorithm:   kmodels.KeyAlgorithmEC,
		Name:        og.opts.Name,
		IsCA:        &isCA,
		Description: &description,
		Curve:       &curveP256,
		ProviderID:  &providerID,
	}, int(org.SignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to create sign key: %w", err)
	}

	validFor := kmodels.Duration(time.Hour * 24 * 365)
	signKeyDB, err = og.keyService.SignCertificate(ctx, signKeyDB.ID, signCAKeyDB.ID, kmodels.CertificateRequest{
		CommonName:         og.opts.Name,
		Organization:       []string{og.mspID},
		OrganizationalUnit: []string{"orderer"},
		DNSNames:           []string{og.opts.Name},
		IsCA:               false,
		ValidFor:           validFor,
		KeyUsage:           x509.KeyUsageDigitalSignature,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign certificate: %w", err)
	}

	signKey, err := og.keyService.GetDecryptedPrivateKey(int(signKeyDB.ID))
	if err != nil {
		return nil, fmt.Errorf("failed to get sign private key: %w", err)
	}

	// Create TLS key
	tlsKeyDB, err := og.keyService.CreateKey(ctx, kmodels.CreateKeyRequest{
		Algorithm:   kmodels.KeyAlgorithmEC,
		Name:        og.opts.Name + "-tls",
		IsCA:        &isCA,
		Description: &description,
		Curve:       &curveP256,
		ProviderID:  &providerID,
	}, int(org.SignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS key: %w", err)
	}

	domainNames := og.opts.DomainNames
	var ipAddresses []net.IP
	var domains []string
	hasLocalhost := false
	hasLoopback := false
	for _, domain := range domainNames {
		if domain == "localhost" {
			hasLocalhost = true
			domains = append(domains, domain)
			continue
		}
		if domain == "127.0.0.1" {
			hasLoopback = true
			ipAddresses = append(ipAddresses, net.ParseIP(domain))
			continue
		}
		if ip := net.ParseIP(domain); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		} else {
			domains = append(domains, domain)
		}
	}
	if !hasLocalhost {
		domains = append(domains, "localhost")
	}
	if !hasLoopback {
		ipAddresses = append(ipAddresses, net.ParseIP("127.0.0.1"))
	}
	// Always include host.docker.internal in the SANs. When local-dev mode
	// is later toggled on for a network, Docker Desktop containers dial each
	// other via this name and need it in the cert; when local-dev is off, the
	// extra SAN is harmless because nothing dials that name.
	if !slices.Contains(domains, localDevHost) {
		domains = append(domains, localDevHost)
	}

	tlsKeyDB, err = og.keyService.SignCertificate(ctx, tlsKeyDB.ID, tlsCAKeyDB.ID, kmodels.CertificateRequest{
		CommonName:         og.opts.Name,
		Organization:       []string{og.mspID},
		OrganizationalUnit: []string{"orderer"},
		DNSNames:           domains,
		IPAddresses:        ipAddresses,
		IsCA:               false,
		ValidFor:           validFor,
		KeyUsage:           x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign TLS certificate: %w", err)
	}

	tlsKey, err := og.keyService.GetDecryptedPrivateKey(int(tlsKeyDB.ID))
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS private key: %w", err)
	}

	// Create directory structure
	baseDir := og.baseDir()
	components := []string{"router", "batcher", "consenter", "assembler"}
	for _, comp := range components {
		for _, sub := range []string{"config", "msp", "tls", "genesis", "data", "store"} {
			if err := os.MkdirAll(filepath.Join(baseDir, comp, sub), 0755); err != nil {
				return nil, fmt.Errorf("failed to create dir %s/%s: %w", comp, sub, err)
			}
		}
	}

	// Write MSP and TLS for each component
	for _, comp := range components {
		if err := writeMSP(
			filepath.Join(baseDir, comp, "msp"),
			*signKeyDB.Certificate, signKey, *signCAKeyDB.Certificate, *tlsCAKeyDB.Certificate,
		); err != nil {
			return nil, fmt.Errorf("failed to write MSP for %s: %w", comp, err)
		}
		if err := writeTLS(
			filepath.Join(baseDir, comp, "tls"),
			*tlsKeyDB.Certificate, tlsKey, *tlsCAKeyDB.Certificate,
		); err != nil {
			return nil, fmt.Errorf("failed to write TLS for %s: %w", comp, err)
		}
	}

	prefix := og.prefix()
	version := og.opts.Version
	if version == "" {
		version = DefaultOrdererVersion
	}

	consenterType := og.opts.ConsenterType
	if consenterType == "" {
		consenterType = "pbft"
	}

	cfg := &nodetypes.FabricXOrdererGroupDeploymentConfig{
		BaseDeploymentConfig: nodetypes.BaseDeploymentConfig{
			Type: "fabricx-orderer-group",
			Mode: "docker",
		},
		OrganizationID: og.organizationID,
		MSPID:          og.mspID,
		PartyID:        og.opts.PartyID,
		ExternalIP:     og.opts.ExternalIP,
		DomainNames:    domains,
		Version:        version,
		SignKeyID:      int64(signKeyDB.ID),
		TLSKeyID:      int64(tlsKeyDB.ID),
		SignCert:       *signKeyDB.Certificate,
		TLSCert:        *tlsKeyDB.Certificate,
		CACert:         *signCAKeyDB.Certificate,
		TLSCACert:      *tlsCAKeyDB.Certificate,
		RouterPort:     og.opts.RouterPort,
		BatcherPort:    og.opts.BatcherPort,
		ConsenterPort:  og.opts.ConsenterPort,
		AssemblerPort:  og.opts.AssemblerPort,
		ConsenterType:  consenterType,

		RouterContainer:    prefix + "-router",
		BatcherContainer:   prefix + "-batcher",
		ConsenterContainer: prefix + "-consenter",
		AssemblerContainer: prefix + "-assembler",
	}

	// Write config files for each component
	if err := og.writeRouterConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to write router config: %w", err)
	}
	if err := og.writeBatcherConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to write batcher config: %w", err)
	}
	if err := og.writeConsenterConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to write consenter config: %w", err)
	}
	if err := og.writeAssemblerConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to write assembler config: %w", err)
	}

	og.logger.Info("Initialized FabricX orderer group",
		"name", og.opts.Name,
		"partyId", og.opts.PartyID,
		"baseDir", baseDir,
	)

	return cfg, nil
}

// SetGenesisBlock writes the genesis block to the orderer group's genesis directories
func (og *OrdererGroup) SetGenesisBlock(genesisBlock []byte) error {
	baseDir := og.baseDir()
	components := []string{"router", "batcher", "consenter", "assembler"}
	for _, comp := range components {
		dir := filepath.Join(baseDir, comp, "genesis")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create genesis dir for %s: %w", comp, err)
		}
		path := filepath.Join(dir, "genesis.block")
		if err := os.WriteFile(path, genesisBlock, 0644); err != nil {
			return fmt.Errorf("failed to write genesis block for %s: %w", comp, err)
		}
	}
	return nil
}

// Start launches all 4 sub-containers on a bridge network with published
// ports. In local-dev mode (CHAINLAUNCH_FABRICX_LOCAL_DEV=true) we also add
// an extra_hosts entry mapping the externalIP to host-gateway, so that
// containers dialing peers at externalIP:port on Docker Desktop Mac/Windows
// actually route out to the host where the published ports live (working
// around the hairpin-NAT limitation). In production on Linux the externalIP
// is directly reachable from containers, so no extra_hosts is needed.
// ensureMaterials re-creates the directory tree and rewrites MSP/TLS from the
// signing/TLS keys stored in the DB when any component's TLS key is missing.
// This lets operators safely wipe data/ state between runs without losing the
// node's identity.
func (og *OrdererGroup) ensureMaterials(cfg *nodetypes.FabricXOrdererGroupDeploymentConfig) error {
	baseDir := og.baseDir()
	components := []string{"router", "batcher", "consenter", "assembler"}
	for _, comp := range components {
		for _, sub := range []string{"config", "msp", "tls", "genesis", "data", "store"} {
			if err := os.MkdirAll(filepath.Join(baseDir, comp, sub), 0755); err != nil {
				return fmt.Errorf("failed to create dir %s/%s: %w", comp, sub, err)
			}
		}
	}

	needsRestore := false
	for _, comp := range components {
		keyPath := filepath.Join(baseDir, comp, "tls", "server.key")
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			needsRestore = true
			break
		}
		// Detect drift between on-disk cert and the cert carried in the
		// deployment_config. After Init() regenerates keys in the DB (e.g. on
		// a rerun where the DB rows were re-created but data/ was not wiped),
		// the on-disk cert is stale. The genesis block will have been built
		// from the fresh DB keys, so the orderer will crash at startup with
		// "local TLS cert differs from shared config TLS cert". Rewriting
		// from the DB here aligns disk with what genesis expects.
		if len(cfg.TLSCert) > 0 {
			diskCert, err := os.ReadFile(filepath.Join(baseDir, comp, "tls", "server.crt"))
			if err != nil || !bytes.Equal(bytes.TrimSpace(diskCert), bytes.TrimSpace([]byte(cfg.TLSCert))) {
				og.logger.Info("FabricX orderer TLS cert drift detected; rewriting from DB",
					"component", comp, "name", og.opts.Name)
				needsRestore = true
				break
			}
		}
	}
	if !needsRestore {
		return nil
	}

	if cfg.SignKeyID == 0 || cfg.TLSKeyID == 0 {
		return fmt.Errorf("cannot restore MSP/TLS: deployment config missing SignKeyID or TLSKeyID")
	}
	signKey, err := og.keyService.GetDecryptedPrivateKey(int(cfg.SignKeyID))
	if err != nil {
		return fmt.Errorf("failed to load sign private key: %w", err)
	}
	tlsKey, err := og.keyService.GetDecryptedPrivateKey(int(cfg.TLSKeyID))
	if err != nil {
		return fmt.Errorf("failed to load TLS private key: %w", err)
	}

	for _, comp := range components {
		if err := writeMSP(
			filepath.Join(baseDir, comp, "msp"),
			cfg.SignCert, signKey, cfg.CACert, cfg.TLSCACert,
		); err != nil {
			return fmt.Errorf("failed to restore MSP for %s: %w", comp, err)
		}
		if err := writeTLS(
			filepath.Join(baseDir, comp, "tls"),
			cfg.TLSCert, tlsKey, cfg.TLSCACert,
		); err != nil {
			return fmt.Errorf("failed to restore TLS for %s: %w", comp, err)
		}
	}
	og.logger.Info("Restored FabricX orderer MSP/TLS from DB keys", "name", og.opts.Name)
	return nil
}

// Start is the legacy monolithic entry point. It now delegates to the new
// per-role lifecycle (PrepareOrdererStart + StartOrdererRole loop) so both
// the pre-node_groups callers and the new per-child StartNode path share one
// implementation. Order follows the documented start sequence.
func (og *OrdererGroup) Start(cfg *nodetypes.FabricXOrdererGroupDeploymentConfig) error {
	ctx := context.Background()

	if err := og.PrepareOrdererStart(cfg); err != nil {
		return err
	}

	roles := []nodetypes.FabricXRole{
		nodetypes.FabricXRoleOrdererRouter,
		nodetypes.FabricXRoleOrdererBatcher,
		nodetypes.FabricXRoleOrdererConsenter,
		nodetypes.FabricXRoleOrdererAssembler,
	}
	for _, role := range roles {
		if err := og.StartOrdererRole(ctx, cfg, role); err != nil {
			return err
		}
	}
	og.logger.Info("All FabricX orderer group containers started", "name", og.opts.Name)
	return nil
}

// Stop stops all orderer group containers
func (og *OrdererGroup) Stop(cfg *nodetypes.FabricXOrdererGroupDeploymentConfig) error {
	ctx := context.Background()
	containers := []string{
		cfg.RouterContainer,
		cfg.BatcherContainer,
		cfg.ConsenterContainer,
		cfg.AssemblerContainer,
	}
	var errs []string
	for _, name := range containers {
		if err := stopContainer(ctx, name); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to stop some containers: %s", strings.Join(errs, "; "))
	}
	return nil
}

// IsHealthy checks if all sub-containers are running
func (og *OrdererGroup) IsHealthy(cfg *nodetypes.FabricXOrdererGroupDeploymentConfig) (bool, error) {
	ctx := context.Background()
	containers := []string{
		cfg.RouterContainer,
		cfg.BatcherContainer,
		cfg.ConsenterContainer,
		cfg.AssemblerContainer,
	}
	for _, name := range containers {
		running, err := isContainerRunning(ctx, name)
		if err != nil {
			return false, err
		}
		if !running {
			return false, nil
		}
	}
	return true, nil
}

// --- Config templates ---

const routerConfigTemplate = `PartyID: {{.PartyID}}
General:
    ListenAddress: 0.0.0.0
    ListenPort: {{.RouterPort}}
    TLS:
        Enabled: true
        PrivateKey: /etc/hyperledger/fabricx/router/tls/server.key
        Certificate: /etc/hyperledger/fabricx/router/tls/server.crt
        RootCAs:
            - /etc/hyperledger/fabricx/router/tls/ca.crt
        ClientAuthRequired: false
    Keepalive:
        ClientInterval: 1m0s
        ClientTimeout: 20s
        ServerInterval: 2h0m0s
        ServerTimeout: 20s
        ServerMinInterval: 1m0s
    Backoff:
        BaseDelay: 1s
        Multiplier: 1.6
        MaxDelay: 2m0s
    MaxRecvMsgSize: 104857600
    MaxSendMsgSize: 104857600
    Bootstrap:
        Method: block
        File: /etc/hyperledger/fabricx/router/genesis/genesis.block
    LocalMSPDir: /etc/hyperledger/fabricx/router/msp
    LocalMSPID: {{.MSPID}}
    LogSpec: debug
FileStore:
    Location: /etc/hyperledger/fabricx/router/store
Router:
    NumberOfConnectionsPerBatcher: 12
    NumberOfStreamsPerConnection: 6
`

const batcherConfigTemplate = `PartyID: {{.PartyID}}
General:
    ListenAddress: 0.0.0.0
    ListenPort: {{.BatcherPort}}
    TLS:
        Enabled: true
        PrivateKey: /etc/hyperledger/fabricx/batcher/tls/server.key
        Certificate: /etc/hyperledger/fabricx/batcher/tls/server.crt
        RootCAs:
            - /etc/hyperledger/fabricx/batcher/tls/ca.crt
        ClientAuthRequired: false
    Keepalive:
        ClientInterval: 1m0s
        ClientTimeout: 20s
        ServerInterval: 2h0m0s
        ServerTimeout: 20s
        ServerMinInterval: 1m0s
    Backoff:
        BaseDelay: 1s
        Multiplier: 1.6
        MaxDelay: 2m0s
    MaxRecvMsgSize: 104857600
    MaxSendMsgSize: 104857600
    Bootstrap:
        Method: block
        File: /etc/hyperledger/fabricx/batcher/genesis/genesis.block
    LocalMSPDir: /etc/hyperledger/fabricx/batcher/msp
    LocalMSPID: {{.MSPID}}
    LogSpec: debug
FileStore:
    Location: /etc/hyperledger/fabricx/batcher/store
Batcher:
    ShardID: 0
    BatchSequenceGap: 12
    MemPoolMaxSize: 1200000
    SubmitTimeout: 600ms
`

const consenterConfigTemplate = `PartyID: {{.PartyID}}
ConsenterID: {{.PartyID}}
General:
    ListenAddress: 0.0.0.0
    ListenPort: {{.ConsenterPort}}
    TLS:
        Enabled: true
        PrivateKey: /etc/hyperledger/fabricx/consenter/tls/server.key
        Certificate: /etc/hyperledger/fabricx/consenter/tls/server.crt
        RootCAs:
            - /etc/hyperledger/fabricx/consenter/tls/ca.crt
        ClientAuthRequired: false
    Keepalive:
        ClientInterval: 1m0s
        ClientTimeout: 20s
        ServerInterval: 2h0m0s
        ServerTimeout: 20s
        ServerMinInterval: 1m0s
    Backoff:
        BaseDelay: 1s
        Multiplier: 1.6
        MaxDelay: 2m0s
    MaxRecvMsgSize: 104857600
    MaxSendMsgSize: 104857600
    Bootstrap:
        Method: block
        File: /etc/hyperledger/fabricx/consenter/genesis/genesis.block
    Cluster:
        SendBufferSize: 2000
        ClientCertificate: /etc/hyperledger/fabricx/consenter/tls/server.crt
        ClientPrivateKey: /etc/hyperledger/fabricx/consenter/tls/server.key
    LocalMSPDir: /etc/hyperledger/fabricx/consenter/msp
    LocalMSPID: {{.MSPID}}
    LogSpec: debug
FileStore:
    Location: /etc/hyperledger/fabricx/consenter/data/store
Consensus:
    WALDir: /etc/hyperledger/fabricx/consenter/data/wal
    ConsensusType: {{.ConsenterType}}
    BatchTimeout: 2s
    BatchSize:
        MaxMessageCount: 500
        AbsoluteMaxBytes: 10MB
        PreferredMaxBytes: 2MB
`

const assemblerConfigTemplate = `PartyID: {{.PartyID}}
General:
    ListenAddress: 0.0.0.0
    ListenPort: {{.AssemblerPort}}
    TLS:
        Enabled: true
        PrivateKey: /etc/hyperledger/fabricx/assembler/tls/server.key
        Certificate: /etc/hyperledger/fabricx/assembler/tls/server.crt
        RootCAs:
            - /etc/hyperledger/fabricx/assembler/tls/ca.crt
        ClientAuthRequired: false
    Keepalive:
        ClientInterval: 1m0s
        ClientTimeout: 20s
        ServerInterval: 2h0m0s
        ServerTimeout: 20s
        ServerMinInterval: 1m0s
    Backoff:
        BaseDelay: 1s
        Multiplier: 1.6
        MaxDelay: 2m0s
    MaxRecvMsgSize: 104857600
    MaxSendMsgSize: 104857600
    Bootstrap:
        Method: block
        File: /etc/hyperledger/fabricx/assembler/genesis/genesis.block
    LocalMSPDir: /etc/hyperledger/fabricx/assembler/msp
    LocalMSPID: {{.MSPID}}
    LogSpec: debug
FileStore:
    Location: /etc/hyperledger/fabricx/assembler/store
Assembler:
    PrefetchBufferMemoryBytes: 1342177280
    RestartLedgerScanTimeout: 6s
    PrefetchEvictionTtl: 1h30m0s
    ReplicationChannelSize: 120
    BatchRequestsChannelSize: 1200
`

type ordererConfigData struct {
	PartyID       int
	MSPID         string
	ConsenterType string
	RouterPort    int
	BatcherPort   int
	ConsenterPort int
	AssemblerPort int
}

func (og *OrdererGroup) ordererConfigData(cfg *nodetypes.FabricXOrdererGroupDeploymentConfig) ordererConfigData {
	return ordererConfigData{
		PartyID:       cfg.PartyID,
		MSPID:         cfg.MSPID,
		ConsenterType: cfg.ConsenterType,
		RouterPort:    cfg.RouterPort,
		BatcherPort:   cfg.BatcherPort,
		ConsenterPort: cfg.ConsenterPort,
		AssemblerPort: cfg.AssemblerPort,
	}
}

func (og *OrdererGroup) writeRouterConfig(cfg *nodetypes.FabricXOrdererGroupDeploymentConfig) error {
	return writeTemplate(routerConfigTemplate, filepath.Join(og.baseDir(), "router", "config", "node_config.yaml"), og.ordererConfigData(cfg))
}

func (og *OrdererGroup) writeBatcherConfig(cfg *nodetypes.FabricXOrdererGroupDeploymentConfig) error {
	return writeTemplate(batcherConfigTemplate, filepath.Join(og.baseDir(), "batcher", "config", "node_config.yaml"), og.ordererConfigData(cfg))
}

func (og *OrdererGroup) writeConsenterConfig(cfg *nodetypes.FabricXOrdererGroupDeploymentConfig) error {
	return writeTemplate(consenterConfigTemplate, filepath.Join(og.baseDir(), "consenter", "config", "node_config.yaml"), og.ordererConfigData(cfg))
}

func (og *OrdererGroup) writeAssemblerConfig(cfg *nodetypes.FabricXOrdererGroupDeploymentConfig) error {
	return writeTemplate(assemblerConfigTemplate, filepath.Join(og.baseDir(), "assembler", "config", "node_config.yaml"), og.ordererConfigData(cfg))
}
