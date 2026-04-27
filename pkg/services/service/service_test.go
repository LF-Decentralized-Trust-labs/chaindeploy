// Unit tests for the standalone services coordinator.
//
// Scope: state-transition guards and happy-path CRUD on a real in-memory
// sqlite with migrations applied. The postgres adapter is stubbed — no
// docker, no real container — so these tests stay hermetic.
//
// What we cover:
//   - CreatePostgres: validation (name/db/user/password required),
//     persists CREATED row with config JSON
//   - Get/List: hydration + filter by ServiceType and Status
//   - UpdatePostgres: rejects RUNNING/STARTING, merges partial fields,
//     rejects empty password
//   - Delete: rejects RUNNING/STARTING/STOPPING
//   - StartPostgres: rejects empty networkName; persists deployment_config
//     and marks RUNNING on success; records calls on the stub
//   - Stop: dispatches to postgres backend; marks STOPPED
package service_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	pgservice "github.com/chainlaunch/chainlaunch/pkg/services/postgres"
	"github.com/chainlaunch/chainlaunch/pkg/services/service"
	svctypes "github.com/chainlaunch/chainlaunch/pkg/services/types"
	_ "github.com/mattn/go-sqlite3"
)

// --- helpers ---------------------------------------------------------

func newTestQueries(t *testing.T) *db.Queries {
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
	return db.New(sqlDB)
}

// fakePostgres satisfies service.PostgresLifecycle and records Start/Stop
// calls. Returns canned errors when set.
type fakePostgres struct {
	mu          sync.Mutex
	startCfgs   []pgservice.Config
	stoppedBy   []string
	startErr    error
	stopErr     error
	isRunningOK bool
}

func (f *fakePostgres) Start(_ context.Context, cfg pgservice.Config) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startCfgs = append(f.startCfgs, cfg)
	if f.startErr != nil {
		return "", f.startErr
	}
	return "cid-" + cfg.ContainerName, nil
}

func (f *fakePostgres) Stop(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stoppedBy = append(f.stoppedBy, name)
	return f.stopErr
}

func (f *fakePostgres) IsRunning(_ context.Context, _ string) (bool, error) {
	return f.isRunningOK, nil
}

func (f *fakePostgres) Logs(_ context.Context, _ string, _ int) (string, error) {
	return "", nil
}

func (f *fakePostgres) CreateDatabases(_ context.Context, _ string, _ string, _ []pgservice.DatabaseSpec) error {
	return nil
}

func newSvc(t *testing.T) (*service.Service, *db.Queries, *fakePostgres) {
	t.Helper()
	q := newTestQueries(t)
	fp := &fakePostgres{}
	s := service.NewService(q, logger.NewDefault()).WithPostgresLifecycle(fp)
	return s, q, fp
}

// --- tests -----------------------------------------------------------

func TestCreatePostgres_ValidatesRequiredFields(t *testing.T) {
	s, _, _ := newSvc(t)
	ctx := context.Background()

	cases := []struct {
		name string
		in   service.CreatePostgresInput
	}{
		{"missing name", service.CreatePostgresInput{DB: "x", User: "y", Password: "z"}},
		{"missing db", service.CreatePostgresInput{Name: "a", User: "y", Password: "z"}},
		{"missing user", service.CreatePostgresInput{Name: "a", DB: "x", Password: "z"}},
		{"missing password", service.CreatePostgresInput{Name: "a", DB: "x", User: "y"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := s.CreatePostgres(ctx, c.in); err == nil {
				t.Fatalf("expected error for %s", c.name)
			}
		})
	}
}

func TestCreatePostgres_PersistsCreatedRow(t *testing.T) {
	s, _, _ := newSvc(t)
	ctx := context.Background()

	svc, err := s.CreatePostgres(ctx, service.CreatePostgresInput{
		Name: "pg1", DB: "app", User: "u", Password: "p", Version: "16",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if svc.Status != nodetypes.NodeStatusCreated {
		t.Fatalf("status = %q, want CREATED", svc.Status)
	}
	if svc.ServiceType != svctypes.ServiceTypePostgres {
		t.Fatalf("serviceType = %q, want POSTGRES", svc.ServiceType)
	}
	if svc.Version != "16" {
		t.Fatalf("version = %q, want 16", svc.Version)
	}
}

func TestList_FiltersByTypeAndStatus(t *testing.T) {
	s, q, _ := newSvc(t)
	ctx := context.Background()

	// Seed: two POSTGRES (CREATED + RUNNING). List by status filters.
	a, _ := s.CreatePostgres(ctx, service.CreatePostgresInput{Name: "a", DB: "x", User: "u", Password: "p"})
	b, _ := s.CreatePostgres(ctx, service.CreatePostgresInput{Name: "b", DB: "x", User: "u", Password: "p"})
	_ = a
	if _, err := q.UpdateServiceStatus(ctx, &db.UpdateServiceStatusParams{
		ID: b.ID, Status: string(nodetypes.NodeStatusRunning),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	running, err := s.List(ctx, service.ListFilter{Status: string(nodetypes.NodeStatusRunning)})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(running) != 1 || running[0].ID != b.ID {
		t.Fatalf("running list = %+v, want only b", running)
	}

	byType, err := s.List(ctx, service.ListFilter{ServiceType: svctypes.ServiceTypePostgres})
	if err != nil {
		t.Fatalf("list by type: %v", err)
	}
	if len(byType) != 2 {
		t.Fatalf("by-type list len = %d, want 2", len(byType))
	}
}

func TestUpdatePostgres_RejectsWhenRunning(t *testing.T) {
	s, q, _ := newSvc(t)
	ctx := context.Background()

	svc, err := s.CreatePostgres(ctx, service.CreatePostgresInput{Name: "pg", DB: "x", User: "u", Password: "p"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := q.UpdateServiceStatus(ctx, &db.UpdateServiceStatusParams{
		ID: svc.ID, Status: string(nodetypes.NodeStatusRunning),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	newPW := "new"
	if _, err := s.UpdatePostgres(ctx, svc.ID, service.UpdatePostgresInput{Password: &newPW}); err == nil {
		t.Fatalf("expected error updating RUNNING service")
	}
}

func TestUpdatePostgres_MergesPartialFields(t *testing.T) {
	s, _, _ := newSvc(t)
	ctx := context.Background()

	svc, err := s.CreatePostgres(ctx, service.CreatePostgresInput{Name: "pg", DB: "x", User: "u", Password: "p", Version: "16"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	newName := "renamed"
	newPort := 5433
	updated, err := s.UpdatePostgres(ctx, svc.ID, service.UpdatePostgresInput{
		Name:     &newName,
		HostPort: &newPort,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "renamed" {
		t.Fatalf("name = %q, want renamed", updated.Name)
	}
	// Version must remain "16" — UpdatePostgresInput.Version was nil.
	if updated.Version != "16" {
		t.Fatalf("version = %q, want 16", updated.Version)
	}
}

func TestUpdatePostgres_RejectsEmptyPassword(t *testing.T) {
	s, _, _ := newSvc(t)
	ctx := context.Background()

	svc, _ := s.CreatePostgres(ctx, service.CreatePostgresInput{Name: "pg", DB: "x", User: "u", Password: "p"})
	empty := ""
	if _, err := s.UpdatePostgres(ctx, svc.ID, service.UpdatePostgresInput{Password: &empty}); err == nil {
		t.Fatalf("expected error for empty password")
	}
}

func TestDelete_RejectsWhenActive(t *testing.T) {
	s, q, _ := newSvc(t)
	ctx := context.Background()

	for _, status := range []nodetypes.NodeStatus{
		nodetypes.NodeStatusRunning,
		nodetypes.NodeStatusStarting,
		nodetypes.NodeStatusStopping,
	} {
		svc, _ := s.CreatePostgres(ctx, service.CreatePostgresInput{Name: "pg-" + string(status), DB: "x", User: "u", Password: "p"})
		if _, err := q.UpdateServiceStatus(ctx, &db.UpdateServiceStatusParams{
			ID: svc.ID, Status: string(status),
		}); err != nil {
			t.Fatalf("seed %s: %v", status, err)
		}
		if err := s.Delete(ctx, svc.ID); err == nil {
			t.Fatalf("expected delete rejection for status=%s", status)
		}
	}
}

func TestDelete_SucceedsWhenStopped(t *testing.T) {
	s, q, _ := newSvc(t)
	ctx := context.Background()

	svc, _ := s.CreatePostgres(ctx, service.CreatePostgresInput{Name: "pg", DB: "x", User: "u", Password: "p"})
	if _, err := q.UpdateServiceStatus(ctx, &db.UpdateServiceStatusParams{
		ID: svc.ID, Status: string(nodetypes.NodeStatusStopped),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.Delete(ctx, svc.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestStartPostgres_RequiresNetworkName(t *testing.T) {
	s, _, _ := newSvc(t)
	ctx := context.Background()
	svc, _ := s.CreatePostgres(ctx, service.CreatePostgresInput{Name: "pg", DB: "x", User: "u", Password: "p"})
	if _, err := s.StartPostgres(ctx, svc.ID, ""); err == nil {
		t.Fatalf("expected error for empty networkName")
	}
}

func TestStartPostgres_HappyPath(t *testing.T) {
	s, q, fp := newSvc(t)
	ctx := context.Background()

	svc, _ := s.CreatePostgres(ctx, service.CreatePostgresInput{Name: "pg", DB: "app", User: "u", Password: "p", HostPort: 5432})
	started, err := s.StartPostgres(ctx, svc.ID, "net-a")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if started.Status != nodetypes.NodeStatusRunning {
		t.Fatalf("status after start = %q, want RUNNING", started.Status)
	}
	if len(fp.startCfgs) != 1 {
		t.Fatalf("postgres Start called %d times, want 1", len(fp.startCfgs))
	}
	if fp.startCfgs[0].NetworkName != "net-a" {
		t.Fatalf("network = %q, want net-a", fp.startCfgs[0].NetworkName)
	}

	// deployment_config should be persisted with the committed host/port.
	row, err := q.GetService(ctx, svc.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !row.DeploymentConfig.Valid || row.DeploymentConfig.String == "" {
		t.Fatalf("deployment_config not persisted")
	}
}

func TestStop_MarksStopped(t *testing.T) {
	s, q, fp := newSvc(t)
	ctx := context.Background()

	svc, _ := s.CreatePostgres(ctx, service.CreatePostgresInput{Name: "pg", DB: "x", User: "u", Password: "p"})
	if _, err := s.StartPostgres(ctx, svc.ID, "net-a"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := s.Stop(ctx, svc.ID); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if len(fp.stoppedBy) != 1 {
		t.Fatalf("Stop called %d times, want 1", len(fp.stoppedBy))
	}
	row, _ := q.GetService(ctx, svc.ID)
	if row.Status != string(nodetypes.NodeStatusStopped) {
		t.Fatalf("status = %q, want STOPPED", row.Status)
	}
}
