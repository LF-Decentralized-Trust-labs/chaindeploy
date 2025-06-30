package projects

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"crypto/rand"
	"encoding/hex"
	"strings"

	"archive/zip"

	"github.com/chainlaunch/chainlaunch/pkg/common/addresses"
	"github.com/chainlaunch/chainlaunch/pkg/db"
	fabricService "github.com/chainlaunch/chainlaunch/pkg/fabric/service"
	keyMgmtService "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	networkservice "github.com/chainlaunch/chainlaunch/pkg/networks/service"
	"github.com/chainlaunch/chainlaunch/pkg/scai/boilerplates"
	"github.com/chainlaunch/chainlaunch/pkg/scai/projectrunner"
	"github.com/chainlaunch/chainlaunch/pkg/scai/versionmanagement"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type ProjectsService struct {
	Queries            *db.Queries
	Runner             *projectrunner.Runner
	ProjectsDir        string
	BoilerplateService *boilerplates.BoilerplateService
	OrgService         *fabricService.OrganizationService
	KeyMgmtService     *keyMgmtService.KeyManagementService
	NetworkService     *networkservice.NetworkService
}

type Project struct {
	ID                int64   `json:"id" example:"1" description:"Project ID"`
	Name              string  `json:"name" example:"myproject" description:"Project name"`
	Slug              string  `json:"slug" example:"myproject-abc12" description:"Project slug (used for proxying and folder name)"`
	Description       string  `json:"description" example:"A sample project" description:"Project description"`
	Boilerplate       string  `json:"boilerplate" example:"go-basic" description:"Boilerplate template used for scaffolding"`
	Status            string  `json:"status" example:"running" description:"Project container status (running/stopped/etc)"`
	LastStartedAt     *string `json:"lastStartedAt,omitempty" description:"Last time the project was started (RFC3339)"`
	LastStoppedAt     *string `json:"lastStoppedAt,omitempty" description:"Last time the project was stopped (RFC3339)"`
	ContainerPort     *int    `json:"containerPort,omitempty" description:"Host port mapped to the container, if running"`
	NetworkID         *int64  `json:"networkId,omitempty" description:"ID of the linked network"`
	NetworkName       *string `json:"networkName,omitempty" description:"Name of the linked network"`
	NetworkPlatform   *string `json:"networkPlatform,omitempty" description:"Platform of the linked network (FABRIC/BESU)"`
	EndorsementPolicy string  `json:"endorsementPolicy,omitempty" example:"OR('Org1MSP.member','Org2MSP.member')" description:"Endorsement policy for the chaincode"`
}

// ProjectProcessManager manages running server processes for projects
var projectProcessManager = struct {
	mu      sync.Mutex
	servers map[int64]*exec.Cmd
}{servers: make(map[int64]*exec.Cmd)}

// NewProjectsService creates a new ProjectsService instance
func NewProjectsService(queries *db.Queries, runner *projectrunner.Runner, projectsDir string, orgService *fabricService.OrganizationService, keyMgmtService *keyMgmtService.KeyManagementService, networkService *networkservice.NetworkService) (*ProjectsService, error) {
	boilerplateService, err := boilerplates.NewBoilerplateService(queries)
	if err != nil {
		return nil, err
	}

	return &ProjectsService{
		Queries:            queries,
		Runner:             runner,
		ProjectsDir:        projectsDir,
		BoilerplateService: boilerplateService,
		OrgService:         orgService,
		KeyMgmtService:     keyMgmtService,
		NetworkService:     networkService,
	}, nil
}

func getReqID(ctx context.Context) string {
	if reqID, ok := ctx.Value(middleware.RequestIDKey).(string); ok {
		return reqID
	}
	return ""
}

// Helper to copy a directory recursively
func copyDir(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()
		dstFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()
		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

func generateShortGUID(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b)[:n], nil
}

func generateSlug(name string, queries *db.Queries, ctx context.Context) (string, error) {
	base := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	for {
		guid, err := generateShortGUID(5)
		if err != nil {
			return "", err
		}
		slug := base + "-" + guid
		// Check uniqueness
		_, err = queries.GetProjectBySlug(ctx, slug)
		if err != nil && err != sql.ErrNoRows {
			return "", err
		}
		if err == sql.ErrNoRows {
			return slug, nil
		}
		// else, collision, try again
	}
}

func (s *ProjectsService) CreateProject(ctx context.Context, name, description, boilerplate string, networkID *int64, endorsementPolicy string) (Project, error) {
	slug, err := generateSlug(name, s.Queries, ctx)
	if err != nil {
		return Project{}, err
	}
	proj, err := s.Queries.CreateProject(ctx, &db.CreateProjectParams{
		Name:              name,
		Description:       sql.NullString{String: description, Valid: description != ""},
		Boilerplate:       sql.NullString{String: boilerplate, Valid: boilerplate != ""},
		Slug:              slug,
		NetworkID:         sql.NullInt64{Int64: *networkID, Valid: networkID != nil},
		EndorsementPolicy: sql.NullString{String: endorsementPolicy, Valid: endorsementPolicy != ""},
	})
	if err != nil {
		zap.L().Error("DB error in CreateProject", zap.String("name", name), zap.Error(err), zap.String("request_id", getReqID(ctx)))
		return Project{}, err
	}
	zap.L().Info("created project in DB", zap.Int64("id", proj.ID), zap.String("name", proj.Name), zap.String("slug", proj.Slug), zap.String("request_id", getReqID(ctx)))

	// Create a default conversation for the project
	_, err = s.Queries.CreateConversation(ctx, proj.ID)
	if err != nil {
		zap.L().Error("failed to create default conversation for project", zap.Int64("projectID", proj.ID), zap.Error(err), zap.String("request_id", getReqID(ctx)))
		// Don't fail the project creation if conversation creation fails
	} else {
		zap.L().Info("created default conversation for project", zap.Int64("projectID", proj.ID), zap.String("request_id", getReqID(ctx)))
	}

	// Download boilerplate if specified
	if boilerplate != "" {
		projectDir, err := s.GetProjectDirectoryBySlug(slug)
		if err != nil {
			zap.L().Error("failed to build safe project path for boilerplate", zap.String("slug", slug), zap.Error(err))
			return Project{}, err
		}
		if err := s.BoilerplateService.DownloadBoilerplate(ctx, boilerplate, projectDir); err != nil {
			zap.L().Error("failed to download boilerplate", zap.String("boilerplate", boilerplate), zap.Error(err))
			return Project{}, err
		}

		// Ensure git repository is initialized before committing
		gitDir := filepath.Join(projectDir, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			// Initialize the repo using go-git
			_, err := versionmanagement.InitRepo(projectDir)
			if err != nil {
				zap.L().Error("failed to initialize git repo", zap.Error(err))
			}
		}
		vm := versionmanagement.NewDefaultManager()
		err = vm.CommitChange(ctx, projectDir, "Initial commit for project "+name)
		if err != nil {
			zap.L().Error("failed to commit", zap.Error(err))
		}
	}
	return dbProjectToAPI(proj), nil
}

func (s *ProjectsService) ListProjects(ctx context.Context) ([]Project, error) {
	dbProjects, err := s.Queries.ListProjects(ctx)
	if err != nil {
		zap.L().Error("DB error in ListProjects", zap.Error(err), zap.String("request_id", getReqID(ctx)))
		return nil, err
	}
	var projects []Project
	for _, p := range dbProjects {
		projects = append(projects, listDBProjectToAPI(p))
	}
	zap.L().Info("listed projects from DB", zap.Int("count", len(projects)), zap.String("request_id", getReqID(ctx)))
	return projects, nil
}

func (s *ProjectsService) GetProject(ctx context.Context, id int64) (Project, error) {
	p, err := s.Queries.GetProject(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			zap.L().Warn("project not found in DB", zap.Int64("id", id), zap.String("request_id", getReqID(ctx)))
			return Project{}, ErrNotFound
		}
		zap.L().Error("DB error in GetProject", zap.Int64("id", id), zap.Error(err), zap.String("request_id", getReqID(ctx)))
		return Project{}, err
	}
	zap.L().Info("got project from DB", zap.Int64("id", p.ID), zap.String("name", p.Name), zap.String("request_id", getReqID(ctx)))
	return getDBProjectToAPI(p), nil
}

// DeleteProject deletes a project and its associated files
func (s *ProjectsService) DeleteProject(ctx context.Context, id int64) error {
	// First get the project to get the slug for file cleanup
	proj, err := s.Queries.GetProject(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			zap.L().Warn("project not found for deletion", zap.Int64("id", id), zap.String("request_id", getReqID(ctx)))
			return ErrNotFound
		}
		zap.L().Error("DB error in GetProject for deletion", zap.Int64("id", id), zap.Error(err), zap.String("request_id", getReqID(ctx)))
		return err
	}

	// Stop the project server if it's running
	if err := s.StopProjectServer(ctx, id); err != nil {
		zap.L().Warn("failed to stop project server before deletion", zap.Int64("id", id), zap.Error(err))
		// Continue with deletion even if stop fails
	}

	// Delete the project from database
	if err := s.Queries.DeleteProject(ctx, id); err != nil {
		zap.L().Error("DB error in DeleteProject", zap.Int64("id", id), zap.Error(err), zap.String("request_id", getReqID(ctx)))
		return err
	}

	// Clean up project files
	projectDir, err := s.safeJoinPath(proj.Slug)
	if err != nil {
		zap.L().Warn("failed to build safe project path for cleanup", zap.Int64("id", id), zap.String("slug", proj.Slug), zap.Error(err))
		// Continue with deletion even if path validation fails
	} else {
		if err := os.RemoveAll(projectDir); err != nil {
			zap.L().Warn("failed to remove project directory", zap.Int64("id", id), zap.String("slug", proj.Slug), zap.Error(err))
			// Continue even if file cleanup fails
		}
	}

	zap.L().Info("deleted project", zap.Int64("id", id), zap.String("name", proj.Name), zap.String("slug", proj.Slug), zap.String("request_id", getReqID(ctx)))
	return nil
}

// UpdateProjectEndorsementPolicy updates the endorsement policy of a project
func (s *ProjectsService) UpdateProjectEndorsementPolicy(ctx context.Context, id int64, endorsementPolicy string) (Project, error) {
	// First check if project exists
	_, err := s.Queries.GetProject(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			zap.L().Warn("project not found", zap.Int64("id", id), zap.String("request_id", getReqID(ctx)))
			return Project{}, ErrNotFound
		}
		zap.L().Error("DB error in GetProject", zap.Int64("id", id), zap.Error(err), zap.String("request_id", getReqID(ctx)))
		return Project{}, err
	}

	proj, err := s.Queries.UpdateProjectEndorsementPolicy(ctx, &db.UpdateProjectEndorsementPolicyParams{
		ID:                id,
		EndorsementPolicy: sql.NullString{String: endorsementPolicy, Valid: endorsementPolicy != ""},
	})
	if err != nil {
		zap.L().Error("DB error in UpdateProjectEndorsementPolicy", zap.Int64("id", id), zap.Error(err), zap.String("request_id", getReqID(ctx)))
		return Project{}, err
	}

	zap.L().Info("updated project endorsement policy", zap.Int64("id", proj.ID), zap.String("name", proj.Name), zap.String("request_id", getReqID(ctx)))

	return dbProjectToAPI(proj), nil
}

func listDBProjectToAPI(p *db.ListProjectsRow) Project {
	var started, stopped *string
	if p.LastStartedAt.Valid {
		ts := p.LastStartedAt.Time.UTC().Format(time.RFC3339)
		started = &ts
	}
	if p.LastStoppedAt.Valid {
		ts := p.LastStoppedAt.Time.UTC().Format(time.RFC3339)
		stopped = &ts
	}
	var containerPort *int
	if p.ContainerPort.Valid {
		v := int(p.ContainerPort.Int64)
		containerPort = &v
	}
	var networkID *int64
	if p.NetworkID.Valid {
		networkID = &p.NetworkID.Int64
	}
	var networkName *string
	if p.NetworkName.Valid {
		networkName = &p.NetworkName.String
	}
	var networkPlatform *string
	if p.NetworkPlatform.Valid {
		networkPlatform = &p.NetworkPlatform.String
	}
	return Project{
		ID:                p.ID,
		Name:              p.Name,
		Slug:              p.Slug,
		Description:       p.Description.String,
		Boilerplate:       p.Boilerplate.String,
		Status:            p.Status.String,
		LastStartedAt:     started,
		LastStoppedAt:     stopped,
		ContainerPort:     containerPort,
		NetworkID:         networkID,
		NetworkName:       networkName,
		NetworkPlatform:   networkPlatform,
		EndorsementPolicy: p.EndorsementPolicy.String,
	}
}

func getDBProjectToAPI(p *db.GetProjectRow) Project {
	var started, stopped *string
	if p.LastStartedAt.Valid {
		ts := p.LastStartedAt.Time.UTC().Format(time.RFC3339)
		started = &ts
	}
	if p.LastStoppedAt.Valid {
		ts := p.LastStoppedAt.Time.UTC().Format(time.RFC3339)
		stopped = &ts
	}
	var containerPort *int
	if p.ContainerPort.Valid {
		v := int(p.ContainerPort.Int64)
		containerPort = &v
	}
	var networkID *int64
	if p.NetworkID.Valid {
		networkID = &p.NetworkID.Int64
	}
	var networkName *string
	if p.NetworkName.Valid {
		networkName = &p.NetworkName.String
	}
	var networkPlatform *string
	if p.NetworkPlatform.Valid {
		networkPlatform = &p.NetworkPlatform.String
	}
	return Project{
		ID:                p.ID,
		Name:              p.Name,
		Slug:              p.Slug,
		Description:       p.Description.String,
		Boilerplate:       p.Boilerplate.String,
		Status:            p.Status.String,
		LastStartedAt:     started,
		LastStoppedAt:     stopped,
		ContainerPort:     containerPort,
		NetworkID:         networkID,
		NetworkName:       networkName,
		NetworkPlatform:   networkPlatform,
		EndorsementPolicy: p.EndorsementPolicy.String,
	}
}

func dbProjectToAPI(p *db.ChaincodeProject) Project {
	var started, stopped *string
	if p.LastStartedAt.Valid {
		ts := p.LastStartedAt.Time.UTC().Format(time.RFC3339)
		started = &ts
	}
	if p.LastStoppedAt.Valid {
		ts := p.LastStoppedAt.Time.UTC().Format(time.RFC3339)
		stopped = &ts
	}
	var containerPort *int
	if p.ContainerPort.Valid {
		v := int(p.ContainerPort.Int64)
		containerPort = &v
	}
	var networkID *int64
	if p.NetworkID.Valid {
		networkID = &p.NetworkID.Int64
	}
	return Project{
		ID:                p.ID,
		Name:              p.Name,
		Slug:              p.Slug,
		Description:       p.Description.String,
		Boilerplate:       p.Boilerplate.String,
		Status:            p.Status.String,
		LastStartedAt:     started,
		LastStoppedAt:     stopped,
		ContainerPort:     containerPort,
		NetworkID:         networkID,
		EndorsementPolicy: p.EndorsementPolicy.String,
	}
}

var ErrNotFound = errors.New("not found")

// findAvailablePort finds an available port starting from the given port
func findAvailablePort(startPort int) (int, error) {
	maxAttempts := 100
	for port := startPort; port < startPort+maxAttempts; port++ {
		addr := fmt.Sprintf(":%d", port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports found after %d attempts starting from %d", maxAttempts, startPort)
}

// StartProjectServer starts the server process for a project
func (s *ProjectsService) StartProjectServer(ctx context.Context, projectID int64) error {
	project, err := s.Queries.GetProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	if !project.Boilerplate.Valid {
		return fmt.Errorf("project has no boilerplate configured")
	}
	projectDB, err := s.Queries.GetProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}
	networkDB, err := s.Queries.GetNetwork(ctx, projectDB.NetworkID.Int64)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}

	// Get the appropriate lifecycle implementation for the platform
	lifecycle, err := GetPlatformLifecycle(networkDB.Platform, s.Queries, s.OrgService, s.KeyMgmtService, s.NetworkService, zap.L())
	if err != nil {
		zap.L().Warn("failed to get platform lifecycle, continuing without lifecycle hooks",
			zap.String("platform", project.Boilerplate.String),
			zap.Error(err),
		)
	}

	command, args, image, err := projectrunner.GetBoilerplateRunner(s.BoilerplateService, project.Boilerplate.String)
	if err != nil {
		return fmt.Errorf("failed to get boilerplate runner: %w", err)
	}

	projectDir, err := s.safeJoinPath(project.Slug)
	if err != nil {
		return fmt.Errorf("failed to build safe project path: %w", err)
	}

	// Get the host IP for smart contract deployment
	hostIP, err := addresses.GetExternalIP()
	if err != nil {
		zap.L().Warn("failed to get host IP, using localhost",
			zap.Error(err),
		)
		hostIP = "127.0.0.1"
	}

	// Find an available port
	port, err := findAvailablePort(40000)
	if err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}

	// Call PreStart lifecycle hook if available
	var env map[string]string
	if lifecycle != nil {
		preStartParams := PreStartParams{
			ProjectLifecycleParams: ProjectLifecycleParams{
				ProjectID:         project.ID,
				ProjectName:       project.Name,
				ProjectSlug:       project.Slug,
				NetworkID:         project.NetworkID.Int64,
				NetworkName:       networkDB.Name,
				Platform:          networkDB.Platform,
				Boilerplate:       project.Boilerplate.String,
				EndorsementPolicy: project.EndorsementPolicy.String,
			},
			Image:       image,
			Port:        port,
			Command:     command,
			Args:        args,
			Environment: make(map[string]string),
			HostIP:      hostIP,
		}
		result, err := lifecycle.PreStart(ctx, preStartParams)
		if err != nil {
			return fmt.Errorf("pre-start lifecycle hook failed: %w", err)
		}
		if result != nil {
			env = result.Environment
		}
	}

	// Prepend the command to the args
	allArgs := append([]string{command}, args...)
	port, err = s.Runner.Start(ctx, fmt.Sprintf("%d", projectID), projectDir, image, port, env, allArgs...)
	if err != nil {
		return err
	}

	// Call PostStart lifecycle hook if available
	if lifecycle != nil {
		postStartParams := PostStartParams{
			ProjectLifecycleParams: ProjectLifecycleParams{
				ProjectID:         project.ID,
				ProjectName:       project.Name,
				ProjectSlug:       project.Slug,
				NetworkID:         project.NetworkID.Int64,
				NetworkName:       networkDB.Name,
				Platform:          networkDB.Platform,
				Boilerplate:       project.Boilerplate.String,
				EndorsementPolicy: project.EndorsementPolicy.String,
			},
			ContainerID: project.ContainerID.String,
			Image:       image,
			Port:        port,
			StartedAt:   time.Now(),
			Status:      "running",
			HostIP:      hostIP,
		}
		if err := lifecycle.PostStart(ctx, postStartParams); err != nil {
			// Log the error but don't fail the start operation
			zap.L().Error("post-start lifecycle hook failed",
				zap.Int64("projectID", project.ID),
				zap.Error(err),
			)
		}
	}

	return nil
}

// StopProjectServer stops the server process for a project
func (s *ProjectsService) StopProjectServer(ctx context.Context, projectID int64) error {
	project, err := s.Queries.GetProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	// Get the appropriate lifecycle implementation for the platform
	projectDB, err := s.Queries.GetProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}
	networkDB, err := s.Queries.GetNetwork(ctx, projectDB.NetworkID.Int64)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}

	// Get the appropriate lifecycle implementation for the platform
	lifecycle, err := GetPlatformLifecycle(networkDB.Platform, s.Queries, s.OrgService, s.KeyMgmtService, s.NetworkService, zap.L())
	if err != nil {
		zap.L().Warn("failed to get platform lifecycle, continuing without lifecycle hooks",
			zap.String("platform", project.Boilerplate.String),
			zap.Error(err),
		)
	}
	// Call PreStop lifecycle hook if available
	if lifecycle != nil {
		preStopParams := PreStopParams{
			ProjectLifecycleParams: ProjectLifecycleParams{
				ProjectID:         project.ID,
				ProjectName:       project.Name,
				ProjectSlug:       project.Slug,
				NetworkID:         project.NetworkID.Int64,
				NetworkName:       networkDB.Name,
				Platform:          networkDB.Platform,
				Boilerplate:       project.Boilerplate.String,
				EndorsementPolicy: project.EndorsementPolicy.String,
			},
			ContainerID: project.ContainerID.String,
			StartedAt:   project.LastStartedAt.Time,
		}
		if err := lifecycle.PreStop(ctx, preStopParams); err != nil {
			// Log the error but don't fail the stop operation
			zap.L().Error("pre-stop lifecycle hook failed",
				zap.Int64("projectID", project.ID),
				zap.Error(err),
			)
		}
	}

	if err := s.Runner.Stop(fmt.Sprintf("%d", projectID)); err != nil {
		return err
	}

	// Call PostStop lifecycle hook if available
	if lifecycle != nil {
		now := time.Now()
		postStopParams := PostStopParams{
			ProjectLifecycleParams: ProjectLifecycleParams{
				ProjectID:         project.ID,
				ProjectName:       project.Name,
				ProjectSlug:       project.Slug,
				NetworkID:         project.NetworkID.Int64,
				NetworkName:       networkDB.Name,
				Platform:          networkDB.Platform,
				Boilerplate:       project.Boilerplate.String,
				EndorsementPolicy: project.EndorsementPolicy.String,
			},
			ContainerID: project.ContainerID.String,
			StartedAt:   project.LastStartedAt.Time,
			StoppedAt:   now,
		}
		if err := lifecycle.PostStop(ctx, postStopParams); err != nil {
			// Log the error but don't fail the stop operation
			zap.L().Error("post-stop lifecycle hook failed",
				zap.Int64("projectID", project.ID),
				zap.Error(err),
			)
		}
	}

	return nil
}

func (s *ProjectsService) GetProjectLogs(ctx context.Context, projectID int64) (string, error) {
	return s.Runner.GetLogs(fmt.Sprintf("%d", projectID))
}

func (s *ProjectsService) StreamProjectLogs(ctx context.Context, projectID int64, onLog func([]byte)) error {
	return s.Runner.StreamLogs(ctx, fmt.Sprintf("%d", projectID), onLog)
}

// DownloadProjectAsZip creates a zip file containing the project files, excluding common ignored folders
// This version streams the zip directly to the provided writer to avoid memory issues
func (s *ProjectsService) DownloadProjectAsZip(ctx context.Context, projectID int64, writer io.Writer) error {
	// Get project details
	project, err := s.Queries.GetProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	// Build project directory path safely
	projectDir, err := s.safeJoinPath(project.Slug)
	if err != nil {
		return fmt.Errorf("failed to build safe project path: %w", err)
	}

	// Check if project directory exists
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		return fmt.Errorf("project directory does not exist: %s", projectDir)
	}

	// Create zip writer that writes directly to the provided writer
	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	// Define folders to ignore
	ignoredFolders := map[string]bool{
		"node_modules":           true,
		".vscode":                true,
		".git":                   true,
		"tmp":                    true,
		"temp":                   true,
		".DS_Store":              true,
		"__pycache__":            true,
		".pytest_cache":          true,
		"coverage":               true,
		"dist":                   true,
		"build":                  true,
		".next":                  true,
		".nuxt":                  true,
		".cache":                 true,
		"logs":                   true,
		".env.local":             true,
		".env.development.local": true,
		".env.test.local":        true,
		".env.production.local":  true,
	}

	// Walk through the project directory
	err = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return err
		}

		// Get relative path from project directory
		relPath, err := filepath.Rel(projectDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Check if this is a directory that should be ignored
		if info.IsDir() {
			dirName := filepath.Base(path)
			if ignoredFolders[dirName] {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if this is a file that should be ignored
		fileName := filepath.Base(path)
		if ignoredFolders[fileName] {
			return nil
		}

		// Create zip file entry
		zipFile, err := zipWriter.Create(relPath)
		if err != nil {
			return fmt.Errorf("failed to create zip entry for %s: %w", relPath, err)
		}

		// Open and read the file
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", path, err)
		}
		defer file.Close()

		// Copy file content to zip
		_, err = io.Copy(zipFile, file)
		if err != nil {
			return fmt.Errorf("failed to copy file %s to zip: %w", path, err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}

	// Close the zip writer to finalize the zip file
	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close zip writer: %w", err)
	}

	return nil
}

// validateSlug ensures a slug is safe for use in file paths
// It checks that the slug only contains lowercase letters, numbers, and hyphens
// and doesn't contain any path traversal characters
func validateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("slug cannot be empty")
	}

	// Check for path traversal characters
	if strings.Contains(slug, "..") || strings.Contains(slug, "/") || strings.Contains(slug, "\\") {
		return fmt.Errorf("slug contains invalid path characters")
	}

	// Validate slug format: lowercase letters, numbers, and hyphens only
	validSlugRegex := regexp.MustCompile(`^[a-z0-9-]+$`)
	if !validSlugRegex.MatchString(slug) {
		return fmt.Errorf("slug contains invalid characters, only lowercase letters, numbers, and hyphens are allowed")
	}

	// Ensure slug doesn't start or end with hyphen
	if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
		return fmt.Errorf("slug cannot start or end with hyphen")
	}

	return nil
}

// safeJoinPath safely joins the projects directory with a validated slug
func (s *ProjectsService) safeJoinPath(slug string) (string, error) {
	if err := validateSlug(slug); err != nil {
		return "", fmt.Errorf("invalid slug: %w", err)
	}

	// Use filepath.Join and then validate the result is within the projects directory
	projectPath := filepath.Join(s.ProjectsDir, slug)

	// Ensure the resulting path is within the projects directory (prevents path traversal)
	absProjectsDir, err := filepath.Abs(s.ProjectsDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for projects directory: %w", err)
	}

	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for project: %w", err)
	}

	// Check if the project path is within the projects directory
	if !strings.HasPrefix(absProjectPath, absProjectsDir) {
		return "", fmt.Errorf("project path is outside projects directory")
	}

	return projectPath, nil
}

// GetProjectDirectory returns the safe project directory path for a given project
func (s *ProjectsService) GetProjectDirectory(project Project) (string, error) {
	return s.safeJoinPath(project.Slug)
}

// GetProjectDirectoryBySlug returns the safe project directory path for a given slug
func (s *ProjectsService) GetProjectDirectoryBySlug(slug string) (string, error) {
	return s.safeJoinPath(slug)
}
