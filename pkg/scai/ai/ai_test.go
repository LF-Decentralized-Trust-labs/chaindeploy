package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/scai/sessionchanges"
)

// MockAgentStepObserver implements AgentStepObserver for testing
type MockAgentStepObserver struct {
	LLMContent      []string
	ToolCalls       []ToolCallEvent
	MaxStepsReached bool
}

type ToolCallEvent struct {
	ID     string
	Name   string
	Args   map[string]interface{}
	Result interface{}
	Error  error
}

func (m *MockAgentStepObserver) OnLLMContent(content string) {
	m.LLMContent = append(m.LLMContent, content)
}

func (m *MockAgentStepObserver) OnToolCallStart(toolCallID, name string) {
	m.ToolCalls = append(m.ToolCalls, ToolCallEvent{ID: toolCallID, Name: name})
}

func (m *MockAgentStepObserver) OnToolCallUpdate(toolCallID, name, arguments string) {
	// Find the tool call and update it
	for i := range m.ToolCalls {
		if m.ToolCalls[i].ID == toolCallID {
			m.ToolCalls[i].Name = name
			break
		}
	}
}

func (m *MockAgentStepObserver) OnToolCallExecute(toolCallID, name string, args map[string]interface{}) {
	// Find the tool call and update it
	for i := range m.ToolCalls {
		if m.ToolCalls[i].ID == toolCallID {
			m.ToolCalls[i].Args = args
			break
		}
	}
}

func (m *MockAgentStepObserver) OnToolCallResult(toolCallID, name string, result interface{}, err error) {
	// Find the tool call and update it
	for i := range m.ToolCalls {
		if m.ToolCalls[i].ID == toolCallID {
			m.ToolCalls[i].Result = result
			m.ToolCalls[i].Error = err
			break
		}
	}
}

func (m *MockAgentStepObserver) OnMaxStepsReached() {
	m.MaxStepsReached = true
}

// TestWithExistingProject tests the AI service with an existing project
func TestWithExistingProject(t *testing.T) {
	// Skip if no OpenAI API key is available
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	// Use the existing project directory
	projectDir := "/Users/davidviejo/projects/kfs/beast-mode/lfdt-chainlaunch/projects-data-node1"

	// Check if the project directory exists
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		t.Skipf("Project directory %s does not exist", projectDir)
	}
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer dbConn.Close()
	queries := db.New(dbConn)
	if err := db.RunMigrations(dbConn); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	// Create mock dependencies
	logger := logger.NewDefault()
	chatService := NewChatService(queries)

	// Create the AI service
	aiService := NewOpenAIChatService(apiKey, logger, chatService, queries, projectDir)

	// Create a mock project
	project := &db.GetProjectRow{
		ID:          1,
		Slug:        "testgo-4cf84",
		Boilerplate: sql.NullString{String: "chaincode-fabric-go", Valid: true},
	}

	// Create a mock conversation
	conversationID := int64(1)

	// Single test case: Add a function
	// userMessage := "Add a new function called 'GetAssetCount' to the contract asset contract in go that returns the total number of assets in the ledger"
	// userMessage := "Modify the function InitLedger to create 10 new assets instead of 5"
	userMessage := "Instead of creating in the smart contract asset 20 assets, create only 5 assets"
	// userMessage := "Fix the contract.go file"

	// Create observer to capture the model's behavior
	observer := &MockAgentStepObserver{}

	// Create messages
	messages := []Message{
		{
			Sender:  "user",
			Content: userMessage,
		},
	}

	// Create session tracker
	sessionTracker := sessionchanges.NewTracker()

	// Call StreamChat
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = aiService.StreamChat(ctx, project, conversationID, messages, observer, 5, sessionTracker)

	// Output everything to stdout for analysis
	fmt.Println("=== AI RESPONSE ANALYSIS ===")
	fmt.Printf("User Message: %s\n", userMessage)
	fmt.Printf("LLM Content: %s\n", strings.Join(observer.LLMContent, ""))
	fmt.Printf("Tool calls: %d\n", len(observer.ToolCalls))

	for i, toolCall := range observer.ToolCalls {
		fmt.Printf("\nTool call %d: %s\n", i+1, toolCall.Name)
		if toolCall.Args != nil {
			argsJSON, _ := json.MarshalIndent(toolCall.Args, "", "  ")
			fmt.Printf("Args:\n%s\n", string(argsJSON))
		}
		if toolCall.Result != nil {
			resultJSON, _ := json.MarshalIndent(toolCall.Result, "", "  ")
			fmt.Printf("Result:\n%s\n", string(resultJSON))
		}
		if toolCall.Error != nil {
			fmt.Printf("Error: %v\n", toolCall.Error)
		}
	}

	if err != nil {
		fmt.Printf("StreamChat error: %v\n", err)
	}

	// Check if edit_file tool was used and analyze its usage
	var editFileCall *ToolCallEvent
	for i := range observer.ToolCalls {
		if observer.ToolCalls[i].Name == "edit_file" {
			editFileCall = &observer.ToolCalls[i]
			break
		}
	}

	if editFileCall != nil {
		fmt.Println("\n=== EDIT_FILE TOOL ANALYSIS ===")
		if editFileCall.Args != nil {
			if instructions, ok := editFileCall.Args["instructions"].(string); ok {
				fmt.Printf("Instructions: %s\n", instructions)
			}
			if codeEdit, ok := editFileCall.Args["code_edit"].(string); ok {
				fmt.Printf("Code Edit Length: %d characters\n", len(codeEdit))
				fmt.Printf("Code Edit:\n%s\n", codeEdit)
			}
		}
	} else {
		fmt.Println("\n=== NO EDIT_FILE TOOL USED ===")
	}

	fmt.Println("\n=== END ANALYSIS ===")
}
