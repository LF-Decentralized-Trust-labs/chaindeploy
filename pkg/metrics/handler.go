package metrics

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/errors"
	"github.com/chainlaunch/chainlaunch/pkg/http/response"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/metrics/common"
	"github.com/chainlaunch/chainlaunch/pkg/metrics/types"
	"github.com/go-chi/chi/v5"
)

// Handler handles HTTP requests for metrics
type Handler struct {
	service common.Service
	logger  *logger.Logger
}

// NewHandler creates a new metrics handler
func NewHandler(service common.Service, logger *logger.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes registers the metrics routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/metrics", func(r chi.Router) {
		r.Post("/deploy", response.Middleware(h.DeployPrometheus))
		r.Post("/undeploy", response.Middleware(h.UndeployPrometheus))
		r.Get("/node/{id}", response.Middleware(h.GetNodeMetrics))
		r.Post("/reload", response.Middleware(h.ReloadConfiguration))
		r.Get("/node/{id}/label/{label}/values", response.Middleware(h.GetLabelValues))
		r.Get("/node/{id}/range", response.Middleware(h.GetNodeMetricsRange))
		r.Post("/node/{id}/query", response.Middleware(h.CustomQuery))
		r.Get("/status", response.Middleware(h.GetStatus))
	})
}

// DeployPrometheus deploys a new Prometheus instance
// @Summary Deploy a new Prometheus instance
// @Description Deploys a new Prometheus instance with the specified configuration
// @Tags Metrics
// @Accept json
// @Produce json
// @Param request body types.DeployPrometheusRequest true "Prometheus deployment configuration"
// @Success 200 {object} types.MessageResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /metrics/deploy [post]
func (h *Handler) DeployPrometheus(w http.ResponseWriter, r *http.Request) error {
	var req types.DeployPrometheusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("Invalid request body", nil)
	}

	config := &common.Config{
		PrometheusVersion: req.PrometheusVersion,
		PrometheusPort:    req.PrometheusPort,
		ScrapeInterval:    time.Duration(req.ScrapeInterval) * time.Second,
	}

	if err := h.service.Start(r.Context(), config); err != nil {
		h.logger.Error("Failed to deploy Prometheus", "error", err)
		return errors.NewInternalError("Failed to deploy Prometheus", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, types.MessageResponse{Message: "Prometheus deployed successfully"})
}

// GetNodeMetrics retrieves metrics for a specific node
// @Summary Get metrics for a specific node
// @Description Retrieves metrics for a specific node by ID and optional PromQL query
// @Tags Metrics
// @Produce json
// @Param id path string true "Node ID"
// @Param query query string false "PromQL query to filter metrics"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /metrics/node/{id} [get]
func (h *Handler) GetNodeMetrics(w http.ResponseWriter, r *http.Request) error {
	nodeID := chi.URLParam(r, "id")
	if nodeID == "" {
		return errors.NewValidationError("Node ID is required", nil)
	}
	nodeIDInt, err := strconv.ParseInt(nodeID, 10, 64)
	if err != nil {
		return errors.NewValidationError("Invalid node ID", nil)
	}

	query := r.URL.Query().Get("query")

	metrics, err := h.service.QueryMetrics(r.Context(), nodeIDInt, query)
	if err != nil {
		h.logger.Error("Failed to get node metrics", "error", err)
		return errors.NewInternalError("Failed to get node metrics", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, metrics)
}

// ReloadConfiguration reloads the Prometheus configuration
// @Summary Reload Prometheus configuration
// @Description Triggers a reload of the Prometheus configuration to pick up any changes
// @Tags Metrics
// @Produce json
// @Success 200 {object} types.MessageResponse
// @Failure 500 {object} map[string]string
// @Router /metrics/reload [post]
func (h *Handler) ReloadConfiguration(w http.ResponseWriter, r *http.Request) error {
	if err := h.service.Reload(r.Context()); err != nil {
		h.logger.Error("Failed to reload Prometheus configuration", "error", err)
		return errors.NewInternalError("Failed to reload Prometheus configuration", err, nil)
	}
	return response.WriteJSON(w, http.StatusOK, types.MessageResponse{Message: "Prometheus configuration reloaded successfully"})
}

// @Summary Get label values for a specific label
// @Description Retrieves all values for a specific label, optionally filtered by metric matches and node ID
// @Tags Metrics
// @Accept json
// @Produce json
// @Param id path string true "Node ID"
// @Param label path string true "Label name"
// @Param match query array false "Metric matches (e.g. {__name__=\"metric_name\"})"
// @Success 200 {object} types.LabelValuesResponse "Label values"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /metrics/node/{id}/label/{label}/values [get]
func (h *Handler) GetLabelValues(w http.ResponseWriter, r *http.Request) error {
	nodeID := chi.URLParam(r, "id")
	if nodeID == "" {
		return errors.NewValidationError("Node ID is required", nil)
	}
	nodeIDInt, err := strconv.ParseInt(nodeID, 10, 64)
	if err != nil {
		return errors.NewValidationError("Invalid node ID", nil)
	}
	labelName := chi.URLParam(r, "label")
	if labelName == "" {
		return errors.NewValidationError("Label name is required", nil)
	}
	matches := r.URL.Query()["match"]
	values, err := h.service.GetLabelValues(r.Context(), nodeIDInt, labelName, matches)
	if err != nil {
		return errors.NewInternalError("Failed to get label values", err, nil)
	}
	return response.WriteJSON(w, http.StatusOK, types.LabelValuesResponse{
		Status: "success",
		Data:   values,
	})
}

// @Summary Get metrics for a specific node with time range
// @Description Retrieves metrics for a specific node within a specified time range
// @Tags Metrics
// @Accept json
// @Produce json
// @Param id path string true "Node ID"
// @Param query query string true "PromQL query"
// @Param start query string true "Start time (RFC3339 format)"
// @Param end query string true "End time (RFC3339 format)"
// @Param step query string true "Step duration (e.g. 1m, 5m, 1h)"
// @Success 200 {object} types.MetricsDataResponse "Metrics data"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /metrics/node/{id}/range [get]
func (h *Handler) GetNodeMetricsRange(w http.ResponseWriter, r *http.Request) error {
	nodeID := chi.URLParam(r, "id")
	if nodeID == "" {
		return errors.NewValidationError("Node ID is required", nil)
	}
	nodeIDInt, err := strconv.ParseInt(nodeID, 10, 64)
	if err != nil {
		return errors.NewValidationError("Invalid node ID", nil)
	}
	query := r.URL.Query().Get("query")
	if query == "" {
		return errors.NewValidationError("Query is required", nil)
	}
	startStr := r.URL.Query().Get("start")
	if startStr == "" {
		return errors.NewValidationError("Start time is required", nil)
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return errors.NewValidationError("Invalid start time format (use RFC3339)", nil)
	}
	endStr := r.URL.Query().Get("end")
	if endStr == "" {
		return errors.NewValidationError("End time is required", nil)
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return errors.NewValidationError("Invalid end time format (use RFC3339)", nil)
	}
	stepStr := r.URL.Query().Get("step")
	if stepStr == "" {
		return errors.NewValidationError("Step is required", nil)
	}
	step, err := time.ParseDuration(stepStr)
	if err != nil {
		return errors.NewValidationError("Invalid step duration", nil)
	}
	if end.Before(start) {
		return errors.NewValidationError("End time must be after start time", nil)
	}
	metrics, err := h.service.QueryMetricsRange(r.Context(), nodeIDInt, query, start, end, step)
	if err != nil {
		h.logger.Error("Failed to get node metrics range", "error", err)
		return errors.NewInternalError("Failed to get node metrics range", err, nil)
	}
	return response.WriteJSON(w, http.StatusOK, types.MetricsDataResponse{
		Status: "success",
		Data:   metrics,
	})
}

// CustomQuery executes a custom Prometheus query
// @Summary Execute custom Prometheus query
// @Description Execute a custom Prometheus query with optional time range
// @Tags Metrics
// @Accept json
// @Produce json
// @Param id path string true "Node ID"
// @Param request body types.CustomQueryRequest true "Query parameters"
// @Success 200 {object} common.QueryResult
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /metrics/node/{id}/query [post]
func (h *Handler) CustomQuery(w http.ResponseWriter, r *http.Request) error {
	nodeID := chi.URLParam(r, "id")
	if nodeID == "" {
		return errors.NewValidationError("Node ID is required", nil)
	}
	nodeIDInt, err := strconv.ParseInt(nodeID, 10, 64)
	if err != nil {
		return errors.NewValidationError("Invalid node ID", nil)
	}
	var req types.CustomQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("Invalid request body", nil)
	}
	if req.Start != nil && req.End != nil {
		step := 1 * time.Minute
		if req.Step != nil {
			var err error
			step, err = time.ParseDuration(*req.Step)
			if err != nil {
				return errors.NewValidationError("Invalid step duration", nil)
			}
		}
		result, err := h.service.QueryRange(r.Context(), nodeIDInt, req.Query, *req.Start, *req.End, step)
		if err != nil {
			h.logger.Error("Failed to execute range query", "error", err)
			return errors.NewInternalError("Failed to execute range query", err, nil)
		}
		return response.WriteJSON(w, http.StatusOK, result)
	}
	result, err := h.service.Query(r.Context(), nodeIDInt, req.Query)
	if err != nil {
		h.logger.Error("Failed to execute query", "error", err)
		return errors.NewInternalError("Failed to execute query", err, nil)
	}
	return response.WriteJSON(w, http.StatusOK, result)
}

// GetStatus returns the current status of the Prometheus instance
// @Summary Get Prometheus status
// @Description Returns the current status of the Prometheus instance including version, port, and configuration
// @Tags Metrics
// @Produce json
// @Success 200 {object} common.Status
// @Failure 500 {object} map[string]string
// @Router /metrics/status [get]
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) error {
	status, err := h.service.GetStatus(r.Context())
	if err != nil {
		h.logger.Error("Failed to get Prometheus status", "error", err)
		return errors.NewInternalError("Failed to get Prometheus status", err, nil)
	}
	return response.WriteJSON(w, http.StatusOK, status)
}

// UndeployPrometheus stops the Prometheus instance
// @Summary Undeploy Prometheus instance
// @Description Stops and removes the Prometheus instance
// @Tags Metrics
// @Produce json
// @Success 200 {object} types.MessageResponse
// @Failure 500 {object} map[string]string
// @Router /metrics/undeploy [post]
func (h *Handler) UndeployPrometheus(w http.ResponseWriter, r *http.Request) error {
	if err := h.service.Stop(r.Context()); err != nil {
		h.logger.Error("Failed to undeploy Prometheus", "error", err)
		return errors.NewInternalError("Failed to undeploy Prometheus", err, nil)
	}
	return response.WriteJSON(w, http.StatusOK, types.MessageResponse{Message: "Prometheus undeployed successfully"})
}
