package notifications

import "time"

// NotificationType represents different types of notifications
type NotificationType string

const (
	NotificationTypeNodeDowntime     NotificationType = "NODE_DOWNTIME"
	NotificationTypeBackupSuccess    NotificationType = "BACKUP_SUCCESS"
	NotificationTypeBackupFailure    NotificationType = "BACKUP_FAILURE"
	NotificationTypeS3ConnIssue      NotificationType = "S3_CONNECTION_ISSUE"
	NotificationTypeDiskSpaceWarning NotificationType = "DISK_SPACE_WARNING"
)

// NotificationDeliveryType represents different notification providers
type NotificationDeliveryType string

const (
	NotificationDeliveryEmail NotificationDeliveryType = "EMAIL"
)

// ProviderType represents the type of notification provider
type ProviderType string

const (
	ProviderTypeSMTP ProviderType = "SMTP"
)

// NotificationProvider represents a notification provider configuration
type NotificationProvider struct {
	ID                     int64        `json:"id"`
	Type                   ProviderType `json:"type"`
	Name                   string       `json:"name"`
	Config                 interface{}  `json:"config"`
	IsDefault              bool         `json:"isDefault"`
	NotifyNodeDowntime     bool         `json:"notifyNodeDowntime"`
	NotifyBackupSuccess    bool         `json:"notifyBackupSuccess"`
	NotifyBackupFailure    bool         `json:"notifyBackupFailure"`
	NotifyS3ConnIssue      bool         `json:"notifyS3ConnIssue"`
	NotifyDiskSpaceWarning bool         `json:"notifyDiskSpaceWarning"`
	LastTestAt             *time.Time   `json:"lastTestAt,omitempty"`
	LastTestStatus         string       `json:"lastTestStatus,omitempty"`
	LastTestMessage        string       `json:"lastTestMessage,omitempty"`
	CreatedAt              time.Time    `json:"createdAt"`
	UpdatedAt              time.Time    `json:"updatedAt,omitempty"`
}

// CreateProviderParams represents parameters for creating a provider
type CreateProviderParams struct {
	Type                   ProviderType `validate:"required,oneof=SMTP"`
	Name                   string       `validate:"required,min=1,max=255"`
	Config                 interface{}  `validate:"required"`
	IsDefault              bool
	NotifyNodeDowntime     bool
	NotifyBackupSuccess    bool
	NotifyBackupFailure    bool
	NotifyS3ConnIssue      bool
	NotifyDiskSpaceWarning bool
}

// UpdateProviderParams represents parameters for updating a provider
type UpdateProviderParams struct {
	ID                     int64        `validate:"required"`
	Type                   ProviderType `validate:"required,oneof=SMTP"`
	Name                   string       `validate:"required,min=1,max=255"`
	Config                 interface{}  `validate:"required"`
	IsDefault              bool
	NotifyNodeDowntime     bool
	NotifyBackupSuccess    bool
	NotifyBackupFailure    bool
	NotifyS3ConnIssue      bool
	NotifyDiskSpaceWarning bool
}

// SMTPConfig represents SMTP provider configuration
type SMTPConfig struct {
	Host       string   `json:"host" validate:"required"`
	Port       int      `json:"port" validate:"required,min=1,max=65535"`
	Username   string   `json:"username" validate:"required"`
	Password   string   `json:"password" validate:"required"`
	From       string   `json:"from" validate:"required,email"`
	TLS        bool     `json:"tls"`
	Recipients []string `json:"recipients,omitempty"` // Optional list of recipient email addresses. If empty, defaults to From address.
}

// TestProviderParams represents parameters for testing a provider
type TestProviderParams struct {
	TestEmail string `json:"testEmail" validate:"required,email"`
}

// TestResult represents the result of testing a notification provider
type TestResult struct {
	Status   string    `json:"status"`
	Message  string    `json:"message"`
	TestedAt time.Time `json:"testedAt"`
}

// BackupSuccessData represents data for backup success notifications
type BackupSuccessData struct {
	BackupID       int64     `json:"backupId"`
	ScheduleName   string    `json:"scheduleName"`
	TargetName     string    `json:"targetName"`
	TargetType     string    `json:"targetType"`
	BucketName     string    `json:"bucketName"`
	Endpoint       string    `json:"endpoint"`
	SizeBytes      int64     `json:"sizeBytes"`
	SuccessTime    time.Time `json:"successTime"`
	StartedAt      time.Time `json:"startedAt"`
	Duration       string    `json:"duration"`
	RetentionDays  int64     `json:"retentionDays"`
	CronExpression string    `json:"cronExpression"`
}

// BackupFailureData represents data for backup failure notifications
type BackupFailureData struct {
	BackupID       int64     `json:"backupId"`
	ScheduleName   string    `json:"scheduleName"`
	TargetName     string    `json:"targetName"`
	TargetType     string    `json:"targetType"`
	BucketName     string    `json:"bucketName"`
	Endpoint       string    `json:"endpoint"`
	ErrorMessage   string    `json:"errorMessage"`
	FailureTime    time.Time `json:"failureTime"`
	StartedAt      time.Time `json:"startedAt"`
	Duration       string    `json:"duration"`
	RetentionDays  int64     `json:"retentionDays"`
	CronExpression string    `json:"cronExpression"`
}

// S3ConnectionIssueData represents data for S3 connection issue notifications
type S3ConnectionIssueData struct {
	TargetName   string    `json:"targetName"`
	Endpoint     string    `json:"endpoint"`
	BucketName   string    `json:"bucketName"`
	ErrorMessage string    `json:"errorMessage"`
	DetectedTime time.Time `json:"detectedTime"`
}

// NodeDowntimeData represents data for node downtime notifications
type NodeDowntimeData struct {
	NodeID        int64     `json:"nodeId"`
	NodeName      string    `json:"nodeName"`
	NodeType      string    `json:"nodeType"`
	NetworkName   string    `json:"networkName"`
	Endpoint      string    `json:"endpoint"`
	DowntimeStart time.Time `json:"downtimeStart"`
	LastSeen      time.Time `json:"lastSeen"`
	Duration      string    `json:"duration"`
	ErrorMessage  string    `json:"errorMessage"`
	NodeURL       string    `json:"nodeURL"`
	DownSince     time.Time `json:"downSince"`
	FailureCount  int       `json:"failureCount"`
	Error         string    `json:"error"`
}

type NodeUpData struct {
	NodeID           int64         `json:"nodeId"`
	NodeName         string        `json:"nodeName"`
	NodeURL          string        `json:"nodeURL"`
	DownSince        time.Time     `json:"downSince"`
	RecoveredAt      time.Time     `json:"recoveredAt"`
	Duration         string        `json:"duration"`
	ResponseTime     time.Duration `json:"responseTime"`
	DowntimeDuration time.Duration `json:"downtimeDuration"`
}

// DiskSpaceWarningData represents data for disk space warning notifications
type DiskSpaceWarningData struct {
	DataPath       string    `json:"dataPath"`
	UsedBytes      int64     `json:"usedBytes"`
	AvailableBytes int64     `json:"availableBytes"`
	TotalBytes     int64     `json:"totalBytes"`
	UsedPercent    float64   `json:"usedPercent"`
	Threshold      float64   `json:"threshold"`
	DetectedTime   time.Time `json:"detectedTime"`
	MountPoint     string    `json:"mountPoint"`
}
