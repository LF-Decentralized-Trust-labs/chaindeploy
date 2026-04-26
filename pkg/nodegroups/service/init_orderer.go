// Package service — orderer-group initialization.
//
// InitOrdererGroup is the entry point that turns an empty node_groups row
// into a fully populated parent (with generated crypto + deployment_config)
// plus 4 per-role child `nodes` rows (router, batcher, consenter, assembler).
//
// After Init:
//   - node_groups.deployment_config carries the shared FabricXOrdererGroupDeploymentConfig
//   - node_groups.{sign,tls}_{key_id,cert}, ca_cert, tls_ca_cert are populated
//   - 4 child nodes exist with node_type=FABRICX_ORDERER_{ROUTER,BATCHER,CONSENTER,ASSEMBLER}
//     each carrying a thin FabricXChildDeploymentConfig envelope
//
// Starting the group via POST /node-groups/{id}/start then fans out to the
// existing per-role StartOrdererRole path without further orchestration.
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	ngtypes "github.com/chainlaunch/chainlaunch/pkg/nodegroups/types"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// OrdererInitInput carries the port allocations needed by the group's
// deployment config. Everything else (MSPID, org, partyID, externalIP,
// domain names, version) comes from the persisted node_groups row — we
// re-use them rather than forcing the caller to re-submit.
type OrdererInitInput struct {
	RouterPort    int    `json:"routerPort"`
	BatcherPort   int    `json:"batcherPort"`
	ConsenterPort int    `json:"consenterPort"`
	AssemblerPort int    `json:"assemblerPort"`
	ConsenterType string `json:"consenterType,omitempty"`
}

// InitOrdererGroup generates crypto, writes on-disk config, persists the
// resulting deployment_config + certs on the group, and creates one child
// nodes row per role. Idempotency: if the group already has deployment_config
// set and children exist, Init returns an error — callers should Delete and
// recreate rather than re-init.
func (s *Service) InitOrdererGroup(ctx context.Context, id int64, in OrdererInitInput) (*ngtypes.NodeGroup, error) {
	grp, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if grp.GroupType != ngtypes.GroupTypeFabricXOrderer {
		return nil, fmt.Errorf("group %d is %s; InitOrdererGroup is for %s only",
			id, grp.GroupType, ngtypes.GroupTypeFabricXOrderer)
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
	if grp.PartyID == nil {
		return nil, fmt.Errorf("group %d has no partyId; required for Init", id)
	}

	// Guard: no existing children.
	existing, err := s.db.ListNodesByGroup(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list existing children: %w", err)
	}
	if len(existing) > 0 {
		return nil, fmt.Errorf("group %d already has %d children; delete them before re-init", id, len(existing))
	}

	og := s.fabricxDeps.ordererFactory(s.db, 0, nodetypes.FabricXOrdererGroupConfig{
		Name:           grp.Name,
		OrganizationID: *grp.OrganizationID,
		MSPID:          grp.MSPID,
		PartyID:        int(*grp.PartyID),
		ExternalIP:     grp.ExternalIP,
		DomainNames:    grp.DomainNames,
		Version:        grp.Version,
		RouterPort:     in.RouterPort,
		BatcherPort:    in.BatcherPort,
		ConsenterPort:  in.ConsenterPort,
		AssemblerPort:  in.AssemblerPort,
		ConsenterType:  in.ConsenterType,
	})

	cfg, err := og.Init()
	if err != nil {
		s.markGroupError(ctx, id, err)
		return nil, fmt.Errorf("orderer Init: %w", err)
	}

	// Persist the group's shared deployment_config and crypto fingerprints.
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

	// Create one child per role with thin FabricXChildDeploymentConfig.
	// The container name comes from cfg.{Role}Container — those are the
	// docker names the fabricx start code will look up. HostPort is the
	// same published port (used by the nodes detail view for quick reference).
	children := []struct {
		role      nodetypes.FabricXRole
		nodeType  nodetypes.NodeType
		container string
		port      int
	}{
		{nodetypes.FabricXRoleOrdererRouter, nodetypes.NodeTypeFabricXOrdererRouter, cfg.RouterContainer, cfg.RouterPort},
		{nodetypes.FabricXRoleOrdererBatcher, nodetypes.NodeTypeFabricXOrdererBatcher, cfg.BatcherContainer, cfg.BatcherPort},
		{nodetypes.FabricXRoleOrdererConsenter, nodetypes.NodeTypeFabricXOrdererConsenter, cfg.ConsenterContainer, cfg.ConsenterPort},
		{nodetypes.FabricXRoleOrdererAssembler, nodetypes.NodeTypeFabricXOrdererAssembler, cfg.AssemblerContainer, cfg.AssemblerPort},
	}

	for _, c := range children {
		childName := grp.Name + "-" + string(c.role)
		childDep := nodetypes.FabricXChildDeploymentConfig{
			BaseDeploymentConfig: nodetypes.BaseDeploymentConfig{
				Type: "fabricx-child",
				Mode: "docker",
			},
			NodeGroupID:   id,
			Role:          c.role,
			ContainerName: c.container,
			HostPort:      c.port,
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

		// The nodes table uses a StoredConfig envelope; mirror the
		// monolithic path by wrapping the child config.
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

// slugify is a local helper used when creating child node rows. The
// fabricx package has its own slugify used for container names; this
// one only shapes the nodes.slug column. Kept independent so renaming
// there doesn't change DB slugs here.
func slugify(s string) string {
	out := strings.ToLower(s)
	out = strings.ReplaceAll(out, " ", "-")
	out = strings.ReplaceAll(out, "_", "-")
	return out
}
