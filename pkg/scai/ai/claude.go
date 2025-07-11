package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
)

// ClaudeAdapter adapts Anthropic's Claude API to our AIClient interface
type ClaudeAdapter struct {
	client anthropic.Client
}

// NewClaudeAdapter creates a new Claude adapter
func NewClaudeAdapter(apiKey string) *ClaudeAdapter {
	return &ClaudeAdapter{
		client: anthropic.NewClient(),
	}
}

// CreateChatCompletion implements AIClient interface for Claude
func (a *ClaudeAdapter) CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	// Convert our messages to Claude's format
	claudeMessages := make([]anthropic.MessageParam, len(req.Messages))
	for i, m := range req.Messages {
		claudeMessages[i] = anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content))
	}

	// Convert our tools to Claude's format
	claudeTools := make([]anthropic.ToolUnionParam, len(req.Tools))
	for i, t := range req.Tools {
		claudeTools[i] = anthropic.ToolUnionParamOfTool(anthropic.ToolInputSchemaParam{
			Type:       "object",
			Properties: t.Function.Parameters,
		}, t.Function.Name)
	}

	// Call Claude
	resp, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:    anthropic.Model(req.Model),
		Messages: claudeMessages,
		Tools:    claudeTools,
	})
	if err != nil {
		return ChatCompletionResponse{}, err
	}

	// Convert Claude's response to our format
	var toolCalls []ToolCall
	for _, block := range resp.Content {
		if block.Type == "tool_use" {
			toolUse := block.AsToolUse()
			args, _ := json.Marshal(toolUse.Input)
			toolCalls = append(toolCalls, ToolCall{
				ID:   toolUse.ID,
				Type: "function",
				Function: FunctionCall{
					Name:      toolUse.Name,
					Arguments: string(args),
				},
			})
		}
	}

	return ChatCompletionResponse{
		Choices: []ChatCompletionChoice{
			{
				Message: ChatCompletionMessage{
					Role:      "assistant",
					Content:   resp.Content[0].AsText().Text,
					ToolCalls: toolCalls,
				},
				FinishReason: string(resp.StopReason),
			},
		},
	}, nil
}

// CreateChatCompletionStream implements AIClient interface for Claude
func (a *ClaudeAdapter) CreateChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (ChatCompletionStream, error) {
	// Convert our messages to Claude's format
	claudeMessages := make([]anthropic.MessageParam, len(req.Messages))
	for i, m := range req.Messages {
		claudeMessages[i] = anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content))
	}

	// Convert our tools to Claude's format
	claudeTools := make([]anthropic.ToolUnionParam, len(req.Tools))
	for i, t := range req.Tools {
		claudeTools[i] = anthropic.ToolUnionParamOfTool(anthropic.ToolInputSchemaParam{
			Type:       "object",
			Properties: t.Function.Parameters,
		}, t.Function.Name)
	}

	// Call Claude
	stream := a.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:    anthropic.Model(req.Model),
		Messages: claudeMessages,
		Tools:    claudeTools,
	})

	// Return our adapter for the stream
	return &ClaudeStreamAdapter{stream: stream}, nil
}

// ClaudeStreamAdapter adapts Claude's stream to our ChatCompletionStream interface
type ClaudeStreamAdapter struct {
	stream *ssestream.Stream[anthropic.MessageStreamEventUnion]
}

func (a *ClaudeStreamAdapter) Recv() (ChatCompletionStreamResponse, error) {
	if !a.stream.Next() {
		if err := a.stream.Err(); err != nil {
			return ChatCompletionStreamResponse{}, err
		}
		return ChatCompletionStreamResponse{}, nil
	}

	event := a.stream.Current()
	switch event.Type {
	case "message_delta":
		delta := event.AsMessageDelta()
		return ChatCompletionStreamResponse{
			Choices: []ChatCompletionStreamChoice{
				{
					Delta: ChatCompletionStreamDelta{
						Role:         "assistant",
						FinishReason: string(delta.Delta.StopReason),
					},
				},
			},
		}, nil
	default:
		return ChatCompletionStreamResponse{}, fmt.Errorf("unexpected event type: %s", event.Type)
	}
}

func (a *ClaudeStreamAdapter) Close() {
	a.stream.Close()
}

// NewClaudeChatService creates a new chat service using Claude
func NewClaudeChatService(apiKey string, logger *logger.Logger, chatService *ChatService, queries *db.Queries, projectsDir string) *AIChatService {
	// Create Claude provider (not implemented yet)
	// claudeProvider := NewClaudeProvider(apiKey, logger)

	// For now, return a generic AI chat service with nil provider
	// This should be updated when Claude provider is implemented
	return &AIChatService{
		Logger:      logger,
		ChatService: chatService,
		Queries:     queries,
		ProjectsDir: projectsDir,
		AIProvider:  nil, // TODO: Implement ClaudeProvider
	}
}
