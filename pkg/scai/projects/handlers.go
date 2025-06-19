package projects

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/chainlaunch/chainlaunch/pkg/errors"
	"github.com/chainlaunch/chainlaunch/pkg/http/response"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/scai/versionmanagement"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func NewProjectsHandler(service *ProjectsService, root string, chaincodeService *ChaincodeService, logger *logger.Logger) *ProjectsHandler {
	return &ProjectsHandler{
		Root:             root,
		Service:          service,
		chaincodeService: chaincodeService,
		logger:           logger,
	}
}

type ProjectsHandler struct {
	Root             string
	Service          *ProjectsService
	chaincodeService *ChaincodeService
	logger           *logger.Logger
}

type CreateProjectRequest struct {
	Name              string `json:"name" validate:"required" example:"myproject" description:"Project name"`
	Description       string `json:"description" example:"A sample project" description:"Project description"`
	Boilerplate       string `json:"boilerplate" example:"go-basic" description:"Boilerplate template to use for scaffolding"`
	NetworkID         *int64 `json:"networkId,omitempty" example:"1" description:"ID of the network to link with"`
	EndorsementPolicy string `json:"endorsementPolicy,omitempty" example:"OR('Org1MSP.member','Org2MSP.member')" description:"Endorsement policy for the chaincode"`
}

type UpdateProjectEndorsementPolicyRequest struct {
	EndorsementPolicy string `json:"endorsementPolicy" validate:"required" example:"OR('Org1MSP.member','Org2MSP.member')" description:"Endorsement policy for the chaincode"`
}

type CreateProjectResponse struct {
	ID                int64  `json:"id" example:"1" description:"Project ID"`
	Name              string `json:"name" example:"myproject" description:"Project name"`
	Slug              string `json:"slug" example:"myproject-abc12" description:"Project slug (used for proxying and folder name)"`
	Description       string `json:"description" example:"A sample project" description:"Project description"`
	Boilerplate       string `json:"boilerplate" example:"go-basic" description:"Boilerplate template used for scaffolding"`
	ContainerPort     *int   `json:"containerPort,omitempty" description:"Host port mapped to the container, if running"`
	NetworkID         *int64 `json:"networkId,omitempty" description:"ID of the linked network"`
	EndorsementPolicy string `json:"endorsementPolicy,omitempty" example:"OR('Org1MSP.member','Org2MSP.member')" description:"Endorsement policy for the chaincode"`
}

type ListProjectsResponse struct {
	Projects []Project `json:"projects"`
}

// CommitWithFileChangesAPI is the API response struct for a commit with file changes
// (mirrors versionmanagement.CommitWithFileChanges)
type CommitWithFileChangesAPI struct {
	Hash      string   `json:"hash"`
	Author    string   `json:"author"`
	Timestamp string   `json:"timestamp"`
	Message   string   `json:"message"`
	Added     []string `json:"added"`
	Removed   []string `json:"removed"`
	Modified  []string `json:"modified"`
	Parent    *string  `json:"parent"`
}

// CommitDetailAPI is the API response struct for a single commit with file changes
// (mirrors versionmanagement.CommitWithFileChanges)
type CommitDetailAPI struct {
	Hash      string   `json:"hash"`
	Author    string   `json:"author"`
	Timestamp string   `json:"timestamp"`
	Message   string   `json:"message"`
	Added     []string `json:"added"`
	Removed   []string `json:"removed"`
	Modified  []string `json:"modified"`
	Parent    *string  `json:"parent"`
}

// CommitsListResponse is the API response struct for a list of commits
// Used for OpenAPI/Swagger documentation
// swagger:model
type CommitsListResponse struct {
	Commits []CommitWithFileChangesAPI `json:"commits"`
}

// RegisterRoutes registers project endpoints to the router
func (h *ProjectsHandler) RegisterRoutes(r chi.Router) {
	r.Route("/chaincode-projects", func(r chi.Router) {
		r.Post("/", response.Middleware(h.CreateProject))
		r.Get("/", response.Middleware(h.ListProjects))
		r.Get("/{id}", response.Middleware(h.GetProject))
		r.Delete("/{id}", response.Middleware(h.DeleteProject))
		r.Post("/{id}/start", response.Middleware(h.StartProjectServer))
		r.Post("/{id}/stop", response.Middleware(h.StopProjectServer))
		r.Get("/{id}/logs", response.Middleware(h.GetProjectLogs))
		r.Get("/{id}/logs/stream", response.Middleware(h.StreamProjectLogs))
		r.Get("/{id}/commits", response.Middleware(h.GetProjectCommits))
		r.Get("/{id}/commits/{commitHash}", response.Middleware(h.GetProjectCommitDetail))
		r.Get("/{id}/diff", response.Middleware(h.GetProjectFileDiff))
		r.Get("/{id}/file_at_commit", response.Middleware(h.GetProjectFileAtCommit))
		r.Post("/{id}/invoke", response.Middleware(h.InvokeTransaction))
		r.Post("/{id}/query", response.Middleware(h.QueryTransaction))
		r.Put("/{id}/endorsement-policy", response.Middleware(h.UpdateProjectEndorsementPolicy))
	})
}

// CreateProject godoc
// @Summary      Create a project
// @Description  Create a new project, scaffold its directory, and store it in the DB
// @Tags         Chaincode Projects
// @Accept       json
// @Produce      json
// @Param        request body CreateProjectRequest true "Project info"
// @Success      201 {object} CreateProjectResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      401 {object} response.ErrorResponse
// @Failure      403 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      409 {object} response.ErrorResponse
// @Failure      422 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /chaincode-projects [post]
func (h *ProjectsHandler) CreateProject(w http.ResponseWriter, r *http.Request) error {
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"error": err.Error(),
		})
	}

	if req.Name == "" {
		return errors.NewValidationError("name is required", nil)
	}

	// Example: check for forbidden name
	if req.Name == "forbidden" {
		return errors.NewAuthorizationError("project name is forbidden", nil)
	}

	// Example: check for conflict (duplicate project)
	if req.Name == "conflict" {
		return errors.NewConflictError("project already exists", nil)
	}

	proj, err := h.Service.CreateProject(r.Context(), req.Name, req.Description, req.Boilerplate, req.NetworkID, req.EndorsementPolicy)
	if err != nil {
		return errors.NewInternalError("failed to create project", err, nil)
	}

	zap.L().Info("created project", zap.Int64("id", proj.ID), zap.String("name", proj.Name), zap.String("request_id", middleware.GetReqID(r.Context())))

	return response.WriteJSON(w, http.StatusCreated, CreateProjectResponse{
		ID:                proj.ID,
		Name:              proj.Name,
		Slug:              proj.Slug,
		Description:       proj.Description,
		Boilerplate:       proj.Boilerplate,
		ContainerPort:     proj.ContainerPort,
		NetworkID:         proj.NetworkID,
		EndorsementPolicy: proj.EndorsementPolicy,
	})
}

// ListProjects godoc
// @Summary      List all projects
// @Description  Get a list of all projects
// @Tags         Chaincode Projects
// @Produce      json
// @Success      200 {object} ListProjectsResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /chaincode-projects [get]
func (h *ProjectsHandler) ListProjects(w http.ResponseWriter, r *http.Request) error {
	projs, err := h.Service.ListProjects(r.Context())
	if err != nil {
		return errors.NewInternalError("failed to list projects", err, nil)
	}

	zap.L().Info("listed projects", zap.Int("count", len(projs)), zap.String("request_id", middleware.GetReqID(r.Context())))

	return response.WriteJSON(w, http.StatusOK, ListProjectsResponse{Projects: projs})
}

// GetProject godoc
// @Summary      Get a project by ID
// @Description  Get details of a project by its ID
// @Tags         Chaincode Projects
// @Produce      json
// @Param        id path int true "Project ID"
// @Success      200 {object} Project
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /chaincode-projects/{id} [get]
func (h *ProjectsHandler) GetProject(w http.ResponseWriter, r *http.Request) error {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}

	proj, err := h.Service.GetProject(r.Context(), id)
	if err != nil {
		if err == ErrNotFound {
			return errors.NewNotFoundError("project not found", nil)
		}
		return errors.NewInternalError("failed to get project", err, nil)
	}

	zap.L().Info("got project", zap.Int64("id", proj.ID), zap.String("name", proj.Name), zap.String("request_id", middleware.GetReqID(r.Context())))

	return response.WriteJSON(w, http.StatusOK, proj)
}

// DeleteProject godoc
// @Summary      Delete a project
// @Description  Delete a project and its associated files from the system
// @Tags         Chaincode Projects
// @Produce      json
// @Param        id path int true "Project ID"
// @Success      200 {object} map[string]string
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /chaincode-projects/{id} [delete]
func (h *ProjectsHandler) DeleteProject(w http.ResponseWriter, r *http.Request) error {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}

	err = h.Service.DeleteProject(r.Context(), id)
	if err != nil {
		if err == ErrNotFound {
			return errors.NewNotFoundError("project not found", nil)
		}
		return errors.NewInternalError("failed to delete project", err, nil)
	}

	zap.L().Info("deleted project", zap.Int64("id", id), zap.String("request_id", middleware.GetReqID(r.Context())))

	return response.WriteJSON(w, http.StatusOK, map[string]string{
		"status": "project deleted successfully",
	})
}

// StartProjectServer godoc
// @Summary      Start the server for a project
// @Description  Start the server process for a given project using its boilerplate
// @Tags         Chaincode Projects
// @Produce      json
// @Param        id path int true "Project ID"
// @Success      200 {object} map[string]string
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /chaincode-projects/{id}/start [post]
func (h *ProjectsHandler) StartProjectServer(w http.ResponseWriter, r *http.Request) error {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}

	proj, err := h.Service.GetProject(r.Context(), id)
	if err != nil {
		if err == ErrNotFound {
			return errors.NewNotFoundError("project not found", nil)
		}
		return errors.NewInternalError("failed to get project", err, nil)
	}

	err = h.Service.StartProjectServer(r.Context(), proj.ID)
	if err != nil {
		return errors.NewInternalError("failed to start project server", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, map[string]string{
		"status": "server started for project id " + idStr,
	})
}

// StopProjectServer godoc
// @Summary      Stop the server for a project
// @Description  Stop the server process for a given project
// @Tags         Chaincode Projects
// @Produce      json
// @Param        id path int true "Project ID"
// @Success      200 {object} map[string]string
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /chaincode-projects/{id}/stop [post]
func (h *ProjectsHandler) StopProjectServer(w http.ResponseWriter, r *http.Request) error {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}

	err = h.Service.StopProjectServer(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to stop project server", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, map[string]string{
		"status": "server stopped for project id " + idStr,
	})
}

// GetProjectLogs godoc
// @Summary      Get logs for a project server
// @Description  Stream or return the logs for the project's running container
// @Tags         Chaincode Projects
// @Produce      text/plain
// @Param        id path int true "Project ID"
// @Success      200 {string} string "Logs"
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /chaincode-projects/{id}/logs [get]
func (h *ProjectsHandler) GetProjectLogs(w http.ResponseWriter, r *http.Request) error {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}

	logs, err := h.Service.GetProjectLogs(r.Context(), id)
	if err != nil {
		return errors.NewInternalError("failed to get logs", err, nil)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(logs))
	return nil
}

// StreamProjectLogs godoc
// @Summary      Stream real-time logs for a project server
// @Description  Stream logs for the project's running container using SSE
// @Tags         Chaincode Projects
// @Produce      text/event-stream
// @Param        id path int true "Project ID"
// @Success      200 {string} string "SSE stream of logs"
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /chaincode-projects/{id}/logs/stream [get]
func (h *ProjectsHandler) StreamProjectLogs(w http.ResponseWriter, r *http.Request) error {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.NewInternalError("streaming not supported", nil, nil)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx := r.Context()
	err = h.Service.StreamProjectLogs(ctx, id, func(chunk []byte) {
		fmt.Fprintf(w, "data: %s\n\n", chunk)
		flusher.Flush()
	})

	if err != nil && err != context.Canceled {
		return errors.NewInternalError("failed to stream logs", err, nil)
	}

	return nil
}

// GetProjectCommits godoc
// @Summary      List project commits with file changes
// @Description  Get a paginated list of commits for a project, including added/removed/modified files
// @Tags         Chaincode Projects
// @Produce      json
// @Param        id path int true "Project ID"
// @Param        page query int false "Page number (default 1)"
// @Param        pageSize query int false "Page size (default 20)"
// @Success      200 {object} CommitsListResponse
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /chaincode-projects/{id}/commits [get]
func (h *ProjectsHandler) GetProjectCommits(w http.ResponseWriter, r *http.Request) error {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}

	proj, err := h.Service.GetProject(r.Context(), id)
	if err != nil {
		if err == ErrNotFound {
			return errors.NewNotFoundError("project not found", nil)
		}
		return errors.NewInternalError("failed to get project", err, nil)
	}

	// Pagination params
	page := 1
	pageSize := 20
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if ps := r.URL.Query().Get("pageSize"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 {
			pageSize = v
		}
	}

	projectDir := filepath.Join(h.Service.ProjectsDir, proj.Slug)
	maxCommits := page * pageSize
	commits, err := versionmanagement.ListCommitsWithFileChanges(r.Context(), projectDir, maxCommits)
	if err != nil {
		return errors.NewInternalError("failed to get commits", err, nil)
	}

	// Paginate
	start := (page - 1) * pageSize
	if start > len(commits) {
		start = len(commits)
	}
	end := start + pageSize
	if end > len(commits) {
		end = len(commits)
	}

	apiCommits := make([]CommitWithFileChangesAPI, 0, end-start)
	for _, c := range commits[start:end] {
		apiCommits = append(apiCommits, CommitWithFileChangesAPI{
			Hash:      c.Hash,
			Author:    c.Author,
			Timestamp: c.Timestamp,
			Message:   c.Message,
			Added:     c.Added,
			Removed:   c.Removed,
			Modified:  c.Modified,
			Parent:    c.Parent,
		})
	}

	return response.WriteJSON(w, http.StatusOK, CommitsListResponse{Commits: apiCommits})
}

// GetProjectCommitDetail godoc
// @Summary      Get commit details
// @Description  Get details for a single commit, including file changes
// @Tags         Chaincode Projects
// @Produce      json
// @Param        id path int true "Project ID"
// @Param        commitHash path string true "Commit hash"
// @Success      200 {object} CommitDetailAPI
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /chaincode-projects/{id}/commits/{commitHash} [get]
func (h *ProjectsHandler) GetProjectCommitDetail(w http.ResponseWriter, r *http.Request) error {
	idStr := chi.URLParam(r, "id")
	commitHash := chi.URLParam(r, "commitHash")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}

	proj, err := h.Service.GetProject(r.Context(), id)
	if err != nil {
		if err == ErrNotFound {
			return errors.NewNotFoundError("project not found", nil)
		}
		return errors.NewInternalError("failed to get project", err, nil)
	}

	projectDir := filepath.Join(h.Service.ProjectsDir, proj.Slug)
	commits, err := versionmanagement.ListCommitsWithFileChanges(r.Context(), projectDir, 1000)
	if err != nil {
		return errors.NewInternalError("failed to get commits", err, nil)
	}

	for _, c := range commits {
		if c.Hash == commitHash {
			return response.WriteJSON(w, http.StatusOK, CommitDetailAPI{
				Hash:      c.Hash,
				Author:    c.Author,
				Timestamp: c.Timestamp,
				Message:   c.Message,
				Added:     c.Added,
				Removed:   c.Removed,
				Modified:  c.Modified,
				Parent:    c.Parent,
			})
		}
	}

	return errors.NewNotFoundError("commit not found", nil)
}

// GetProjectFileDiff godoc
// @Summary      Get file diff between two commits
// @Description  Get the diff of a file between two commits
// @Tags         Chaincode Projects
// @Produce      text/plain
// @Param        id path int true "Project ID"
// @Param        file query string true "File path (relative to project root)"
// @Param        from query string true "From commit hash"
// @Param        to query string true "To commit hash"
// @Success      200 {string} string "Diff"
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /chaincode-projects/{id}/diff [get]
func (h *ProjectsHandler) GetProjectFileDiff(w http.ResponseWriter, r *http.Request) error {
	idStr := chi.URLParam(r, "id")
	file := r.URL.Query().Get("file")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	if file == "" || from == "" || to == "" {
		return errors.NewValidationError("missing file, from, or to parameter", nil)
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}

	proj, err := h.Service.GetProject(r.Context(), id)
	if err != nil {
		if err == ErrNotFound {
			return errors.NewNotFoundError("project not found", nil)
		}
		return errors.NewInternalError("failed to get project", err, nil)
	}

	projectDir := filepath.Join(h.Service.ProjectsDir, proj.Slug)
	diff, err := versionmanagement.GetFileDiffBetweenCommits(r.Context(), projectDir, file, from, to)
	if err != nil {
		return errors.NewInternalError("failed to get diff", err, nil)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(diff))
	return nil
}

// GetProjectFileAtCommit godoc
// @Summary      Get file contents at a specific commit
// @Description  Get the contents of a file at a specific commit hash
// @Tags         Chaincode Projects
// @Produce      text/plain
// @Param        id path int true "Project ID"
// @Param        file query string true "File path (relative to project root)"
// @Param        commit query string true "Commit hash"
// @Success      200 {string} string "File contents"
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /chaincode-projects/{id}/file_at_commit [get]
func (h *ProjectsHandler) GetProjectFileAtCommit(w http.ResponseWriter, r *http.Request) error {
	idStr := chi.URLParam(r, "id")
	file := r.URL.Query().Get("file")
	commit := r.URL.Query().Get("commit")

	if file == "" || commit == "" {
		return errors.NewValidationError("missing file or commit parameter", nil)
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}

	proj, err := h.Service.GetProject(r.Context(), id)
	if err != nil {
		if err == ErrNotFound {
			return errors.NewNotFoundError("project not found", nil)
		}
		return errors.NewInternalError("failed to get project", err, nil)
	}

	projectDir := filepath.Join(h.Service.ProjectsDir, proj.Slug)
	content, err := versionmanagement.GetFileAtCommit(r.Context(), projectDir, file, commit)
	if err != nil {
		return errors.NewInternalError("failed to get file at commit", err, nil)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(content))
	return nil
}

// UpdateProjectEndorsementPolicy godoc
// @Summary      Update a project's endorsement policy
// @Description  Update the endorsement policy of an existing project
// @Tags         Chaincode Projects
// @Accept       json
// @Produce      json
// @Param        id path int true "Project ID"
// @Param        request body UpdateProjectEndorsementPolicyRequest true "Update project endorsement policy request"
// @Success      200 {object} Project
// @Failure      400 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /chaincode-projects/{id}/endorsement-policy [put]
func (h *ProjectsHandler) UpdateProjectEndorsementPolicy(w http.ResponseWriter, r *http.Request) error {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid project id", map[string]interface{}{
			"error": err.Error(),
		})
	}

	var req UpdateProjectEndorsementPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", map[string]interface{}{
			"error": err.Error(),
		})
	}

	proj, err := h.Service.UpdateProjectEndorsementPolicy(r.Context(), id, req.EndorsementPolicy)
	if err != nil {
		if err == ErrNotFound {
			return errors.NewNotFoundError("project not found", nil)
		}
		return errors.NewInternalError("failed to update project endorsement policy", err, nil)
	}

	zap.L().Info("updated project endorsement policy", zap.Int64("id", proj.ID), zap.String("name", proj.Name), zap.String("request_id", middleware.GetReqID(r.Context())))

	return response.WriteJSON(w, http.StatusOK, proj)
}
