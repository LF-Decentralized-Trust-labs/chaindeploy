package boilerplates

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/google/go-github/v45/github"
	"gopkg.in/yaml.v3"
)

// BoilerplateConfig represents a boilerplate configuration
type BoilerplateConfig struct {
	ID              string   `yaml:"-" json:"id"` // ID is the key in the configs map
	Name            string   `yaml:"name" json:"name"`
	Description     string   `yaml:"description" json:"description"`
	Platform        string   `yaml:"platform" json:"platform"`
	Command         string   `yaml:"command" json:"command"`
	Args            []string `yaml:"args" json:"args"`
	Image           string   `yaml:"image" json:"image"`
	RepoOwner       string   `yaml:"repoOwner" json:"repoOwner"`
	RepoName        string   `yaml:"repoName" json:"repoName"`
	RepoPath        string   `yaml:"repoPath,omitempty" json:"repoPath,omitempty"`
	ValidateCommand string   `yaml:"validateCommand,omitempty" json:"validateCommand,omitempty"`
}

// BoilerplatesConfig represents the top-level configuration structure
type BoilerplatesConfig struct {
	Boilerplates map[string]BoilerplateConfig `yaml:"boilerplates"`
}

// BoilerplateService manages boilerplate templates and their configurations
type BoilerplateService struct {
	Queries   *db.Queries
	configs   map[string]BoilerplateConfig
	client    *github.Client
	owner     string
	repo      string
	path      string
	lastFetch time.Time
}

// NewBoilerplateService creates a new BoilerplateService instance
func NewBoilerplateService(queries *db.Queries) (*BoilerplateService, error) {
	service := &BoilerplateService{
		Queries: queries,
		configs: make(map[string]BoilerplateConfig),
		client:  github.NewClient(nil),
	}

	// Load configurations from the default location
	if err := service.loadConfigs(); err != nil {
		return nil, fmt.Errorf("failed to load boilerplate configs: %w", err)
	}

	return service, nil
}

// loadConfigs loads boilerplate configurations from the default location
func (s *BoilerplateService) loadConfigs() error {
	// Load from the embedded YAML file
	configPath := "configs/boilerplates.yaml"
	data, err := embedFS.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read boilerplate configs: %w", err)
	}

	var config BoilerplatesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse boilerplate configs: %w", err)
	}

	// Set the ID field for each config
	for id, boilerplateConfig := range config.Boilerplates {
		boilerplateConfig.ID = id
		config.Boilerplates[id] = boilerplateConfig
	}

	s.configs = config.Boilerplates
	return nil
}

// downloadContents recursively downloads files and directories from GitHub
func (s *BoilerplateService) downloadContents(url string, targetDir string) error {
	// Make the request to GitHub API
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch repository contents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch repository contents: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the response as JSON
	var contents []struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		Type        string `json:"type"`
		DownloadURL string `json:"download_url"`
	}
	if err := json.Unmarshal(body, &contents); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Process each item
	for _, item := range contents {
		targetPath := filepath.Join(targetDir, item.Name)

		if item.Type == "dir" {
			// Create directory and recursively download its contents
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", item.Name, err)
			}
			if err := s.downloadContents(item.DownloadURL, targetPath); err != nil {
				return fmt.Errorf("failed to download directory %s: %w", item.Name, err)
			}
		} else if item.Type == "file" {
			// Download the file
			resp, err := http.Get(item.DownloadURL)
			if err != nil {
				return fmt.Errorf("failed to download file %s: %w", item.Name, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to download file %s: %s", item.Name, resp.Status)
			}

			// Create the target directory if it doesn't exist
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", item.Name, err)
			}

			// Create the target file
			file, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", item.Name, err)
			}
			defer file.Close()

			// Copy the file contents
			if _, err := io.Copy(file, resp.Body); err != nil {
				return fmt.Errorf("failed to write file %s: %w", item.Name, err)
			}
		}
	}

	return nil
}

// DownloadBoilerplate downloads a boilerplate from GitHub
func (s *BoilerplateService) DownloadBoilerplate(ctx context.Context, name, targetDir string) error {
	config, err := s.GetBoilerplateConfig(name)
	if err != nil {
		return err
	}

	// Create the target directory if it doesn't exist
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Construct the GitHub archive URL
	url := fmt.Sprintf("https://github.com/%s/%s/archive/refs/heads/main.tar.gz", config.RepoOwner, config.RepoName)

	// Download the tarball
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download repository: %s", resp.Status)
	}

	// Create a gzip reader
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	// Create a tar reader
	tr := tar.NewReader(gzr)

	// Extract the tarball
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Skip the root directory
		if header.Name == fmt.Sprintf("%s-main/", config.RepoName) {
			continue
		}

		// If RepoPath is specified, only extract files from that path
		if config.RepoPath != "" {
			expectedPrefix := fmt.Sprintf("%s-main/%s/", config.RepoName, config.RepoPath)
			if !strings.HasPrefix(header.Name, expectedPrefix) {
				continue
			}
		}

		// Remove the root directory prefix
		targetPath := strings.TrimPrefix(header.Name, fmt.Sprintf("%s-main/", config.RepoName))
		targetPath = filepath.Join(targetDir, targetPath)

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
			}

			// Create the file
			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}

			// Copy the file contents
			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			file.Close()
		}
	}

	return nil
}

// RefreshConfigs reloads the configurations from GitHub if they're older than the specified duration
func (s *BoilerplateService) RefreshConfigs(maxAge time.Duration) error {
	if time.Since(s.lastFetch) > maxAge {
		return s.loadConfigs()
	}
	return nil
}

// GetBoilerplateConfig returns the configuration for a specific boilerplate
func (s *BoilerplateService) GetBoilerplateConfig(name string) (BoilerplateConfig, error) {
	config, ok := s.configs[name]
	if !ok {
		return BoilerplateConfig{}, fmt.Errorf("boilerplate not found: %s", name)
	}
	return config, nil
}

// GetBoilerplatesByPlatform returns all boilerplates for a specific platform
func (s *BoilerplateService) GetBoilerplatesByPlatform(platform string) []BoilerplateConfig {
	var result []BoilerplateConfig
	for id, config := range s.configs {
		if config.Platform == platform {
			config.ID = id
			result = append(result, config)
		}
	}
	return result
}

// GetBoilerplates returns all available boilerplates
func (s *BoilerplateService) GetBoilerplates() []BoilerplateConfig {
	var result []BoilerplateConfig
	for id, config := range s.configs {
		config.ID = id
		result = append(result, config)
	}
	return result
}

// GetBoilerplatesByNetworkID returns all boilerplates for a specific network
func (s *BoilerplateService) GetBoilerplatesByNetworkID(ctx context.Context, networkID int64) ([]BoilerplateConfig, error) {
	// Get network platform from database
	network, err := s.Queries.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	// Get boilerplates for the network's platform
	return s.GetBoilerplatesByPlatform(network.Platform), nil
}

//go:embed configs/boilerplates.yaml
var embedFS embed.FS
