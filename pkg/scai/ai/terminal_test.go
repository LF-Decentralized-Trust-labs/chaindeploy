package ai

import (
	"strings"
	"testing"
)

func TestShouldTriggerValidationForTerminalCommand(t *testing.T) {
	// Terminal command should NOT trigger validation
	result := ShouldTriggerValidation("run_terminal_cmd")
	if result {
		t.Errorf("run_terminal_cmd should not trigger validation, got %v", result)
	}
}

func TestTerminalCommandDescription(t *testing.T) {
	// This test verifies that the terminal command tool is properly configured
	// by checking that it's included in the tool schemas
	service := &AIChatService{}
	tools := service.GetExtendedToolSchemas("/tmp/test")

	var terminalTool *ToolSchema
	for _, tool := range tools {
		if tool.Name == "run_terminal_cmd" {
			terminalTool = &tool
			break
		}
	}

	if terminalTool == nil {
		t.Fatal("run_terminal_cmd tool not found in tool schemas")
	}

	// Check that the description mentions container execution
	description := terminalTool.Description
	if !strings.Contains(description, "Docker container") {
		t.Errorf("Terminal command description should mention Docker container, got: %s", description)
	}

	// Check that required parameters are present
	params := terminalTool.Parameters
	if params == nil {
		t.Fatal("Terminal command parameters not found")
	}

	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("Required parameters not found in terminal command")
	}

	expectedRequired := []string{"command", "is_background"}
	for _, req := range expectedRequired {
		found := false
		for _, r := range required {
			if r == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Required parameter '%s' not found in terminal command", req)
		}
	}
}
