package ai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/errors"
	"github.com/chainlaunch/chainlaunch/pkg/http/response"
	"github.com/chainlaunch/chainlaunch/pkg/scai/boilerplates"
	"github.com/chainlaunch/chainlaunch/pkg/scai/projects"
	"github.com/chainlaunch/chainlaunch/pkg/scai/sessionchanges"
	"github.com/chainlaunch/chainlaunch/pkg/scai/versionmanagement"
	"github.com/go-chi/chi/v5"
)

// Model represents an AI model
type Model struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	MaxTokens   int    `json:"maxTokens"`
}

// Template represents a project template
type Template struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// GenerateRequest represents a code generation request
type GenerateRequest struct {
	ProjectID int64  `json:"projectId"`
	Prompt    string `json:"prompt"`
}

// GenerateResponse represents a code generation response
type GenerateResponse struct {
	Code string `json:"code"`
}

// NewAIHandler creates a new instance of AIHandler with the required dependencies
func NewAIHandler(aiService *AIChatService, chatService *ChatService, projectsService *projects.ProjectsService, boilerplateService *boilerplates.BoilerplateService) *AIHandler {
	return &AIHandler{
		AIChatService: aiService,
		ChatService:   chatService,
		Projects:      projectsService,
		Boilerplates:  boilerplateService,
	}
}

// AIHandler now has a ChatService field for dependency injection.
type AIHandler struct {
	AIChatService *AIChatService
	ChatService   *ChatService
	Projects      *projects.ProjectsService
	Boilerplates  *boilerplates.BoilerplateService
}

// RegisterRoutes registers all AI-related routes
func (h *AIHandler) RegisterRoutes(r chi.Router) {
	r.Route("/ai", func(r chi.Router) {
		r.Get("/boilerplates", response.Middleware(h.GetBoilerplates))
		r.Get("/models", response.Middleware(h.GetModels))
		r.Post("/{projectId}/chat", response.Middleware(h.Chat))
		r.Get("/{projectId}/conversations", response.Middleware(h.GetConversations))
		r.Post("/{projectId}/conversations", response.Middleware(h.CreateConversation))
		r.Get("/{projectId}/conversations/{conversationId}", response.Middleware(h.GetConversationMessages))
		r.Get("/{projectId}/conversations/{conversationId}/export", response.Middleware(h.GetConversationDetail))
	})
}

// GetBoilerplates godoc
// @Summary      Get available boilerplates
// @Description  Returns a list of available boilerplates filtered by network platform
// @Tags         ai
// @Produce      json
// @Param        network_id query int true "Network ID to filter boilerplates by platform"
// @Success      200 {array} boilerplates.BoilerplateConfig
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /ai/boilerplates [get]
func (h *AIHandler) GetBoilerplates(w http.ResponseWriter, r *http.Request) error {
	networkIDStr := r.URL.Query().Get("network_id")
	if networkIDStr == "" {
		return errors.NewValidationError("network_id is required", nil)
	}

	networkID, err := strconv.ParseInt(networkIDStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid network_id", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Get boilerplates for the network
	boilerplates, err := h.Boilerplates.GetBoilerplatesByNetworkID(r.Context(), networkID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return errors.NewNotFoundError("network not found", nil)
		}
		return errors.NewInternalError("failed to get boilerplates", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, boilerplates)
}

// GetModels godoc
// @Summary      Get available AI models
// @Description  Returns a list of available AI models for code generation
// @Tags         ai
// @Produce      json
// @Success      200 {array} Model
// @Failure      500 {object} response.ErrorResponse
// @Router       /ai/models [get]
func (h *AIHandler) GetModels(w http.ResponseWriter, r *http.Request) error {
	models := []Model{
		{
			Name:        "GPT-4",
			Description: "Most capable model, best for complex tasks",
			MaxTokens:   8192,
		},
		{
			Name:        "GPT-3.5",
			Description: "Fast and efficient model for simpler tasks",
			MaxTokens:   4096,
		},
	}
	return response.WriteJSON(w, http.StatusOK, models)
}

// GetConversations godoc
// @Summary      Get all conversations for a project
// @Description  Returns a list of all chat conversations associated with a specific project
// @Tags         ai
// @Produce      json
// @Param        projectId path int true "Project ID"
// @Success      200 {array} ConversationResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /ai/{projectId}/conversations [get]
func (h *AIHandler) GetConversations(w http.ResponseWriter, r *http.Request) error {
	projectID, err := strconv.ParseInt(chi.URLParam(r, "projectId"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid project ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	convs, err := h.ChatService.Queries.ListConversationsForProject(r.Context(), projectID)
	if err != nil {
		return errors.NewInternalError("failed to get conversations", err, nil)
	}

	var resp []ConversationResponse
	for _, c := range convs {
		resp = append(resp, ConversationResponse{
			ID:        c.ID,
			ProjectID: c.ProjectID,
			StartedAt: c.StartedAt.Format(time.RFC3339),
		})
	}

	return response.WriteJSON(w, http.StatusOK, resp)
}

// GetConversationMessages godoc
// @Summary      Get conversation messages
// @Description  Get all messages in a conversation
// @Tags         ai
// @Produce      json
// @Param        projectId path int true "Project ID"
// @Param        conversationId path int true "Conversation ID"
// @Success      200 {array} Message
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /ai/{projectId}/conversations/{conversationId} [get]
func (h *AIHandler) GetConversationMessages(w http.ResponseWriter, r *http.Request) error {
	projectID, err := strconv.ParseInt(chi.URLParam(r, "projectId"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid project ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	conversationID, err := strconv.ParseInt(chi.URLParam(r, "conversationId"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid conversation ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	messages, err := h.ChatService.GetConversationMessages(r.Context(), projectID, conversationID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return errors.NewNotFoundError("conversation not found", nil)
		}
		return errors.NewInternalError("failed to get conversation messages", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, messages)
}

// GetConversationDetail godoc
// @Summary      Get conversation detail
// @Description  Get detailed information about a conversation including all messages and metadata
// @Tags         ai
// @Produce      json
// @Param        projectId path int true "Project ID"
// @Param        conversationId path int true "Conversation ID"
// @Success      200 {object} ConversationDetail
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /ai/{projectId}/conversations/{conversationId}/export [get]
func (h *AIHandler) GetConversationDetail(w http.ResponseWriter, r *http.Request) error {
	projectID, err := strconv.ParseInt(chi.URLParam(r, "projectId"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid project ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	conversationID, err := strconv.ParseInt(chi.URLParam(r, "conversationId"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid conversation ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	detail, err := h.ChatService.GetConversationDetail(r.Context(), projectID, conversationID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return errors.NewNotFoundError("conversation not found", nil)
		}
		return errors.NewInternalError("failed to get conversation detail", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, detail)
}

// ConversationResponse represents a conversation for API responses
// swagger:model
type ConversationResponse struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"projectId"`
	StartedAt string `json:"startedAt"`
}

// MessageResponse represents a message for API responses
// swagger:model
type MessageResponse struct {
	ID             int64  `json:"id"`
	ConversationID int64  `json:"conversationId"`
	Sender         string `json:"sender"`
	Content        string `json:"content"`
	CreatedAt      string `json:"createdAt"`
}

// MessageDetailResponse represents a message with tool calls
// swagger:model
type MessageDetailResponse struct {
	ID             int64              `json:"id"`
	ConversationID int64              `json:"conversationId"`
	Sender         string             `json:"sender"`
	Content        string             `json:"content"`
	CreatedAt      string             `json:"createdAt"`
	ToolCalls      []ToolCallResponse `json:"toolCalls"`
}

// ToolCallResponse represents a tool call for API responses
// swagger:model
type ToolCallResponse struct {
	ID        int64  `json:"id"`
	MessageID int64  `json:"messageId"`
	ToolName  string `json:"toolName"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
	Error     string `json:"error"`
	CreatedAt string `json:"createdAt"`
}

// ParentMessageDetailResponse represents a parent message with its children (tool calls, etc.)
// swagger:model
type ParentMessageDetailResponse struct {
	Message  MessageDetailResponse   `json:"message"`
	Children []MessageDetailResponse `json:"children"`
}

type ChatMessagePart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ChatMessage struct {
	Role    string            `json:"role"`
	Content string            `json:"content"`
	Parts   []ChatMessagePart `json:"parts"`
}

type ChatRequest struct {
	ID             string        `json:"id"`
	Messages       []ChatMessage `json:"messages"`
	ProjectID      string        `json:"projectId"`
	ConversationID int64         `json:"conversationId"`
}

// ChatResponse is not used for streaming, but kept for reference.
type ChatResponse struct {
	Reply string `json:"reply"`
}

// EventType is a string type for SSE event types
// (stringer is optional, but helps with enums)
//
//go:generate stringer -type=EventType
type EventType string

const (
	EventTypeLLM             EventType = "llm"
	EventTypeToolStart       EventType = "tool_start"
	EventTypeToolUpdate      EventType = "tool_update"
	EventTypeToolExecute     EventType = "tool_execute"
	EventTypeToolResult      EventType = "tool_result"
	EventTypeMaxStepsReached EventType = "max_steps_reached"
)

// Update all event structs to use EventType for the Type field

type TokenEvent struct {
	Type  EventType `json:"type"`
	Token string    `json:"token"`
}

type LLMEvent struct {
	Type    EventType `json:"type"`
	Content string    `json:"content"`
}

type ToolStartEvent struct {
	Type       EventType `json:"type"`
	ToolCallID string    `json:"toolCallID"`
	Name       string    `json:"name"`
}

type ToolUpdateEvent struct {
	Type       EventType `json:"type"`
	ToolCallID string    `json:"toolCallID"`
	Name       string    `json:"name"`
	Arguments  string    `json:"arguments"`
}

type ToolExecuteEvent struct {
	Type       EventType              `json:"type"`
	ToolCallID string                 `json:"toolCallID"`
	Name       string                 `json:"name"`
	Args       map[string]interface{} `json:"args"`
}

type ToolResultEvent struct {
	Type       EventType   `json:"type"`
	ToolCallID string      `json:"toolCallID"`
	Name       string      `json:"name"`
	Result     interface{} `json:"result"`
	Error      string      `json:"error,omitempty"`
}

type MaxStepsReachedEvent struct {
	Type EventType `json:"type"`
}

// sseAgentStepObserver streams agent step events as SSE to the client
// Needs access to http.ResponseWriter and http.Flusher
// We'll store them as fields in the struct
type sseAgentStepObserver struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func (o *sseAgentStepObserver) OnLLMContent(content string) {
	if content == "" {
		return
	}
	evt := LLMEvent{Type: EventTypeLLM, Content: content}
	data, _ := json.Marshal(evt)
	fmt.Fprintf(o.w, "data: %s\n\n", data)
	o.flusher.Flush()
}

func (o *sseAgentStepObserver) OnToolCallStart(toolCallID, name string) {
	evt := ToolStartEvent{Type: EventTypeToolStart, ToolCallID: toolCallID, Name: name}
	data, _ := json.Marshal(evt)
	fmt.Fprintf(o.w, "data: %s\n\n", data)
	o.flusher.Flush()
}

func (o *sseAgentStepObserver) OnToolCallUpdate(toolCallID, name, arguments string) {
	evt := ToolUpdateEvent{Type: EventTypeToolUpdate, ToolCallID: toolCallID, Name: name, Arguments: arguments}
	data, _ := json.Marshal(evt)
	fmt.Fprintf(o.w, "data: %s\n\n", data)
	o.flusher.Flush()
}

func (o *sseAgentStepObserver) OnToolCallExecute(toolCallID, name string, args map[string]interface{}) {
	evt := ToolExecuteEvent{Type: EventTypeToolExecute, ToolCallID: toolCallID, Name: name, Args: args}
	data, _ := json.Marshal(evt)
	fmt.Fprintf(o.w, "data: %s\n\n", data)
	o.flusher.Flush()
}

func (o *sseAgentStepObserver) OnToolCallResult(toolCallID, name string, result interface{}, err error) {
	evt := ToolResultEvent{Type: EventTypeToolResult, ToolCallID: toolCallID, Name: name, Result: result}
	if err != nil {
		evt.Error = err.Error()
	}
	data, _ := json.Marshal(evt)
	fmt.Fprintf(o.w, "data: %s\n\n", data)
	o.flusher.Flush()
}

func (o *sseAgentStepObserver) OnMaxStepsReached() {
	evt := MaxStepsReachedEvent{Type: EventTypeMaxStepsReached}
	data, _ := json.Marshal(evt)
	fmt.Fprintf(o.w, "data: %s\n\n", data)
	o.flusher.Flush()
}

// CreateConversationRequest represents a request to create a new conversation
type CreateConversationRequest struct {
	Title string `json:"title,omitempty"`
}

// CreateConversation godoc
// @Summary      Create a new conversation for a project
// @Description  Creates a new empty conversation for the specified project
// @Tags         ai
// @Accept       json
// @Produce      json
// @Param        projectId path int true "Project ID"
// @Param        request body CreateConversationRequest false "Optional conversation title"
// @Success      201 {object} ConversationResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /ai/{projectId}/conversations [post]
func (h *AIHandler) CreateConversation(w http.ResponseWriter, r *http.Request) error {
	projectID, err := strconv.ParseInt(chi.URLParam(r, "projectId"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid project ID", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Verify project exists
	_, err = h.Projects.Queries.GetProject(r.Context(), projectID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return errors.NewNotFoundError("project not found", nil)
		}
		return errors.NewInternalError("failed to get project", err, nil)
	}

	// Parse optional request body
	var req CreateConversationRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return errors.NewValidationError("invalid request body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Create the conversation
	conversation, err := h.ChatService.CreateConversation(r.Context(), projectID, req.Title)
	if err != nil {
		return errors.NewInternalError("failed to create conversation", err, nil)
	}

	// Return the created conversation
	resp := ConversationResponse{
		ID:        conversation.ID,
		ProjectID: conversation.ProjectID,
		StartedAt: conversation.StartedAt.Format(time.RFC3339),
	}

	return response.WriteJSON(w, http.StatusCreated, resp)
}

// Chat godoc
// @Summary      Chat with AI assistant
// @Description  Stream a conversation with the AI assistant using Server-Sent Events (SSE)
// @Tags         ai
// @Accept       json
// @Produce      text/event-stream
// @Param        projectId path int true "Project ID"
// @Param        request body ChatRequest true "Chat request containing project ID and messages"
// @Success      200 {string} string "SSE stream of chat responses"
// @Failure      400 {string} string "Invalid request"
// @Failure      500 {string} string "Internal server error"
// @Router       /ai/{projectId}/chat [post]
func (h *AIHandler) Chat(w http.ResponseWriter, r *http.Request) error {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}
	projectIdStr := chi.URLParam(r, "projectId")
	if projectIdStr == "" {
		return fmt.Errorf("projectId is required")
	}
	if req.ConversationID == 0 {
		return fmt.Errorf("conversationId is required")
	}

	// Use projectId from path or from body (prefer path param)
	projectID, err := strconv.ParseInt(projectIdStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid projectId: %w", err)
	}

	if len(req.Messages) == 0 {
		return fmt.Errorf("messages are required")
	}

	// Use the last user message as the prompt
	var userMessage string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			userMessage = req.Messages[i].Content
			break
		}
	}
	if userMessage == "" {
		return fmt.Errorf("no user message found")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming unsupported")
	}

	observer := &sseAgentStepObserver{w: w, flusher: flusher}

	// Create a new session changes tracker for this chat session
	sessionTracker := sessionchanges.NewTracker()
	err = h.AIChatService.ChatWithPersistence(r.Context(), projectID, userMessage, observer, 0, req.ConversationID, sessionTracker)
	if err != nil && err != io.EOF {
		return fmt.Errorf("chat error: %w", err)
	}

	// After chat session, commit all changed files in the correct project directory
	files := sessionTracker.GetAndResetChanges()
	if len(files) > 0 {
		msg := "AI Chat Session: Modified files:\n- " + strings.Join(files, "\n- ")
		vm := versionmanagement.NewDefaultManager()
		ctx := r.Context()
		// Get project directory from projectID
		proj, err := h.Projects.GetProject(ctx, projectID)
		if err == nil {
			projectDir, err := h.Projects.GetProjectDirectory(proj)
			if err != nil {
				fmt.Printf("Failed to get safe project directory: %v\n", err)
			} else {
				if err := vm.CommitChange(ctx, projectDir, msg); err != nil {
					fmt.Printf("Failed to commit session changes: %v\n", err)
				}
			}
		}
	}
	return nil
}
