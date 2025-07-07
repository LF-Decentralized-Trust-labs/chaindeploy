package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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

// AddMessage stores a message in the conversation. Accepts optional parentID, enhanced content, and tool arguments.
func (s *ChatService) AddMessage(ctx context.Context, conversationID int64, parentID *int64, sender, content string, enhancedContent string, toolArguments string, isInternal ...bool) (*db.Message, error) {
	var parentNull sql.NullInt64
	if parentID != nil {
		parentNull = sql.NullInt64{Int64: *parentID, Valid: true}
	}

	var enhancedContentNull sql.NullString
	if len(enhancedContent) > 0 && enhancedContent != "" {
		enhancedContentNull = sql.NullString{String: enhancedContent, Valid: true}
	}

	var toolArgumentsNull sql.NullString
	if len(toolArguments) > 0 && toolArguments != "" {
		toolArgumentsNull = sql.NullString{String: toolArguments, Valid: true}
	}

	internal := false
	if len(isInternal) > 0 {
		internal = isInternal[0]
	}

	row, err := s.Queries.InsertMessage(ctx, &db.InsertMessageParams{
		ConversationID:  conversationID,
		ParentID:        parentNull,
		Sender:          sender,
		Content:         content,
		EnhancedContent: enhancedContentNull,
		ToolArguments:   toolArgumentsNull,
		IsInternal:      internal,
	})
	if err != nil {
		return nil, err
	}
	return row, nil
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

// GetMessages returns all messages for a conversation. If showInternal is true, includes internal messages.
func (s *ChatService) GetMessages(ctx context.Context, conversationID int64, showInternal bool) ([]*db.Message, error) {
	msgs, err := s.Queries.ListMessagesForConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if showInternal {
		return msgs, nil
	}
	var filtered []*db.Message
	for _, m := range msgs {
		if !m.IsInternal {
			filtered = append(filtered, m)
		}
	}
	return filtered, nil
}

// GetConversationMessages returns all messages for a conversation with their tool calls. If showInternal is true, includes internal messages.
func (s *ChatService) GetConversationMessages(ctx context.Context, projectID, conversationID int64, showInternal bool) ([]Message, error) {
	// Get all messages for the conversation
	messages, err := s.Queries.ListMessagesForConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	if !showInternal {
		var filteredMessages []*db.Message
		for _, msg := range messages {
			if !msg.IsInternal {
				filteredMessages = append(filteredMessages, msg)
			}
		}
		messages = filteredMessages
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
		var toolArguments *string
		if msg.ToolArguments.Valid {
			toolArguments = &msg.ToolArguments.String
		}
		var toolCalls []*ToolCallAI
		for _, toolCall := range toolCallsByMsg[msg.ID] {
			toolCalls = append(toolCalls, &ToolCallAI{
				ID:        toolCall.ID,
				MessageID: toolCall.MessageID,
				ToolName:  toolCall.ToolName,
				Arguments: toolCall.Arguments,
				Result:    toolCall.Result.String,
				Error:     toolCall.Error.String,
				CreatedAt: toolCall.CreatedAt,
			})
		}
		result = append(result, Message{
			ID:             msg.ID,
			ConversationID: msg.ConversationID,
			Sender:         msg.Sender,
			Content:        msg.Content,
			CreatedAt:      msg.CreatedAt.Format(time.RFC3339),
			ToolArguments:  toolArguments,
			ToolCalls:      toolCalls,
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
	messages, err := s.GetConversationMessages(ctx, projectID, conversationID, false)
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
	ID             int64         `json:"id"`
	ConversationID int64         `json:"conversationId"`
	Sender         string        `json:"sender"`
	Content        string        `json:"content"`
	CreatedAt      string        `json:"createdAt"`
	ToolArguments  *string       `json:"toolArguments,omitempty"`
	ToolCalls      []*ToolCallAI `json:"toolCalls,omitempty"`
}

type ToolCallAI struct {
	ID        int64     `json:"id"`
	MessageID int64     `json:"messageId"`
	ToolName  string    `json:"toolName"`
	Arguments string    `json:"arguments"`
	Result    string    `json:"result"`
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"createdAt"`
}

// ConversationDetail represents detailed information about a conversation
type ConversationDetail struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"projectId"`
	StartedAt string    `json:"startedAt"`
	Messages  []Message `json:"messages"`
}

// SummarizeConversation creates a new conversation from an existing one with a summary message generated by the AI.
func (s *ChatService) SummarizeConversation(
	ctx context.Context,
	projectID int64,
	conversationID int64,
	aiService *AIChatService,
) (int64, string, string, error) {
	// Fetch conversation messages
	messages, err := s.GetConversationMessages(ctx, projectID, conversationID, false)
	if err != nil {
		return 0, "", "", err
	}
	if len(messages) == 0 {
		return 0, "", "", fmt.Errorf("no messages found in conversation")
	}

	// Build the conversation text for the prompt
	var conversationText strings.Builder
	for _, m := range messages {
		conversationText.WriteString("[" + m.Sender + "] ")
		conversationText.WriteString(m.Content)
		conversationText.WriteString("\n")
	}

	prompt := `You are a technical conversation summarizer for an AI coding assistant. Create a concise but comprehensive summary of this coding conversation that will be used to maintain context in a new chat session.

PRESERVE THESE CRITICAL ELEMENTS:

**PROJECT CONTEXT:**
- Project type, tech stack, and architecture decisions
- Key libraries, frameworks, and dependencies discussed
- Database schema or data structures established
- API endpoints or interfaces defined

**TECHNICAL DECISIONS:**
- Design patterns and coding conventions adopted
- Performance optimizations implemented
- Security considerations addressed
- Testing strategies discussed

**CODE STATE:**
- Files created, modified, or referenced
- Functions/classes implemented or planned
- Configuration changes made
- Dependencies added or updated

**ACTIVE WORK:**
- Current implementation status
- Unresolved issues or bugs being addressed
- Next steps or TODOs explicitly mentioned
- Features in progress or planned

**CONTEXT REFERENCES:**
- Important code snippets or algorithms discussed
- Error messages or debugging insights
- External resources or documentation referenced
- Specific requirements or constraints

EXCLUDE THESE ELEMENTS:
- Conversational pleasantries and acknowledgments
- Repetitive explanations of the same concept
- Detailed step-by-step code walkthroughs (keep only final outcomes)
- Tangential discussions not related to the main task

FORMAT REQUIREMENTS:
- Use clear section headers
- Keep technical details precise but concise
- Include specific file names, function names, and key variables
- Mention version numbers or specific configurations when relevant
- Maximum 500 words unless complexity requires more

CONVERSATION TO SUMMARIZE:
` + conversationText.String() + `

Create a summary that allows the AI to seamlessly continue the technical discussion with full context of what has been accomplished and what needs to be done next.`

	model := aiService.Model
	aiProvider := aiService.AIProvider

	// Define the JSON schema for the summary
	summarySchema := `{
	  "type": "object",
	  "properties": {
		"title": {"type": "string", "description": "A concise title for the summary."},
		"content": {"type": "string", "description": "The full summary content."}
	  },
	  "required": ["title", "content"]
	}`

	jsonStr, err := aiProvider.GenerateJSONSchemaFromMessage(ctx, prompt, model, summarySchema)
	if err != nil {
		return 0, "", "", fmt.Errorf("failed to generate summary: %w", err)
	}

	var summaryObj struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &summaryObj); err != nil {
		return 0, "", "", fmt.Errorf("failed to parse summary JSON: %w", err)
	}
	if strings.TrimSpace(summaryObj.Title) == "" || strings.TrimSpace(summaryObj.Content) == "" {
		return 0, "", "", fmt.Errorf("AI did not return a valid summary (missing title or content)")
	}

	// Create a new conversation
	title := summaryObj.Title
	conv, err := s.CreateConversation(ctx, projectID, title)
	if err != nil {
		return 0, "", "", fmt.Errorf("failed to create new conversation: %w", err)
	}

	summaryTitle := title
	summaryContent := summaryObj.Content
	_, err = s.AddMessage(ctx, conv.ID, nil, "summary", summaryContent, "", summaryTitle)
	if err != nil {
		return 0, "", "", fmt.Errorf("failed to add summary message: %w", err)
	}

	return conv.ID, summaryTitle, summaryContent, nil
}
