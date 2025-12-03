package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/chainlaunch/chainlaunch/pkg/notifications"
	"github.com/chainlaunch/chainlaunch/pkg/notifications/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

type NotificationHandler struct {
	service  *service.NotificationService
	validate *validator.Validate
}

func NewNotificationHandler(service *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{
		service:  service,
		validate: validator.New(),
	}
}

func (h *NotificationHandler) RegisterRoutes(r chi.Router) {
	r.Route("/notifications", func(r chi.Router) {
		r.Post("/providers", h.CreateProvider)
		r.Get("/providers", h.ListProviders)
		r.Route("/providers/{providerId}", func(r chi.Router) {
			r.Get("/", h.GetProvider)
			r.Put("/", h.UpdateProvider)
			r.Delete("/", h.DeleteProvider)
			r.Post("/test", h.TestProvider)
		})
	})
}

// @Summary Create a notification provider
// @Description Create a new notification provider with the specified configuration
// @Tags Notifications
// @Accept json
// @Produce json
// @Param request body CreateProviderRequest true "Provider creation request"
// @Success 201 {object} ProviderResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /notifications/providers [post]
func (h *NotificationHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var req CreateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	provider, err := h.service.CreateProvider(r.Context(), notifications.CreateProviderParams{
		Type:                   req.Type,
		Name:                   req.Name,
		Config:                 req.Config,
		IsDefault:              req.IsDefault,
		NotifyNodeDowntime:     req.NotifyNodeDowntime,
		NotifyBackupSuccess:    req.NotifyBackupSuccess,
		NotifyBackupFailure:    req.NotifyBackupFailure,
		NotifyS3ConnIssue:      req.NotifyS3ConnIssue,
		NotifyDiskSpaceWarning: req.NotifyDiskSpaceWarning,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(provider)
}

// @Summary List notification providers
// @Description Get a list of all notification providers
// @Tags Notifications
// @Accept json
// @Produce json
// @Success 200 {array} ProviderResponse
// @Failure 500 {object} ErrorResponse
// @Router /notifications/providers [get]
func (h *NotificationHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.service.ListProviders(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providers)
}

// @Summary Get a notification provider
// @Description Get detailed information about a specific notification provider
// @Tags Notifications
// @Accept json
// @Produce json
// @Param id path int true "Provider ID"
// @Success 200 {object} ProviderResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /notifications/providers/{id} [get]
func (h *NotificationHandler) GetProvider(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "providerId"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	provider, err := h.service.GetProvider(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(provider)
}

// @Summary Update a notification provider
// @Description Update an existing notification provider with new configuration
// @Tags Notifications
// @Accept json
// @Produce json
// @Param id path int true "Provider ID"
// @Param request body UpdateProviderRequest true "Provider update request"
// @Success 200 {object} ProviderResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /notifications/providers/{id} [put]
func (h *NotificationHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "providerId"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var req UpdateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	provider, err := h.service.UpdateProvider(r.Context(), notifications.UpdateProviderParams{
		ID:                     id,
		Type:                   req.Type,
		Name:                   req.Name,
		Config:                 req.Config,
		IsDefault:              req.IsDefault,
		NotifyNodeDowntime:     req.NotifyNodeDowntime,
		NotifyBackupSuccess:    req.NotifyBackupSuccess,
		NotifyBackupFailure:    req.NotifyBackupFailure,
		NotifyS3ConnIssue:      req.NotifyS3ConnIssue,
		NotifyDiskSpaceWarning: req.NotifyDiskSpaceWarning,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(provider)
}

// @Summary Delete a notification provider
// @Description Delete a notification provider
// @Tags Notifications
// @Accept json
// @Produce json
// @Param id path int true "Provider ID"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /notifications/providers/{id} [delete]
func (h *NotificationHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "providerId"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteProvider(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary Test a notification provider
// @Description Test a notification provider
// @Tags Notifications
// @Accept json
// @Produce json
// @Param id path int true "Provider ID"
// @Param request body TestProviderRequest true "Test provider request"
// @Success 200 {object} TestProviderResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /notifications/providers/{id}/test [post]
func (h *NotificationHandler) TestProvider(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "providerId"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var req TestProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.service.TestProvider(r.Context(), id, notifications.TestProviderParams{
		TestEmail: req.TestEmail,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
