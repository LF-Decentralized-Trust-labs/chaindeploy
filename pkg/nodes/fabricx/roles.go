package fabricx

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"

	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// This file exposes per-role Init/Start/Stop for FabricX children. Previously
// OrdererGroup.Start() and Committer.Start() started 4/5+1 containers in a
// single loop, conflating many `nodes` rows' worth of lifecycle into one.
//
// The node_groups refactor (ADR 0001) writes one child `nodes` row per role.
// StartNode on a child row calls into StartOrdererRole / StartCommitterRole
// here, passing just the role. All crypto/config generation remains on the
// group coordinator (OrdererGroup / Committer Init) and writes to a shared
// base directory; this file just picks the per-role container def and
// delegates to startContainer().

// ordererComponent holds the startup definition for a single orderer role.
type ordererComponent struct {
	role          nodetypes.FabricXRole
	containerName string
	hostPort      int
	cmd           []string
	// mountTag is the trailing path segment inside /etc/hyperledger/fabricx
	// where this component expects its msp/tls/genesis/store/data bind mounts
	// (matches the historical per-component layout: "router", "batcher",
	// "consenter", "assembler").
	mountTag string
}

// ordererComponents derives the per-role definitions from a group deployment
// config so callers can start any single role or all four.
func ordererComponents(cfg *nodetypes.FabricXOrdererGroupDeploymentConfig) []ordererComponent {
	return []ordererComponent{
		{
			role:          nodetypes.FabricXRoleOrdererRouter,
			containerName: cfg.RouterContainer,
			hostPort:      cfg.RouterPort,
			cmd:           []string{"--config=/etc/hyperledger/fabricx/config/node_config.yaml", "router"},
			mountTag:      "router",
		},
		{
			role:          nodetypes.FabricXRoleOrdererBatcher,
			containerName: cfg.BatcherContainer,
			hostPort:      cfg.BatcherPort,
			cmd:           []string{"--config=/etc/hyperledger/fabricx/config/node_config.yaml", "batcher"},
			mountTag:      "batcher",
		},
		{
			role:          nodetypes.FabricXRoleOrdererConsenter,
			containerName: cfg.ConsenterContainer,
			hostPort:      cfg.ConsenterPort,
			cmd:           []string{"--config=/etc/hyperledger/fabricx/config/node_config.yaml", "consensus"},
			mountTag:      "consenter",
		},
		{
			role:          nodetypes.FabricXRoleOrdererAssembler,
			containerName: cfg.AssemblerContainer,
			hostPort:      cfg.AssemblerPort,
			cmd:           []string{"--config=/etc/hyperledger/fabricx/config/node_config.yaml", "assembler"},
			mountTag:      "assembler",
		},
	}
}

// findOrdererComponent returns the component def for a role or an error if
// the role doesn't belong to an orderer group.
func findOrdererComponent(cfg *nodetypes.FabricXOrdererGroupDeploymentConfig, role nodetypes.FabricXRole) (ordererComponent, error) {
	for _, c := range ordererComponents(cfg) {
		if c.role == role {
			return c, nil
		}
	}
	return ordererComponent{}, fmt.Errorf("role %q is not an orderer group role", role)
}

// StartOrdererRole starts one container for an orderer-group role. Materials
// (MSP/TLS/genesis/config) must already exist in the group's base directory —
// the group coordinator's EnsureMaterials + config-writing is the caller's
// responsibility before the first role starts.
func (og *OrdererGroup) StartOrdererRole(
	ctx context.Context,
	cfg *nodetypes.FabricXOrdererGroupDeploymentConfig,
	role nodetypes.FabricXRole,
) error {
	comp, err := findOrdererComponent(cfg, role)
	if err != nil {
		return err
	}

	// See committer.go: legacy nodes pinned cfg.Version to docker-hub tag
	// "0.0.24" which does not exist at the new ghcr.io path. Promote
	// legacy pins to the current default image.
	version := cfg.Version
	if version == "" || version == "0.0.24" {
		version = DefaultOrdererVersion
	}
	imageName := fmt.Sprintf("%s:%s", DefaultOrdererImage, version)
	compDir := filepath.Join(og.baseDir(), comp.mountTag)

	env := map[string]string{}
	for k, v := range og.opts.Env {
		env[k] = v
	}

	mounts := []mount.Mount{
		{Type: mount.TypeBind, Source: filepath.Join(compDir, "config"), Target: "/etc/hyperledger/fabricx/config", ReadOnly: true},
		{Type: mount.TypeBind, Source: filepath.Join(compDir, "msp"), Target: fmt.Sprintf("/etc/hyperledger/fabricx/%s/msp", comp.mountTag), ReadOnly: true},
		{Type: mount.TypeBind, Source: filepath.Join(compDir, "tls"), Target: fmt.Sprintf("/etc/hyperledger/fabricx/%s/tls", comp.mountTag), ReadOnly: true},
		{Type: mount.TypeBind, Source: filepath.Join(compDir, "genesis"), Target: fmt.Sprintf("/etc/hyperledger/fabricx/%s/genesis", comp.mountTag), ReadOnly: true},
		{Type: mount.TypeBind, Source: filepath.Join(compDir, "store"), Target: fmt.Sprintf("/etc/hyperledger/fabricx/%s/store", comp.mountTag)},
		{Type: mount.TypeBind, Source: filepath.Join(compDir, "data"), Target: fmt.Sprintf("/etc/hyperledger/fabricx/%s/data", comp.mountTag)},
	}

	containerPort := nat.Port(fmt.Sprintf("%d/tcp", comp.hostPort))
	portBindings := map[nat.Port][]nat.PortBinding{
		containerPort: {{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", comp.hostPort)}},
	}

	var extraHosts []string
	if cfg.ExternalIP != "" && resolveLocalDevForNode(ctx, og.db, og.configService, og.nodeID) {
		extraHosts = []string{fmt.Sprintf("%s:host-gateway", cfg.ExternalIP)}
	}

	og.logger.Info("Starting FabricX orderer component",
		"role", role, "container", comp.containerName, "hostPort", comp.hostPort)

	if err := startContainer(ctx, og.logger, imageName, comp.containerName, comp.cmd, env, portBindings, mounts, "", extraHosts); err != nil {
		return fmt.Errorf("failed to start %s: %w", role, err)
	}
	return waitForContainer(ctx, comp.containerName, 30*time.Second)
}

// StopOrdererRole stops and removes one container. Safe to call when the
// container does not exist.
func (og *OrdererGroup) StopOrdererRole(ctx context.Context, cfg *nodetypes.FabricXOrdererGroupDeploymentConfig, role nodetypes.FabricXRole) error {
	comp, err := findOrdererComponent(cfg, role)
	if err != nil {
		return err
	}
	return stopContainer(ctx, comp.containerName)
}

// IsOrdererRoleRunning reports docker-level running state for a single role.
func (og *OrdererGroup) IsOrdererRoleRunning(ctx context.Context, cfg *nodetypes.FabricXOrdererGroupDeploymentConfig, role nodetypes.FabricXRole) (bool, error) {
	comp, err := findOrdererComponent(cfg, role)
	if err != nil {
		return false, err
	}
	return isContainerRunning(ctx, comp.containerName)
}

// PrepareOrdererStart performs the one-time per-group setup that must run
// before any role starts (directory tree, restored MSP/TLS, regenerated
// configs). The group coordinator calls this exactly once per StartGroup;
// per-role StartOrdererRole assumes it has already run.
func (og *OrdererGroup) PrepareOrdererStart(cfg *nodetypes.FabricXOrdererGroupDeploymentConfig) error {
	if err := og.ensureMaterials(cfg); err != nil {
		return fmt.Errorf("ensure orderer materials: %w", err)
	}
	if err := og.writeRouterConfig(cfg); err != nil {
		return fmt.Errorf("refresh router config: %w", err)
	}
	if err := og.writeBatcherConfig(cfg); err != nil {
		return fmt.Errorf("refresh batcher config: %w", err)
	}
	if err := og.writeConsenterConfig(cfg); err != nil {
		return fmt.Errorf("refresh consenter config: %w", err)
	}
	if err := og.writeAssemblerConfig(cfg); err != nil {
		return fmt.Errorf("refresh assembler config: %w", err)
	}
	return nil
}

// committerComponent holds the per-role definition for one of the five
// committer containers.
type committerComponent struct {
	role          nodetypes.FabricXRole
	containerName string
	hostPort      int
	cmd           []string
	subDir        string
	needsMSP      bool
	needsGenesis  bool
}

// committerComponents derives the 5 role definitions from a deployment cfg.
func committerComponents(cfg *nodetypes.FabricXCommitterDeploymentConfig) []committerComponent {
	return []committerComponent{
		{
			role:          nodetypes.FabricXRoleCommitterSidecar,
			containerName: cfg.SidecarContainer,
			hostPort:      cfg.SidecarPort,
			// v1.0.0-alpha restructured the CLI: `start-sidecar` → `start sidecar`
			// (see hyperledger/fabric-x-committer #491). Config flag path unchanged.
			cmd:           []string{"start", "sidecar", "--config=/etc/hyperledger/fabricx/config/sidecar_config.yaml"},
			subDir:        "sidecar",
			needsMSP:      true,
			needsGenesis:  true,
		},
		{
			role:          nodetypes.FabricXRoleCommitterCoordinator,
			containerName: cfg.CoordinatorContainer,
			hostPort:      cfg.CoordinatorPort,
			cmd:           []string{"start", "coordinator", "--config=/etc/hyperledger/fabricx/config/coordinator_config.yaml"},
			subDir:        "coordinator",
		},
		{
			role:          nodetypes.FabricXRoleCommitterValidator,
			containerName: cfg.ValidatorContainer,
			hostPort:      cfg.ValidatorPort,
			cmd:           []string{"start", "vc", "--config=/etc/hyperledger/fabricx/config/validator_config.yaml"},
			subDir:        "validator",
		},
		{
			role:          nodetypes.FabricXRoleCommitterVerifier,
			containerName: cfg.VerifierContainer,
			hostPort:      cfg.VerifierPort,
			cmd:           []string{"start", "verifier", "--config=/etc/hyperledger/fabricx/config/verifier_config.yaml"},
			subDir:        "verifier",
		},
		{
			role:          nodetypes.FabricXRoleCommitterQueryService,
			containerName: cfg.QueryServiceContainer,
			hostPort:      cfg.QueryServicePort,
			cmd:           []string{"start", "query", "--config=/etc/hyperledger/fabricx/config/config.yaml"},
			subDir:        "query-service",
		},
	}
}

func findCommitterComponent(cfg *nodetypes.FabricXCommitterDeploymentConfig, role nodetypes.FabricXRole) (committerComponent, error) {
	for _, c := range committerComponents(cfg) {
		if c.role == role {
			return c, nil
		}
	}
	return committerComponent{}, fmt.Errorf("role %q is not a committer role", role)
}

// CommitterNetworkName returns the docker bridge network shared by
// committer sibling containers for name-based service discovery.
func (c *Committer) CommitterNetworkName() string {
	return c.prefix() + "-net"
}

// StartCommitterRole starts one committer container. The group coordinator
// must have called PrepareCommitterStart first (ensures materials, creates
// the bridge network, and rewrites role configs against the current
// postgres endpoint).
func (c *Committer) StartCommitterRole(
	ctx context.Context,
	cfg *nodetypes.FabricXCommitterDeploymentConfig,
	role nodetypes.FabricXRole,
) error {
	comp, err := findCommitterComponent(cfg, role)
	if err != nil {
		return err
	}

	// Nodes created before the v1.0.0-alpha migration have cfg.Version pinned
	// to legacy "0.1.9" and pointed at the old docker-hub path. The new
	// DefaultCommitterImage (ghcr.io) has no 0.1.9 tag. Promote legacy-pin
	// nodes to the current default so they pull the ghcr.io image. Explicit
	// non-default versions are still honored.
	version := cfg.Version
	if version == "" || version == "0.1.9" {
		version = DefaultCommitterVersion
	}
	imageName := fmt.Sprintf("%s:%s", DefaultCommitterImage, version)
	compDir := filepath.Join(c.baseDir(), comp.subDir)

	env := map[string]string{}
	for k, v := range c.opts.Env {
		env[k] = v
	}

	mounts := []mount.Mount{
		{Type: mount.TypeBind, Source: filepath.Join(compDir, "config"), Target: "/etc/hyperledger/fabricx/config", ReadOnly: true},
		{Type: mount.TypeBind, Source: filepath.Join(compDir, "data"), Target: "/var/hyperledger/fabricx/data"},
	}
	if comp.needsMSP {
		mounts = append(mounts,
			mount.Mount{Type: mount.TypeBind, Source: filepath.Join(compDir, "msp"), Target: "/var/hyperledger/fabricx/msp", ReadOnly: true},
			mount.Mount{Type: mount.TypeBind, Source: filepath.Join(compDir, "tls"), Target: "/var/hyperledger/fabricx/tls", ReadOnly: true},
			mount.Mount{Type: mount.TypeBind, Source: filepath.Join(compDir, "ledger"), Target: "/var/hyperledger/fabricx/ledger"},
		)
	}
	if comp.needsGenesis {
		mounts = append(mounts,
			mount.Mount{Type: mount.TypeBind, Source: filepath.Join(compDir, "genesis"), Target: "/etc/hyperledger/fabricx/genesis", ReadOnly: true},
		)
	}

	containerPort := nat.Port(fmt.Sprintf("%d/tcp", comp.hostPort))
	portBindings := map[nat.Port][]nat.PortBinding{
		containerPort: {{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", comp.hostPort)}},
	}

	var extraHosts []string
	if cfg.ExternalIP != "" && resolveLocalDevForNode(ctx, c.db, c.configService, c.nodeID) {
		extraHosts = []string{fmt.Sprintf("%s:host-gateway", cfg.ExternalIP)}
	}

	c.logger.Info("Starting FabricX committer component",
		"role", role, "container", comp.containerName, "hostPort", comp.hostPort)

	if err := startContainer(ctx, c.logger, imageName, comp.containerName, comp.cmd, env, portBindings, mounts, c.CommitterNetworkName(), extraHosts); err != nil {
		return fmt.Errorf("failed to start %s: %w", role, err)
	}
	return waitForContainer(ctx, comp.containerName, 30*time.Second)
}

func (c *Committer) StopCommitterRole(ctx context.Context, cfg *nodetypes.FabricXCommitterDeploymentConfig, role nodetypes.FabricXRole) error {
	comp, err := findCommitterComponent(cfg, role)
	if err != nil {
		return err
	}
	return stopContainer(ctx, comp.containerName)
}

func (c *Committer) IsCommitterRoleRunning(ctx context.Context, cfg *nodetypes.FabricXCommitterDeploymentConfig, role nodetypes.FabricXRole) (bool, error) {
	comp, err := findCommitterComponent(cfg, role)
	if err != nil {
		return false, err
	}
	return isContainerRunning(ctx, comp.containerName)
}

// PrepareCommitterStart performs the one-time per-group setup before any
// committer role starts: ensure materials, create the bridge network, and
// rewrite every role's config against the current postgres endpoint.
//
// The postgres endpoint can be overridden by pgHost/pgPort when the
// committer's postgres has been externalized to a services row (see the
// node_groups coordinator). When both are zero the values on cfg are used
// verbatim.
func (c *Committer) PrepareCommitterStart(ctx context.Context, cfg *nodetypes.FabricXCommitterDeploymentConfig, pgHost string, pgPort int) error {
	if err := c.ensureMaterials(cfg); err != nil {
		return fmt.Errorf("ensure committer materials: %w", err)
	}
	if err := createDockerNetwork(ctx, c.CommitterNetworkName()); err != nil {
		return fmt.Errorf("create committer network: %w", err)
	}
	if pgHost != "" {
		cfg.PostgresHost = pgHost
	}
	if pgPort != 0 {
		cfg.PostgresPort = pgPort
	}
	// When a managed postgres service is used we mustn't dial via the old
	// per-committer postgres container — clear the container name so
	// postgresEndpoint falls through to (PostgresHost, PostgresPort).
	if pgHost != "" || pgPort != 0 {
		cfg.PostgresContainer = ""
	}
	if err := c.writeSidecarConfig(cfg); err != nil {
		return fmt.Errorf("refresh sidecar config: %w", err)
	}
	if err := c.writeCoordinatorConfig(cfg); err != nil {
		return fmt.Errorf("refresh coordinator config: %w", err)
	}
	if err := c.writeValidatorConfig(cfg); err != nil {
		return fmt.Errorf("refresh validator config: %w", err)
	}
	if err := c.writeVerifierConfig(cfg); err != nil {
		return fmt.Errorf("refresh verifier config: %w", err)
	}
	if err := c.writeQueryServiceConfig(cfg); err != nil {
		return fmt.Errorf("refresh query-service config: %w", err)
	}
	return nil
}
