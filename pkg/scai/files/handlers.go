package files

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/chainlaunch/chainlaunch/pkg/errors"
	"github.com/chainlaunch/chainlaunch/pkg/http/response"
	"github.com/chainlaunch/chainlaunch/pkg/scai/projects"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// NewFilesHandler creates a new instance of FilesHandler
func NewFilesHandler(service *FilesService, projectsService *projects.ProjectsService) *FilesHandler {
	return &FilesHandler{
		Service:         service,
		ProjectsService: projectsService,
	}
}

type FilesHandler struct {
	Service         *FilesService
	ProjectsService *projects.ProjectsService
}

type ListFilesResponse struct {
	Files []string `json:"files" example:"[\"main.go\",\"README.md\"]" description:"List of file names"`
}

type ReadFileResponse struct {
	Content string `json:"content" example:"file contents" description:"File contents as string"`
}

type WriteFileRequest struct {
	Project string `json:"project" example:"myproject" description:"Project name"`
	Path    string `json:"path" example:"main.go" description:"File path relative to project root"`
	Content string `json:"content" example:"new file contents" description:"New file contents as string"`
}

type WriteFileResponse struct {
	Status string `json:"status" example:"written" description:"Status message"`
}

type DeleteFileRequest struct {
	Project string `json:"project" example:"myproject" description:"Project name"`
	Path    string `json:"path" example:"main.go" description:"File path relative to project root"`
}

type DeleteFileResponse struct {
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

// DirectoryTreeNode represents a node in the directory tree
// swagger:model
type DirectoryTreeNode struct {
	Name     string               `json:"name"`
	Path     string               `json:"path"`
	IsDir    bool                 `json:"isDir"`
	Children []*DirectoryTreeNode `json:"children,omitempty"`
}

// RegisterRoutes registers file endpoints to the router, now project-scoped
func (h *FilesHandler) RegisterRoutes(r chi.Router) {
	r.Route("/projects/{projectId}/files", func(r chi.Router) {
		r.Get("/read", response.Middleware(h.ReadFile))
		r.Post("/write", response.Middleware(h.WriteFile))
		r.Delete("/delete", response.Middleware(h.DeleteFile))
		r.Get("/list", response.Middleware(h.ListFiles))
		r.Get("/entries", response.Middleware(h.ListEntries))
	})
}

func (h *FilesHandler) getProjectRoot(r *http.Request) (string, error) {
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

// ListFiles godoc
// @Summary      List files
// @Description  List files in a given project and directory
// @Tags         files
// @Accept       json
// @Produce      json
// @Param        projectId path int true "Project ID"
// @Param        dir     query string false "Directory to list, relative to project root"
// @Success      200 {object} ListFilesResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      403 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Failure      422 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /projects/{projectId}/files/list [get]
func (h *FilesHandler) ListFiles(w http.ResponseWriter, r *http.Request) error {
	projectRoot, err := h.getProjectRoot(r)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		dir = "."
	}
	files, err := h.Service.ListFiles(projectRoot, dir)
	if err != nil {
		return errors.NewInternalError("failed to list files", err, nil)
	}

	zap.L().Info("listed files", zap.String("projectRoot", projectRoot), zap.String("dir", dir), zap.Int("count", len(files)))
	return response.WriteJSON(w, http.StatusOK, ListFilesResponse{Files: files})
}

// ReadFile godoc
// @Summary      Read file contents
// @Description  Get the contents of a file in a project
// @Tags         files
// @Accept       json
// @Produce      json
// @Param        projectId path int true "Project ID"
// @Param        path    query string true "File path relative to project root"
// @Success      200 {object} ReadFileResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      403 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Failure      422 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /projects/{projectId}/files/read [get]
func (h *FilesHandler) ReadFile(w http.ResponseWriter, r *http.Request) error {
	projectRoot, err := h.getProjectRoot(r)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		return errors.NewValidationError("path is required", nil)
	}
	// Example: forbidden file
	if path == "forbidden.txt" {
		return errors.NewAuthorizationError("access to this file is forbidden", nil)
	}
	// Example: not found
	if path == "notfound.txt" {
		return errors.NewNotFoundError("file not found", nil)
	}
	content, err := h.Service.ReadFile(projectRoot, path)
	if err != nil {
		return errors.NewInternalError("failed to read file", err, nil)
	}

	zap.L().Info("read file", zap.String("projectRoot", projectRoot), zap.String("path", path), zap.Int("size", len(content)))
	return response.WriteJSON(w, http.StatusOK, ReadFileResponse{Content: string(content)})
}

// WriteFile godoc
// @Summary      Write file contents
// @Description  Write or modify the contents of a file in a project
// @Tags         files
// @Accept       json
// @Produce      json
// @Param        projectId path int true "Project ID"
// @Param        request body WriteFileRequest true "File write info"
// @Success      201 {object} WriteFileResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      403 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Failure      422 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /projects/{projectId}/files/write [post]
func (h *FilesHandler) WriteFile(w http.ResponseWriter, r *http.Request) error {
	projectRoot, err := h.getProjectRoot(r)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}
	var req WriteFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"error": err.Error(),
		})
	}
	if req.Path == "" {
		return errors.NewValidationError("path is required", nil)
	}
	// Example: forbidden file
	if req.Path == "forbidden.txt" {
		return errors.NewAuthorizationError("writing to this file is forbidden", nil)
	}
	// Example: conflict (file already exists)
	if req.Path == "conflict.txt" {
		return errors.NewConflictError("file already exists", nil)
	}
	if err := h.Service.WriteFile(projectRoot, req.Path, []byte(req.Content)); err != nil {
		return errors.NewInternalError("failed to write file", err, nil)
	}

	zap.L().Info("wrote file", zap.String("projectRoot", projectRoot), zap.String("path", req.Path), zap.Int("size", len(req.Content)))
	return response.WriteJSON(w, http.StatusCreated, WriteFileResponse{Status: "written"})
}

// DeleteFile godoc
// @Summary      Delete a file
// @Description  Delete a file in a project
// @Tags         files
// @Accept       json
// @Produce      json
// @Param        projectId path int true "Project ID"
// @Param        path    query string true "File path relative to project root"
// @Success      200 {object} DeleteFileResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      403 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Failure      422 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /projects/{projectId}/files/delete [delete]
func (h *FilesHandler) DeleteFile(w http.ResponseWriter, r *http.Request) error {
	projectRoot, err := h.getProjectRoot(r)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		return errors.NewValidationError("path is required", nil)
	}
	// Example: forbidden file
	if path == "forbidden.txt" {
		return errors.NewAuthorizationError("deleting this file is forbidden", nil)
	}
	// Example: not found
	if path == "notfound.txt" {
		return errors.NewNotFoundError("file not found", nil)
	}
	if err := h.Service.DeleteFile(projectRoot, path); err != nil {
		return errors.NewInternalError("failed to delete file", err, nil)
	}

	zap.L().Info("deleted file", zap.String("projectRoot", projectRoot), zap.String("path", path))
	return response.WriteJSON(w, http.StatusOK, DeleteFileResponse{Status: "deleted"})
}

// ListEntries godoc
// @Summary      List full project directory tree
// @Description  List the full directory tree for a project, excluding large/ignored folders (e.g., node_modules, .git)
// @Tags         files
// @Produce      json
// @Param        projectId path int true "Project ID"
// @Success      200 {object} DirectoryTreeNode
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      403 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Failure      422 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /projects/{projectId}/files/entries [get]
func (h *FilesHandler) ListEntries(w http.ResponseWriter, r *http.Request) error {
	projectRoot, err := h.getProjectRoot(r)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}
	tree, err := buildDirectoryTree(projectRoot, projectRoot)
	if err != nil {
		return errors.NewInternalError("failed to build directory tree", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, tree)
}

// buildDirectoryTree recursively builds the directory tree, excluding ignored folders
func buildDirectoryTree(root, current string) (*DirectoryTreeNode, error) {
	ignored := map[string]bool{
		"node_modules": true,
		".git":         true,
		".DS_Store":    true,
	}
	info, err := os.Stat(current)
	if err != nil {
		return nil, err
	}
	relPath, _ := filepath.Rel(root, current)
	node := &DirectoryTreeNode{
		Name:  info.Name(),
		Path:  relPath,
		IsDir: info.IsDir(),
	}
	if !info.IsDir() {
		return node, nil
	}
	entries, err := os.ReadDir(current)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if ignored[entry.Name()] {
			continue
		}
		childPath := filepath.Join(current, entry.Name())
		childNode, err := buildDirectoryTree(root, childPath)
		if err == nil {
			node.Children = append(node.Children, childNode)
		}
	}
	return node, nil
}
