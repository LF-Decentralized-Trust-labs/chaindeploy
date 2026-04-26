//go:build integration

package fabricx

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/config"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	fabricservice "github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	keymanagement "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	_ "github.com/mattn/go-sqlite3"
)

// getContainerLogs retrieves the last N lines from a container's logs
func getContainerLogs(ctx context.Context, containerName string, tailLines int) (string, error) {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}
	defer cli.Close()

	reader, err := cli.ContainerLogs(ctx, containerName, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tailLines),
	})
	if err != nil {
		return "", err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// testEnv holds all the services needed for integration tests
type testEnv struct {
	db            *db.Queries
	sqlDB         *sql.DB
	keyService    *keymanagement.KeyManagementService
	orgService    *fabricservice.OrganizationService
	configService *config.ConfigService
	logger        *logger.Logger
	dataDir       string
	cleanup       func()
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	// Create temp directory for test data
	dataDir, err := os.MkdirTemp("", "fabricx-integration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create SQLite database
	dbPath := filepath.Join(dataDir, "test.db")
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		os.RemoveAll(dataDir)
		t.Fatalf("failed to open database: %v", err)
	}

	// Run migrations
	if err := db.RunMigrations(sqlDB); err != nil {
		sqlDB.Close()
		os.RemoveAll(dataDir)
		t.Fatalf("failed to run migrations: %v", err)
	}

	queries := db.New(sqlDB)
	configService := config.NewConfigService(dataDir)

	keyService, err := keymanagement.NewKeyManagementService(queries)
	if err != nil {
		sqlDB.Close()
		os.RemoveAll(dataDir)
		t.Fatalf("failed to create key management service: %v", err)
	}

	// Initialize default key provider
	if err := keyService.InitializeKeyProviders(context.Background()); err != nil {
		sqlDB.Close()
		os.RemoveAll(dataDir)
		t.Fatalf("failed to initialize key providers: %v", err)
	}

	orgService := fabricservice.NewOrganizationService(queries, keyService, configService)
	log := logger.NewDefault()

	return &testEnv{
		db:            queries,
		sqlDB:         sqlDB,
		keyService:    keyService,
		orgService:    orgService,
		configService: configService,
		logger:        log,
		dataDir:       dataDir,
		cleanup: func() {
			sqlDB.Close()
			os.RemoveAll(dataDir)
		},
	}
}

// createTestOrganization creates a test organization with all required keys
func (env *testEnv) createTestOrganization(t *testing.T, mspID string) *fabricservice.OrganizationDTO {
	t.Helper()
	ctx := context.Background()

	org, err := env.orgService.CreateOrganization(ctx, fabricservice.CreateOrganizationParams{
		MspID:       mspID,
		Name:        mspID,
		Description: "Test organization for FabricX integration tests",
	})
	if err != nil {
		t.Fatalf("failed to create organization %s: %v", mspID, err)
	}
	return org
}

// TestOrdererGroupInit tests that Init() generates certificates and config files
func TestOrdererGroupInit(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	org := env.createTestOrganization(t, "Org1MSP")

	// Allocate ports
	ports, err := service.GetFabricXOrdererGroupPorts(17150)
	if err != nil {
		t.Fatalf("failed to allocate ports: %v", err)
	}

	og := NewOrdererGroup(
		env.db,
		env.orgService,
		env.keyService,
		env.configService,
		env.logger,
		0, // nodeID not needed for Init
		nodetypes.FabricXOrdererGroupConfig{
			BaseNodeConfig: nodetypes.BaseNodeConfig{Type: "fabricx-orderer-group", Mode: "docker"},
			Name:           "test-orderer-party1",
			OrganizationID: org.ID,
			MSPID:          org.MspID,
			PartyID:        1,
			RouterPort:     ports.Router,
			BatcherPort:    ports.Batcher,
			ConsenterPort:  ports.Consenter,
			AssemblerPort:  ports.Assembler,
		},
	)

	cfg, err := og.Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Verify deployment config
	if cfg.MSPID != "Org1MSP" {
		t.Errorf("expected MSPID Org1MSP, got %s", cfg.MSPID)
	}
	if cfg.PartyID != 1 {
		t.Errorf("expected PartyID 1, got %d", cfg.PartyID)
	}
	if cfg.SignCert == "" {
		t.Error("expected SignCert to be non-empty")
	}
	if cfg.TLSCert == "" {
		t.Error("expected TLSCert to be non-empty")
	}
	if cfg.CACert == "" {
		t.Error("expected CACert to be non-empty")
	}
	if cfg.RouterContainer == "" {
		t.Error("expected RouterContainer to be non-empty")
	}

	// Verify config files were written
	baseDir := og.baseDir()
	for _, comp := range []string{"router", "batcher", "consenter", "assembler"} {
		configPath := filepath.Join(baseDir, comp, "config", "node_config.yaml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Errorf("expected config file at %s", configPath)
		}

		mspPath := filepath.Join(baseDir, comp, "msp", "signcerts", "cert.pem")
		if _, err := os.Stat(mspPath); os.IsNotExist(err) {
			t.Errorf("expected MSP cert at %s", mspPath)
		}

		tlsPath := filepath.Join(baseDir, comp, "tls", "server.crt")
		if _, err := os.Stat(tlsPath); os.IsNotExist(err) {
			t.Errorf("expected TLS cert at %s", tlsPath)
		}
	}

	t.Logf("OrdererGroup Init() succeeded: containers=%s,%s,%s,%s",
		cfg.RouterContainer, cfg.BatcherContainer, cfg.ConsenterContainer, cfg.AssemblerContainer)
}

// TestCommitterInit tests that Committer Init() generates certificates and config files
func TestCommitterInit(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	org := env.createTestOrganization(t, "Org1MSP")

	ports, err := service.GetFabricXCommitterPorts(18050)
	if err != nil {
		t.Fatalf("failed to allocate ports: %v", err)
	}

	cm := NewCommitter(
		env.db,
		env.orgService,
		env.keyService,
		env.configService,
		env.logger,
		0,
		nodetypes.FabricXCommitterConfig{
			BaseNodeConfig: nodetypes.BaseNodeConfig{Type: "fabricx-committer", Mode: "docker"},
			Name:           "test-committer1",
			OrganizationID: org.ID,
			MSPID:          org.MspID,
			SidecarPort:    ports.Sidecar,
			CoordinatorPort: ports.Coordinator,
			ValidatorPort:  ports.Validator,
			VerifierPort:   ports.Verifier,
			QueryServicePort: ports.QueryService,
			OrdererEndpoints: []string{"localhost:7050"},
			PostgresHost:   "localhost",
			PostgresPort:   5432,
			PostgresDB:     "fabricx_test",
			PostgresUser:   "fabricx",
			PostgresPassword: "fabricx",
		},
	)

	cfg, err := cm.Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	if cfg.MSPID != "Org1MSP" {
		t.Errorf("expected MSPID Org1MSP, got %s", cfg.MSPID)
	}
	if cfg.SignCert == "" {
		t.Error("expected SignCert to be non-empty")
	}
	if cfg.SidecarContainer == "" {
		t.Error("expected SidecarContainer to be non-empty")
	}

	// Verify config files were written
	baseDir := cm.baseDir()
	configFiles := map[string]string{
		"sidecar/config/sidecar_config.yaml":         "sidecar config",
		"coordinator/config/coordinator_config.yaml":  "coordinator config",
		"validator/config/validator_config.yaml":      "validator config",
		"verifier/config/verifier_config.yaml":        "verifier config",
		"query-service/config/config.yaml":            "query-service config",
		"sidecar/msp/signcerts/cert.pem":              "sidecar MSP cert",
		"sidecar/tls/server.crt":                      "sidecar TLS cert",
	}

	for path, desc := range configFiles {
		fullPath := filepath.Join(baseDir, path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("expected %s at %s", desc, fullPath)
		}
	}

	t.Logf("Committer Init() succeeded: containers=%s,%s,%s,%s,%s",
		cfg.SidecarContainer, cfg.CoordinatorContainer, cfg.ValidatorContainer,
		cfg.VerifierContainer, cfg.QueryServiceContainer)
}

// TestOrdererGroupDockerStart tests that orderer group containers can be created and started.
// This requires Docker to be running.
func TestOrdererGroupDockerStart(t *testing.T) {
	if os.Getenv("FABRICX_DOCKER_TEST") == "" {
		t.Skip("Set FABRICX_DOCKER_TEST=1 to run Docker-based tests")
	}

	env := setupTestEnv(t)
	defer env.cleanup()

	org := env.createTestOrganization(t, "TestOrd1MSP")

	ports, err := service.GetFabricXOrdererGroupPorts(19150)
	if err != nil {
		t.Fatalf("failed to allocate ports: %v", err)
	}

	og := NewOrdererGroup(
		env.db,
		env.orgService,
		env.keyService,
		env.configService,
		env.logger,
		0,
		nodetypes.FabricXOrdererGroupConfig{
			BaseNodeConfig: nodetypes.BaseNodeConfig{Type: "fabricx-orderer-group", Mode: "docker"},
			Name:           "docker-test-orderer",
			OrganizationID: org.ID,
			MSPID:          org.MspID,
			PartyID:        1,
			RouterPort:     ports.Router,
			BatcherPort:    ports.Batcher,
			ConsenterPort:  ports.Consenter,
			AssemblerPort:  ports.Assembler,
		},
	)

	cfg, err := og.Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Generate a real genesis block with Arma consensus
	genesisBlock, err := GenerateGenesisBlock(GenesisConfig{
		ChannelID: "testchannel",
		Parties: []GenesisParty{
			{
				PartyID:    cfg.PartyID,
				MSPID:      cfg.MSPID,
				SignCACert: cfg.CACert,
				TLSCACert:  cfg.TLSCACert,

				RouterHost:    "localhost",
				RouterPort:    cfg.RouterPort,
				RouterTLSCert: cfg.TLSCert,

				BatcherHost:     "localhost",
				BatcherPort:     cfg.BatcherPort,
				BatcherSignCert: cfg.SignCert,
				BatcherTLSCert:  cfg.TLSCert,

				ConsenterHost:     "localhost",
				ConsenterPort:     cfg.ConsenterPort,
				ConsenterSignCert: cfg.SignCert,
				ConsenterTLSCert:  cfg.TLSCert,

				AssemblerHost:    "localhost",
				AssemblerPort:    cfg.AssemblerPort,
				AssemblerTLSCert: cfg.TLSCert,

				IdentityCert: cfg.SignCert,
			},
		},
	})
	if err != nil {
		t.Fatalf("GenerateGenesisBlock() failed: %v", err)
	}

	if err := og.SetGenesisBlock(genesisBlock); err != nil {
		t.Fatalf("SetGenesisBlock() failed: %v", err)
	}

	// Start containers
	err = og.Start(cfg)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Cleanup: stop containers
	defer func() {
		if err := og.Stop(cfg); err != nil {
			t.Logf("Warning: Stop() failed: %v", err)
		}
	}()

	// Check containers are initially running (they may crash later due to dummy genesis)
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
			t.Errorf("failed to check container %s: %v", name, err)
			continue
		}
		if !running {
			t.Errorf("container %s is not running", name)
		} else {
			t.Logf("container %s is running", name)
		}
	}
}

// TestCommitterDockerStart tests that committer containers can be created and started.
// This requires Docker to be running.
func TestCommitterDockerStart(t *testing.T) {
	if os.Getenv("FABRICX_DOCKER_TEST") == "" {
		t.Skip("Set FABRICX_DOCKER_TEST=1 to run Docker-based tests")
	}

	env := setupTestEnv(t)
	defer env.cleanup()

	org := env.createTestOrganization(t, "TestCom1MSP")

	ports, err := service.GetFabricXCommitterPorts(19050)
	if err != nil {
		t.Fatalf("failed to allocate ports: %v", err)
	}

	cm := NewCommitter(
		env.db,
		env.orgService,
		env.keyService,
		env.configService,
		env.logger,
		0,
		nodetypes.FabricXCommitterConfig{
			BaseNodeConfig: nodetypes.BaseNodeConfig{Type: "fabricx-committer", Mode: "docker"},
			Name:           "docker-test-committer",
			OrganizationID: org.ID,
			MSPID:          org.MspID,
			SidecarPort:    ports.Sidecar,
			CoordinatorPort: ports.Coordinator,
			ValidatorPort:  ports.Validator,
			VerifierPort:   ports.Verifier,
			QueryServicePort: ports.QueryService,
			OrdererEndpoints: []string{"localhost:7050"},
			PostgresHost:   "localhost",
			PostgresPort:   15432, // use a non-standard port to avoid conflicts
			PostgresDB:     "fabricx_test",
			PostgresUser:   "fabricx",
			PostgresPassword: "fabricx",
		},
	)

	cfg, err := cm.Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Generate a real genesis block for the sidecar
	genesisBlock, err := GenerateGenesisBlock(GenesisConfig{
		ChannelID: "testchannel",
		Parties: []GenesisParty{
			{
				PartyID:    1,
				MSPID:      cfg.MSPID,
				SignCACert: cfg.CACert,
				TLSCACert:  cfg.TLSCACert,

				RouterHost:    "localhost",
				RouterPort:    7050,
				RouterTLSCert: cfg.TLSCert,

				BatcherHost:     "localhost",
				BatcherPort:     7051,
				BatcherSignCert: cfg.SignCert,
				BatcherTLSCert:  cfg.TLSCert,

				ConsenterHost:     "localhost",
				ConsenterPort:     7052,
				ConsenterSignCert: cfg.SignCert,
				ConsenterTLSCert:  cfg.TLSCert,

				AssemblerHost:    "localhost",
				AssemblerPort:    7053,
				AssemblerTLSCert: cfg.TLSCert,

				IdentityCert: cfg.SignCert,
			},
		},
	})
	if err != nil {
		t.Fatalf("GenerateGenesisBlock() failed: %v", err)
	}
	if err := cm.SetGenesisBlock(genesisBlock); err != nil {
		t.Fatalf("SetGenesisBlock() failed: %v", err)
	}

	// Start containers (including postgres)
	err = cm.Start(cfg, true) // startPostgres=true
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	defer func() {
		if err := cm.Stop(cfg); err != nil {
			t.Logf("Warning: Stop() failed: %v", err)
		}
	}()

	// Give containers a moment to stabilize
	time.Sleep(2 * time.Second)

	// Check postgres container
	ctx := context.Background()
	running, err := isContainerRunning(ctx, cfg.PostgresContainer)
	if err != nil {
		t.Errorf("failed to check postgres container: %v", err)
	} else if !running {
		t.Errorf("postgres container %s is not running", cfg.PostgresContainer)
	} else {
		t.Logf("postgres container %s is running", cfg.PostgresContainer)
	}

	// Check committer containers
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
			t.Errorf("failed to check container %s: %v", name, err)
			continue
		}
		if !running {
			t.Errorf("container %s is not running", name)
		} else {
			t.Logf("container %s is running", name)
		}
	}
}

// TestFullNetworkInit tests creating both orderer group and committer for the same org
func TestFullNetworkInit(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	org := env.createTestOrganization(t, "FullNetMSP")

	// Create orderer group
	ogPorts, err := service.GetFabricXOrdererGroupPorts(20150)
	if err != nil {
		t.Fatalf("failed to allocate orderer ports: %v", err)
	}

	og := NewOrdererGroup(
		env.db, env.orgService, env.keyService, env.configService, env.logger, 0,
		nodetypes.FabricXOrdererGroupConfig{
			BaseNodeConfig: nodetypes.BaseNodeConfig{Type: "fabricx-orderer-group", Mode: "docker"},
			Name:           "fullnet-orderer-party1",
			OrganizationID: org.ID,
			MSPID:          org.MspID,
			PartyID:        1,
			RouterPort:     ogPorts.Router,
			BatcherPort:    ogPorts.Batcher,
			ConsenterPort:  ogPorts.Consenter,
			AssemblerPort:  ogPorts.Assembler,
		},
	)

	ogCfg, err := og.Init()
	if err != nil {
		t.Fatalf("OrdererGroup Init() failed: %v", err)
	}

	// Create committer pointing to the orderer assembler
	cmPorts, err := service.GetFabricXCommitterPorts(20050)
	if err != nil {
		t.Fatalf("failed to allocate committer ports: %v", err)
	}

	assemblerEndpoint := fmt.Sprintf("localhost:%d", ogCfg.AssemblerPort)
	cm := NewCommitter(
		env.db, env.orgService, env.keyService, env.configService, env.logger, 0,
		nodetypes.FabricXCommitterConfig{
			BaseNodeConfig: nodetypes.BaseNodeConfig{Type: "fabricx-committer", Mode: "docker"},
			Name:           "fullnet-committer1",
			OrganizationID: org.ID,
			MSPID:          org.MspID,
			SidecarPort:    cmPorts.Sidecar,
			CoordinatorPort: cmPorts.Coordinator,
			ValidatorPort:  cmPorts.Validator,
			VerifierPort:   cmPorts.Verifier,
			QueryServicePort: cmPorts.QueryService,
			OrdererEndpoints: []string{assemblerEndpoint},
			PostgresHost:   "localhost",
			PostgresPort:   15433,
			PostgresDB:     "fabricx_fullnet",
			PostgresUser:   "fabricx",
			PostgresPassword: "fabricx",
		},
	)

	cmCfg, err := cm.Init()
	if err != nil {
		t.Fatalf("Committer Init() failed: %v", err)
	}

	// Verify both configs are valid
	if ogCfg.OrganizationID != cmCfg.OrganizationID {
		t.Error("expected same organization ID for orderer and committer")
	}
	if ogCfg.MSPID != cmCfg.MSPID {
		t.Error("expected same MSPID for orderer and committer")
	}

	// Verify the committer's orderer endpoint matches the assembler
	if len(cmCfg.OrdererEndpoints) != 1 || cmCfg.OrdererEndpoints[0] != assemblerEndpoint {
		t.Errorf("expected orderer endpoint %s, got %v", assemblerEndpoint, cmCfg.OrdererEndpoints)
	}

	t.Logf("Full network init succeeded:")
	t.Logf("  OrdererGroup: router=%d, batcher=%d, consenter=%d, assembler=%d",
		ogCfg.RouterPort, ogCfg.BatcherPort, ogCfg.ConsenterPort, ogCfg.AssemblerPort)
	t.Logf("  Committer: sidecar=%d, coordinator=%d, validator=%d, verifier=%d, query=%d",
		cmCfg.SidecarPort, cmCfg.CoordinatorPort, cmCfg.ValidatorPort,
		cmCfg.VerifierPort, cmCfg.QueryServicePort)
}

// TestPortAllocation verifies port allocation functions work correctly
func TestPortAllocation(t *testing.T) {
	// OrdererGroup ports
	ogPorts, err := service.GetFabricXOrdererGroupPorts(21150)
	if err != nil {
		t.Fatalf("GetFabricXOrdererGroupPorts failed: %v", err)
	}
	if ogPorts.Router == 0 || ogPorts.Batcher == 0 || ogPorts.Consenter == 0 || ogPorts.Assembler == 0 {
		t.Error("expected all orderer group ports to be non-zero")
	}
	if ogPorts.Batcher != ogPorts.Router+1 || ogPorts.Consenter != ogPorts.Router+2 || ogPorts.Assembler != ogPorts.Router+3 {
		t.Error("expected consecutive port allocation")
	}
	t.Logf("OrdererGroup ports: %d, %d, %d, %d", ogPorts.Router, ogPorts.Batcher, ogPorts.Consenter, ogPorts.Assembler)

	// Committer ports
	cmPorts, err := service.GetFabricXCommitterPorts(21050)
	if err != nil {
		t.Fatalf("GetFabricXCommitterPorts failed: %v", err)
	}
	if cmPorts.Sidecar == 0 || cmPorts.Coordinator == 0 || cmPorts.Validator == 0 || cmPorts.Verifier == 0 || cmPorts.QueryService == 0 {
		t.Error("expected all committer ports to be non-zero")
	}
	t.Logf("Committer ports: %d, %d, %d, %d, %d", cmPorts.Sidecar, cmPorts.Coordinator, cmPorts.Validator, cmPorts.Verifier, cmPorts.QueryService)
}

// TestFourPartyNetwork creates 4 orderer group parties (simulating 4 independent machines),
// generates a shared genesis block, starts all parties, and verifies the network forms.
func TestFourPartyNetwork(t *testing.T) {
	if os.Getenv("FABRICX_DOCKER_TEST") == "" {
		t.Skip("Set FABRICX_DOCKER_TEST=1 to run Docker-based tests")
	}

	env := setupTestEnv(t)
	defer env.cleanup()

	// On macOS Docker Desktop, containers reach the host via host.docker.internal.
	// On Linux, use 172.17.0.1 (docker0 bridge) or host networking.
	externalHost := "host.docker.internal"

	numParties := 4
	basePort := 22000 // each party gets 4 consecutive ports starting from basePort + i*10

	type partyInfo struct {
		org *fabricservice.OrganizationDTO
		og  *OrdererGroup
		cfg *nodetypes.FabricXOrdererGroupDeploymentConfig
	}

	parties := make([]partyInfo, numParties)

	// Step 1: Create orgs and init all orderer groups (generates certs + configs)
	for i := 0; i < numParties; i++ {
		mspID := fmt.Sprintf("Party%dMSP", i+1)
		org := env.createTestOrganization(t, mspID)

		portBase := basePort + i*10
		ports, err := service.GetFabricXOrdererGroupPorts(portBase)
		if err != nil {
			t.Fatalf("party %d: failed to allocate ports: %v", i+1, err)
		}

		og := NewOrdererGroup(env.db, env.orgService, env.keyService, env.configService, env.logger, 0,
			nodetypes.FabricXOrdererGroupConfig{
				BaseNodeConfig: nodetypes.BaseNodeConfig{Type: "fabricx-orderer-group", Mode: "docker"},
				Name:           fmt.Sprintf("party%d-orderer", i+1),
				OrganizationID: org.ID,
				MSPID:          org.MspID,
				PartyID:        i + 1,
				ExternalIP:     externalHost,
				DomainNames:    []string{externalHost, "localhost"},
				RouterPort:     ports.Router,
				BatcherPort:    ports.Batcher,
				ConsenterPort:  ports.Consenter,
				AssemblerPort:  ports.Assembler,
			},
		)

		cfg, err := og.Init()
		if err != nil {
			t.Fatalf("party %d: Init() failed: %v", i+1, err)
		}

		parties[i] = partyInfo{org: org, og: og, cfg: cfg}
		t.Logf("Party %d (%s): router=%d, batcher=%d, consenter=%d, assembler=%d",
			i+1, mspID, cfg.RouterPort, cfg.BatcherPort, cfg.ConsenterPort, cfg.AssemblerPort)
	}

	// Step 2: Build genesis block with all 4 parties
	var genesisParties []GenesisParty
	for i, p := range parties {
		genesisParties = append(genesisParties, GenesisParty{
			PartyID:    i + 1,
			MSPID:      p.cfg.MSPID,
			SignCACert: p.cfg.CACert,
			TLSCACert:  p.cfg.TLSCACert,

			RouterHost:    externalHost,
			RouterPort:    p.cfg.RouterPort,
			RouterTLSCert: p.cfg.TLSCert,

			BatcherHost:     externalHost,
			BatcherPort:     p.cfg.BatcherPort,
			BatcherSignCert: p.cfg.SignCert,
			BatcherTLSCert:  p.cfg.TLSCert,

			ConsenterHost:     externalHost,
			ConsenterPort:     p.cfg.ConsenterPort,
			ConsenterSignCert: p.cfg.SignCert,
			ConsenterTLSCert:  p.cfg.TLSCert,

			AssemblerHost:    externalHost,
			AssemblerPort:    p.cfg.AssemblerPort,
			AssemblerTLSCert: p.cfg.TLSCert,

			IdentityCert: p.cfg.SignCert,
		})
	}

	genesisBlock, err := GenerateGenesisBlock(GenesisConfig{
		ChannelID: "testchannel",
		Parties:   genesisParties,
	})
	if err != nil {
		t.Fatalf("GenerateGenesisBlock() failed: %v", err)
	}
	t.Logf("Genesis block generated with %d parties (%d bytes)", numParties, len(genesisBlock))

	// Step 3: Set genesis block on all parties and start them
	for i, p := range parties {
		if err := p.og.SetGenesisBlock(genesisBlock); err != nil {
			t.Fatalf("party %d: SetGenesisBlock() failed: %v", i+1, err)
		}
		if err := p.og.Start(p.cfg); err != nil {
			t.Fatalf("party %d: Start() failed: %v", i+1, err)
		}
		t.Logf("Party %d started", i+1)
	}

	// Cleanup all parties on exit
	defer func() {
		for i, p := range parties {
			if err := p.og.Stop(p.cfg); err != nil {
				t.Logf("Warning: party %d Stop() failed: %v", i+1, err)
			}
		}
	}()

	// Step 4: Wait for network to stabilize and verify health
	t.Log("All 4 parties started. Waiting 20 seconds for network formation...")
	time.Sleep(20 * time.Second)

	ctx := context.Background()
	allHealthy := true
	totalContainers := 0

	for i, p := range parties {
		containers := map[string]string{
			"router":    p.cfg.RouterContainer,
			"batcher":   p.cfg.BatcherContainer,
			"consenter": p.cfg.ConsenterContainer,
			"assembler": p.cfg.AssemblerContainer,
		}
		for comp, name := range containers {
			totalContainers++
			running, err := isContainerRunning(ctx, name)
			if err != nil {
				t.Errorf("party %d %s (%s): error: %v", i+1, comp, name, err)
				allHealthy = false
			} else if !running {
				t.Errorf("party %d %s (%s): NOT RUNNING", i+1, comp, name)
				allHealthy = false
			} else {
				t.Logf("party %d %s (%s): HEALTHY", i+1, comp, name)
			}
		}
	}

	if allHealthy {
		t.Logf("ALL %d CONTAINERS HEALTHY across %d parties after 20 seconds", totalContainers, numParties)
	} else {
		t.Error("Some containers are not healthy")
	}

}
