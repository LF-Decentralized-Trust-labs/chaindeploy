package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"go.uber.org/zap"
)

// Boilerplate represents a project boilerplate template
type Boilerplate struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Platform    string `json:"platform"`
	Path        string `json:"path"`
}

// BoilerplateService handles boilerplate-related operations
type BoilerplateService struct {
	Queries         *db.Queries
	BoilerplatesDir string
}

// NewBoilerplateService creates a new BoilerplateService instance
func NewBoilerplateService(queries *db.Queries, boilerplatesDir string) *BoilerplateService {
	return &BoilerplateService{
		Queries:         queries,
		BoilerplatesDir: boilerplatesDir,
	}
}

// GetBoilerplates returns a list of available boilerplates filtered by network platform
func (s *BoilerplateService) GetBoilerplates(ctx context.Context, networkID int64) ([]Boilerplate, error) {
	// Get network platform from network ID
	network, err := s.Queries.GetNetwork(ctx, networkID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("network not found")
		}
		return nil, fmt.Errorf("failed to get network: %w", err)
	}

	// List boilerplate directories
	entries, err := os.ReadDir(s.BoilerplatesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read boilerplates directory: %w", err)
	}

	var boilerplates []Boilerplate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Read boilerplate metadata
		metadataPath := filepath.Join(s.BoilerplatesDir, entry.Name(), "metadata.json")
		metadata, err := os.ReadFile(metadataPath)
		if err != nil {
			zap.L().Warn("failed to read boilerplate metadata",
				zap.String("boilerplate", entry.Name()),
				zap.Error(err))
			continue
		}

		var meta struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Platform    string `json:"platform"`
		}
		if err := json.Unmarshal(metadata, &meta); err != nil {
			zap.L().Warn("failed to parse boilerplate metadata",
				zap.String("boilerplate", entry.Name()),
				zap.Error(err))
			continue
		}

		// Filter by platform
		if meta.Platform != network.Platform {
			continue
		}

		boilerplates = append(boilerplates, Boilerplate{
			Name:        meta.Name,
			Description: meta.Description,
			Platform:    meta.Platform,
			Path:        entry.Name(),
		})
	}

	return boilerplates, nil
}
