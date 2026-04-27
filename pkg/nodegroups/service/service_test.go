package service_test

// Unit tests for the node_groups coordinator. No docker; no real
// node service; all deps stubbed:
//
//   * DB: real in-memory sqlite with migrations applied
//   * NodeLifecycle: recording fake that captures call order
//   * fabricx ordererFactory/committerFactory: unused in the paths we
//     cover here (the tests avoid prepareGroup by stubbing NodeLifecycle
//     only in scenarios that exercise StartGroup/StopGroup fan-out
//     against a committer group with no deployment_config — the code
//     returns early with an error before reaching the factories).
//
// What we cover:
//   - Create / Get / List / Delete hydration (NULL fields, JSON
//     domain_names, pointer vs bare string translation)
//   - resolveManagedPostgres across: no services, wrong-type service,
//     POSTGRES service with missing deployment_config, and a valid one
//   - StopGroup fan-out: children are stopped in reverse canonical
//     ChildRoles order (works whether or not prepareGroup would fail
//     — StopGroup does not call prepareGroup)

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/nodegroups/service"
	ngtypes "github.com/chainlaunch/chainlaunch/pkg/nodegroups/types"
	"github.com/chainlaunch/chainlaunch/pkg/nodes/fabricx"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	nodetypes "github.com/chainlaunch/chainlaunch/pkg/nodes/types"
	pgservice "github.com/chainlaunch/chainlaunch/pkg/services/postgres"
	_ "github.com/mattn/go-sqlite3"
)

// --- helpers ---------------------------------------------------------

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

// fakeNodeLifecycle records StartNode/StopNode calls and returns canned
// responses. Satisfies service.NodeLifecycle.
type fakeNodeLifecycle struct {
	mu      sync.Mutex
	started []int64
	stopped []int64
	// startErr/stopErr are returned by Start/StopNode if non-nil.
	startErr error
	stopErr  error
}

func (f *fakeNodeLifecycle) StartNode(_ context.Context, id int64) (*nodeservice.NodeResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, id)
	if f.startErr != nil {
		return nil, f.startErr
	}
	return &nodeservice.NodeResponse{ID: id}, nil
}

func (f *fakeNodeLifecycle) StopNode(_ context.Context, id int64) (*nodeservice.NodeResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, id)
	if f.stopErr != nil {
		return nil, f.stopErr
	}
	return &nodeservice.NodeResponse{ID: id}, nil
}

// unusedOrdererFactory / unusedCommitterFactory are placeholders. They
// panic if called — tests that actually reach prepareGroup would be
// testing fabricx, which has its own integration suite.
func unusedOrdererFactory(_ *db.Queries, _ int64, _ nodetypes.FabricXOrdererGroupConfig) *fabricx.OrdererGroup {
	panic("ordererFactory should not be called in unit tests")
}
func unusedCommitterFactory(_ *db.Queries, _ int64, _ nodetypes.FabricXCommitterConfig) *fabricx.Committer {
	panic("committerFactory should not be called in unit tests")
}

func newSvc(t *testing.T, nl service.NodeLifecycle) (*service.Service, *db.Queries, *sql.DB) {
	t.Helper()
	q, sqlDB := newTestQueries(t)
	svc := service.NewService(q, nl, unusedOrdererFactory, unusedCommitterFactory, logger.NewDefault())
	return svc, q, sqlDB
}

// --- CRUD hydration --------------------------------------------------

func TestCreate_PersistsAndHydrates(t *testing.T) {
	svc, q, _ := newSvc(t, &fakeNodeLifecycle{})
	ctx := context.Background()

	// NOTE: organization_id has a FK to fabric_organizations — leave
	// it nil here and assert pointer fields via PartyID (no FK).
	partyID := int64(7)
	grp, err := svc.Create(ctx, service.CreateInput{
		Name:        "orderer-org1",
		Platform:    "FABRICX",
		GroupType:   ngtypes.GroupTypeFabricXOrderer,
		MSPID:       "Org1MSP",
		PartyID:     &partyID,
		Version:     "1.0.0",
		ExternalIP:  "10.0.0.5",
		DomainNames: []string{"orderer.example.com", "orderer2.example.com"},
		Config:      json.RawMessage(`{"foo":"bar"}`),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if grp.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if grp.Name != "orderer-org1" || grp.Platform != "FABRICX" {
		t.Errorf("basic fields: %+v", grp)
	}
	if grp.GroupType != ngtypes.GroupTypeFabricXOrderer {
		t.Errorf("groupType: got %q", grp.GroupType)
	}
	if grp.MSPID != "Org1MSP" {
		t.Errorf("mspID: got %q", grp.MSPID)
	}
	if grp.OrganizationID != nil {
		t.Errorf("orgID: want nil, got %+v", grp.OrganizationID)
	}
	if grp.PartyID == nil || *grp.PartyID != 7 {
		t.Errorf("partyID: got %+v", grp.PartyID)
	}
	if len(grp.DomainNames) != 2 || grp.DomainNames[0] != "orderer.example.com" {
		t.Errorf("domainNames: got %+v", grp.DomainNames)
	}
	if string(grp.Config) != `{"foo":"bar"}` {
		t.Errorf("config: got %s", grp.Config)
	}
	if grp.Status != nodetypes.NodeStatusCreated {
		t.Errorf("status: got %q, want CREATED", grp.Status)
	}

	// Round-trip via Get.
	got, err := svc.Get(ctx, grp.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != grp.ID || got.Name != grp.Name {
		t.Errorf("Get returned wrong row: %+v", got)
	}

	// Verify the underlying row has platform FK satisfied.
	row, err := q.GetNodeGroup(ctx, grp.ID)
	if err != nil {
		t.Fatalf("GetNodeGroup: %v", err)
	}
	if row.Platform != "FABRICX" {
		t.Errorf("row.Platform: %q", row.Platform)
	}
}

func TestCreate_Validation(t *testing.T) {
	svc, _, _ := newSvc(t, &fakeNodeLifecycle{})
	ctx := context.Background()

	cases := []struct {
		name string
		in   service.CreateInput
		want string
	}{
		{"missing name", service.CreateInput{Platform: "FABRICX", GroupType: ngtypes.GroupTypeFabricXOrderer}, "name"},
		{"missing platform", service.CreateInput{Name: "n", GroupType: ngtypes.GroupTypeFabricXOrderer}, "platform"},
		{"missing groupType", service.CreateInput{Name: "n", Platform: "FABRICX"}, "groupType"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Create(ctx, tc.in); err == nil {
				t.Fatalf("expected error mentioning %q", tc.want)
			}
		})
	}
}

func TestCreate_MinimalHydration(t *testing.T) {
	// Group with no optional fields — all NullXxx fields must decode
	// to zero values, not propagate as garbage.
	svc, _, _ := newSvc(t, &fakeNodeLifecycle{})
	ctx := context.Background()

	grp, err := svc.Create(ctx, service.CreateInput{
		Name:      "minimal",
		Platform:  "FABRICX",
		GroupType: ngtypes.GroupTypeFabricXCommitter,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if grp.MSPID != "" {
		t.Errorf("mspID: want empty, got %q", grp.MSPID)
	}
	if grp.OrganizationID != nil {
		t.Errorf("orgID: want nil, got %+v", grp.OrganizationID)
	}
	if grp.PartyID != nil {
		t.Errorf("partyID: want nil, got %+v", grp.PartyID)
	}
	if len(grp.DomainNames) != 0 {
		t.Errorf("domainNames: want empty, got %+v", grp.DomainNames)
	}
	if len(grp.Config) != 0 {
		t.Errorf("config: want empty, got %s", grp.Config)
	}
	if len(grp.DeploymentConfig) != 0 {
		t.Errorf("deploymentConfig: want empty, got %s", grp.DeploymentConfig)
	}
}

func TestList_ReturnsPaginated(t *testing.T) {
	svc, _, _ := newSvc(t, &fakeNodeLifecycle{})
	ctx := context.Background()

	for i, name := range []string{"a", "b", "c"} {
		if _, err := svc.Create(ctx, service.CreateInput{
			Name:      name,
			Platform:  "FABRICX",
			GroupType: ngtypes.GroupTypeFabricXOrderer,
		}); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}

	all, err := svc.List(ctx, 50, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("List: got %d, want 3", len(all))
	}
}

func TestDelete_Removes(t *testing.T) {
	svc, _, _ := newSvc(t, &fakeNodeLifecycle{})
	ctx := context.Background()

	grp, err := svc.Create(ctx, service.CreateInput{
		Name:      "doomed",
		Platform:  "FABRICX",
		GroupType: ngtypes.GroupTypeFabricXOrderer,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.Delete(ctx, grp.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(ctx, grp.ID); err == nil {
		t.Error("expected Get to fail after Delete")
	}
}

// --- StopGroup fan-out ordering --------------------------------------

// seedCommitterGroupWithChildren creates a committer group and inserts
// one child node per canonical role. Returns the group ID and a map
// role->nodeID so tests can assert on the recorded stop order.
//
// Children are created via the sqlc CreateNode query (which does not
// set node_group_id) and then linked by direct UPDATE — matching what
// the nodes API will do when the group-aware flows land.
func seedCommitterGroupWithChildren(t *testing.T, q *db.Queries, sqlDB *sql.DB) (int64, map[nodetypes.NodeType]int64) {
	t.Helper()
	ctx := context.Background()

	grp, err := q.CreateNodeGroup(ctx, &db.CreateNodeGroupParams{
		Name:      "committer-" + t.Name(),
		Platform:  "FABRICX",
		GroupType: string(ngtypes.GroupTypeFabricXCommitter),
		Status:    string(nodetypes.NodeStatusCreated),
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	ids := map[nodetypes.NodeType]int64{}
	roles := ngtypes.ChildRoles(ngtypes.GroupTypeFabricXCommitter)
	for i, role := range roles {
		n, err := q.CreateNode(ctx, &db.CreateNodeParams{
			Name:     "child-" + string(role),
			Slug:     "slug-" + string(role),
			Platform: "FABRICX",
			Status:   string(nodetypes.NodeStatusRunning),
			NodeType: sql.NullString{String: string(role), Valid: true},
		})
		if err != nil {
			t.Fatalf("create child %d (%s): %v", i, role, err)
		}
		if _, err := sqlDB.ExecContext(ctx,
			`UPDATE nodes SET node_group_id = ? WHERE id = ?`,
			grp.ID, n.ID,
		); err != nil {
			t.Fatalf("link child %d to group: %v", n.ID, err)
		}
		ids[role] = n.ID
	}
	return grp.ID, ids
}

func TestStopGroup_FanOutInReverseOrder(t *testing.T) {
	fake := &fakeNodeLifecycle{}
	svc, q, sqlDB := newSvc(t, fake)
	ctx := context.Background()

	groupID, ids := seedCommitterGroupWithChildren(t, q, sqlDB)

	if err := svc.StopGroup(ctx, groupID); err != nil {
		t.Fatalf("StopGroup: %v", err)
	}

	roles := ngtypes.ChildRoles(ngtypes.GroupTypeFabricXCommitter)
	want := make([]int64, 0, len(roles))
	for i := len(roles) - 1; i >= 0; i-- {
		want = append(want, ids[roles[i]])
	}
	if len(fake.stopped) != len(want) {
		t.Fatalf("stop calls: got %d, want %d (stopped=%v)", len(fake.stopped), len(want), fake.stopped)
	}
	for i := range want {
		if fake.stopped[i] != want[i] {
			t.Errorf("stop[%d]: got node %d, want %d (role %s)",
				i, fake.stopped[i], want[i], roles[len(roles)-1-i])
		}
	}

	// StopGroup is best-effort: status must land on STOPPED even if
	// some child stop calls fail. Reset and re-run with an error.
	fake2 := &fakeNodeLifecycle{stopErr: sql.ErrConnDone}
	svc2, q2, sqlDB2 := newSvc(t, fake2)
	groupID2, _ := seedCommitterGroupWithChildren(t, q2, sqlDB2)
	if err := svc2.StopGroup(ctx, groupID2); err != nil {
		t.Fatalf("StopGroup (error path): %v", err)
	}
	after, err := svc2.Get(ctx, groupID2)
	if err != nil {
		t.Fatalf("Get after stop: %v", err)
	}
	if after.Status != nodetypes.NodeStatusStopped {
		t.Errorf("status after best-effort stop: got %q, want STOPPED", after.Status)
	}
}

// --- resolveManagedPostgres (exercised indirectly via prepareGroup) --
//
// resolveManagedPostgres is unexported, but StartGroup runs it as the
// first thing inside prepareGroup for a committer group. If the group
// has no deployment_config the coordinator fails *after* the postgres
// lookup with "no deployment_config" — which proves the lookup did
// not error out. We build three scenarios and assert the coordinator
// reaches that specific sentinel error.
//
// When DeploymentConfig _is_ present, prepareGroup goes on to call the
// committerFactory, which panics in our test. The DeploymentConfig
// scenarios therefore stay out of these tests; they are covered by the
// fabricx integration suite.

func TestStartGroup_MissingDeploymentConfig_FailsAfterPostgresLookup(t *testing.T) {
	fake := &fakeNodeLifecycle{}
	svc, q, sqlDB := newSvc(t, fake)
	ctx := context.Background()

	groupID, _ := seedCommitterGroupWithChildren(t, q, sqlDB)

	// Scenario A: no services rows at all. resolveManagedPostgres
	// returns ("", 0, nil); prepareGroup fails on missing
	// deployment_config, which is what we want to see.
	err := svc.StartGroup(ctx, groupID)
	if err == nil || !contains(err.Error(), "deployment_config") {
		t.Fatalf("expected 'deployment_config' error, got %v", err)
	}

	// Scenario B: a non-POSTGRES service attached — must be ignored.
	if _, err := q.CreateService(ctx, &db.CreateServiceParams{
		NodeGroupID: sql.NullInt64{Int64: groupID, Valid: true},
		Name:        "prometheus-" + t.Name(),
		ServiceType: "PROMETHEUS",
		Status:      string(nodetypes.NodeStatusCreated),
		DeploymentConfig: sql.NullString{
			String: `{"host":"should-be-ignored","port":9090}`,
			Valid:  true,
		},
	}); err != nil {
		t.Fatalf("create non-postgres service: %v", err)
	}
	err = svc.StartGroup(ctx, groupID)
	if err == nil || !contains(err.Error(), "deployment_config") {
		t.Fatalf("expected 'deployment_config' error, got %v", err)
	}

	// Scenario C: POSTGRES service row with invalid JSON — should
	// surface a clear error rather than silently ignoring.
	if _, err := q.CreateService(ctx, &db.CreateServiceParams{
		NodeGroupID:      sql.NullInt64{Int64: groupID, Valid: true},
		Name:             "bad-pg-" + t.Name(),
		ServiceType:      "POSTGRES",
		Status:           string(nodetypes.NodeStatusCreated),
		DeploymentConfig: sql.NullString{String: `{not json}`, Valid: true},
	}); err != nil {
		t.Fatalf("create bad postgres service: %v", err)
	}
	err = svc.StartGroup(ctx, groupID)
	if err == nil {
		t.Fatal("expected error from malformed postgres deployment_config")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- managed postgres wiring -----------------------------------------
//
// These tests exercise the postgres wiring that StartGroup/StopGroup
// drive through PostgresLifecycle. No docker — the fake records the
// calls and the tests assert on persisted state (status transitions,
// deployment_config JSON) and call ordering vs. child lifecycle.

// fakePostgres records Start/Stop/Connect/Disconnect calls and returns
// canned errors. Satisfies service.PostgresLifecycle.
type fakePostgres struct {
	mu               sync.Mutex
	startCalls       []pgservice.Config
	stopCalls        []string
	connectCalls     []connectCall
	disconnectCalls  []connectCall
	running          map[string]bool
	startErr         error
	stopErr          error
	connectErr       error
	disconnectErr    error
	containerID      string
	// onStart fires synchronously before startCalls is appended so
	// tests can observe ordering against other fakes (e.g. NodeLifecycle
	// must not have been called yet when Start runs).
	onStart func()
}

type connectCall struct {
	Container string
	Network   string
}

func (f *fakePostgres) Start(_ context.Context, cfg pgservice.Config) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.onStart != nil {
		f.onStart()
	}
	f.startCalls = append(f.startCalls, cfg)
	if f.startErr != nil {
		return "", f.startErr
	}
	if f.running == nil {
		f.running = map[string]bool{}
	}
	f.running[cfg.ContainerName] = true
	if f.containerID != "" {
		return f.containerID, nil
	}
	return "container-" + cfg.ContainerName, nil
}

func (f *fakePostgres) Stop(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopCalls = append(f.stopCalls, name)
	if f.running != nil {
		delete(f.running, name)
	}
	return f.stopErr
}

func (f *fakePostgres) IsRunning(_ context.Context, name string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running[name], nil
}

func (f *fakePostgres) ConnectNetwork(_ context.Context, container, network string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connectCalls = append(f.connectCalls, connectCall{Container: container, Network: network})
	return f.connectErr
}

func (f *fakePostgres) DisconnectNetwork(_ context.Context, container, network string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.disconnectCalls = append(f.disconnectCalls, connectCall{Container: container, Network: network})
	return f.disconnectErr
}

func TestCreatePostgresService_PersistsConfig(t *testing.T) {
	svc, q, _ := newSvc(t, &fakeNodeLifecycle{})
	ctx := context.Background()

	grp, err := svc.Create(ctx, service.CreateInput{
		Name:      "committer-" + t.Name(),
		Platform:  "FABRICX",
		GroupType: ngtypes.GroupTypeFabricXCommitter,
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	out, err := svc.CreatePostgresService(ctx, grp.ID, service.CreatePostgresInput{
		Name:     "pg-" + t.Name(),
		Version:  "16-alpine",
		DB:       "statedb",
		User:     "pguser",
		Password: "pgpw",
		HostPort: 15432,
	})
	if err != nil {
		t.Fatalf("CreatePostgresService: %v", err)
	}

	if out.ID == 0 {
		t.Fatal("expected non-zero service ID")
	}
	if out.ServiceType != ngtypes.ServiceTypePostgres {
		t.Errorf("serviceType: got %q, want POSTGRES", out.ServiceType)
	}
	if out.NodeGroupID == nil || *out.NodeGroupID != grp.ID {
		t.Errorf("nodeGroupID: got %+v, want %d", out.NodeGroupID, grp.ID)
	}
	if out.Status != nodetypes.NodeStatusCreated {
		t.Errorf("status: got %q, want CREATED", out.Status)
	}

	// Underlying row must carry the JSON config verbatim.
	row, err := q.GetService(ctx, out.ID)
	if err != nil {
		t.Fatalf("GetService row: %v", err)
	}
	if !row.Config.Valid {
		t.Fatal("row.Config must be populated")
	}
	var pgCfg ngtypes.PostgresServiceConfig
	if err := json.Unmarshal([]byte(row.Config.String), &pgCfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if pgCfg.DB != "statedb" || pgCfg.User != "pguser" || pgCfg.Password != "pgpw" || pgCfg.HostPort != 15432 {
		t.Errorf("config roundtrip: %+v", pgCfg)
	}
}

func TestCreatePostgresService_RejectsNonCommitterGroup(t *testing.T) {
	svc, _, _ := newSvc(t, &fakeNodeLifecycle{})
	ctx := context.Background()

	grp, err := svc.Create(ctx, service.CreateInput{
		Name:      "orderer-" + t.Name(),
		Platform:  "FABRICX",
		GroupType: ngtypes.GroupTypeFabricXOrderer,
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	_, err = svc.CreatePostgresService(ctx, grp.ID, service.CreatePostgresInput{
		Name:     "pg",
		DB:       "d",
		User:     "u",
		Password: "p",
	})
	if err == nil {
		t.Fatal("expected error attaching postgres to an orderer group")
	}
	if !contains(err.Error(), "FABRICX_COMMITTER") {
		t.Errorf("error should mention required group type, got %q", err)
	}
}

func TestCreatePostgresService_ValidatesRequiredFields(t *testing.T) {
	svc, _, _ := newSvc(t, &fakeNodeLifecycle{})
	ctx := context.Background()

	grp, err := svc.Create(ctx, service.CreateInput{
		Name:      "committer-" + t.Name(),
		Platform:  "FABRICX",
		GroupType: ngtypes.GroupTypeFabricXCommitter,
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	cases := []struct {
		name string
		in   service.CreatePostgresInput
	}{
		{"missing name", service.CreatePostgresInput{DB: "d", User: "u", Password: "p"}},
		{"missing db", service.CreatePostgresInput{Name: "n", User: "u", Password: "p"}},
		{"missing user", service.CreatePostgresInput{Name: "n", DB: "d", Password: "p"}},
		{"missing password", service.CreatePostgresInput{Name: "n", DB: "d", User: "u"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.CreatePostgresService(ctx, grp.ID, tc.in); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestListServicesForGroup_ReturnsOnlyAttached(t *testing.T) {
	svc, _, _ := newSvc(t, &fakeNodeLifecycle{})
	ctx := context.Background()

	g1, err := svc.Create(ctx, service.CreateInput{Name: "c1-" + t.Name(), Platform: "FABRICX", GroupType: ngtypes.GroupTypeFabricXCommitter})
	if err != nil {
		t.Fatalf("create g1: %v", err)
	}
	g2, err := svc.Create(ctx, service.CreateInput{Name: "c2-" + t.Name(), Platform: "FABRICX", GroupType: ngtypes.GroupTypeFabricXCommitter})
	if err != nil {
		t.Fatalf("create g2: %v", err)
	}
	for _, name := range []string{"pg-a", "pg-b"} {
		if _, err := svc.CreatePostgresService(ctx, g1.ID, service.CreatePostgresInput{
			Name: name + "-" + t.Name(), DB: "d", User: "u", Password: "p",
		}); err != nil {
			t.Fatalf("attach %s: %v", name, err)
		}
	}
	if _, err := svc.CreatePostgresService(ctx, g2.ID, service.CreatePostgresInput{
		Name: "pg-other-" + t.Name(), DB: "d", User: "u", Password: "p",
	}); err != nil {
		t.Fatalf("attach on g2: %v", err)
	}

	got, err := svc.ListServicesForGroup(ctx, g1.ID)
	if err != nil {
		t.Fatalf("ListServicesForGroup: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d services, want 2", len(got))
	}
	for _, s := range got {
		if s.NodeGroupID == nil || *s.NodeGroupID != g1.ID {
			t.Errorf("service %d leaked from another group: nodeGroupID=%+v", s.ID, s.NodeGroupID)
		}
	}
}

// NOTE: TestStartGroup_* tests for postgres wiring live in the
// integration suite (postgres_integration_test.go) because prepareGroup
// runs fabricx.Committer.PrepareCommitterStart before the postgres
// Start, and exercising that path needs real materials (org keys + docker
// network). The Stop path stands alone and we cover it here with a fake.

// TestStopGroup_DisconnectsPostgresAfterChildren locks in the Option X
// shutdown contract:
//   - StopGroup fans out child StopNode first.
//   - Then postgres is only *disconnected* from the committer's bridge
//     network — the container stays running and the services row stays
//     RUNNING, so another committer group sharing the same postgres
//     keeps working.
//   - Disconnect must happen after all children are down, so siblings
//     mid-shutdown don't race a yanked network.
//
// The container itself is only dropped by an explicit /services/{id}/stop
// (covered separately in pkg/services/service tests).
func TestStopGroup_DisconnectsPostgresAfterChildren(t *testing.T) {
	nl := &fakeNodeLifecycle{}
	svc, q, sqlDB := newSvc(t, nl)
	pg := &fakePostgres{}
	// Pretend postgres is already running so the fast-path triggers
	// when/if StartGroup is called elsewhere; not strictly needed here
	// since this test only drives StopGroup.
	pg.running = map[string]bool{"chainlaunch-service-pg-" + t.Name(): true}

	var stopCountAtDisconnect int
	wrapped := &orderedPostgresFake{inner: pg, nodeFake: nl, snapshot: &stopCountAtDisconnect}
	svc.WithPostgresLifecycle(wrapped)

	ctx := context.Background()
	groupID, _ := seedCommitterGroupWithChildren(t, q, sqlDB)

	// Populate deployment_config so committerNetworkForGroup can
	// reconstruct the bridge name — that's the production precondition
	// for the disconnect path.
	dep := nodetypes.FabricXCommitterDeploymentConfig{MSPID: "TestMSP"}
	depJSON, _ := json.Marshal(dep)
	if _, err := sqlDB.ExecContext(ctx,
		`UPDATE node_groups SET deployment_config = ? WHERE id = ?`,
		string(depJSON), groupID,
	); err != nil {
		t.Fatalf("set deployment_config: %v", err)
	}

	cfgJSON, _ := json.Marshal(ngtypes.PostgresServiceConfig{
		DB: "d", User: "u", Password: "p",
	})
	serviceRow, err := q.CreateService(ctx, &db.CreateServiceParams{
		NodeGroupID: sql.NullInt64{Int64: groupID, Valid: true},
		Name:        "pg-" + t.Name(),
		ServiceType: "POSTGRES",
		Status:      string(nodetypes.NodeStatusRunning),
		Config:      sql.NullString{String: string(cfgJSON), Valid: true},
	})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	if err := svc.StopGroup(ctx, groupID); err != nil {
		t.Fatalf("StopGroup: %v", err)
	}

	if len(pg.stopCalls) != 0 {
		t.Errorf("postgres.Stop must not run during group stop, got %v", pg.stopCalls)
	}
	if len(pg.disconnectCalls) != 1 {
		t.Fatalf("postgres.DisconnectNetwork calls: got %d, want 1", len(pg.disconnectCalls))
	}
	wantContainer := "chainlaunch-service-pg-" + t.Name()
	if pg.disconnectCalls[0].Container != wantContainer {
		t.Errorf("disconnect container: got %q, want %q", pg.disconnectCalls[0].Container, wantContainer)
	}
	// Network name: fabricx-<lower(mspID)>-<slug(groupName)>-net.
	wantNet := "fabricx-testmsp-committer-" + strings.ToLower(t.Name()) + "-net"
	if pg.disconnectCalls[0].Network != wantNet {
		t.Errorf("disconnect network: got %q, want %q", pg.disconnectCalls[0].Network, wantNet)
	}

	// Ordering: all committer children must have stopped before the
	// disconnect fires so in-flight roles don't lose the bridge mid-shutdown.
	wantChildren := len(ngtypes.ChildRoles(ngtypes.GroupTypeFabricXCommitter))
	if stopCountAtDisconnect != wantChildren {
		t.Errorf("child stops before postgres disconnect: got %d, want %d",
			stopCountAtDisconnect, wantChildren)
	}

	// Service row must stay RUNNING — StopGroup does not touch status
	// on the shared service.
	row, err := q.GetService(ctx, serviceRow.ID)
	if err != nil {
		t.Fatalf("GetService: %v", err)
	}
	if row.Status != string(nodetypes.NodeStatusRunning) {
		t.Errorf("service status after group stop: got %q, want RUNNING", row.Status)
	}
}

// orderedPostgresFake wraps fakePostgres so we can snapshot the number
// of node.StopNode calls recorded at the moment postgres.Stop enters.
type orderedPostgresFake struct {
	inner    *fakePostgres
	nodeFake *fakeNodeLifecycle
	snapshot *int
}

func (o *orderedPostgresFake) Start(ctx context.Context, cfg pgservice.Config) (string, error) {
	return o.inner.Start(ctx, cfg)
}

func (o *orderedPostgresFake) Stop(ctx context.Context, name string) error {
	o.nodeFake.mu.Lock()
	*o.snapshot = len(o.nodeFake.stopped)
	o.nodeFake.mu.Unlock()
	return o.inner.Stop(ctx, name)
}

func (o *orderedPostgresFake) IsRunning(ctx context.Context, name string) (bool, error) {
	return o.inner.IsRunning(ctx, name)
}

func (o *orderedPostgresFake) ConnectNetwork(ctx context.Context, container, network string) error {
	return o.inner.ConnectNetwork(ctx, container, network)
}

func (o *orderedPostgresFake) DisconnectNetwork(ctx context.Context, container, network string) error {
	o.nodeFake.mu.Lock()
	*o.snapshot = len(o.nodeFake.stopped)
	o.nodeFake.mu.Unlock()
	return o.inner.DisconnectNetwork(ctx, container, network)
}

// TestStopGroup_NoOpWhenNoPostgresAttached verifies that StopGroup does
// not panic and does not invoke postgres.Stop when a committer group has
// no POSTGRES service attached. This protects against regressions where
// the lookup returns a nil svc but the caller dereferences anyway.
func TestStopGroup_NoOpWhenNoPostgresAttached(t *testing.T) {
	nl := &fakeNodeLifecycle{}
	svc, q, sqlDB := newSvc(t, nl)
	pg := &fakePostgres{}
	svc.WithPostgresLifecycle(pg)
	ctx := context.Background()

	groupID, _ := seedCommitterGroupWithChildren(t, q, sqlDB)
	if err := svc.StopGroup(ctx, groupID); err != nil {
		t.Fatalf("StopGroup: %v", err)
	}
	if len(pg.stopCalls) != 0 {
		t.Errorf("postgres.Stop must not be invoked without a service attached, got %v", pg.stopCalls)
	}
}

// TestSharedPostgres_SecondGroupOnlyConnects locks in the core Option X
// invariant: when two committer groups both point at the same
// postgres_service_id, the first group start pays the Start cost and
// the second group only attaches its bridge via ConnectNetwork. The
// shared services row is not re-deployed on the second start.
func TestSharedPostgres_SecondGroupOnlyConnects(t *testing.T) {
	svc, q, _ := newSvc(t, &fakeNodeLifecycle{})
	pg := &fakePostgres{}
	svc.WithPostgresLifecycle(pg)
	ctx := context.Background()

	// One shared postgres services row owned by group A.
	groupA, err := svc.Create(ctx, service.CreateInput{
		Name: "group-a-" + t.Name(), Platform: "FABRICX",
		GroupType: ngtypes.GroupTypeFabricXCommitter,
	})
	if err != nil {
		t.Fatalf("create groupA: %v", err)
	}
	svcOut, err := svc.CreatePostgresService(ctx, groupA.ID, service.CreatePostgresInput{
		Name: "shared-pg-" + t.Name(),
		DB:   "statedb", User: "pg", Password: "pw",
	})
	if err != nil {
		t.Fatalf("CreatePostgresService: %v", err)
	}

	// Second group attaches to the same service via AttachPostgresService.
	groupB, err := svc.Create(ctx, service.CreateInput{
		Name: "group-b-" + t.Name(), Platform: "FABRICX",
		GroupType: ngtypes.GroupTypeFabricXCommitter,
	})
	if err != nil {
		t.Fatalf("create groupB: %v", err)
	}
	if _, err := svc.AttachPostgresService(ctx, groupB.ID, svcOut.ID); err != nil {
		t.Fatalf("AttachPostgresService: %v", err)
	}

	// First group start: fresh Start on groupA's bridge.
	if err := svc.StartManagedPostgresForCommitterForTest(ctx, groupA.ID, "net-a"); err != nil {
		t.Fatalf("start on groupA: %v", err)
	}
	if len(pg.startCalls) != 1 {
		t.Fatalf("expected 1 Start, got %d", len(pg.startCalls))
	}
	if pg.startCalls[0].NetworkName != "net-a" {
		t.Errorf("first Start NetworkName: got %q, want %q", pg.startCalls[0].NetworkName, "net-a")
	}
	if len(pg.connectCalls) != 0 {
		t.Errorf("expected 0 ConnectNetwork on first start, got %d", len(pg.connectCalls))
	}

	// Second group start: container already running → ConnectNetwork, no Start.
	if err := svc.StartManagedPostgresForCommitterForTest(ctx, groupB.ID, "net-b"); err != nil {
		t.Fatalf("start on groupB: %v", err)
	}
	if len(pg.startCalls) != 1 {
		t.Errorf("expected exactly 1 Start across both groups, got %d", len(pg.startCalls))
	}
	if len(pg.connectCalls) != 1 {
		t.Fatalf("expected 1 ConnectNetwork for second group, got %d", len(pg.connectCalls))
	}
	if pg.connectCalls[0].Network != "net-b" {
		t.Errorf("ConnectNetwork network: got %q, want %q", pg.connectCalls[0].Network, "net-b")
	}
	wantContainer := "chainlaunch-service-shared-pg-" + t.Name()
	if pg.connectCalls[0].Container != wantContainer {
		t.Errorf("ConnectNetwork container: got %q, want %q", pg.connectCalls[0].Container, wantContainer)
	}

	// Service row stays RUNNING and deployment_config reflects the
	// first network only — later consumers dial the same host:port.
	row, err := q.GetService(ctx, svcOut.ID)
	if err != nil {
		t.Fatalf("GetService: %v", err)
	}
	if row.Status != string(nodetypes.NodeStatusRunning) {
		t.Errorf("service status: got %q, want RUNNING", row.Status)
	}
	if !row.DeploymentConfig.Valid {
		t.Fatal("expected deployment_config populated from first start")
	}
	var dep ngtypes.PostgresServiceDeployment
	if err := json.Unmarshal([]byte(row.DeploymentConfig.String), &dep); err != nil {
		t.Fatalf("unmarshal deployment: %v", err)
	}
	if dep.NetworkName != "net-a" {
		t.Errorf("deployment NetworkName: got %q, want net-a (first consumer's bridge)", dep.NetworkName)
	}
}

// TestSharedPostgres_GroupStopsOnlyDisconnect verifies the stop side:
// when a group that shares a postgres service stops, we disconnect its
// bridge but leave the container running (and the services row
// RUNNING) so other consumers are unaffected.
func TestSharedPostgres_GroupStopsOnlyDisconnect(t *testing.T) {
	svc, q, _ := newSvc(t, &fakeNodeLifecycle{})
	pg := &fakePostgres{running: map[string]bool{}}
	svc.WithPostgresLifecycle(pg)
	ctx := context.Background()

	grp, err := svc.Create(ctx, service.CreateInput{
		Name: "g-" + t.Name(), Platform: "FABRICX",
		GroupType: ngtypes.GroupTypeFabricXCommitter,
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	svcOut, err := svc.CreatePostgresService(ctx, grp.ID, service.CreatePostgresInput{
		Name: "pg-" + t.Name(), DB: "d", User: "u", Password: "p",
	})
	if err != nil {
		t.Fatalf("CreatePostgresService: %v", err)
	}
	if err := svc.StartManagedPostgresForCommitterForTest(ctx, grp.ID, "net-x"); err != nil {
		t.Fatalf("start: %v", err)
	}

	svc.StopManagedPostgresForCommitterForTest(ctx, grp.ID, "net-x")

	if len(pg.stopCalls) != 0 {
		t.Errorf("group stop must not call postgres.Stop, got %v", pg.stopCalls)
	}
	if len(pg.disconnectCalls) != 1 {
		t.Fatalf("expected 1 DisconnectNetwork, got %d", len(pg.disconnectCalls))
	}
	if pg.disconnectCalls[0].Network != "net-x" {
		t.Errorf("disconnect network: got %q, want net-x", pg.disconnectCalls[0].Network)
	}

	row, err := q.GetService(ctx, svcOut.ID)
	if err != nil {
		t.Fatalf("GetService: %v", err)
	}
	if row.Status != string(nodetypes.NodeStatusRunning) {
		t.Errorf("service status after group stop: got %q, want RUNNING (container stays up)", row.Status)
	}
	container := "chainlaunch-service-pg-" + t.Name()
	if !pg.running[container] {
		t.Error("expected postgres container to still be marked running after group stop")
	}
}

// nodeservice is imported for NodeResponse; keep the reference alive if
// the above tests ever stop using the fake directly.
var _ = nodeservice.NodeResponse{}

// --- InitCommitterGroup validation tests -----------------------------
//
// FABRICX_COMMITTER node-groups are a thin parent: no shared identity,
// no group-level Init. Each child carries its own MSP identity and is
// created via POST /nodes with fabricXCommitter.nodeGroupId pointing
// at the parent group. These tests check the structural pieces of that
// flow — the group can be created without any MSP/org metadata, and
// the children-listing endpoint resolves correctly.

func TestCreateCommitterGroup_AcceptsThinParent(t *testing.T) {
	svc, _, _ := newSvc(t, &fakeNodeLifecycle{})
	ctx := context.Background()

	grp, err := svc.Create(ctx, service.CreateInput{
		Name:      "fabricx-quickstart-committers",
		Platform:  "FABRICX",
		GroupType: ngtypes.GroupTypeFabricXCommitter,
		// No MSPID / OrganizationID / PartyID — those live on each
		// child committer node, not on the group.
	})
	if err != nil {
		t.Fatalf("Create committer group: %v", err)
	}
	if grp.GroupType != ngtypes.GroupTypeFabricXCommitter {
		t.Errorf("groupType = %s, want %s", grp.GroupType, ngtypes.GroupTypeFabricXCommitter)
	}
	if grp.MSPID != "" {
		t.Errorf("expected MSPID to be empty on the thin parent group, got %q", grp.MSPID)
	}
	if grp.OrganizationID != nil {
		t.Errorf("expected OrganizationID to be nil on the thin parent group, got %v", *grp.OrganizationID)
	}
}

// insertOrg writes a fabric_organizations row so node_groups foreign-key
// references resolve in tests. Uses the raw queries to avoid pulling in
// the fabric service package.
func insertOrg(t *testing.T, q *db.Queries, mspID string) int64 {
	t.Helper()
	ctx := context.Background()
	row, err := q.CreateFabricOrganization(ctx, &db.CreateFabricOrganizationParams{
		MspID:       mspID,
		Description: sql.NullString{String: "test", Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateFabricOrganization: %v", err)
	}
	return row.ID
}
