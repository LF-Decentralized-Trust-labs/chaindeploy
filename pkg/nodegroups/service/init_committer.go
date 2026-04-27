// Package service — committer-group initialization.
//
// InitCommitterGroup is the entry point that turns an empty node_groups row
// into a fully populated parent (with generated crypto + deployment_config)
// plus 5 per-role child `nodes` rows: sidecar, coordinator, validator,
// verifier, query-service.
//
// After Init:
//   - node_groups.deployment_config carries the shared FabricXCommitterDeploymentConfig
//   - node_groups.{sign,tls}_{key_id,cert}, ca_cert, tls_ca_cert are populated
//   - 5 child nodes exist with node_type=FABRICX_COMMITTER_{SIDECAR,COORDINATOR,...}
//     each carrying a thin FabricXChildDeploymentConfig envelope
//
// Starting the group via POST /node-groups/{id}/start then fans out to the
// existing per-role StartCommitterRole path without further orchestration.
//
// This mirrors init_orderer.go's structure and persists the committer-only
// fields (orderer endpoints, postgres connection info, channel id) into
// the deployment_config blob — they're not first-class columns on
// node_groups.
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	ngtypes "github.com/chainlaunch/chainlaunch/pkg/nodegroups/types"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// CommitterInitInput carries the committer-specific fields that aren't
// stored as first-class columns on node_groups: per-role port allocations,
// upstream orderer endpoints (assemblers), postgres connection info, and
// the target channel id. The persisted node_groups row provides MSPID,
// org, externalIP, domains, and version — same as the orderer path.
type CommitterInitInput struct {
	SidecarPort      int      `json:"sidecarPort"`
	CoordinatorPort  int      `json:"coordinatorPort"`
	ValidatorPort    int      `json:"validatorPort"`
	VerifierPort     int      `json:"verifierPort"`
	QueryServicePort int      `json:"queryServicePort"`
	OrdererEndpoints []string `json:"ordererEndpoints"`
	PostgresHost     string   `json:"postgresHost"`
	PostgresPort     int      `json:"postgresPort,omitempty"`
	PostgresDB       string   `json:"postgresDb,omitempty"`
	PostgresUser     string   `json:"postgresUser,omitempty"`
	PostgresPassword string   `json:"postgresPassword,omitempty"`
	ChannelID        string   `json:"channelId,omitempty"`
}

// InitCommitterGroup generates crypto, writes on-disk config, persists
// the resulting deployment_config + certs on the group, and creates one
// child nodes row per role. Idempotency: rejects re-init on a populated
// group — the caller should Delete and recreate instead.
func (s *Service) InitCommitterGroup(ctx context.Context, id int64, in CommitterInitInput) (*ngtypes.NodeGroup, error) {
	grp, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if grp.GroupType != ngtypes.GroupTypeFabricXCommitter {
		return nil, fmt.Errorf("group %d is %s; InitCommitterGroup is for %s only",
			id, grp.GroupType, ngtypes.GroupTypeFabricXCommitter)
	}
	if len(grp.DeploymentConfig) > 0 {
		return nil, fmt.Errorf("group %d already initialized; delete and recreate to re-init", id)
	}
	if grp.OrganizationID == nil || *grp.OrganizationID == 0 {
		return nil, fmt.Errorf("group %d has no organizationId; required for Init", id)
	}
	if grp.MSPID == "" {
		return nil, fmt.Errorf("group %d has no mspId; required for Init", id)
	}
	if in.PostgresHost == "" {
		return nil, fmt.Errorf("postgresHost is required")
	}
	if len(in.OrdererEndpoints) == 0 {
		return nil, fmt.Errorf("ordererEndpoints is required")
	}

	existing, err := s.db.ListNodesByGroup(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list existing children: %w", err)
	}
	if len(existing) > 0 {
		return nil, fmt.Errorf("group %d already has %d children; delete them before re-init", id, len(existing))
	}

	c := s.fabricxDeps.committerFactory(s.db, 0, nodetypes.FabricXCommitterConfig{
		Name:             grp.Name,
		OrganizationID:   *grp.OrganizationID,
		MSPID:            grp.MSPID,
		ExternalIP:       grp.ExternalIP,
		DomainNames:      grp.DomainNames,
		Version:          grp.Version,
		SidecarPort:      in.SidecarPort,
		CoordinatorPort:  in.CoordinatorPort,
		ValidatorPort:    in.ValidatorPort,
		VerifierPort:     in.VerifierPort,
		QueryServicePort: in.QueryServicePort,
		OrdererEndpoints: in.OrdererEndpoints,
		PostgresHost:     in.PostgresHost,
		PostgresPort:     in.PostgresPort,
		PostgresDB:       in.PostgresDB,
		PostgresUser:     in.PostgresUser,
		PostgresPassword: in.PostgresPassword,
		ChannelID:        in.ChannelID,
	})

	cfg, err := c.Init()
	if err != nil {
		s.markGroupError(ctx, id, err)
		return nil, fmt.Errorf("committer Init: %w", err)
	}

	depCfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal deployment config: %w", err)
	}

	domainsJSON := sql.NullString{}
	if len(cfg.DomainNames) > 0 {
		b, _ := json.Marshal(cfg.DomainNames)
		domainsJSON = sql.NullString{String: string(b), Valid: true}
	}

	configJSON := sql.NullString{}
	if len(grp.Config) > 0 {
		configJSON = sql.NullString{String: string(grp.Config), Valid: true}
	}

	if _, err := s.db.UpdateNodeGroup(ctx, &db.UpdateNodeGroupParams{
		ID:               id,
		Name:             grp.Name,
		MspID:            nullStringFrom(grp.MSPID),
		OrganizationID:   nullInt64FromPtr(grp.OrganizationID),
		PartyID:          nullInt64FromPtr(grp.PartyID),
		Version:          nullStringFrom(cfg.Version),
		ExternalIp:       nullStringFrom(cfg.ExternalIP),
		DomainNames:      domainsJSON,
		SignKeyID:        sql.NullInt64{Int64: cfg.SignKeyID, Valid: cfg.SignKeyID != 0},
		TlsKeyID:         sql.NullInt64{Int64: cfg.TLSKeyID, Valid: cfg.TLSKeyID != 0},
		SignCert:         nullStringFrom(cfg.SignCert),
		TlsCert:          nullStringFrom(cfg.TLSCert),
		CaCert:           nullStringFrom(cfg.CACert),
		TlsCaCert:        nullStringFrom(cfg.TLSCACert),
		Config:           configJSON,
		DeploymentConfig: sql.NullString{String: string(depCfgJSON), Valid: true},
	}); err != nil {
		return nil, fmt.Errorf("persist group deployment_config: %w", err)
	}

	// Create one child nodes row per committer role. Container names come
	// from cfg.{Role}Container so the existing per-role start/stop code
	// (StartCommitterRole) finds the right docker name without further
	// indirection. HostPort + MonitoringPort are mirrored onto the child
	// so the node detail view + metrics URL surface render with no extra
	// round-trip to the parent group.
	children := []struct {
		role           nodetypes.FabricXRole
		nodeType       nodetypes.NodeType
		container      string
		port           int
		monitoringPort int
	}{
		{nodetypes.FabricXRoleCommitterSidecar, nodetypes.NodeTypeFabricXCommitterSidecar, cfg.SidecarContainer, cfg.SidecarPort, cfg.SidecarMonitoringPort},
		{nodetypes.FabricXRoleCommitterCoordinator, nodetypes.NodeTypeFabricXCommitterCoordinator, cfg.CoordinatorContainer, cfg.CoordinatorPort, cfg.CoordinatorMonitoringPort},
		{nodetypes.FabricXRoleCommitterValidator, nodetypes.NodeTypeFabricXCommitterValidator, cfg.ValidatorContainer, cfg.ValidatorPort, cfg.ValidatorMonitoringPort},
		{nodetypes.FabricXRoleCommitterVerifier, nodetypes.NodeTypeFabricXCommitterVerifier, cfg.VerifierContainer, cfg.VerifierPort, cfg.VerifierMonitoringPort},
		{nodetypes.FabricXRoleCommitterQueryService, nodetypes.NodeTypeFabricXCommitterQueryService, cfg.QueryServiceContainer, cfg.QueryServicePort, cfg.QueryServiceMonitoringPort},
	}

	for _, c := range children {
		childName := grp.Name + "-" + string(c.role)
		childDep := nodetypes.FabricXChildDeploymentConfig{
			BaseDeploymentConfig: nodetypes.BaseDeploymentConfig{
				Type: "fabricx-child",
				Mode: "docker",
			},
			NodeGroupID:    id,
			Role:           c.role,
			ContainerName:  c.container,
			HostPort:       c.port,
			MonitoringPort: c.monitoringPort,
		}
		childDepJSON, err := json.Marshal(childDep)
		if err != nil {
			return nil, fmt.Errorf("marshal child deployment: %w", err)
		}

		childCfg := nodetypes.FabricXChildConfig{
			NodeGroupID: id,
			Role:        c.role,
			Name:        childName,
		}
		childCfgJSON, err := json.Marshal(childCfg)
		if err != nil {
			return nil, fmt.Errorf("marshal child config: %w", err)
		}

		stored, err := json.Marshal(nodetypes.StoredConfig{
			Type:   "fabricx-child",
			Config: childCfgJSON,
		})
		if err != nil {
			return nil, fmt.Errorf("marshal stored config envelope: %w", err)
		}

		newNode, err := s.db.CreateNode(ctx, &db.CreateNodeParams{
			Name:                 childName,
			Slug:                 slugify(childName),
			Platform:             string(nodetypes.PlatformFabricX),
			Status:               string(nodetypes.NodeStatusCreated),
			FabricOrganizationID: sql.NullInt64{Int64: *grp.OrganizationID, Valid: true},
			NodeType:             sql.NullString{String: string(c.nodeType), Valid: true},
			NodeConfig:           sql.NullString{String: string(stored), Valid: true},
			Endpoint:             nullStringFrom(fmt.Sprintf("%s:%d", cfg.ExternalIP, c.port)),
		})
		if err != nil {
			return nil, fmt.Errorf("create child %s: %w", c.role, err)
		}

		if _, err := s.db.UpdateNodeDeploymentConfig(ctx, &db.UpdateNodeDeploymentConfigParams{
			ID:               newNode.ID,
			DeploymentConfig: sql.NullString{String: string(childDepJSON), Valid: true},
		}); err != nil {
			return nil, fmt.Errorf("persist child %s deployment_config: %w", c.role, err)
		}

		if err := s.db.UpdateNodeGroupID(ctx, &db.UpdateNodeGroupIDParams{
			ID:          newNode.ID,
			NodeGroupID: sql.NullInt64{Int64: id, Valid: true},
		}); err != nil {
			return nil, fmt.Errorf("set node_group_id on child %s: %w", c.role, err)
		}
	}

	return s.Get(ctx, id)
}
