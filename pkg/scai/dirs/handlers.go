package dirs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/chainlaunch/chainlaunch/pkg/errors"
	"github.com/chainlaunch/chainlaunch/pkg/http/response"
	"github.com/chainlaunch/chainlaunch/pkg/scai/projects"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// NewDirsHandler creates a new instance of DirsHandler
func NewDirsHandler(service *DirsService, projectsService *projects.ProjectsService) *DirsHandler {
	return &DirsHandler{
		Service:         service,
		ProjectsService: projectsService,
	}
}

type DirsHandler struct {
	Service         *DirsService
	ProjectsService *projects.ProjectsService
}

type CreateDirRequest struct {
	Project string `json:"project" example:"myproject" description:"Project name"`
	Dir     string `json:"dir" example:"newdir" description:"Directory to create, relative to project root"`
}

type CreateDirResponse struct {
	Status string `json:"status" example:"created" description:"Status message"`
}

type DeleteDirRequest struct {
	Project string `json:"project" example:"myproject" description:"Project name"`
	Dir     string `json:"dir" example:"olddir" description:"Directory to delete, relative to project root"`
}

type DeleteDirResponse struct {
	Status string `json:"status" example:"deleted" description:"Status message"`
}

// ListEntriesResponse is a unified response for listing both files and directories
// @Description Unified response for listing files and directories in a directory
// @Success      200 {object} ListEntriesResponse
// @Failure      400 {object} errors.ErrorResponse
// @Failure      401 {object} errors.ErrorResponse
// @Failure      403 {object} errors.ErrorResponse
// @Failure      404 {object} errors.ErrorResponse
// @Failure      409 {object} errors.ErrorResponse
// @Failure      422 {object} errors.ErrorResponse
// @Failure      500 {object} errors.ErrorResponse
// @Router       /entries/list [get]
type ListEntriesResponse struct {
	Files       []string `json:"files" example:"[\"main.go\",\"README.md\"]" description:"List of file names"`
	Directories []string `json:"directories" example:"[\"src\",\"docs\"]" description:"List of directory names"`
	Skipped     []string `json:"skipped,omitempty" example:"[\"node_modules\"]" description:"Directories skipped due to size or policy"`
}

// RegisterRoutes registers directory endpoints to the router, now project-scoped
func (h *DirsHandler) RegisterRoutes(r chi.Router) {
	r.Route("/projects/{projectId}/dirs", func(r chi.Router) {
		r.Post("/create", response.Middleware(h.CreateDir))
		r.Delete("/delete", response.Middleware(h.DeleteDir))
		r.Get("/list", response.Middleware(h.ListEntries))
	})
}

func (h *DirsHandler) getProjectRoot(r *http.Request) (string, error) {
	projectIdStr := chi.URLParam(r, "projectId")
	if projectIdStr == "" {
		return "", fmt.Errorf("projectId is required")
	}
	projectId, err := strconv.ParseInt(projectIdStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid projectId")
	}
	proj, err := h.ProjectsService.GetProject(r.Context(), projectId)
	if err != nil {
		return "", fmt.Errorf("project not found: %w", err)
	}
	return h.ProjectsService.GetProjectDirectory(proj)
}

// CreateDir godoc
// @Summary      Create a directory
// @Description  Create a new directory in a project
// @Tags         directories
// @Accept       json
// @Produce      json
// @Param        projectId path int true "Project ID"
// @Param        request body CreateDirRequest true "Directory create info"
// @Success      201 {object} CreateDirResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      403 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Failure      422 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /projects/{projectId}/dirs/create [post]
func (h *DirsHandler) CreateDir(w http.ResponseWriter, r *http.Request) error {
	var req CreateDirRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"error": err.Error(),
		})
	}
	if req.Dir == "" {
		return errors.NewValidationError("dir is required", nil)
	}
	projectRoot, err := h.getProjectRoot(r)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}
	// Example: forbidden directory name
	if req.Dir == "forbidden" {
		return errors.NewAuthorizationError("directory name is forbidden", nil)
	}
	// Example: conflict (directory already exists)
	if req.Dir == "conflict" {
		return errors.NewConflictError("directory already exists", nil)
	}
	if err := h.Service.CreateDir(projectRoot, req.Dir); err != nil {
		return errors.NewInternalError("failed to create directory", err, nil)
	}

	zap.L().Info("created dir", zap.String("projectRoot", projectRoot), zap.String("dir", req.Dir))
	return response.WriteJSON(w, http.StatusCreated, CreateDirResponse{Status: "created"})
}

// DeleteDir godoc
// @Summary      Delete a directory
// @Description  Delete a directory in a project
// @Tags         directories
// @Accept       json
// @Produce      json
// @Param        projectId path int true "Project ID"
// @Param        project query string true "Project name"
// @Param        dir     query string true "Directory to delete, relative to project root"
// @Success      200 {object} DeleteDirResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      403 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Failure      422 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /projects/{projectId}/dirs/delete [delete]
func (h *DirsHandler) DeleteDir(w http.ResponseWriter, r *http.Request) error {
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		return errors.NewValidationError("dir is required", nil)
	}
	projectRoot, err := h.getProjectRoot(r)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}
	if err := h.Service.DeleteDir(projectRoot, dir); err != nil {
		return errors.NewInternalError("failed to delete directory", err, nil)
	}

	zap.L().Info("deleted dir", zap.String("projectRoot", projectRoot), zap.String("dir", dir))
	return response.WriteJSON(w, http.StatusOK, DeleteDirResponse{Status: "deleted"})
}

// ListEntries godoc
// @Summary      List files and directories
// @Description  List files and directories in a given project and directory. Large directories (e.g., node_modules) are summarized/skipped.
// @Tags         directories
// @Produce      json
// @Param        projectId path int true "Project ID"
// @Param        dir     query string false "Directory to list, relative to project root"
// @Success      200 {object} ListEntriesResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      403 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Failure      422 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /projects/{projectId}/dirs/list [get]
func (h *DirsHandler) ListEntries(w http.ResponseWriter, r *http.Request) error {
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		dir = "."
	}
	projectRoot, err := h.getProjectRoot(r)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}
	files, dirs, skipped, err := h.Service.ListEntries(projectRoot, dir)
	if err != nil {
		return errors.NewInternalError("failed to list entries", err, nil)
	}

	zap.L().Info("listed entries", zap.String("projectRoot", projectRoot), zap.String("dir", dir), zap.Int("files", len(files)), zap.Int("dirs", len(dirs)), zap.Int("skipped", len(skipped)))
	return response.WriteJSON(w, http.StatusOK, ListEntriesResponse{Files: files, Directories: dirs, Skipped: skipped})
}
