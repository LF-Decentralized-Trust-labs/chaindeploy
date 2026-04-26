package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/fabricx"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

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
