package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements AIProviderInterface using OpenAI's API
type OpenAIProvider struct {
	Client *openai.Client
	Logger *logger.Logger
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey string, logger *logger.Logger) *OpenAIProvider {
	return &OpenAIProvider{
		Client: openai.NewClient(apiKey),
		Logger: logger,
	}
}

// StreamAgentStep streams the assistant's response for a single agent step, executes tool calls if present, and streams tool execution progress.
func (p *OpenAIProvider) StreamAgentStep(
	ctx context.Context,
	messages []AIMessage,
	model string,
	tools []AITool,
	toolSchemas map[string]ToolSchema,
	observer AgentStepObserver,
) (*AIMessage, []AIToolCall, []ToolCallResult, error) {
	// Validate that we have messages to process
	if len(messages) == 0 {
		return nil, nil, nil, fmt.Errorf("no messages provided for AI processing")
	}

	// Validate that all messages have content
	for i, msg := range messages {
		if msg.Content == "" {
			return nil, nil, nil, fmt.Errorf("message at index %d has empty content", i)
		}
	}

	// Convert generic AI types to OpenAI types
	var openAIMessages []openai.ChatCompletionMessage
	for _, msg := range messages {
		openAIMessages = append(openAIMessages, aiMessageToOpenAIMessage(msg))
	}

	var openAITools []openai.Tool
	for _, tool := range tools {
		openAITools = append(openAITools, aiToolToOpenAITool(tool))
	}

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

	stream, err := p.Client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    model,
		Messages: openAIMessages,
		Tools:    openAITools,
		Stream:   true,
	})
	if err != nil {
		return nil, nil, nil, err
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
			return nil, nil, nil, err
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

	// Convert OpenAI tool calls to generic AI tool calls
	var aiToolCalls []AIToolCall
	for _, tc := range toolCalls {
		aiToolCalls = append(aiToolCalls, openAIToolCallToAIToolCall(tc))
	}

	// If there are tool calls, execute them and stream progress
	var toolCallResults []ToolCallResult
	for _, toolCall := range toolCalls {
		toolSchema, ok := toolSchemas[toolCall.Function.Name]
		if !ok {
			result := ToolCallResult{
				ToolCall: openAIToolCallToAIToolCall(toolCall),
				Result:   nil,
				Error:    fmt.Errorf("Unknown tool function: %s", toolCall.Function.Name),
			}
			toolCallResults = append(toolCallResults, result)
			if observer != nil {
				observer.OnToolCallResult(toolCall.ID, toolCall.Function.Name, nil, result.Error)
			}
			continue
		}
		var args map[string]interface{}
		err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		if err != nil {
			result := ToolCallResult{
				ToolCall: openAIToolCallToAIToolCall(toolCall),
				Result:   nil,
				Error:    err,
			}
			toolCallResults = append(toolCallResults, result)
			if observer != nil {
				observer.OnToolCallResult(toolCall.ID, toolCall.Function.Name, nil, err)
			}
			continue
		}
		if observer != nil {
			observer.OnToolCallExecute(toolCall.ID, toolCall.Function.Name, args)
		}
		result, err := toolSchema.Handler(toolCall.Function.Name, args)
		toolCallResult := ToolCallResult{
			ToolCall: openAIToolCallToAIToolCall(toolCall),
			Result:   result,
			Error:    err,
		}
		toolCallResults = append(toolCallResults, toolCallResult)
		if observer != nil {
			observer.OnToolCallResult(toolCall.ID, toolCall.Function.Name, result, err)
		}
		if err != nil {
			p.Logger.Debugf("[StreamChat] Error in tool call: %v", err)
			continue
		}
	}

	// Always create one assistant message with content and tool calls
	content := contentBuilder.String()
	if content == "" {
		content = "No content generated"
	}

	// Create the single assistant message with all tool calls linked to it
	assistantMsg := &AIMessage{
		Role:      "assistant",
		Content:   content,
		ToolCalls: aiToolCalls, // All tool calls are linked to this single assistant message
	}

	return assistantMsg, aiToolCalls, toolCallResults, nil
}
