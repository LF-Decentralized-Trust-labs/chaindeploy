package ai

import (
	"testing"

	"github.com/chainlaunch/chainlaunch/pkg/scai/projectrunner"
)

func TestShouldTriggerValidation(t *testing.T) {
	tests := []struct {
		toolName string
		expected bool
	}{
		{"write_file", true},
		{"edit_file", true},
		{"delete_file", true},
		{"read_file", false},
		{"list_dir", false},
		{"grep_search", false},
	}

	for _, test := range tests {
		result := ShouldTriggerValidation(test.toolName)
		if result != test.expected {
			t.Errorf("ShouldTriggerValidation(%s) = %v, expected %v", test.toolName, result, test.expected)
		}
	}
}

func TestCreateValidationMessage(t *testing.T) {
	// Test successful validation
	successResult := &projectrunner.ValidationResult{
		Success:  true,
		Output:   "All good",
		ExitCode: 0,
	}

	message := CreateValidationMessage(successResult)
	expected := "Validation passed successfully."
	if message != expected {
		t.Errorf("CreateValidationMessage(success) = %s, expected %s", message, expected)
	}

	// Test failed validation
	failedResult := &projectrunner.ValidationResult{
		Success:  false,
		Output:   "main.go:10: undefined: x\nmain.go:15: syntax error",
		Error:    "Validation command failed with exit code 1",
		ExitCode: 1,
	}

	message = CreateValidationMessage(failedResult)
	expected = "Fix these errors in the project:\n\nmain.go:10: undefined: x\nmain.go:15: syntax error\n\nError: Validation command failed with exit code 1"
	if message != expected {
		t.Errorf("CreateValidationMessage(failed) = %s, expected %s", message, expected)
	}
}
