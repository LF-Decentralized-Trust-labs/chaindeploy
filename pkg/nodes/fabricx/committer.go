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
	"github.com/docker/go-connections/nat"
)

// Committer manages the 5 sub-containers of a Fabric X committer:
// sidecar, coordinator, validator, verifier, query-service
type Committer struct {
	db             *db.Queries
	orgService     *fabricservice.OrganizationService
	keyService     *keymanagement.KeyManagementService
	configService  *config.ConfigService
	logger         *logger.Logger
	nodeID         int64
	organizationID int64
	mspID          string
	opts           nodetypes.FabricXCommitterConfig
}

func NewCommitter(
	db *db.Queries,
	orgService *fabricservice.OrganizationService,
	keyService *keymanagement.KeyManagementService,
	configService *config.ConfigService,
	logger *logger.Logger,
	nodeID int64,
	opts nodetypes.FabricXCommitterConfig,
) *Committer {
	return &Committer{
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

func (c *Committer) baseDir() string {
	slug := slugify(c.opts.Name)
	return filepath.Join(c.configService.GetDataPath(), "fabricx-committers", slug)
}

func (c *Committer) prefix() string {
	return containerNamePrefix(c.mspID, c.opts.Name)
}

// Init generates certificates, writes configs, returns deployment config
func (c *Committer) Init() (*nodetypes.FabricXCommitterDeploymentConfig, error) {
	ctx := context.Background()

	org, err := c.orgService.GetOrganization(ctx, c.organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	signCAKeyDB, err := c.keyService.GetKey(ctx, int(org.SignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to get sign CA key: %w", err)
	}
	tlsCAKeyDB, err := c.keyService.GetKey(ctx, int(org.TlsRootKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS CA key: %w", err)
	}

	isCA := 0
	description := fmt.Sprintf("FabricX committer key for %s", c.opts.Name)
	curveP256 := kmodels.ECCurveP256
	providerID := int(org.ProviderID)

	// Create sign key
	signKeyDB, err := c.keyService.CreateKey(ctx, kmodels.CreateKeyRequest{
		Algorithm:   kmodels.KeyAlgorithmEC,
		Name:        c.opts.Name,
		IsCA:        &isCA,
		Description: &description,
		Curve:       &curveP256,
		ProviderID:  &providerID,
	}, int(org.SignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to create sign key: %w", err)
	}

	validFor := kmodels.Duration(time.Hour * 24 * 365)
	signKeyDB, err = c.keyService.SignCertificate(ctx, signKeyDB.ID, signCAKeyDB.ID, kmodels.CertificateRequest{
		CommonName:         c.opts.Name,
		Organization:       []string{c.mspID},
		OrganizationalUnit: []string{"peer"},
		DNSNames:           []string{c.opts.Name},
		IsCA:               false,
		ValidFor:           validFor,
		KeyUsage:           x509.KeyUsageDigitalSignature,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign certificate: %w", err)
	}

	signKey, err := c.keyService.GetDecryptedPrivateKey(int(signKeyDB.ID))
	if err != nil {
		return nil, fmt.Errorf("failed to get sign private key: %w", err)
	}

	// Create TLS key
	tlsKeyDB, err := c.keyService.CreateKey(ctx, kmodels.CreateKeyRequest{
		Algorithm:   kmodels.KeyAlgorithmEC,
		Name:        c.opts.Name + "-tls",
		IsCA:        &isCA,
		Description: &description,
		Curve:       &curveP256,
		ProviderID:  &providerID,
	}, int(org.SignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS key: %w", err)
	}

	domainNames := c.opts.DomainNames
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
	// is later toggled on for the network, Docker Desktop containers dial
	// the committer via this name and need it in the cert; otherwise the
	// extra SAN is harmless.
	if !slices.Contains(domains, localDevHost) {
		domains = append(domains, localDevHost)
	}

	tlsKeyDB, err = c.keyService.SignCertificate(ctx, tlsKeyDB.ID, tlsCAKeyDB.ID, kmodels.CertificateRequest{
		CommonName:         c.opts.Name,
		Organization:       []string{c.mspID},
		OrganizationalUnit: []string{"peer"},
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

	tlsKey, err := c.keyService.GetDecryptedPrivateKey(int(tlsKeyDB.ID))
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS private key: %w", err)
	}

	// Create directory structure
	baseDir := c.baseDir()
	components := []string{"sidecar", "coordinator", "validator", "verifier", "query-service"}
	for _, comp := range components {
		for _, sub := range []string{"config", "msp", "tls", "genesis", "data", "ledger"} {
			if err := os.MkdirAll(filepath.Join(baseDir, comp, sub), 0755); err != nil {
				return nil, fmt.Errorf("failed to create dir %s/%s: %w", comp, sub, err)
			}
		}
	}

	// Write MSP and TLS for sidecar (the only committer component that needs MSP)
	if err := writeMSP(
		filepath.Join(baseDir, "sidecar", "msp"),
		*signKeyDB.Certificate, signKey, *signCAKeyDB.Certificate, *tlsCAKeyDB.Certificate,
	); err != nil {
		return nil, fmt.Errorf("failed to write MSP for sidecar: %w", err)
	}
	if err := writeTLS(
		filepath.Join(baseDir, "sidecar", "tls"),
		*tlsKeyDB.Certificate, tlsKey, *tlsCAKeyDB.Certificate,
	); err != nil {
		return nil, fmt.Errorf("failed to write TLS for sidecar: %w", err)
	}

	prefix := c.prefix()
	version := c.opts.Version
	if version == "" {
		version = DefaultCommitterVersion
	}

	postgresPort := c.opts.PostgresPort
	if postgresPort == 0 {
		postgresPort = 5432
	}
	postgresDB := c.opts.PostgresDB
	if postgresDB == "" {
		postgresDB = "fabricx"
	}
	postgresUser := c.opts.PostgresUser
	if postgresUser == "" {
		postgresUser = "fabricx"
	}
	postgresPassword := c.opts.PostgresPassword
	if postgresPassword == "" {
		postgresPassword = "fabricx"
	}
	channelID := c.opts.ChannelID
	if channelID == "" {
		channelID = "testchannel"
	}

	// Allocate per-role Prometheus /metrics ports unless the caller
	// pinned them. Search starts above the GRPC ports to keep metrics
	// out of the adjacency that GRPC port allocators rely on.
	exclude := map[int]struct{}{
		c.opts.SidecarPort:      {},
		c.opts.CoordinatorPort:  {},
		c.opts.ValidatorPort:    {},
		c.opts.VerifierPort:     {},
		c.opts.QueryServicePort: {},
	}
	monSidecar := c.opts.SidecarMonitoringPort
	monCoordinator := c.opts.CoordinatorMonitoringPort
	monValidator := c.opts.ValidatorMonitoringPort
	monVerifier := c.opts.VerifierMonitoringPort
	monQueryService := c.opts.QueryServiceMonitoringPort
	if monSidecar == 0 || monCoordinator == 0 || monValidator == 0 || monVerifier == 0 || monQueryService == 0 {
		base := c.opts.SidecarPort
		for _, p := range []int{c.opts.CoordinatorPort, c.opts.ValidatorPort, c.opts.VerifierPort, c.opts.QueryServicePort} {
			if p > base {
				base = p
			}
		}
		ports, err := findFreePortsExcluding(base+100, 5, exclude)
		if err != nil {
			return nil, fmt.Errorf("allocate committer monitoring ports: %w", err)
		}
		if monSidecar == 0 {
			monSidecar = ports[0]
		}
		if monCoordinator == 0 {
			monCoordinator = ports[1]
		}
		if monValidator == 0 {
			monValidator = ports[2]
		}
		if monVerifier == 0 {
			monVerifier = ports[3]
		}
		if monQueryService == 0 {
			monQueryService = ports[4]
		}
	}

	cfg := &nodetypes.FabricXCommitterDeploymentConfig{
		BaseDeploymentConfig: nodetypes.BaseDeploymentConfig{
			Type: "fabricx-committer",
			Mode: "docker",
		},
		OrganizationID: c.organizationID,
		MSPID:          c.mspID,
		ExternalIP:     c.opts.ExternalIP,
		DomainNames:    domains,
		Version:        version,
		SignKeyID:      int64(signKeyDB.ID),
		TLSKeyID:       int64(tlsKeyDB.ID),
		SignCert:       *signKeyDB.Certificate,
		TLSCert:        *tlsKeyDB.Certificate,
		CACert:         *signCAKeyDB.Certificate,
		TLSCACert:      *tlsCAKeyDB.Certificate,

		SidecarPort:      c.opts.SidecarPort,
		CoordinatorPort:  c.opts.CoordinatorPort,
		ValidatorPort:    c.opts.ValidatorPort,
		VerifierPort:     c.opts.VerifierPort,
		QueryServicePort: c.opts.QueryServicePort,

		SidecarMonitoringPort:      monSidecar,
		CoordinatorMonitoringPort:  monCoordinator,
		ValidatorMonitoringPort:    monValidator,
		VerifierMonitoringPort:     monVerifier,
		QueryServiceMonitoringPort: monQueryService,

		SidecarContainer:      prefix + "-sidecar",
		CoordinatorContainer:  prefix + "-coordinator",
		ValidatorContainer:    prefix + "-validator",
		VerifierContainer:     prefix + "-verifier",
		QueryServiceContainer: prefix + "-query-service",
		PostgresContainer:     prefix + "-postgres",

		OrdererEndpoints: c.opts.OrdererEndpoints,
		PostgresHost:     c.opts.PostgresHost,
		PostgresPort:     postgresPort,
		PostgresDB:       postgresDB,
		PostgresUser:     postgresUser,
		PostgresPassword: postgresPassword,
		ChannelID:        channelID,
	}

	// Write configs
	if err := c.writeSidecarConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to write sidecar config: %w", err)
	}
	if err := c.writeCoordinatorConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to write coordinator config: %w", err)
	}
	if err := c.writeValidatorConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to write validator config: %w", err)
	}
	if err := c.writeVerifierConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to write verifier config: %w", err)
	}
	if err := c.writeQueryServiceConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to write query-service config: %w", err)
	}

	c.logger.Info("Initialized FabricX committer",
		"name", c.opts.Name,
		"baseDir", baseDir,
	)

	return cfg, nil
}

// SetGenesisBlock writes the genesis block to the sidecar's genesis directory
// and extracts every orderer org's TLS root CA into sidecar/msp/tlscacerts/.
//
// The sidecar's ordererconn client builds its gRPC dial credentials from
// root-ca-paths in its YAML config (see upstream connection/tls.go). If no
// roots are configured it falls back to insecure credentials — which the
// TLS-only orderers reject with "error reading server preface: EOF". The
// upstream sidecar does not yet load root CAs from the genesis block itself
// (sidecar/config.go still has a "TODO: Fetch Root CAs."), so we do it here.
func (c *Committer) SetGenesisBlock(genesisBlock []byte) error {
	dir := filepath.Join(c.baseDir(), "sidecar", "genesis")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create sidecar genesis dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "genesis.block"), genesisBlock, 0644); err != nil {
		return err
	}

	certsByMSP, err := extractOrdererTLSRootCerts(genesisBlock)
	if err != nil {
		return fmt.Errorf("extract tls root ca from genesis: %w", err)
	}
	tlsCADir := filepath.Join(c.baseDir(), "sidecar", "msp", "tlscacerts")
	if err := os.MkdirAll(tlsCADir, 0755); err != nil {
		return fmt.Errorf("failed to create tlscacerts dir: %w", err)
	}
	// Preserve the sidecar's own MSP TLS CA (ca.pem) and drop any previous
	// party CA files from a stale genesis before rewriting.
	if entries, err := os.ReadDir(tlsCADir); err == nil {
		for _, e := range entries {
			name := e.Name()
			if name == "ca.pem" || !strings.HasPrefix(name, "party-") {
				continue
			}
			_ = os.Remove(filepath.Join(tlsCADir, name))
		}
	}
	for mspID, certs := range certsByMSP {
		for i, certPEM := range certs {
			fname := fmt.Sprintf("party-%s-%d.pem", strings.ToLower(mspID), i)
			if err := os.WriteFile(filepath.Join(tlsCADir, fname), certPEM, 0644); err != nil {
				return fmt.Errorf("write tls ca %s: %w", fname, err)
			}
		}
	}
	c.logger.Info("Wrote party TLS root CAs from genesis for sidecar",
		"name", c.opts.Name, "orgs", len(certsByMSP), "dir", tlsCADir)
	return nil
}

// Start launches all committer sub-containers in host network mode (so they
// reach the orderer group at externalIP:externalPort without any Docker
// Desktop hairpin/NAT round-trip). Postgres stays on a per-committer bridge
// network because its image only listens on 5432 — that's still reachable from
// the host-mode committer via 127.0.0.1 on the published postgres port.
// ensureMaterials rebuilds the sidecar MSP/TLS materials from the DB keys if
// they are missing. This lets operators wipe runtime data (ledger, state) and
// safely restart the committer stack.
func (c *Committer) ensureMaterials(cfg *nodetypes.FabricXCommitterDeploymentConfig) error {
	baseDir := c.baseDir()
	components := []string{"sidecar", "coordinator", "validator", "verifier", "query-service"}
	for _, comp := range components {
		for _, sub := range []string{"config", "msp", "tls", "genesis", "data", "ledger"} {
			if err := os.MkdirAll(filepath.Join(baseDir, comp, sub), 0755); err != nil {
				return fmt.Errorf("failed to create dir %s/%s: %w", comp, sub, err)
			}
		}
	}

	keyPath := filepath.Join(baseDir, "sidecar", "tls", "server.key")
	if _, err := os.Stat(keyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat sidecar TLS key: %w", err)
	} else if err == nil {
		// Key present — check for drift between disk cert and the cert in the
		// deployment_config. Same class of failure as the orderer group: a
		// rerun regenerates DB keys, genesis is built from the new keys, but
		// stale disk cert makes the sidecar refuse to bootstrap.
		if len(cfg.TLSCert) > 0 {
			diskCert, rerr := os.ReadFile(filepath.Join(baseDir, "sidecar", "tls", "server.crt"))
			if rerr == nil && bytes.Equal(bytes.TrimSpace(diskCert), bytes.TrimSpace([]byte(cfg.TLSCert))) {
				return nil
			}
			c.logger.Info("FabricX committer sidecar TLS cert drift detected; rewriting from DB", "name", c.opts.Name)
		} else {
			return nil
		}
	}

	if cfg.SignKeyID == 0 || cfg.TLSKeyID == 0 {
		return fmt.Errorf("cannot restore sidecar MSP/TLS: deployment config missing SignKeyID or TLSKeyID")
	}
	signKey, err := c.keyService.GetDecryptedPrivateKey(int(cfg.SignKeyID))
	if err != nil {
		return fmt.Errorf("failed to load sidecar sign private key: %w", err)
	}
	tlsKey, err := c.keyService.GetDecryptedPrivateKey(int(cfg.TLSKeyID))
	if err != nil {
		return fmt.Errorf("failed to load sidecar TLS private key: %w", err)
	}

	if err := writeMSP(
		filepath.Join(baseDir, "sidecar", "msp"),
		cfg.SignCert, signKey, cfg.CACert, cfg.TLSCACert,
	); err != nil {
		return fmt.Errorf("failed to restore sidecar MSP: %w", err)
	}
	if err := writeTLS(
		filepath.Join(baseDir, "sidecar", "tls"),
		cfg.TLSCert, tlsKey, cfg.TLSCACert,
	); err != nil {
		return fmt.Errorf("failed to restore sidecar TLS: %w", err)
	}
	c.logger.Info("Restored FabricX committer sidecar MSP/TLS from DB keys", "name", c.opts.Name)
	return nil
}

// Start is the legacy monolithic entry point. It delegates to the new
// per-role lifecycle (PrepareCommitterStart + StartCommitterRole loop) so
// both legacy callers and the per-child StartNode path share one
// implementation. The embedded postgres container is still started here
// when startPostgres is true; the node_groups coordinator can instead
// externalize postgres by writing it as a services row and calling the
// per-role path directly.
func (c *Committer) Start(cfg *nodetypes.FabricXCommitterDeploymentConfig, startPostgres bool) error {
	ctx := context.Background()

	if err := c.PrepareCommitterStart(ctx, cfg, "", 0); err != nil {
		return err
	}

	if startPostgres {
		if err := c.startPostgres(ctx, cfg, c.CommitterNetworkName()); err != nil {
			return fmt.Errorf("failed to start postgres: %w", err)
		}
		time.Sleep(3 * time.Second)
	}

	roles := []nodetypes.FabricXRole{
		nodetypes.FabricXRoleCommitterSidecar,
		nodetypes.FabricXRoleCommitterCoordinator,
		nodetypes.FabricXRoleCommitterValidator,
		nodetypes.FabricXRoleCommitterVerifier,
		nodetypes.FabricXRoleCommitterQueryService,
	}
	for _, role := range roles {
		if err := c.StartCommitterRole(ctx, cfg, role); err != nil {
			return err
		}
	}
	c.logger.Info("All FabricX committer containers started", "name", c.opts.Name)
	return nil
}

func (c *Committer) startPostgres(ctx context.Context, cfg *nodetypes.FabricXCommitterDeploymentConfig, networkName string) error {
	imageName := fmt.Sprintf("%s:%s", DefaultPostgresImage, DefaultPostgresVersion)
	containerName := cfg.PostgresContainer

	env := map[string]string{
		"POSTGRES_DB":       cfg.PostgresDB,
		"POSTGRES_USER":     cfg.PostgresUser,
		"POSTGRES_PASSWORD": cfg.PostgresPassword,
	}

	// Map the postgres port to the configured port
	containerPort := nat.Port("5432/tcp")
	portBindings := map[nat.Port][]nat.PortBinding{
		containerPort: {{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", cfg.PostgresPort)}},
	}

	return startContainer(ctx, c.logger, imageName, containerName, nil, env, portBindings, nil, networkName, nil)
}

// Stop stops all committer containers
func (c *Committer) Stop(cfg *nodetypes.FabricXCommitterDeploymentConfig) error {
	ctx := context.Background()
	containers := []string{
		cfg.SidecarContainer,
		cfg.CoordinatorContainer,
		cfg.ValidatorContainer,
		cfg.VerifierContainer,
		cfg.QueryServiceContainer,
	}
	if cfg.PostgresContainer != "" {
		containers = append(containers, cfg.PostgresContainer)
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
func (c *Committer) IsHealthy(cfg *nodetypes.FabricXCommitterDeploymentConfig) (bool, error) {
	ctx := context.Background()
	containers := []string{
		cfg.SidecarContainer,
		cfg.CoordinatorContainer,
		cfg.ValidatorContainer,
		cfg.VerifierContainer,
		cfg.QueryServiceContainer,
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

const sidecarConfigTemplate = `server:
  endpoint: 0.0.0.0:{{.SidecarPort}}
  keep-alive:
    params:
      time: 300s
      timeout: 600s
    enforcement-policy:
      min-time: 10s
      permit-without-stream: true
orderer:
  # v0.1.9 used channel-id + consensus-type here. v1.0.0-alpha renamed
  # consensus-type -> fault-tolerance-level and made the genesis/config
  # block path a required field on the sidecar (previously set via the
  # top-level bootstrap block, which the new binary ignores).
  channel-id: {{.ChannelID}}
  fault-tolerance-level: BFT
  latest-known-config-block-path: /etc/hyperledger/fabricx/genesis/genesis.block
  identity:
    msp-id: {{.MSPID}}
    msp-dir: /var/hyperledger/fabricx/msp
  tls:
    mode: tls
    cert-path: /var/hyperledger/fabricx/tls/server.crt
    key-path: /var/hyperledger/fabricx/tls/server.key
{{- if .RootCAPaths}}
    ca-cert-paths:
{{- range .RootCAPaths}}
      - {{.}}
{{- end}}
{{- end}}
committer:
  # v0.1.9: endpoint was an object with host and port. v1.0.0-alpha expects
  # a host:port string.
  endpoint: {{.CoordinatorHost}}:{{.CoordinatorPort}}
ledger:
  path: /var/hyperledger/fabricx/ledger
last-committed-block-set-interval: 5s
# v1.0.0-alpha replaced the old monitoring.prometheus.enabled stanza
# with a top-level monitoring server config. The unknown legacy keys
# would be silently ignored and the metrics endpoint would never start,
# leaving /metrics unreachable. See fabric-x-committer cmd/config/samples/.
monitoring:
  endpoint: 0.0.0.0:{{.MonitoringPort}}
  rate-limit:
    requests-per-second: 0
    burst: 0
logging:
  # logSpec controls per-module log levels (flogging syntax); info is the
  # sane default. level: keys in earlier versions were no-ops — the real
  # field is logSpec and setting Format to anything other than "json"
  # switches flogging to its human-readable FormatEncoder so container
  # logs show up as lines of text instead of JSON blobs.
  logSpec: info
  format: '%{color}%{time:2006-01-02 15:04:05.000 MST} [%{module}] %{shortfunc} -> %{level:.4s}%{color:reset} %{message}'
`

const coordinatorConfigTemplate = `server:
  endpoint:
    host: 0.0.0.0
    port: {{.CoordinatorPort}}
verifier:
  endpoints:
    - {{.VerifierEndpoint}}
validator-committer:
  endpoints:
    - {{.ValidatorEndpoint}}
dependency-graph:
  num-of-local-dep-constructors: 4
  waiting-txs-limit: 20000000
  num-of-workers-for-global-dep-manager: 1
per-channel-buffer-size-per-goroutine: 10
monitoring:
  endpoint: 0.0.0.0:{{.MonitoringPort}}
  rate-limit:
    requests-per-second: 0
    burst: 0
logging:
  # logSpec controls per-module log levels (flogging syntax); info is the
  # sane default. level: keys in earlier versions were no-ops — the real
  # field is logSpec and setting Format to anything other than "json"
  # switches flogging to its human-readable FormatEncoder so container
  # logs show up as lines of text instead of JSON blobs.
  logSpec: info
  format: '%{color}%{time:2006-01-02 15:04:05.000 MST} [%{module}] %{shortfunc} -> %{level:.4s}%{color:reset} %{message}'
`

const validatorConfigTemplate = `server:
  endpoint:
    host: 0.0.0.0
    port: {{.ValidatorPort}}
database:
  endpoints:
    - {{.PostgresHost}}:{{.PostgresPort}}
  username: {{.PostgresUser}}
  password: {{.PostgresPassword}}
  database: {{.PostgresDB}}
  max-connections: 80
  min-connections: 5
  load-balance: false
  retry:
    max-elapsed-time: 1h
resource-limits:
  max-workers-for-preparer: 2
  max-workers-for-validator: 2
  max-workers-for-committer: 20
  min-transaction-batch-size: 1000
monitoring:
  endpoint: 0.0.0.0:{{.MonitoringPort}}
  rate-limit:
    requests-per-second: 0
    burst: 0
logging:
  # logSpec controls per-module log levels (flogging syntax); info is the
  # sane default. level: keys in earlier versions were no-ops — the real
  # field is logSpec and setting Format to anything other than "json"
  # switches flogging to its human-readable FormatEncoder so container
  # logs show up as lines of text instead of JSON blobs.
  logSpec: info
  format: '%{color}%{time:2006-01-02 15:04:05.000 MST} [%{module}] %{shortfunc} -> %{level:.4s}%{color:reset} %{message}'
`

const verifierConfigTemplate = `server:
  endpoint: 0.0.0.0:{{.VerifierPort}}
parallel-executor:
  batch-size-cutoff: 500
  batch-time-cutoff: 2ms
  channel-buffer-size: 1000
  parallelism: 80
monitoring:
  endpoint: 0.0.0.0:{{.MonitoringPort}}
  rate-limit:
    requests-per-second: 0
    burst: 0
logging:
  # logSpec controls per-module log levels (flogging syntax); info is the
  # sane default. level: keys in earlier versions were no-ops — the real
  # field is logSpec and setting Format to anything other than "json"
  # switches flogging to its human-readable FormatEncoder so container
  # logs show up as lines of text instead of JSON blobs.
  logSpec: info
  format: '%{color}%{time:2006-01-02 15:04:05.000 MST} [%{module}] %{shortfunc} -> %{level:.4s}%{color:reset} %{message}'
`

const queryServiceConfigTemplate = `server:
  endpoint: 0.0.0.0:{{.QueryServicePort}}
min-batch-keys: 1024
max-batch-wait: 100ms
view-aggregation-window: 100ms
max-aggregated-views: 1024
max-view-timeout: 10s
database:
  endpoints:
    - {{.PostgresHost}}:{{.PostgresPort}}
  username: {{.PostgresUser}}
  password: {{.PostgresPassword}}
  database: {{.PostgresDB}}
  max-connections: 80
  min-connections: 5
  load-balance: false
  retry:
    max-elapsed-time: 1h
monitoring:
  endpoint: 0.0.0.0:{{.MonitoringPort}}
  rate-limit:
    requests-per-second: 0
    burst: 0
logging:
  # logSpec controls per-module log levels (flogging syntax); info is the
  # sane default. level: keys in earlier versions were no-ops — the real
  # field is logSpec and setting Format to anything other than "json"
  # switches flogging to its human-readable FormatEncoder so container
  # logs show up as lines of text instead of JSON blobs.
  logSpec: info
  format: '%{color}%{time:2006-01-02 15:04:05.000 MST} [%{module}] %{shortfunc} -> %{level:.4s}%{color:reset} %{message}'
`

type sidecarConfigData struct {
	MSPID           string
	ChannelID       string
	SidecarPort     int
	MonitoringPort  int
	CoordinatorHost string
	CoordinatorPort int
	// RootCAPaths are container-side absolute paths to PEM files containing
	// each party's TLS root CA. The sidecar loads them to verify TLS handshakes
	// against every participant's orderer when pulling blocks.
	RootCAPaths []string
}

type coordinatorConfigData struct {
	CoordinatorPort   int
	MonitoringPort    int
	VerifierEndpoint  string
	ValidatorEndpoint string
}

type verifierConfigData struct {
	VerifierPort   int
	MonitoringPort int
}

type dbConfigData struct {
	ValidatorPort    int
	QueryServicePort int
	MonitoringPort   int
	PostgresHost     string
	PostgresPort     int
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
}

func (c *Committer) writeSidecarConfig(cfg *nodetypes.FabricXCommitterDeploymentConfig) error {
	// Enumerate every PEM in the sidecar's tlscacerts dir and emit them as
	// root-ca-paths. SetGenesisBlock pre-populated this dir with the TLS CA
	// from each orderer org. Without these paths the sidecar falls back to
	// insecure gRPC and the TLS-only orderers reject the handshake with
	// "error reading server preface: EOF".
	hostTLSDir := filepath.Join(c.baseDir(), "sidecar", "msp", "tlscacerts")
	var rootCAPaths []string
	if entries, err := os.ReadDir(hostTLSDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if !strings.HasSuffix(e.Name(), ".pem") {
				continue
			}
			rootCAPaths = append(rootCAPaths,
				filepath.Join("/var/hyperledger/fabricx/msp/tlscacerts", e.Name()))
		}
	}
	data := sidecarConfigData{
		MSPID:           cfg.MSPID,
		ChannelID:       cfg.ChannelID,
		SidecarPort:     cfg.SidecarPort,
		MonitoringPort:  cfg.SidecarMonitoringPort,
		CoordinatorHost: cfg.CoordinatorContainer,
		CoordinatorPort: cfg.CoordinatorPort,
		RootCAPaths:     rootCAPaths,
	}
	return writeTemplate(sidecarConfigTemplate, filepath.Join(c.baseDir(), "sidecar", "config", "sidecar_config.yaml"), data)
}

func (c *Committer) writeCoordinatorConfig(cfg *nodetypes.FabricXCommitterDeploymentConfig) error {
	// Committer siblings share a per-committer bridge network, so reach each
	// other by container name on the container port.
	data := coordinatorConfigData{
		CoordinatorPort:   cfg.CoordinatorPort,
		MonitoringPort:    cfg.CoordinatorMonitoringPort,
		VerifierEndpoint:  fmt.Sprintf("%s:%d", cfg.VerifierContainer, cfg.VerifierPort),
		ValidatorEndpoint: fmt.Sprintf("%s:%d", cfg.ValidatorContainer, cfg.ValidatorPort),
	}
	return writeTemplate(coordinatorConfigTemplate, filepath.Join(c.baseDir(), "coordinator", "config", "coordinator_config.yaml"), data)
}

func (c *Committer) writeValidatorConfig(cfg *nodetypes.FabricXCommitterDeploymentConfig) error {
	pgHost, pgPort := c.postgresEndpoint(cfg)
	data := dbConfigData{
		ValidatorPort:    cfg.ValidatorPort,
		MonitoringPort:   cfg.ValidatorMonitoringPort,
		PostgresHost:     pgHost,
		PostgresPort:     pgPort,
		PostgresUser:     cfg.PostgresUser,
		PostgresPassword: cfg.PostgresPassword,
		PostgresDB:       cfg.PostgresDB,
	}
	return writeTemplate(validatorConfigTemplate, filepath.Join(c.baseDir(), "validator", "config", "validator_config.yaml"), data)
}

func (c *Committer) writeVerifierConfig(cfg *nodetypes.FabricXCommitterDeploymentConfig) error {
	data := verifierConfigData{
		VerifierPort:   cfg.VerifierPort,
		MonitoringPort: cfg.VerifierMonitoringPort,
	}
	return writeTemplate(verifierConfigTemplate, filepath.Join(c.baseDir(), "verifier", "config", "verifier_config.yaml"), data)
}

func (c *Committer) writeQueryServiceConfig(cfg *nodetypes.FabricXCommitterDeploymentConfig) error {
	pgHost, pgPort := c.postgresEndpoint(cfg)
	data := dbConfigData{
		QueryServicePort: cfg.QueryServicePort,
		MonitoringPort:   cfg.QueryServiceMonitoringPort,
		PostgresHost:     pgHost,
		PostgresPort:     pgPort,
		PostgresUser:     cfg.PostgresUser,
		PostgresPassword: cfg.PostgresPassword,
		PostgresDB:       cfg.PostgresDB,
	}
	return writeTemplate(queryServiceConfigTemplate, filepath.Join(c.baseDir(), "query-service", "config", "config.yaml"), data)
}

// postgresEndpoint returns the host+port committer components should dial to
// reach postgres. When postgres is started locally (same bridge network) we
// prefer the container name + internal port 5432, which is independent of
// whatever external host port was published. If the user configured an
// external postgres, we return the configured host/port unchanged.
//
// In local-dev mode (CHAINLAUNCH_FABRICX_LOCAL_DEV=true) a PostgresHost of
// 127.0.0.1 / localhost resolves to the container's own loopback inside
// Docker — not the host — because getent consults the loopback interface
// before /etc/hosts. Rewrite to host.docker.internal so Docker Desktop's
// built-in host-gateway name resolves correctly.
func (c *Committer) postgresEndpoint(cfg *nodetypes.FabricXCommitterDeploymentConfig) (string, int) {
	if cfg.PostgresContainer != "" {
		return cfg.PostgresContainer, 5432
	}
	host := cfg.PostgresHost
	if resolveLocalDevForNode(context.Background(), c.db, c.configService, c.nodeID) {
		if host == "127.0.0.1" || host == "localhost" {
			host = localDevHost
		}
	}
	return host, cfg.PostgresPort
}

// RenewCertificates re-signs the committer's signing and TLS certs
// using the SAME key pair already in the DB (matching Fabric peer
// renewal semantics). Returns an updated deployment config with the
// new SignCert/TLSCert. The caller persists it and restarts the five
// child containers so the rewritten msp/tls dirs take effect.
//
// Only the sidecar materializes MSP/TLS on disk (the other four
// committer roles run unauthenticated on the per-group bridge
// network), so the on-disk rewrite touches just sidecar/msp + sidecar/tls.
func (c *Committer) RenewCertificates(cfg *nodetypes.FabricXCommitterDeploymentConfig) (*nodetypes.FabricXCommitterDeploymentConfig, error) {
	ctx := context.Background()
	c.logger.Info("Starting FabricX committer certificate renewal", "name", c.opts.Name)

	if cfg.SignKeyID == 0 || cfg.TLSKeyID == 0 {
		return nil, fmt.Errorf("committer missing SignKeyID/TLSKeyID; cannot renew")
	}

	org, err := c.orgService.GetOrganization(ctx, c.organizationID)
	if err != nil {
		return nil, fmt.Errorf("get organization: %w", err)
	}

	signCAKey, err := c.keyService.GetKey(ctx, int(org.SignKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("get sign CA key: %w", err)
	}
	tlsCAKey, err := c.keyService.GetKey(ctx, int(org.TlsRootKeyID.Int64))
	if err != nil {
		return nil, fmt.Errorf("get TLS CA key: %w", err)
	}

	signKeyDB, err := c.keyService.GetKey(ctx, int(cfg.SignKeyID))
	if err != nil {
		return nil, fmt.Errorf("get sign key: %w", err)
	}
	if signKeyDB.SigningKeyID == nil || *signKeyDB.SigningKeyID == 0 {
		if err := c.keyService.SetSigningKeyIDForKey(ctx, int(cfg.SignKeyID), int(signCAKey.ID)); err != nil {
			return nil, fmt.Errorf("set signing key id on sign key: %w", err)
		}
	}
	tlsKeyDB, err := c.keyService.GetKey(ctx, int(cfg.TLSKeyID))
	if err != nil {
		return nil, fmt.Errorf("get TLS key: %w", err)
	}
	if tlsKeyDB.SigningKeyID == nil || *tlsKeyDB.SigningKeyID == 0 {
		if err := c.keyService.SetSigningKeyIDForKey(ctx, int(cfg.TLSKeyID), int(tlsCAKey.ID)); err != nil {
			return nil, fmt.Errorf("set signing key id on TLS key: %w", err)
		}
	}

	validFor := kmodels.Duration(time.Hour * 24 * 365)

	renewedSignKeyDB, err := c.keyService.RenewCertificate(ctx, int(cfg.SignKeyID), kmodels.CertificateRequest{
		CommonName:         c.opts.Name,
		Organization:       []string{c.mspID},
		OrganizationalUnit: []string{"peer"},
		DNSNames:           []string{c.opts.Name},
		IsCA:               false,
		ValidFor:           validFor,
		KeyUsage:           x509.KeyUsageDigitalSignature,
	})
	if err != nil {
		return nil, fmt.Errorf("renew signing certificate: %w", err)
	}

	domains, ipAddresses := expandFabricxSANs(cfg.DomainNames)
	renewedTLSKeyDB, err := c.keyService.RenewCertificate(ctx, int(cfg.TLSKeyID), kmodels.CertificateRequest{
		CommonName:         c.opts.Name,
		Organization:       []string{c.mspID},
		OrganizationalUnit: []string{"peer"},
		DNSNames:           domains,
		IPAddresses:        ipAddresses,
		IsCA:               false,
		ValidFor:           validFor,
		KeyUsage:           x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	if err != nil {
		return nil, fmt.Errorf("renew TLS certificate: %w", err)
	}

	if renewedSignKeyDB.Certificate == nil || renewedTLSKeyDB.Certificate == nil {
		return nil, fmt.Errorf("renewed certificates returned with nil PEM")
	}

	updated := *cfg
	updated.SignCert = *renewedSignKeyDB.Certificate
	updated.TLSCert = *renewedTLSKeyDB.Certificate

	signKey, err := c.keyService.GetDecryptedPrivateKey(int(cfg.SignKeyID))
	if err != nil {
		return nil, fmt.Errorf("decrypt sign key: %w", err)
	}
	tlsKey, err := c.keyService.GetDecryptedPrivateKey(int(cfg.TLSKeyID))
	if err != nil {
		return nil, fmt.Errorf("decrypt TLS key: %w", err)
	}

	baseDir := c.baseDir()
	if err := writeMSP(
		filepath.Join(baseDir, "sidecar", "msp"),
		updated.SignCert, signKey, updated.CACert, updated.TLSCACert,
	); err != nil {
		return nil, fmt.Errorf("write renewed sidecar MSP: %w", err)
	}
	if err := writeTLS(
		filepath.Join(baseDir, "sidecar", "tls"),
		updated.TLSCert, tlsKey, updated.TLSCACert,
	); err != nil {
		return nil, fmt.Errorf("write renewed sidecar TLS: %w", err)
	}

	c.logger.Info("Successfully renewed FabricX committer certificates", "name", c.opts.Name)
	return &updated, nil
}
