package ai

import (
	"context"

	"github.com/sashabaranov/go-openai"
)

// AIClient defines the interface for AI model clients
type AIClient interface {
	// CreateChatCompletion creates a chat completion with the given request
	CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error)
	// CreateChatCompletionStream creates a streaming chat completion
	CreateChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (ChatCompletionStream, error)
}

// ChatCompletionRequest represents a request to create a chat completion
type ChatCompletionRequest struct {
	Model    string
	Messages []ChatCompletionMessage
	Tools    []Tool
	Stream   bool
}

// ChatCompletionResponse represents a response from a chat completion
type ChatCompletionResponse struct {
	Choices []ChatCompletionChoice
}

// ChatCompletionChoice represents a single choice in a chat completion response
type ChatCompletionChoice struct {
	Message      ChatCompletionMessage
	FinishReason string
}

// ChatCompletionMessage represents a message in a chat completion
type ChatCompletionMessage struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

// Tool represents a tool that can be used by the AI model
type Tool struct {
	Type     string
	Function *FunctionDefinition
}

// FunctionDefinition defines a function that can be called by the AI model
type FunctionDefinition struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
}

// ToolCall represents a call to a tool by the AI model
type ToolCall struct {
	ID       string
	Type     string
	Function FunctionCall
}

// FunctionCall represents a function call within a tool call
type FunctionCall struct {
	Name      string
	Arguments string
}

// ChatCompletionStream represents a streaming chat completion
type ChatCompletionStream interface {
	Recv() (ChatCompletionStreamResponse, error)
	Close()
}

// ChatCompletionStreamResponse represents a response from a streaming chat completion
type ChatCompletionStreamResponse struct {
	Choices []ChatCompletionStreamChoice
}

// ChatCompletionStreamChoice represents a choice in a streaming chat completion response
type ChatCompletionStreamChoice struct {
	Delta ChatCompletionStreamDelta
}

// ChatCompletionStreamDelta represents a delta in a streaming chat completion response
type ChatCompletionStreamDelta struct {
	Content      string
	ToolCalls    []ToolCall
	Role         string
	FinishReason string
}

// Helper functions to convert between our types and OpenAI's types
func convertMessages(messages []ChatCompletionMessage) []openai.ChatCompletionMessage {
	result := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		result[i] = openai.ChatCompletionMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCalls:  convertToolCallsToOpenAI(m.ToolCalls),
			ToolCallID: m.ToolCallID,
		}
	}
	return result
}

func convertTools(tools []Tool) []openai.Tool {
	result := make([]openai.Tool, len(tools))
	for i, t := range tools {
		result[i] = openai.Tool{
			Type: openai.ToolType(t.Type),
			Function: &openai.FunctionDefinition{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			},
		}
	}
	return result
}

func convertToolCallsToOpenAI(toolCalls []ToolCall) []openai.ToolCall {
	result := make([]openai.ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		result[i] = openai.ToolCall{
			ID:   tc.ID,
			Type: openai.ToolType(tc.Type),
			Function: openai.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return result
}

func convertToolCallsFromOpenAI(toolCalls []openai.ToolCall) []ToolCall {
	result := make([]ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		result[i] = ToolCall{
			ID:   tc.ID,
			Type: string(tc.Type),
			Function: FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return result
}

func convertChoices(choices []openai.ChatCompletionChoice) []ChatCompletionChoice {
	result := make([]ChatCompletionChoice, len(choices))
	for i, c := range choices {
		result[i] = ChatCompletionChoice{
			Message: ChatCompletionMessage{
				Role:       c.Message.Role,
				Content:    c.Message.Content,
				ToolCalls:  convertToolCallsFromOpenAI(c.Message.ToolCalls),
				ToolCallID: c.Message.ToolCallID,
			},
			FinishReason: string(c.FinishReason),
		}
	}
	return result
}

func convertStreamChoices(choices []openai.ChatCompletionStreamChoice) []ChatCompletionStreamChoice {
	result := make([]ChatCompletionStreamChoice, len(choices))
	for i, c := range choices {
		result[i] = ChatCompletionStreamChoice{
			Delta: ChatCompletionStreamDelta{
				Content:      c.Delta.Content,
				ToolCalls:    convertToolCallsFromOpenAI(c.Delta.ToolCalls),
				Role:         c.Delta.Role,
				FinishReason: "", // OpenAI's stream delta doesn't have FinishReason
			},
		}
	}
	return result
}
