package plugin

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/errors"
	"github.com/chainlaunch/chainlaunch/pkg/http/response"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/plugin/registry"
	"github.com/chainlaunch/chainlaunch/pkg/plugin/types"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

func init() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())
}

// Handler handles HTTP requests for plugins
type Handler struct {
	store                 Store
	pm                    *PluginManager
	logger                *logger.Logger
	validate              *validator.Validate
	registry              *registry.Registry
	availablePluginsCache *registry.AvailablePluginsCache // cache for available plugins
}

// NewHandler creates a new plugin handler
func NewHandler(store Store, pm *PluginManager, logger *logger.Logger, registry *registry.Registry, availablePluginsCache *registry.AvailablePluginsCache) *Handler {
	return &Handler{
		store:                 store,
		pm:                    pm,
		logger:                logger,
		validate:              validator.New(),
		registry:              registry,
		availablePluginsCache: availablePluginsCache,
	}
}

// RegisterRoutes registers the plugin routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/plugins", func(r chi.Router) {
		r.Get("/", response.Middleware(h.listPlugins))
		r.Get("/available", response.Middleware(h.listAvailablePlugins))
		r.Post("/available/refresh", response.Middleware(h.refreshAvailablePlugins))
		r.Post("/", response.Middleware(h.createPlugin))
		r.Route("/{name}", func(r chi.Router) {
			r.Get("/", response.Middleware(h.getPlugin))
			r.Put("/", response.Middleware(h.updatePlugin))
			r.Delete("/", response.Middleware(h.deletePlugin))
			r.Post("/deploy", response.Middleware(h.deployPlugin))
			r.Post("/stop", response.Middleware(h.stopPlugin))
			r.Post("/resume", response.Middleware(h.resumePlugin))
			r.Get("/status", response.Middleware(h.getPluginStatus))
			r.Get("/deployment-status", response.Middleware(h.getDeploymentStatus))
			r.Get("/services", response.Middleware(h.getDockerComposeServices))
		})
	})
}

// @Summary List all plugins
// @Description Get a list of all available plugins
// @Tags Plugins
// @Accept json
// @Produce json
// @Success 200 {array} types.Plugin
// @Failure 500 {object} string
// @Router /plugins [get]
func (h *Handler) listPlugins(w http.ResponseWriter, r *http.Request) error {
	plugins, err := h.store.ListPlugins(r.Context())
	if err != nil {
		return errors.NewInternalError("failed to list plugins", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, plugins)
}

// @Summary Get a plugin
// @Description Get a specific plugin by name
// @Tags Plugins
// @Accept json
// @Produce json
// @Param name path string true "Plugin name"
// @Success 200 {object} types.Plugin
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /plugins/{name} [get]
func (h *Handler) getPlugin(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "name")
	plugin, err := h.store.GetPlugin(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.NewNotFoundError("plugin not found", map[string]interface{}{
				"detail":      "The requested plugin does not exist",
				"code":        "PLUGIN_NOT_FOUND",
				"plugin_name": name,
			})
		}
		return errors.NewInternalError("failed to get plugin", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, plugin)
}

// @Summary Create a plugin
// @Description Create a new plugin
// @Tags Plugins
// @Accept json
// @Produce json
// @Param plugin body types.Plugin true "Plugin to create"
// @Success 201 {object} types.Plugin
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /plugins [post]
func (h *Handler) createPlugin(w http.ResponseWriter, r *http.Request) error {
	var plugin types.Plugin
	if err := json.NewDecoder(r.Body).Decode(&plugin); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_REQUEST_BODY",
		})
	}

	if err := h.validate.Struct(plugin); err != nil {
		validationErrors := make(map[string]string)
		for _, err := range err.(validator.ValidationErrors) {
			validationErrors[err.Field()] = err.Tag()
		}
		return errors.NewValidationError("validation failed", map[string]interface{}{
			"detail": "Request validation failed",
			"code":   "VALIDATION_ERROR",
			"errors": validationErrors,
		})
	}

	if err := h.pm.ValidatePlugin(&plugin); err != nil {
		return errors.NewValidationError("invalid plugin", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_PLUGIN",
		})
	}

	if err := h.store.CreatePlugin(r.Context(), &plugin); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return errors.NewConflictError("plugin already exists", map[string]interface{}{
				"detail": err.Error(),
				"code":   "PLUGIN_ALREADY_EXISTS",
			})
		}
		return errors.NewInternalError("failed to create plugin", err, nil)
	}

	return response.WriteJSON(w, http.StatusCreated, plugin)
}

// @Summary Update a plugin
// @Description Update an existing plugin
// @Tags Plugins
// @Accept json
// @Produce json
// @Param name path string true "Plugin name"
// @Param plugin body types.Plugin true "Plugin to update"
// @Success 200 {object} types.Plugin
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /plugins/{name} [put]
func (h *Handler) updatePlugin(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "name")
	var plugin types.Plugin
	if err := json.NewDecoder(r.Body).Decode(&plugin); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_REQUEST_BODY",
		})
	}

	if plugin.Metadata.Name != name {
		return errors.NewValidationError("plugin name mismatch", map[string]interface{}{
			"detail": "Plugin name in URL does not match body",
			"code":   "NAME_MISMATCH",
		})
	}

	// Get existing plugin to validate it exists
	existingPlugin, err := h.store.GetPlugin(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.NewNotFoundError("plugin not found", map[string]interface{}{
				"detail":      "The requested plugin does not exist",
				"code":        "PLUGIN_NOT_FOUND",
				"plugin_name": name,
			})
		}
		return errors.NewInternalError("failed to get plugin", err, nil)
	}

	// Preserve deployment status
	plugin.DeploymentStatus = existingPlugin.DeploymentStatus

	if err := h.validate.Struct(plugin); err != nil {
		validationErrors := make(map[string]string)
		for _, err := range err.(validator.ValidationErrors) {
			validationErrors[err.Field()] = err.Tag()
		}
		return errors.NewValidationError("validation failed", map[string]interface{}{
			"detail": "Request validation failed",
			"code":   "VALIDATION_ERROR",
			"errors": validationErrors,
		})
	}

	if err := h.pm.ValidatePlugin(&plugin); err != nil {
		return errors.NewValidationError("invalid plugin", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_PLUGIN",
		})
	}

	if err := h.store.UpdatePlugin(r.Context(), &plugin); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.NewNotFoundError("plugin not found", map[string]interface{}{
				"detail":      "The requested plugin does not exist",
				"code":        "PLUGIN_NOT_FOUND",
				"plugin_name": name,
			})
		}
		return errors.NewInternalError("failed to update plugin", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, plugin)
}

// @Summary Delete a plugin
// @Description Delete a plugin (will stop it first if running)
// @Tags Plugins
// @Accept json
// @Produce json
// @Param name path string true "Plugin name"
// @Success 204
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /plugins/{name} [delete]
func (h *Handler) deletePlugin(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "name")

	// First, check if the plugin exists
	plugin, err := h.store.GetPlugin(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.NewNotFoundError("plugin not found", map[string]interface{}{
				"detail":      "The requested plugin does not exist",
				"code":        "PLUGIN_NOT_FOUND",
				"plugin_name": name,
			})
		}
		return errors.NewInternalError("failed to get plugin", err, nil)
	}

	// Check if plugin is currently running
	deploymentStatus, err := h.store.GetDeploymentStatus(r.Context(), name)
	if err != nil {
		// If we can't get deployment status, log it but continue with deletion
		h.logger.Warnf("Failed to get deployment status for plugin %s: %v", name, err)
	} else {
		// If plugin is running, stop it first
		if deploymentStatus == "deployed" || deploymentStatus == "deploying" {
			h.logger.Infof("Plugin %s is currently running, stopping it before deletion", name)

			if err := h.pm.StopPlugin(r.Context(), plugin, h.store); err != nil {
				h.logger.Errorf("Failed to stop plugin %s before deletion: %v", name, err)
				return errors.NewInternalError("failed to stop plugin before deletion", err, map[string]interface{}{
					"detail":      "Plugin is running and could not be stopped",
					"code":        "PLUGIN_STOP_FAILED",
					"plugin_name": name,
					"status":      deploymentStatus,
				})
			}

			h.logger.Infof("Successfully stopped plugin %s before deletion", name)
		}
	}

	// Now delete the plugin
	if err := h.store.DeletePlugin(r.Context(), name); err != nil {
		return errors.NewInternalError("failed to delete plugin", err, nil)
	}

	h.logger.Infof("Successfully deleted plugin %s", name)
	return response.WriteJSON(w, http.StatusNoContent, nil)
}

// @Summary Deploy a plugin
// @Description Deploy a plugin with the given parameters
// @Tags Plugins
// @Accept json
// @Produce json
// @Param name path string true "Plugin name"
// @Param parameters body map[string]interface{} true "Deployment parameters"
// @Success 200
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /plugins/{name}/deploy [post]
func (h *Handler) deployPlugin(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "name")
	plugin, err := h.store.GetPlugin(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.NewNotFoundError("plugin not found", map[string]interface{}{
				"detail":      "The requested plugin does not exist",
				"code":        "PLUGIN_NOT_FOUND",
				"plugin_name": name,
			})
		}
		return errors.NewInternalError("failed to get plugin", err, nil)
	}

	var parameters map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&parameters); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"detail": err.Error(),
			"code":   "INVALID_REQUEST_BODY",
		})
	}

	// Validate required parameters
	for _, required := range plugin.Spec.Parameters.Required {
		if _, ok := parameters[required]; !ok {
			return errors.NewValidationError("missing required parameter", map[string]interface{}{
				"detail":    "Required parameter is missing",
				"code":      "MISSING_PARAMETER",
				"parameter": required,
			})
		}
	}

	// Validate parameter types and values
	for name, value := range parameters {
		if spec, ok := plugin.Spec.Parameters.Properties[name]; ok {
			if spec.Type == "string" {
				if _, ok := value.(string); !ok {
					return errors.NewValidationError("invalid parameter type", map[string]interface{}{
						"detail":    "Parameter type mismatch",
						"code":      "INVALID_PARAMETER_TYPE",
						"parameter": name,
						"expected":  "string",
					})
				}
			}

			if len(spec.Enum) > 0 {
				valid := false
				for _, allowed := range spec.Enum {
					if value == allowed {
						valid = true
						break
					}
				}
				if !valid {
					return errors.NewValidationError("invalid parameter value", map[string]interface{}{
						"detail":    "Parameter value not in allowed values",
						"code":      "INVALID_PARAMETER_VALUE",
						"parameter": name,
						"allowed":   spec.Enum,
					})
				}
			}
		}
	}

	// Deploy plugin (x-source validation is handled in DeployPlugin)
	if err := h.pm.DeployPlugin(r.Context(), plugin, parameters, h.store); err != nil {
		if strings.Contains(err.Error(), "x-source parameter validation failed") {
			return errors.NewValidationError("invalid x-source parameter", map[string]interface{}{
				"detail": err.Error(),
				"code":   "INVALID_XSOURCE_PARAMETER",
			})
		}
		_ = h.store.UpdateDeploymentStatus(r.Context(), name, "failed")
		return errors.NewInternalError("failed to deploy plugin", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status": "deploying",
		"metadata": map[string]interface{}{
			"parameters":   parameters,
			"project_name": plugin.Metadata.Name + "-" + generateRandomSuffix(),
			"created_at":   time.Now().UTC(),
		},
	})
}

// Helper function to generate random suffix
func generateRandomSuffix() string {
	// Generate a random 6-character string
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// @Summary Stop a plugin deployment
// @Description Stop a running plugin deployment
// @Tags Plugins
// @Accept json
// @Produce json
// @Param name path string true "Plugin name"
// @Success 200
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /plugins/{name}/stop [post]
func (h *Handler) stopPlugin(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "name")
	plugin, err := h.store.GetPlugin(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.NewNotFoundError("plugin not found", map[string]interface{}{
				"detail":      "The requested plugin does not exist",
				"code":        "PLUGIN_NOT_FOUND",
				"plugin_name": name,
			})
		}
		return errors.NewInternalError("failed to get plugin", err, nil)
	}

	if err := h.pm.StopPlugin(r.Context(), plugin, h.store); err != nil {
		return errors.NewInternalError("failed to stop plugin", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, map[string]string{
		"status": "stopped",
	})
}

// @Summary Get plugin deployment status
// @Description Get the current status of a plugin deployment
// @Tags Plugins
// @Accept json
// @Produce json
// @Param name path string true "Plugin name"
// @Success 200 {object} types.DeploymentStatus
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /plugins/{name}/status [get]
func (h *Handler) getPluginStatus(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "name")
	plugin, err := h.store.GetPlugin(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.NewNotFoundError("plugin not found", map[string]interface{}{
				"detail":      "The requested plugin does not exist",
				"code":        "PLUGIN_NOT_FOUND",
				"plugin_name": name,
			})
		}
		return errors.NewInternalError("failed to get plugin", err, nil)
	}

	status, err := h.pm.GetPluginStatus(r.Context(), plugin)
	if err != nil {
		return errors.NewInternalError("failed to get plugin status", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, status)
}

// @Summary Get detailed deployment status
// @Description Get detailed information about a plugin deployment including service status, logs, and metrics
// @Tags Plugins
// @Accept json
// @Produce json
// @Param name path string true "Plugin name"
// @Success 200 {object} types.DeploymentStatus
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /plugins/{name}/deployment-status [get]
func (h *Handler) getDeploymentStatus(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "name")
	plugin, err := h.store.GetPlugin(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.NewNotFoundError("plugin not found", map[string]interface{}{
				"detail":      "The requested plugin does not exist",
				"code":        "PLUGIN_NOT_FOUND",
				"plugin_name": name,
			})
		}
		return errors.NewInternalError("failed to get plugin", err, nil)
	}

	status, err := h.pm.GetDeploymentStatus(r.Context(), plugin, h.store)
	if err != nil {
		return errors.NewInternalError("failed to get deployment status", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, status)
}

// @Summary Get Docker Compose services
// @Description Get all services defined in the plugin's docker-compose configuration
// @Tags Plugins
// @Accept json
// @Produce json
// @Param name path string true "Plugin name"
// @Success 200 {array} ServiceStatus
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /plugins/{name}/services [get]
func (h *Handler) getDockerComposeServices(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "name")
	plugin, err := h.store.GetPlugin(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.NewNotFoundError("plugin not found", map[string]interface{}{
				"detail":      "The requested plugin does not exist",
				"code":        "PLUGIN_NOT_FOUND",
				"plugin_name": name,
			})
		}
		return errors.NewInternalError("failed to get plugin", err, nil)
	}

	services, err := h.pm.GetDockerComposeServices(r.Context(), plugin, h.store)
	if err != nil {
		return errors.NewInternalError("failed to get docker-compose services", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, services)
}

// @Summary Resume a plugin deployment
// @Description Resume a previously deployed plugin
// @Tags Plugins
// @Accept json
// @Produce json
// @Param name path string true "Plugin name"
// @Success 200 {object} map[string]string
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /plugins/{name}/resume [post]
func (h *Handler) resumePlugin(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "name")
	plugin, err := h.store.GetPlugin(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.NewNotFoundError("plugin not found", map[string]interface{}{
				"detail":      "The requested plugin does not exist",
				"code":        "PLUGIN_NOT_FOUND",
				"plugin_name": name,
			})
		}
		return errors.NewInternalError("failed to get plugin", err, nil)
	}

	if err := h.pm.ResumePlugin(r.Context(), plugin, h.store); err != nil {
		return errors.NewInternalError("failed to resume plugin", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, map[string]string{
		"status": "resumed",
	})
}

// @Summary List available plugins from GitHub sources
// @Description Get a list of all available plugins from configured GitHub repositories
// @Tags Plugins
// @Accept json
// @Produce json
// @Success 200 {object} AvailablePluginsResponse
// @Failure 500 {object} response.Response
// @Router /plugins/available [get]
func (h *Handler) listAvailablePlugins(w http.ResponseWriter, r *http.Request) error {
	plugins, lastUpdated := h.availablePluginsCache.Get()
	resp := AvailablePluginsResponse{
		Plugins:     plugins,
		LastUpdated: lastUpdated.Format(time.RFC3339),
	}
	return response.WriteJSON(w, http.StatusOK, resp)
}

type AvailablePluginsResponse struct {
	Plugins     []registry.PluginMetadata `json:"plugins"`
	LastUpdated string                    `json:"last_updated"`
}

// @Summary Refresh available plugins
// @Description Triggers a refresh of the available plugins cache from GitHub sources
// @Tags Plugins
// @Accept json
// @Produce json
// @Success 200 {object} AvailablePluginsResponse
// @Failure 500 {object} response.Response
// @Router /plugins/available/refresh [post]
func (h *Handler) refreshAvailablePlugins(w http.ResponseWriter, r *http.Request) error {
	plugins, err := h.registry.ListAvailablePluginsFromGitHub()
	if err != nil {
		return errors.NewInternalError("failed to refresh available plugins from GitHub", err, nil)
	}
	h.availablePluginsCache.Set(plugins)
	resp := AvailablePluginsResponse{
		Plugins:     plugins,
		LastUpdated: time.Now().Format(time.RFC3339),
	}
	return response.WriteJSON(w, http.StatusOK, resp)
}
