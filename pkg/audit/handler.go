package audit

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/auth"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/go-chi/chi/v5"
)

// Handler handles HTTP requests for audit logs
type Handler struct {
	service *AuditService
	logger  *logger.Logger
}

// NewHandler creates a new audit handler
func NewHandler(service *AuditService, logger *logger.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes registers the audit routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/audit", func(r chi.Router) {
		r.Get("/logs", h.ListLogs)
		r.Get("/logs/{id}", h.GetLog)
	})
}

// ListLogsRequest represents the request parameters for listing audit logs
type ListLogsRequest struct {
	Page      int       `json:"page"`
	PageSize  int       `json:"page_size"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	EventType string    `json:"event_type"`
	UserID    string    `json:"user_id"`
}

// ListLogsResponse represents the response for listing audit logs
type ListLogsResponse struct {
	Items      []Event `json:"items"`
	TotalCount int     `json:"total_count"`
	Page       int     `json:"page"`
	PageSize   int     `json:"page_size"`
}

// ListLogs retrieves a list of audit logs
// @Summary List audit logs
// @Description Retrieves a paginated list of audit logs with optional filters
// @Tags audit
// @Accept json
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param page_size query int false "Page size (default: 10)"
// @Param start query string false "Start time (RFC3339 format)"
// @Param end query string false "End time (RFC3339 format)"
// @Param event_type query string false "Filter by event type"
// @Param user_id query string false "Filter by user ID"
// @Success 200 {object} ListLogsResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /audit/logs [get]
// @BasePath /api/v1
func (h *Handler) ListLogs(w http.ResponseWriter, r *http.Request) {
	// Check if user has admin role
	user, ok := auth.UserFromContext(r.Context())
	if !ok || user.Role != auth.RoleAdmin {
		http.Error(w, "Unauthorized: Admin role required", http.StatusForbidden)
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 10
	}

	startStr := r.URL.Query().Get("start")
	var start *time.Time
	if startStr != "" {
		var err error
		startTime, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			http.Error(w, "Invalid start time format", http.StatusBadRequest)
			return
		}
		start = &startTime
	}

	endStr := r.URL.Query().Get("end")
	var end *time.Time
	if endStr != "" {
		var err error
		endTime, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			http.Error(w, "Invalid end time format", http.StatusBadRequest)
			return
		}
		end = &endTime
	}

	eventType := r.URL.Query().Get("event_type")
	userIDInt := int64(0)
	userID := r.URL.Query().Get("user_id")
	if userID != "" {
		userInt, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}
		userIDInt = userInt
	}

	// TODO: Implement pagination and filtering in the service layer
	// For now, we'll return all logs
	logs, err := h.service.ListLogs(r.Context(), page, pageSize, start, end, eventType, userIDInt)
	if err != nil {
		h.logger.Error("Failed to list audit logs", "error", err)
		http.Error(w, "Failed to list audit logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(logs)
}

// GetLog retrieves a specific audit log by ID
// @Summary Get audit log
// @Description Retrieves a specific audit log by ID
// @Tags audit
// @Produce json
// @Param id path string true "Log ID"
// @Success 200 {object} Event
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /audit/logs/{id} [get]
// @BasePath /api/v1
func (h *Handler) GetLog(w http.ResponseWriter, r *http.Request) {
	// Check if user has admin role
	user, ok := auth.UserFromContext(r.Context())
	if !ok || user.Role != auth.RoleAdmin {
		http.Error(w, "Unauthorized: Admin role required", http.StatusForbidden)
		return
	}

	logID := chi.URLParam(r, "id")
	if logID == "" {
		http.Error(w, "Log ID is required", http.StatusBadRequest)
		return
	}
	logIDInt, err := strconv.ParseInt(logID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid log ID", http.StatusBadRequest)
		return
	}
	log, err := h.service.GetLog(r.Context(), logIDInt)
	if err != nil {
		h.logger.Error("Failed to get audit log", "error", err)
		http.Error(w, "Failed to get audit log", http.StatusInternalServerError)
		return
	}

	if log == nil {
		http.Error(w, "Log not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(log)
}
