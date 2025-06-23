package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
)

// ClaudeProvider implements AIProviderInterface using Anthropic's Claude API
type ClaudeProvider struct {
	Client anthropic.Client
	Logger *logger.Logger
}

func addHeaderMiddleware(r *http.Request, next option.MiddlewareNext) (res *http.Response, err error) {
	r.Header.Add("anthropic-beta", "fine-grained-tool-streaming-2025-05-14")
	return next(r)
}

// NewClaudeProvider creates a new Claude provider
func NewClaudeProvider(apiKey string, logger *logger.Logger) *ClaudeProvider {
	return &ClaudeProvider{
		Client: anthropic.NewClient(
			option.WithMiddleware(addHeaderMiddleware),
		),
		Logger: logger,
	}
}

// StreamAgentStep streams the assistant's response for a single agent step, executes tool calls if present, and streams tool execution progress.
func (p *ClaudeProvider) StreamAgentStep(
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

	// Convert generic AI types to Claude types
	var claudeMessages []anthropic.MessageParam
	for _, msg := range messages {
		claudeMessages = append(claudeMessages, aiMessageToClaudeMessage(msg))
	}

	var claudeTools []anthropic.ToolUnionParam
	for _, tool := range tools {
		claudeTools = append(claudeTools, aiToolToClaudeTool(tool))
	}

	// Call Claude with streaming
	stream := p.Client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:       anthropic.Model(model),
		Messages:    claudeMessages,
		Tools:       claudeTools,
		MaxTokens:   4096,
		Temperature: param.Opt[float64]{Value: 0.3},
	})
	defer stream.Close()

	// Process streaming response with sophisticated tool call handling
	var contentBuilder strings.Builder
	toolCallsMap := map[string]*AIToolCall{} // toolCallID -> AIToolCall
	var currentToolCallID string             // Track the current tool call being built

	for {
		if !stream.Next() {
			break
		}

		event := stream.Current()

		switch event.Type {
		case "content_block_start":
			// Handle content block start - could be tool use
			contentBlock := event.AsContentBlockStart()
			if contentBlock.ContentBlock.Type == "tool_use" {
				toolUse := contentBlock.ContentBlock.AsToolUse()
				currentToolCallID = toolUse.ID

				// Create new tool call
				toolCall := &AIToolCall{
					ID:   toolUse.ID,
					Type: "function",
					Function: AIFunctionCall{
						Name:      toolUse.Name,
						Arguments: "",
					},
				}
				toolCallsMap[toolUse.ID] = toolCall

				// Notify observer of tool call start
				if observer != nil {
					observer.OnToolCallStart(toolUse.ID, toolUse.Name)
				}

			}

		case "content_block_delta":
			// Handle content block delta - could be tool call arguments
			delta := event.AsContentBlockDelta()
			if delta.Delta.Type == "text_delta" {
				text := delta.Delta.AsTextDelta().Text
				contentBuilder.WriteString(text)
				// Stream content to observer
				if observer != nil {
					observer.OnLLMContent(text)
				}
			} else if delta.Delta.Type == "input_json_delta" {
				// This is tool call argument accumulation
				inputDelta := delta.Delta.AsInputJSONDelta()

				if currentToolCallID != "" {
					// Update the tool call arguments immediately
					if toolCall, exists := toolCallsMap[currentToolCallID]; exists {
						toolCall.Function.Arguments += inputDelta.PartialJSON
						// Send update to observer immediately
						if observer != nil {
							observer.OnToolCallUpdate(currentToolCallID, toolCall.Function.Name, inputDelta.PartialJSON)
						}
					} else {
						p.Logger.Errorf("[StreamChat] Tool call not found in map for ID: %s", currentToolCallID)
					}
				} else {
					p.Logger.Errorf("[StreamChat] No current tool call ID found for delta: %s", inputDelta.PartialJSON)
				}
			} else {
			}

		case "message_delta":
			// Message delta event - could contain tool calls completion
			delta := event.AsMessageDelta()
			if delta.Delta.StopReason == "tool_use" {
			}

		case "message_stop":
			// Message is complete
			p.Logger.Debugf("[StreamChat] Message stop event received")
			break

		default:
			p.Logger.Debugf("[StreamChat] Unhandled event type: %s", event.Type)
		}
	}

	if stream.Err() != nil {
		return nil, nil, nil, stream.Err()
	}

	// Convert tool calls map to slice
	var aiToolCalls []AIToolCall
	for _, toolCall := range toolCallsMap {
		aiToolCalls = append(aiToolCalls, *toolCall)
	}

	// Create the final AI message
	aiMessage := &AIMessage{
		Role:       "assistant",
		Content:    contentBuilder.String(),
		ToolCalls:  aiToolCalls,
		ToolCallID: "",
	}

	// Execute tool calls if any
	var toolCallResults []ToolCallResult
	if len(aiToolCalls) > 0 {
		for _, toolCall := range aiToolCalls {
			// Parse arguments
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				p.Logger.Errorf("[StreamChat] Failed to parse tool call arguments: %v", err)
				toolCallResults = append(toolCallResults, ToolCallResult{
					ToolCall: toolCall,
					Result:   nil,
					Error:    err,
				})
				continue
			}

			// Notify observer of tool execution
			if observer != nil {
				observer.OnToolCallExecute(toolCall.ID, toolCall.Function.Name, args)
			}

			// Execute the tool
			schema, exists := toolSchemas[toolCall.Function.Name]
			if !exists {
				err := fmt.Errorf("tool schema not found for: %s", toolCall.Function.Name)
				p.Logger.Errorf("[StreamChat] %v", err)
				toolCallResults = append(toolCallResults, ToolCallResult{
					ToolCall: toolCall,
					Result:   nil,
					Error:    err,
				})
				if observer != nil {
					observer.OnToolCallResult(toolCall.ID, toolCall.Function.Name, nil, err)
				}
				continue
			}

			// Execute tool and get result
			result, err := schema.Handler(toolCall.Function.Name, args)
			toolCallResults = append(toolCallResults, ToolCallResult{
				ToolCall: toolCall,
				Result:   result,
				Error:    err,
			})

			// Notify observer of tool result
			if observer != nil {
				observer.OnToolCallResult(toolCall.ID, toolCall.Function.Name, result, err)
			}
		}
	}

	return aiMessage, aiToolCalls, toolCallResults, nil
}

// Conversion functions between Claude types and generic AI types
func aiMessageToClaudeMessage(msg AIMessage) anthropic.MessageParam {
	switch msg.Role {
	case "user":
		return anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content))
	case "assistant":
		return anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content))
	case "system":
		// Claude doesn't have a system message type, so we'll prepend it to the first user message
		// This is a limitation of Claude's API
		return anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content))
	case "tool":
		// Claude doesn't have tool messages, so we'll skip them
		// This is a limitation of Claude's API
		return anthropic.NewUserMessage(anthropic.NewTextBlock("Tool result: " + msg.Content))
	default:
		return anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content))
	}
}

func aiToolToClaudeTool(tool AITool) anthropic.ToolUnionParam {
	properties, ok := tool.Function.Parameters["properties"].(map[string]interface{})
	if !ok {
		return anthropic.ToolUnionParamOfTool(anthropic.ToolInputSchemaParam{
			Type:       "object",
			Properties: tool.Function.Parameters,
		}, tool.Function.Name)
	}
	return anthropic.ToolUnionParamOfTool(anthropic.ToolInputSchemaParam{
		Type:       "object",
		Properties: properties,
	}, tool.Function.Name)
}

// Helper functions for max and min
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
