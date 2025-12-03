package http

import (
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/notifications"
)

type CreateNotificationSettingRequest struct {
	Type    notifications.NotificationType `json:"type" validate:"required,oneof=NODE_DOWNTIME BACKUP_SUCCESS BACKUP_FAILURE S3_CONNECTION_ISSUE"`
	Enabled bool                           `json:"enabled"`
	Config  interface{}                    `json:"config" validate:"required"`
}

type UpdateNotificationSettingRequest struct {
	Type    notifications.NotificationType `json:"type" validate:"required,oneof=NODE_DOWNTIME BACKUP_SUCCESS BACKUP_FAILURE S3_CONNECTION_ISSUE"`
	Enabled bool                           `json:"enabled"`
	Config  interface{}                    `json:"config" validate:"required"`
}

type NotificationSettingResponse struct {
	ID         int64                          `json:"id"`
	Type       notifications.NotificationType `json:"type"`
	ProviderID int64                          `json:"providerId"`
	Enabled    bool                           `json:"enabled"`
	Config     interface{}                    `json:"config"`
	CreatedAt  time.Time                      `json:"createdAt"`
	UpdatedAt  *time.Time                     `json:"updatedAt,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type CreateProviderRequest struct {
	Type                   notifications.ProviderType `json:"type" validate:"required,oneof=SMTP"`
	Name                   string                     `json:"name" validate:"required,min=1,max=255"`
	Config                 interface{}                `json:"config" validate:"required"`
	IsDefault              bool                       `json:"isDefault"`
	NotifyNodeDowntime     bool                       `json:"notifyNodeDowntime"`
	NotifyBackupSuccess    bool                       `json:"notifyBackupSuccess"`
	NotifyBackupFailure    bool                       `json:"notifyBackupFailure"`
	NotifyS3ConnIssue      bool                       `json:"notifyS3ConnIssue"`
	NotifyDiskSpaceWarning bool                       `json:"notifyDiskSpaceWarning"`
}

type UpdateProviderRequest struct {
	Type                   notifications.ProviderType `json:"type" validate:"required,oneof=SMTP"`
	Name                   string                     `json:"name" validate:"required,min=1,max=255"`
	Config                 interface{}                `json:"config" validate:"required"`
	IsDefault              bool                       `json:"isDefault"`
	NotifyNodeDowntime     bool                       `json:"notifyNodeDowntime"`
	NotifyBackupSuccess    bool                       `json:"notifyBackupSuccess"`
	NotifyBackupFailure    bool                       `json:"notifyBackupFailure"`
	NotifyS3ConnIssue      bool                       `json:"notifyS3ConnIssue"`
	NotifyDiskSpaceWarning bool                       `json:"notifyDiskSpaceWarning"`
}

type ProviderResponse struct {
	ID                     int64                      `json:"id"`
	Type                   notifications.ProviderType `json:"type"`
	Name                   string                     `json:"name"`
	Config                 interface{}                `json:"config"`
	IsDefault              bool                       `json:"isDefault"`
	NotifyNodeDowntime     bool                       `json:"notifyNodeDowntime"`
	NotifyBackupSuccess    bool                       `json:"notifyBackupSuccess"`
	NotifyBackupFailure    bool                       `json:"notifyBackupFailure"`
	NotifyS3ConnIssue      bool                       `json:"notifyS3ConnIssue"`
	NotifyDiskSpaceWarning bool                       `json:"notifyDiskSpaceWarning"`
	LastTestAt             *time.Time                 `json:"lastTestAt,omitempty"`
	LastTestStatus         string                     `json:"lastTestStatus,omitempty"`
	LastTestMessage        string                     `json:"lastTestMessage,omitempty"`
	CreatedAt              time.Time                  `json:"createdAt"`
	UpdatedAt              *time.Time                 `json:"updatedAt,omitempty"`
}

// TestProviderRequest represents the request to test a provider
type TestProviderRequest struct {
	TestEmail string `json:"testEmail" validate:"required,email"`
}

// TestProviderResponse represents the response from testing a provider
type TestProviderResponse struct {
	Status   string    `json:"status"`
	Message  string    `json:"message"`
	TestedAt time.Time `json:"testedAt"`
}
