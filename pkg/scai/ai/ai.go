package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/scai/projectrunner"
	"github.com/chainlaunch/chainlaunch/pkg/scai/sessionchanges"
	"github.com/sashabaranov/go-openai"
)

// ToolSchema defines a tool with its JSON schema and handler.
type ToolSchema struct {
	Name        string
	Description string
	Parameters  map[string]interface{} // JSON schema
	Handler     func(projectRoot string, args map[string]interface{}) (interface{}, error)
}

// GetDefaultToolSchemas returns all registered tools with their schemas and handlers, scoped to a project root.
func GetDefaultToolSchemas(projectRoot string) []ToolSchema {
	return []ToolSchema{
		{
			Name:        "read_file",
			Description: "Read the contents of a file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Path to the file (relative to project root)"},
				},
				"required": []string{"path"},
			},
			Handler: func(funcName string, args map[string]interface{}) (interface{}, error) {
				path, _ := args["path"].(string)
				absPath := filepath.Join(projectRoot, path)
				data, err := os.ReadFile(absPath)
				if err != nil {
					return nil, err
				}
				return map[string]interface{}{"content": string(data)}, nil
			},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string", "description": "Path to the file (relative to project root)"},
					"content": map[string]interface{}{"type": "string", "description": "Content to write"},
				},
				"required": []string{"path", "content"},
			},
			Handler: func(funcName string, args map[string]interface{}) (interface{}, error) {
				path, _ := args["path"].(string)
				content, _ := args["content"].(string)
				absPath := filepath.Join(projectRoot, path)
				if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
					return nil, err
				}
				// Register the change with the global tracker for backward compatibility
				sessionchanges.RegisterChange(absPath)
				return map[string]interface{}{"result": "file written successfully"}, nil
			},
		},
	}
}

// getToolSchemas returns all registered tools with their schemas and handlers.
func getToolSchemas(projectRoot string) []ToolSchema {
	return GetDefaultToolSchemas(projectRoot)
}

// OpenAIChatService implements ChatServiceInterface using OpenAI's API and function-calling tools.
type OpenAIChatService struct {
	Client      *openai.Client
	Logger      *logger.Logger
	ChatService *ChatService
	Queries     *db.Queries
	ProjectsDir string
}

func NewOpenAIChatService(apiKey string, logger *logger.Logger, chatService *ChatService, queries *db.Queries, projectsDir string) *OpenAIChatService {
	return &OpenAIChatService{
		Client:      openai.NewClient(apiKey),
		Logger:      logger,
		ChatService: chatService,
		Queries:     queries,
		ProjectsDir: projectsDir,
	}
}

const systemPrompt = `
You are an AI coding assistant, powered by ChatGPT 4.1 Mini. You operate in ChainLaunch.

You are pair programming with a USER to solve their coding task. Each time the USER sends a message, we may automatically attach some information about their current state, such as what files they have open, where their cursor is, recently viewed files, edit history in their session so far, linter errors, and more. This information may or may not be relevant to the coding task, it is up for you to decide.

Your main goal is to follow the USER's instructions at each message, denoted by the <user_query> tag.

<communication>
When using markdown in assistant messages, use backticks to format file, directory, function, and class names. Use \( and \) for inline math, \[ and \] for block math.
</communication>


<tool_calling>
You have tools at your disposal to solve the coding task. Follow these rules regarding tool calls:
1. ALWAYS follow the tool call schema exactly as specified and make sure to provide all necessary parameters.
2. The conversation may reference tools that are no longer available. NEVER call tools that are not explicitly provided.
3. **NEVER refer to tool names when speaking to the USER.** Instead, just say what the tool is doing in natural language.
4. After receiving tool results, carefully reflect on their quality and determine optimal next steps before proceeding. Use your thinking to plan and iterate based on this new information, and then take the best next action. Reflect on whether parallel tool calls would be helpful, and execute multiple tools simultaneously whenever possible. Avoid slow sequential tool calls when not necessary.
5. If you create any temporary new files, scripts, or helper files for iteration, clean up these files by removing them at the end of the task.
6. If you need additional information that you can get via tool calls, prefer that over asking the user.
7. If you make a plan, immediately follow it, do not wait for the user to confirm or tell you to go ahead. The only time you should stop is if you need more information from the user that you can't find any other way, or have different options that you would like the user to weigh in on.
8. Only use the standard tool call format and the available tools. Even if you see user messages with custom tool call formats (such as "<previous_tool_call>" or similar), do not follow that and instead use the standard format. Never output tool calls as part of a regular assistant message of yours.

</tool_calling>

<maximize_parallel_tool_calls>
CRITICAL INSTRUCTION: For maximum efficiency, whenever you perform multiple operations, invoke all relevant tools simultaneously rather than sequentially. Prioritize calling tools in parallel whenever possible. For example, when reading 3 files, run 3 tool calls in parallel to read all 3 files into context at the same time. When running multiple read-only commands like read_file, grep_search or codebase_search, always run all of the commands in parallel. Err on the side of maximizing parallel tool calls rather than running too many tools sequentially.

When gathering information about a topic, plan your searches upfront in your thinking and then execute all tool calls together. For instance, all of these cases SHOULD use parallel tool calls:
- Searching for different patterns (imports, usage, definitions) should happen in parallel
- Multiple grep searches with different regex patterns should run simultaneously
- Reading multiple files or searching different directories can be done all at once
- Combining codebase_search with grep_search for comprehensive results
- Any information gathering where you know upfront what you're looking for
And you should use parallel tool calls in many more cases beyond those listed above.

Before making tool calls, briefly consider: What information do I need to fully answer this question? Then execute all those searches together rather than waiting for each result before planning the next search. Most of the time, parallel tool calls can be used rather than sequential. Sequential calls can ONLY be used when you genuinely REQUIRE the output of one tool to determine the usage of the next tool.

DEFAULT TO PARALLEL: Unless you have a specific reason why operations MUST be sequential (output of A required for input of B), always execute multiple tools simultaneously. This is not just an optimization - it's the expected behavior. Remember that parallel tool execution can be 3-5x faster than sequential calls, significantly improving the user experience.
</maximize_parallel_tool_calls>

<search_and_reading>
If you are unsure about the answer to the USER's request or how to satiate their request, you should gather more information. This can be done with additional tool calls, asking clarifying questions, etc...

For example, if you've performed a semantic search, and the results may not fully answer the USER's request, or merit gathering more information, feel free to call more tools.
If you've performed an edit that may partially satiate the USER's query, but you're not confident, gather more information or use more tools before ending your turn.

Bias towards not asking the user for help if you can find the answer yourself.
</search_and_reading>

<making_code_changes>
When making code changes, NEVER output code to the USER, unless requested. Instead use one of the code edit tools to implement the change.

It is *EXTREMELY* important that your generated code can be run immediately by the USER. To ensure this, follow these instructions carefully:
1. Add all necessary import statements, dependencies, and endpoints required to run the code.
2. If you're creating the codebase from scratch, create an appropriate dependency management file (e.g. requirements.txt) with package versions and a helpful README.
3. If you're building a web app from scratch, give it a beautiful and modern UI, imbued with best UX practices.
4. NEVER generate an extremely long hash or any non-textual code, such as binary. These are not helpful to the USER and are very expensive.
5. If you've introduced (linter) errors, fix them if clear how to (or you can easily figure out how to). Do not make uneducated guesses. And DO NOT loop more than 3 times on fixing linter errors on the same file. On the third time, you should stop and ask the user what to do next.
6. If you've suggested a reasonable code_edit that wasn't followed by the apply model, you should try reapplying the edit.
7. You have both the edit_file and search_replace tools at your disposal. Use the search_replace tool for files larger than 2500 lines, otherwise prefer the edit_file tool.
8. **ALWAYS read the current content of a file before making changes to it, unless you are creating a new file.** This ensures your modifications are accurate and contextually appropriate.

</making_code_changes>

Answer the user's request using the relevant tool(s), if they are available. Check that all the required parameters for each tool call are provided or can reasonably be inferred from context. IF there are no relevant tools or there are missing values for required parameters, ask the user to supply these values; otherwise proceed with the tool calls. If you provide a specific value for a parameter (for example provided in quotes), make sure to use that value EXACTLY. DO NOT make up values for or ask about optional parameters. Carefully analyze descriptive terms in the request as they may indicate required parameter values that should be included even if not explicitly quoted.

Do what has been asked; nothing more, nothing less.
NEVER create files unless they're absolutely necessary for achieving your goal.
ALWAYS prefer editing an existing file to creating a new one.
NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.

<summarization>
If you see a section called "<most_important_user_query>", you should treat that query as the one to answer, and ignore previous user queries. If you are asked to summarize the conversation, you MUST NOT use any tools, even if they are available. You MUST answer the "<most_important_user_query>" query.
</summarization>


<tool_calling>
You have tools at your disposal to solve the coding task. Follow these rules regarding tool calls:
1. ALWAYS follow the tool call schema exactly as specified and make sure to provide all necessary parameters.
2. The conversation may reference tools that are no longer available. NEVER call tools that are not explicitly provided.
3. **NEVER refer to tool names when speaking to the user.** For example, instead of saying 'I need to use the edit_file tool to edit your file', just say 'I will edit your file'.
4. Only calls tools when they are necessary. If the user's task is general or you already know the answer, just respond without calling tools.
5. Before calling each tool, first explain to the user why you are calling it.
</tool_calling>

<making_code_changes>
When making code edits, NEVER output code to the user, unless requested. Instead use one of the code edit tools to implement the change.
It is *EXTREMELY* important that your generated code can be run immediately by the user, ERROR-FREE. To ensure this, follow these instructions carefully:
1. Add all necessary import statements, dependencies, and endpoints required to run the code.
2. NEVER generate an extremely long hash, binary, ico, or any non-textual code. These are not helpful to the user and are very expensive.
3. **ALWAYS read the current content of a file before making changes to it, unless you are creating a new file.** This ensures your modifications are accurate and contextually appropriate.
3. Unless you are appending some small easy to apply edit to a file, or creating a new file, you MUST read the contents or section of what you're editing before editing it.
4. If you are copying the UI of a website, you should scrape the website to get the screenshot, styling, and assets. Aim for pixel-perfect cloning. Pay close attention to the every detail of the design: backgrounds, gradients, colors, spacing, etc.
5. If you see linter or runtime errors, fix them if clear how to (or you can easily figure out how to). DO NOT loop more than 3 times on fixing errors on the same file. On the third time, you should stop and ask the user what to do next. You don't have to fix warnings. If the server has a 502 bad gateway error, you can fix this by simply restarting the dev server.
6. If you've suggested a reasonable code_edit that wasn't followed by the apply model, you should use the intelligent_apply argument to reapply the edit.
7. If the runtime errors are preventing the app from running, fix the errors immediately.
</making_code_changes>


This is the ONLY acceptable format for code citations. The format is startLine:endLine:filepath where startLine and endLine are line numbers.

Answer the user's request using the relevant tool(s), if they are available. Check that all the required parameters for each tool call are provided or can reasonably be inferred from context. IF there are no relevant tools or there are missing values for required parameters, ask the user to supply these values; otherwise proceed with the tool calls. If the user provides a specific value for a parameter (for example provided in quotes), make sure to use that value EXACTLY. DO NOT make up values for or ask about optional parameters. Carefully analyze descriptive terms in the request as they may indicate required parameter values that should be included even if not explicitly quoted.

`

// getProjectStructurePrompt generates a system prompt with the project structure and file contents.
func getProjectStructurePrompt(projectRoot string, toolSchemas []ToolSchema, project *db.ChaincodeProject) string {
	ignored := map[string]bool{
		"node_modules": true,
		".git":         true,
		".DS_Store":    true,
	}
	var sb strings.Builder
	sb.WriteString(systemPrompt)

	// Add boilerplate-specific prompt if available
	if project.Boilerplate.Valid && project.Boilerplate.String != "" {
		boilerplatePrompt := projectrunner.GetBoilerplatePrompt(project.Boilerplate.String)
		if boilerplatePrompt != "" {
			sb.WriteString("\n\n")
			sb.WriteString(boilerplatePrompt)
			sb.WriteString("\n")
		}
	}

	sb.WriteString(`

Read and write files as needed to achieve the user's request. Avoid giving answers without using tools unless you need more information from the user.

When replying the user, use the tool "write_file" instead of telling them to edit the file.
`)
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
			data, err := os.ReadFile(path)
			if err == nil {
				sb.WriteString("\n---\nFile: " + rel + "\n" + string(data) + "\n---\n")
			}
		} else {
			sb.WriteString("\n---\nFile: " + rel + " (too large to display)\n---\n")
		}
		return nil
	})
	sb.WriteString(`
These are the available tools, use them as much as possible to solve the user's request:

`)
	for _, tool := range toolSchemas {
		sb.WriteString(fmt.Sprintf("\nTool: %s\n", tool.Name))
		sb.WriteString(fmt.Sprintf("Description: %s\n", tool.Description))
		sb.WriteString("Arguments:\n")
		for name, _ := range tool.Parameters {
			sb.WriteString(fmt.Sprintf("  - %s\n", name))
		}
		sb.WriteString("---\n")
	}
	return sb.String()
}

const maxAgentSteps = 10

// handleToolCall executes a tool call and returns the result as a string.
func (s *OpenAIChatService) handleToolCall(toolCall openai.ToolCall, projectRoot string) string {
	toolSchemas := getToolSchemas(projectRoot)
	var tool ToolSchema
	ok := false
	for _, t := range toolSchemas {
		if t.Name == toolCall.Function.Name {
			tool = t
			ok = true
			break
		}
	}
	if !ok {
		return `{"error": "Unknown tool function: ` + toolCall.Function.Name + `"}`
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return `{"error": "Failed to parse arguments: ` + err.Error() + `"}`
	}
	result, err := tool.Handler(projectRoot, args)
	if err != nil {
		return `{"error": "Tool error: ` + err.Error() + `"}`
	}
	resultJson, _ := json.Marshal(result)
	return string(resultJson)
}

// StreamChat uses a multi-step tool execution loop with OpenAI function-calling.
func (s *OpenAIChatService) StreamChat(
	ctx context.Context,
	project *db.ChaincodeProject,
	conversationID int64,
	messages []Message,
	observer AgentStepObserver,
	maxSteps int,
	sessionTracker *sessionchanges.Tracker,
) error {
	var chatMsgs []openai.ChatCompletionMessage
	projectID := project.ID
	projectSlug := project.Slug
	projectRoot := filepath.Join(s.ProjectsDir, projectSlug)

	// Update the tool schemas to use the session tracker
	toolSchemas := getToolSchemas(projectRoot)
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
	systemPrompt := getProjectStructurePrompt(projectRoot, toolSchemas, project)
	s.Logger.Debugf("[StreamChat] projectID: %s", projectID)
	s.Logger.Debugf("[StreamChat] projectRoot: %s", projectRoot)
	s.Logger.Debugf("[StreamChat] systemPrompt: %s", systemPrompt)
	chatMsgs = append(chatMsgs, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: systemPrompt,
	})
	var lastParentMsgID *int64
	for _, m := range messages {
		role := openai.ChatMessageRoleUser
		if m.Sender == "assistant" {
			role = openai.ChatMessageRoleAssistant
		}
		chatMsgs = append(chatMsgs, openai.ChatCompletionMessage{
			Role:    role,
			Content: m.Content,
		})
	}

	toolSchemasMap := make(map[string]ToolSchema)
	for _, tool := range toolSchemas {
		toolSchemasMap[tool.Name] = tool
	}
	tools := []openai.Tool{}
	for _, tool := range toolSchemas {
		tools = append(tools, openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	if maxSteps <= 0 {
		maxSteps = maxAgentSteps
	}

	for step := 0; step < maxSteps; step++ {
		s.Logger.Debugf("[StreamChat] Agent step: %d", step)
		msg, err := StreamAgentStep(
			ctx,
			s.Client,
			chatMsgs,
			"gpt-4.1-mini",
			tools,
			toolSchemasMap,
			observer,
		)
		if err != nil {
			s.Logger.Debugf("[StreamChat] Error in StreamAgentStep: %v", err)
			return err
		}

		s.Logger.Debugf("[StreamChat] Agent step: %d, assistant message: %s", step, msg.Content)
		if len(msg.ToolCalls) > 0 {
			s.Logger.Debugf("[StreamChat] Tool calls in step: %d, %v", step, msg.ToolCalls)
		}

		chatMsgs = append(chatMsgs, msg)

		// If no tool calls, we're done
		if len(msg.ToolCalls) == 0 {
			s.Logger.Debugf("[StreamChat] No tool calls in step: %d - finishing", step)
			return nil
		}

		// Process all tool calls in this step
		for _, toolCall := range msg.ToolCalls {
			s.Logger.Debugf("[StreamChat] Handling tool call: %s, args: %s", toolCall.Function.Name, toolCall.Function.Arguments)
			resultObj, _ := s.executeAndSerializeToolCall(toolCall, projectRoot)
			resultStr := resultObj.resultStr
			errStr := resultObj.errStr
			argsStr := resultObj.argsStr
			s.Logger.Debugf("[StreamChat] Tool result for: %s, %v", toolCall.Function.Name, resultStr)

			// Add tool result message to DB and get its ID, set parentID to lastParentMsgID
			toolMsg, err := s.ChatService.AddMessage(ctx, conversationID, lastParentMsgID, "tool", resultStr, "")
			if err != nil {
				s.Logger.Debugf("[StreamChat] Failed to persist tool message: %v", err)
				continue
			}
			// Persist tool call
			_, err = s.ChatService.AddToolCall(ctx, toolMsg.ID, toolCall.Function.Name, argsStr, resultStr, errStr)
			if err != nil {
				s.Logger.Debugf("[StreamChat] Failed to persist tool call: %v", err)
			}
			// Add tool result message to chatMsgs for next step
			chatMsgs = append(chatMsgs, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    resultStr,
				ToolCallID: toolCall.ID,
			})
		}
	}

	// If we reach max steps, notify observer and make one final call and stream the response
	if observer != nil {
		observer.OnMaxStepsReached()
	}
	s.Logger.Debugf("[StreamChat] Reached maxSteps, making final call")
	msg, err := StreamAgentStep(
		ctx,
		s.Client,
		chatMsgs,
		"gpt-4o",
		tools,
		toolSchemasMap,
		observer,
	)
	if err != nil {
		s.Logger.Debugf("[StreamChat] Error in final StreamAgentStep: %v", err)
		return err
	}
	chatMsgs = append(chatMsgs, msg)
	s.Logger.Debugf("[StreamChat] Final assistant message: %s", msg.Content)
	if len(msg.ToolCalls) > 0 {
		s.Logger.Debugf("[StreamChat] Final tool calls: %v", msg.ToolCalls)
	}

	return nil
}

// Helper to execute a tool call and serialize args/result/error
func (s *OpenAIChatService) executeAndSerializeToolCall(toolCall openai.ToolCall, projectRoot string) (struct {
	resultStr, argsStr string
	errStr             *string
}, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		errMsg := err.Error()
		return struct {
			resultStr, argsStr string
			errStr             *string
		}{"", toolCall.Function.Arguments, &errMsg}, err
	}
	result, err := getToolSchemas(projectRoot)[0].Handler(projectRoot, args) // Find the correct handler
	var resultStr string
	if result != nil {
		b, _ := json.Marshal(result)
		resultStr = string(b)
	}
	var errStr *string
	if err != nil {
		errMsg := err.Error()
		errStr = &errMsg
	}
	argsStr, _ := json.Marshal(args)
	return struct {
		resultStr, argsStr string
		errStr             *string
	}{resultStr, string(argsStr), errStr}, nil
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

// StreamAgentStep streams the assistant's response for a single agent step, executes tool calls if present, and streams tool execution progress.
func StreamAgentStep(
	ctx context.Context,
	client *openai.Client,
	messages []openai.ChatCompletionMessage,
	model string,
	tools []openai.Tool,
	toolSchemas map[string]ToolSchema,
	observer AgentStepObserver, // new observer argument, can be nil
) (openai.ChatCompletionMessage, error) {
	var contentBuilder strings.Builder
	toolCallsMap := map[string]*openai.ToolCall{} // toolCallID -> ToolCall
	var lastToolCallID string                     // Track the last tool call ID for argument accumulation

	// Delta grouping and delay mechanism
	type pendingUpdate struct {
		toolCallID string
		name       string
		delta      string
	}
	pendingUpdates := make(map[string]*pendingUpdate) // toolCallID -> pendingUpdate
	updateTimer := time.NewTimer(100 * time.Millisecond)
	updateTimer.Stop() // Start stopped

	// Function to send accumulated updates
	sendPendingUpdates := func() {
		if observer == nil {
			return
		}
		for _, update := range pendingUpdates {
			observer.OnToolCallUpdate(update.toolCallID, update.name, update.delta)
		}
		pendingUpdates = make(map[string]*pendingUpdate)
	}

	stream, err := client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Tools:    tools,
		Stream:   true,
	})
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}
	defer stream.Close()

	for {
		select {
		case <-updateTimer.C:
			sendPendingUpdates()
		default:
			// Continue with normal processing
		}

		response, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return openai.ChatCompletionMessage{}, err
		}
		for _, choice := range response.Choices {
			// Stream assistant text
			if choice.Delta.Content != "" {
				contentBuilder.WriteString(choice.Delta.Content)
				if observer != nil {
					observer.OnLLMContent(choice.Delta.Content)
				}
			}

			// Handle tool call deltas robustly
			for _, tc := range choice.Delta.ToolCalls {
				if tc.ID != "" {
					// New tool call or new chunk for an existing one
					lastToolCallID = tc.ID
					if _, ok := toolCallsMap[tc.ID]; !ok {
						toolCallsMap[tc.ID] = &openai.ToolCall{
							ID:       tc.ID,
							Type:     tc.Type,
							Function: openai.FunctionCall{},
						}
						if observer != nil {
							observer.OnToolCallStart(tc.ID, tc.Function.Name)
						}
					}
				}
				// Use lastToolCallID for argument accumulation
				if lastToolCallID != "" {
					toolCall := toolCallsMap[lastToolCallID]
					if tc.Function.Name != "" && toolCall.Function.Name != tc.Function.Name {
						toolCall.Function.Name = tc.Function.Name
					}
					if tc.Function.Arguments != "" {
						toolCall.Function.Arguments += tc.Function.Arguments
						// Group deltas and schedule update with delay
						if observer != nil {
							if pending, exists := pendingUpdates[lastToolCallID]; exists {
								// Accumulate delta
								pending.delta += tc.Function.Arguments
							} else {
								// Create new pending update
								pendingUpdates[lastToolCallID] = &pendingUpdate{
									toolCallID: lastToolCallID,
									name:       toolCall.Function.Name,
									delta:      tc.Function.Arguments,
								}
								// Start/reset timer for this group
								updateTimer.Reset(100 * time.Millisecond)
							}
						}
					}
				}
			}

			// If we get a tool calls finish reason, break out of the stream and reset state
			if choice.FinishReason == openai.FinishReasonToolCalls {
				lastToolCallID = ""
				break
			}
		}
	}

	// Send any remaining pending updates
	sendPendingUpdates()

	// After stream, reconstruct tool calls
	var toolCalls []openai.ToolCall
	for _, tc := range toolCallsMap {
		toolCalls = append(toolCalls, *tc)
	}
	assistantMsg := openai.ChatCompletionMessage{
		Role:      openai.ChatMessageRoleAssistant,
		Content:   contentBuilder.String(),
		ToolCalls: toolCalls,
	}

	// If there are tool calls, execute them and stream progress
	for _, toolCall := range toolCalls {
		toolSchema, ok := toolSchemas[toolCall.Function.Name]
		if !ok {
			if observer != nil {
				observer.OnToolCallResult(toolCall.ID, toolCall.Function.Name, nil,
					fmt.Errorf("Unknown tool function: %s", toolCall.Function.Name))
			}
			continue
		}
		var args map[string]interface{}
		err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		if err != nil {
			if observer != nil {
				observer.OnToolCallResult(toolCall.ID, toolCall.Function.Name, nil, err)
			}
			continue
		}
		if observer != nil {
			observer.OnToolCallExecute(toolCall.ID, toolCall.Function.Name, args)
		}
		result, err := toolSchema.Handler(toolCall.Function.Name, args)
		if observer != nil {
			observer.OnToolCallResult(toolCall.ID, toolCall.Function.Name, result, err)
		}
		if err != nil {
			continue
		}
		// resultJson, _ := json.Marshal(result)
	}

	return assistantMsg, nil
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

// enhancePrompt uses AI to improve the user's prompt for better results
func (s *OpenAIChatService) enhancePrompt(ctx context.Context, userMessage string) (string, error) {
	enhancementPrompt := fmt.Sprintf(`You are a professional prompt engineer specializing in crafting precise, effective prompts.
Your task is to enhance prompts by making them more specific, actionable, and effective.

I want you to improve the user prompt that is wrapped in <original_prompt> tags.

For valid prompts:
- Make instructions explicit and unambiguous
- Add relevant context and constraints
- Remove redundant information
- Maintain the core intent
- Ensure the prompt is self-contained
- Use professional language

For invalid or unclear prompts:
- Respond with clear, professional guidance
- Keep responses concise and actionable
- Maintain a helpful, constructive tone
- Focus on what the user should provide
- Use a standard template for consistency

IMPORTANT: Your response must ONLY contain the enhanced prompt text.
Do not include any explanations, metadata, or wrapper tags.

<original_prompt>
%s
</original_prompt>`, userMessage)

	// Use a simpler model for prompt enhancement to save costs
	resp, err := s.Client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "gpt-4.1-mini",

		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: `You are a senior software principal architect, you should help the user analyse the user query and enrich it with the necessary context and constraints to make it more specific, actionable, and effective. You should also ensure that the prompt is self-contained and uses professional language. Your response should ONLY contain the enhanced prompt text. Do not include any explanations, metadata, or wrapper tags.`,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: enhancementPrompt,
			},
		},
		MaxTokens:   1000,
		Temperature: 0.3, // Lower temperature for more consistent results
	})
	if err != nil {
		s.Logger.Warnf("Failed to enhance prompt: %v, using original", err)
		return userMessage, nil // Fallback to original message
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		s.Logger.Warnf("Empty response from prompt enhancement, using original")
		return userMessage, nil
	}

	enhancedPrompt := strings.TrimSpace(resp.Choices[0].Message.Content)
	s.Logger.Debugf("Enhanced prompt: %s", enhancedPrompt)
	return enhancedPrompt, nil
}

// ChatWithPersistence handles chat with DB persistence for a project.
func (s *OpenAIChatService) ChatWithPersistence(
	ctx context.Context,
	projectID int64,
	userMessage string,
	observer AgentStepObserver,
	maxSteps int,
	conversationID int64,
	sessionTracker *sessionchanges.Tracker,
) error {
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
	_, err = s.ChatService.AddMessage(ctx, conv.ID, nil, "user", userMessage, enhancedMessage)
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

	// 6. Store the assistant's reply in the DB
	_, err = s.ChatService.AddMessage(ctx, conv.ID, nil, "assistant", assistantReply.String(), "")
	return err
}
