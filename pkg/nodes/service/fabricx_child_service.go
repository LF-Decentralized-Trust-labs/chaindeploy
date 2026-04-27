package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/fabricx"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// Per-child FabricX lifecycle. A child `nodes` row represents exactly one
// container (router / batcher / ... / query-service). The deployment
// config persisted on the child is intentionally thin: the heavy state
// (keys/certs/ports/external IP) lives on the parent node_groups row.
//
// StartNode on one of these rows loads:
//   - the child's FabricXChildDeploymentConfig (for role + container name)
//   - the parent group's full deployment config (from node_groups.deployment_config)
//
// and then delegates to fabricx.{OrdererGroup,Committer}.Start{Orderer,Committer}Role.

// loadChildDeploymentConfig parses the thin per-child config stored on a
// `nodes` row. Returns a typed struct the caller can switch on.
func loadChildDeploymentConfig(dbNode *db.Node) (*types.FabricXChildDeploymentConfig, error) {
	if !dbNode.DeploymentConfig.Valid {
		return nil, fmt.Errorf("node %d has no deployment config", dbNode.ID)
	}
	var cfg types.FabricXChildDeploymentConfig
	if err := json.Unmarshal([]byte(dbNode.DeploymentConfig.String), &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal child deployment config: %w", err)
	}
	if cfg.NodeGroupID == 0 {
		return nil, fmt.Errorf("node %d child deployment config missing nodeGroupId", dbNode.ID)
	}
	if cfg.Role == "" {
		return nil, fmt.Errorf("node %d child deployment config missing role", dbNode.ID)
	}
	return &cfg, nil
}

// loadGroupOrdererDeployment hydrates the parent orderer-group deployment
// config from the node_groups row referenced by the child. The caller is
// expected to also have loaded the child config first so the node_group
// id is known.
func (s *NodeService) loadGroupOrdererDeployment(ctx context.Context, nodeGroupID int64) (*types.FabricXOrdererGroupDeploymentConfig, error) {
	row, err := s.db.GetNodeGroup(ctx, nodeGroupID)
	if err != nil {
		return nil, fmt.Errorf("load node_group %d: %w", nodeGroupID, err)
	}
	if !row.DeploymentConfig.Valid {
		return nil, fmt.Errorf("node_group %d has no deployment_config; run Init on the group first", nodeGroupID)
	}
	var cfg types.FabricXOrdererGroupDeploymentConfig
	if err := json.Unmarshal([]byte(row.DeploymentConfig.String), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal orderer group deployment config: %w", err)
	}
	return &cfg, nil
}

// loadGroupCommitterDeployment hydrates the parent committer deployment
// config from the node_groups row referenced by the child.
func (s *NodeService) loadGroupCommitterDeployment(ctx context.Context, nodeGroupID int64) (*types.FabricXCommitterDeploymentConfig, error) {
	row, err := s.db.GetNodeGroup(ctx, nodeGroupID)
	if err != nil {
		return nil, fmt.Errorf("load node_group %d: %w", nodeGroupID, err)
	}
	if !row.DeploymentConfig.Valid {
		return nil, fmt.Errorf("node_group %d has no deployment_config; run Init on the group first", nodeGroupID)
	}
	var cfg types.FabricXCommitterDeploymentConfig
	if err := json.Unmarshal([]byte(row.DeploymentConfig.String), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal committer deployment config: %w", err)
	}
	return &cfg, nil
}

// startFabricXChild dispatches a child `nodes` row to the appropriate
// per-role starter on the parent's in-memory OrdererGroup / Committer
// instance. Called by StartNode when it sees a FABRICX_ORDERER_* /
// FABRICX_COMMITTER_* node type.
//
// Note: this path assumes the group coordinator has already run
// Prepare*Start for the owning group. In the single-role-start case
// (one child being started in isolation) the caller is responsible for
// running Prepare first; see pkg/nodegroups/service.
func (s *NodeService) startFabricXChild(ctx context.Context, dbNode *db.Node) error {
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
		og := fabricx.NewOrdererGroup(
			s.db, s.orgService, s.keymanagementService, s.configService, s.logger,
			dbNode.ID,
			// The per-role path only needs the handful of opts used by
			// ensureMaterials/StartOrdererRole (Name, MSPID, Env,
			// OrganizationID, ExternalIP). Everything else lives on cfg.
			types.FabricXOrdererGroupConfig{
				Name:           deriveGroupNameFromChild(dbNode, child),
				OrganizationID: groupCfg.OrganizationID,
				MSPID:          groupCfg.MSPID,
				ExternalIP:     groupCfg.ExternalIP,
				PartyID:        groupCfg.PartyID,
			},
		)
		return og.StartOrdererRole(ctx, groupCfg, child.Role)

	case types.FabricXRoleCommitterSidecar,
		types.FabricXRoleCommitterCoordinator,
		types.FabricXRoleCommitterValidator,
		types.FabricXRoleCommitterVerifier,
		types.FabricXRoleCommitterQueryService:
		groupCfg, err := s.loadGroupCommitterDeployment(ctx, child.NodeGroupID)
		if err != nil {
			return err
		}
		c := fabricx.NewCommitter(
			s.db, s.orgService, s.keymanagementService, s.configService, s.logger,
			dbNode.ID,
			types.FabricXCommitterConfig{
				Name:             deriveGroupNameFromChild(dbNode, child),
				OrganizationID:   groupCfg.OrganizationID,
				MSPID:            groupCfg.MSPID,
				ExternalIP:       groupCfg.ExternalIP,
				OrdererEndpoints: groupCfg.OrdererEndpoints,
				PostgresHost:     groupCfg.PostgresHost,
				PostgresPort:     groupCfg.PostgresPort,
				PostgresDB:       groupCfg.PostgresDB,
				PostgresUser:     groupCfg.PostgresUser,
				PostgresPassword: groupCfg.PostgresPassword,
				ChannelID:        groupCfg.ChannelID,
			},
		)
		// StartCommitterRole requires the per-group docker bridge network to
		// already exist (committer roles dial each other by container name
		// on that network). PrepareCommitterStart is idempotent: if the
		// network + role configs are already on disk, it's a no-op beyond
		// rewrites. Running it here means single-role joins (e.g. via the
		// deployer's JoinNode loop) don't need an upstream StartGroup call.
		if err := c.PrepareCommitterStart(ctx, groupCfg, "", 0); err != nil {
			return fmt.Errorf("prepare committer group %d: %w", child.NodeGroupID, err)
		}
		return c.StartCommitterRole(ctx, groupCfg, child.Role)

	default:
		return fmt.Errorf("unknown fabricx child role %q on node %d", child.Role, dbNode.ID)
	}
}

// stopFabricXChild mirrors startFabricXChild but stops exactly one
// container. Group teardown (network removal, postgres) stays on the
// coordinator in pkg/nodegroups/service.
func (s *NodeService) stopFabricXChild(ctx context.Context, dbNode *db.Node) error {
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
		og := fabricx.NewOrdererGroup(
			s.db, s.orgService, s.keymanagementService, s.configService, s.logger,
			dbNode.ID,
			types.FabricXOrdererGroupConfig{
				Name:           deriveGroupNameFromChild(dbNode, child),
				OrganizationID: groupCfg.OrganizationID,
				MSPID:          groupCfg.MSPID,
			},
		)
		return og.StopOrdererRole(ctx, groupCfg, child.Role)

	case types.FabricXRoleCommitterSidecar,
		types.FabricXRoleCommitterCoordinator,
		types.FabricXRoleCommitterValidator,
		types.FabricXRoleCommitterVerifier,
		types.FabricXRoleCommitterQueryService:
		groupCfg, err := s.loadGroupCommitterDeployment(ctx, child.NodeGroupID)
		if err != nil {
			return err
		}
		c := fabricx.NewCommitter(
			s.db, s.orgService, s.keymanagementService, s.configService, s.logger,
			dbNode.ID,
			types.FabricXCommitterConfig{
				Name:           deriveGroupNameFromChild(dbNode, child),
				OrganizationID: groupCfg.OrganizationID,
				MSPID:          groupCfg.MSPID,
			},
		)
		return c.StopCommitterRole(ctx, groupCfg, child.Role)

	default:
		return fmt.Errorf("unknown fabricx child role %q on node %d", child.Role, dbNode.ID)
	}
}

// deriveGroupNameFromChild reconstructs the group-level name the fabricx
// OrdererGroup/Committer uses to compute container-name prefixes and
// baseDir(). Children are conventionally named "<group>-<role>"; the
// baseDir is derived from the group name only, so we strip the trailing
// "-<role>" suffix from the child's DB name when present. If the name
// doesn't contain the expected suffix (legacy rows) we fall back to the
// first path segment of the deployment container name.
func deriveGroupNameFromChild(dbNode *db.Node, child *types.FabricXChildDeploymentConfig) string {
	name := dbNode.Name
	suffix := "-" + string(child.Role)
	if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
		return name[:len(name)-len(suffix)]
	}
	// Fallback: fabricx container names are "fabricx-<mspid>-<slug>-<role>".
	// The Go code computes baseDir from opts.Name directly; if the DB Name
	// is already the group name, return it verbatim.
	return name
}
