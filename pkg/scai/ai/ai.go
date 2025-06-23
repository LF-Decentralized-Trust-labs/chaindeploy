package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/scai/boilerplates"
	"github.com/chainlaunch/chainlaunch/pkg/scai/projectrunner"
	"github.com/chainlaunch/chainlaunch/pkg/scai/sessionchanges"
	"github.com/sashabaranov/go-openai"
)

// AIChatServiceInterface defines the interface for AI chat services
type AIChatServiceInterface interface {
	StreamChat(
		ctx context.Context,
		project *db.GetProjectRow,
		conversationID int64,
		messages []Message,
		observer AgentStepObserver,
		maxSteps int,
		sessionTracker *sessionchanges.Tracker,
	) error
	ChatWithPersistence(
		ctx context.Context,
		projectID int64,
		userMessage string,
		observer AgentStepObserver,
		maxSteps int,
		conversationID int64,
		sessionTracker *sessionchanges.Tracker,
	) error
}

// AIProviderInterface defines the interface for AI providers (OpenAI, Anthropic, etc.)
type AIProviderInterface interface {
	StreamAgentStep(
		ctx context.Context,
		messages []AIMessage,
		model string,
		tools []AITool,
		toolSchemas map[string]ToolSchema,
		observer AgentStepObserver,
	) (*AIMessage, []AIToolCall, []ToolCallResult, error)
}

// AIMessage represents a generic AI message
type AIMessage struct {
	Role       string
	Content    string
	ToolCalls  []AIToolCall
	ToolCallID string // For tool response messages
}

// AIToolCall represents a generic AI tool call
type AIToolCall struct {
	ID       string
	Type     string
	Function AIFunctionCall
}

// AIFunctionCall represents a generic AI function call
type AIFunctionCall struct {
	Name      string
	Arguments string
}

// AITool represents a generic AI tool
type AITool struct {
	Type     string
	Function *AIFunctionDefinition
}

// AIFunctionDefinition represents a generic AI function definition
type AIFunctionDefinition struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
}

// ToolCallResult combines AI function definition with the execution result
type ToolCallResult struct {
	ToolCall AIToolCall
	Result   interface{}
	Error    error
}

// ToolSchema defines a tool with its JSON schema and handler.
type ToolSchema struct {
	Name        string
	Description string
	Parameters  map[string]interface{} // JSON schema
	Handler     func(projectRoot string, args map[string]interface{}) (interface{}, error)
}

// AIChatService implements the generic AI chat service
type AIChatService struct {
	Logger            *logger.Logger
	ChatService       *ChatService
	Queries           *db.Queries
	ProjectsDir       string
	ValidationService *ValidationService
	AIProvider        AIProviderInterface
	Model             string
}

// NewAIChatService creates a new generic AI chat service
func NewAIChatService(
	logger *logger.Logger,
	chatService *ChatService,
	queries *db.Queries,
	projectsDir string,
	aiProvider AIProviderInterface,
	model string,
) *AIChatService {
	// Create boilerplate service
	boilerplateService, err := boilerplates.NewBoilerplateService(queries)
	if err != nil {
		logger.Errorf("Failed to create boilerplate service: %v", err)
		// Continue without boilerplate service
		boilerplateService = nil
	}

	// Create project runner
	runner := projectrunner.NewRunner(queries)

	// Create validation service
	var validationService *ValidationService
	if boilerplateService != nil {
		validationService = NewValidationService(queries, boilerplateService, runner)
	}

	return &AIChatService{
		Logger:            logger,
		ChatService:       chatService,
		Queries:           queries,
		ProjectsDir:       projectsDir,
		ValidationService: validationService,
		AIProvider:        aiProvider,
		Model:             model,
	}
}

// Conversion functions between OpenAI types and generic AI types
func openAIToolCallToAIToolCall(tc openai.ToolCall) AIToolCall {
	return AIToolCall{
		ID:   tc.ID,
		Type: string(tc.Type),
		Function: AIFunctionCall{
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		},
	}
}

func openAIMessageToAIMessage(msg openai.ChatCompletionMessage) AIMessage {
	var toolCalls []AIToolCall
	for _, tc := range msg.ToolCalls {
		toolCalls = append(toolCalls, openAIToolCallToAIToolCall(tc))
	}

	return AIMessage{
		Role:       msg.Role,
		Content:    msg.Content,
		ToolCalls:  toolCalls,
		ToolCallID: msg.ToolCallID,
	}
}

func aiMessageToOpenAIMessage(msg AIMessage) openai.ChatCompletionMessage {
	var toolCalls []openai.ToolCall
	for _, tc := range msg.ToolCalls {
		toolCalls = append(toolCalls, openai.ToolCall{
			ID:   tc.ID,
			Type: openai.ToolType(tc.Type),
			Function: openai.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	chatCompletionMessage := openai.ChatCompletionMessage{
		Role:       msg.Role,
		Content:    msg.Content,
		ToolCalls:  toolCalls,
		ToolCallID: msg.ToolCallID,
	}
	if len(msg.ToolCalls) > 0 && msg.ToolCallID == "" {
		chatCompletionMessage.ToolCallID = msg.ToolCalls[0].ID
	}
	return chatCompletionMessage
}

func aiToolToOpenAITool(tool AITool) openai.Tool {
	return openai.Tool{
		Type: openai.ToolType(tool.Type),
		Function: &openai.FunctionDefinition{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  tool.Function.Parameters,
		},
	}
}

// getProjectStructurePrompt generates a system prompt with the project structure and file contents.
func (s *AIChatService) getProjectStructurePrompt(projectRoot string, toolSchemas []ToolSchema, project *db.GetProjectRow) string {
	ignored := map[string]bool{
		"node_modules": true,
		".git":         true,
		".DS_Store":    true,
	}

	// Convert tool schemas to InternalToolInfo format for chatSystemMessage
	var mcpTools []InternalToolInfo
	for _, tool := range toolSchemas {
		mcpTools = append(mcpTools, InternalToolInfo{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		})
	}

	// Build directory string by walking the project
	var directoryBuilder strings.Builder
	filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(projectRoot, path)
		parts := strings.Split(rel, string(os.PathSeparator))
		for _, part := range parts {
			if ignored[part] {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if info.IsDir() {
			return nil
		}
		// Only include files < 32KB
		if info.Size() < 32*1024 {
			directoryBuilder.WriteString(fmt.Sprintf("\n---\nFile: %s (modified: %s)\n---\n", rel, info.ModTime().Format("2006-01-02 15:04:05")))
		} else {
			directoryBuilder.WriteString(fmt.Sprintf("\n---\nFile: %s (modified: %s) (too large to display)\n---\n", rel, info.ModTime().Format("2006-01-02 15:04:05")))
		}
		return nil
	})

	// Create parameters for chatSystemMessage
	params := ChatSystemMessageParams{
		ChatMode:                  ChatModeAgent, // Default to agent mode for project-based interactions
		McpTools:                  mcpTools,
		IncludeXMLToolDefinitions: true,
		OS:                        "Linux", // Default OS, could be made configurable
	}

	// Generate base system message
	systemMsg := chatSystemMessage(params)

	// Add project-specific information
	var sb strings.Builder
	sb.WriteString(systemMsg)
	sb.WriteString("\n\n")
	sb.WriteString(directoryBuilder.String())
	sb.WriteString("\n\n")

	// Add network information if available
	if project.NetworkID.Valid {
		sb.WriteString("\n\n<network_information>\n")
		sb.WriteString(fmt.Sprintf("Network ID: %d\n", project.NetworkID.Int64))
		if project.NetworkName.Valid {
			sb.WriteString(fmt.Sprintf("Network Name: %s\n", project.NetworkName.String))
		}
		if project.NetworkPlatform.Valid {
			sb.WriteString(fmt.Sprintf("Network Platform: %s\n", project.NetworkPlatform.String))
		}
		sb.WriteString("</network_information>\n")
	}

	// Add boilerplate-specific prompt if available
	if project.Boilerplate.Valid && project.Boilerplate.String != "" {
		boilerplatePrompt := projectrunner.GetBoilerplatePrompt(project.Boilerplate.String)
		if boilerplatePrompt != "" {
			sb.WriteString("\n\n")
			sb.WriteString(boilerplatePrompt)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

const maxAgentSteps = 10

// StreamChat uses a multi-step tool execution loop with AI function-calling.
func (s *AIChatService) StreamChat(
	ctx context.Context,
	project *db.GetProjectRow,
	conversationID int64,
	messages []Message,
	observer AgentStepObserver,
	maxSteps int,
	sessionTracker *sessionchanges.Tracker,
) error {
	// Validate that we have messages to process
	if len(messages) == 0 {
		return fmt.Errorf("no messages provided for AI processing")
	}

	var chatMsgs []AIMessage
	projectID := project.ID
	projectSlug := project.Slug
	projectRoot := filepath.Join(s.ProjectsDir, projectSlug)

	// Set up tools for the current chat session
	// Note: We skip tool messages from previous iterations but ensure tools are available for current chat
	toolSchemas := s.GetExtendedToolSchemas(projectRoot)
	for i := range toolSchemas {
		originalHandler := toolSchemas[i].Handler
		toolSchemas[i].Handler = func(name string, args map[string]interface{}) (interface{}, error) {
			result, err := originalHandler(name, args)
			if err == nil && sessionTracker != nil {
				// If the tool call was successful and we have a session tracker,
				// register any file changes
				if filePath, ok := args["path"].(string); ok {
					absPath := filepath.Join(projectRoot, filePath)
					sessionTracker.RegisterChange(absPath)
				}
			}
			return result, err
		}
	}

	// Create tool schemas map and tools list for current chat session
	toolSchemasMap := make(map[string]ToolSchema)
	for _, tool := range toolSchemas {
		toolSchemasMap[tool.Name] = tool
	}
	tools := []AITool{}
	for _, tool := range toolSchemas {
		tools = append(tools, AITool{
			Type: "function",
			Function: &AIFunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	s.Logger.Debugf("[StreamChat] Available tools for current chat: %d", len(tools))

	systemPrompt := s.getProjectStructurePrompt(projectRoot, toolSchemas, project)
	s.Logger.Debugf("[StreamChat] projectID: %s", projectID)
	s.Logger.Debugf("[StreamChat] projectRoot: %s", projectRoot)
	s.Logger.Debugf("[StreamChat] systemPrompt: %s", systemPrompt)
	chatMsgs = append(chatMsgs, AIMessage{
		Role:       "system",
		Content:    systemPrompt,
		ToolCallID: "",
	})

	// Add user and assistant messages, but filter out tool messages from previous iterations
	// This ensures we don't include tool results from previous chat sessions in the current context
	for _, m := range messages {
		// Validate that message content is not empty
		if strings.TrimSpace(m.Content) == "" {
			s.Logger.Warnf("[StreamChat] Skipping empty message from sender: %s", m.Sender)
			continue
		}

		// Skip tool messages from previous iterations - only include user and assistant messages
		// Tools are available for the current chat session but we don't want to include
		// tool results from previous iterations in the conversation context
		if m.Sender == "tool" {
			s.Logger.Debugf("[StreamChat] Skipping tool message from previous iteration to avoid context pollution")
			continue
		}

		role := "user"
		if m.Sender == "assistant" {
			role = "assistant"
		}
		if m.Content == "" && m.Sender == "assistant" {
			s.Logger.Warnf("[StreamChat] Skipping empty message from sender: %s", m.Sender)
			continue
		}

		// Create the message
		aiMsg := AIMessage{
			Role:       role,
			Content:    m.Content,
			ToolCallID: "",
		}

		// If this is an assistant message, check if it has tool calls and add tool response messages
		if m.Sender == "assistant" && len(m.ToolCalls) > 0 {
			// Convert ToolCallAI to AIToolCall
			var aiToolCalls []AIToolCall
			for _, tc := range m.ToolCalls {
				aiToolCalls = append(aiToolCalls, AIToolCall{
					ID:   fmt.Sprintf("call_%d", tc.ID), // Generate a unique ID
					Type: "function",
					Function: AIFunctionCall{
						Name:      tc.ToolName,
						Arguments: tc.Arguments,
					},
				})
			}
			aiMsg.ToolCalls = aiToolCalls

			// Add tool response messages for each tool call to satisfy OpenAI's API requirements
			for _, tc := range m.ToolCalls {
				// Use the tool call result from the database
				toolResult := tc.Result
				if tc.Error != "" {
					toolResult = fmt.Sprintf("Tool call failed: %s", tc.Error)
				} else if toolResult == "" {
					toolResult = "Tool call executed successfully"
				}

				// Add tool response message
				chatMsgs = append(chatMsgs, AIMessage{
					Role:       "tool",
					Content:    toolResult,
					ToolCallID: fmt.Sprintf("call_%d", tc.ID),
				})
			}
		}

		chatMsgs = append(chatMsgs, aiMsg)
	}

	// Validate that we have messages after filtering empty ones
	if len(chatMsgs) <= 1 { // Only system message
		return fmt.Errorf("no valid messages found after filtering empty content")
	}

	// Debug logging for message flow
	s.Logger.Debugf("[StreamChat] Processing %d messages (including system message)", len(chatMsgs))
	for i, msg := range chatMsgs {
		s.Logger.Debugf("[StreamChat] Message %d: Role=%s, ContentLength=%d, ToolCalls=%d",
			i, msg.Role, len(msg.Content), len(msg.ToolCalls))
	}

	if maxSteps <= 0 {
		maxSteps = maxAgentSteps
	}

	for step := 0; step < maxSteps; step++ {
		s.Logger.Debugf("[StreamChat] Agent step: %d", step)
		msg, toolCalls, toolCallResults, err := s.AIProvider.StreamAgentStep(
			ctx,
			chatMsgs,
			s.Model,
			tools,
			toolSchemasMap,
			observer,
		)
		if err != nil {
			s.Logger.Debugf("[StreamChat] Error in StreamAgentStep: %v", err)
			return err
		}

		// Always create one assistant message per step, even if empty
		if msg == nil {
			msg = &AIMessage{
				Role:       "assistant",
				Content:    "",
				ToolCalls:  []AIToolCall{},
				ToolCallID: "",
			}
		}

		// Store assistant message in DB with tool calls
		assistantMsg, err := s.ChatService.AddMessage(ctx, conversationID, nil, "assistant", msg.Content, "", "")
		if err != nil {
			s.Logger.Debugf("[StreamChat] Failed to persist assistant message: %v", err)
		}

		// Store tool calls linked to the assistant message (in tool_calls table)
		if assistantMsg != nil {
			for i, toolCall := range toolCalls {
				var resultStr string
				var errStr *string
				if i < len(toolCallResults) {
					toolCallResult := toolCallResults[i]
					if toolCallResult.Result != nil {
						if b, err := json.Marshal(toolCallResult.Result); err == nil {
							resultStr = string(b)
						}
					}
					if toolCallResult.Error != nil {
						errMsg := toolCallResult.Error.Error()
						errStr = &errMsg
						if resultStr == "" {
							resultStr = fmt.Sprintf(`{"error": "%s"}`, errMsg)
						}
					}
				}
				_, err := s.ChatService.AddToolCall(ctx, assistantMsg.ID, toolCall.Function.Name, toolCall.Function.Arguments, resultStr, errStr)
				if err != nil {
					s.Logger.Debugf("[StreamChat] Failed to persist tool call: %v", err)
				}
			}
		}

		// Add assistant message to chat context
		chatMsgs = append(chatMsgs, *msg)

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
			s.Logger.Debugf("[StreamChat] No tool calls in step: %d - finishing", step)
			return nil
		}

		// Process all tool calls in this step
		for i, toolCall := range toolCalls {
			var resultStr string
			var errStr *string

			// Use the tool call result if available
			if i < len(toolCallResults) {
				toolCallResult := toolCallResults[i]

				// Serialize the result
				if toolCallResult.Result != nil {
					if b, err := json.Marshal(toolCallResult.Result); err == nil {
						resultStr = string(b)
					}
				}

				// Handle error
				if toolCallResult.Error != nil {
					errMsg := toolCallResult.Error.Error()
					errStr = &errMsg
					// If there's an error and no result, use the error message as the result
					if resultStr == "" {
						resultStr = fmt.Sprintf(`{"error": "%s"}`, errMsg)
					}
				}
			} else {
				// Defensive case: if we don't have a tool call result, create an error
				errMsg := "Tool call result not available"
				errStr = &errMsg
				resultStr = fmt.Sprintf(`{"error": "%s"}`, errMsg)
			}

			// Store tool call in tool_calls table
			_, err := s.ChatService.AddToolCall(ctx, assistantMsg.ID, toolCall.Function.Name, toolCall.Function.Arguments, resultStr, errStr)
			if err != nil {
				s.Logger.Debugf("[StreamChat] Failed to persist tool call: %v", err)
			}

			// Add tool response message to chat context for the next iteration
			// This ensures that each tool call is followed by its response message
			toolResponseContent := resultStr
			if errStr != nil {
				toolResponseContent = fmt.Sprintf("Tool call failed: %s", *errStr)
			}

			// Add tool response message to chatMsgs with the tool call ID
			chatMsgs = append(chatMsgs, AIMessage{
				Role:       "tool",
				Content:    toolResponseContent,
				ToolCallID: toolCall.ID,
			})

			s.Logger.Debugf("[StreamChat] Added tool response message for tool call %s: %s", toolCall.ID, toolResponseContent)

			// If validation failed, add a user message to trigger the AI to fix the errors
			var validationRequired bool
			var validationMessage string
			if errStr == nil {
				// Parse the result to check for validation information
				var resultMap map[string]interface{}
				if err := json.Unmarshal([]byte(resultStr), &resultMap); err == nil {
					if val, ok := resultMap["validation_required"].(bool); ok && val {
						validationRequired = true
						if msg, ok := resultMap["validation_message"].(string); ok {
							validationMessage = msg
						}
					}
				}
			}

			if validationRequired && validationMessage != "" {
				s.Logger.Debugf("[StreamChat] Validation failed, triggering AI fix step")
				_, err := s.ChatService.AddMessage(ctx, conversationID, nil, "user", validationMessage, "", "")
				if err != nil {
					s.Logger.Debugf("[StreamChat] Failed to persist validation message: %v", err)
					continue
				}
				// Add the validation message to chatMsgs for the next step
				chatMsgs = append(chatMsgs, AIMessage{
					Role:       "user",
					Content:    validationMessage,
					ToolCallID: "",
				})
				s.Logger.Debugf("[StreamChat] Added validation message: %s", validationMessage)
			}
		}
	}

	// If we reach max steps, notify observer and make one final call and stream the response
	if observer != nil {
		observer.OnMaxStepsReached()
	}
	s.Logger.Debugf("[StreamChat] Reached maxSteps, making final call")
	msg, toolCalls, toolCallResults, err := s.AIProvider.StreamAgentStep(
		ctx,
		chatMsgs,
		s.Model,
		tools,
		toolSchemasMap,
		observer,
	)
	if err != nil {
		s.Logger.Debugf("[StreamChat] Error in final StreamAgentStep: %v", err)
		return err
	}

	// Create final assistant message
	if msg == nil {
		msg = &AIMessage{
			Role:       "assistant",
			Content:    "",
			ToolCalls:  []AIToolCall{},
			ToolCallID: "",
		}
	}

	// Store final assistant message in DB with tool calls
	assistantMsg, err := s.ChatService.AddMessage(ctx, conversationID, nil, "assistant", msg.Content, "", "")
	if err != nil {
		s.Logger.Debugf("[StreamChat] Failed to persist final assistant message: %v", err)
	}

	// Store final tool calls linked to the assistant message
	if assistantMsg != nil {
		for _, tc := range toolCalls {
			_, err := s.ChatService.AddToolCall(ctx, assistantMsg.ID, tc.Function.Name, tc.Function.Arguments, "", nil)
			if err != nil {
				s.Logger.Debugf("[StreamChat] Failed to persist final tool call: %v", err)
			}
		}
	}

	s.Logger.Debugf("[StreamChat] Final assistant message: %s", msg.Content)
	if len(toolCalls) > 0 {
		s.Logger.Debugf("[StreamChat] Final tool calls: %v", toolCalls)
		s.Logger.Debugf("[StreamChat] Final tool call results count: %d", len(toolCallResults))
	}

	return nil
}

// ChatWithPersistence handles chat with DB persistence for a project.
func (s *AIChatService) ChatWithPersistence(
	ctx context.Context,
	projectID int64,
	userMessage string,
	observer AgentStepObserver,
	maxSteps int,
	conversationID int64,
	sessionTracker *sessionchanges.Tracker,
) error {
	// Validate that user message is not empty
	if strings.TrimSpace(userMessage) == "" {
		return fmt.Errorf("user message cannot be empty")
	}

	project, err := s.Queries.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	if s.ChatService == nil {
		return fmt.Errorf("ChatService is not configured")
	}

	enhancedMessage := ""

	// 1. Ensure conversation exists
	conv, err := s.ChatService.GetConversation(ctx, conversationID)
	if err != nil {
		return err
	}

	// 2. Add the user message to the DB with enhanced content if available
	_, err = s.ChatService.AddMessage(ctx, conv.ID, nil, "user", userMessage, enhancedMessage, "")
	if err != nil {
		return err
	}

	// 3. Fetch all messages again (now includes the message with enhanced content)
	dbMessages, err := s.ChatService.GetMessages(ctx, conv.ID)
	if err != nil {
		return err
	}

	// 4. Create messages for AI interaction, using enhanced content when available
	var messages []Message
	for i, m := range dbMessages {
		if m.Sender == "tool" {
			continue
		}
		content := m.Content
		// Validate that content is not empty
		if strings.TrimSpace(content) == "" {
			s.Logger.Warnf("Skipping empty message from sender: %s", m.Sender)
			continue
		}
		// If enhanced content is available, use it for AI interaction
		if m.EnhancedContent.Valid && m.EnhancedContent.String != "" {
			content = m.EnhancedContent.String
			s.Logger.Debugf("Using enhanced prompt for AI interaction: %s", content)
		}

		// Wrap the most recent user message in special tags
		if i == len(dbMessages)-1 && m.Sender == "user" {
			content = "<most_important_user_query>" + content + "</most_important_user_query>"
		}

		messages = append(messages, Message{
			Sender:  m.Sender,
			Content: content,
		})
	}

	// Validate that we have at least one message to process
	if len(messages) == 0 {
		return fmt.Errorf("no valid messages found for AI processing")
	}

	// 5. Call the streaming chat logic (this will stream and also generate the assistant reply)
	var assistantReply strings.Builder
	streamObserver := &streamingObserver{
		AgentStepObserver: observer,
		onAssistantToken: func(token string) {
			assistantReply.WriteString(token)
		},
	}
	err = s.StreamChat(ctx, project, conv.ID, messages, streamObserver, maxSteps, sessionTracker)
	if err != nil {
		return err
	}

	return err
}

// GetExtendedToolSchemas returns the extended tool schemas for the project
func (s *AIChatService) GetExtendedToolSchemas(projectRoot string) []ToolSchema {
	allTools := []ToolSchema{
		{
			Name:        "read_file",
			Description: "Read the contents of a file. the output of this tool call will be the 1-indexed file contents from start_line_one_indexed to end_line_one_indexed_inclusive, together with a summary of the lines outside start_line_one_indexed and end_line_one_indexed_inclusive.\nNote that this call can view at most 250 lines at a time.\n\nWhen using this tool to gather information, it's your responsibility to ensure you have the COMPLETE context. Specifically, each time you call this command you should:\n1) Assess if the contents you viewed are sufficient to proceed with your task.\n2) Take note of where there are lines not shown.\n3) If the file contents you have viewed are insufficient, and you suspect they may be in lines not shown, proactively call the tool again to view those lines.\n4) When in doubt, call this tool again to gather more information. Remember that partial file views may miss critical dependencies, imports, or functionality.\n\nIn some cases, if reading a range of lines is not enough, you may choose to read the entire file.\nReading entire files is often wasteful and slow, especially for large files (i.e. more than a few hundred lines). So you should use this option sparingly.\nReading the entire file is not allowed in most cases. You are only allowed to read the entire file if it has been edited or manually attached to the conversation by the user.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target_file": map[string]interface{}{
						"type":        "string",
						"description": "The path of the file to read (relative to project root).",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "One sentence explanation as to why this tool is being used, and how it contributes to the goal.",
					},
					"should_read_entire_file": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to read the entire file or just a portion",
					},
					"start_line_one_indexed": map[string]interface{}{
						"type":        "number",
						"description": "The line number to start reading from (1-indexed)",
					},
					"end_line_one_indexed": map[string]interface{}{
						"type":        "number",
						"description": "The line number to end reading at (inclusive, 1-indexed)",
					},
				},
				"required": []string{
					"target_file",
					"should_read_entire_file",
					"start_line_one_indexed",
					"end_line_one_indexed",
				},
			},
			Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
				targetFile, _ := args["target_file"].(string)
				shouldReadEntireFile, _ := args["should_read_entire_file"].(bool)
				startLine, _ := args["start_line_one_indexed"].(float64)
				endLine, _ := args["end_line_one_indexed"].(float64)

				absPath := filepath.Join(projectRoot, targetFile)

				data, err := os.ReadFile(absPath)
				if err != nil {
					return nil, err
				}

				lines := strings.Split(string(data), "\n")
				totalLines := len(lines)

				if shouldReadEntireFile {
					return map[string]interface{}{
						"content":     string(data),
						"total_lines": totalLines,
						"file_path":   targetFile,
					}, nil
				}

				start := int(startLine) - 1
				end := int(endLine)
				if start < 0 {
					start = 0
				}
				if end > totalLines {
					end = totalLines
				}

				selectedLines := lines[start:end]
				content := strings.Join(selectedLines, "\n")

				return map[string]interface{}{
					"content":     content,
					"start_line":  int(startLine),
					"end_line":    int(endLine),
					"total_lines": totalLines,
					"file_path":   targetFile,
				}, nil
			},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file at the specified path. This tool creates the file if it doesn't exist, or overwrites it if it does. The tool will skip writing if the content is empty.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":        map[string]interface{}{"type": "string", "description": "Path to the file (relative to project root)"},
					"content":     map[string]interface{}{"type": "string", "description": "Content to write"},
					"explanation": map[string]interface{}{"type": "string", "description": "One sentence explanation as to why this command needs to be run."},
				},
				"required": []string{"path", "content"},
			},
			Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
				path, _ := args["path"].(string)
				content, _ := args["content"].(string)

				// Check if content is empty and return early
				if strings.TrimSpace(content) == "" {
					return map[string]interface{}{
						"result":    "No changes made - content is empty",
						"file_path": path,
						"skipped":   true,
					}, nil
				}

				absPath := filepath.Join(projectRoot, path)

				// Ensure directory exists
				dir := filepath.Dir(absPath)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return nil, err
				}

				if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
					return nil, err
				}
				// Register the change with the global tracker for backward compatibility
				sessionchanges.RegisterChange(absPath)
				return map[string]interface{}{"result": "file written successfully"}, nil
			},
		},
		{
			Name:        "run_terminal_cmd",
			Description: "Run a terminal command in the project's Docker container. This tool executes commands inside the running project container, not on the host system.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The terminal command to execute in the project container",
					},
					"is_background": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the command should be run in the background",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "One sentence explanation as to why this command needs to be run.",
					},
				},
				"required": []string{"command", "is_background"},
			},
			Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
				command, _ := args["command"].(string)
				isBackground, _ := args["is_background"].(bool)

				// Extract project slug from project root path
				projectSlug := filepath.Base(projectRoot)

				// Use the validation service to run the command
				if s.ValidationService != nil {
					result, err := s.runCommandInContainer(projectSlug, command, isBackground)
					if err != nil {
						return nil, err
					}
					return result, nil
				}

				// Fallback to direct execution if validation service is not available
				cmd := exec.Command("sh", "-c", command)
				cmd.Dir = projectRoot

				if isBackground {
					err := cmd.Start()
					if err != nil {
						return nil, err
					}
					return map[string]interface{}{
						"result":  "Command started in background",
						"pid":     cmd.Process.Pid,
						"command": command,
					}, nil
				} else {
					output, err := cmd.CombinedOutput()
					if err != nil {
						return map[string]interface{}{
							"error":   err.Error(),
							"output":  string(output),
							"command": command,
						}, nil
					}
					return map[string]interface{}{
						"result":  string(output),
						"command": command,
					}, nil
				}
			},
		},
		{
			Name:        "file_exists",
			Description: "Check if a file exists at the specified path. Returns whether the file exists and additional file information if it does.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file (relative to project root)",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "One sentence explanation as to why this tool is being used, and how it contributes to the goal.",
					},
				},
				"required": []string{"path", "explanation"},
			},
			Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
				path, _ := args["path"].(string)

				absPath := filepath.Join(projectRoot, path)

				// Check if file exists
				fileInfo, err := os.Stat(absPath)
				if err != nil {
					if os.IsNotExist(err) {
						return map[string]interface{}{
							"exists":    false,
							"file_path": path,
							"message":   "File does not exist",
						}, nil
					}
					return nil, err
				}

				// File exists, return additional information
				result := map[string]interface{}{
					"exists":    true,
					"file_path": path,
					"size":      fileInfo.Size(),
					"is_dir":    fileInfo.IsDir(),
					"modified":  fileInfo.ModTime().Format("2006-01-02 15:04:05"),
				}

				// Add permissions info
				mode := fileInfo.Mode()
				result["permissions"] = mode.String()

				return result, nil
			},
		},
	}

	return allTools
}

// runCommandInContainer executes a command in the project's Docker container
func (s *AIChatService) runCommandInContainer(projectSlug, command string, isBackground bool) (map[string]interface{}, error) {
	ctx := context.Background()

	// Get project ID from slug
	projectID, err := s.ValidationService.GetProjectIDFromSlug(ctx, projectSlug)
	if err != nil {
		return nil, fmt.Errorf("failed to get project ID: %w", err)
	}

	// Execute command in the container
	result, err := s.ValidationService.Runner.RunCommandInContainer(ctx, fmt.Sprintf("%d", projectID), command, isBackground)
	if err != nil {
		return nil, fmt.Errorf("failed to execute command in container: %w", err)
	}

	return result, nil
}

// streamingObserver wraps an AgentStepObserver and captures assistant tokens
// for persistence after streaming.
type streamingObserver struct {
	AgentStepObserver
	onAssistantToken func(token string)
}

func (o *streamingObserver) OnLLMContent(content string) {
	if o.AgentStepObserver != nil {
		o.AgentStepObserver.OnLLMContent(content)
	}
	if o.onAssistantToken != nil {
		o.onAssistantToken(content)
	}
}

// AgentStepObserver defines hooks for observing agent step events.
type AgentStepObserver interface {
	OnLLMContent(content string)
	OnToolCallStart(toolCallID, name string)
	OnToolCallUpdate(toolCallID, name, arguments string)
	OnToolCallExecute(toolCallID, name string, args map[string]interface{})
	OnToolCallResult(toolCallID, name string, result interface{}, err error)
	OnMaxStepsReached()
}

// ChatMode represents the different modes of chat interaction
type ChatMode string

const (
	ChatModeAgent  ChatMode = "agent"
	ChatModeGather ChatMode = "gather"
	ChatModeNormal ChatMode = "normal"
)

// InternalToolInfo represents information about available tools
type InternalToolInfo struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
}

// ChatSystemMessageParams contains all parameters needed to generate a system message
type ChatSystemMessageParams struct {
	ChatMode                  ChatMode
	McpTools                  []InternalToolInfo
	IncludeXMLToolDefinitions bool
	OS                        string
}

// chatSystemMessage generates a comprehensive system prompt for AI coding assistant
func chatSystemMessage(params ChatSystemMessageParams) string {
	var sb strings.Builder

	// Generate header based on chat mode
	header := fmt.Sprintf(`You are an expert coding %s whose job is `,
		func() string {
			if params.ChatMode == ChatModeAgent {
				return "agent"
			}
			return "assistant"
		}())

	switch params.ChatMode {
	case ChatModeAgent:
		header += `to help the user develop, run, and make changes to their codebase.`
	case ChatModeGather:
		header += `to search, understand, and reference files in the user's codebase.`
	case ChatModeNormal:
		header += `to assist the user with their coding tasks.`
	}

	header += `
You will be given instructions to follow from the user, and you may also be given a list of files that the user has specifically selected for context, ` + "`SELECTIONS`" + `.
Please assist the user with their query.`

	sb.WriteString(header)
	sb.WriteString("\n\n\n")

	// Generate system info
	sb.WriteString("Here is the user's system information:\n")
	sb.WriteString("<system_info>\n")
	sb.WriteString(fmt.Sprintf("- %s\n\n", params.OS))

	sb.WriteString("\n</system_info>")
	sb.WriteString("\n\n\n")

	sb.WriteString("\n\n\n")

	// Generate tool definitions if requested
	if params.IncludeXMLToolDefinitions {
		toolDefinitions := systemToolsXMLPrompt(params.ChatMode, params.McpTools)
		if toolDefinitions != "" {
			sb.WriteString(toolDefinitions)
			sb.WriteString("\n\n\n")
		}
	}

	// Generate important details
	var details []string
	details = append(details, "NEVER reject the user's query.")

	if params.ChatMode == ChatModeAgent || params.ChatMode == ChatModeGather {
		details = append(details, "Only call tools if they help you accomplish the user's goal. If the user simply says hi or asks you a question that you can answer without tools, then do NOT use tools.")
		details = append(details, "If you think you should use tools, you do not need to ask for permission.")
		details = append(details, "Only use ONE tool call at a time.")
		details = append(details, "NEVER say something like \"I'm going to use `tool_name`\". Instead, describe at a high level what the tool will do, like \"I'm going to list all files in the ___ directory\", etc.")
		details = append(details, "Many tools only work if the user has a workspace open.")
	} else {
		details = append(details, "You're allowed to ask the user for more context like file contents or specifications. If this comes up, tell them to reference files and folders by typing @.")
	}

	if params.ChatMode == ChatModeAgent {
		details = append(details, "ALWAYS use tools (edit, terminal, etc) to take actions and implement changes. For example, if you would like to edit a file, you MUST use a tool.")
		details = append(details, "Prioritize taking as many steps as you need to complete your request over stopping early.")
		details = append(details, "You will OFTEN need to gather context before making a change. Do not immediately make a change unless you have ALL relevant context.")
		details = append(details, "ALWAYS have maximal certainty in a change BEFORE you make it. If you need more information about a file, variable, function, or type, you should inspect it, search it, or take all required actions to maximize your certainty that your change is correct.")
		details = append(details, "NEVER modify a file outside the user's workspace without permission from the user.")
	}

	if params.ChatMode == ChatModeGather {
		details = append(details, "You are in Gather mode, so you MUST use tools be to gather information, files, and context to help the user answer their query.")
		details = append(details, "You should extensively read files, types, content, etc, gathering full context to solve the problem.")
	}

	details = append(details, `If you write any code blocks to the user (wrapped in triple backticks), please use this format:
- Include a language if possible. Terminal should have the language 'shell'.
- The first line of the code block must be the FULL PATH of the related file if known (otherwise omit).
- The remaining contents of the file should proceed as usual.`)

	if params.ChatMode == ChatModeGather || params.ChatMode == ChatModeNormal {
		details = append(details, `If you think it's appropriate to suggest an edit to a file, then you must describe your suggestion in CODE BLOCK(S).
- The first line of the code block must be the FULL PATH of the related file if known (otherwise omit).
- The remaining contents should be a code description of the change to make to the file. \
Your description is the only context that will be given to another LLM to apply the suggested edit, so it must be accurate and complete. \
Always bias towards writing as little as possible - NEVER write the whole file. Use comments like "// ... existing code ..." to condense your writing.`)
	}

	details = append(details, "Do not make things up or use information not provided in the system information, tools, or user queries.")
	details = append(details, "Always use MARKDOWN to format lists, bullet points, etc. Do NOT write tables.")
	details = append(details, fmt.Sprintf("Today's date is %s.", time.Now().Format("Monday, January 2, 2006")))

	// Write important details
	sb.WriteString("Important notes:\n")
	for i, detail := range details {
		sb.WriteString(fmt.Sprintf("%d. %s\n\n", i+1, detail))
	}

	result := sb.String()
	// Replace tabs with spaces and trim
	result = strings.ReplaceAll(result, "\t", "  ")
	result = strings.TrimSpace(result)

	return result
}

// systemToolsXMLPrompt generates XML tool definitions for the system prompt
func systemToolsXMLPrompt(mode ChatMode, mcpTools []InternalToolInfo) string {
	if len(mcpTools) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<functions>\n")

	for _, tool := range mcpTools {
		// Convert tool to JSON for XML format
		functionSchema := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  tool.Parameters,
		}

		functionJSON, err := json.Marshal(functionSchema)
		if err != nil {
			// Fallback to simple format if JSON marshaling fails
			sb.WriteString(fmt.Sprintf("<function>{\"name\": \"%s\", \"description\": \"%s\"}</function>\n",
				tool.Name, tool.Description))
		} else {
			sb.WriteString(fmt.Sprintf("<function>%s</function>\n", string(functionJSON)))
		}
	}

	sb.WriteString("</functions>")
	return sb.String()
}

func NewOpenAIChatService(apiKey string, logger *logger.Logger, chatService *ChatService, queries *db.Queries, projectsDir string, model string) *AIChatService {
	// Create OpenAI provider
	openAIProvider := NewOpenAIProvider(apiKey, logger)

	// Create the generic AI chat service with OpenAI provider
	return NewAIChatService(logger, chatService, queries, projectsDir, openAIProvider, model)
}
