package xsource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	ptypes "github.com/chainlaunch/chainlaunch/pkg/plugin/types"
)

// FileValue represents a file x-source value
type FileValue struct {
	BaseXSourceValue
	ConfigContents string
	FilePath       string
}

// Validate checks if the file exists and is accessible
func (f *FileValue) Validate(ctx context.Context) error {
	if f.ConfigContents == "" {
		return fmt.Errorf("config contents are required")
	}

	return nil
}

type FileTemplateValue struct {
	Path string
}

// GetValue returns the path inside the container where the file will be mounted
func (f *FileValue) GetValue(ctx context.Context, spec ptypes.ParameterSpec) (interface{}, error) {
	if err := f.Validate(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/etc/chainlaunch/files/%s", f.Key)
	fTemplate := FileTemplateValue{
		Path: path,
	}

	// Return the path inside the container
	return fTemplate, nil
}

// GetVolumeMounts returns the volume mount configuration for the file
func (f *FileValue) GetVolumeMounts(ctx context.Context) ([]VolumeMount, error) {
	if err := f.Validate(ctx); err != nil {
		return nil, err
	}

	// Create a temporary directory for the file
	tempDir := fmt.Sprintf("/tmp/chainlaunch/files/%s", f.Key)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Write the file contents to the temporary directory
	filePath := filepath.Join(tempDir, "file")
	if err := os.WriteFile(filePath, []byte(f.ConfigContents), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Get absolute path for the file
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create a volume mount for the file
	return []VolumeMount{
		{
			Source:      filePath,
			Target:      fmt.Sprintf("/etc/chainlaunch/files/%s", f.Key),
			Type:        "bind",
			ReadOnly:    true,
			Description: fmt.Sprintf("Mounts file %s into the container", absPath),
		},
	}, nil
}

// NewFileHandler creates a new FileHandler
func NewFileHandler() *FileHandler {
	return &FileHandler{}
}

// FileHandler implements XSourceHandler for file type
type FileHandler struct{}

// GetType returns the type of x-source this handler manages
func (h *FileHandler) GetType() XSourceType {
	return File
}

// CreateValue creates a new FileValue from the raw input
func (h *FileHandler) CreateValue(key string, rawValue interface{}) (XSourceValue, error) {
	configContents, ok := rawValue.(string)
	if !ok {
		return nil, fmt.Errorf("file path must be a string")
	}
	filePath := fmt.Sprintf("/etc/chainlaunch/files/%s", key)

	return &FileValue{
		BaseXSourceValue: BaseXSourceValue{
			Key:      key,
			RawValue: configContents,
		},
		ConfigContents: configContents,
		FilePath:       filePath,
	}, nil
}

// ListOptions returns an empty list as files are not selectable options
func (h *FileHandler) ListOptions(ctx context.Context) ([]OptionItem, error) {
	return []OptionItem{}, nil
}
