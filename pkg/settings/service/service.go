package service

import (
	"context"
	"encoding/json"
	"fmt"
	"text/template"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
)

// Default settings configuration
var defaultConfig = SettingConfig{
	PeerTemplateCMD:     "{{.Cmd}}",
	OrdererTemplateCMD:  "{{.Cmd}}",
	BesuTemplateCMD:     "{{.Cmd}}",
	DefaultNodeExposeIP: "127.0.0.1", // Default to all interfaces
}

// Setting represents a setting in the service layer
type Setting struct {
	ID        int64         `json:"id"`
	Config    SettingConfig `json:"config"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type SettingConfig struct {
	PeerTemplateCMD     string `json:"peerTemplateCMD"`
	OrdererTemplateCMD  string `json:"ordererTemplateCMD"`
	BesuTemplateCMD     string `json:"besuTemplateCMD"`
	DefaultNodeExposeIP string `json:"defaultNodeExposeIP"`
}

// CreateSettingParams represents the parameters for creating a setting
type CreateSettingParams struct {
	Config SettingConfig `json:"config"`
}

// UpdateSettingParams represents the parameters for updating a setting
type UpdateSettingParams struct {
	Config SettingConfig `json:"config"`
}

// SettingsService handles operations for settings
type SettingsService struct {
	queries *db.Queries
	logger  *logger.Logger
}

// NewSettingsService creates a new settings service
func NewSettingsService(queries *db.Queries, logger *logger.Logger) *SettingsService {
	return &SettingsService{
		queries: queries,
		logger:  logger,
	}
}

// validateTemplates checks if all templates in the config are valid Go templates
func validateTemplates(config SettingConfig) error {
	templates := map[string]string{
		"PeerTemplate":    config.PeerTemplateCMD,
		"OrdererTemplate": config.OrdererTemplateCMD,
		"BesuTemplate":    config.BesuTemplateCMD,
	}

	for name, tmpl := range templates {
		_, err := template.New(name).Parse(tmpl)
		if err != nil {
			return fmt.Errorf("invalid %s: %w", name, err)
		}
	}
	// Optionally, validate DefaultNodeExposeIP is a valid IP (not required, but could be added)
	return nil
}

// CreateSetting creates or updates the setting
func (s *SettingsService) CreateSetting(ctx context.Context, params CreateSettingParams) (*Setting, error) {
	// Validate templates before proceeding
	if err := validateTemplates(params.Config); err != nil {
		return nil, fmt.Errorf("template validation failed: %w", err)
	}

	configJSON, err := json.Marshal(params.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Get existing setting if any
	settings, err := s.queries.ListSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list settings: %w", err)
	}

	var dbSetting *db.Setting
	if len(settings) > 0 {
		// Update existing setting
		dbSetting, err = s.queries.UpdateSetting(ctx, &db.UpdateSettingParams{
			ID:     settings[0].ID,
			Config: string(configJSON),
		})
	} else {
		// Create new setting
		dbSetting, err = s.queries.CreateSetting(ctx, string(configJSON))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to save setting: %w", err)
	}

	var config SettingConfig
	if err := json.Unmarshal([]byte(dbSetting.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &Setting{
		ID:        dbSetting.ID,
		Config:    config,
		CreatedAt: dbSetting.CreatedAt.Time,
		UpdatedAt: dbSetting.UpdatedAt.Time,
	}, nil
}

// GetSetting retrieves the setting or initializes with defaults if none exist
func (s *SettingsService) GetSetting(ctx context.Context) (*Setting, error) {
	settings, err := s.queries.ListSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list settings: %w", err)
	}

	if len(settings) == 0 {
		return nil, fmt.Errorf("no settings found")
	}

	dbSetting := settings[0]

	var config SettingConfig
	if err := json.Unmarshal([]byte(dbSetting.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &Setting{
		ID:        dbSetting.ID,
		Config:    config,
		CreatedAt: dbSetting.CreatedAt.Time,
		UpdatedAt: dbSetting.UpdatedAt.Time,
	}, nil
}

// initializeDefaultSettings creates the default settings in the database
func (s *SettingsService) InitializeDefaultSettings(ctx context.Context) (*Setting, error) {
	configJSON, err := json.Marshal(defaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal default config: %w", err)
	}

	dbSetting, err := s.queries.CreateSetting(ctx, string(configJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create default settings: %w", err)
	}

	return &Setting{
		ID:        dbSetting.ID,
		Config:    defaultConfig,
		CreatedAt: dbSetting.CreatedAt.Time,
		UpdatedAt: dbSetting.UpdatedAt.Time,
	}, nil
}

// ListSettings returns all settings (only one row exists)
func (s *SettingsService) ListSettings(ctx context.Context) ([]*Setting, error) {
	settings, err := s.queries.ListSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list settings: %w", err)
	}

	if len(settings) == 0 {
		return nil, fmt.Errorf("no settings found")
	}

	dbSetting := settings[0]

	var config SettingConfig
	if err := json.Unmarshal([]byte(dbSetting.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return []*Setting{
		{
			ID:        dbSetting.ID,
			Config:    config,
			CreatedAt: dbSetting.CreatedAt.Time,
			UpdatedAt: dbSetting.UpdatedAt.Time,
		},
	}, nil
}

// UpdateSetting updates the setting
func (s *SettingsService) UpdateSetting(ctx context.Context, id int64, params UpdateSettingParams) (*Setting, error) {
	// Validate templates before proceeding
	if err := validateTemplates(params.Config); err != nil {
		return nil, fmt.Errorf("template validation failed: %w", err)
	}

	configJSON, err := json.Marshal(params.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	settings, err := s.queries.ListSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list settings: %w", err)
	}

	if len(settings) == 0 {
		return nil, fmt.Errorf("no settings found")
	}

	dbSetting, err := s.queries.UpdateSetting(ctx, &db.UpdateSettingParams{
		ID:     settings[0].ID,
		Config: string(configJSON),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update setting: %w", err)
	}

	var config SettingConfig
	if err := json.Unmarshal([]byte(dbSetting.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &Setting{
		ID:        dbSetting.ID,
		Config:    config,
		CreatedAt: dbSetting.CreatedAt.Time,
		UpdatedAt: dbSetting.UpdatedAt.Time,
	}, nil
}

// DeleteSetting is deprecated as we maintain one persistent setting
func (s *SettingsService) DeleteSetting(ctx context.Context, id int64) error {
	return fmt.Errorf("delete operation is not supported for settings")
}
