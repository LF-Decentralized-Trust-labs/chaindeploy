package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
)

// ClaudeProvider implements AIProviderInterface using Anthropic's Claude API
type ClaudeProvider struct {
	Client anthropic.Client
	Logger *logger.Logger
}

// NewClaudeProvider creates a new Claude provider
func NewClaudeProvider(apiKey string, logger *logger.Logger) *ClaudeProvider {
	return &ClaudeProvider{
		Client: anthropic.NewClient(),
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

	// Call Claude (non-streaming for now)
	resp, err := p.Client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		Messages:  claudeMessages,
		Tools:     claudeTools,
		MaxTokens: 4096,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	// Extract content and tool calls from response
	var contentBuilder strings.Builder
	var aiToolCalls []AIToolCall

	for _, block := range resp.Content {
		if block.Type == "text" {
			textBlock := block.AsText()
			contentBuilder.WriteString(textBlock.Text)
			// Stream content to observer
			if observer != nil {
				observer.OnLLMContent(textBlock.Text)
			}
		} else if block.Type == "tool_use" {
			toolUse := block.AsToolUse()
			args, _ := json.Marshal(toolUse.Input)
			aiToolCall := AIToolCall{
				ID:   toolUse.ID,
				Type: "function",
				Function: AIFunctionCall{
					Name:      toolUse.Name,
					Arguments: string(args),
				},
			}
			aiToolCalls = append(aiToolCalls, aiToolCall)

			// Notify observer of tool call start
			if observer != nil {
				observer.OnToolCallStart(toolUse.ID, toolUse.Name)
			}
		}
	}

	// If there are tool calls, execute them and stream progress
	var toolCallResults []ToolCallResult
	for _, toolCall := range aiToolCalls {
		toolSchema, ok := toolSchemas[toolCall.Function.Name]
		if !ok {
			result := ToolCallResult{
				ToolCall: toolCall,
				Result:   nil,
				Error:    fmt.Errorf("Unknown tool function: %s", toolCall.Function.Name),
			}
			toolCallResults = append(toolCallResults, result)
			if observer != nil {
				observer.OnToolCallResult(toolCall.ID, toolCall.Function.Name, nil, result.Error)
			}
			continue
		}

		// Parse arguments
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			args = make(map[string]interface{})
		}

		if observer != nil {
			observer.OnToolCallExecute(toolCall.ID, toolCall.Function.Name, args)
		}

		// Execute the tool
		result, err := toolSchema.Handler(toolCall.Function.Name, args)
		toolCallResult := ToolCallResult{
			ToolCall: toolCall,
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

	content := contentBuilder.String()
	if content == "" {
		content = "No content generated"
	}
	assistantMsg := &AIMessage{
		Role:      "assistant",
		Content:   content,
		ToolCalls: aiToolCalls,
	}
	return assistantMsg, aiToolCalls, toolCallResults, nil
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
