package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"crypto/rand"
	"encoding/base64"

	"github.com/chainlaunch/chainlaunch/pkg/config"
	"github.com/chainlaunch/chainlaunch/pkg/crypto"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/notifications"
	notificationService "github.com/chainlaunch/chainlaunch/pkg/notifications/service"
	"github.com/robfig/cron/v3"
)

// BackupCheckInterval is the interval at which the backup service checks for completed backups
const BackupCheckInterval = 1 * time.Minute

// BackupService handles backup operations
type BackupService struct {
	queries             *db.Queries
	cron                *cron.Cron
	logger              *logger.Logger
	notificationService *notificationService.NotificationService
	cronEntryIDs        map[int64]cron.EntryID
	mu                  sync.Mutex
	stopCh              chan struct{}
	databasePath        string
	configService       *config.ConfigService
	encryptor           *crypto.Encryptor
}

// NewBackupService creates a new backup service.
// The encryptor parameter is used to encrypt/decrypt sensitive backup target
// credentials (secret keys, restic passwords). Pass nil to disable encryption
// (not recommended for production).
func NewBackupService(
	queries *db.Queries,
	logger *logger.Logger,
	notificationSvc *notificationService.NotificationService,
	databasePath string,
	configService *config.ConfigService,
	encryptor *crypto.Encryptor,
) *BackupService {
	c := cron.New(cron.WithSeconds())
	c.Start()

	service := &BackupService{
		queries:             queries,
		cron:                c,
		logger:              logger,
		notificationService: notificationSvc,
		cronEntryIDs:        make(map[int64]cron.EntryID),
		stopCh:              make(chan struct{}),
		databasePath:        databasePath,
		configService:       configService,
		encryptor:           encryptor,
	}

	// Load and schedule existing backup schedules
	service.loadExistingSchedules()

	return service
}

// loadExistingSchedules loads and schedules existing backup schedules
func (s *BackupService) loadExistingSchedules() {
	ctx := context.Background()
	schedules, err := s.queries.ListBackupSchedules(ctx)
	if err != nil {
		return
	}

	for _, schedule := range schedules {
		if schedule.Enabled {
			s.scheduleBackup(schedule)
		}
	}
}

// CreateBackupTarget creates a new backup target
func (s *BackupService) CreateBackupTarget(ctx context.Context, params CreateBackupTargetParams) (*BackupTargetDTO, error) {
	// Validate config based on type
	if params.Type == BackupTargetTypeS3 {
		// Validate required S3 fields
		if params.BucketName == "" || params.Endpoint == "" ||
			params.BucketPath == "" || params.AccessKeyID == "" ||
			params.SecretKey == "" {
			return nil, fmt.Errorf("missing required S3 configuration fields")
		}

		// Validate endpoint URL format
		_, err := url.Parse(params.Endpoint)
		if err != nil {
			return nil, fmt.Errorf("invalid endpoint URL: %w", err)
		}
	}

	// Generate restic password
	resticPassword, err := generateSecurePassword()
	if err != nil {
		return nil, fmt.Errorf("failed to generate restic password: %w", err)
	}

	// Encrypt sensitive credentials before storing
	encAccessKeyID, err := s.encryptSecret(params.AccessKeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt access key ID: %w", err)
	}
	encSecretKey, err := s.encryptSecret(params.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret key: %w", err)
	}
	encResticPassword, err := s.encryptSecret(resticPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt restic password: %w", err)
	}

	target, err := s.queries.CreateBackupTarget(ctx, &db.CreateBackupTargetParams{
		Name:           params.Name,
		Type:           string(params.Type),
		BucketName:     sql.NullString{String: params.BucketName, Valid: params.BucketName != ""},
		Region:         sql.NullString{String: params.Region, Valid: params.Region != ""},
		BucketPath:     sql.NullString{String: params.BucketPath, Valid: params.BucketPath != ""},
		AccessKeyID:    sql.NullString{String: encAccessKeyID, Valid: encAccessKeyID != ""},
		SecretKey:      sql.NullString{String: encSecretKey, Valid: encSecretKey != ""},
		S3PathStyle:    sql.NullBool{Bool: params.ForcePathStyle, Valid: true},
		Endpoint:       sql.NullString{String: params.Endpoint, Valid: params.Endpoint != ""},
		ResticPassword: sql.NullString{String: encResticPassword, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create backup target: %w", err)
	}

	decAccessKeyID, err := s.decryptSecret(target.AccessKeyID.String)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt access key ID: %w", err)
	}

	return &BackupTargetDTO{
		ID:             target.ID,
		Name:           target.Name,
		Type:           BackupTargetType(target.Type),
		BucketName:     target.BucketName.String,
		Region:         target.Region.String,
		Endpoint:       target.Endpoint.String,
		BucketPath:     target.BucketPath.String,
		AccessKeyID:    decAccessKeyID,
		ForcePathStyle: target.S3PathStyle.Bool,
		CreatedAt:      target.CreatedAt,
		UpdatedAt:      &target.UpdatedAt.Time,
	}, nil
}

// CreateBackupSchedule creates a new backup schedule
func (s *BackupService) CreateBackupSchedule(ctx context.Context, params CreateBackupScheduleParams) (*BackupScheduleDTO, error) {
	schedule, err := s.queries.CreateBackupSchedule(ctx, &db.CreateBackupScheduleParams{
		Name:           params.Name,
		Description:    sql.NullString{String: params.Description, Valid: params.Description != ""},
		CronExpression: params.CronExpression,
		TargetID:       params.TargetID,
		RetentionDays:  int64(params.RetentionDays),
		Enabled:        params.Enabled,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create backup schedule: %w", err)
	}

	if schedule.Enabled {
		s.scheduleBackup(schedule)
	}

	return &BackupScheduleDTO{
		ID:             schedule.ID,
		Name:           schedule.Name,
		Description:    schedule.Description.String,
		CronExpression: schedule.CronExpression,
		TargetID:       schedule.TargetID,
		RetentionDays:  int(schedule.RetentionDays),
		Enabled:        schedule.Enabled,
		CreatedAt:      schedule.CreatedAt,
		UpdatedAt:      &schedule.UpdatedAt.Time,
		LastRunAt:      &schedule.LastRunAt.Time,
		NextRunAt:      &schedule.NextRunAt.Time,
	}, nil
}

// CreateBackup creates a new backup
func (s *BackupService) CreateBackup(ctx context.Context, params CreateBackupParams) (*BackupDTO, error) {

	backup, err := s.queries.CreateBackup(ctx, &db.CreateBackupParams{
		ScheduleID: sql.NullInt64{Int64: *params.ScheduleID, Valid: params.ScheduleID != nil},
		TargetID:   params.TargetID,
		Status:     string(BackupStatusPending),
		StartedAt:  time.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	// Start backup process asynchronously
	go s.performBackup(backup)

	return &BackupDTO{
		ID:         backup.ID,
		ScheduleID: &backup.ScheduleID.Int64,
		TargetID:   backup.TargetID,
		Status:     BackupStatus(backup.Status),
		SizeBytes:  &backup.SizeBytes.Int64,
		StartedAt:  backup.StartedAt,
		CreatedAt:  backup.CreatedAt,
	}, nil
}

// TriggerBackup triggers a backup for a specific source
func (s *BackupService) TriggerBackup(ctx context.Context, sourceID int64, targetID int64) (*BackupDTO, error) {
	// Create and start the backup
	return s.CreateBackup(ctx, CreateBackupParams{
		TargetID: targetID,
	})
}

// getResticRepoURL constructs the repository URL based on the target configuration
func (s *BackupService) getResticRepoURL(target *db.BackupTarget) (string, error) {
	if !target.BucketName.Valid || !target.BucketPath.Valid {
		return "", fmt.Errorf("invalid bucket configuration")
	}

	// Require endpoint for all S3 targets
	if !target.Endpoint.Valid || target.Endpoint.String == "" {
		return "", fmt.Errorf("endpoint is required")
	}

	// For all S3 targets, use format: s3:https://endpoint/bucket-name/path
	repoURL := fmt.Sprintf("s3:%s/%s/%s",
		strings.TrimSuffix(target.Endpoint.String, "/"),
		target.BucketName.String,
		strings.TrimPrefix(target.BucketPath.String, "/"))

	return repoURL, nil
}

// ResticSnapshot represents the JSON output from restic snapshots command
type ResticSnapshot struct {
	Time           time.Time `json:"time"`
	Parent         string    `json:"parent"`
	Tree           string    `json:"tree"`
	Paths          []string  `json:"paths"`
	Hostname       string    `json:"hostname"`
	Username       string    `json:"username"`
	UID            int       `json:"uid"`
	GID            int       `json:"gid"`
	ID             string    `json:"id"`
	ShortID        string    `json:"short_id"`
	ProgramVersion string    `json:"program_version"`
	Summary        struct {
		BackupStart         time.Time `json:"backup_start"`
		BackupEnd           time.Time `json:"backup_end"`
		FilesNew            int       `json:"files_new"`
		FilesChanged        int       `json:"files_changed"`
		FilesUnmodified     int       `json:"files_unmodified"`
		DirsNew             int       `json:"dirs_new"`
		DirsChanged         int       `json:"dirs_changed"`
		DirsUnmodified      int       `json:"dirs_unmodified"`
		DataBlobs           int       `json:"data_blobs"`
		TreeBlobs           int       `json:"tree_blobs"`
		DataAdded           int64     `json:"data_added"`
		DataAddedPacked     int64     `json:"data_added_packed"`
		TotalFilesProcessed int       `json:"total_files_processed"`
		TotalBytesProcessed int64     `json:"total_bytes_processed"`
	} `json:"summary"`
}

// resticStatsOutput represents the JSON output of `restic stats`
type resticStatsOutput struct {
	TotalSize      int64 `json:"total_size"`
	TotalFileCount int   `json:"total_file_count"`
}

// getBackupSizeViaStats uses `restic stats latest --json` to get the snapshot size.
// This works across all restic versions, unlike the summary field in snapshots which
// was only added in restic 0.17.0.
func (s *BackupService) getBackupSizeViaStats(env []string) (int64, error) {
	cmd := exec.Command("restic", "stats", "latest", "--json")
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("restic stats failed: %w", err)
	}
	s.logger.Debugf("Restic stats output: %s", string(output))

	var stats resticStatsOutput
	if err := json.Unmarshal(output, &stats); err != nil {
		return 0, fmt.Errorf("failed to parse restic stats output: %w", err)
	}

	return stats.TotalSize, nil
}

// getBackupSize returns the size of the latest backup snapshot.
// It first tries the snapshot summary (restic >= 0.17.0), then falls back
// to `restic stats` for older versions where the summary field is absent.
func (s *BackupService) getBackupSize(env []string) (int64, error) {
	cmd := exec.Command("restic", "snapshots", "latest", "--json")
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("restic snapshots failed: %w", err)
	}
	s.logger.Debugf("Restic snapshots output: %s", string(output))

	var snapshots []ResticSnapshot
	if err := json.Unmarshal(output, &snapshots); err != nil {
		return 0, fmt.Errorf("failed to parse restic output: %w", err)
	}

	if len(snapshots) == 0 {
		return 0, fmt.Errorf("no snapshots found")
	}

	// Try the summary field first (available in restic >= 0.17.0)
	if snapshots[0].Summary.TotalBytesProcessed > 0 {
		return snapshots[0].Summary.TotalBytesProcessed, nil
	}

	// Fall back to `restic stats` for older restic versions (e.g. 0.16.x)
	// where the summary field is not populated in snapshot JSON
	s.logger.Debugf("Snapshot summary has 0 bytes, falling back to restic stats")
	return s.getBackupSizeViaStats(env)
}

// Update performS3Backup to include S3 connection issue notifications
func (s *BackupService) performS3Backup(ctx context.Context, backup *db.Backup, target *db.BackupTarget) error {
	if !target.Endpoint.Valid || target.Endpoint.String == "" {
		return fmt.Errorf("backup configuration error: endpoint is required")
	}

	// Parse endpoint URL to get the host
	customURL, err := url.Parse(target.Endpoint.String)
	if err != nil {
		return fmt.Errorf("backup configuration error: invalid endpoint URL: %w", err)
	}

	// Decrypt credentials for use
	accessKeyID, err := s.decryptSecret(target.AccessKeyID.String)
	if err != nil {
		return fmt.Errorf("failed to decrypt access key ID: %w", err)
	}
	secretKey, err := s.decryptSecret(target.SecretKey.String)
	if err != nil {
		return fmt.Errorf("failed to decrypt secret key: %w", err)
	}
	resticPassword, err := s.decryptSecret(target.ResticPassword.String)
	if err != nil {
		return fmt.Errorf("failed to decrypt restic password: %w", err)
	}

	// Set up restic environment variables for S3
	env := []string{
		fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", accessKeyID),
		fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", secretKey),
		fmt.Sprintf("RESTIC_PASSWORD=%s", resticPassword),
		fmt.Sprintf("AWS_ENDPOINT=%s", customURL.Host),
	}

	// Configure path style
	if target.S3PathStyle.Bool {
		env = append(env, "AWS_S3_FORCE_PATH_STYLE=true")
	}

	// Get repository URL
	repoURL, err := s.getResticRepoURL(target)
	if err != nil {
		return fmt.Errorf("failed to construct repository URL: %w", err)
	}
	env = append(env, fmt.Sprintf("RESTIC_REPOSITORY=%s", repoURL))

	// Initialize repository if it doesn't exist
	if err := s.initResticRepo(env); err != nil {
		// Check if it's a connection issue
		if strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "no such host") ||
			strings.Contains(err.Error(), "network") {
			// Send S3 connection issue notification
			s.notifyS3ConnectionIssue(ctx, target, err.Error())
		}
		return fmt.Errorf("failed to initialize restic repository: %w", err)
	}

	// Construct .chainlaunch path
	chainlaunchPath := s.configService.GetDataPath()

	// Check if directory exists
	if _, err := os.Stat(chainlaunchPath); os.IsNotExist(err) {
		return fmt.Errorf("backup source error: .chainlaunch directory does not exist at %s", chainlaunchPath)
	}

	// Create dbs directory if it doesn't exist
	dbsPath := filepath.Join(chainlaunchPath, "dbs")
	if err := os.MkdirAll(dbsPath, 0755); err != nil {
		return fmt.Errorf("backup preparation error: failed to create dbs directory: %w", err)
	}

	// Generate a custom filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	dbFileName := fmt.Sprintf("chainlaunch-%s.db", timestamp)
	dbBackupPath := filepath.Join(dbsPath, dbFileName)

	// Create a copy of the database file
	s.logger.Infof("Copying database from %s to %s", s.databasePath, dbBackupPath)

	// Open source file
	sourceFile, err := os.Open(s.databasePath)
	if err != nil {
		return fmt.Errorf("backup preparation error: failed to open source database file: %w", err)
	}
	defer sourceFile.Close()

	// Create destination file
	destFile, err := os.Create(dbBackupPath)
	if err != nil {
		return fmt.Errorf("backup preparation error: failed to create destination database file: %w", err)
	}
	defer destFile.Close()

	// Copy the contents
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("backup preparation error: failed to copy database file: %w", err)
	}

	// Ensure all data is written to disk
	if err := destFile.Sync(); err != nil {
		return fmt.Errorf("backup preparation error: failed to sync database file: %w", err)
	}

	// Close files explicitly before backup
	sourceFile.Close()
	destFile.Close()

	// Set up deferred cleanup to remove the database copy after backup
	defer func() {
		s.logger.Infof("Cleaning up database copy at %s", dbBackupPath)
		if err := os.Remove(dbBackupPath); err != nil {
			s.logger.Errorf("Failed to remove database copy: %v", err)
		}
	}()

	// Perform backup using restic with JSON output
	cmd := exec.CommandContext(ctx, "restic", "backup", chainlaunchPath, "--json")
	cmd.Env = append(os.Environ(), env...)

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start restic backup: %w", err)
	}

	// Create decoder for JSON output
	decoder := json.NewDecoder(stdout)

	// Read JSON messages
	for {
		var message struct {
			MessageType string `json:"message_type"`
			DataSize    int64  `json:"data_size,omitempty"`
			TotalFiles  int    `json:"total_files,omitempty"`
			TotalBytes  int64  `json:"total_bytes,omitempty"`
		}

		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode restic output: %w", err)
		}
		s.logger.Debugf("Restic message: %+v", message)

		// Log progress if needed
		if message.MessageType == "status" {
			s.logger.Infof("Backup progress: %d files, %d bytes", message.TotalFiles, message.TotalBytes)
		}
	}
	// Read stderr for any errors
	errBuf := new(bytes.Buffer)
	if _, err := io.Copy(errBuf, stderr); err != nil {
		return fmt.Errorf("backup process error: failed to read stderr: %w", err)
	}

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		errMsg := errBuf.String()
		if strings.Contains(errMsg, "connection refused") ||
			strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "no such host") {
			// Send S3 connection issue notification
			s.notifyS3ConnectionIssue(ctx, target, errMsg)
			return fmt.Errorf("network error: failed to connect to backup storage: %s", errMsg)
		} else if strings.Contains(errMsg, "permission denied") {
			s.notifyS3ConnectionIssue(ctx, target, errMsg)
			return fmt.Errorf("access error: insufficient permissions to access backup storage: %s", errMsg)
		} else if strings.Contains(errMsg, "no space left") {
			return fmt.Errorf("storage error: insufficient space in backup storage: %s", errMsg)
		}
		return fmt.Errorf("backup process error: %s: %w", errMsg, err)
	}
	backupSize, err := s.getBackupSize(env)
	if err != nil {
		return fmt.Errorf("failed to get backup size: %w", err)
	}
	// Update backup size
	_, err = s.queries.UpdateBackupSize(ctx, &db.UpdateBackupSizeParams{
		ID:        backup.ID,
		SizeBytes: sql.NullInt64{Int64: backupSize, Valid: true},
	})

	return err
}

// notifyS3ConnectionIssue sends a notification for S3 connection issues
func (s *BackupService) notifyS3ConnectionIssue(ctx context.Context, target *db.BackupTarget, errorMessage string) {
	// Skip notification if notification service is not available
	if s.notificationService == nil {
		s.logger.Info("Notification service not available, skipping S3 connection issue notification")
		return
	}

	// Prepare notification data
	data := notifications.S3ConnectionIssueData{
		TargetName:   target.Name,
		Endpoint:     target.Endpoint.String,
		BucketName:   target.BucketName.String,
		DetectedTime: time.Now(),
		ErrorMessage: errorMessage,
	}

	// Send notification
	err := s.notificationService.SendS3ConnectionIssueNotification(ctx, data)
	if err != nil {
		s.logger.Error("Failed to send S3 connection issue notification", "error", err)
		return
	}

	s.logger.Info("Sent S3 connection issue notification", "targetName", target.Name)
}

// markBackupFailed marks a backup as failed with an error message
func (s *BackupService) markBackupFailed(ctx context.Context, backupID int64, errorMessage string) {
	s.queries.UpdateBackupFailed(ctx, &db.UpdateBackupFailedParams{
		ID:           backupID,
		Status:       string(BackupStatusFailed),
		ErrorMessage: sql.NullString{String: errorMessage, Valid: true},
		CompletedAt:  sql.NullTime{Time: time.Now(), Valid: true},
	})
}

// scheduleBackup adds a backup schedule to the cron scheduler
func (s *BackupService) scheduleBackup(schedule *db.BackupSchedule) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If this schedule is already scheduled, remove it first
	if entryID, exists := s.cronEntryIDs[schedule.ID]; exists {
		s.cron.Remove(entryID)
		delete(s.cronEntryIDs, schedule.ID)
	}

	// Add the new schedule
	entryID, err := s.cron.AddFunc(schedule.CronExpression, func() {
		ctx := context.Background()
		s.createScheduledBackup(ctx, schedule)
	})

	if err != nil {
		s.logger.Error("Failed to schedule backup", "error", err, "scheduleID", schedule.ID)
		return
	}

	s.cronEntryIDs[schedule.ID] = entryID
	s.logger.Info("Scheduled backup", "scheduleID", schedule.ID, "cronExpression", schedule.CronExpression)
}

// createScheduledBackup creates a backup from a schedule
func (s *BackupService) createScheduledBackup(ctx context.Context, schedule *db.BackupSchedule) {
	// Create backup entry
	backup, err := s.queries.CreateBackup(ctx, &db.CreateBackupParams{
		ScheduleID: sql.NullInt64{Int64: schedule.ID, Valid: true},
		TargetID:   schedule.TargetID,
		Status:     string(BackupStatusPending),
		StartedAt:  time.Now(),
	})
	if err != nil {
		return
	}

	// Update schedule's last run time
	s.queries.UpdateBackupScheduleLastRun(ctx, &db.UpdateBackupScheduleLastRunParams{
		ID:        schedule.ID,
		LastRunAt: sql.NullTime{Time: time.Now(), Valid: true},
	})

	// Start backup process
	go s.performBackup(backup)
}

// performBackup executes the actual backup process
func (s *BackupService) performBackup(backup *db.Backup) {
	ctx := context.Background()

	// Update status to in progress
	_, err := s.queries.UpdateBackupStatus(ctx, &db.UpdateBackupStatusParams{
		ID:     backup.ID,
		Status: string(BackupStatusInProgress),
	})
	if err != nil {
		errorMsg := fmt.Sprintf("Database error: Failed to update backup status: %v", err)
		s.markBackupFailed(ctx, backup.ID, errorMsg)
		s.notifyBackupFailure(ctx, backup, errorMsg)
		return
	}

	// Get target configuration
	target, err := s.queries.GetBackupTarget(ctx, backup.TargetID)
	if err != nil {
		var errorMsg string
		if err == sql.ErrNoRows {
			errorMsg = fmt.Sprintf("Configuration error: Backup target not found (ID: %d)", backup.TargetID)
		} else {
			errorMsg = fmt.Sprintf("Database error: Failed to get backup target: %v", err)
		}
		s.markBackupFailed(ctx, backup.ID, errorMsg)
		s.notifyBackupFailure(ctx, backup, errorMsg)
		return
	}

	// Perform backup based on target type
	var backupErr error
	switch BackupTargetType(target.Type) {
	case BackupTargetTypeS3:
		backupErr = s.performS3Backup(ctx, backup, target)
	default:
		backupErr = fmt.Errorf("Configuration error: Unsupported backup target type: %s", target.Type)
	}

	if backupErr != nil {
		s.markBackupFailed(ctx, backup.ID, backupErr.Error())
		s.notifyBackupFailure(ctx, backup, backupErr.Error())
		return
	}

	// Mark backup as completed
	updatedBackup, err := s.queries.UpdateBackupCompleted(ctx, &db.UpdateBackupCompletedParams{
		ID:          backup.ID,
		Status:      string(BackupStatusCompleted),
		CompletedAt: sql.NullTime{Time: time.Now(), Valid: true},
	})
	if err != nil {
		errorMsg := fmt.Sprintf("Database error: Failed to mark backup as completed: %v", err)
		s.markBackupFailed(ctx, backup.ID, errorMsg)
		s.notifyBackupFailure(ctx, backup, errorMsg)
		return
	}

	// Send success notification
	s.notifyBackupSuccess(ctx, updatedBackup)
}

// ListBackupTargets returns all backup targets
func (s *BackupService) ListBackupTargets(ctx context.Context) ([]*BackupTargetDTO, error) {
	targets, err := s.queries.ListBackupTargets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list backup targets: %w", err)
	}

	dtos := make([]*BackupTargetDTO, len(targets))
	for i, target := range targets {
		decAccessKeyID, err := s.decryptSecret(target.AccessKeyID.String)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt access key ID: %w", err)
		}
		dtos[i] = &BackupTargetDTO{
			ID:             target.ID,
			Name:           target.Name,
			Type:           BackupTargetType(target.Type),
			BucketName:     target.BucketName.String,
			Region:         target.Region.String,
			Endpoint:       target.Endpoint.String,
			BucketPath:     target.BucketPath.String,
			AccessKeyID:    decAccessKeyID,
			ForcePathStyle: target.S3PathStyle.Bool,
			CreatedAt:      target.CreatedAt,
			UpdatedAt:      &target.UpdatedAt.Time,
		}
	}

	return dtos, nil
}

// GetBackupTarget returns a backup target by ID
func (s *BackupService) GetBackupTarget(ctx context.Context, id int64) (*BackupTargetDTO, error) {
	target, err := s.queries.GetBackupTarget(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup target: %w", err)
	}

	decAccessKeyID, err := s.decryptSecret(target.AccessKeyID.String)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt access key ID: %w", err)
	}

	return &BackupTargetDTO{
		ID:             target.ID,
		Name:           target.Name,
		Type:           BackupTargetType(target.Type),
		BucketName:     target.BucketName.String,
		Region:         target.Region.String,
		Endpoint:       target.Endpoint.String,
		BucketPath:     target.BucketPath.String,
		AccessKeyID:    decAccessKeyID,
		ForcePathStyle: target.S3PathStyle.Bool,
		CreatedAt:      target.CreatedAt,
		UpdatedAt:      &target.UpdatedAt.Time,
	}, nil
}

// DeleteBackupTarget deletes a backup target
func (s *BackupService) DeleteBackupTarget(ctx context.Context, id int64) error {
	return s.queries.DeleteBackupTarget(ctx, id)
}

// ListBackupSchedules returns all backup schedules
func (s *BackupService) ListBackupSchedules(ctx context.Context) ([]*BackupScheduleDTO, error) {
	schedules, err := s.queries.ListBackupSchedules(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list backup schedules: %w", err)
	}

	dtos := make([]*BackupScheduleDTO, len(schedules))
	for i, schedule := range schedules {
		dtos[i] = &BackupScheduleDTO{
			ID:             schedule.ID,
			Name:           schedule.Name,
			Description:    schedule.Description.String,
			CronExpression: schedule.CronExpression,
			TargetID:       schedule.TargetID,
			RetentionDays:  int(schedule.RetentionDays),
			Enabled:        schedule.Enabled,
			CreatedAt:      schedule.CreatedAt,
			UpdatedAt:      &schedule.UpdatedAt.Time,
			LastRunAt:      &schedule.LastRunAt.Time,
			NextRunAt:      &schedule.NextRunAt.Time,
		}
	}

	return dtos, nil
}

// GetBackupSchedule returns a backup schedule by ID
func (s *BackupService) GetBackupSchedule(ctx context.Context, id int64) (*BackupScheduleDTO, error) {
	schedule, err := s.queries.GetBackupSchedule(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup schedule: %w", err)
	}

	return &BackupScheduleDTO{
		ID:             schedule.ID,
		Name:           schedule.Name,
		Description:    schedule.Description.String,
		CronExpression: schedule.CronExpression,
		TargetID:       schedule.TargetID,
		RetentionDays:  int(schedule.RetentionDays),
		Enabled:        schedule.Enabled,
		CreatedAt:      schedule.CreatedAt,
		UpdatedAt:      &schedule.UpdatedAt.Time,
		LastRunAt:      &schedule.LastRunAt.Time,
		NextRunAt:      &schedule.NextRunAt.Time,
	}, nil
}

// EnableBackupSchedule enables a backup schedule
func (s *BackupService) EnableBackupSchedule(ctx context.Context, id int64) (*BackupScheduleDTO, error) {
	schedule, err := s.queries.EnableBackupSchedule(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to enable backup schedule: %w", err)
	}

	s.scheduleBackup(schedule)

	return &BackupScheduleDTO{
		ID:             schedule.ID,
		Name:           schedule.Name,
		Description:    schedule.Description.String,
		CronExpression: schedule.CronExpression,
		TargetID:       schedule.TargetID,
		RetentionDays:  int(schedule.RetentionDays),
		Enabled:        schedule.Enabled,
		CreatedAt:      schedule.CreatedAt,
		UpdatedAt:      &schedule.UpdatedAt.Time,
		LastRunAt:      &schedule.LastRunAt.Time,
		NextRunAt:      &schedule.NextRunAt.Time,
	}, nil
}

// DisableBackupSchedule disables a backup schedule
func (s *BackupService) DisableBackupSchedule(ctx context.Context, id int64) (*BackupScheduleDTO, error) {
	schedule, err := s.queries.DisableBackupSchedule(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to disable backup schedule: %w", err)
	}

	// Remove the schedule from the cron scheduler
	s.mu.Lock()
	if entryID, exists := s.cronEntryIDs[schedule.ID]; exists {
		s.cron.Remove(entryID)
		delete(s.cronEntryIDs, schedule.ID)
		s.logger.Info("Removed backup schedule from cron scheduler", "scheduleID", schedule.ID)
	}
	s.mu.Unlock()

	return &BackupScheduleDTO{
		ID:             schedule.ID,
		Name:           schedule.Name,
		Description:    schedule.Description.String,
		CronExpression: schedule.CronExpression,
		TargetID:       schedule.TargetID,
		RetentionDays:  int(schedule.RetentionDays),
		Enabled:        schedule.Enabled,
		CreatedAt:      schedule.CreatedAt,
		UpdatedAt:      &schedule.UpdatedAt.Time,
		LastRunAt:      &schedule.LastRunAt.Time,
		NextRunAt:      &schedule.NextRunAt.Time,
	}, nil
}

// DeleteBackupSchedule deletes a backup schedule
func (s *BackupService) DeleteBackupSchedule(ctx context.Context, id int64) error {
	return s.queries.DeleteBackupSchedule(ctx, id)
}

// ListBackups returns all backups
func (s *BackupService) ListBackups(ctx context.Context) ([]*BackupDTO, error) {
	backups, err := s.queries.ListBackups(ctx, &db.ListBackupsParams{
		Limit:  100,
		Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	dtos := make([]*BackupDTO, len(backups))
	for i, backup := range backups {
		dtos[i] = &BackupDTO{
			ID:           backup.ID,
			ScheduleID:   &backup.ScheduleID.Int64,
			TargetID:     backup.TargetID,
			Status:       BackupStatus(backup.Status),
			SizeBytes:    &backup.SizeBytes.Int64,
			StartedAt:    backup.StartedAt,
			CompletedAt:  &backup.CompletedAt.Time,
			ErrorMessage: &backup.ErrorMessage.String,
			CreatedAt:    backup.CreatedAt,
		}
	}

	return dtos, nil
}

// GetBackup retrieves a backup by ID
func (s *BackupService) GetBackup(ctx context.Context, id int64) (*BackupDTO, error) {
	backup, err := s.queries.GetBackup(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("backup not found")
		}
		return nil, fmt.Errorf("failed to get backup: %w", err)
	}

	return &BackupDTO{
		ID:           backup.ID,
		ScheduleID:   &backup.ScheduleID.Int64,
		TargetID:     backup.TargetID,
		Status:       BackupStatus(backup.Status),
		SizeBytes:    &backup.SizeBytes.Int64,
		StartedAt:    backup.StartedAt,
		CompletedAt:  &backup.CompletedAt.Time,
		ErrorMessage: &backup.ErrorMessage.String,
		CreatedAt:    backup.CreatedAt,
	}, nil
}

// DeleteBackup deletes a backup
func (s *BackupService) DeleteBackup(ctx context.Context, id int64) error {
	// Get backup details first
	backup, err := s.queries.GetBackup(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("backup not found")
		}
		return fmt.Errorf("failed to get backup: %w", err)
	}

	// Get target details
	target, err := s.queries.GetBackupTarget(ctx, backup.TargetID)
	if err != nil {
		return fmt.Errorf("failed to get backup target: %w", err)
	}

	// Delete the actual backup file based on target type
	if err := s.deleteBackupFile(ctx, backup, target); err != nil {
		return fmt.Errorf("failed to delete backup file: %w", err)
	}

	// Delete from database
	if err := s.queries.DeleteBackup(ctx, id); err != nil {
		return fmt.Errorf("failed to delete backup record: %w", err)
	}

	return nil
}

// deleteBackupFile deletes the actual backup file from storage
func (s *BackupService) deleteBackupFile(ctx context.Context, backup *db.Backup, target *db.BackupTarget) error {
	switch BackupTargetType(target.Type) {
	case BackupTargetTypeS3:
		return s.deleteS3BackupFile(ctx, backup, target)
	default:
		return fmt.Errorf("unsupported backup target type: %s", target.Type)
	}
}

// deleteS3BackupFile deletes a backup file from S3 using restic
func (s *BackupService) deleteS3BackupFile(ctx context.Context, backup *db.Backup, target *db.BackupTarget) error {
	// Decrypt credentials for use
	accessKeyID, err := s.decryptSecret(target.AccessKeyID.String)
	if err != nil {
		return fmt.Errorf("failed to decrypt access key ID: %w", err)
	}
	secretKey, err := s.decryptSecret(target.SecretKey.String)
	if err != nil {
		return fmt.Errorf("failed to decrypt secret key: %w", err)
	}
	resticPassword, err := s.decryptSecret(target.ResticPassword.String)
	if err != nil {
		return fmt.Errorf("failed to decrypt restic password: %w", err)
	}

	// Set up restic environment variables
	env := []string{
		fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", accessKeyID),
		fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", secretKey),
		fmt.Sprintf("RESTIC_PASSWORD=%s", resticPassword),
		fmt.Sprintf("AWS_ENDPOINT=%s", target.Endpoint.String),
	}

	if target.S3PathStyle.Bool {
		env = append(env, "AWS_S3_FORCE_PATH_STYLE=true")
	}

	// Get repository URL
	repoURL, err := s.getResticRepoURL(target)
	if err != nil {
		return fmt.Errorf("failed to construct repository URL: %w", err)
	}
	env = append(env, fmt.Sprintf("RESTIC_REPOSITORY=%s", repoURL))

	// Get latest snapshot ID
	snapshotID, err := s.findLatestSnapshot(env)
	if err != nil {
		return fmt.Errorf("failed to find snapshot: %w", err)
	}

	// Delete the snapshot
	cmd := exec.Command("restic", "forget", "--remove-snapshots", snapshotID)
	cmd.Env = append(os.Environ(), env...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %s: %w", string(output), err)
	}

	s.logger.Infof("Deleted snapshot %s", snapshotID)
	return nil
}

// Helper functions for restic operations

func (s *BackupService) initResticRepo(env []string) error {
	cmd := exec.Command("restic", "init")
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore error if repository already exists
		if strings.Contains(string(output), "already initialized") {
			return nil
		}
		return fmt.Errorf("restic init failed: %s: %w", string(output), err)
	}
	return nil
}

// Update findSnapshotWithFile to use latest snapshot
func (s *BackupService) findLatestSnapshot(env []string) (string, error) {
	cmd := exec.Command("restic", "snapshots", "latest", "--json")
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("restic snapshots failed: %w", err)
	}

	var snapshots []ResticSnapshot
	if err := json.Unmarshal(output, &snapshots); err != nil {
		return "", fmt.Errorf("failed to parse snapshots: %w", err)
	}

	if len(snapshots) == 0 {
		return "", fmt.Errorf("no snapshots found")
	}

	// Return the ID of the latest snapshot
	return snapshots[0].ID, nil
}

// encryptSecret encrypts a secret string if an encryptor is configured.
// Returns the original string if no encryptor is set (backwards compatible).
func (s *BackupService) encryptSecret(secret string) (string, error) {
	if s.encryptor == nil || secret == "" {
		return secret, nil
	}
	return s.encryptor.Encrypt(secret)
}

// decryptSecret decrypts a secret string if an encryptor is configured.
// Handles both encrypted and plaintext values for backwards compatibility.
func (s *BackupService) decryptSecret(secret string) (string, error) {
	if s.encryptor == nil || secret == "" {
		return secret, nil
	}
	if !crypto.IsEncrypted(secret) {
		return secret, nil
	}
	return s.encryptor.Decrypt(secret)
}

// Add helper function to generate secure password
func generateSecurePassword() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// UpdateBackupTarget updates an existing backup target
func (s *BackupService) UpdateBackupTarget(ctx context.Context, params UpdateBackupTargetParams) (*BackupTargetDTO, error) {
	// Validate config based on type
	if params.Type == BackupTargetTypeS3 {
		// Validate required S3 fields
		if params.BucketName == "" || params.Endpoint == "" ||
			params.BucketPath == "" || params.AccessKeyID == "" ||
			params.SecretKey == "" {
			return nil, fmt.Errorf("configuration error: missing required S3 configuration fields")
		}

		// Validate endpoint URL format
		_, err := url.Parse(params.Endpoint)
		if err != nil {
			return nil, fmt.Errorf("configuration error: invalid endpoint URL: %w", err)
		}
	}

	// Encrypt sensitive credentials before storing
	encAccessKeyID, err := s.encryptSecret(params.AccessKeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt access key ID: %w", err)
	}
	encSecretKey, err := s.encryptSecret(params.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret key: %w", err)
	}

	// Update the target
	target, err := s.queries.UpdateBackupTarget(ctx, &db.UpdateBackupTargetParams{
		ID:          params.ID,
		Name:        params.Name,
		Type:        string(params.Type),
		BucketName:  sql.NullString{String: params.BucketName, Valid: params.BucketName != ""},
		Region:      sql.NullString{String: params.Region, Valid: params.Region != ""},
		BucketPath:  sql.NullString{String: params.BucketPath, Valid: params.BucketPath != ""},
		AccessKeyID: sql.NullString{String: encAccessKeyID, Valid: encAccessKeyID != ""},
		SecretKey:   sql.NullString{String: encSecretKey, Valid: encSecretKey != ""},
		S3PathStyle: sql.NullBool{Bool: params.ForcePathStyle, Valid: true},
		Endpoint:    sql.NullString{String: params.Endpoint, Valid: params.Endpoint != ""},
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("backup target not found with ID: %d", params.ID)
		}
		return nil, fmt.Errorf("failed to update backup target: %w", err)
	}

	decAccessKeyID, err := s.decryptSecret(target.AccessKeyID.String)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt access key ID: %w", err)
	}

	return &BackupTargetDTO{
		ID:             target.ID,
		Name:           target.Name,
		Type:           BackupTargetType(target.Type),
		BucketName:     target.BucketName.String,
		Region:         target.Region.String,
		Endpoint:       target.Endpoint.String,
		BucketPath:     target.BucketPath.String,
		AccessKeyID:    decAccessKeyID,
		ForcePathStyle: target.S3PathStyle.Bool,
		CreatedAt:      target.CreatedAt,
		UpdatedAt:      &target.UpdatedAt.Time,
	}, nil
}

// UpdateBackupSchedule updates an existing backup schedule
func (s *BackupService) UpdateBackupSchedule(ctx context.Context, params UpdateBackupScheduleParams) (*BackupScheduleDTO, error) {
	// Get existing schedule to check if enabled status changed
	existingSchedule, err := s.queries.GetBackupSchedule(ctx, params.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("backup schedule not found with ID: %d", params.ID)
		}
		return nil, fmt.Errorf("failed to get existing backup schedule: %w", err)
	}

	// Update the schedule
	schedule, err := s.queries.UpdateBackupSchedule(ctx, &db.UpdateBackupScheduleParams{
		ID:             params.ID,
		Name:           params.Name,
		Description:    sql.NullString{String: params.Description, Valid: params.Description != ""},
		CronExpression: params.CronExpression,
		TargetID:       params.TargetID,
		RetentionDays:  int64(params.RetentionDays),
		Enabled:        params.Enabled,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update backup schedule: %w", err)
	}

	// Handle enabled status change
	if existingSchedule.Enabled != schedule.Enabled {
		if schedule.Enabled {
			// Schedule was enabled, add it to the cron scheduler
			s.scheduleBackup(schedule)
			s.logger.Info("Enabled backup schedule", "scheduleID", schedule.ID)
		} else {
			// Schedule was disabled, remove it from the cron scheduler
			s.mu.Lock()
			if entryID, exists := s.cronEntryIDs[schedule.ID]; exists {
				s.cron.Remove(entryID)
				delete(s.cronEntryIDs, schedule.ID)
				s.logger.Info("Disabled backup schedule", "scheduleID", schedule.ID)
			}
			s.mu.Unlock()
		}
	} else if schedule.Enabled {
		// Schedule was already enabled but cron expression might have changed
		// Remove the old schedule and add the new one
		s.mu.Lock()
		if entryID, exists := s.cronEntryIDs[schedule.ID]; exists {
			s.cron.Remove(entryID)
			delete(s.cronEntryIDs, schedule.ID)
		}
		s.mu.Unlock()

		// Add the updated schedule
		s.scheduleBackup(schedule)
		s.logger.Info("Updated backup schedule", "scheduleID", schedule.ID)
	}

	return &BackupScheduleDTO{
		ID:             schedule.ID,
		Name:           schedule.Name,
		Description:    schedule.Description.String,
		CronExpression: schedule.CronExpression,
		TargetID:       schedule.TargetID,
		RetentionDays:  int(schedule.RetentionDays),
		Enabled:        schedule.Enabled,
		CreatedAt:      schedule.CreatedAt,
		UpdatedAt:      &schedule.UpdatedAt.Time,
		LastRunAt:      &schedule.LastRunAt.Time,
		NextRunAt:      &schedule.NextRunAt.Time,
	}, nil
}

// notifyBackupSuccess sends a notification about a successful backup
func (s *BackupService) notifyBackupSuccess(ctx context.Context, backup *db.Backup) {
	// Skip notification if notification service is not available
	if s.notificationService == nil {
		s.logger.Info("Notification service not available, skipping backup success notification")
		return
	}

	// Get target details for the notification
	target, err := s.queries.GetBackupTarget(ctx, backup.TargetID)
	if err != nil {
		s.logger.Error("Failed to get backup target", "error", err, "targetID", backup.TargetID)
		return
	}

	// Get schedule details if available
	var scheduleName string
	var retentionDays int64
	var cronExpression string
	if backup.ScheduleID.Valid {
		schedule, err := s.queries.GetBackupSchedule(ctx, backup.ScheduleID.Int64)
		if err == nil {
			scheduleName = schedule.Name
			retentionDays = schedule.RetentionDays
			cronExpression = schedule.CronExpression
		}
	}

	// Format the success time
	successTime := time.Now()
	if backup.CompletedAt.Valid {
		successTime = backup.CompletedAt.Time
	}

	// Calculate duration if started time is available
	var duration string
	if !backup.StartedAt.IsZero() {
		duration = formatDuration(successTime.Sub(backup.StartedAt))
	}

	// Prepare notification data
	data := notifications.BackupSuccessData{
		BackupID:       backup.ID,
		ScheduleName:   scheduleName,
		TargetName:     target.Name,
		TargetType:     target.Type,
		BucketName:     target.BucketName.String,
		Endpoint:       target.Endpoint.String,
		SizeBytes:      backup.SizeBytes.Int64,
		SuccessTime:    successTime,
		StartedAt:      backup.StartedAt,
		Duration:       duration,
		RetentionDays:  retentionDays,
		CronExpression: cronExpression,
	}

	// Send notification
	err = s.notificationService.SendBackupSuccessNotification(ctx, data)
	if err != nil {
		s.logger.Error("Failed to send backup success notification", "error", err)
		return
	}

	s.logger.Info("Sent backup success notification", "backupID", backup.ID)

	// Mark backup as notified
	s.queries.MarkBackupNotified(ctx, backup.ID)
}

// notifyBackupFailure sends a notification about a failed backup
func (s *BackupService) notifyBackupFailure(ctx context.Context, backup *db.Backup, errorMessage string) {
	// Skip notification if notification service is not available
	if s.notificationService == nil {
		s.logger.Info("Notification service not available, skipping backup failure notification")
		return
	}

	// Get target details for the notification
	target, err := s.queries.GetBackupTarget(ctx, backup.TargetID)
	if err != nil {
		s.logger.Error("Failed to get backup target", "error", err, "targetID", backup.TargetID)
		return
	}

	// Get schedule details if available
	var scheduleName string
	var retentionDays int64
	var cronExpression string
	if backup.ScheduleID.Valid {
		schedule, err := s.queries.GetBackupSchedule(ctx, backup.ScheduleID.Int64)
		if err == nil {
			scheduleName = schedule.Name
			retentionDays = schedule.RetentionDays
			cronExpression = schedule.CronExpression
		}
	}

	// Format the failure time
	failureTime := time.Now()
	if backup.CompletedAt.Valid {
		failureTime = backup.CompletedAt.Time
	}

	// Calculate duration if started time is available
	var duration string
	if !backup.StartedAt.IsZero() {
		duration = formatDuration(failureTime.Sub(backup.StartedAt))
	}

	// Prepare notification data
	data := notifications.BackupFailureData{
		BackupID:       backup.ID,
		ScheduleName:   scheduleName,
		TargetName:     target.Name,
		TargetType:     target.Type,
		BucketName:     target.BucketName.String,
		Endpoint:       target.Endpoint.String,
		ErrorMessage:   errorMessage,
		FailureTime:    failureTime,
		StartedAt:      backup.StartedAt,
		Duration:       duration,
		RetentionDays:  retentionDays,
		CronExpression: cronExpression,
	}

	// Send notification
	err = s.notificationService.SendBackupFailureNotification(ctx, data)
	if err != nil {
		s.logger.Error("Failed to send backup failure notification", "error", err)
		return
	}

	s.logger.Info("Sent backup failure notification", "backupID", backup.ID)

	// Mark backup as notified
	s.queries.MarkBackupNotified(ctx, backup.ID)
}

// formatDuration formats a duration in a human-readable format
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	hours := d / time.Hour
	d -= hours * time.Hour

	minutes := d / time.Minute
	d -= minutes * time.Minute

	seconds := d / time.Second

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// Stop stops the backup service
func (s *BackupService) Stop() {
	s.logger.Info("Stopping backup service")

	// Stop the cron scheduler
	s.cron.Stop()

	// Signal any background processes to stop
	close(s.stopCh)
}
