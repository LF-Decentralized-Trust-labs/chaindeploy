package notifications

import (
	"context"
)

// Service defines the interface for notification services
type Service interface {
	// SendBackupSuccessNotification sends a notification about a successful backup
	SendBackupSuccessNotification(ctx context.Context, data BackupSuccessData) error

	// SendBackupFailureNotification sends a notification about a failed backup
	SendBackupFailureNotification(ctx context.Context, data BackupFailureData) error

	// SendS3ConnectionIssueNotification sends a notification about S3 connection issues
	SendS3ConnectionIssueNotification(ctx context.Context, data S3ConnectionIssueData) error

	// SendNodeDowntimeNotification sends a notification about node downtime
	SendNodeDowntimeNotification(ctx context.Context, data NodeDowntimeData) error

	// SendNodeRecoveryNotification sends a notification about a node that has recovered
	SendNodeRecoveryNotification(ctx context.Context, data NodeUpData) error

	// SendDiskSpaceWarningNotification sends a notification about disk space usage
	SendDiskSpaceWarningNotification(ctx context.Context, data DiskSpaceWarningData) error
}
