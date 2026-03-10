package monitoring

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/notifications"
	"golang.org/x/sys/unix"
)

// DiskSpaceMonitor monitors disk space usage for the ChainDeploy data directory
type DiskSpaceMonitor struct {
	dataPath        string
	notificationSvc notifications.Service
	logger          *logger.Logger
	threshold       float64 // Percentage threshold (e.g., 80.0 for 80%)
	checkInterval   time.Duration
	lastAlertTime   time.Time
	alertCooldown   time.Duration // Minimum time between alerts
	stopChan        chan struct{}
}

// NewDiskSpaceMonitor creates a new disk space monitor
func NewDiskSpaceMonitor(dataPath string, notificationSvc notifications.Service, logger *logger.Logger, threshold float64) *DiskSpaceMonitor {
	return &DiskSpaceMonitor{
		dataPath:        dataPath,
		notificationSvc: notificationSvc,
		logger:          logger,
		threshold:       threshold,
		checkInterval:   5 * time.Minute, // Check every 5 minutes
		alertCooldown:   3 * time.Hour,   // Alert at most once per 3 hours
		stopChan:        make(chan struct{}),
	}
}

// Start begins monitoring disk space
func (m *DiskSpaceMonitor) Start(ctx context.Context) {
	m.logger.Info("Starting disk space monitoring", "path", m.dataPath, "threshold", m.threshold)

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	// Check immediately on start
	m.checkDiskSpace(ctx)

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("Stopping disk space monitoring")
			return
		case <-m.stopChan:
			m.logger.Info("Stopping disk space monitoring")
			return
		case <-ticker.C:
			m.checkDiskSpace(ctx)
		}
	}
}

// Stop stops the disk space monitoring
func (m *DiskSpaceMonitor) Stop() {
	close(m.stopChan)
}

// checkDiskSpace checks the current disk space usage and sends notifications if needed
func (m *DiskSpaceMonitor) checkDiskSpace(ctx context.Context) {
	stats, err := m.getDiskStats()
	if err != nil {
		m.logger.Error("Failed to get disk stats", "error", err)
		return
	}

	usedPercent := float64(stats.UsedBytes) / float64(stats.TotalBytes) * 100

	m.logger.Debug("Disk space check",
		"path", m.dataPath,
		"used", stats.UsedBytes,
		"total", stats.TotalBytes,
		"usedPercent", fmt.Sprintf("%.2f%%", usedPercent),
		"threshold", fmt.Sprintf("%.2f%%", m.threshold))

	// Check if we've exceeded the threshold
	if usedPercent >= m.threshold {
		// Check if we're within the cooldown period
		if time.Since(m.lastAlertTime) < m.alertCooldown {
			m.logger.Debug("Disk space threshold exceeded but within cooldown period")
			return
		}

		m.logger.Warn("Disk space threshold exceeded",
			"path", m.dataPath,
			"usedPercent", fmt.Sprintf("%.2f%%", usedPercent),
			"threshold", fmt.Sprintf("%.2f%%", m.threshold))

		// Send notification
		if err := m.sendDiskSpaceAlert(ctx, stats, usedPercent); err != nil {
			m.logger.Error("Failed to send disk space alert", "error", err)
		} else {
			m.lastAlertTime = time.Now()
		}
	}
}

// getDiskStats retrieves disk usage statistics for the data path
func (m *DiskSpaceMonitor) getDiskStats() (*notifications.DiskSpaceWarningData, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(m.dataPath, &stat); err != nil {
		return nil, fmt.Errorf("failed to get filesystem stats: %w", err)
	}

	// Calculate disk space metrics
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	availableBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - availableBytes

	safeInt64 := func(v uint64) int64 {
		if v > uint64(math.MaxInt64) {
			return math.MaxInt64
		}
		return int64(v)
	}

	return &notifications.DiskSpaceWarningData{
		DataPath:       m.dataPath,
		UsedBytes:      safeInt64(usedBytes),
		AvailableBytes: safeInt64(availableBytes),
		TotalBytes:     safeInt64(totalBytes),
		DetectedTime:   time.Now(),
		MountPoint:     m.dataPath, // This could be enhanced to get the actual mount point
	}, nil
}

// sendDiskSpaceAlert sends a disk space warning notification
func (m *DiskSpaceMonitor) sendDiskSpaceAlert(ctx context.Context, stats *notifications.DiskSpaceWarningData, usedPercent float64) error {
	stats.UsedPercent = usedPercent
	stats.Threshold = m.threshold

	return m.notificationSvc.SendDiskSpaceWarningNotification(ctx, *stats)
}

// SetThreshold updates the threshold percentage
func (m *DiskSpaceMonitor) SetThreshold(threshold float64) {
	m.threshold = threshold
	m.logger.Info("Updated disk space threshold", "threshold", fmt.Sprintf("%.2f%%", threshold))
}

// SetCheckInterval updates the check interval
func (m *DiskSpaceMonitor) SetCheckInterval(interval time.Duration) {
	m.checkInterval = interval
	m.logger.Info("Updated disk space check interval", "interval", interval)
}

// GetCurrentStats returns the current disk space statistics without sending alerts
func (m *DiskSpaceMonitor) GetCurrentStats() (*notifications.DiskSpaceWarningData, error) {
	stats, err := m.getDiskStats()
	if err != nil {
		return nil, err
	}

	usedPercent := float64(stats.UsedBytes) / float64(stats.TotalBytes) * 100
	stats.UsedPercent = usedPercent
	stats.Threshold = m.threshold

	return stats, nil
}
