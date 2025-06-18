package ai

import (
	"context"
	"database/sql"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
)

type ChatService struct {
	Queries *db.Queries
}

type Conversation struct {
	ID        int64
	ProjectID int64
	StartedAt time.Time
}

func NewChatService(queries *db.Queries) *ChatService {
	return &ChatService{Queries: queries}
}

// CreateConversation creates a new conversation with the given title for a project.
func (s *ChatService) CreateConversation(ctx context.Context, projectID int64, title string) (Conversation, error) {
	row, err := s.Queries.CreateConversation(ctx, projectID)
	if err != nil {
		return Conversation{}, err
	}
	return Conversation{
		ID:        row.ID,
		ProjectID: row.ProjectID,
		StartedAt: row.StartedAt,
	}, nil
}

func (s *ChatService) GetConversation(ctx context.Context, conversationID int64) (Conversation, error) {
	row, err := s.Queries.GetConversation(ctx, conversationID)
	if err != nil {
		return Conversation{}, err
	}
	return Conversation{
		ID:        row.ID,
		ProjectID: row.ProjectID,
		StartedAt: row.StartedAt,
	}, nil
}

// EnsureConversationForProject returns the default conversation for a project, creating it if needed.
func (s *ChatService) EnsureConversationForProject(ctx context.Context, projectID int64) (Conversation, error) {
	conv, err := s.Queries.GetDefaultConversationForProject(ctx, projectID)
	if err == sql.ErrNoRows {
		// Create new conversation
		row, err := s.Queries.CreateConversation(ctx, projectID)
		if err != nil {
			return Conversation{}, err
		}
		return Conversation{
			ID:        row.ID,
			ProjectID: row.ProjectID,
			StartedAt: row.StartedAt,
		}, nil
	} else if err != nil {
		return Conversation{}, err
	}
	return Conversation{
		ID:        conv.ID,
		ProjectID: conv.ProjectID,
		StartedAt: conv.StartedAt,
	}, nil
}

// AddMessage stores a message in the conversation. Accepts optional parentID and enhanced content.
func (s *ChatService) AddMessage(ctx context.Context, conversationID int64, parentID *int64, sender, content string, enhancedContent string) (*db.Message, error) {
	var parentNull sql.NullInt64
	if parentID != nil {
		parentNull = sql.NullInt64{Int64: *parentID, Valid: true}
	}

	var enhancedContentNull sql.NullString
	if len(enhancedContent) > 0 && enhancedContent != "" {
		enhancedContentNull = sql.NullString{String: enhancedContent, Valid: true}
	}

	row, err := s.Queries.InsertMessage(ctx, &db.InsertMessageParams{
		ConversationID:  conversationID,
		ParentID:        parentNull,
		Sender:          sender,
		Content:         content,
		EnhancedContent: enhancedContentNull,
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

// GetMessages returns all messages for a conversation.
func (s *ChatService) GetMessages(ctx context.Context, conversationID int64) ([]*db.Message, error) {
	return s.Queries.ListMessagesForConversation(ctx, conversationID)
}

// AddToolCall stores a tool call for a message.
func (s *ChatService) AddToolCall(ctx context.Context, messageID int64, toolName, arguments, result string, errStr *string) (*db.ToolCall, error) {
	var resultNull sql.NullString
	if result != "" {
		resultNull = sql.NullString{String: result, Valid: true}
	}
	var errorNull sql.NullString
	if errStr != nil {
		errorNull = sql.NullString{String: *errStr, Valid: true}
	}
	return s.Queries.InsertToolCall(ctx, &db.InsertToolCallParams{
		MessageID: messageID,
		ToolName:  toolName,
		Arguments: arguments,
		Result:    resultNull,
		Error:     errorNull,
	})
}

// GetConversationMessages returns all messages for a conversation with their tool calls.
func (s *ChatService) GetConversationMessages(ctx context.Context, projectID, conversationID int64) ([]Message, error) {
	// Get all messages for the conversation
	messages, err := s.Queries.ListMessagesForConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	// Get tool calls for all messages
	toolCallsByMsg := make(map[int64][]*db.ToolCall)
	for _, msg := range messages {
		toolCalls, _ := s.Queries.ListToolCallsForMessage(ctx, msg.ID)
		toolCallsByMsg[msg.ID] = toolCalls
	}

	// Convert messages to response format
	result := []Message{}
	for _, msg := range messages {
		result = append(result, Message{
			ID:             msg.ID,
			ConversationID: msg.ConversationID,
			Sender:         msg.Sender,
			Content:        msg.Content,
			CreatedAt:      msg.CreatedAt.Format(time.RFC3339),
			ToolCalls:      toolCallsByMsg[msg.ID],
		})
	}

	return result, nil
}

// GetConversationDetail returns detailed information about a conversation.
func (s *ChatService) GetConversationDetail(ctx context.Context, projectID, conversationID int64) (*ConversationDetail, error) {
	// Get conversation info
	conv, err := s.Queries.GetDefaultConversationForProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Get all messages with their tool calls
	messages, err := s.GetConversationMessages(ctx, projectID, conversationID)
	if err != nil {
		return nil, err
	}

	return &ConversationDetail{
		ID:        conv.ID,
		ProjectID: conv.ProjectID,
		StartedAt: conv.StartedAt.Format(time.RFC3339),
		Messages:  messages,
	}, nil
}

// GenerateCode generates code using the AI service.
func (s *ChatService) GenerateCode(ctx context.Context, prompt string, project *db.ChaincodeProject) (string, error) {
	// This is a placeholder implementation. In a real implementation, this would use the AI service
	// to generate code based on the prompt and project context.
	return "// Generated code placeholder", nil
}

// Message represents a chat message with its tool calls
type Message struct {
	ID             int64          `json:"id"`
	ConversationID int64          `json:"conversationId"`
	Sender         string         `json:"sender"`
	Content        string         `json:"content"`
	CreatedAt      string         `json:"createdAt"`
	ToolCalls      []*db.ToolCall `json:"toolCalls,omitempty"`
}

// ConversationDetail represents detailed information about a conversation
type ConversationDetail struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"projectId"`
	StartedAt string    `json:"startedAt"`
	Messages  []Message `json:"messages"`
}
