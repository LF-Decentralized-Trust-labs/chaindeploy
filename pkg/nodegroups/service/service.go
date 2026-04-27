// Package service coordinates node_groups — the shared-identity parents
// that own per-role FabricX child nodes. See ADR 0001 for the full
// design.
//
// The coordinator is intentionally thin: it translates DB rows into
// typed domain structs, drives the Prepare → Start sequence across the
// fabricx package, and fans out per-child Start/Stop through the
// existing node service so one code path handles both per-child
// StartNode requests and group-level lifecycle.
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	ngtypes "github.com/chainlaunch/chainlaunch/pkg/nodegroups/types"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/fabricx"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	pgservice "github.com/chainlaunch/chainlaunch/pkg/services/postgres"
)

// NodeLifecycle is the narrow subset of *nodeservice.NodeService the
// coordinator uses to start / stop individual child nodes. Pulled out
// as an interface so StartGroup / StopGroup fan-out ordering can be
// verified with a recording fake in unit tests. The production
// *nodeservice.NodeService satisfies it as-is.
type NodeLifecycle interface {
	StartNode(ctx context.Context, id int64) (*nodeservice.NodeResponse, error)
	StopNode(ctx context.Context, id int64) (*nodeservice.NodeResponse, error)
}

// PostgresLifecycle is the narrow subset of pkg/services/postgres the
// coordinator uses. Pulled behind an interface so unit tests can record
// Start/Stop calls without touching docker. Production impl is
// postgresAdapter in this package wrapping the package-level funcs.
//
// IsRunning / ConnectNetwork / DisconnectNetwork are what make a single
// managed postgres service shareable across N committer groups: if the
// container is already running for service S, the coordinator just
// attaches the new committer's bridge network instead of restarting the
// container. Group teardown disconnects that network but leaves the
// container up — explicit /services/{id}/stop is the only path that
// actually drops postgres.
type PostgresLifecycle interface {
	Start(ctx context.Context, cfg pgservice.Config) (containerID string, err error)
	Stop(ctx context.Context, containerName string) error
	IsRunning(ctx context.Context, containerName string) (bool, error)
	ConnectNetwork(ctx context.Context, containerName, networkName string) error
	DisconnectNetwork(ctx context.Context, containerName, networkName string) error
}

// Service is the node_groups coordinator. It depends on the node
// service for per-child lifecycle and on the DB for persistence.
//
// The fabricx *OrdererGroup / *Committer instances needed by Prepare*
// are constructed on demand; the coordinator does not cache them
// because they are stateless wrappers around the opts + DB handle.
type Service struct {
	db          *db.Queries
	nodeService NodeLifecycle
	postgres    PostgresLifecycle
	fabricxDeps fabricxDeps
	logger      *logger.Logger
	// dataPath is the chainlaunch on-disk root; postgres bind mounts
	// land under ${dataPath}/services/postgres/<container>. Empty means
	// "no bind" (tests / legacy installs).
	dataPath string
}

// fabricxDeps is the subset of dependencies fabricx.NewOrdererGroup /
// NewCommitter require. Passed in by the composition root (serve.go)
// so this package does not reach into org/key management itself.
type fabricxDeps struct {
	ordererFactory   func(dbQueries *db.Queries, nodeID int64, opts nodetypes.FabricXOrdererGroupConfig) *fabricx.OrdererGroup
	committerFactory func(dbQueries *db.Queries, nodeID int64, opts nodetypes.FabricXCommitterConfig) *fabricx.Committer
}

// NewService wires the coordinator.
//
// ordererFactory and committerFactory produce fabricx orchestrators
// with the heavy dependencies (org service, key service, config
// service, logger) already baked in. The coordinator only supplies the
// per-request db handle, node ID, and opts.
func NewService(
	dbQueries *db.Queries,
	nodeService NodeLifecycle,
	ordererFactory func(dbQueries *db.Queries, nodeID int64, opts nodetypes.FabricXOrdererGroupConfig) *fabricx.OrdererGroup,
	committerFactory func(dbQueries *db.Queries, nodeID int64, opts nodetypes.FabricXCommitterConfig) *fabricx.Committer,
	log *logger.Logger,
) *Service {
	return &Service{
		db:          dbQueries,
		nodeService: nodeService,
		postgres:    defaultPostgresAdapter{log: log},
		fabricxDeps: fabricxDeps{
			ordererFactory:   ordererFactory,
			committerFactory: committerFactory,
		},
		logger: log,
	}
}

// WithPostgresLifecycle swaps the default (docker-backed) postgres
// adapter for a caller-supplied implementation. Used by tests to avoid
// docker; production code can leave it alone.
func (s *Service) WithPostgresLifecycle(pl PostgresLifecycle) *Service {
	s.postgres = pl
	return s
}

// WithDataPath enables on-host bind mounts for managed postgres data
// directories. Required for backups to capture committer/queryservice
// state — without it, postgres state lives only in the container's
// writable layer. Production wires this in serve.go; tests pass "".
func (s *Service) WithDataPath(dataPath string) *Service {
	s.dataPath = dataPath
	return s
}

// defaultPostgresAdapter routes PostgresLifecycle calls to the
// package-level pkg/services/postgres helpers. Kept private so the
// default wiring lives in one place and the public API stays on the
// PostgresLifecycle interface.
type defaultPostgresAdapter struct {
	log *logger.Logger
}

func (a defaultPostgresAdapter) Start(ctx context.Context, cfg pgservice.Config) (string, error) {
	return pgservice.Start(ctx, a.log, cfg)
}

func (a defaultPostgresAdapter) Stop(ctx context.Context, containerName string) error {
	return pgservice.Stop(ctx, containerName)
}

func (a defaultPostgresAdapter) IsRunning(ctx context.Context, containerName string) (bool, error) {
	return pgservice.IsRunning(ctx, containerName)
}

func (a defaultPostgresAdapter) ConnectNetwork(ctx context.Context, containerName, networkName string) error {
	return pgservice.ConnectNetwork(ctx, containerName, networkName)
}

func (a defaultPostgresAdapter) DisconnectNetwork(ctx context.Context, containerName, networkName string) error {
	return pgservice.DisconnectNetwork(ctx, containerName, networkName)
}

// CreateInput carries the fields needed to persist a new node_group row.
// The coordinator does not generate certs/keys here; that runs on the
// first Start call (via fabricx.Init) so the CRUD endpoint stays cheap.
type CreateInput struct {
	Name           string
	Platform       string
	GroupType      ngtypes.GroupType
	MSPID          string
	OrganizationID *int64
	PartyID        *int64
	Version        string
	ExternalIP     string
	DomainNames    []string
	Config         json.RawMessage
}

// Create persists a new node_group with CREATED status. Returns the
// hydrated domain object.
func (s *Service) Create(ctx context.Context, in CreateInput) (*ngtypes.NodeGroup, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if in.Platform == "" {
		return nil, fmt.Errorf("platform is required")
	}
	if in.GroupType == "" {
		return nil, fmt.Errorf("groupType is required")
	}

	domains := sql.NullString{}
	if len(in.DomainNames) > 0 {
		b, err := json.Marshal(in.DomainNames)
		if err != nil {
			return nil, fmt.Errorf("marshal domainNames: %w", err)
		}
		domains = sql.NullString{String: string(b), Valid: true}
	}
	cfg := sql.NullString{}
	if len(in.Config) > 0 {
		cfg = sql.NullString{String: string(in.Config), Valid: true}
	}

	row, err := s.db.CreateNodeGroup(ctx, &db.CreateNodeGroupParams{
		Name:           in.Name,
		Platform:       in.Platform,
		GroupType:      string(in.GroupType),
		MspID:          nullStringFrom(in.MSPID),
		OrganizationID: nullInt64FromPtr(in.OrganizationID),
		PartyID:        nullInt64FromPtr(in.PartyID),
		Version:        nullStringFrom(in.Version),
		ExternalIp:     nullStringFrom(in.ExternalIP),
		DomainNames:    domains,
		Config:         cfg,
		Status:         string(nodetypes.NodeStatusCreated),
	})
	if err != nil {
		return nil, fmt.Errorf("persist node_group: %w", err)
	}
	return hydrateRow(row), nil
}

// Get returns the hydrated group by ID, or ErrNotFound wrapped.
func (s *Service) Get(ctx context.Context, id int64) (*ngtypes.NodeGroup, error) {
	row, err := s.db.GetNodeGroup(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load node_group %d: %w", id, err)
	}
	return hydrateRow(row), nil
}

// List returns all groups, paginated.
func (s *Service) List(ctx context.Context, limit, offset int32) ([]*ngtypes.NodeGroup, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.ListNodeGroups(ctx, &db.ListNodeGroupsParams{Limit: int64(limit), Offset: int64(offset)})
	if err != nil {
		return nil, fmt.Errorf("list node_groups: %w", err)
	}
	out := make([]*ngtypes.NodeGroup, 0, len(rows))
	for _, row := range rows {
		out = append(out, hydrateRow(row))
	}
	return out, nil
}

// Delete removes the node_group row. Children must be removed
// separately via the nodes API — the FK is ON DELETE SET NULL so child
// rows remain but lose their group reference. Callers should stop
// children first.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.db.DeleteNodeGroup(ctx, id)
}

// CreatePostgresInput is the coordinator-facing payload for attaching a
// managed postgres service to a node_group. Credentials are required;
// Version/HostPort default to sensible values when omitted.
type CreatePostgresInput struct {
	Name     string
	Version  string
	DB       string
	User     string
	Password string
	HostPort int
}

// CreatePostgresService persists a POSTGRES services row attached to
// the given node_group. The row is created in CREATED state — the
// container is not started until the next StartGroup call.
func (s *Service) CreatePostgresService(ctx context.Context, nodeGroupID int64, in CreatePostgresInput) (*ngtypes.NodeGroupService, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if in.DB == "" || in.User == "" || in.Password == "" {
		return nil, fmt.Errorf("db, user and password are required")
	}
	// Guard: group must exist and be a committer group. Orderer groups
	// do not take managed postgres.
	grp, err := s.Get(ctx, nodeGroupID)
	if err != nil {
		return nil, err
	}
	if grp.GroupType != ngtypes.GroupTypeFabricXCommitter {
		return nil, fmt.Errorf("group %d is %s; only %s groups accept postgres services",
			nodeGroupID, grp.GroupType, ngtypes.GroupTypeFabricXCommitter)
	}

	cfg := ngtypes.PostgresServiceConfig{
		Version:  in.Version,
		DB:       in.DB,
		User:     in.User,
		Password: in.Password,
		HostPort: in.HostPort,
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal postgres config: %w", err)
	}

	row, err := s.db.CreateService(ctx, &db.CreateServiceParams{
		NodeGroupID: sql.NullInt64{Int64: nodeGroupID, Valid: true},
		Name:        in.Name,
		ServiceType: string(ngtypes.ServiceTypePostgres),
		Version:     nullStringFrom(in.Version),
		Status:      string(nodetypes.NodeStatusCreated),
		Config:      sql.NullString{String: string(cfgJSON), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("persist service: %w", err)
	}
	// Also set node_groups.postgres_service_id so the group resolves the
	// same service through the new pointer. Writing both columns keeps
	// legacy callers (that still read services.node_group_id) and the new
	// coordinator (that follows the pointer) in sync during the
	// deprecation window.
	if _, err := s.db.UpdateNodeGroupPostgresServiceID(ctx, &db.UpdateNodeGroupPostgresServiceIDParams{
		ID:                nodeGroupID,
		PostgresServiceID: sql.NullInt64{Int64: row.ID, Valid: true},
	}); err != nil {
		s.logger.Warn("failed to set postgres_service_id", "group", nodeGroupID, "service", row.ID, "err", err)
	}
	return hydrateServiceRow(row), nil
}

// GetService returns the hydrated service by ID.
func (s *Service) GetService(ctx context.Context, id int64) (*ngtypes.NodeGroupService, error) {
	row, err := s.db.GetService(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load service %d: %w", id, err)
	}
	return hydrateServiceRow(row), nil
}

// ListServicesForGroup returns all services attached to the given
// node_group, in creation order.
//
// During the deprecation window this merges two sources: (a) services
// whose legacy node_group_id column matches, and (b) the service
// referenced by node_groups.postgres_service_id. Once the legacy column
// is dropped this collapses to (b) only.
func (s *Service) ListServicesForGroup(ctx context.Context, nodeGroupID int64) ([]*ngtypes.NodeGroupService, error) {
	rows, err := s.db.ListServicesByNodeGroup(ctx, sql.NullInt64{Int64: nodeGroupID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list services for group %d: %w", nodeGroupID, err)
	}
	seen := make(map[int64]struct{}, len(rows))
	out := make([]*ngtypes.NodeGroupService, 0, len(rows)+1)
	for _, row := range rows {
		seen[row.ID] = struct{}{}
		out = append(out, hydrateServiceRow(row))
	}

	// Also include the service referenced by postgres_service_id, even
	// if its legacy node_group_id is NULL (the new, post-refactor shape).
	pgID, err := s.db.GetNodeGroupPostgresServiceID(ctx, nodeGroupID)
	if err == nil && pgID.Valid {
		if _, already := seen[pgID.Int64]; !already {
			if row, err := s.db.GetService(ctx, pgID.Int64); err == nil {
				out = append(out, hydrateServiceRow(row))
			}
		}
	}
	return out, nil
}

// Child is a thin projection of a node row owned by a group, shaped
// for the UI: enough to render a status list without leaking every DB
// column. Platform/config payloads stay in /nodes/{id} for drill-down.
type Child struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	NodeType     string  `json:"nodeType,omitempty"`
	Status       string  `json:"status"`
	Endpoint     string  `json:"endpoint,omitempty"`
	ErrorMessage string  `json:"errorMessage,omitempty"`
	CreatedAt    string  `json:"createdAt,omitempty"`
	UpdatedAt    string  `json:"updatedAt,omitempty"`
}

// ListChildren returns the nodes owned by the given group in canonical
// role order (the same order StartGroup uses). Unknown roles are
// appended after the known ones in creation order so nothing is lost.
func (s *Service) ListChildren(ctx context.Context, nodeGroupID int64) ([]Child, error) {
	grp, err := s.Get(ctx, nodeGroupID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.ListNodesByGroup(ctx, sql.NullInt64{Int64: nodeGroupID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list children of node_group %d: %w", nodeGroupID, err)
	}

	// Index rows by NodeType for canonical ordering.
	byType := make(map[nodetypes.NodeType]*db.Node, len(rows))
	var unknown []*db.Node
	for _, r := range rows {
		if r.NodeType.Valid {
			byType[nodetypes.NodeType(r.NodeType.String)] = r
		} else {
			unknown = append(unknown, r)
		}
	}

	order := ngtypes.ChildRoles(grp.GroupType)
	out := make([]Child, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, role := range order {
		if r, ok := byType[role]; ok {
			out = append(out, hydrateChild(r))
			seen[r.ID] = struct{}{}
		}
	}
	// Append any known-type rows that weren't in the canonical order
	// (defensive — shouldn't happen in practice) plus unknown-type rows.
	for _, r := range rows {
		if _, already := seen[r.ID]; already {
			continue
		}
		out = append(out, hydrateChild(r))
	}
	_ = unknown
	return out, nil
}

func hydrateChild(r *db.Node) Child {
	c := Child{
		ID:     r.ID,
		Name:   r.Name,
		Status: r.Status,
	}
	if r.NodeType.Valid {
		c.NodeType = r.NodeType.String
	}
	if r.Endpoint.Valid {
		c.Endpoint = r.Endpoint.String
	}
	if r.ErrorMessage.Valid {
		c.ErrorMessage = r.ErrorMessage.String
	}
	c.CreatedAt = r.CreatedAt.Format(time.RFC3339)
	if r.UpdatedAt.Valid {
		c.UpdatedAt = r.UpdatedAt.Time.Format(time.RFC3339)
	}
	return c
}

// AttachPostgresService sets node_groups.postgres_service_id to the
// given service. Validates: group is a committer, service exists and is
// POSTGRES type. Idempotent: attaching the same service twice is a
// no-op. Clears any previous attachment without stopping the old
// container — callers must stop the group first if they want a clean
// swap.
func (s *Service) AttachPostgresService(ctx context.Context, nodeGroupID, serviceID int64) (*ngtypes.NodeGroup, error) {
	grp, err := s.Get(ctx, nodeGroupID)
	if err != nil {
		return nil, err
	}
	if grp.GroupType != ngtypes.GroupTypeFabricXCommitter {
		return nil, fmt.Errorf("group %d is %s; only %s groups accept postgres services",
			nodeGroupID, grp.GroupType, ngtypes.GroupTypeFabricXCommitter)
	}
	svc, err := s.db.GetService(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("load service %d: %w", serviceID, err)
	}
	if svc.ServiceType != string(ngtypes.ServiceTypePostgres) {
		return nil, fmt.Errorf("service %d is %s; only POSTGRES services can be attached", serviceID, svc.ServiceType)
	}
	row, err := s.db.UpdateNodeGroupPostgresServiceID(ctx, &db.UpdateNodeGroupPostgresServiceIDParams{
		ID:                nodeGroupID,
		PostgresServiceID: sql.NullInt64{Int64: serviceID, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("attach service: %w", err)
	}
	return hydrateRow(row), nil
}

// DetachPostgresService clears the postgres_service_id pointer on a
// node_group. Does not stop the service — that's a separate lifecycle
// concern owned by the services coordinator.
func (s *Service) DetachPostgresService(ctx context.Context, nodeGroupID int64) (*ngtypes.NodeGroup, error) {
	row, err := s.db.UpdateNodeGroupPostgresServiceID(ctx, &db.UpdateNodeGroupPostgresServiceIDParams{
		ID:                nodeGroupID,
		PostgresServiceID: sql.NullInt64{},
	})
	if err != nil {
		return nil, fmt.Errorf("detach service: %w", err)
	}
	return hydrateRow(row), nil
}

// StartGroup runs the full group startup sequence: Prepare (materials
// + configs + network) once, then per-role Start in canonical order.
//
// For FABRICX_COMMITTER groups this also starts any POSTGRES service
// attached to the group first and passes its external address to the
// committer Prepare call, so the coordinator-managed postgres replaces
// the embedded one.
func (s *Service) StartGroup(ctx context.Context, id int64) error {
	grp, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if _, err := s.db.UpdateNodeGroupStatus(ctx, &db.UpdateNodeGroupStatusParams{
		ID:     id,
		Status: string(nodetypes.NodeStatusStarting),
	}); err != nil {
		s.logger.Warn("failed to update node_group status to STARTING", "id", id, "err", err)
	}

	if err := s.prepareGroup(ctx, grp); err != nil {
		s.markGroupError(ctx, id, err)
		return err
	}

	// Fan out: start each child in the group in canonical role order.
	children, err := s.db.ListNodesByGroup(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		err = fmt.Errorf("list children of node_group %d: %w", id, err)
		s.markGroupError(ctx, id, err)
		return err
	}

	order := ngtypes.ChildRoles(grp.GroupType)
	byType := map[nodetypes.NodeType]*db.Node{}
	for _, c := range children {
		if c.NodeType.Valid {
			byType[nodetypes.NodeType(c.NodeType.String)] = c
		}
	}

	var firstErr error
	for _, role := range order {
		child, ok := byType[role]
		if !ok {
			continue
		}
		if _, err := s.nodeService.StartNode(ctx, child.ID); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("start child %d (%s): %w", child.ID, role, err)
		}
	}

	if firstErr != nil {
		s.markGroupError(ctx, id, firstErr)
		return firstErr
	}

	if _, err := s.db.UpdateNodeGroupStatus(ctx, &db.UpdateNodeGroupStatusParams{
		ID:     id,
		Status: string(nodetypes.NodeStatusRunning),
	}); err != nil {
		s.logger.Warn("failed to update node_group status to RUNNING", "id", id, "err", err)
	}
	return nil
}

// StopGroup stops children in reverse of the canonical start order so
// sidecar/router (the public-facing ones) drain before coordinator/
// consenter. Best-effort: logs per-child failures but attempts the full
// teardown.
func (s *Service) StopGroup(ctx context.Context, id int64) error {
	grp, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if _, err := s.db.UpdateNodeGroupStatus(ctx, &db.UpdateNodeGroupStatusParams{
		ID:     id,
		Status: string(nodetypes.NodeStatusStopping),
	}); err != nil {
		s.logger.Warn("failed to update node_group status to STOPPING", "id", id, "err", err)
	}

	children, err := s.db.ListNodesByGroup(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		return fmt.Errorf("list children: %w", err)
	}

	order := ngtypes.ChildRoles(grp.GroupType)
	byType := map[nodetypes.NodeType]*db.Node{}
	for _, c := range children {
		if c.NodeType.Valid {
			byType[nodetypes.NodeType(c.NodeType.String)] = c
		}
	}
	// Stop in reverse start order.
	for i := len(order) - 1; i >= 0; i-- {
		child, ok := byType[order[i]]
		if !ok {
			continue
		}
		if _, err := s.nodeService.StopNode(ctx, child.ID); err != nil {
			s.logger.Error("stop child failed", "child", child.ID, "role", order[i], "err", err)
		}
	}

	// After children are down, detach this group's committer bridge
	// network from the shared postgres container so other groups that
	// share the service keep working. The postgres container is only
	// fully stopped via the explicit services Stop endpoint — group
	// teardown never drops it. Committer groups only: orderer groups
	// don't take postgres.
	if grp.GroupType == ngtypes.GroupTypeFabricXCommitter {
		network, err := s.committerNetworkForGroup(grp)
		if err != nil {
			s.logger.Warn("derive committer network name; skipping postgres disconnect",
				"group", id, "err", err)
		} else {
			s.stopManagedPostgresForCommitter(ctx, id, network)
		}
	}

	if _, err := s.db.UpdateNodeGroupStatus(ctx, &db.UpdateNodeGroupStatusParams{
		ID:     id,
		Status: string(nodetypes.NodeStatusStopped),
	}); err != nil {
		s.logger.Warn("failed to update node_group status to STOPPED", "id", id, "err", err)
	}
	return nil
}

// RestartGroup stops then starts a group. Simple sequence: we do not
// preserve individual child state.
func (s *Service) RestartGroup(ctx context.Context, id int64) error {
	if err := s.StopGroup(ctx, id); err != nil {
		return err
	}
	// Brief pause between stop and start matches the existing node restart.
	time.Sleep(2 * time.Second)
	return s.StartGroup(ctx, id)
}

// prepareGroup runs Prepare{Orderer,Committer}Start on the in-memory
// fabricx orchestrator. Requires the group's deployment_config to
// already exist (populated by an earlier Init call — typically when
// the group's first child was created).
func (s *Service) prepareGroup(ctx context.Context, grp *ngtypes.NodeGroup) error {
	switch grp.GroupType {
	case ngtypes.GroupTypeFabricXOrderer:
		var cfg nodetypes.FabricXOrdererGroupDeploymentConfig
		if len(grp.DeploymentConfig) == 0 {
			return fmt.Errorf("node_group %d has no deployment_config; Init required", grp.ID)
		}
		if err := json.Unmarshal(grp.DeploymentConfig, &cfg); err != nil {
			return fmt.Errorf("unmarshal orderer deployment config: %w", err)
		}
		og := s.fabricxDeps.ordererFactory(s.db, 0, nodetypes.FabricXOrdererGroupConfig{
			Name:           grp.Name,
			OrganizationID: cfg.OrganizationID,
			MSPID:          cfg.MSPID,
			PartyID:        cfg.PartyID,
			ExternalIP:     cfg.ExternalIP,
			DomainNames:    cfg.DomainNames,
			Version:        cfg.Version,
		})
		return og.PrepareOrdererStart(&cfg)

	case ngtypes.GroupTypeFabricXCommitter:
		var cfg nodetypes.FabricXCommitterDeploymentConfig
		if len(grp.DeploymentConfig) == 0 {
			return fmt.Errorf("node_group %d has no deployment_config; Init required", grp.ID)
		}
		if err := json.Unmarshal(grp.DeploymentConfig, &cfg); err != nil {
			return fmt.Errorf("unmarshal committer deployment config: %w", err)
		}

		c := s.fabricxDeps.committerFactory(s.db, 0, nodetypes.FabricXCommitterConfig{
			Name:             grp.Name,
			OrganizationID:   cfg.OrganizationID,
			MSPID:            cfg.MSPID,
			ExternalIP:       cfg.ExternalIP,
			DomainNames:      cfg.DomainNames,
			Version:          cfg.Version,
			OrdererEndpoints: cfg.OrdererEndpoints,
			PostgresHost:     cfg.PostgresHost,
			PostgresPort:     cfg.PostgresPort,
			PostgresDB:       cfg.PostgresDB,
			PostgresUser:     cfg.PostgresUser,
			PostgresPassword: cfg.PostgresPassword,
			ChannelID:        cfg.ChannelID,
		})

		// Start any managed POSTGRES service attached to this group on
		// the committer's bridge network so siblings can dial it by
		// container name. Populates deployment_config so subsequent
		// calls skip re-starting when the container is already running.
		if err := s.startManagedPostgresForCommitter(ctx, grp.ID, c.CommitterNetworkName()); err != nil {
			return err
		}

		// If a POSTGRES service exists for this group, use its host/port
		// and clear the embedded container fallback so siblings dial the
		// managed postgres. Otherwise fall through and let the legacy
		// per-committer postgres spin up inside Committer.Start.
		pgHost, pgPort, err := s.resolveManagedPostgres(ctx, grp.ID)
		if err != nil {
			return err
		}

		return c.PrepareCommitterStart(ctx, &cfg, pgHost, pgPort)

	default:
		return fmt.Errorf("unknown group type %q", grp.GroupType)
	}
}

// findPostgresService returns the POSTGRES services row for the group,
// or nil when none is attached.
//
// Resolution order:
//  1. node_groups.postgres_service_id pointer (the post-refactor shape).
//  2. Legacy: first POSTGRES row with services.node_group_id == groupID.
//
// Step 2 stays for the deprecation window so existing installs without
// the pointer set continue to resolve their attached service.
func (s *Service) findPostgresService(ctx context.Context, groupID int64) (*db.Service, error) {
	if pgID, err := s.db.GetNodeGroupPostgresServiceID(ctx, groupID); err == nil && pgID.Valid {
		svc, err := s.db.GetService(ctx, pgID.Int64)
		if err == nil && svc.ServiceType == string(ngtypes.ServiceTypePostgres) {
			return svc, nil
		}
		if err != nil {
			s.logger.Warn("postgres_service_id points to missing service; falling back to legacy scan",
				"group", groupID, "service", pgID.Int64, "err", err)
		}
	}
	services, err := s.db.ListServicesByNodeGroup(ctx, sql.NullInt64{Int64: groupID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list services for group %d: %w", groupID, err)
	}
	for _, svc := range services {
		if svc.ServiceType == string(ngtypes.ServiceTypePostgres) {
			return svc, nil
		}
	}
	return nil, nil
}

// startManagedPostgresForCommitter ensures the POSTGRES service
// attached to the given committer group is reachable on the committer's
// bridge network.
//
// Sharing model (Option X — the single postgres container adapts to
// consumers, not the other way around):
//
//   - If the postgres container is already running (another group
//     started it first), just attach `networkName` via docker
//     NetworkConnect. deployment_config stays as-is — siblings already
//     have a stable Host/Port from the first start.
//   - If it is not running yet, full Start on the caller's network,
//     then persist deployment_config and flip status to RUNNING.
//
// No-op when no postgres service is attached — the committer falls
// through to its legacy embedded postgres path.
//
// Called from prepareGroup before fabricx.PrepareCommitterStart so the
// resolved endpoint is available to resolveManagedPostgres and the
// bridge network already contains the postgres container when committer
// roles boot.
func (s *Service) startManagedPostgresForCommitter(ctx context.Context, groupID int64, networkName string) error {
	svc, err := s.findPostgresService(ctx, groupID)
	if err != nil {
		return err
	}
	if svc == nil {
		return nil
	}

	if !svc.Config.Valid || svc.Config.String == "" {
		return fmt.Errorf("postgres service %d has no config; recreate with credentials", svc.ID)
	}
	var pgCfg ngtypes.PostgresServiceConfig
	if err := json.Unmarshal([]byte(svc.Config.String), &pgCfg); err != nil {
		return fmt.Errorf("unmarshal postgres service %d config: %w", svc.ID, err)
	}
	if pgCfg.DB == "" || pgCfg.User == "" || pgCfg.Password == "" {
		return fmt.Errorf("postgres service %d config missing db/user/password", svc.ID)
	}

	containerName := postgresContainerName(svc)

	// Fast path: service is already running (another committer group
	// started it). Just attach this committer's bridge network so its
	// roles can dial postgres by container name — no restart, no
	// deployment_config churn.
	running, err := s.postgres.IsRunning(ctx, containerName)
	if err != nil {
		s.logger.Warn("postgres IsRunning check failed; attempting full start", "service", svc.ID, "err", err)
	}
	if running {
		if err := s.postgres.ConnectNetwork(ctx, containerName, networkName); err != nil {
			s.markServiceError(ctx, svc.ID, err)
			return fmt.Errorf("attach postgres service %d to network %s: %w", svc.ID, networkName, err)
		}
		return nil
	}

	if _, err := s.db.UpdateServiceStatus(ctx, &db.UpdateServiceStatusParams{
		ID:     svc.ID,
		Status: string(nodetypes.NodeStatusStarting),
	}); err != nil {
		s.logger.Warn("failed to update service status to STARTING", "id", svc.ID, "err", err)
	}

	if _, err := s.postgres.Start(ctx, pgservice.Config{
		ContainerName: containerName,
		NetworkName:   networkName,
		HostPort:      pgCfg.HostPort,
		Version:       pgCfg.Version,
		DB:            pgCfg.DB,
		User:          pgCfg.User,
		Password:      pgCfg.Password,
		DataDir:       postgresDataDir(s.dataPath, containerName),
	}); err != nil {
		s.markServiceError(ctx, svc.ID, err)
		return fmt.Errorf("start postgres service %d: %w", svc.ID, err)
	}

	deployment := ngtypes.PostgresServiceDeployment{
		// Committer siblings on the same bridge network dial postgres
		// by container name on port 5432 — not via the published host
		// port. The host port is only for human access / debugging.
		//
		// Host/Port is stable across consumers: the container name is
		// derived from the services row, so later consumers joining via
		// ConnectNetwork dial the same endpoint even though they came
		// in on a different bridge.
		Host:          containerName,
		Port:          5432,
		ContainerName: containerName,
		NetworkName:   networkName,
	}
	depJSON, err := json.Marshal(deployment)
	if err != nil {
		s.markServiceError(ctx, svc.ID, err)
		return fmt.Errorf("marshal postgres deployment config: %w", err)
	}
	if _, err := s.db.UpdateServiceDeploymentConfig(ctx, &db.UpdateServiceDeploymentConfigParams{
		ID:               svc.ID,
		DeploymentConfig: sql.NullString{String: string(depJSON), Valid: true},
	}); err != nil {
		s.markServiceError(ctx, svc.ID, err)
		return fmt.Errorf("persist postgres deployment config: %w", err)
	}
	if _, err := s.db.UpdateServiceStatus(ctx, &db.UpdateServiceStatusParams{
		ID:     svc.ID,
		Status: string(nodetypes.NodeStatusRunning),
	}); err != nil {
		s.logger.Warn("failed to update service status to RUNNING", "id", svc.ID, "err", err)
	}
	return nil
}

// stopManagedPostgresForCommitter detaches the given group's committer
// bridge network from the shared postgres container. The container is
// left running so other consumers can keep using it — a fully stopped
// postgres is only triggered by the explicit services Stop endpoint.
//
// Best-effort: logs and swallows errors so group teardown completes
// even when docker plumbing has drifted. `networkName` is the owning
// committer's bridge network (derived from mspID+name inside fabricx);
// an empty value skips the disconnect so callers that don't know the
// network (early errors before Prepare) don't trip here.
func (s *Service) stopManagedPostgresForCommitter(ctx context.Context, groupID int64, networkName string) {
	svc, err := s.findPostgresService(ctx, groupID)
	if err != nil {
		s.logger.Warn("find postgres service during stop", "group", groupID, "err", err)
		return
	}
	if svc == nil {
		return
	}
	if networkName == "" {
		return
	}
	containerName := postgresContainerName(svc)
	if err := s.postgres.DisconnectNetwork(ctx, containerName, networkName); err != nil {
		s.logger.Warn("disconnect postgres from committer network",
			"service", svc.ID, "container", containerName, "network", networkName, "err", err)
	}
}

// committerNetworkForGroup reconstructs the docker bridge network name
// fabricx assigns a committer group's siblings. The derivation must
// match fabricx.Committer.CommitterNetworkName() — which is
// `fabricx-<lower(mspID)>-<slug(name)>-net` — so a refactor over there
// must update this in lockstep. Keeping the logic here (rather than
// reaching through the factory) lets StopGroup resolve the network
// from the committer's config alone, without pulling in the fabricx
// committer's org/key dependencies.
//
// Only valid for committer groups with deployment_config populated
// (i.e. post-Init). Returns an error for orderer groups or
// uninitialized committers — the caller must guard.
func (s *Service) committerNetworkForGroup(grp *ngtypes.NodeGroup) (string, error) {
	if grp.GroupType != ngtypes.GroupTypeFabricXCommitter {
		return "", fmt.Errorf("group %d is not a committer group", grp.ID)
	}
	if len(grp.DeploymentConfig) == 0 {
		return "", fmt.Errorf("group %d has no deployment_config", grp.ID)
	}
	var cfg nodetypes.FabricXCommitterDeploymentConfig
	if err := json.Unmarshal(grp.DeploymentConfig, &cfg); err != nil {
		return "", fmt.Errorf("unmarshal committer deployment config: %w", err)
	}
	if cfg.MSPID == "" {
		return "", fmt.Errorf("group %d deployment_config missing mspID", grp.ID)
	}
	slug := strings.ReplaceAll(strings.ToLower(grp.Name), " ", "-")
	return fmt.Sprintf("fabricx-%s-%s-net", strings.ToLower(cfg.MSPID), slug), nil
}

// postgresContainerName returns a stable, unique container name for a
// services row. Uses the service name so operators can recognize it in
// `docker ps`; falls back to an ID-based name if the service name is
// somehow empty (defensive — services.name is NOT NULL in schema).
func postgresContainerName(svc *db.Service) string {
	if svc.Name != "" {
		return "chainlaunch-service-" + svc.Name
	}
	return fmt.Sprintf("chainlaunch-service-%d", svc.ID)
}

// markServiceError persists ERROR status + message on a services row.
// Best-effort — we do not want DB failures here to mask the original
// lifecycle error the caller is about to surface.
func (s *Service) markServiceError(ctx context.Context, id int64, cause error) {
	if _, err := s.db.UpdateServiceStatusWithError(ctx, &db.UpdateServiceStatusWithErrorParams{
		ID:           id,
		Status:       string(nodetypes.NodeStatusError),
		ErrorMessage: nullStringFrom(cause.Error()),
	}); err != nil {
		s.logger.Warn("failed to persist service error status", "id", id, "err", err)
	}
}

// resolveManagedPostgres returns (host, port) for a POSTGRES service
// attached to this group when present, or ("", 0) when not. The
// coordinator uses the returned pair as an override for
// Committer.PrepareCommitterStart.
//
// Populated by startManagedPostgresForCommitter in the current start
// pass, or persisted from a previous start if the postgres container
// is still up.
func (s *Service) resolveManagedPostgres(ctx context.Context, groupID int64) (string, int, error) {
	svc, err := s.findPostgresService(ctx, groupID)
	if err != nil {
		return "", 0, err
	}
	if svc != nil && svc.DeploymentConfig.Valid {
		var dep struct {
			Host string `json:"host"`
			Port int    `json:"port"`
		}
		if err := json.Unmarshal([]byte(svc.DeploymentConfig.String), &dep); err != nil {
			return "", 0, fmt.Errorf("unmarshal service %d deployment_config: %w", svc.ID, err)
		}
		if dep.Host != "" {
			return dep.Host, dep.Port, nil
		}
	}
	return "", 0, nil
}

// markGroupError persists an ERROR status and message on a group,
// swallowing inner DB errors (we don't want to mask the original
// lifecycle failure).
func (s *Service) markGroupError(ctx context.Context, id int64, cause error) {
	if _, err := s.db.UpdateNodeGroupStatusWithError(ctx, &db.UpdateNodeGroupStatusWithErrorParams{
		ID:           id,
		Status:       string(nodetypes.NodeStatusError),
		ErrorMessage: nullStringFrom(cause.Error()),
	}); err != nil {
		s.logger.Warn("failed to persist node_group error status", "id", id, "err", err)
	}
}

// hydrateRow converts a sqlc *db.NodeGroup into the typed domain model.
func hydrateRow(row *db.NodeGroup) *ngtypes.NodeGroup {
	out := &ngtypes.NodeGroup{
		ID:        row.ID,
		Name:      row.Name,
		Platform:  row.Platform,
		GroupType: ngtypes.GroupType(row.GroupType),
		Status:    ngtypes.GroupStatus(row.Status),
		CreatedAt: row.CreatedAt,
	}
	if row.MspID.Valid {
		out.MSPID = row.MspID.String
	}
	if row.OrganizationID.Valid {
		v := row.OrganizationID.Int64
		out.OrganizationID = &v
	}
	if row.PartyID.Valid {
		v := row.PartyID.Int64
		out.PartyID = &v
	}
	if row.Version.Valid {
		out.Version = row.Version.String
	}
	if row.ExternalIp.Valid {
		out.ExternalIP = row.ExternalIp.String
	}
	if row.DomainNames.Valid && row.DomainNames.String != "" {
		var names []string
		if err := json.Unmarshal([]byte(row.DomainNames.String), &names); err == nil {
			out.DomainNames = names
		}
	}
	if row.SignKeyID.Valid {
		v := row.SignKeyID.Int64
		out.SignKeyID = &v
	}
	if row.TlsKeyID.Valid {
		v := row.TlsKeyID.Int64
		out.TLSKeyID = &v
	}
	if row.SignCert.Valid {
		out.SignCert = row.SignCert.String
	}
	if row.TlsCert.Valid {
		out.TLSCert = row.TlsCert.String
	}
	if row.CaCert.Valid {
		out.CACert = row.CaCert.String
	}
	if row.TlsCaCert.Valid {
		out.TLSCACert = row.TlsCaCert.String
	}
	if row.Config.Valid {
		out.Config = json.RawMessage(row.Config.String)
	}
	if row.DeploymentConfig.Valid {
		out.DeploymentConfig = json.RawMessage(row.DeploymentConfig.String)
	}
	if row.ErrorMessage.Valid {
		out.ErrorMessage = row.ErrorMessage.String
	}
	if row.UpdatedAt.Valid {
		t := row.UpdatedAt.Time
		out.UpdatedAt = &t
	}
	return out
}

// hydrateServiceRow converts a sqlc *db.Service into the typed domain
// model. Mirrors hydrateRow for node_groups.
func hydrateServiceRow(row *db.Service) *ngtypes.NodeGroupService {
	out := &ngtypes.NodeGroupService{
		ID:          row.ID,
		Name:        row.Name,
		ServiceType: ngtypes.ServiceType(row.ServiceType),
		Status:      ngtypes.GroupStatus(row.Status),
		CreatedAt:   row.CreatedAt,
	}
	if row.NodeGroupID.Valid {
		v := row.NodeGroupID.Int64
		out.NodeGroupID = &v
	}
	if row.Version.Valid {
		out.Version = row.Version.String
	}
	if row.Config.Valid {
		out.Config = json.RawMessage(row.Config.String)
	}
	if row.DeploymentConfig.Valid {
		out.DeploymentConfig = json.RawMessage(row.DeploymentConfig.String)
	}
	if row.ErrorMessage.Valid {
		out.ErrorMessage = row.ErrorMessage.String
	}
	if row.UpdatedAt.Valid {
		t := row.UpdatedAt.Time
		out.UpdatedAt = &t
	}
	return out
}

func nullStringFrom(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt64FromPtr(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

// postgresDataDir is the canonical bind-mount source for a managed
// postgres container. Returns "" when dataPath is empty so the postgres
// helper falls back to the container's writable layer (used in tests).
func postgresDataDir(dataPath, containerName string) string {
	if dataPath == "" || containerName == "" {
		return ""
	}
	return filepath.Join(dataPath, "services", "postgres", containerName)
}
