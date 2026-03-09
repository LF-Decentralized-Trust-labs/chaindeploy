package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	metricscommon "github.com/chainlaunch/chainlaunch/pkg/metrics/common"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMonitoringService is a mock implementation of the MonitoringService interface
type mockMonitoringService struct {
	removeNodeCalls []int64
	removeNodeErr   error
	addNodeCalls    int
	triggerCalls    int
}

func (m *mockMonitoringService) AddNodeToMonitor(nodeID int64, name string, endpoint string, platform string, nodeType string, networkNames []string) error {
	m.addNodeCalls++
	return nil
}

func (m *mockMonitoringService) TriggerImmediateCheckForNode(ctx context.Context, nodeID int64) {
	m.triggerCalls++
}

func (m *mockMonitoringService) RemoveNode(nodeID int64) error {
	m.removeNodeCalls = append(m.removeNodeCalls, nodeID)
	return m.removeNodeErr
}

// mockMetricsService is a mock implementation of the metricscommon.Service interface
type mockMetricsService struct {
	reloadCalls int
}

func (m *mockMetricsService) Start(ctx context.Context, config *metricscommon.Config) error {
	return nil
}
func (m *mockMetricsService) Stop(ctx context.Context) error { return nil }
func (m *mockMetricsService) GetStatus(ctx context.Context) (*metricscommon.Status, error) {
	return nil, nil
}
func (m *mockMetricsService) Reload(ctx context.Context) error {
	m.reloadCalls++
	return nil
}
func (m *mockMetricsService) Query(ctx context.Context, query string) (*metricscommon.QueryResult, error) {
	return nil, nil
}
func (m *mockMetricsService) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*metricscommon.QueryResult, error) {
	return nil, nil
}
func (m *mockMetricsService) GetLabelValues(ctx context.Context, labelName string, matches []string) ([]string, error) {
	return nil, nil
}
func (m *mockMetricsService) GetDefaults(ctx context.Context) (interface{}, error) {
	return nil, nil
}
func (m *mockMetricsService) CheckPortAvailability(ctx context.Context, port int) bool {
	return true
}
func (m *mockMetricsService) RefreshPrometheus(ctx context.Context, req interface{}) error {
	return nil
}
func (m *mockMetricsService) QueryMetrics(ctx context.Context, nodeID int64, query string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockMetricsService) QueryMetricsRange(ctx context.Context, nodeID int64, query string, start, end time.Time, step time.Duration) (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockMetricsService) GetLabelValuesForNode(ctx context.Context, nodeID int64, labelName string, matches []string) ([]string, error) {
	return nil, nil
}
func (m *mockMetricsService) QueryForNode(ctx context.Context, nodeID int64, query string) (*metricscommon.QueryResult, error) {
	return nil, nil
}
func (m *mockMetricsService) QueryRangeForNode(ctx context.Context, nodeID int64, query string, start, end time.Time, step time.Duration) (*metricscommon.QueryResult, error) {
	return nil, nil
}
func (m *mockMetricsService) GetCurrentConfig(ctx context.Context) (*metricscommon.Config, error) {
	return nil, nil
}
func (m *mockMetricsService) TailLogs(ctx context.Context, tail int, follow bool) (<-chan string, error) {
	return nil, nil
}

// newTestLogger creates a logger suitable for tests
func newTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	l, err := logger.New(&logger.Config{
		Level:      "warn",
		OutputPath: "stderr",
		Format:     "console",
	})
	require.NoError(t, err)
	return l
}

// setupTestService creates a NodeService with mocks and a test database,
// returning the service, monitoring mock, metrics mock, and raw sql.DB
func setupTestService(t *testing.T) (*NodeService, *mockMonitoringService, *mockMetricsService, *sql.DB) {
	t.Helper()

	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	_, err = sqlDB.Exec(`
		CREATE TABLE nodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			slug TEXT NOT NULL DEFAULT '',
			platform TEXT NOT NULL,
			status TEXT NOT NULL,
			description TEXT,
			network_id INTEGER,
			config TEXT,
			resources TEXT,
			endpoint TEXT,
			public_endpoint TEXT,
			p2p_address TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_by INTEGER,
			updated_at TIMESTAMP,
			fabric_organization_id INTEGER,
			node_type TEXT,
			node_config TEXT,
			deployment_config TEXT,
			error_message TEXT
		)
	`)
	require.NoError(t, err)

	queries := db.New(sqlDB)
	log := newTestLogger(t)
	monMock := &mockMonitoringService{}
	metricsMock := &mockMetricsService{}

	svc := &NodeService{
		db:                queries,
		logger:            log,
		monitoringService: monMock,
		metricsService:    metricsMock,
	}

	return svc, monMock, metricsMock, sqlDB
}

func TestDeleteNode_calls_monitoring_RemoveNode(t *testing.T) {
	svc, monMock, metricsMock, sqlDB := setupTestService(t)
	ctx := context.Background()

	// Insert a stopped node (so DeleteNode does not try to stop it)
	result, err := sqlDB.Exec(
		`INSERT INTO nodes (name, slug, platform, status, created_at) VALUES (?, ?, ?, ?, ?)`,
		"test-node", "test-node", "fabric", "stopped", time.Now(),
	)
	require.NoError(t, err)
	nodeID, err := result.LastInsertId()
	require.NoError(t, err)

	// Act
	err = svc.DeleteNode(ctx, nodeID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []int64{nodeID}, monMock.removeNodeCalls,
		"RemoveNode should be called with the deleted node's ID")
	assert.Equal(t, 1, metricsMock.reloadCalls,
		"metrics Reload should be called after deletion")
}

func TestDeleteNode_monitoring_RemoveNode_failure_is_non_blocking(t *testing.T) {
	svc, monMock, metricsMock, sqlDB := setupTestService(t)
	ctx := context.Background()

	// Configure monitoring mock to return an error
	monMock.removeNodeErr = fmt.Errorf("monitoring service unavailable")

	// Insert a stopped node
	result, err := sqlDB.Exec(
		`INSERT INTO nodes (name, slug, platform, status, created_at) VALUES (?, ?, ?, ?, ?)`,
		"test-node", "test-node", "fabric", "stopped", time.Now(),
	)
	require.NoError(t, err)
	nodeID, err := result.LastInsertId()
	require.NoError(t, err)

	// Act
	err = svc.DeleteNode(ctx, nodeID)

	// Assert: deletion succeeds even though monitoring removal failed
	require.NoError(t, err, "DeleteNode should succeed even when RemoveNode fails")
	assert.Equal(t, []int64{nodeID}, monMock.removeNodeCalls,
		"RemoveNode should still be called despite returning an error")
	assert.Equal(t, 1, metricsMock.reloadCalls,
		"metrics Reload should still be called after deletion")

	// Verify the node was actually deleted from the database
	_, dbErr := svc.db.GetNode(ctx, nodeID)
	assert.ErrorIs(t, dbErr, sql.ErrNoRows,
		"node should be deleted from database even if monitoring removal fails")
}

func TestDeleteNode_nil_monitoring_service_does_not_panic(t *testing.T) {
	// Setup service without monitoring service (nil)
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	_, err = sqlDB.Exec(`
		CREATE TABLE nodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			slug TEXT NOT NULL DEFAULT '',
			platform TEXT NOT NULL,
			status TEXT NOT NULL,
			description TEXT,
			network_id INTEGER,
			config TEXT,
			resources TEXT,
			endpoint TEXT,
			public_endpoint TEXT,
			p2p_address TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_by INTEGER,
			updated_at TIMESTAMP,
			fabric_organization_id INTEGER,
			node_type TEXT,
			node_config TEXT,
			deployment_config TEXT,
			error_message TEXT
		)
	`)
	require.NoError(t, err)

	queries := db.New(sqlDB)
	log := newTestLogger(t)
	metricsMock := &mockMetricsService{}

	svc := &NodeService{
		db:                queries,
		logger:            log,
		monitoringService: nil, // explicitly nil
		metricsService:    metricsMock,
	}

	// Insert a stopped node
	result, err := sqlDB.Exec(
		`INSERT INTO nodes (name, slug, platform, status, created_at) VALUES (?, ?, ?, ?, ?)`,
		"test-node", "test-node", "fabric", "stopped", time.Now(),
	)
	require.NoError(t, err)
	nodeID, err := result.LastInsertId()
	require.NoError(t, err)

	// Act: should not panic when monitoring service is nil
	ctx := context.Background()
	err = svc.DeleteNode(ctx, nodeID)

	// Assert
	require.NoError(t, err, "DeleteNode should succeed when monitoring service is nil")
	assert.Equal(t, 1, metricsMock.reloadCalls,
		"metrics Reload should still be called")
}

func TestDeleteNode_node_not_found_skips_monitoring_removal(t *testing.T) {
	svc, monMock, _, _ := setupTestService(t)
	ctx := context.Background()

	// Act: delete a node that does not exist
	err := svc.DeleteNode(ctx, 99999)

	// Assert: should return nil (not found is treated as already deleted)
	require.NoError(t, err)
	assert.Empty(t, monMock.removeNodeCalls,
		"RemoveNode should not be called when the node does not exist")
}
