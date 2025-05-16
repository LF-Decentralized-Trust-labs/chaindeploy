package metrics

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/metrics/common"
	"github.com/go-chi/chi/v5"
)

// Node represents a node to be monitored
type Node struct {
	ID      string
	Address string
	Port    int
}

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
	r.Route("/api/v1/metrics", func(r chi.Router) {
		r.Post("/deploy", h.DeployPrometheus)
		r.Get("/node/{id}", h.GetNodeMetrics)
		r.Post("/reload", h.ReloadConfiguration)
		r.Get("/node/{id}/label/{label}/values", h.GetLabelValues)
		r.Get("/node/{id}/range", h.GetNodeMetricsRange)
		r.Post("/node/{id}/query", h.CustomQuery)
	})
}

// DeployPrometheusRequest represents the request to deploy Prometheus
type DeployPrometheusRequest struct {
	PrometheusVersion string `json:"prometheus_version" binding:"required"`
	PrometheusPort    int    `json:"prometheus_port" binding:"required"`
	ScrapeInterval    int    `json:"scrape_interval" binding:"required"`
	DeploymentMode    string `json:"deployment_mode" binding:"required"`
}

// DeployPrometheus deploys a new Prometheus instance
// @Summary Deploy a new Prometheus instance
// @Description Deploys a new Prometheus instance with the specified configuration
// @Tags metrics
// @Accept json
// @Produce json
// @Param request body DeployPrometheusRequest true "Prometheus deployment configuration"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/metrics/deploy [post]
func (h *Handler) DeployPrometheus(w http.ResponseWriter, r *http.Request) {
	var req DeployPrometheusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	config := &common.Config{
		PrometheusVersion: req.PrometheusVersion,
		PrometheusPort:    req.PrometheusPort,
		ScrapeInterval:    time.Duration(req.ScrapeInterval) * time.Second,
		DeploymentMode:    req.DeploymentMode,
	}

	if err := h.service.Start(r.Context(), config); err != nil {
		h.logger.Error("Failed to deploy Prometheus", "error", err)
		http.Error(w, "Failed to deploy Prometheus", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Prometheus deployed successfully"})
}

// RefreshNodesRequest represents the request to refresh nodes
type RefreshNodesRequest struct {
	Nodes []struct {
		ID      string `json:"id" binding:"required"`
		Address string `json:"address" binding:"required"`
		Port    int    `json:"port" binding:"required"`
	} `json:"nodes" binding:"required"`
}

// GetNodeMetrics retrieves metrics for a specific node
// @Summary Get metrics for a specific node
// @Description Retrieves metrics for a specific node by ID and optional PromQL query
// @Tags metrics
// @Produce json
// @Param id path string true "Node ID"
// @Param query query string false "PromQL query to filter metrics"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/metrics/node/{id} [get]
func (h *Handler) GetNodeMetrics(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "id")
	if nodeID == "" {
		http.Error(w, "Node ID is required", http.StatusBadRequest)
		return
	}
	nodeIDInt, err := strconv.ParseInt(nodeID, 10, 64)
	if err != nil {
		http.Error(w, "invalid node ID", http.StatusBadRequest)
		return
	}

	// Get PromQL query from query parameter
	query := r.URL.Query().Get("query")

	metrics, err := h.service.QueryMetrics(r.Context(), nodeIDInt, query)
	if err != nil {
		h.logger.Error("Failed to get node metrics", "error", err)
		http.Error(w, "Failed to get node metrics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metrics)
}

// ReloadConfiguration reloads the Prometheus configuration
// @Summary Reload Prometheus configuration
// @Description Triggers a reload of the Prometheus configuration to pick up any changes
// @Tags metrics
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/metrics/reload [post]
func (h *Handler) ReloadConfiguration(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Reload(r.Context()); err != nil {
		h.logger.Error("Failed to reload Prometheus configuration", "error", err)
		http.Error(w, "Failed to reload Prometheus configuration", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Prometheus configuration reloaded successfully"})
}

// @Summary Get label values for a specific label
// @Description Retrieves all values for a specific label, optionally filtered by metric matches and node ID
// @Tags metrics
// @Accept json
// @Produce json
// @Param id path string true "Node ID"
// @Param label path string true "Label name"
// @Param match query array false "Metric matches (e.g. {__name__=\"metric_name\"})"
// @Success 200 {object} map[string]interface{} "Label values"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/metrics/node/{id}/label/{label}/values [get]
func (h *Handler) GetLabelValues(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "id")
	if nodeID == "" {
		http.Error(w, "node ID is required", http.StatusBadRequest)
		return
	}

	nodeIDInt, err := strconv.ParseInt(nodeID, 10, 64)
	if err != nil {
		http.Error(w, "invalid node ID", http.StatusBadRequest)
		return
	}

	labelName := chi.URLParam(r, "label")
	if labelName == "" {
		http.Error(w, "label name is required", http.StatusBadRequest)
		return
	}

	// Get matches from query parameters
	matches := r.URL.Query()["match"]

	values, err := h.service.GetLabelValues(r.Context(), nodeIDInt, labelName, matches)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   values,
	})
}

// @Summary Get metrics for a specific node with time range
// @Description Retrieves metrics for a specific node within a specified time range
// @Tags metrics
// @Accept json
// @Produce json
// @Param id path string true "Node ID"
// @Param query query string true "PromQL query"
// @Param start query string true "Start time (RFC3339 format)"
// @Param end query string true "End time (RFC3339 format)"
// @Param step query string true "Step duration (e.g. 1m, 5m, 1h)"
// @Success 200 {object} map[string]interface{} "Metrics data"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/metrics/node/{id}/range [get]
func (h *Handler) GetNodeMetricsRange(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "id")
	if nodeID == "" {
		http.Error(w, "node ID is required", http.StatusBadRequest)
		return
	}

	nodeIDInt, err := strconv.ParseInt(nodeID, 10, 64)
	if err != nil {
		http.Error(w, "invalid node ID", http.StatusBadRequest)
		return
	}

	// Get query parameters
	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "query is required", http.StatusBadRequest)
		return
	}

	startStr := r.URL.Query().Get("start")
	if startStr == "" {
		http.Error(w, "start time is required", http.StatusBadRequest)
		return
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		http.Error(w, "invalid start time format (use RFC3339)", http.StatusBadRequest)
		return
	}

	endStr := r.URL.Query().Get("end")
	if endStr == "" {
		http.Error(w, "end time is required", http.StatusBadRequest)
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		http.Error(w, "invalid end time format (use RFC3339)", http.StatusBadRequest)
		return
	}

	stepStr := r.URL.Query().Get("step")
	if stepStr == "" {
		http.Error(w, "step is required", http.StatusBadRequest)
		return
	}
	step, err := time.ParseDuration(stepStr)
	if err != nil {
		http.Error(w, "invalid step duration", http.StatusBadRequest)
		return
	}

	// Validate time range
	if end.Before(start) {
		http.Error(w, "end time must be after start time", http.StatusBadRequest)
		return
	}

	// Get metrics with time range
	metrics, err := h.service.QueryMetricsRange(r.Context(), nodeIDInt, query, start, end, step)
	if err != nil {
		h.logger.Error("Failed to get node metrics range", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   metrics,
	})
}

// CustomQueryRequest represents the request body for custom Prometheus queries
type CustomQueryRequest struct {
	Query string     `json:"query" binding:"required"`
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
	Step  *string    `json:"step,omitempty"`
}

// CustomQuery executes a custom Prometheus query
// @Summary Execute custom Prometheus query
// @Description Execute a custom Prometheus query with optional time range
// @Tags metrics
// @Accept json
// @Produce json
// @Param id path string true "Node ID"
// @Param request body CustomQueryRequest true "Query parameters"
// @Success 200 {object} common.QueryResult
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/metrics/node/{id}/query [post]
func (h *Handler) CustomQuery(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "id")
	if nodeID == "" {
		http.Error(w, "Node ID is required", http.StatusBadRequest)
		return
	}
	nodeIDInt, err := strconv.ParseInt(nodeID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	var req CustomQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// If time range parameters are provided, use QueryRange
	if req.Start != nil && req.End != nil {
		step := 1 * time.Minute // Default step
		if req.Step != nil {
			var err error
			step, err = time.ParseDuration(*req.Step)
			if err != nil {
				http.Error(w, "Invalid step duration: "+err.Error(), http.StatusBadRequest)
				return
			}
		}

		result, err := h.service.QueryRange(r.Context(), nodeIDInt, req.Query, *req.Start, *req.End, step)
		if err != nil {
			h.logger.Error("Failed to execute range query", "error", err)
			http.Error(w, "Failed to execute range query: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
		return
	}

	// Otherwise use regular Query
	result, err := h.service.Query(r.Context(), nodeIDInt, req.Query)
	if err != nil {
		h.logger.Error("Failed to execute query", "error", err)
		http.Error(w, "Failed to execute query: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}
