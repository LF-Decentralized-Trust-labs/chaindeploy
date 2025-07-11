package ai

import (
	"context"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/ssestream"
)

type OpenAIAdapter struct {
	client openai.Client
}

func NewOpenAIAdapter(apiKey string) *OpenAIAdapter {
	return &OpenAIAdapter{
		client: openai.NewClient(option.WithAPIKey(apiKey)),
	}
}

func (a *OpenAIAdapter) CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	messages := make([]openai.ChatCompletionMessageParamUnion, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = openai.UserMessage(m.Content)
	}

	oaiReq := openai.ChatCompletionNewParams{
		Model:    req.Model,
		Messages: messages,
		// Add more fields as needed (Tools, etc.)
	}

	resp, err := a.client.Chat.Completions.New(ctx, oaiReq)
	if err != nil {
		return ChatCompletionResponse{}, err
	}

	choices := make([]ChatCompletionChoice, len(resp.Choices))
	for i, c := range resp.Choices {
		choices[i] = ChatCompletionChoice{
			Message: ChatCompletionMessage{
				Role:    string(c.Message.Role),
				Content: c.Message.Content,
			},
			FinishReason: string(c.FinishReason),
		}
	}

	return ChatCompletionResponse{
		Choices: choices,
	}, nil
}

func (a *OpenAIAdapter) CreateChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (ChatCompletionStream, error) {
	messages := make([]openai.ChatCompletionMessageParamUnion, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = openai.UserMessage(m.Content)
	}
	oaiReq := openai.ChatCompletionNewParams{
		Model:    req.Model,
		Messages: messages,
		// Add more fields as needed (Tools, etc.)
	}
	stream := a.client.Chat.Completions.NewStreaming(ctx, oaiReq)
	return &OpenAIStreamAdapter{stream: stream}, nil
}

type OpenAIStreamAdapter struct {
	stream *ssestream.Stream[openai.ChatCompletionChunk]
}

func (a *OpenAIStreamAdapter) Recv() (ChatCompletionStreamResponse, error) {
	if !a.stream.Next() {
		if err := a.stream.Err(); err != nil {
			return ChatCompletionStreamResponse{}, err
		}
		return ChatCompletionStreamResponse{}, nil // End of stream
	}
	chunk := a.stream.Current()
	choices := make([]ChatCompletionStreamChoice, len(chunk.Choices))
	for i, c := range chunk.Choices {
		choices[i] = ChatCompletionStreamChoice{
			Delta: ChatCompletionStreamDelta{
				Role:    string(c.Delta.Role),
				Content: c.Delta.Content,
			},
		}
	}
	return ChatCompletionStreamResponse{
		Choices: choices,
	}, nil
}

func (a *OpenAIStreamAdapter) Close() {
	a.stream.Close()
}
