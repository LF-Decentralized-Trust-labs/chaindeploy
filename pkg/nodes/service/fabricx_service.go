package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/fabricx"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// nullStringValid is a small wrapper that returns an empty/invalid
// NullString for "" so renewal calls don't accidentally clear a
// previously-set DB column with a blank string.
func nullStringValid(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// fabricxMetricsURL formats the host-side Prometheus /metrics URL for a
// FabricX role. Returns "" when externalIP or port are unset, so callers
// can leave the field blank rather than emit a broken http://:0/metrics.
func fabricxMetricsURL(externalIP string, port int) string {
	if externalIP == "" || port <= 0 {
		return ""
	}
	return fmt.Sprintf("http://%s:%d/metrics", externalIP, port)
}

// fillOrdererMetricsUrls renders the per-role metrics URLs on an
// orderer-group properties block from its externalIP + monitoring ports.
// Idempotent and safe to call when monitoring ports are zero (legacy
// nodes pre-monitoring); it leaves the URL fields empty in that case.
func fillOrdererMetricsUrls(p *FabricXOrdererGroupProperties) {
	if p == nil || p.ExternalIP == "" {
		return
	}
	p.RouterMetricsUrl = fabricxMetricsURL(p.ExternalIP, p.RouterMonitoringPort)
	p.BatcherMetricsUrl = fabricxMetricsURL(p.ExternalIP, p.BatcherMonitoringPort)
	p.ConsenterMetricsUrl = fabricxMetricsURL(p.ExternalIP, p.ConsenterMonitoringPort)
	p.AssemblerMetricsUrl = fabricxMetricsURL(p.ExternalIP, p.AssemblerMonitoringPort)
}

// fillCommitterMetricsUrls is the committer counterpart of
// fillOrdererMetricsUrls.
func fillCommitterMetricsUrls(p *FabricXCommitterProperties) {
	if p == nil || p.ExternalIP == "" {
		return
	}
	p.SidecarMetricsUrl = fabricxMetricsURL(p.ExternalIP, p.SidecarMonitoringPort)
	p.CoordinatorMetricsUrl = fabricxMetricsURL(p.ExternalIP, p.CoordinatorMonitoringPort)
	p.ValidatorMetricsUrl = fabricxMetricsURL(p.ExternalIP, p.ValidatorMonitoringPort)
	p.VerifierMetricsUrl = fabricxMetricsURL(p.ExternalIP, p.VerifierMonitoringPort)
	p.QueryServiceMetricsUrl = fabricxMetricsURL(p.ExternalIP, p.QueryServiceMonitoringPort)
}

// unmarshalStoredNodeConfig unwraps the StoredConfig envelope ({type, config})
// and decodes the inner config into out. Falls back to direct unmarshal for
// legacy rows that were written without the envelope.
func unmarshalStoredNodeConfig(data []byte, out interface{}) error {
	var stored types.StoredConfig
	if err := json.Unmarshal(data, &stored); err == nil && len(stored.Config) > 0 {
		return json.Unmarshal(stored.Config, out)
	}
	return json.Unmarshal(data, out)
}

// initializeFabricXOrdererGroup initializes a Fabric X orderer group node
func (s *NodeService) initializeFabricXOrdererGroup(ctx context.Context, dbNode *db.Node, config *types.FabricXOrdererGroupConfig) (*types.FabricXOrdererGroupDeploymentConfig, error) {
	og := fabricx.NewOrdererGroup(
		s.db,
		s.orgService,
		s.keymanagementService,
		s.configService,
		s.logger,
		dbNode.ID,
		*config,
	)

	deploymentConfig, err := og.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize orderer group: %w", err)
	}

	return deploymentConfig, nil
}

// initializeFabricXCommitter initializes a Fabric X committer node
func (s *NodeService) initializeFabricXCommitter(ctx context.Context, dbNode *db.Node, config *types.FabricXCommitterConfig) (*types.FabricXCommitterDeploymentConfig, error) {
	c := fabricx.NewCommitter(
		s.db,
		s.orgService,
		s.keymanagementService,
		s.configService,
		s.logger,
		dbNode.ID,
		*config,
	)

	deploymentConfig, err := c.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize committer: %w", err)
	}

	return deploymentConfig, nil
}

// startFabricXOrdererGroup starts a Fabric X orderer group (all 4 sub-containers).
// Note: The orderer group needs a genesis block set before starting.
// If no genesis block is set, it will return an error.
func (s *NodeService) startFabricXOrdererGroup(ctx context.Context, dbNode *db.Node) error {
	if !dbNode.DeploymentConfig.Valid {
		return fmt.Errorf("node %d has no deployment config", dbNode.ID)
	}

	var cfg types.FabricXOrdererGroupDeploymentConfig
	if err := json.Unmarshal([]byte(dbNode.DeploymentConfig.String), &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal deployment config: %w", err)
	}

	// Parse original config to reconstruct OrdererGroup
	var nodeConfig types.FabricXOrdererGroupConfig
	if dbNode.NodeConfig.Valid {
		if err := unmarshalStoredNodeConfig([]byte(dbNode.NodeConfig.String), &nodeConfig); err != nil {
			return fmt.Errorf("failed to unmarshal node config: %w", err)
		}
	}

	og := fabricx.NewOrdererGroup(
		s.db,
		s.orgService,
		s.keymanagementService,
		s.configService,
		s.logger,
		dbNode.ID,
		nodeConfig,
	)

	if err := og.Start(&cfg); err != nil {
		return fmt.Errorf("failed to start orderer group: %w", err)
	}

	return nil
}

// startFabricXCommitter starts a Fabric X committer (all 5 sub-containers + postgres).
func (s *NodeService) startFabricXCommitter(ctx context.Context, dbNode *db.Node) error {
	if !dbNode.DeploymentConfig.Valid {
		return fmt.Errorf("node %d has no deployment config", dbNode.ID)
	}

	var cfg types.FabricXCommitterDeploymentConfig
	if err := json.Unmarshal([]byte(dbNode.DeploymentConfig.String), &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal deployment config: %w", err)
	}

	var nodeConfig types.FabricXCommitterConfig
	if dbNode.NodeConfig.Valid {
		if err := unmarshalStoredNodeConfig([]byte(dbNode.NodeConfig.String), &nodeConfig); err != nil {
			return fmt.Errorf("failed to unmarshal node config: %w", err)
		}
	}

	c := fabricx.NewCommitter(
		s.db,
		s.orgService,
		s.keymanagementService,
		s.configService,
		s.logger,
		dbNode.ID,
		nodeConfig,
	)

	// When PostgresHost is set, the committer is pointing at an external
	// Postgres (a services row or a shared container from quickstart).
	// Skip the per-committer embedded Postgres — otherwise both containers
	// would try to bind the same host port. We also clear PostgresContainer
	// so postgresEndpoint doesn't route committer roles through the
	// never-started embedded container name (DNS lookup would fail).
	startEmbeddedPostgres := cfg.PostgresHost == ""
	if !startEmbeddedPostgres {
		cfg.PostgresContainer = ""
	}
	if err := c.Start(&cfg, startEmbeddedPostgres); err != nil {
		return fmt.Errorf("failed to start committer: %w", err)
	}

	return nil
}

// stopFabricXOrdererGroup stops a Fabric X orderer group (all 4 sub-containers).
func (s *NodeService) stopFabricXOrdererGroup(ctx context.Context, dbNode *db.Node) error {
	if !dbNode.DeploymentConfig.Valid {
		return fmt.Errorf("node %d has no deployment config", dbNode.ID)
	}

	var cfg types.FabricXOrdererGroupDeploymentConfig
	if err := json.Unmarshal([]byte(dbNode.DeploymentConfig.String), &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal deployment config: %w", err)
	}

	var nodeConfig types.FabricXOrdererGroupConfig
	if dbNode.NodeConfig.Valid {
		if err := unmarshalStoredNodeConfig([]byte(dbNode.NodeConfig.String), &nodeConfig); err != nil {
			return fmt.Errorf("failed to unmarshal node config: %w", err)
		}
	}

	og := fabricx.NewOrdererGroup(
		s.db,
		s.orgService,
		s.keymanagementService,
		s.configService,
		s.logger,
		dbNode.ID,
		nodeConfig,
	)

	return og.Stop(&cfg)
}

// SetFabricXGenesisBlock writes the genesis block into a FabricX node's config
// directory (for orderer groups: router/batcher/consenter/assembler; for
// committers: sidecar bootstrap). Must be called after network genesis has
// been generated and before the node is started.
func (s *NodeService) SetFabricXGenesisBlock(ctx context.Context, nodeID int64, genesisBlock []byte) error {
	dbNode, err := s.db.GetNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node %d: %w", nodeID, err)
	}
	if !dbNode.DeploymentConfig.Valid {
		return fmt.Errorf("node %d has no deployment config", nodeID)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(dbNode.DeploymentConfig.String), &raw); err != nil {
		return fmt.Errorf("failed to parse deployment config: %w", err)
	}
	deployType, _ := raw["type"].(string)

	switch deployType {
	case "fabricx-child":
		// ADR-0001 path: the child's deployment_config points at a parent
		// node_group. The genesis block always lives on the group's shared
		// config dir, so we delegate to the parent's orchestrator (orderer
		// group or committer group). Writing twice is idempotent. Which
		// orchestrator to use is decided by the child's role: orderer roles
		// => OrdererGroup, committer roles => Committer.
		child, err := loadChildDeploymentConfig(dbNode)
		if err != nil {
			return fmt.Errorf("load child deployment: %w", err)
		}
		grp, err := s.db.GetNodeGroup(ctx, child.NodeGroupID)
		if err != nil {
			return fmt.Errorf("get parent group %d: %w", child.NodeGroupID, err)
		}
		if !grp.DeploymentConfig.Valid {
			return fmt.Errorf("parent group %d has no deployment config", child.NodeGroupID)
		}

		switch child.Role {
		case types.FabricXRoleOrdererRouter,
			types.FabricXRoleOrdererBatcher,
			types.FabricXRoleOrdererConsenter,
			types.FabricXRoleOrdererAssembler:
			var nodeConfig types.FabricXOrdererGroupConfig
			nodeConfig.Name = grp.Name
			if grp.OrganizationID.Valid {
				nodeConfig.OrganizationID = grp.OrganizationID.Int64
			}
			if grp.MspID.Valid {
				nodeConfig.MSPID = grp.MspID.String
			}
			og := fabricx.NewOrdererGroup(
				s.db,
				s.orgService,
				s.keymanagementService,
				s.configService,
				s.logger,
				0,
				nodeConfig,
			)
			return og.SetGenesisBlock(genesisBlock)

		case types.FabricXRoleCommitterSidecar,
			types.FabricXRoleCommitterCoordinator,
			types.FabricXRoleCommitterValidator,
			types.FabricXRoleCommitterVerifier,
			types.FabricXRoleCommitterQueryService:
			var nodeConfig types.FabricXCommitterConfig
			nodeConfig.Name = grp.Name
			if grp.OrganizationID.Valid {
				nodeConfig.OrganizationID = grp.OrganizationID.Int64
			}
			if grp.MspID.Valid {
				nodeConfig.MSPID = grp.MspID.String
			}
			c := fabricx.NewCommitter(
				s.db,
				s.orgService,
				s.keymanagementService,
				s.configService,
				s.logger,
				0,
				nodeConfig,
			)
			return c.SetGenesisBlock(genesisBlock)

		default:
			return fmt.Errorf("fabricx-child node %d has unknown role %q", nodeID, child.Role)
		}

	case "fabricx-orderer-group":
		var nodeConfig types.FabricXOrdererGroupConfig
		if dbNode.NodeConfig.Valid {
			if err := unmarshalStoredNodeConfig([]byte(dbNode.NodeConfig.String), &nodeConfig); err != nil {
				return fmt.Errorf("failed to unmarshal node config: %w", err)
			}
		}
		og := fabricx.NewOrdererGroup(
			s.db,
			s.orgService,
			s.keymanagementService,
			s.configService,
			s.logger,
			dbNode.ID,
			nodeConfig,
		)
		return og.SetGenesisBlock(genesisBlock)

	case "fabricx-committer":
		var nodeConfig types.FabricXCommitterConfig
		if dbNode.NodeConfig.Valid {
			if err := unmarshalStoredNodeConfig([]byte(dbNode.NodeConfig.String), &nodeConfig); err != nil {
				return fmt.Errorf("failed to unmarshal node config: %w", err)
			}
		}
		c := fabricx.NewCommitter(
			s.db,
			s.orgService,
			s.keymanagementService,
			s.configService,
			s.logger,
			dbNode.ID,
			nodeConfig,
		)
		return c.SetGenesisBlock(genesisBlock)

	default:
		return fmt.Errorf("node %d is not a FabricX node (type=%s)", nodeID, deployType)
	}
}

// stopFabricXCommitter stops a Fabric X committer (all 5 sub-containers + postgres).
func (s *NodeService) stopFabricXCommitter(ctx context.Context, dbNode *db.Node) error {
	if !dbNode.DeploymentConfig.Valid {
		return fmt.Errorf("node %d has no deployment config", dbNode.ID)
	}

	var cfg types.FabricXCommitterDeploymentConfig
	if err := json.Unmarshal([]byte(dbNode.DeploymentConfig.String), &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal deployment config: %w", err)
	}

	var nodeConfig types.FabricXCommitterConfig
	if dbNode.NodeConfig.Valid {
		if err := unmarshalStoredNodeConfig([]byte(dbNode.NodeConfig.String), &nodeConfig); err != nil {
			return fmt.Errorf("failed to unmarshal node config: %w", err)
		}
	}

	c := fabricx.NewCommitter(
		s.db,
		s.orgService,
		s.keymanagementService,
		s.configService,
		s.logger,
		dbNode.ID,
		nodeConfig,
	)

	return c.Stop(&cfg)
}

// renewFabricXNodeCertificates handles cert renewal for legacy
// monolithic FabricX node rows (FABRICX_ORDERER_GROUP /
// FABRICX_COMMITTER) — pre-node_groups installs where one nodes row
// owns all child containers. Re-uses the same key pair (matches
// Fabric peer renewal semantics): only the certs are reissued.
//
// Sequence:
//  1. emit RENEWING_CERTIFICATES event
//  2. stop all child containers (so we never have running
//     containers reading stale msp/tls dirs while we rewrite them)
//  3. call OrdererGroup.RenewCertificates / Committer.RenewCertificates
//  4. persist updated deployment_config (new SignCert/TLSCert) on the row
//  5. start the node again
//  6. emit RENEWED_CERTIFICATES event
func (s *NodeService) renewFabricXNodeCertificates(ctx context.Context, dbNode *db.Node, deploymentConfig types.NodeDeploymentConfig) error {
	if err := s.eventService.CreateEvent(ctx, dbNode.ID, NodeEventRenewingCertificates, map[string]interface{}{
		"node_id": dbNode.ID,
		"name":    dbNode.Name,
		"action":  "renewing_certificates",
	}); err != nil {
		s.logger.Error("Failed to create certificate renewal starting event", "error", err)
	}

	// Reconstruct the orchestrator from the node_config + deployment.
	// Same opts shape Init/Start use so renewal sees the same domains,
	// MSP id, etc. that were active when the certs were originally issued.
	switch types.NodeType(dbNode.NodeType.String) {
	case types.NodeTypeFabricXOrdererGroup:
		ogCfg, ok := deploymentConfig.(*types.FabricXOrdererGroupDeploymentConfig)
		if !ok {
			return fmt.Errorf("expected FabricXOrdererGroupDeploymentConfig for node %d", dbNode.ID)
		}
		var nodeConfig types.FabricXOrdererGroupConfig
		if dbNode.NodeConfig.Valid {
			if err := unmarshalStoredNodeConfig([]byte(dbNode.NodeConfig.String), &nodeConfig); err != nil {
				return fmt.Errorf("unmarshal node config: %w", err)
			}
		}
		if nodeConfig.Name == "" {
			nodeConfig.Name = dbNode.Name
		}
		og := fabricx.NewOrdererGroup(
			s.db, s.orgService, s.keymanagementService, s.configService, s.logger,
			dbNode.ID, nodeConfig,
		)
		if err := og.Stop(ogCfg); err != nil {
			s.logger.Warn("orderer group stop before renewal failed; continuing", "node", dbNode.ID, "err", err)
		}
		updated, err := og.RenewCertificates(ogCfg)
		if err != nil {
			return err
		}
		if err := s.persistFabricXOrdererGroupDeployment(ctx, dbNode, updated); err != nil {
			return fmt.Errorf("persist updated orderer group config: %w", err)
		}
		if err := og.Start(updated); err != nil {
			return fmt.Errorf("restart orderer group after renewal: %w", err)
		}

	case types.NodeTypeFabricXCommitter:
		cCfg, ok := deploymentConfig.(*types.FabricXCommitterDeploymentConfig)
		if !ok {
			return fmt.Errorf("expected FabricXCommitterDeploymentConfig for node %d", dbNode.ID)
		}
		var nodeConfig types.FabricXCommitterConfig
		if dbNode.NodeConfig.Valid {
			if err := unmarshalStoredNodeConfig([]byte(dbNode.NodeConfig.String), &nodeConfig); err != nil {
				return fmt.Errorf("unmarshal node config: %w", err)
			}
		}
		if nodeConfig.Name == "" {
			nodeConfig.Name = dbNode.Name
		}
		c := fabricx.NewCommitter(
			s.db, s.orgService, s.keymanagementService, s.configService, s.logger,
			dbNode.ID, nodeConfig,
		)
		if err := c.Stop(cCfg); err != nil {
			s.logger.Warn("committer stop before renewal failed; continuing", "node", dbNode.ID, "err", err)
		}
		updated, err := c.RenewCertificates(cCfg)
		if err != nil {
			return err
		}
		if err := s.persistFabricXCommitterDeployment(ctx, dbNode, updated); err != nil {
			return fmt.Errorf("persist updated committer config: %w", err)
		}
		if err := c.Start(updated, true); err != nil {
			return fmt.Errorf("restart committer after renewal: %w", err)
		}

	default:
		return fmt.Errorf("renewFabricXNodeCertificates: unsupported node type %q", dbNode.NodeType.String)
	}

	if err := s.eventService.CreateEvent(ctx, dbNode.ID, NodeEventRenewedCertificates, map[string]interface{}{
		"node_id": dbNode.ID,
		"name":    dbNode.Name,
		"action":  "renewing_certificates",
	}); err != nil {
		s.logger.Error("Failed to create certificate renewal completed event", "error", err)
	}
	return nil
}

// renewFabricXChildCertificates handles cert renewal for per-role
// child node rows (FABRICX_ORDERER_ROUTER, FABRICX_COMMITTER_SIDECAR,
// etc). Identity lives on the parent node_group, so we renew the
// group's keys once and restart every sibling so the rewritten msp/
// tls dirs take effect for the whole group.
//
// Calling renewal on any one child triggers a group-wide renewal.
// That's the right tradeoff: all children share one cert pair by
// design, so per-child renewal is meaningless.
func (s *NodeService) renewFabricXChildCertificates(ctx context.Context, dbNode *db.Node) error {
	if err := s.eventService.CreateEvent(ctx, dbNode.ID, NodeEventRenewingCertificates, map[string]interface{}{
		"node_id": dbNode.ID,
		"name":    dbNode.Name,
		"action":  "renewing_certificates",
	}); err != nil {
		s.logger.Error("Failed to create certificate renewal starting event", "error", err)
	}

	child, err := loadChildDeploymentConfig(dbNode)
	if err != nil {
		return err
	}

	switch child.Role {
	case types.FabricXRoleOrdererRouter,
		types.FabricXRoleOrdererBatcher,
		types.FabricXRoleOrdererConsenter,
		types.FabricXRoleOrdererAssembler:
		groupCfg, err := s.loadGroupOrdererDeployment(ctx, child.NodeGroupID)
		if err != nil {
			return err
		}
		grp, err := s.db.GetNodeGroup(ctx, child.NodeGroupID)
		if err != nil {
			return fmt.Errorf("load node_group %d: %w", child.NodeGroupID, err)
		}
		og := fabricx.NewOrdererGroup(
			s.db, s.orgService, s.keymanagementService, s.configService, s.logger,
			0,
			types.FabricXOrdererGroupConfig{
				Name:           grp.Name,
				OrganizationID: groupCfg.OrganizationID,
				MSPID:          groupCfg.MSPID,
				PartyID:        groupCfg.PartyID,
				ExternalIP:     groupCfg.ExternalIP,
				DomainNames:    groupCfg.DomainNames,
				Version:        groupCfg.Version,
			},
		)
		// Group-wide renewal stops all four children, rewrites all
		// msp/tls dirs, then restarts via Start. That covers the calling
		// child too — no separate per-child restart needed.
		if err := og.Stop(groupCfg); err != nil {
			s.logger.Warn("orderer group stop before renewal failed; continuing", "group", child.NodeGroupID, "err", err)
		}
		updated, err := og.RenewCertificates(groupCfg)
		if err != nil {
			return err
		}
		if err := s.persistNodeGroupDeployment(ctx, grp, updated.SignCert, updated.TLSCert, updated); err != nil {
			return fmt.Errorf("persist renewed orderer group: %w", err)
		}
		if err := og.Start(updated); err != nil {
			return fmt.Errorf("restart orderer group after renewal: %w", err)
		}

	case types.FabricXRoleCommitterSidecar,
		types.FabricXRoleCommitterCoordinator,
		types.FabricXRoleCommitterValidator,
		types.FabricXRoleCommitterVerifier,
		types.FabricXRoleCommitterQueryService:
		groupCfg, err := s.loadGroupCommitterDeployment(ctx, child.NodeGroupID)
		if err != nil {
			return err
		}
		grp, err := s.db.GetNodeGroup(ctx, child.NodeGroupID)
		if err != nil {
			return fmt.Errorf("load node_group %d: %w", child.NodeGroupID, err)
		}
		c := fabricx.NewCommitter(
			s.db, s.orgService, s.keymanagementService, s.configService, s.logger,
			0,
			types.FabricXCommitterConfig{
				Name:             grp.Name,
				OrganizationID:   groupCfg.OrganizationID,
				MSPID:            groupCfg.MSPID,
				ExternalIP:       groupCfg.ExternalIP,
				DomainNames:      groupCfg.DomainNames,
				Version:          groupCfg.Version,
				OrdererEndpoints: groupCfg.OrdererEndpoints,
				PostgresHost:     groupCfg.PostgresHost,
				PostgresPort:     groupCfg.PostgresPort,
				PostgresDB:       groupCfg.PostgresDB,
				PostgresUser:     groupCfg.PostgresUser,
				PostgresPassword: groupCfg.PostgresPassword,
				ChannelID:        groupCfg.ChannelID,
			},
		)
		if err := c.Stop(groupCfg); err != nil {
			s.logger.Warn("committer stop before renewal failed; continuing", "group", child.NodeGroupID, "err", err)
		}
		updated, err := c.RenewCertificates(groupCfg)
		if err != nil {
			return err
		}
		if err := s.persistNodeGroupDeployment(ctx, grp, updated.SignCert, updated.TLSCert, updated); err != nil {
			return fmt.Errorf("persist renewed committer: %w", err)
		}
		if err := c.Start(updated, true); err != nil {
			return fmt.Errorf("restart committer after renewal: %w", err)
		}

	default:
		return fmt.Errorf("renewFabricXChildCertificates: unknown role %q on node %d", child.Role, dbNode.ID)
	}

	if err := s.eventService.CreateEvent(ctx, dbNode.ID, NodeEventRenewedCertificates, map[string]interface{}{
		"node_id": dbNode.ID,
		"name":    dbNode.Name,
		"action":  "renewing_certificates",
	}); err != nil {
		s.logger.Error("Failed to create certificate renewal completed event", "error", err)
	}
	return nil
}

// persistFabricXOrdererGroupDeployment serializes an updated orderer
// group deployment_config back to its `nodes` row (legacy monolithic path).
func (s *NodeService) persistFabricXOrdererGroupDeployment(ctx context.Context, dbNode *db.Node, cfg *types.FabricXOrdererGroupDeploymentConfig) error {
	js, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal orderer group deployment_config: %w", err)
	}
	_, err = s.db.UpdateNodeDeploymentConfig(ctx, &db.UpdateNodeDeploymentConfigParams{
		ID:               dbNode.ID,
		DeploymentConfig: nullStringValid(string(js)),
	})
	return err
}

// persistFabricXCommitterDeployment serializes an updated committer
// deployment_config back to its `nodes` row.
func (s *NodeService) persistFabricXCommitterDeployment(ctx context.Context, dbNode *db.Node, cfg *types.FabricXCommitterDeploymentConfig) error {
	js, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal committer deployment_config: %w", err)
	}
	_, err = s.db.UpdateNodeDeploymentConfig(ctx, &db.UpdateNodeDeploymentConfigParams{
		ID:               dbNode.ID,
		DeploymentConfig: nullStringValid(string(js)),
	})
	return err
}

// persistNodeGroupDeployment serializes an updated node_group
// deployment_config (orderer or committer; the JSON shape is opaque
// to the SQL layer) and refreshes the denormalized cert columns so
// the /node-groups list view doesn't lag the canonical deployment.
func (s *NodeService) persistNodeGroupDeployment(ctx context.Context, grp *db.NodeGroup, signCert, tlsCert string, deploymentConfig interface{}) error {
	js, err := json.Marshal(deploymentConfig)
	if err != nil {
		return fmt.Errorf("marshal node_group deployment_config: %w", err)
	}
	_, err = s.db.UpdateNodeGroup(ctx, &db.UpdateNodeGroupParams{
		ID:               grp.ID,
		Name:             grp.Name,
		MspID:            grp.MspID,
		OrganizationID:   grp.OrganizationID,
		PartyID:          grp.PartyID,
		Version:          grp.Version,
		ExternalIp:       grp.ExternalIp,
		DomainNames:      grp.DomainNames,
		SignKeyID:        grp.SignKeyID,
		TlsKeyID:         grp.TlsKeyID,
		SignCert:         nullStringValid(signCert),
		TlsCert:          nullStringValid(tlsCert),
		CaCert:           grp.CaCert,
		TlsCaCert:        grp.TlsCaCert,
		Config:           grp.Config,
		DeploymentConfig: nullStringValid(string(js)),
	})
	return err
}

// UpdateFabricXOrdererGroup updates the mutable fields of a Fabric-X
// orderer group node. Today only the image-tag (Version) is mutable.
//
// Semantics match Fabric peer/orderer: the new version is persisted to
// the node's NodeConfig and DeploymentConfig — and to the parent
// node_groups row so per-role child rows pick it up — but NO containers
// are stopped or restarted. The user clicks Restart manually to apply.
//
// Why also touch the parent node_groups row: per-role child rows
// (FABRICX_ORDERER_ROUTER, etc.) don't carry their own version. They
// inherit it from the group's deployment_config when StartCommitterRole
// / StartOrdererRole reads it. Without this update, the child would
// keep starting against the old image even after the parent node row
// said the version had changed.
func (s *NodeService) UpdateFabricXOrdererGroup(ctx context.Context, opts UpdateFabricXOrdererGroupOpts) (*NodeResponse, error) {
	dbNode, err := s.db.GetNode(ctx, opts.NodeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("fabric-x orderer group node %d not found", opts.NodeID)
		}
		return nil, fmt.Errorf("get fabric-x orderer group node: %w", err)
	}
	if types.NodeType(dbNode.NodeType.String) != types.NodeTypeFabricXOrdererGroup {
		return nil, fmt.Errorf("node %d is not a fabric-x orderer group", opts.NodeID)
	}

	if opts.Version == "" {
		// No-op: nothing to change. Return the current node state instead
		// of pushing an empty write through every persistence layer.
		_, resp := s.mapDBNodeToServiceNode(dbNode)
		return resp, nil
	}

	// 1. Update the node-level NodeConfig (the user-input opts blob).
	var nodeCfg types.FabricXOrdererGroupConfig
	if dbNode.NodeConfig.Valid {
		if err := unmarshalStoredNodeConfig([]byte(dbNode.NodeConfig.String), &nodeCfg); err != nil {
			return nil, fmt.Errorf("unmarshal node config: %w", err)
		}
	}
	if nodeCfg.Type == "" {
		nodeCfg.Type = "fabricx-orderer-group"
	}
	nodeCfg.Version = opts.Version

	nodeCfgJSON, err := json.Marshal(nodeCfg)
	if err != nil {
		return nil, fmt.Errorf("marshal node config: %w", err)
	}
	storedJSON, err := json.Marshal(types.StoredConfig{
		Type:   "fabricx-orderer-group",
		Config: nodeCfgJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal stored node config: %w", err)
	}
	if _, err := s.db.UpdateNodeConfig(ctx, &db.UpdateNodeConfigParams{
		ID:         opts.NodeID,
		NodeConfig: sql.NullString{String: string(storedJSON), Valid: true},
	}); err != nil {
		return nil, fmt.Errorf("persist node config: %w", err)
	}

	// 2. Update the node-level DeploymentConfig (the resolved/realized
	//    config the lifecycle reads from).
	if dbNode.DeploymentConfig.Valid {
		var depCfg types.FabricXOrdererGroupDeploymentConfig
		if err := json.Unmarshal([]byte(dbNode.DeploymentConfig.String), &depCfg); err != nil {
			return nil, fmt.Errorf("unmarshal deployment config: %w", err)
		}
		depCfg.Version = opts.Version
		if err := s.persistFabricXOrdererGroupDeployment(ctx, dbNode, &depCfg); err != nil {
			return nil, fmt.Errorf("persist deployment config: %w", err)
		}
	}

	// 3. If this node belongs to a node_group, update the group's Version
	//    column + deployment_config.Version so per-role child rows pick up
	//    the new image at next start.
	if dbNode.NodeGroupID.Valid {
		grp, err := s.db.GetNodeGroup(ctx, dbNode.NodeGroupID.Int64)
		if err == nil {
			var grpDep types.FabricXOrdererGroupDeploymentConfig
			if grp.DeploymentConfig.Valid {
				if err := json.Unmarshal([]byte(grp.DeploymentConfig.String), &grpDep); err != nil {
					return nil, fmt.Errorf("unmarshal group deployment config: %w", err)
				}
			}
			grpDep.Version = opts.Version
			grp.Version = nullStringValid(opts.Version)
			if err := s.persistNodeGroupDeployment(ctx, grp, grpDep.SignCert, grpDep.TLSCert, &grpDep); err != nil {
				return nil, fmt.Errorf("persist node_group deployment: %w", err)
			}
		} else {
			s.logger.Warn("update fabric-x orderer group: failed to load parent node_group; skipping group-level version update",
				"nodeID", opts.NodeID, "nodeGroupID", dbNode.NodeGroupID.Int64, "err", err)
		}
	}

	// Re-read so the response reflects the persisted state.
	updated, err := s.db.GetNode(ctx, opts.NodeID)
	if err != nil {
		return nil, fmt.Errorf("re-load updated node: %w", err)
	}
	_, resp := s.mapDBNodeToServiceNode(updated)
	return resp, nil
}

// UpdateFabricXCommitter is the committer counterpart of
// UpdateFabricXOrdererGroup. Same scope (Version-only), same
// no-auto-restart semantics.
func (s *NodeService) UpdateFabricXCommitter(ctx context.Context, opts UpdateFabricXCommitterOpts) (*NodeResponse, error) {
	dbNode, err := s.db.GetNode(ctx, opts.NodeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("fabric-x committer node %d not found", opts.NodeID)
		}
		return nil, fmt.Errorf("get fabric-x committer node: %w", err)
	}
	if types.NodeType(dbNode.NodeType.String) != types.NodeTypeFabricXCommitter {
		return nil, fmt.Errorf("node %d is not a fabric-x committer", opts.NodeID)
	}

	if opts.Version == "" {
		_, resp := s.mapDBNodeToServiceNode(dbNode)
		return resp, nil
	}

	var nodeCfg types.FabricXCommitterConfig
	if dbNode.NodeConfig.Valid {
		if err := unmarshalStoredNodeConfig([]byte(dbNode.NodeConfig.String), &nodeCfg); err != nil {
			return nil, fmt.Errorf("unmarshal node config: %w", err)
		}
	}
	if nodeCfg.Type == "" {
		nodeCfg.Type = "fabricx-committer"
	}
	nodeCfg.Version = opts.Version

	nodeCfgJSON, err := json.Marshal(nodeCfg)
	if err != nil {
		return nil, fmt.Errorf("marshal node config: %w", err)
	}
	storedJSON, err := json.Marshal(types.StoredConfig{
		Type:   "fabricx-committer",
		Config: nodeCfgJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal stored node config: %w", err)
	}
	if _, err := s.db.UpdateNodeConfig(ctx, &db.UpdateNodeConfigParams{
		ID:         opts.NodeID,
		NodeConfig: sql.NullString{String: string(storedJSON), Valid: true},
	}); err != nil {
		return nil, fmt.Errorf("persist node config: %w", err)
	}

	if dbNode.DeploymentConfig.Valid {
		var depCfg types.FabricXCommitterDeploymentConfig
		if err := json.Unmarshal([]byte(dbNode.DeploymentConfig.String), &depCfg); err != nil {
			return nil, fmt.Errorf("unmarshal deployment config: %w", err)
		}
		depCfg.Version = opts.Version
		if err := s.persistFabricXCommitterDeployment(ctx, dbNode, &depCfg); err != nil {
			return nil, fmt.Errorf("persist deployment config: %w", err)
		}
	}

	if dbNode.NodeGroupID.Valid {
		grp, err := s.db.GetNodeGroup(ctx, dbNode.NodeGroupID.Int64)
		if err == nil {
			var grpDep types.FabricXCommitterDeploymentConfig
			if grp.DeploymentConfig.Valid {
				if err := json.Unmarshal([]byte(grp.DeploymentConfig.String), &grpDep); err != nil {
					return nil, fmt.Errorf("unmarshal group deployment config: %w", err)
				}
			}
			grpDep.Version = opts.Version
			grp.Version = nullStringValid(opts.Version)
			if err := s.persistNodeGroupDeployment(ctx, grp, grpDep.SignCert, grpDep.TLSCert, &grpDep); err != nil {
				return nil, fmt.Errorf("persist node_group deployment: %w", err)
			}
		} else {
			s.logger.Warn("update fabric-x committer: failed to load parent node_group; skipping group-level version update",
				"nodeID", opts.NodeID, "nodeGroupID", dbNode.NodeGroupID.Int64, "err", err)
		}
	}

	updated, err := s.db.GetNode(ctx, opts.NodeID)
	if err != nil {
		return nil, fmt.Errorf("re-load updated node: %w", err)
	}
	_, resp := s.mapDBNodeToServiceNode(updated)
	return resp, nil
}
