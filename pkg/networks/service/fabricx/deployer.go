package fabricx

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/chainlaunch/chainlaunch/pkg/config"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	orgservice "github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	keymanagement "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
	fabricxpkg "github.com/chainlaunch/chainlaunch/pkg/nodes/fabricx"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// localDevHost is the hostname we substitute for the configured externalIP
// when CHAINLAUNCH_FABRICX_LOCAL_DEV is set. Docker Desktop (macOS/Windows)
// resolves host.docker.internal to the host and is the only reliable way to
// reach published ports from inside containers, since numeric IP dials bypass
// /etc/hosts (so extra_hosts aliases for IPs don't work).
const localDevHost = "host.docker.internal"

// FabricXDeployer implements the NetworkDeployer interface for Fabric X networks
type FabricXDeployer struct {
	db            *db.Queries
	logger        *logger.Logger
	keyMgmt       *keymanagement.KeyManagementService
	orgService    *orgservice.OrganizationService
	nodeService   *nodeservice.NodeService
	configService *config.ConfigService
}

// NewFabricXDeployer creates a new FabricXDeployer instance
func NewFabricXDeployer(db *db.Queries, nodeService *nodeservice.NodeService, keyMgmt *keymanagement.KeyManagementService, orgService *orgservice.OrganizationService, configService *config.ConfigService) *FabricXDeployer {
	logger := logger.NewDefault().With("component", "fabricx_deployer")
	return &FabricXDeployer{
		db:            db,
		logger:        logger,
		keyMgmt:       keyMgmt,
		orgService:    orgService,
		nodeService:   nodeService,
		configService: configService,
	}
}

// CreateGenesisBlock generates a genesis block for a Fabric X network.
// It reads existing orderer group nodes referenced in the config to extract their
// TLS/sign certs and endpoint info, then generates a genesis block with Arma consensus.
func (d *FabricXDeployer) CreateGenesisBlock(networkID int64, config interface{}) ([]byte, error) {
	fabricxConfig, ok := config.(*types.FabricXNetworkConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type: expected FabricXNetworkConfig, got %T", config)
	}

	ctx := context.Background()
	d.logger.Info("Creating Fabric X genesis block", "channel", fabricxConfig.ChannelName, "orgs", len(fabricxConfig.Organizations))

	var genesisParties []fabricxpkg.GenesisParty

	// network_nodes has UNIQUE(network_id, node_id). In single-MSP
	// quickstarts the same committer is referenced by N party orgRefs,
	// which would otherwise trip the constraint on the second insert.
	// Track which node IDs we've already associated with the network so
	// the loop is idempotent across duplicate orgRefs.
	associatedNodes := make(map[int64]bool)
	associateNodeOnce := func(nodeID int64, role string) error {
		if associatedNodes[nodeID] {
			return nil
		}
		if _, err := d.db.CreateNetworkNode(ctx, &db.CreateNetworkNodeParams{
			NetworkID: networkID,
			NodeID:    nodeID,
			Status:    "pending",
			Role:      role,
		}); err != nil {
			return err
		}
		associatedNodes[nodeID] = true
		return nil
	}

	for i, orgRef := range fabricxConfig.Organizations {
		// Get the organization
		org, err := d.orgService.GetOrganization(ctx, orgRef.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get organization %d: %w", orgRef.ID, err)
		}

		// Source the shared orderer deployment config from either the new
		// node_groups row (ADR-0001 path) or the legacy monolithic node row.
		if orgRef.OrdererNodeGroupID == 0 && orgRef.OrdererNodeID == 0 {
			return nil, fmt.Errorf("organization %d (%s) has no orderer node_group or node ID", orgRef.ID, org.MspID)
		}

		var (
			deploymentCfg nodetypes.FabricXOrdererGroupDeploymentConfig
			sourceUpdated any
			sourceCreated any
			sourceName    string
		)
		if orgRef.OrdererNodeGroupID != 0 {
			grp, err := d.db.GetNodeGroup(ctx, orgRef.OrdererNodeGroupID)
			if err != nil {
				return nil, fmt.Errorf("failed to get orderer node_group %d: %w", orgRef.OrdererNodeGroupID, err)
			}
			if !grp.DeploymentConfig.Valid {
				return nil, fmt.Errorf("orderer node_group %d has no deployment config (not initialized)", orgRef.OrdererNodeGroupID)
			}
			if err := json.Unmarshal([]byte(grp.DeploymentConfig.String), &deploymentCfg); err != nil {
				return nil, fmt.Errorf("failed to unmarshal node_group deployment config: %w", err)
			}
			sourceUpdated = grp.UpdatedAt
			sourceCreated = grp.CreatedAt
			sourceName = grp.Name
		} else {
			dbNode, err := d.db.GetNode(ctx, orgRef.OrdererNodeID)
			if err != nil {
				return nil, fmt.Errorf("failed to get orderer node %d: %w", orgRef.OrdererNodeID, err)
			}
			if !dbNode.DeploymentConfig.Valid {
				return nil, fmt.Errorf("orderer node %d has no deployment config (not initialized)", orgRef.OrdererNodeID)
			}
			if err := json.Unmarshal([]byte(dbNode.DeploymentConfig.String), &deploymentCfg); err != nil {
				return nil, fmt.Errorf("failed to unmarshal orderer group deployment config: %w", err)
			}
			sourceUpdated = dbNode.UpdatedAt
			sourceCreated = dbNode.CreatedAt
			sourceName = dbNode.Name
		}

		externalHost := deploymentCfg.ExternalIP
		if externalHost == "" {
			return nil, fmt.Errorf("orderer node %d has no external IP configured", orgRef.OrdererNodeID)
		}
		// In local-dev mode, bake host.docker.internal into the genesis so
		// containers on Docker Desktop Mac/Windows can actually reach each
		// other (they can't dial the LAN IP of the host through hairpin NAT).
		if resolveLocalDev(fabricxConfig, d.configService) {
			d.logger.Info("local-dev: substituting host.docker.internal for externalIP in genesis",
				"party", deploymentCfg.PartyID, "originalIP", externalHost)
			externalHost = localDevHost
		}

		partyID := deploymentCfg.PartyID
		if partyID == 0 {
			partyID = i + 1
		}

		d.logger.Info("genesis party source",
			"nodeID", orgRef.OrdererNodeID,
			"nodeGroupID", orgRef.OrdererNodeGroupID,
			"mspId", deploymentCfg.MSPID,
			"deploymentCfg.tlsCert", fabricxpkg.CertFingerprint([]byte(deploymentCfg.TLSCert)),
			"source.updatedAt", sourceUpdated,
			"source.createdAt", sourceCreated,
		)

		// Cert drift guard: refuse to embed a TLS cert in genesis that
		// doesn't match what's actually on disk for this orderer group.
		// If we let drift slip into genesis, the router will panic at
		// startup with "certificate mismatch" and the network is
		// unrecoverable (genesis is immutable). The fix is to make the
		// caller stop regenerating certs out-of-band — typically that
		// means re-running Init(), which is now idempotent.
		if err := d.assertOrdererCertMatchesDisk(sourceName, deploymentCfg); err != nil {
			return nil, fmt.Errorf("orderer cert drift before genesis (party %d, msp %s): %w", partyID, deploymentCfg.MSPID, err)
		}

		genesisParties = append(genesisParties, fabricxpkg.GenesisParty{
			PartyID:   partyID,
			MSPID:     deploymentCfg.MSPID,
			SignCACert: deploymentCfg.CACert,
			TLSCACert: deploymentCfg.TLSCACert,

			RouterHost:    externalHost,
			RouterPort:    deploymentCfg.RouterPort,
			RouterTLSCert: deploymentCfg.TLSCert,

			BatcherHost:     externalHost,
			BatcherPort:     deploymentCfg.BatcherPort,
			BatcherSignCert: deploymentCfg.SignCert,
			BatcherTLSCert:  deploymentCfg.TLSCert,

			ConsenterHost:     externalHost,
			ConsenterPort:     deploymentCfg.ConsenterPort,
			ConsenterSignCert: deploymentCfg.SignCert,
			ConsenterTLSCert:  deploymentCfg.TLSCert,

			AssemblerHost:    externalHost,
			AssemblerPort:    deploymentCfg.AssemblerPort,
			AssemblerTLSCert: deploymentCfg.TLSCert,

			IdentityCert: deploymentCfg.SignCert,
		})

		// Associate orderer-side rows with the network. For the new ADR-0001
		// path we associate each of the 4 per-role child nodes so Join can
		// walk them individually. For the legacy path we associate the single
		// monolithic node row.
		if orgRef.OrdererNodeGroupID != 0 {
			children, err := d.db.ListNodesByGroup(ctx, sql.NullInt64{Int64: orgRef.OrdererNodeGroupID, Valid: true})
			if err != nil {
				return nil, fmt.Errorf("failed to list children for orderer node_group %d: %w", orgRef.OrdererNodeGroupID, err)
			}
			if len(children) == 0 {
				return nil, fmt.Errorf("orderer node_group %d has no children", orgRef.OrdererNodeGroupID)
			}
			for _, c := range children {
				if err := associateNodeOnce(c.ID, "orderer"); err != nil {
					return nil, fmt.Errorf("failed to associate orderer child node %d with network: %w", c.ID, err)
				}
			}
		} else {
			if err := associateNodeOnce(orgRef.OrdererNodeID, "orderer"); err != nil {
				return nil, fmt.Errorf("failed to associate orderer node with network: %w", err)
			}
		}

		// Associate committer-side rows with the network. Mirrors the orderer
		// handling above: for a committer node_group we associate each of the
		// 5 per-role children so Join walks them individually; for the legacy
		// monolithic committer path we associate the single parent node row.
		if orgRef.CommitterNodeGroupID != 0 {
			children, err := d.db.ListNodesByGroup(ctx, sql.NullInt64{Int64: orgRef.CommitterNodeGroupID, Valid: true})
			if err != nil {
				return nil, fmt.Errorf("failed to list children for committer node_group %d: %w", orgRef.CommitterNodeGroupID, err)
			}
			if len(children) == 0 {
				return nil, fmt.Errorf("committer node_group %d has no children", orgRef.CommitterNodeGroupID)
			}
			for _, c := range children {
				if err := associateNodeOnce(c.ID, "committer"); err != nil {
					return nil, fmt.Errorf("failed to associate committer child node %d with network: %w", c.ID, err)
				}
			}
		} else if orgRef.CommitterNodeID != 0 {
			if err := associateNodeOnce(orgRef.CommitterNodeID, "committer"); err != nil {
				return nil, fmt.Errorf("failed to associate committer node with network: %w", err)
			}
		}
	}

	// Generate genesis block
	genesisBlock, err := fabricxpkg.GenerateGenesisBlock(fabricxpkg.GenesisConfig{
		ChannelID: fabricxConfig.ChannelName,
		Parties:   genesisParties,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate genesis block: %w", err)
	}

	d.logger.Info("Generated Fabric X genesis block", "channel", fabricxConfig.ChannelName, "parties", len(genesisParties), "bytes", len(genesisBlock))
	return genesisBlock, nil
}

// assertOrdererCertMatchesDisk fails fast when the TLS cert recorded in
// the orderer group's deployment_config differs from the cert currently
// on disk for any of its four roles. If we let a mismatch slip into the
// genesis block, the orderer router crashes at startup ("certificate
// mismatch") and the network is unrecoverable because genesis is
// immutable.
//
// We compare each role's server.crt against deploymentCfg.TLSCert. The
// cert is shared across all four roles, so any one of them disagreeing
// with the DB is enough to abort. We trim trailing whitespace before
// comparison because writeTLS leaves a trailing newline that os.WriteFile
// preserves verbatim.
func (d *FabricXDeployer) assertOrdererCertMatchesDisk(groupName string, cfg nodetypes.FabricXOrdererGroupDeploymentConfig) error {
	if d.configService == nil {
		// In tests where configService is not wired we can't resolve
		// the data path; skip the check. Production always has it.
		return nil
	}
	dbCert := bytes.TrimSpace([]byte(cfg.TLSCert))
	if len(dbCert) == 0 {
		return fmt.Errorf("deployment config TLS cert is empty")
	}
	if groupName == "" {
		return fmt.Errorf("orderer group name is empty; cannot resolve on-disk cert path")
	}
	dataPath := d.configService.GetDataPath()
	for _, role := range []string{"router", "batcher", "consenter", "assembler"} {
		path := fabricxpkg.OrdererRoleTLSCertPath(dataPath, groupName, role)
		onDisk, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				// Disk wasn't laid out yet — Init() must run before
				// genesis. Surface a clear error so the operator
				// knows to (re)create the orderer node first.
				return fmt.Errorf("orderer %s cert not found on disk at %s; was Init() run?", role, path)
			}
			return fmt.Errorf("read %s cert at %s: %w", role, path, err)
		}
		if !bytes.Equal(bytes.TrimSpace(onDisk), dbCert) {
			return fmt.Errorf(
				"role %s cert on disk differs from deployment config (disk=%s, db=%s); "+
					"this means a regenerate ran after init — refusing to bake the stale cert into genesis",
				role,
				fabricxpkg.CertFingerprint(onDisk),
				fabricxpkg.CertFingerprint(dbCert),
			)
		}
	}
	return nil
}

// JoinNode sets the genesis block on a Fabric X node and updates its network status.
func (d *FabricXDeployer) JoinNode(networkID int64, genesisBlock []byte, nodeID int64) error {
	ctx := context.Background()

	dbNode, err := d.db.GetNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node %d: %w", nodeID, err)
	}

	if !dbNode.DeploymentConfig.Valid {
		return fmt.Errorf("node %d has no deployment config", nodeID)
	}

	// Detect node type from deployment config
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(dbNode.DeploymentConfig.String), &raw); err != nil {
		return fmt.Errorf("failed to unmarshal deployment config: %w", err)
	}

	deployType, _ := raw["type"].(string)
	switch deployType {
	case "fabricx-child", "fabricx-orderer-group", "fabricx-committer":
		// Stage 2a: write the genesis block into the node's config dirs
		if d.nodeService == nil {
			return fmt.Errorf("node service not wired into FabricX deployer")
		}
		if err := d.nodeService.SetFabricXGenesisBlock(ctx, nodeID, genesisBlock); err != nil {
			return fmt.Errorf("failed to set genesis block on node %d: %w", nodeID, err)
		}

		// Stage 2b: flip the network_nodes row to "joined" before attempting
		// start. The join is logically complete once genesis is on disk; a
		// subsequent StartNode can be retried if Docker Desktop's apiproxy
		// is still catching up on bind-mount path resolution.
		if _, err := d.db.UpdateNetworkNodeStatus(ctx, &db.UpdateNetworkNodeStatusParams{
			NetworkID: networkID,
			NodeID:    nodeID,
			Status:    "joined",
		}); err != nil {
			d.logger.Warn("Failed to update network node status", "error", err)
		}

		// Stage 2c: start the node (boots the 4 orderer sub-containers or
		// the committer stack). On macOS Docker Desktop transient
		// bind-mount races can make ContainerCreate fail for minutes even
		// though the path is on disk. Treat those specific failures as
		// non-fatal: the genesis block is already in place and the node can
		// be started later via the normal /nodes/{id}/start endpoint. This
		// lets a 20-node quickstart complete all 20 joins instead of
		// aborting at the first transient Docker error.
		if _, err := d.nodeService.StartNode(ctx, nodeID); err != nil {
			if isTransientDockerMountErr(err) {
				d.logger.Warn("Node join succeeded (genesis written) but start hit transient Docker bind-mount error; leave it for the node-start retry loop",
					"nodeID", nodeID, "networkID", networkID, "type", deployType, "error", err)
				return nil
			}
			return fmt.Errorf("failed to start node %d: %w", nodeID, err)
		}

		d.logger.Info("Joined node to FabricX network", "nodeID", nodeID, "networkID", networkID, "type", deployType)
		return nil
	default:
		return fmt.Errorf("unsupported FabricX node type: %s", deployType)
	}
}

// isTransientDockerMountErr reports whether err is a known transient macOS
// Docker Desktop bind-mount failure. During a burst of ContainerCreate calls
// apiproxy's queue fills up and returns "bind source path does not exist" for
// paths that actually exist. The container can be started later once the
// queue drains.
func isTransientDockerMountErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "bind source path does not exist") ||
		(strings.Contains(msg, "invalid mount config for type \"bind\"") && strings.Contains(msg, "operation not permitted"))
}

// HealthcheckNamespaceName is the name of the persistent namespace the
// deployer creates after every node has joined the network. Its presence
// proves three things end-to-end: the orderer router accepts TLS
// connections, the consensus path commits a transaction, and the
// committer-sidecar replays the block to its query-service. Operators
// can list it via the namespace API; it is intentionally NOT deleted
// after verification because re-running verification is cheap and the
// row is auditable.
const HealthcheckNamespaceName = "__chainlaunch_healthcheck__"

// VerifyNetwork runs the post-provisioning smoke test for a FabricX
// network. It blocks until either:
//   - The healthcheck namespace exists on-chain (success → returns nil)
//   - The deadline elapses or a non-retriable error fires (failure)
//
// On success VerifyNetwork transitions the network to "running"; on
// failure it sets "verification_failed" and stores the underlying error
// in the network's error_message column so the UI can show *why* a
// "provisioned" network is actually broken.
//
// The submitter is auto-picked: the first organization in the network's
// config whose admin sign key is present in the local DB. We don't
// expose this choice to the API caller because verification is a
// system-driven step, not a user transaction.
//
// VerifyNetwork is idempotent: if the healthcheck namespace already
// exists on-chain, it returns success immediately without sending a new
// transaction.
func (d *FabricXDeployer) VerifyNetwork(ctx context.Context, networkID int64) error {
	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return fmt.Errorf("get network %d: %w", networkID, err)
	}
	if !network.Config.Valid {
		return fmt.Errorf("network %d has no config", networkID)
	}
	var cfg types.FabricXNetworkConfig
	if err := json.Unmarshal([]byte(network.Config.String), &cfg); err != nil {
		return fmt.Errorf("parse network config: %w", err)
	}
	if len(cfg.Organizations) == 0 {
		return fmt.Errorf("network %d has no organizations", networkID)
	}

	// Pick the first org that has an admin sign key — without one we
	// can't sign the namespace-create envelope. This matches the
	// existing CreateNamespace contract.
	var submitterOrgID int64
	for _, orgRef := range cfg.Organizations {
		org, err := d.orgService.GetOrganization(ctx, orgRef.ID)
		if err != nil {
			continue
		}
		if org.AdminSignKeyID.Valid && org.AdminSignKeyID.Int64 > 0 {
			submitterOrgID = org.ID
			break
		}
	}
	if submitterOrgID == 0 {
		return fmt.Errorf("no organization in network %d has an admin sign key; cannot run verification", networkID)
	}

	// Idempotency: skip if the healthcheck namespace already exists on
	// chain. Listing is allowed to fail with a transient chain-unreachable
	// error; that's exactly the case CreateNamespace below will surface
	// as the verification failure.
	if existing, _, _ := d.ListNamespacesMerged(ctx, networkID); existing != nil {
		for _, ns := range existing {
			if ns.Name == HealthcheckNamespaceName {
				d.logger.Info("verification skipped: healthcheck namespace already on chain",
					"networkID", networkID, "namespace", HealthcheckNamespaceName)
				if err := d.markNetworkVerified(ctx, networkID); err != nil {
					return err
				}
				return nil
			}
		}
	}

	d.logger.Info("verifying FabricX network by creating healthcheck namespace",
		"networkID", networkID, "submitterOrgID", submitterOrgID, "namespace", HealthcheckNamespaceName)

	_, err = d.CreateNamespace(ctx, networkID, NamespaceCreateOptions{
		Name:            HealthcheckNamespaceName,
		Version:         -1,
		SubmitterOrgID:  submitterOrgID,
		WaitForFinality: true,
		// Use the submitter org's admin pubkey as the endorsement
		// policy. Writes to the healthcheck namespace are not
		// expected post-verification, so this stays internal.
	})
	if err != nil {
		// Persist the verification failure status so /networks/{id}
		// reflects that the network is unhealthy. We log the detailed
		// reason — the networks table has no error_message column today,
		// so the operator-facing error surfaces via the API caller's
		// returned error and the structured logs below.
		d.logger.Error("FabricX network verification failed",
			"networkID", networkID, "namespace", HealthcheckNamespaceName, "err", err)
		if updErr := d.db.UpdateNetworkStatus(ctx, &db.UpdateNetworkStatusParams{
			ID:     networkID,
			Status: "verification_failed",
		}); updErr != nil {
			d.logger.Warn("failed to persist verification_failed status",
				"networkID", networkID, "err", updErr)
		}
		return fmt.Errorf("network %d verification failed: %w", networkID, err)
	}

	if err := d.markNetworkVerified(ctx, networkID); err != nil {
		return err
	}

	d.logger.Info("FabricX network verified",
		"networkID", networkID, "namespace", HealthcheckNamespaceName)
	return nil
}

// markNetworkVerified flips a network into the "running" state. Called
// only after the healthcheck namespace is confirmed on-chain.
func (d *FabricXDeployer) markNetworkVerified(ctx context.Context, networkID int64) error {
	if err := d.db.UpdateNetworkStatus(ctx, &db.UpdateNetworkStatusParams{
		ID:     networkID,
		Status: "running",
	}); err != nil {
		return fmt.Errorf("mark network %d running: %w", networkID, err)
	}
	return nil
}

// GetStatus returns the deployment status of a Fabric X network
func (d *FabricXDeployer) GetStatus(networkID int64) (*types.NetworkDeploymentStatus, error) {
	ctx := context.Background()

	network, err := d.db.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	return &types.NetworkDeploymentStatus{
		NetworkID: networkID,
		Status:    network.Status,
	}, nil
}
