// Package types defines the public domain model for managed supporting
// services (PostgreSQL, etc.) attached to node groups.
//
// A service is a first-class managed component with its own lifecycle,
// backups, and restore path. It is NOT a blockchain node: services live
// in the services table and do not pollute /nodes inventories. See ADR
// 0001 for rationale.
package types

import (
	"encoding/json"
	"time"

	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
)

// ServiceType identifies the concrete service implementation. The first
// citizen is POSTGRES; future services (Redis, vault agents, metric
// sidecars) land here.
type ServiceType string

const (
	ServiceTypePostgres ServiceType = "POSTGRES"
)

// ServiceStatus reuses the node_statuses enum (constrained by FK) so
// operators see a consistent lifecycle vocabulary across nodes, groups,
// and services.
type ServiceStatus = nodetypes.NodeStatus

// Service is the domain representation of a row in services.
type Service struct {
	ID               int64           `json:"id"`
	NodeGroupID      *int64          `json:"nodeGroupId,omitempty"`
	Name             string          `json:"name"`
	ServiceType      ServiceType     `json:"serviceType"`
	Version          string          `json:"version,omitempty"`
	Status           ServiceStatus   `json:"status"`
	Config           json.RawMessage `json:"config,omitempty"`
	DeploymentConfig json.RawMessage `json:"deploymentConfig,omitempty"`
	BackupTargetID   *int64          `json:"backupTargetId,omitempty"`
	BackupConfig     *BackupConfig   `json:"backupConfig,omitempty"`
	ErrorMessage     string          `json:"errorMessage,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        *time.Time      `json:"updatedAt,omitempty"`
}

// BackupConfig is persisted as JSON in services.backup_config. For
// POSTGRES it drives WAL-G base-backup cadence and WAL retention.
//
// Intentionally a single struct (not an interface) for now — a second
// service type would warrant per-type configs.
type BackupConfig struct {
	// Enabled gates WAL archiving and base backups. When false the
	// service still runs but no backups are produced.
	Enabled bool `json:"enabled"`
	// S3Prefix is the per-service key prefix inside the backup target's
	// bucket (WALG_S3_PREFIX = s3://<bucket>/<prefix>).
	S3Prefix string `json:"s3Prefix,omitempty"`
	// BaseBackupCron is a cron expression used by the scheduler to
	// trigger `wal-g backup-push`. Empty disables base backups (WAL
	// archiving remains active if Enabled).
	BaseBackupCron string `json:"baseBackupCron,omitempty"`
	// RetentionBaseBackups is the number of base backups to keep;
	// WAL-G's `delete retain FIND_FULL N` semantics.
	RetentionBaseBackups int `json:"retentionBaseBackups,omitempty"`
	// RetentionWALDays retains WAL segments for this many days past
	// the oldest kept base backup.
	RetentionWALDays int `json:"retentionWalDays,omitempty"`
}

// BackupType distinguishes WAL-G's two archive flavours for the
// service_backups table. WAL segments are tracked by WAL-G itself, not
// row-per-segment here; we insert WAL rows only to surface failures.
type BackupType string

const (
	BackupTypeBase BackupType = "BASE"
	BackupTypeWAL  BackupType = "WAL"
)

// BackupStatus is the row-level status of an individual service_backups
// entry (independent of the parent service's lifecycle status).
type BackupStatus string

const (
	BackupStatusPending    BackupStatus = "PENDING"
	BackupStatusInProgress BackupStatus = "IN_PROGRESS"
	BackupStatusCompleted  BackupStatus = "COMPLETED"
	BackupStatusFailed     BackupStatus = "FAILED"
)

// ServiceBackup is the domain representation of a row in service_backups.
type ServiceBackup struct {
	ID           int64           `json:"id"`
	ServiceID    int64           `json:"serviceId"`
	BackupType   BackupType      `json:"backupType"`
	S3Key        string          `json:"s3Key,omitempty"`
	SizeBytes    int64           `json:"sizeBytes,omitempty"`
	LSN          string          `json:"lsn,omitempty"`
	Timeline     int64           `json:"timeline,omitempty"`
	Status       BackupStatus    `json:"status"`
	StartedAt    time.Time       `json:"startedAt"`
	CompletedAt  *time.Time      `json:"completedAt,omitempty"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

// EventType names lifecycle events emitted to service_events. Mirrors
// node_events semantics so the UI can present a consistent feed.
type EventType string

const (
	EventTypeStart   EventType = "START"
	EventTypeStop    EventType = "STOP"
	EventTypeBackup  EventType = "BACKUP"
	EventTypeRestore EventType = "RESTORE"
	EventTypeError   EventType = "ERROR"
)

// ServiceEvent is the domain representation of a row in service_events.
type ServiceEvent struct {
	ID        int64           `json:"id"`
	ServiceID int64           `json:"serviceId"`
	Type      EventType       `json:"type"`
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

// PostgresConfig is the JSON payload stored in services.config for
// POSTGRES services. Canonical shape — used by the services coordinator,
// HTTP layer, and node_groups consumer.
type PostgresConfig struct {
	Version  string `json:"version,omitempty"`
	DB       string `json:"db"`
	User     string `json:"user"`
	Password string `json:"password"`
	HostPort int    `json:"hostPort,omitempty"`
}

// PostgresDeployment is the JSON payload stored in
// services.deployment_config once the postgres container is running.
// Siblings on the same docker network read this to dial the container.
type PostgresDeployment struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	ContainerName string `json:"containerName,omitempty"`
	NetworkName   string `json:"networkName,omitempty"`
}
