package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"crypto/tls"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/notifications"
	"gopkg.in/mail.v2"
)

type NotificationService struct {
	queries *db.Queries
	logger  *logger.Logger
}

func NewNotificationService(queries *db.Queries, logger *logger.Logger) *NotificationService {
	return &NotificationService{
		queries: queries,
		logger:  logger,
	}
}

// EmailContent represents the content of an email with both HTML and plain text versions
type EmailContent struct {
	Subject   string
	PlainText string
	HTML      string
}

func (s *NotificationService) CreateProvider(ctx context.Context, params notifications.CreateProviderParams) (*notifications.NotificationProvider, error) {
	configJSON, err := json.Marshal(params.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	if params.IsDefault {
		// Unset existing default provider of the same type
		err = s.queries.UnsetDefaultNotificationProvider(ctx, string(params.Type))
		if err != nil {
			return nil, fmt.Errorf("failed to unset default provider: %w", err)
		}
	}

	provider, err := s.queries.CreateNotificationProvider(ctx, &db.CreateNotificationProviderParams{
		Type:                    string(params.Type),
		Name:                    params.Name,
		Config:                  string(configJSON),
		IsDefault:               params.IsDefault,
		NotifyNodeDowntime:      params.NotifyNodeDowntime,
		NotifyBackupSuccess:     params.NotifyBackupSuccess,
		NotifyBackupFailure:     params.NotifyBackupFailure,
		NotifyS3ConnectionIssue: params.NotifyS3ConnIssue,
		NotifyDiskSpaceWarning:  params.NotifyDiskSpaceWarning,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	var config interface{}
	if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return s.providerToDTO(provider, config), nil
}

func (s *NotificationService) UpdateProvider(ctx context.Context, params notifications.UpdateProviderParams) (*notifications.NotificationProvider, error) {
	configJSON, err := json.Marshal(params.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	if params.IsDefault {
		// Unset existing default provider of the same type
		err = s.queries.UnsetDefaultNotificationProvider(ctx, string(params.Type))
		if err != nil {
			return nil, fmt.Errorf("failed to unset default provider: %w", err)
		}
	}

	provider, err := s.queries.UpdateNotificationProvider(ctx, &db.UpdateNotificationProviderParams{
		ID:                      params.ID,
		Type:                    string(params.Type),
		Name:                    params.Name,
		Config:                  string(configJSON),
		IsDefault:               params.IsDefault,
		NotifyNodeDowntime:      params.NotifyNodeDowntime,
		NotifyBackupSuccess:     params.NotifyBackupSuccess,
		NotifyBackupFailure:     params.NotifyBackupFailure,
		NotifyS3ConnectionIssue: params.NotifyS3ConnIssue,
		NotifyDiskSpaceWarning:  params.NotifyDiskSpaceWarning,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update provider: %w", err)
	}

	var config interface{}
	if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return s.providerToDTO(provider, config), nil
}

func (s *NotificationService) GetProvider(ctx context.Context, id int64) (*notifications.NotificationProvider, error) {
	provider, err := s.queries.GetNotificationProvider(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	var config interface{}
	if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return s.providerToDTO(provider, config), nil
}

func (s *NotificationService) ListProviders(ctx context.Context) ([]*notifications.NotificationProvider, error) {
	providers, err := s.queries.ListNotificationProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}

	result := make([]*notifications.NotificationProvider, len(providers))
	for i, provider := range providers {
		var config interface{}
		if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		result[i] = s.providerToDTO(provider, config)
	}

	return result, nil
}

func (s *NotificationService) DeleteProvider(ctx context.Context, id int64) error {
	return s.queries.DeleteNotificationProvider(ctx, id)
}

func (s *NotificationService) GetDefaultProvider(ctx context.Context, providerType notifications.ProviderType) (*notifications.NotificationProvider, error) {
	provider, err := s.queries.GetDefaultNotificationProvider(ctx, string(providerType))
	if err != nil {
		return nil, fmt.Errorf("failed to get default provider: %w", err)
	}

	var config interface{}
	if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return s.providerToDTO(provider, config), nil
}

func (s *NotificationService) TestProvider(ctx context.Context, id int64, params notifications.TestProviderParams) (*notifications.TestResult, error) {
	provider, err := s.queries.GetNotificationProvider(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	var config notifications.SMTPConfig
	if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	s.logger.Info("Sending test email", "from", config.From, "to", params.TestEmail)

	// Test email sending
	err = s.sendTestEmail(config, params.TestEmail)
	testStatus := "success"
	testMessage := "Email sent successfully"
	if err != nil {
		testStatus = "failure"
		testMessage = fmt.Sprintf("Failed to send email: %v", err)
	}

	// Update provider with test results
	_, err = s.queries.UpdateProviderTestResults(ctx, &db.UpdateProviderTestResultsParams{
		ID:              id,
		LastTestAt:      sql.NullTime{Time: time.Now(), Valid: true},
		LastTestStatus:  sql.NullString{String: testStatus, Valid: true},
		LastTestMessage: sql.NullString{String: testMessage, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update test results: %w", err)
	}

	return &notifications.TestResult{
		Status:   testStatus,
		Message:  testMessage,
		TestedAt: time.Now(),
	}, nil
}

func (s *NotificationService) sendTestEmail(config notifications.SMTPConfig, testEmail string) error {
	// Create test email content
	content := EmailContent{
		Subject: "Test Email from ChainDeploy",
		PlainText: `This is a test email to verify your SMTP configuration is working correctly.
		
If you're seeing this, your email configuration is working!`,
		HTML: `
		<html>
			<body>
				<h2>ChainDeploy Email Test</h2>
				<p>This is a test email to verify your SMTP configuration is working correctly.</p>
				<p style="color: green;">If you're seeing this, your email configuration is working!</p>
				<hr>
				<small>Sent from ChainDeploy</small>
			</body>
		</html>`,
	}

	return s.sendEmail(config, config.From, []string{testEmail}, content)
}
func (s *NotificationService) sendEmail(config notifications.SMTPConfig, from string, to []string, content EmailContent) error {
	m := mail.NewMessage()

	// Set email headers
	m.SetHeader("From", from)
	m.SetHeader("To", to...)
	m.SetHeader("Subject", content.Subject)

	// Set both HTML and plain text body
	m.SetBody("text/plain", content.PlainText)
	m.AddAlternative("text/html", content.HTML)

	// Configure dialer
	d := mail.NewDialer(config.Host, config.Port, config.Username, config.Password)

	// Configure TLS
	d.TLSConfig = &tls.Config{
		ServerName:         config.Host,
		InsecureSkipVerify: false,
	}

	// Set timeout to 15 seconds
	d.Timeout = 15 * time.Second

	// Enable SSL/TLS if configured
	if config.TLS {
		d.SSL = true
	} else {
		d.SSL = false
		d.StartTLSPolicy = mail.OpportunisticStartTLS
	}

	// Create channel for timeout
	done := make(chan error, 1)

	// Send email in goroutine
	go func() {
		done <- d.DialAndSend(m)
	}()

	// Wait for either timeout or completion
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to send email: %w", err)
		}
		return nil
	case <-time.After(15 * time.Second):
		return fmt.Errorf("timeout sending email after 15 seconds")
	}
}

func (s *NotificationService) providerToDTO(provider *db.NotificationProvider, config interface{}) *notifications.NotificationProvider {
	return &notifications.NotificationProvider{
		ID:                     provider.ID,
		Type:                   notifications.ProviderType(provider.Type),
		Name:                   provider.Name,
		Config:                 config,
		IsDefault:              provider.IsDefault,
		NotifyNodeDowntime:     provider.NotifyNodeDowntime,
		NotifyBackupSuccess:    provider.NotifyBackupSuccess,
		NotifyBackupFailure:    provider.NotifyBackupFailure,
		NotifyS3ConnIssue:      provider.NotifyS3ConnectionIssue,
		NotifyDiskSpaceWarning: provider.NotifyDiskSpaceWarning,
		LastTestAt: func() *time.Time {
			if provider.LastTestAt.Valid {
				return &provider.LastTestAt.Time
			}
			return nil
		}(),
		LastTestStatus:  provider.LastTestStatus.String,
		LastTestMessage: provider.LastTestMessage.String,
		CreatedAt:       provider.CreatedAt,
		UpdatedAt:       provider.UpdatedAt,
	}
}

// SendBackupSuccessNotification sends a notification for a successful backup
func (s *NotificationService) SendBackupSuccessNotification(ctx context.Context, data notifications.BackupSuccessData) error {
	// Get default notification provider for backup successes
	provider, err := s.queries.GetDefaultNotificationProviderForType(ctx, "BACKUP_SUCCESS")
	if err != nil {
		s.logger.Warn("Failed to get default notification provider for backup successes", "error", err)
		return nil
	}

	if !provider.NotifyBackupSuccess {
		// Provider is configured to not notify for backup successes
		return nil
	}

	var config notifications.SMTPConfig
	if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Create notification content
	content := s.createBackupSuccessContent(data)

	// Send the email
	if err := s.sendEmail(config, config.From, []string{config.From}, content); err != nil {
		return fmt.Errorf("failed to send backup success notification: %w", err)
	}

	s.logger.Info("Sent backup success notification", "backupID", data.BackupID)
	return nil
}

// SendBackupFailureNotification sends a notification for a failed backup
func (s *NotificationService) SendBackupFailureNotification(ctx context.Context, data notifications.BackupFailureData) error {
	// Get default notification provider for backup failures
	provider, err := s.queries.GetDefaultNotificationProviderForType(ctx, "BACKUP_FAILURE")
	if err != nil {
		s.logger.Warn("Failed to get default notification provider for backup failures", "error", err)
		return nil
	}

	if !provider.NotifyBackupFailure {
		// Provider is configured to not notify for backup failures
		return nil
	}

	var config notifications.SMTPConfig
	if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Create notification content
	content := s.createBackupFailureContent(data)

	// Send the email
	if err := s.sendEmail(config, config.From, []string{config.From}, content); err != nil {
		return fmt.Errorf("failed to send backup failure notification: %w", err)
	}

	s.logger.Info("Sent backup failure notification", "backupID", data.BackupID)
	return nil
}

// SendS3ConnectionIssueNotification sends a notification for S3 connection issues
func (s *NotificationService) SendS3ConnectionIssueNotification(ctx context.Context, data notifications.S3ConnectionIssueData) error {
	// Get default notification provider for S3 connection issues
	provider, err := s.queries.GetDefaultNotificationProviderForType(ctx, "S3_CONNECTION_ISSUE")
	if err != nil {
		s.logger.Warn("Failed to get default notification provider for S3 connection issues", "error", err)
		return nil
	}

	if !provider.NotifyS3ConnectionIssue {
		// Provider is configured to not notify for S3 connection issues
		return nil
	}

	var config notifications.SMTPConfig
	if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Create notification content
	content := s.createS3ConnIssueContent(data)

	// Send the email
	if err := s.sendEmail(config, config.From, []string{config.From}, content); err != nil {
		return fmt.Errorf("failed to send S3 connection issue notification: %w", err)
	}

	s.logger.Info("Sent S3 connection issue notification", "targetName", data.TargetName)
	return nil
}

// SendNodeDowntimeNotification sends a notification for node downtime
func (s *NotificationService) SendNodeDowntimeNotification(ctx context.Context, data notifications.NodeDowntimeData) error {
	// Get default notification provider for node downtime
	provider, err := s.queries.GetDefaultNotificationProviderForType(ctx, "NODE_DOWNTIME")
	if err != nil {
		s.logger.Warn("Failed to get default notification provider for node downtime", "error", err)
		return nil
	}

	if !provider.NotifyNodeDowntime {
		// Provider is configured to not notify for node downtime
		return nil
	}

	var config notifications.SMTPConfig
	if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Create notification content
	content := s.createNodeDowntimeContent(data)

	// Send the email
	if err := s.sendEmail(config, config.From, []string{config.From}, content); err != nil {
		return fmt.Errorf("failed to send node downtime notification: %w", err)
	}

	s.logger.Info("Sent node downtime notification", "nodeID", data.NodeID, "nodeName", data.NodeName)
	return nil
}

// SendNodeRecoveryNotification sends a notification for node recovery
func (s *NotificationService) SendNodeRecoveryNotification(ctx context.Context, data notifications.NodeUpData) error {
	// Get default notification provider for node downtime (same provider handles recovery)
	provider, err := s.queries.GetDefaultNotificationProviderForType(ctx, "NODE_DOWNTIME")
	if err != nil {
		s.logger.Warn("Failed to get default notification provider for node recovery", "error", err)
		return nil
	}

	if !provider.NotifyNodeDowntime {
		// Provider is configured to not notify for node events
		return nil
	}

	var config notifications.SMTPConfig
	if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Create notification content
	content := s.createNodeRecoveryContent(data)

	// Send the email
	if err := s.sendEmail(config, config.From, []string{config.From}, content); err != nil {
		return fmt.Errorf("failed to send node recovery notification: %w", err)
	}

	s.logger.Info("Sent node recovery notification", "nodeID", data.NodeID, "nodeName", data.NodeName)
	return nil
}

// createNodeRecoveryContent creates the email content for node recovery notifications
func (s *NotificationService) createNodeRecoveryContent(data notifications.NodeUpData) EmailContent {
	// Create plain text content
	plainText := fmt.Sprintf(`Node Recovery Detected
		
A node in your infrastructure has recovered and is now online.

Details:
- Node ID: %d
- Node Name: %s
- Node URL: %s
- Down Since: %s
- Recovered At: %s
- Downtime Duration: %s
- Response Time: %s

Your node is now functioning normally.`,
		data.NodeID, data.NodeName, data.NodeURL,
		data.DownSince.Format(time.RFC3339), data.RecoveredAt.Format(time.RFC3339),
		data.Duration, data.ResponseTime)

	// Create HTML content
	html := fmt.Sprintf(`
	<html>
		<body>
			<h2>Node Recovery Detected</h2>
			<p>A node in your infrastructure has recovered and is now online.</p>
			
			<h3>Details:</h3>
			<ul>
				<li><strong>Node ID:</strong> %d</li>
				<li><strong>Node Name:</strong> %s</li>
				<li><strong>Node URL:</strong> %s</li>
				<li><strong>Down Since:</strong> %s</li>
				<li><strong>Recovered At:</strong> %s</li>
				<li><strong>Downtime Duration:</strong> %s</li>
				<li><strong>Response Time:</strong> %s</li>
			</ul>
			
			<p style="color: green;">Your node is now functioning normally.</p>
			<hr>
			<small>Sent from ChainDeploy</small>
		</body>
	</html>`,
		data.NodeID, data.NodeName, data.NodeURL,
		data.DownSince.Format(time.RFC3339), data.RecoveredAt.Format(time.RFC3339),
		data.Duration, data.ResponseTime)

	return EmailContent{
		Subject:   fmt.Sprintf("Node Recovery: %s", data.NodeName),
		PlainText: plainText,
		HTML:      html,
	}
}

func (s *NotificationService) createNotificationContent(notificationType notifications.NotificationType, data interface{}) EmailContent {
	switch notificationType {
	case notifications.NotificationTypeNodeDowntime:
		if nodeData, ok := data.(notifications.NodeDowntimeData); ok {
			return s.createNodeDowntimeContent(nodeData)
		}
	case notifications.NotificationTypeBackupSuccess:
		if backupData, ok := data.(notifications.BackupSuccessData); ok {
			return s.createBackupSuccessContent(backupData)
		}
	case notifications.NotificationTypeBackupFailure:
		if backupData, ok := data.(notifications.BackupFailureData); ok {
			return s.createBackupFailureContent(backupData)
		}
	case notifications.NotificationTypeS3ConnIssue:
		if s3Data, ok := data.(notifications.S3ConnectionIssueData); ok {
			return s.createS3ConnIssueContent(s3Data)
		}
	}

	// Fallback for invalid data type
	s.logger.Error("Invalid data type for notification", "type", notificationType)
	return EmailContent{
		Subject:   "ChainDeploy Notification",
		PlainText: "Notification data format error",
		HTML:      "<p>Notification data format error</p>",
	}
}

func (s *NotificationService) createNodeDowntimeContent(data notifications.NodeDowntimeData) EmailContent {
	// Create plain text content
	plainText := fmt.Sprintf(`Node Downtime Detected
		
A node in your infrastructure is experiencing downtime.

Details:
- Node ID: %d
- Node Name: %s
- Node Type: %s
- Network: %s
- Endpoint: %s
- Downtime Start: %s
- Last Seen: %s
- Duration: %s
- Error: %s

Please check your node status immediately.`,
		data.NodeID, data.NodeName, data.NodeType, data.NetworkName,
		data.Endpoint, data.DowntimeStart.Format(time.RFC3339),
		data.LastSeen.Format(time.RFC3339), data.Duration, data.ErrorMessage)

	// Create HTML content
	html := fmt.Sprintf(`
	<html>
		<body>
			<h2 style="color: #ff4444;">⚠️ Node Downtime Alert</h2>
			<p>A node in your infrastructure is experiencing downtime.</p>
			<div style="background: #f8f9fa; padding: 15px; border-radius: 5px; margin-bottom: 20px;">
				<h3>Details:</h3>
				<table style="width: 100%%;">
					<tr>
						<td style="padding: 8px; font-weight: bold;">Node ID:</td>
						<td style="padding: 8px;">%d</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Node Name:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Node Type:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Network:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Endpoint:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Downtime Start:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Last Seen:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Duration:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
				</table>
			</div>
			<div style="background: #f8d7da; color: #721c24; padding: 15px; border-radius: 5px; margin-bottom: 20px;">
				<h3>Error Message:</h3>
				<p style="font-family: monospace; white-space: pre-wrap;">%s</p>
			</div>
			<p style="color: #ff4444;">Please check your node status immediately.</p>
			<hr>
			<small>Sent from ChainDeploy</small>
		</body>
	</html>`, data.NodeID, data.NodeName, data.NodeType, data.NetworkName,
		data.Endpoint, data.DowntimeStart.Format(time.RFC3339),
		data.LastSeen.Format(time.RFC3339), data.Duration, data.ErrorMessage)

	return EmailContent{
		Subject:   fmt.Sprintf("Node Downtime Alert - %s - ChainDeploy", data.NodeName),
		PlainText: plainText,
		HTML:      html,
	}
}

func (s *NotificationService) createBackupSuccessContent(data notifications.BackupSuccessData) EmailContent {
	// Format size in human-readable format
	sizeFormatted := formatBytes(data.SizeBytes)

	// Create plain text content
	plainText := fmt.Sprintf(`Backup Completed Successfully

Your scheduled backup has completed successfully.

Details:
- Backup ID: %d
- Schedule: %s
- Target: %s (%s)
- Storage: %s on %s
- Size: %s
- Started at: %s
- Completed at: %s
- Duration: %s
- Retention: %d days
- Schedule: %s

Your data is now safely backed up.`,
		data.BackupID, data.ScheduleName, data.TargetName, data.TargetType,
		data.BucketName, data.Endpoint, sizeFormatted,
		data.StartedAt.Format(time.RFC3339), data.SuccessTime.Format(time.RFC3339),
		data.Duration, data.RetentionDays, data.CronExpression)

	// Create HTML content
	html := fmt.Sprintf(`
	<html>
		<body>
			<h2 style="color: #28a745;">✅ Backup Success</h2>
			<p>Your scheduled backup has completed successfully.</p>
			<div style="background: #f8f9fa; padding: 15px; border-radius: 5px; margin-bottom: 20px;">
				<h3>Details:</h3>
				<table style="width: 100%%;">
					<tr>
						<td style="padding: 8px; font-weight: bold;">Backup ID:</td>
						<td style="padding: 8px;">%d</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Schedule:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Target:</td>
						<td style="padding: 8px;">%s (%s)</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Storage:</td>
						<td style="padding: 8px;">%s on %s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Size:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Started at:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Completed at:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Duration:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Retention:</td>
						<td style="padding: 8px;">%d days</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Schedule:</td>
						<td style="padding: 8px;"><code>%s</code></td>
					</tr>
				</table>
			</div>
			<p>Your data is now safely backed up.</p>
			<hr>
			<small>Sent from ChainDeploy</small>
		</body>
	</html>`, data.BackupID, data.ScheduleName, data.TargetName, data.TargetType,
		data.BucketName, data.Endpoint, sizeFormatted,
		data.StartedAt.Format(time.RFC3339), data.SuccessTime.Format(time.RFC3339),
		data.Duration, data.RetentionDays, data.CronExpression)

	return EmailContent{
		Subject:   fmt.Sprintf("Backup Completed Successfully - %s - ChainDeploy", data.ScheduleName),
		PlainText: plainText,
		HTML:      html,
	}
}

func (s *NotificationService) createBackupFailureContent(data notifications.BackupFailureData) EmailContent {
	// Create plain text content
	plainText := fmt.Sprintf(`Backup Failure Alert

A scheduled backup operation has failed.

Details:
- Backup ID: %d
- Schedule: %s
- Target: %s (%s)
- Storage: %s on %s
- Started at: %s
- Failed at: %s
- Duration: %s
- Schedule: %s

Error:
%s

Please check your backup configuration and storage settings.
This requires immediate attention to ensure data safety.`,
		data.BackupID, data.ScheduleName, data.TargetName, data.TargetType,
		data.BucketName, data.Endpoint, data.StartedAt.Format(time.RFC3339),
		data.FailureTime.Format(time.RFC3339), data.Duration,
		data.CronExpression, data.ErrorMessage)

	// Create HTML content
	html := fmt.Sprintf(`
	<html>
		<body>
			<h2 style="color: #dc3545;">❌ Backup Failure Alert</h2>
			<p>A scheduled backup operation has failed.</p>
			<div style="background: #f8f9fa; padding: 15px; border-radius: 5px; margin-bottom: 20px;">
				<h3>Details:</h3>
				<table style="width: 100%%;">
					<tr>
						<td style="padding: 8px; font-weight: bold;">Backup ID:</td>
						<td style="padding: 8px;">%d</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Schedule:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Target:</td>
						<td style="padding: 8px;">%s (%s)</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Storage:</td>
						<td style="padding: 8px;">%s on %s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Started at:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Failed at:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Duration:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Schedule:</td>
						<td style="padding: 8px;"><code>%s</code></td>
					</tr>
				</table>
			</div>
			<div style="background: #f8d7da; color: #721c24; padding: 15px; border-radius: 5px; margin-bottom: 20px;">
				<h3>Error Message:</h3>
				<p style="font-family: monospace; white-space: pre-wrap;">%s</p>
			</div>
			<p style="color: #dc3545;">Please check your backup configuration and storage settings.</p>
			<p><strong>This requires immediate attention to ensure data safety.</strong></p>
			<hr>
			<small>Sent from ChainDeploy</small>
		</body>
	</html>`, data.BackupID, data.ScheduleName, data.TargetName, data.TargetType,
		data.BucketName, data.Endpoint, data.StartedAt.Format(time.RFC3339),
		data.FailureTime.Format(time.RFC3339), data.Duration,
		data.CronExpression, data.ErrorMessage)

	return EmailContent{
		Subject:   fmt.Sprintf("Backup Failed - %s - ChainDeploy Alert", data.ScheduleName),
		PlainText: plainText,
		HTML:      html,
	}
}

// formatBytes converts bytes to a human-readable string (KB, MB, GB, etc.)
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func (s *NotificationService) createS3ConnIssueContent(data notifications.S3ConnectionIssueData) EmailContent {
	// Create plain text content
	plainText := fmt.Sprintf(`S3 Connection Issue Detected

Connection issues with your S3 storage have been detected.

Details:
- Target: %s
- Endpoint: %s
- Bucket: %s
- Detected at: %s
- Error: %s

This may affect backup operations and data storage.
Please verify your S3 configuration and credentials.`,
		data.TargetName, data.Endpoint, data.BucketName,
		data.DetectedTime.Format(time.RFC3339), data.ErrorMessage)

	// Create HTML content
	html := fmt.Sprintf(`
	<html>
		<body>
			<h2 style="color: #ffc107;">⚠️ S3 Connection Issue</h2>
			<p>Connection issues with your S3 storage have been detected.</p>
			<div style="background: #f8f9fa; padding: 15px; border-radius: 5px; margin-bottom: 20px;">
				<h3>Details:</h3>
				<table style="width: 100%%;">
					<tr>
						<td style="padding: 8px; font-weight: bold;">Target:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Endpoint:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Bucket:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Detected at:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
				</table>
			</div>
			<div style="background: #fff3cd; color: #856404; padding: 15px; border-radius: 5px; margin-bottom: 20px;">
				<h3>Error Message:</h3>
				<p style="font-family: monospace; white-space: pre-wrap;">%s</p>
			</div>
			<div style="margin-top: 20px;">
				<p><strong>Potential impacts:</strong></p>
				<ul>
					<li>Backup operations may fail</li>
					<li>Data storage operations may be affected</li>
					<li>System reliability might be compromised</li>
				</ul>
			</div>
			<p style="color: #ffc107;"><strong>Please verify your S3 configuration and credentials.</strong></p>
			<hr>
			<small>Sent from ChainDeploy</small>
		</body>
	</html>`, data.TargetName, data.Endpoint, data.BucketName,
		data.DetectedTime.Format(time.RFC3339), data.ErrorMessage)

	return EmailContent{
		Subject:   fmt.Sprintf("S3 Connection Issue - %s - ChainDeploy Alert", data.TargetName),
		PlainText: plainText,
		HTML:      html,
	}
}

// SendNotification sends a notification with the specified type and data
// This is a legacy method that converts the generic data to typed data and calls the appropriate typed method
// It's recommended to use the typed methods directly instead
func (s *NotificationService) SendNotification(ctx context.Context, providerID int64, notificationType notifications.NotificationType, data interface{}) error {
	s.logger.Warn("Using deprecated SendNotification method - consider using typed notification methods instead")

	switch notificationType {
	case notifications.NotificationTypeBackupSuccess:
		// Try to convert to BackupSuccessData
		if typedData, ok := data.(notifications.BackupSuccessData); ok {
			return s.SendBackupSuccessNotification(ctx, typedData)
		}

		// Try to convert from map
		if mapData, ok := data.(map[string]interface{}); ok {
			// Extract and convert fields
			typedData, err := convertMapToBackupSuccessData(mapData)
			if err != nil {
				return fmt.Errorf("failed to convert data: %w", err)
			}
			return s.SendBackupSuccessNotification(ctx, typedData)
		}

	case notifications.NotificationTypeBackupFailure:
		// Try to convert to BackupFailureData
		if typedData, ok := data.(notifications.BackupFailureData); ok {
			return s.SendBackupFailureNotification(ctx, typedData)
		}

		// Try to convert from map
		if mapData, ok := data.(map[string]interface{}); ok {
			// Extract and convert fields
			typedData, err := convertMapToBackupFailureData(mapData)
			if err != nil {
				return fmt.Errorf("failed to convert data: %w", err)
			}
			return s.SendBackupFailureNotification(ctx, typedData)
		}

	case notifications.NotificationTypeS3ConnIssue:
		// Try to convert to S3ConnectionIssueData
		if typedData, ok := data.(notifications.S3ConnectionIssueData); ok {
			return s.SendS3ConnectionIssueNotification(ctx, typedData)
		}

		// Try to convert from map
		if mapData, ok := data.(map[string]interface{}); ok {
			// Extract and convert fields
			typedData, err := convertMapToS3ConnectionIssueData(mapData)
			if err != nil {
				return fmt.Errorf("failed to convert data: %w", err)
			}
			return s.SendS3ConnectionIssueNotification(ctx, typedData)
		}

	case notifications.NotificationTypeNodeDowntime:
		// Try to convert to NodeDowntimeData
		if typedData, ok := data.(notifications.NodeDowntimeData); ok {
			return s.SendNodeDowntimeNotification(ctx, typedData)
		}

		// Try to convert from map
		if mapData, ok := data.(map[string]interface{}); ok {
			// Extract and convert fields
			typedData, err := convertMapToNodeDowntimeData(mapData)
			if err != nil {
				return fmt.Errorf("failed to convert data: %w", err)
			}
			return s.SendNodeDowntimeNotification(ctx, typedData)
		}
	}

	return fmt.Errorf("unsupported notification type or data format")
}

// Helper functions to convert map[string]interface{} to typed structs

func convertMapToBackupSuccessData(data map[string]interface{}) (notifications.BackupSuccessData, error) {
	result := notifications.BackupSuccessData{}

	// Extract and convert fields
	if v, ok := data["backupId"]; ok {
		if id, ok := v.(int64); ok {
			result.BackupID = id
		} else if id, ok := v.(float64); ok {
			result.BackupID = int64(id)
		}
	}

	if v, ok := data["scheduleName"]; ok {
		if s, ok := v.(string); ok {
			result.ScheduleName = s
		}
	}

	if v, ok := data["targetName"]; ok {
		if s, ok := v.(string); ok {
			result.TargetName = s
		}
	}

	if v, ok := data["targetType"]; ok {
		if s, ok := v.(string); ok {
			result.TargetType = s
		}
	}

	if v, ok := data["bucketName"]; ok {
		if s, ok := v.(string); ok {
			result.BucketName = s
		}
	}

	if v, ok := data["endpoint"]; ok {
		if s, ok := v.(string); ok {
			result.Endpoint = s
		}
	}

	if v, ok := data["sizeBytes"]; ok {
		if size, ok := v.(int64); ok {
			result.SizeBytes = size
		} else if size, ok := v.(float64); ok {
			result.SizeBytes = int64(size)
		}
	}

	if v, ok := data["successTime"]; ok {
		if t, ok := v.(time.Time); ok {
			result.SuccessTime = t
		} else if s, ok := v.(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				result.SuccessTime = t
			}
		}
	} else {
		result.SuccessTime = time.Now()
	}

	if v, ok := data["startedAt"]; ok {
		if t, ok := v.(time.Time); ok {
			result.StartedAt = t
		} else if s, ok := v.(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				result.StartedAt = t
			}
		}
	}

	if v, ok := data["duration"]; ok {
		if s, ok := v.(string); ok {
			result.Duration = s
		}
	}

	if v, ok := data["retentionDays"]; ok {
		if days, ok := v.(int64); ok {
			result.RetentionDays = days
		} else if days, ok := v.(float64); ok {
			result.RetentionDays = int64(days)
		} else if days, ok := v.(int); ok {
			result.RetentionDays = int64(days)
		}
	}

	if v, ok := data["cronExpression"]; ok {
		if s, ok := v.(string); ok {
			result.CronExpression = s
		}
	}

	return result, nil
}

func convertMapToBackupFailureData(data map[string]interface{}) (notifications.BackupFailureData, error) {
	result := notifications.BackupFailureData{}

	// Extract and convert fields
	if v, ok := data["backupId"]; ok {
		if id, ok := v.(int64); ok {
			result.BackupID = id
		} else if id, ok := v.(float64); ok {
			result.BackupID = int64(id)
		}
	}

	if v, ok := data["scheduleName"]; ok {
		if s, ok := v.(string); ok {
			result.ScheduleName = s
		}
	}

	if v, ok := data["targetName"]; ok {
		if s, ok := v.(string); ok {
			result.TargetName = s
		}
	}

	if v, ok := data["targetType"]; ok {
		if s, ok := v.(string); ok {
			result.TargetType = s
		}
	}

	if v, ok := data["bucketName"]; ok {
		if s, ok := v.(string); ok {
			result.BucketName = s
		}
	}

	if v, ok := data["endpoint"]; ok {
		if s, ok := v.(string); ok {
			result.Endpoint = s
		}
	}

	if v, ok := data["errorMessage"]; ok {
		if s, ok := v.(string); ok {
			result.ErrorMessage = s
		}
	}

	if v, ok := data["failureTime"]; ok {
		if t, ok := v.(time.Time); ok {
			result.FailureTime = t
		} else if s, ok := v.(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				result.FailureTime = t
			}
		}
	} else {
		result.FailureTime = time.Now()
	}

	if v, ok := data["startedAt"]; ok {
		if t, ok := v.(time.Time); ok {
			result.StartedAt = t
		} else if s, ok := v.(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				result.StartedAt = t
			}
		}
	}

	if v, ok := data["duration"]; ok {
		if s, ok := v.(string); ok {
			result.Duration = s
		}
	}

	if v, ok := data["retentionDays"]; ok {
		if days, ok := v.(int64); ok {
			result.RetentionDays = days
		} else if days, ok := v.(float64); ok {
			result.RetentionDays = int64(days)
		}
	}

	if v, ok := data["cronExpression"]; ok {
		if s, ok := v.(string); ok {
			result.CronExpression = s
		}
	}

	return result, nil
}

func convertMapToS3ConnectionIssueData(data map[string]interface{}) (notifications.S3ConnectionIssueData, error) {
	result := notifications.S3ConnectionIssueData{}

	// Extract and convert fields
	if v, ok := data["targetName"]; ok {
		if s, ok := v.(string); ok {
			result.TargetName = s
		}
	}

	if v, ok := data["endpoint"]; ok {
		if s, ok := v.(string); ok {
			result.Endpoint = s
		}
	}

	if v, ok := data["bucketName"]; ok {
		if s, ok := v.(string); ok {
			result.BucketName = s
		}
	}

	if v, ok := data["errorMessage"]; ok {
		if s, ok := v.(string); ok {
			result.ErrorMessage = s
		}
	}

	if v, ok := data["detectedTime"]; ok {
		if t, ok := v.(time.Time); ok {
			result.DetectedTime = t
		} else if s, ok := v.(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				result.DetectedTime = t
			}
		}
	} else {
		result.DetectedTime = time.Now()
	}

	return result, nil
}

func convertMapToNodeDowntimeData(data map[string]interface{}) (notifications.NodeDowntimeData, error) {
	result := notifications.NodeDowntimeData{}

	// Extract and convert fields
	if v, ok := data["nodeId"]; ok {
		if id, ok := v.(int64); ok {
			result.NodeID = id
		} else if id, ok := v.(float64); ok {
			result.NodeID = int64(id)
		}
	}

	if v, ok := data["nodeName"]; ok {
		if s, ok := v.(string); ok {
			result.NodeName = s
		}
	}

	if v, ok := data["nodeType"]; ok {
		if s, ok := v.(string); ok {
			result.NodeType = s
		}
	}

	if v, ok := data["networkName"]; ok {
		if s, ok := v.(string); ok {
			result.NetworkName = s
		}
	}

	if v, ok := data["endpoint"]; ok {
		if s, ok := v.(string); ok {
			result.Endpoint = s
		}
	}

	if v, ok := data["downtimeStart"]; ok {
		if t, ok := v.(time.Time); ok {
			result.DowntimeStart = t
		} else if s, ok := v.(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				result.DowntimeStart = t
			}
		}
	} else {
		result.DowntimeStart = time.Now()
	}

	if v, ok := data["lastSeen"]; ok {
		if t, ok := v.(time.Time); ok {
			result.LastSeen = t
		} else if s, ok := v.(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				result.LastSeen = t
			}
		}
	}

	if v, ok := data["duration"]; ok {
		if s, ok := v.(string); ok {
			result.Duration = s
		}
	}

	if v, ok := data["errorMessage"]; ok {
		if s, ok := v.(string); ok {
			result.ErrorMessage = s
		}
	}

	return result, nil
}

// SendDiskSpaceWarningNotification sends a notification for disk space warnings
func (s *NotificationService) SendDiskSpaceWarningNotification(ctx context.Context, data notifications.DiskSpaceWarningData) error {
	// Get default notification provider for disk space warnings
	provider, err := s.queries.GetDefaultNotificationProviderForType(ctx, "DISK_SPACE_WARNING")
	if err != nil {
		s.logger.Warn("Failed to get default notification provider for disk space warnings", "error", err)
		return nil
	}

	if !provider.NotifyDiskSpaceWarning {
		// Provider is configured to not notify for disk space warnings
		return nil
	}

	var config notifications.SMTPConfig
	if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Create notification content
	content := s.createDiskSpaceWarningContent(data)

	// Send the email
	if err := s.sendEmail(config, config.From, []string{config.From}, content); err != nil {
		return fmt.Errorf("failed to send disk space warning notification: %w", err)
	}

	s.logger.Info("Sent disk space warning notification", "path", data.DataPath, "usedPercent", data.UsedPercent)
	return nil
}

// createDiskSpaceWarningContent creates the email content for disk space warning notifications
func (s *NotificationService) createDiskSpaceWarningContent(data notifications.DiskSpaceWarningData) EmailContent {
	// Format sizes in human-readable format
	usedFormatted := formatBytes(data.UsedBytes)
	availableFormatted := formatBytes(data.AvailableBytes)
	totalFormatted := formatBytes(data.TotalBytes)

	// Create plain text content
	plainText := fmt.Sprintf(`Disk Space Warning

Disk space usage has exceeded the configured threshold.

Details:
- Data Path: %s
- Mount Point: %s
- Used: %s (%.1f%%)
- Available: %s
- Total: %s
- Threshold: %.1f%%
- Detected at: %s

Recommended Actions:
1. Clean up old log files
2. Remove unused backup files
3. Consider expanding disk storage
4. Archive old data

Please take action to free up disk space to avoid service interruptions.`,
		data.DataPath, data.MountPoint, usedFormatted, data.UsedPercent,
		availableFormatted, totalFormatted, data.Threshold,
		data.DetectedTime.Format(time.RFC3339))

	// Create HTML content
	html := fmt.Sprintf(`
	<html>
		<body>
			<h2 style="color: #ffc107;">⚠️ Disk Space Warning</h2>
			<p>Disk space usage has exceeded the configured threshold.</p>
			<div style="background: #f8f9fa; padding: 15px; border-radius: 5px; margin-bottom: 20px;">
				<h3>Details:</h3>
				<table style="width: 100%%;">
					<tr>
						<td style="padding: 8px; font-weight: bold;">Data Path:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Mount Point:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Used:</td>
						<td style="padding: 8px;">%s <span style="color: #dc3545;">(%.1f%%)</span></td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Available:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Total:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Threshold:</td>
						<td style="padding: 8px;">%.1f%%</td>
					</tr>
					<tr>
						<td style="padding: 8px; font-weight: bold;">Detected at:</td>
						<td style="padding: 8px;">%s</td>
					</tr>
				</table>
			</div>
			<div style="background: #fff3cd; color: #856404; padding: 15px; border-radius: 5px; margin-bottom: 20px;">
				<h3>Recommended Actions:</h3>
				<ol>
					<li>Clean up old log files</li>
					<li>Remove unused backup files</li>
					<li>Consider expanding disk storage</li>
					<li>Archive old data</li>
				</ol>
			</div>
			<p style="color: #dc3545;"><strong>Please take action to free up disk space to avoid service interruptions.</strong></p>
			<hr>
			<small>Sent from ChainDeploy</small>
		</body>
	</html>`, data.DataPath, data.MountPoint, usedFormatted, data.UsedPercent,
		availableFormatted, totalFormatted, data.Threshold,
		data.DetectedTime.Format(time.RFC3339))

	return EmailContent{
		Subject:   "Disk Space Warning - ChainDeploy Alert",
		PlainText: plainText,
		HTML:      html,
	}
}
