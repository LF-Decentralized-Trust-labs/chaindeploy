package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/logger"
	plugintypes "github.com/chainlaunch/chainlaunch/pkg/plugin/types"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	cmdCompose "github.com/docker/compose/v2/cmd/compose"
	"github.com/docker/compose/v2/pkg/api"

	"bytes"
	"text/template"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	key "github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	nodeservice "github.com/chainlaunch/chainlaunch/pkg/nodes/service"
	"github.com/chainlaunch/chainlaunch/pkg/plugin/xsource"
	"github.com/docker/compose/v2/pkg/compose"
	"gopkg.in/yaml.v3"
)

// PluginManager handles plugin operations
type PluginManager struct {
	pluginsDir    string
	compose       api.Service
	dockerCli     *command.DockerCli
	db            *db.Queries
	nodeService   *nodeservice.NodeService
	keyManagement *key.KeyManagementService
	logger        *logger.Logger
}

// NewPluginManager creates a new plugin manager
func NewPluginManager(pluginsDir string, db *db.Queries, nodeService *nodeservice.NodeService, keyManagement *key.KeyManagementService, logger *logger.Logger) (*PluginManager, error) {
	dockerCli, err := command.NewDockerCli()
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker CLI: %w", err)
	}

	err = dockerCli.Initialize(flags.NewClientOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Docker CLI: %w", err)
	}

	// check if docker engine is running
	_, err = dockerCli.Client().Info(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to check if Docker engine is running: %w", err)
	}

	// Create Compose service
	composeService := compose.NewComposeService(dockerCli)

	return &PluginManager{
		pluginsDir:    pluginsDir,
		compose:       composeService,
		dockerCli:     dockerCli,
		db:            db,
		nodeService:   nodeService,
		keyManagement: keyManagement,
		logger:        logger,
	}, nil
}

// LoadPlugin loads a plugin from a file
func (pm *PluginManager) LoadPlugin(filePath string) (*plugintypes.Plugin, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin file: %w", err)
	}

	var plugin plugintypes.Plugin
	if err := yaml.Unmarshal(data, &plugin); err != nil {
		return nil, fmt.Errorf("failed to unmarshal plugin: %w", err)
	}

	return &plugin, nil
}

// validateXSourceParameters validates x-source parameters using the store's fetchers
func validateXSourceParameters(ctx context.Context, plugin *plugintypes.Plugin, parameters map[string]interface{}, db *db.Queries, nodeService *nodeservice.NodeService, keyManagement *key.KeyManagementService) error {
	// Marshal the plugin's parameters schema to JSON
	schemaJSON, err := json.Marshal(plugin.Spec.Parameters)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters schema: %w", err)
	}

	// Extract x-source fields from the schema
	var schemaData map[string]interface{}
	if err := json.Unmarshal(schemaJSON, &schemaData); err != nil {
		return fmt.Errorf("failed to unmarshal parameters schema: %w", err)
	}

	// Create x-source registry
	registry := xsource.NewRegistry(db, nodeService, keyManagement)

	// Get properties from schema
	properties, ok := schemaData["properties"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid schema: properties not found")
	}

	// Validate each parameter
	for name, prop := range properties {
		propMap, ok := prop.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if this is an x-source parameter
		xSourceType, ok := propMap["x-source"].(string)
		if !ok {
			continue
		}

		// Check if parameter is required
		required, _ := propMap["required"].(bool)
		value, exists := parameters[name]

		if required && !exists {
			return fmt.Errorf("required parameter '%s' is missing", name)
		}

		if exists {
			// Validate and process the parameter using the registry
			_, err := registry.ValidateAndProcess(ctx, xsource.XSourceType(xSourceType), name, value)
			if err != nil {
				return fmt.Errorf("invalid value for parameter '%s': %w", name, err)
			}
		}
	}

	return nil
}

// processXSourceParameters processes parameters that have x-source specifications
func (pm *PluginManager) processXSourceParameters(ctx context.Context, plugin *plugintypes.Plugin, parameters map[string]interface{}) (map[string]interface{}, []xsource.VolumeMount, error) {
	processedParameters := make(map[string]interface{})
	var volumeMounts []xsource.VolumeMount

	// Create x-source registry
	registry := xsource.NewRegistry(pm.db, pm.nodeService, pm.keyManagement)

	for key, value := range parameters {
		// Check if this parameter has an x-source specification
		if spec, ok := plugin.Spec.Parameters.Properties[key]; ok && spec.XSource != "" {
			// Get the handler for this x-source type
			handler, err := registry.GetHandler(xsource.XSourceType(spec.XSource))
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get handler for x-source type %s: %w", spec.XSource, err)
			}

			// Create the x-source value
			xsourceValue, err := handler.CreateValue(key, value)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create x-source value for %s: %w", key, err)
			}

			// Validate the value
			if err := xsourceValue.Validate(ctx); err != nil {
				return nil, nil, fmt.Errorf("invalid x-source value for %s: %w", key, err)
			}

			// Get the processed value for templates
			processedValue, err := xsourceValue.GetValue(ctx, spec)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get x-source value for %s: %w", key, err)
			}

			// Get volume mounts
			mounts, err := xsourceValue.GetVolumeMounts(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get volume mounts for %s: %w", key, err)
			}
			volumeMounts = append(volumeMounts, mounts...)

			processedParameters[key] = processedValue
		} else {
			// For non-x-source parameters, keep as is
			processedParameters[key] = value
		}
	}

	return processedParameters, volumeMounts, nil
}

// DeployPlugin deploys a plugin using docker-compose
func (pm *PluginManager) DeployPlugin(ctx context.Context, plugin *plugintypes.Plugin, parameters map[string]interface{}, store Store) error {
	// Validate x-source parameters before deployment
	if err := validateXSourceParameters(ctx, plugin, parameters, pm.db, pm.nodeService, pm.keyManagement); err != nil {
		_ = store.UpdateDeploymentStatus(ctx, plugin.Metadata.Name, "failed")
		return fmt.Errorf("x-source parameter validation failed: %w", err)
	}

	// Update plugin status to deploying
	if err := store.UpdateDeploymentStatus(ctx, plugin.Metadata.Name, "deploying"); err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	// Create a temporary directory for the plugin
	tempDir, err := os.MkdirTemp("", plugin.Metadata.Name)
	if err != nil {
		// Update status to failed
		_ = store.UpdateDeploymentStatus(ctx, plugin.Metadata.Name, "failed")
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Process x-source parameters and fetch complete details
	processedParameters, volumeMounts, err := pm.processXSourceParameters(ctx, plugin, parameters)
	if err != nil {
		_ = store.UpdateDeploymentStatus(ctx, plugin.Metadata.Name, "failed")
		return fmt.Errorf("failed to process x-source parameters: %w", err)
	}

	pm.logger.Infof("Processed parameters: %v", processedParameters)
	pm.logger.Infof("Volume mounts: %v", volumeMounts)

	// Add volume mounts to the template data
	templateData := map[string]interface{}{
		"parameters":   processedParameters,
		"volumeMounts": volumeMounts,
	}

	// Render the docker-compose contents as a Go template
	var renderedCompose bytes.Buffer
	tmpl, err := template.New("docker-compose").Parse(plugin.Spec.DockerCompose.Contents)
	if err != nil {
		_ = store.UpdateDeploymentStatus(ctx, plugin.Metadata.Name, "failed")
		return fmt.Errorf("failed to parse docker-compose template: %w", err)
	}

	if err := tmpl.Execute(&renderedCompose, templateData); err != nil {
		_ = store.UpdateDeploymentStatus(ctx, plugin.Metadata.Name, "failed")
		return fmt.Errorf("failed to render docker-compose template: %w", err)
	}

	// Write the rendered docker-compose contents to a file
	composePath := filepath.Join(tempDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, renderedCompose.Bytes(), 0644); err != nil {
		// Update status to failed
		_ = store.UpdateDeploymentStatus(ctx, plugin.Metadata.Name, "failed")
		return fmt.Errorf("failed to write docker-compose file: %w", err)
	}

	// Create environment variables file
	envVars := make(map[string]string)
	for name, value := range processedParameters {
		if strValue, ok := value.(string); ok {
			envVars[name] = strValue
		}
	}

	envPath := filepath.Join(tempDir, ".env")
	envContent := ""
	for name, value := range envVars {
		envContent += fmt.Sprintf("%s=%s\n", name, value)
	}
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		// Update status to failed
		_ = store.UpdateDeploymentStatus(ctx, plugin.Metadata.Name, "failed")
		return fmt.Errorf("failed to write environment file: %w", err)
	}

	projectOptions := cmdCompose.ProjectOptions{
		ProjectName: plugin.Metadata.Name,
		ConfigPaths: []string{composePath},
	}

	// Turn projectOptions into a project with default values
	projectType, _, err := projectOptions.ToProject(ctx, pm.dockerCli, []string{})
	if err != nil {
		// Update status to failed
		_ = store.UpdateDeploymentStatus(ctx, plugin.Metadata.Name, "failed")
		return err
	}

	upOptions := api.UpOptions{
		Create: api.CreateOptions{
			RemoveOrphans: true,
			QuietPull:     true,
		},
		Start: api.StartOptions{
			Wait: true,
		},
	}

	// Load the project
	err = pm.compose.Up(ctx, projectType, upOptions)
	if err != nil {
		// Update status to failed
		_ = store.UpdateDeploymentStatus(ctx, plugin.Metadata.Name, "failed")
		return fmt.Errorf("failed to load project: %w", err)
	}

	// Save deployment metadata
	deploymentMetadata := map[string]interface{}{
		"deployedAt":  time.Now().Format(time.RFC3339),
		"parameters":  parameters,
		"projectName": plugin.Metadata.Name,
	}

	if err := store.UpdateDeploymentMetadata(ctx, plugin.Metadata.Name, deploymentMetadata); err != nil {
		return fmt.Errorf("failed to update deployment metadata: %w", err)
	}

	// Update status to deployed
	if err := store.UpdateDeploymentStatus(ctx, plugin.Metadata.Name, "deployed"); err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	return nil
}

// SavePlugin saves a plugin to a file
func (pm *PluginManager) SavePlugin(plugin *plugintypes.Plugin) error {
	data, err := yaml.Marshal(plugin)
	if err != nil {
		return fmt.Errorf("failed to marshal plugin: %w", err)
	}

	filePath := filepath.Join(pm.pluginsDir, plugin.Metadata.Name+".yaml")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write plugin file: %w", err)
	}

	return nil
}

// ValidatePlugin validates a plugin definition
func (pm *PluginManager) ValidatePlugin(plugin *plugintypes.Plugin) error {
	if plugin.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}
	if plugin.Kind == "" {
		return fmt.Errorf("kind is required")
	}
	if plugin.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if plugin.Metadata.Version == "" {
		return fmt.Errorf("metadata.version is required")
	}
	if plugin.Spec.DockerCompose.Contents == "" {
		return fmt.Errorf("spec.dockerCompose.contents is required")
	}
	if plugin.Spec.Parameters.Schema == "" {
		return fmt.Errorf("spec.parameters.$schema is required")
	}
	if plugin.Spec.Parameters.Type == "" {
		return fmt.Errorf("spec.parameters.type is required")
	}
	if len(plugin.Spec.Parameters.Properties) == 0 {
		return fmt.Errorf("spec.parameters.properties is required")
	}

	// Validate that all required parameters have corresponding properties
	for _, required := range plugin.Spec.Parameters.Required {
		if _, ok := plugin.Spec.Parameters.Properties[required]; !ok {
			return fmt.Errorf("required parameter %s is not defined in properties", required)
		}
	}

	return nil
}

// StopPlugin stops a running plugin deployment
func (pm *PluginManager) StopPlugin(ctx context.Context, plugin *plugintypes.Plugin, store Store) error {
	// Get the deployment metadata to retrieve the original parameters
	deploymentMetadata, err := store.GetDeploymentMetadata(ctx, plugin.Metadata.Name)
	if err != nil {
		return fmt.Errorf("failed to get deployment metadata: %w", err)
	}

	// Extract parameters from metadata
	parameters, ok := deploymentMetadata["parameters"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid deployment metadata: parameters not found")
	}

	// Process x-source parameters to get volume mounts
	processedParameters, volumeMounts, err := pm.processXSourceParameters(ctx, plugin, parameters)
	if err != nil {
		return fmt.Errorf("failed to process x-source parameters: %w", err)
	}

	// Create a temporary directory for the plugin
	tempDir, err := os.MkdirTemp("", plugin.Metadata.Name)
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Add volume mounts to the template data
	templateData := map[string]interface{}{
		"parameters":   processedParameters,
		"volumeMounts": volumeMounts,
	}

	// Render the docker-compose contents as a Go template
	var renderedCompose bytes.Buffer
	tmpl, err := template.New("docker-compose").Parse(plugin.Spec.DockerCompose.Contents)
	if err != nil {
		return fmt.Errorf("failed to parse docker-compose template: %w", err)
	}

	if err := tmpl.Execute(&renderedCompose, templateData); err != nil {
		return fmt.Errorf("failed to render docker-compose template: %w", err)
	}

	// Write the rendered docker-compose contents to a file
	composePath := filepath.Join(tempDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, renderedCompose.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write docker-compose file: %w", err)
	}

	projectOptions := cmdCompose.ProjectOptions{
		ProjectName: plugin.Metadata.Name,
		ConfigPaths: []string{composePath},
	}

	// Turn projectOptions into a project with default values
	projectType, _, err := projectOptions.ToProject(ctx, pm.dockerCli, []string{})
	if err != nil {
		return err
	}

	downOptions := api.DownOptions{
		RemoveOrphans: true,
		Volumes:       true,
	}

	// Stop the project
	err = pm.compose.Down(ctx, projectType.Name, downOptions)
	if err != nil {
		return fmt.Errorf("failed to stop project: %w", err)
	}

	// Clean up volume mounts
	for _, mount := range volumeMounts {
		if err := os.RemoveAll(mount.Source); err != nil {
			pm.logger.Warnf("Failed to clean up volume mount %s: %v", mount.Source, err)
		}
	}

	if err := store.UpdateDeploymentStatus(ctx, plugin.Metadata.Name, "stopped"); err != nil {
		return fmt.Errorf("failed to store plugin status: %w", err)
	}

	return nil
}

// GetPluginStatus gets the current status of a plugin deployment
func (pm *PluginManager) GetPluginStatus(ctx context.Context, plugin *plugintypes.Plugin) (*plugintypes.DeploymentStatus, error) {
	// Create a temporary directory for the plugin
	tempDir, err := os.MkdirTemp("", plugin.Metadata.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Write the docker-compose contents to a file
	composePath := filepath.Join(tempDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(plugin.Spec.DockerCompose.Contents), 0644); err != nil {
		return nil, fmt.Errorf("failed to write docker-compose file: %w", err)
	}

	projectOptions := cmdCompose.ProjectOptions{
		ProjectName: plugin.Metadata.Name,
		ConfigPaths: []string{composePath},
	}

	// Turn projectOptions into a project with default values
	projectType, _, err := projectOptions.ToProject(ctx, pm.dockerCli, []string{})
	if err != nil {
		return nil, err
	}

	// Get project status
	services, err := pm.compose.Ps(ctx, projectType.Name, api.PsOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get project status: %w", err)
	}

	status := &plugintypes.DeploymentStatus{
		Status:      "running",
		StartedAt:   time.Now(),
		ProjectName: plugin.Metadata.Name,
		Services:    make([]plugintypes.Service, len(services)),
	}

	for i, service := range services {
		// Get container details
		container, err := pm.dockerCli.Client().ContainerInspect(ctx, service.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect container: %w", err)
		}

		ports := make([]plugintypes.Port, len(container.NetworkSettings.Ports))
		j := 0
		for port, bindings := range container.NetworkSettings.Ports {
			if len(bindings) > 0 {
				ports[j] = plugintypes.Port{
					HostPort:      bindings[0].HostPort,
					ContainerPort: port.Port(),
					Protocol:      port.Proto(),
				}
				j++
			}
		}
		ports = ports[:j]

		status.Services[i] = plugintypes.Service{
			Name:      service.Names[0][1:], // Remove leading slash
			Status:    service.State,
			Image:     service.Image,
			Ports:     ports,
			CreatedAt: time.Unix(service.Created, 0).Format(time.RFC3339),
		}
	}

	return status, nil
}

// GetDeploymentStatus gets detailed information about a plugin deployment
func (pm *PluginManager) GetDeploymentStatus(ctx context.Context, plugin *plugintypes.Plugin, store Store) (*plugintypes.DeploymentStatus, error) {
	// Get deployment status from store
	status, err := store.GetDeploymentStatus(ctx, plugin.Metadata.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment status from store: %w", err)
	}

	// Create deployment status object
	deploymentStatus := &plugintypes.DeploymentStatus{
		Status:      status,
		ProjectName: plugin.Metadata.Name,
	}

	return deploymentStatus, nil
}

// GetDockerComposeServices retrieves all services with their current status
func (pm *PluginManager) GetDockerComposeServices(ctx context.Context, plugin *plugintypes.Plugin, store Store) ([]ServiceStatus, error) {
	// Get deployment metadata to get the project name and parameters
	deploymentMetadata, err := store.GetDeploymentMetadata(ctx, plugin.Metadata.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment metadata: %w", err)
	}

	// Extract parameters from metadata
	parameters, ok := deploymentMetadata["parameters"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid deployment metadata: parameters not found")
	}

	// Process x-source parameters to get volume mounts
	processedParameters, volumeMounts, err := pm.processXSourceParameters(ctx, plugin, parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to process x-source parameters: %w", err)
	}

	// Create a temporary directory for the plugin
	tempDir, err := os.MkdirTemp("", plugin.Metadata.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Add volume mounts to the template data
	templateData := map[string]interface{}{
		"parameters":   processedParameters,
		"volumeMounts": volumeMounts,
	}

	// Render the docker-compose contents as a Go template
	var renderedCompose bytes.Buffer
	tmpl, err := template.New("docker-compose").Parse(plugin.Spec.DockerCompose.Contents)
	if err != nil {
		return nil, fmt.Errorf("failed to parse docker-compose template: %w", err)
	}

	if err := tmpl.Execute(&renderedCompose, templateData); err != nil {
		return nil, fmt.Errorf("failed to render docker-compose template: %w", err)
	}

	// Write the rendered docker-compose contents to a file
	composePath := filepath.Join(tempDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, renderedCompose.Bytes(), 0644); err != nil {
		return nil, fmt.Errorf("failed to write docker-compose file: %w", err)
	}

	projectOptions := cmdCompose.ProjectOptions{
		ProjectName: plugin.Metadata.Name,
		ConfigPaths: []string{composePath},
	}

	// Turn projectOptions into a project with default values
	project, _, err := projectOptions.ToProject(ctx, pm.dockerCli, []string{})
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	// Get service status using compose ps
	services, err := pm.compose.Ps(ctx, project.Name, api.PsOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get services status: %w", err)
	}

	// Build response
	serviceStatuses := make([]ServiceStatus, 0, len(services))
	for _, svc := range services {
		// Get container details
		container, err := pm.dockerCli.Client().ContainerInspect(ctx, svc.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect container %s: %w", svc.ID, err)
		}

		// Build ports list
		ports := make([]string, 0)
		for containerPort, bindings := range container.NetworkSettings.Ports {
			for _, binding := range bindings {
				ports = append(ports, fmt.Sprintf("%s:%s", binding.HostPort, containerPort.Port()))
			}
		}

		// Build environment map
		env := make(map[string]string)
		for _, envStr := range container.Config.Env {
			parts := strings.SplitN(envStr, "=", 2)
			if len(parts) == 2 {
				env[parts[0]] = parts[1]
			}
		}

		// Build volumes list
		volumes := make([]string, 0)
		for _, mount := range container.Mounts {
			volumes = append(volumes, fmt.Sprintf("%s:%s", mount.Source, mount.Destination))
		}

		// Initialize health status safely
		var healthStatus string
		if container.State.Health != nil {
			healthStatus = container.State.Health.Status
		}

		// Initialize state safely
		state := ""
		if svc.State != "" {
			state = svc.State
		}

		status := ServiceStatus{
			Service: Service{
				Name:        strings.TrimPrefix(svc.Service, "/"), // Remove leading slash if present
				Image:       svc.Image,
				Ports:       ports,
				Environment: env,
				Volumes:     volumes,
				Config: map[string]interface{}{
					"command":     container.Config.Cmd,
					"working_dir": container.Config.WorkingDir,
					"user":        container.Config.User,
				},
			},
			State:      state,
			Running:    state == "running",
			Health:     healthStatus,
			Containers: []string{svc.ID},
			LastError:  container.State.Error,
			CreatedAt:  container.Created,
			StartedAt:  container.State.StartedAt,
		}

		serviceStatuses = append(serviceStatuses, status)
	}

	return serviceStatuses, nil
}

// ResumePlugin resumes a previously deployed plugin
func (pm *PluginManager) ResumePlugin(ctx context.Context, plugin *plugintypes.Plugin, store Store) error {
	// Get the deployment metadata to retrieve the original parameters
	deploymentMetadata, err := store.GetDeploymentMetadata(ctx, plugin.Metadata.Name)
	if err != nil {
		return fmt.Errorf("failed to get deployment metadata: %w", err)
	}

	// Extract parameters from metadata
	parameters, ok := deploymentMetadata["parameters"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid deployment metadata: parameters not found")
	}

	// Process x-source parameters to get volume mounts
	processedParameters, volumeMounts, err := pm.processXSourceParameters(ctx, plugin, parameters)
	if err != nil {
		return fmt.Errorf("failed to process x-source parameters: %w", err)
	}

	// Create a temporary directory for the plugin
	tempDir, err := os.MkdirTemp("", plugin.Metadata.Name)
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Add volume mounts to the template data
	templateData := map[string]interface{}{
		"parameters":   processedParameters,
		"volumeMounts": volumeMounts,
	}

	// Render the docker-compose contents as a Go template
	var renderedCompose bytes.Buffer
	tmpl, err := template.New("docker-compose").Parse(plugin.Spec.DockerCompose.Contents)
	if err != nil {
		return fmt.Errorf("failed to parse docker-compose template: %w", err)
	}

	if err := tmpl.Execute(&renderedCompose, templateData); err != nil {
		return fmt.Errorf("failed to render docker-compose template: %w", err)
	}

	// Write the rendered docker-compose contents to a file
	composePath := filepath.Join(tempDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, renderedCompose.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write docker-compose file: %w", err)
	}

	projectOptions := cmdCompose.ProjectOptions{
		ProjectName: plugin.Metadata.Name,
		ConfigPaths: []string{composePath},
	}

	// Turn projectOptions into a project with default values
	projectType, _, err := projectOptions.ToProject(ctx, pm.dockerCli, []string{})
	if err != nil {
		return err
	}

	// Update plugin status to deploying
	if err := store.UpdateDeploymentStatus(ctx, plugin.Metadata.Name, "deploying"); err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	upOptions := api.UpOptions{
		Create: api.CreateOptions{
			RemoveOrphans: true,
			QuietPull:     true,
		},
		Start: api.StartOptions{
			Wait: true,
		},
	}

	// Start the project
	err = pm.compose.Up(ctx, projectType, upOptions)
	if err != nil {
		_ = store.UpdateDeploymentStatus(ctx, plugin.Metadata.Name, "failed")
		return fmt.Errorf("failed to start project: %w", err)
	}

	// Update status to deployed
	if err := store.UpdateDeploymentStatus(ctx, plugin.Metadata.Name, "deployed"); err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	return nil
}
