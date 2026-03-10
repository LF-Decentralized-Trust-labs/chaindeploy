package monitoring

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/notifications"
)

// mockNotificationService is a mock implementation of the notification service
type mockNotificationService struct {
	diskSpaceWarnings []notifications.DiskSpaceWarningData
}

func (m *mockNotificationService) SendBackupSuccessNotification(ctx context.Context, data notifications.BackupSuccessData) error {
	return nil
}

func (m *mockNotificationService) SendBackupFailureNotification(ctx context.Context, data notifications.BackupFailureData) error {
	return nil
}

func (m *mockNotificationService) SendS3ConnectionIssueNotification(ctx context.Context, data notifications.S3ConnectionIssueData) error {
	return nil
}

func (m *mockNotificationService) SendNodeDowntimeNotification(ctx context.Context, data notifications.NodeDowntimeData) error {
	return nil
}

func (m *mockNotificationService) SendNodeRecoveryNotification(ctx context.Context, data notifications.NodeUpData) error {
	return nil
}

func (m *mockNotificationService) SendDiskSpaceWarningNotification(ctx context.Context, data notifications.DiskSpaceWarningData) error {
	m.diskSpaceWarnings = append(m.diskSpaceWarnings, data)
	return nil
}

func TestNewDiskSpaceMonitor(t *testing.T) {
	log := logger.NewDefault()
	mockSvc := &mockNotificationService{}

	monitor := NewDiskSpaceMonitor("/tmp", mockSvc, log, 80.0)

	if monitor.dataPath != "/tmp" {
		t.Errorf("expected dataPath to be /tmp, got %s", monitor.dataPath)
	}
	if monitor.threshold != 80.0 {
		t.Errorf("expected threshold to be 80.0, got %f", monitor.threshold)
	}
	if monitor.checkInterval != 5*time.Minute {
		t.Errorf("expected checkInterval to be 5 minutes, got %v", monitor.checkInterval)
	}
	if monitor.alertCooldown != 3*time.Hour {
		t.Errorf("expected alertCooldown to be 3 hours, got %v", monitor.alertCooldown)
	}
}

func TestGetDiskStats(t *testing.T) {
	log := logger.NewDefault()
	mockSvc := &mockNotificationService{}

	// Use a path that exists
	monitor := NewDiskSpaceMonitor("/tmp", mockSvc, log, 80.0)

	stats, err := monitor.getDiskStats()
	if err != nil {
		t.Fatalf("getDiskStats failed: %v", err)
	}

	if stats.TotalBytes <= 0 {
		t.Error("expected TotalBytes to be positive")
	}
	if stats.UsedBytes < 0 {
		t.Error("expected UsedBytes to be non-negative")
	}
	if stats.AvailableBytes < 0 {
		t.Error("expected AvailableBytes to be non-negative")
	}
	if stats.DataPath != "/tmp" {
		t.Errorf("expected DataPath to be /tmp, got %s", stats.DataPath)
	}
}

func TestGetDiskStatsInvalidPath(t *testing.T) {
	log := logger.NewDefault()
	mockSvc := &mockNotificationService{}

	monitor := NewDiskSpaceMonitor("/nonexistent/path/that/does/not/exist", mockSvc, log, 80.0)

	_, err := monitor.getDiskStats()
	if err == nil {
		t.Error("expected error for nonexistent path, got nil")
	}
}

func TestGetCurrentStats(t *testing.T) {
	log := logger.NewDefault()
	mockSvc := &mockNotificationService{}

	monitor := NewDiskSpaceMonitor("/tmp", mockSvc, log, 75.0)

	stats, err := monitor.GetCurrentStats()
	if err != nil {
		t.Fatalf("GetCurrentStats failed: %v", err)
	}

	if stats.UsedPercent < 0 || stats.UsedPercent > 100 {
		t.Errorf("expected UsedPercent to be between 0 and 100, got %f", stats.UsedPercent)
	}
	if stats.Threshold != 75.0 {
		t.Errorf("expected Threshold to be 75.0, got %f", stats.Threshold)
	}
}

func TestSetThreshold(t *testing.T) {
	log := logger.NewDefault()
	mockSvc := &mockNotificationService{}

	monitor := NewDiskSpaceMonitor("/tmp", mockSvc, log, 80.0)

	monitor.SetThreshold(90.0)
	if monitor.threshold != 90.0 {
		t.Errorf("expected threshold to be 90.0 after SetThreshold, got %f", monitor.threshold)
	}
}

func TestSetCheckInterval(t *testing.T) {
	log := logger.NewDefault()
	mockSvc := &mockNotificationService{}

	monitor := NewDiskSpaceMonitor("/tmp", mockSvc, log, 80.0)

	monitor.SetCheckInterval(10 * time.Minute)
	if monitor.checkInterval != 10*time.Minute {
		t.Errorf("expected checkInterval to be 10 minutes after SetCheckInterval, got %v", monitor.checkInterval)
	}
}

func TestCheckDiskSpaceNormal(t *testing.T) {
	log := logger.NewDefault()
	mockSvc := &mockNotificationService{}

	// Use 100% threshold so it won't trigger an alert for /tmp
	monitor := NewDiskSpaceMonitor("/tmp", mockSvc, log, 100.0)

	ctx := context.Background()
	monitor.checkDiskSpace(ctx)

	if len(mockSvc.diskSpaceWarnings) > 0 {
		t.Error("expected no disk space warnings when threshold is 100%")
	}
}

func TestCheckDiskSpaceWithLowThreshold(t *testing.T) {
	log := logger.NewDefault()
	mockSvc := &mockNotificationService{}

	// Use 0.001% threshold so it will always trigger an alert
	monitor := NewDiskSpaceMonitor("/tmp", mockSvc, log, 0.001)

	ctx := context.Background()
	monitor.checkDiskSpace(ctx)

	if len(mockSvc.diskSpaceWarnings) == 0 {
		t.Error("expected disk space warning when threshold is very low")
	}
	if len(mockSvc.diskSpaceWarnings) > 0 {
		warning := mockSvc.diskSpaceWarnings[0]
		if warning.Threshold != 0.001 {
			t.Errorf("expected Threshold in warning to be 0.001, got %f", warning.Threshold)
		}
	}
}

func TestAlertCooldown(t *testing.T) {
	log := logger.NewDefault()
	mockSvc := &mockNotificationService{}

	// Use 0.001% threshold so it will always trigger an alert
	monitor := NewDiskSpaceMonitor("/tmp", mockSvc, log, 0.001)
	// Set a long cooldown
	monitor.alertCooldown = 1 * time.Hour

	ctx := context.Background()

	// First check should trigger an alert
	monitor.checkDiskSpace(ctx)
	if len(mockSvc.diskSpaceWarnings) != 1 {
		t.Fatalf("expected 1 disk space warning after first check, got %d", len(mockSvc.diskSpaceWarnings))
	}

	// Second check should not trigger due to cooldown
	monitor.checkDiskSpace(ctx)
	if len(mockSvc.diskSpaceWarnings) != 1 {
		t.Errorf("expected 1 disk space warning after second check (cooldown), got %d", len(mockSvc.diskSpaceWarnings))
	}
}

func TestStartAndStop(t *testing.T) {
	log := logger.NewDefault()
	mockSvc := &mockNotificationService{}

	monitor := NewDiskSpaceMonitor("/tmp", mockSvc, log, 100.0)
	monitor.checkInterval = 100 * time.Millisecond // Short interval for testing

	ctx, cancel := context.WithCancel(context.Background())

	// Start the monitor in a goroutine
	done := make(chan struct{})
	go func() {
		monitor.Start(ctx)
		close(done)
	}()

	// Let it run for a bit
	time.Sleep(250 * time.Millisecond)

	// Stop via context cancellation
	cancel()

	// Wait for the monitor to stop
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("monitor did not stop within timeout")
	}
}

func TestStartAndStopViaChannel(t *testing.T) {
	log := logger.NewDefault()
	mockSvc := &mockNotificationService{}

	monitor := NewDiskSpaceMonitor("/tmp", mockSvc, log, 100.0)
	monitor.checkInterval = 100 * time.Millisecond // Short interval for testing

	ctx := context.Background()

	// Start the monitor in a goroutine
	done := make(chan struct{})
	go func() {
		monitor.Start(ctx)
		close(done)
	}()

	// Let it run for a bit
	time.Sleep(250 * time.Millisecond)

	// Stop via Stop method
	monitor.Stop()

	// Wait for the monitor to stop
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("monitor did not stop within timeout")
	}
}

func TestDiskStatsDataPath(t *testing.T) {
	log := logger.NewDefault()
	mockSvc := &mockNotificationService{}

	// Test with home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("could not get user home directory")
	}

	monitor := NewDiskSpaceMonitor(homeDir, mockSvc, log, 80.0)

	stats, err := monitor.getDiskStats()
	if err != nil {
		t.Fatalf("getDiskStats failed: %v", err)
	}

	if stats.DataPath != homeDir {
		t.Errorf("expected DataPath to be %s, got %s", homeDir, stats.DataPath)
	}
}
