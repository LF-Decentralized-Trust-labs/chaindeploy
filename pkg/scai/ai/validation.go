package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/scai/boilerplates"
	"github.com/chainlaunch/chainlaunch/pkg/scai/projectrunner"
)

// ValidationService handles project validation after file operations
type ValidationService struct {
	queries            *db.Queries
	boilerplateService *boilerplates.BoilerplateService
	Runner             *projectrunner.Runner
}

// NewValidationService creates a new ValidationService instance
func NewValidationService(queries *db.Queries, boilerplateService *boilerplates.BoilerplateService, runner *projectrunner.Runner) *ValidationService {
	return &ValidationService{
		queries:            queries,
		boilerplateService: boilerplateService,
		Runner:             runner,
	}
}

// GetProjectIDFromSlug gets the project ID from the project slug
func (v *ValidationService) GetProjectIDFromSlug(ctx context.Context, projectSlug string) (int64, error) {
	// Get project by slug
	project, err := v.queries.GetProjectBySlug(ctx, projectSlug)
	if err != nil {
		return 0, fmt.Errorf("failed to get project by slug: %w", err)
	}
	return project.ID, nil
}

// ValidateProjectAfterFileOperation validates a project after file operations
func (v *ValidationService) ValidateProjectAfterFileOperation(ctx context.Context, projectSlug string) (*projectrunner.ValidationResult, error) {
	// Get project ID from slug
	projectID, err := v.GetProjectIDFromSlug(ctx, projectSlug)
	if err != nil {
		return nil, fmt.Errorf("failed to get project ID: %w", err)
	}

	// Get project details
	project, err := v.queries.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	// Check if project has a boilerplate
	if !project.Boilerplate.Valid || project.Boilerplate.String == "" {
		return nil, fmt.Errorf("project has no boilerplate configured")
	}

	// Get boilerplate configuration
	boilerplateConfig, err := v.boilerplateService.GetBoilerplateConfig(project.Boilerplate.String)
	if err != nil {
		return nil, fmt.Errorf("failed to get boilerplate config: %w", err)
	}

	// Check if validation command is configured
	if boilerplateConfig.ValidateCommand == "" {
		return nil, fmt.Errorf("no validation command configured for boilerplate %s", project.Boilerplate.String)
	}

	// Execute validation
	projectIDStr := fmt.Sprintf("%d", projectID)
	result, err := v.Runner.ValidateProject(ctx, projectIDStr, boilerplateConfig.ValidateCommand)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return result, nil
}

// ShouldTriggerValidation checks if validation should be triggered based on the tool name
func ShouldTriggerValidation(toolName string) bool {
	validationTools := []string{"rewrite_file", "write_file", "edit_file", "delete_file"}
	for _, tool := range validationTools {
		if toolName == tool {
			return true
		}
	}
	return false
}

// CreateValidationMessage creates a message for the AI to fix validation errors
func CreateValidationMessage(validationResult *projectrunner.ValidationResult) string {
	if validationResult.Success {
		return "Validation passed successfully."
	}

	var message strings.Builder
	message.WriteString("Fix these errors in the project:\n\n")
	message.WriteString(validationResult.Output)

	if validationResult.Error != "" {
		message.WriteString("\n\nError: ")
		message.WriteString(validationResult.Error)
	}

	return message.String()
}
