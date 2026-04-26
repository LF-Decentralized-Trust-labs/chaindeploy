package db_test

// DB-layer tests for the service_backups queries introduced in
// migration 0022. These run against a real in-memory sqlite database
// with migrations applied, so they exercise the actual SQL (including
// foreign-key cascades) rather than the Go wrappers alone.

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	_ "github.com/mattn/go-sqlite3"
)

func newTestQueries(t *testing.T) (*db.Queries, *sql.DB) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return db.New(sqlDB), sqlDB
}

// seedServiceRow creates the minimal parent chain needed for a
// service_backups row: node_groups → services.
func seedServiceRow(t *testing.T, q *db.Queries) int64 {
	t.Helper()
	ctx := context.Background()

	grp, err := q.CreateNodeGroup(ctx, &db.CreateNodeGroupParams{
		Name:      "grp-" + t.Name(),
		Platform:  "FABRICX",
		GroupType: "FABRICX_COMMITTER",
		Status:    "CREATED",
	})
	if err != nil {
		t.Fatalf("create node_group: %v", err)
	}

	svc, err := q.CreateService(ctx, &db.CreateServiceParams{
		NodeGroupID: sql.NullInt64{Int64: grp.ID, Valid: true},
		Name:        "svc-" + t.Name(),
		ServiceType: "POSTGRES",
		Status:      "CREATED",
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	return svc.ID
}

func TestServiceBackup_CreateAndGet(t *testing.T) {
	q, _ := newTestQueries(t)
	ctx := context.Background()
	serviceID := seedServiceRow(t, q)

	created, err := q.CreateServiceBackup(ctx, &db.CreateServiceBackupParams{
		ServiceID:  serviceID,
		BackupType: "BASE",
		S3Key:      sql.NullString{String: "backups/base/001", Valid: true},
		SizeBytes:  sql.NullInt64{Int64: 1024, Valid: true},
		Lsn:        sql.NullString{String: "0/1A2B3C4D", Valid: true},
		Timeline:   sql.NullInt64{Int64: 1, Valid: true},
		Status:     "RUNNING",
		Metadata:   sql.NullString{String: `{"walg":"v2"}`, Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateServiceBackup: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected non-zero ID on created backup")
	}
	if created.Status != "RUNNING" {
		t.Errorf("status: got %q, want RUNNING", created.Status)
	}
	if created.BackupType != "BASE" {
		t.Errorf("backupType: got %q, want BASE", created.BackupType)
	}
	if !created.S3Key.Valid || created.S3Key.String != "backups/base/001" {
		t.Errorf("s3Key not round-tripped: %+v", created.S3Key)
	}

	got, err := q.GetServiceBackup(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetServiceBackup: %v", err)
	}
	if got.ID != created.ID || got.ServiceID != serviceID {
		t.Errorf("Get returned wrong row: got %+v", got)
	}
	if !got.Metadata.Valid || got.Metadata.String != `{"walg":"v2"}` {
		t.Errorf("metadata not round-tripped: %+v", got.Metadata)
	}
}

func TestServiceBackup_ListAndCount(t *testing.T) {
	q, _ := newTestQueries(t)
	ctx := context.Background()
	serviceID := seedServiceRow(t, q)

	// Insert three backups, spaced in time so started_at ordering is stable.
	for i := range 3 {
		if _, err := q.CreateServiceBackup(ctx, &db.CreateServiceBackupParams{
			ServiceID:  serviceID,
			BackupType: "BASE",
			Status:     "SUCCEEDED",
		}); err != nil {
			t.Fatalf("CreateServiceBackup[%d]: %v", i, err)
		}
		// sqlite CURRENT_TIMESTAMP has 1s resolution; stagger inserts.
		time.Sleep(1100 * time.Millisecond)
	}

	count, err := q.CountServiceBackupsByService(ctx, serviceID)
	if err != nil {
		t.Fatalf("CountServiceBackupsByService: %v", err)
	}
	if count != 3 {
		t.Errorf("count: got %d, want 3", count)
	}

	list, err := q.ListServiceBackupsByService(ctx, &db.ListServiceBackupsByServiceParams{
		ServiceID: serviceID,
		Limit:     10,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("ListServiceBackupsByService: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("list length: got %d, want 3", len(list))
	}
	// ORDER BY started_at DESC — latest first.
	if !list[0].StartedAt.After(list[1].StartedAt) || !list[1].StartedAt.After(list[2].StartedAt) {
		t.Errorf("expected DESC ordering by started_at, got:\n  [0]=%s\n  [1]=%s\n  [2]=%s",
			list[0].StartedAt, list[1].StartedAt, list[2].StartedAt)
	}

	page, err := q.ListServiceBackupsByService(ctx, &db.ListServiceBackupsByServiceParams{
		ServiceID: serviceID,
		Limit:     2,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("ListServiceBackupsByService (paged): %v", err)
	}
	if len(page) != 2 {
		t.Errorf("paged list length: got %d, want 2", len(page))
	}
}

func TestServiceBackup_UpdateStatus(t *testing.T) {
	q, _ := newTestQueries(t)
	ctx := context.Background()
	serviceID := seedServiceRow(t, q)

	created, err := q.CreateServiceBackup(ctx, &db.CreateServiceBackupParams{
		ServiceID:  serviceID,
		BackupType: "BASE",
		Status:     "RUNNING",
	})
	if err != nil {
		t.Fatalf("CreateServiceBackup: %v", err)
	}

	completedAt := time.Now().UTC().Truncate(time.Second)
	updated, err := q.UpdateServiceBackupStatus(ctx, &db.UpdateServiceBackupStatusParams{
		ID:          created.ID,
		Status:      "SUCCEEDED",
		CompletedAt: sql.NullTime{Time: completedAt, Valid: true},
		SizeBytes:   sql.NullInt64{Int64: 4096, Valid: true},
		S3Key:       sql.NullString{String: "backups/base/002", Valid: true},
		Lsn:         sql.NullString{String: "0/DEADBEEF", Valid: true},
		Timeline:    sql.NullInt64{Int64: 2, Valid: true},
		Metadata:    sql.NullString{String: `{"finalized":true}`, Valid: true},
	})
	if err != nil {
		t.Fatalf("UpdateServiceBackupStatus: %v", err)
	}
	if updated.Status != "SUCCEEDED" {
		t.Errorf("status: got %q, want SUCCEEDED", updated.Status)
	}
	if !updated.CompletedAt.Valid || !updated.CompletedAt.Time.Equal(completedAt) {
		t.Errorf("completedAt: got %+v, want %s", updated.CompletedAt, completedAt)
	}
	if !updated.SizeBytes.Valid || updated.SizeBytes.Int64 != 4096 {
		t.Errorf("sizeBytes: got %+v, want 4096", updated.SizeBytes)
	}

	// Failure path: setting error_message with a non-valid completed_at.
	failed, err := q.UpdateServiceBackupStatus(ctx, &db.UpdateServiceBackupStatusParams{
		ID:           created.ID,
		Status:       "FAILED",
		ErrorMessage: sql.NullString{String: "boom", Valid: true},
	})
	if err != nil {
		t.Fatalf("UpdateServiceBackupStatus (fail path): %v", err)
	}
	if failed.Status != "FAILED" || !failed.ErrorMessage.Valid || failed.ErrorMessage.String != "boom" {
		t.Errorf("failed row unexpected: status=%q err=%+v", failed.Status, failed.ErrorMessage)
	}
	if failed.CompletedAt.Valid {
		t.Errorf("expected completed_at to clear on failure update, got %+v", failed.CompletedAt)
	}
}

func TestServiceBackup_DeleteOlderThan(t *testing.T) {
	q, sqlDB := newTestQueries(t)
	ctx := context.Background()
	serviceID := seedServiceRow(t, q)

	// Insert three BASE backups with explicit started_at values so we can
	// prove the cutoff works without sleeping.
	now := time.Now().UTC()
	insertAt := func(offset time.Duration, backupType string) int64 {
		t.Helper()
		res, err := sqlDB.ExecContext(ctx,
			`INSERT INTO service_backups (service_id, backup_type, status, started_at) VALUES (?, ?, ?, ?)`,
			serviceID, backupType, "SUCCEEDED", now.Add(offset),
		)
		if err != nil {
			t.Fatalf("raw insert: %v", err)
		}
		id, _ := res.LastInsertId()
		return id
	}

	oldID := insertAt(-48*time.Hour, "BASE")
	recentID := insertAt(-1*time.Hour, "BASE")
	walID := insertAt(-48*time.Hour, "WAL") // different backup type, must survive

	cutoff := now.Add(-24 * time.Hour)
	if err := q.DeleteServiceBackupsOlderThan(ctx, &db.DeleteServiceBackupsOlderThanParams{
		ServiceID:  serviceID,
		BackupType: "BASE",
		StartedAt:  cutoff,
	}); err != nil {
		t.Fatalf("DeleteServiceBackupsOlderThan: %v", err)
	}

	if _, err := q.GetServiceBackup(ctx, oldID); err != sql.ErrNoRows {
		t.Errorf("old BASE backup should be gone, got err=%v", err)
	}
	if _, err := q.GetServiceBackup(ctx, recentID); err != nil {
		t.Errorf("recent BASE backup should survive, got err=%v", err)
	}
	if _, err := q.GetServiceBackup(ctx, walID); err != nil {
		t.Errorf("old WAL backup should survive (different backup_type), got err=%v", err)
	}
}

func TestServiceBackup_CascadeOnServiceDelete(t *testing.T) {
	q, _ := newTestQueries(t)
	ctx := context.Background()
	serviceID := seedServiceRow(t, q)

	bk, err := q.CreateServiceBackup(ctx, &db.CreateServiceBackupParams{
		ServiceID:  serviceID,
		BackupType: "BASE",
		Status:     "SUCCEEDED",
	})
	if err != nil {
		t.Fatalf("CreateServiceBackup: %v", err)
	}

	if err := q.DeleteService(ctx, serviceID); err != nil {
		t.Fatalf("DeleteService: %v", err)
	}

	// service_backups has ON DELETE CASCADE on service_id — row must be gone.
	if _, err := q.GetServiceBackup(ctx, bk.ID); err != sql.ErrNoRows {
		t.Errorf("expected cascade delete, got err=%v", err)
	}
}
