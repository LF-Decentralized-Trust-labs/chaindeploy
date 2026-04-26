// Package service is the coordinator for managed supporting services
// (PostgreSQL today; more later). Services are standalone resources with
// their own CRUD + lifecycle — a node_group references the service it
// needs, not the other way around.
//
// Responsibilities:
//   - Persist and hydrate services rows (Create/Get/List/Update/Delete)
//   - Drive per-service Start/Stop through a pluggable backend (docker by
//     default; injectable for tests)
//   - Validate state transitions (can't delete a RUNNING service, can't
//     PUT a RUNNING service's config, etc.)
//
// This package is deliberately consumed by the node_groups coordinator
// via a narrow interface (Lifecycle) so fan-out callers don't reach into
// postgres plumbing directly.
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	pgservice "github.com/chainlaunch/chainlaunch/pkg/services/postgres"
	svctypes "github.com/chainlaunch/chainlaunch/pkg/services/types"
)

// postgresDataDir is the canonical bind-mount source for a managed
// postgres container. Returns "" when dataPath is empty so postgres
// falls back to the container's writable layer (used in tests).
func postgresDataDir(dataPath, containerName string) string {
	if dataPath == "" || containerName == "" {
		return ""
	}
	return filepath.Join(dataPath, "services", "postgres", containerName)
}

// PostgresLifecycle is the narrow subset of pkg/services/postgres the
// coordinator uses. Pulled behind an interface so unit tests can record
// Start/Stop calls without docker. Production impl is defaultPostgresAdapter.
type PostgresLifecycle interface {
	Start(ctx context.Context, cfg pgservice.Config) (containerID string, err error)
	Stop(ctx context.Context, containerName string) error
	IsRunning(ctx context.Context, containerName string) (bool, error)
	CreateDatabases(ctx context.Context, containerName, adminUser string, specs []pgservice.DatabaseSpec) error
	Logs(ctx context.Context, containerName string, tail int) (string, error)
}

// Service is the services coordinator.
type Service struct {
	db       *db.Queries
	postgres PostgresLifecycle
	logger   *logger.Logger
	// dataPath is the chainlaunch on-disk root; postgres bind mounts
	// land under ${dataPath}/services/postgres/<container>. Empty means
	// "no bind" (used in tests).
	dataPath string
}

// NewService wires the coordinator. Production uses the docker-backed
// postgres adapter; tests can override via WithPostgresLifecycle.
func NewService(dbQueries *db.Queries, log *logger.Logger) *Service {
	return &Service{
		db:       dbQueries,
		postgres: defaultPostgresAdapter{log: log},
		logger:   log,
	}
}

// WithPostgresLifecycle swaps the default (docker-backed) postgres
// adapter for a caller-supplied one. Used by tests; production leaves it
// alone.
func (s *Service) WithPostgresLifecycle(pl PostgresLifecycle) *Service {
	s.postgres = pl
	return s
}

// WithDataPath enables on-host bind mounts for managed postgres data
// directories. Production wires this in serve.go so backups capture
// PGDATA; tests pass "".
func (s *Service) WithDataPath(dataPath string) *Service {
	s.dataPath = dataPath
	return s
}

// defaultPostgresAdapter routes PostgresLifecycle calls to the
// package-level pkg/services/postgres helpers.
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

func (a defaultPostgresAdapter) CreateDatabases(ctx context.Context, containerName, adminUser string, specs []pgservice.DatabaseSpec) error {
	return pgservice.CreateDatabases(ctx, containerName, adminUser, specs)
}

func (a defaultPostgresAdapter) Logs(ctx context.Context, containerName string, tail int) (string, error) {
	return pgservice.Logs(ctx, containerName, tail)
}

// --- CRUD ------------------------------------------------------------

// CreatePostgresInput carries the fields required to persist a POSTGRES
// services row. The container is not started until Start is called.
type CreatePostgresInput struct {
	Name     string
	Version  string
	DB       string
	User     string
	Password string
	HostPort int
}

// CreatePostgres persists a standalone POSTGRES services row. The row is
// created in CREATED state — the container is not started until the next
// Start call.
func (s *Service) CreatePostgres(ctx context.Context, in CreatePostgresInput) (*svctypes.Service, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if in.DB == "" || in.User == "" || in.Password == "" {
		return nil, fmt.Errorf("db, user and password are required")
	}

	cfg := svctypes.PostgresConfig{
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
		Name:        in.Name,
		ServiceType: string(svctypes.ServiceTypePostgres),
		Version:     nullStringFrom(in.Version),
		Status:      string(nodetypes.NodeStatusCreated),
		Config:      sql.NullString{String: string(cfgJSON), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("persist service: %w", err)
	}
	return hydrateServiceRow(row), nil
}

// Get returns the hydrated service by ID.
func (s *Service) Get(ctx context.Context, id int64) (*svctypes.Service, error) {
	row, err := s.db.GetService(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load service %d: %w", id, err)
	}
	return hydrateServiceRow(row), nil
}

// ListFilter narrows the List call to a subset of services. Zero-valued
// fields are ignored.
type ListFilter struct {
	ServiceType svctypes.ServiceType
	Status      string
	Limit       int64
	Offset      int64
}

// List returns services matching the filter, sorted by creation time
// descending. Filters are applied in Go since the SQL layer exposes
// separate pre-built queries for each axis; the set is small enough that
// post-filtering is fine.
func (s *Service) List(ctx context.Context, f ListFilter) ([]*svctypes.Service, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.ListServices(ctx, &db.ListServicesParams{
		Limit:  limit,
		Offset: f.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	out := make([]*svctypes.Service, 0, len(rows))
	for _, row := range rows {
		if f.ServiceType != "" && row.ServiceType != string(f.ServiceType) {
			continue
		}
		if f.Status != "" && row.Status != f.Status {
			continue
		}
		out = append(out, hydrateServiceRow(row))
	}
	return out, nil
}

// Consumer is a lightweight summary of a node_group that references a
// service via postgres_service_id. Used by ListConsumers so the UI can
// warn on destructive actions and show at-a-glance sharing.
type Consumer struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	GroupType string `json:"groupType"`
	Status    string `json:"status"`
}

// ListConsumers returns the node_groups that point at this service via
// postgres_service_id. An empty slice is valid — services without
// consumers can be freely deleted.
func (s *Service) ListConsumers(ctx context.Context, id int64) ([]Consumer, error) {
	if _, err := s.db.GetService(ctx, id); err != nil {
		return nil, fmt.Errorf("load service %d: %w", id, err)
	}
	rows, err := s.db.ListNodeGroupsByPostgresServiceID(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list consumers of service %d: %w", id, err)
	}
	out := make([]Consumer, 0, len(rows))
	for _, r := range rows {
		out = append(out, Consumer{
			ID:        r.ID,
			Name:      r.Name,
			GroupType: r.GroupType,
			Status:    r.Status,
		})
	}
	return out, nil
}

// UpdatePostgresInput carries mutable fields on a POSTGRES service.
// Password rotation is supported; changing DB/User on a running container
// is not.
type UpdatePostgresInput struct {
	Name     *string
	Version  *string
	Password *string
	HostPort *int
}

// UpdatePostgres mutates a POSTGRES service. Rejected when the service is
// RUNNING — operators must stop the group (or the service) first, since
// the live container can't absorb the change without a restart.
func (s *Service) UpdatePostgres(ctx context.Context, id int64, in UpdatePostgresInput) (*svctypes.Service, error) {
	row, err := s.db.GetService(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load service %d: %w", id, err)
	}
	if row.ServiceType != string(svctypes.ServiceTypePostgres) {
		return nil, fmt.Errorf("service %d is %s; UpdatePostgres only accepts POSTGRES", id, row.ServiceType)
	}
	if row.Status == string(nodetypes.NodeStatusRunning) || row.Status == string(nodetypes.NodeStatusStarting) {
		return nil, fmt.Errorf("cannot update service %d while status=%s; stop it first", id, row.Status)
	}

	// Merge mutations into the current config.
	var cfg svctypes.PostgresConfig
	if row.Config.Valid && row.Config.String != "" {
		if err := json.Unmarshal([]byte(row.Config.String), &cfg); err != nil {
			return nil, fmt.Errorf("unmarshal current config: %w", err)
		}
	}
	if in.Version != nil {
		cfg.Version = *in.Version
	}
	if in.Password != nil {
		if *in.Password == "" {
			return nil, fmt.Errorf("password cannot be empty")
		}
		cfg.Password = *in.Password
	}
	if in.HostPort != nil {
		cfg.HostPort = *in.HostPort
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal updated config: %w", err)
	}

	name := row.Name
	if in.Name != nil {
		name = *in.Name
	}
	version := row.Version
	if in.Version != nil {
		version = nullStringFrom(*in.Version)
	}

	updated, err := s.db.UpdateService(ctx, &db.UpdateServiceParams{
		ID:               id,
		Name:             name,
		Version:          version,
		Config:           sql.NullString{String: string(cfgJSON), Valid: true},
		DeploymentConfig: row.DeploymentConfig,
		BackupTargetID:   row.BackupTargetID,
		BackupConfig:     row.BackupConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("persist service update: %w", err)
	}
	return hydrateServiceRow(updated), nil
}

// Delete removes a service. Rejected when the service is RUNNING or
// STARTING — operators must stop it first so we don't leave a dangling
// container.
func (s *Service) Delete(ctx context.Context, id int64) error {
	row, err := s.db.GetService(ctx, id)
	if err != nil {
		return fmt.Errorf("load service %d: %w", id, err)
	}
	if row.Status == string(nodetypes.NodeStatusRunning) ||
		row.Status == string(nodetypes.NodeStatusStarting) ||
		row.Status == string(nodetypes.NodeStatusStopping) {
		return fmt.Errorf("cannot delete service %d while status=%s; stop it first", id, row.Status)
	}
	if err := s.db.DeleteService(ctx, id); err != nil {
		return fmt.Errorf("delete service %d: %w", id, err)
	}
	return nil
}

// --- lifecycle -------------------------------------------------------

// StartPostgres starts the POSTGRES service on the given docker network
// and persists the resolved {host, port, containerName, networkName} to
// the services row. Idempotent: if the container is already running and
// the service row already has deployment_config, it's a no-op.
//
// Network ownership: the caller picks the network. The node_groups
// coordinator passes the committer's bridge network so siblings can dial
// postgres by container name. Standalone callers pass any network name
// they like.
func (s *Service) StartPostgres(ctx context.Context, id int64, networkName string) (*svctypes.Service, error) {
	if networkName == "" {
		return nil, fmt.Errorf("networkName is required")
	}
	row, err := s.db.GetService(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load service %d: %w", id, err)
	}
	if row.ServiceType != string(svctypes.ServiceTypePostgres) {
		return nil, fmt.Errorf("service %d is %s; StartPostgres only accepts POSTGRES", id, row.ServiceType)
	}
	if !row.Config.Valid || row.Config.String == "" {
		return nil, fmt.Errorf("service %d has no config; recreate with credentials", id)
	}

	var pgCfg svctypes.PostgresConfig
	if err := json.Unmarshal([]byte(row.Config.String), &pgCfg); err != nil {
		return nil, fmt.Errorf("unmarshal postgres service %d config: %w", id, err)
	}
	if pgCfg.DB == "" || pgCfg.User == "" || pgCfg.Password == "" {
		return nil, fmt.Errorf("service %d config missing db/user/password", id)
	}

	containerName := postgresContainerName(row)

	if _, err := s.db.UpdateServiceStatus(ctx, &db.UpdateServiceStatusParams{
		ID:     id,
		Status: string(nodetypes.NodeStatusStarting),
	}); err != nil {
		s.logger.Warn("failed to mark service STARTING", "id", id, "err", err)
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
		s.markServiceError(ctx, id, err)
		return nil, fmt.Errorf("start postgres service %d: %w", id, err)
	}

	deployment := svctypes.PostgresDeployment{
		// Siblings on the same bridge network dial postgres by
		// container name on 5432; the published host port is for
		// human/debug access only.
		Host:          containerName,
		Port:          5432,
		ContainerName: containerName,
		NetworkName:   networkName,
	}
	depJSON, err := json.Marshal(deployment)
	if err != nil {
		s.markServiceError(ctx, id, err)
		return nil, fmt.Errorf("marshal deployment config: %w", err)
	}
	updated, err := s.db.UpdateServiceDeploymentConfig(ctx, &db.UpdateServiceDeploymentConfigParams{
		ID:               id,
		DeploymentConfig: sql.NullString{String: string(depJSON), Valid: true},
	})
	if err != nil {
		s.markServiceError(ctx, id, err)
		return nil, fmt.Errorf("persist deployment config: %w", err)
	}
	if _, err := s.db.UpdateServiceStatus(ctx, &db.UpdateServiceStatusParams{
		ID:     id,
		Status: string(nodetypes.NodeStatusRunning),
	}); err != nil {
		s.logger.Warn("failed to mark service RUNNING", "id", id, "err", err)
	}
	updated.Status = string(nodetypes.NodeStatusRunning)
	return hydrateServiceRow(updated), nil
}

// DatabaseSpec mirrors pgservice.DatabaseSpec for the HTTP surface so
// callers don't import the low-level docker package just to shape a
// request body.
type DatabaseSpec struct {
	DB       string `json:"db"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// CreatePostgresDatabases provisions N databases + login roles inside
// the POSTGRES service's running container. Idempotent — existing
// roles keep (or refresh) their passwords, and existing databases are
// left alone. Rejected when the service is not RUNNING, since the
// container must be up to exec psql.
//
// This is what lets one POSTGRES service back many tenants (e.g. the
// FabricX quickstart uses one container with one DB per party).
func (s *Service) CreatePostgresDatabases(ctx context.Context, id int64, specs []DatabaseSpec) error {
	row, err := s.db.GetService(ctx, id)
	if err != nil {
		return fmt.Errorf("load service %d: %w", id, err)
	}
	if row.ServiceType != string(svctypes.ServiceTypePostgres) {
		return fmt.Errorf("service %d is %s; CreatePostgresDatabases only accepts POSTGRES", id, row.ServiceType)
	}
	if row.Status != string(nodetypes.NodeStatusRunning) {
		return fmt.Errorf("service %d must be RUNNING (got %s); start it first", id, row.Status)
	}
	if !row.Config.Valid || row.Config.String == "" {
		return fmt.Errorf("service %d has no config", id)
	}
	var pgCfg svctypes.PostgresConfig
	if err := json.Unmarshal([]byte(row.Config.String), &pgCfg); err != nil {
		return fmt.Errorf("unmarshal postgres config: %w", err)
	}
	if pgCfg.User == "" {
		return fmt.Errorf("service %d has no admin user in config", id)
	}

	pgSpecs := make([]pgservice.DatabaseSpec, 0, len(specs))
	for _, sp := range specs {
		pgSpecs = append(pgSpecs, pgservice.DatabaseSpec{DB: sp.DB, User: sp.User, Password: sp.Password})
	}

	containerName := postgresContainerName(row)
	if err := s.postgres.CreateDatabases(ctx, containerName, pgCfg.User, pgSpecs); err != nil {
		return fmt.Errorf("create databases on service %d: %w", id, err)
	}
	return nil
}

// GetLogs returns the last `tail` lines of the service's container
// logs. Only supported for POSTGRES today. Returns an empty string
// (not an error) when the container hasn't been started yet, so the
// UI can render a friendly empty state.
func (s *Service) GetLogs(ctx context.Context, id int64, tail int) (string, error) {
	row, err := s.db.GetService(ctx, id)
	if err != nil {
		return "", fmt.Errorf("load service %d: %w", id, err)
	}
	if row.ServiceType != string(svctypes.ServiceTypePostgres) {
		return "", fmt.Errorf("service %d type %s has no logs handler", id, row.ServiceType)
	}
	containerName := postgresContainerName(row)
	return s.postgres.Logs(ctx, containerName, tail)
}

// Stop stops the service's underlying resource (container for POSTGRES)
// and marks the row STOPPED. Best-effort — a failure to stop the
// container marks the row ERROR but returns the error so the caller can
// decide whether to continue a parent teardown.
func (s *Service) Stop(ctx context.Context, id int64) error {
	row, err := s.db.GetService(ctx, id)
	if err != nil {
		return fmt.Errorf("load service %d: %w", id, err)
	}
	switch row.ServiceType {
	case string(svctypes.ServiceTypePostgres):
		return s.stopPostgres(ctx, row)
	default:
		return fmt.Errorf("service %d type %s has no stop handler", id, row.ServiceType)
	}
}

func (s *Service) stopPostgres(ctx context.Context, row *db.Service) error {
	containerName := postgresContainerName(row)
	if _, err := s.db.UpdateServiceStatus(ctx, &db.UpdateServiceStatusParams{
		ID:     row.ID,
		Status: string(nodetypes.NodeStatusStopping),
	}); err != nil {
		s.logger.Warn("failed to mark service STOPPING", "id", row.ID, "err", err)
	}
	if err := s.postgres.Stop(ctx, containerName); err != nil {
		s.logger.Error("stop postgres container failed", "id", row.ID, "container", containerName, "err", err)
		s.markServiceError(ctx, row.ID, err)
		return fmt.Errorf("stop postgres %s: %w", containerName, err)
	}
	if _, err := s.db.UpdateServiceStatus(ctx, &db.UpdateServiceStatusParams{
		ID:     row.ID,
		Status: string(nodetypes.NodeStatusStopped),
	}); err != nil {
		s.logger.Warn("failed to mark service STOPPED", "id", row.ID, "err", err)
	}
	return nil
}

// --- helpers ---------------------------------------------------------

// postgresContainerName returns a stable, unique container name for a
// services row. Uses the service name so operators can recognize it in
// `docker ps`; falls back to an ID-based name defensively.
func postgresContainerName(row *db.Service) string {
	if row.Name != "" {
		return "chainlaunch-service-" + row.Name
	}
	return fmt.Sprintf("chainlaunch-service-%d", row.ID)
}

// markServiceError persists ERROR status + message on a services row.
// Best-effort — DB failures here must not mask the original lifecycle
// error the caller is about to surface.
func (s *Service) markServiceError(ctx context.Context, id int64, cause error) {
	if _, err := s.db.UpdateServiceStatusWithError(ctx, &db.UpdateServiceStatusWithErrorParams{
		ID:           id,
		Status:       string(nodetypes.NodeStatusError),
		ErrorMessage: nullStringFrom(cause.Error()),
	}); err != nil {
		s.logger.Warn("failed to persist service error status", "id", id, "err", err)
	}
}

// hydrateServiceRow converts a sqlc *db.Service into the typed domain
// model. Mirrors the nodegroups hydration style.
func hydrateServiceRow(row *db.Service) *svctypes.Service {
	out := &svctypes.Service{
		ID:          row.ID,
		Name:        row.Name,
		ServiceType: svctypes.ServiceType(row.ServiceType),
		Status:      svctypes.ServiceStatus(row.Status),
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
	if row.BackupTargetID.Valid {
		v := row.BackupTargetID.Int64
		out.BackupTargetID = &v
	}
	if row.BackupConfig.Valid && row.BackupConfig.String != "" {
		var bc svctypes.BackupConfig
		if err := json.Unmarshal([]byte(row.BackupConfig.String), &bc); err == nil {
			out.BackupConfig = &bc
		}
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

// nullStringFrom returns a NullString that is Valid iff s is non-empty.
func nullStringFrom(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
