package chainlaunchdeploy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/audit"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/hyperledger/fabric-admin-sdk/pkg/chaincode"
)

// fabricDeployerAudit is a helper for audit logging in Fabric deployments
var fabricAuditService *audit.AuditService

// SetFabricAuditService sets the audit service for Fabric deployment logging
func SetFabricAuditService(auditService *audit.AuditService) {
	fabricAuditService = auditService
}

func logFabricAuditEvent(ctx context.Context, eventType string, outcome audit.EventOutcome, deploymentID string, details map[string]interface{}) {
	if fabricAuditService == nil {
		return
	}
	event := audit.NewEvent()
	event.EventSource = "chainlaunchdeploy"
	event.EventType = eventType
	event.EventOutcome = outcome
	event.AffectedResource = deploymentID
	event.Details = details
	event.Timestamp = time.Now().UTC()
	if outcome == audit.EventOutcomeFailure {
		event.Severity = audit.SeverityCritical
	} else {
		event.Severity = audit.SeverityInfo
	}
	if _, err := uuid.Parse(deploymentID); err == nil {
		event.RequestID = uuid.MustParse(deploymentID)
	} else {
		event.RequestID = uuid.New()
	}
	_ = fabricAuditService.LogEvent(ctx, event)
}

func GetCurrentChaincodeSequence(ctx context.Context, gateway *chaincode.Gateway, channelID string, name string) (int64, error) {
	chaincodeDef, err := gateway.QueryCommittedWithName(ctx, channelID, name)
	if err != nil {
		return 0, nil
	}
	return chaincodeDef.GetSequence(), nil
}

// InstallChaincode installs a chaincode package on a Fabric peer, reporting status at each stage.
func InstallChaincode(params FabricChaincodeInstallParams, reporter DeploymentStatusReporter) (DeploymentResult, error) {
	ctx := context.Background()
	deploymentID := fmt.Sprintf("install-%s", params.Label)
	logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_INSTALL_START", audit.EventOutcomePending, deploymentID, map[string]interface{}{
		"label": params.Label,
	})
	reporter.ReportStatus(DeploymentStatusUpdate{
		DeploymentID: deploymentID,
		Status:       StatusPending,
		Message:      "Chaincode install pending",
	})
	if err := ValidateFabricChaincodeInstallParams(params); err != nil {
		logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_INSTALL_VALIDATION_FAILED", audit.EventOutcomeFailure, deploymentID, map[string]interface{}{
			"error": err.Error(),
		})
		reporter.ReportStatus(DeploymentStatusUpdate{
			DeploymentID: deploymentID,
			Status:       StatusFailed,
			Message:      "Validation failed",
			Error:        err,
		})
		return DeploymentResult{Success: false, Error: err}, err
	}
	logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_INSTALL_RUNNING", audit.EventOutcomePending, deploymentID, map[string]interface{}{
		"label": params.Label,
	})
	reporter.ReportStatus(DeploymentStatusUpdate{
		DeploymentID: deploymentID,
		Status:       StatusRunning,
		Message:      "Installing chaincode...",
	})
	// TODO: Implement install logic using fabric-admin-sdk
	logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_INSTALL_SUCCESS", audit.EventOutcomeSuccess, deploymentID, map[string]interface{}{
		"label":  params.Label,
		"result": "Chaincode installed (stub)",
	})
	pkg := params.PackageBytes
	result, err := params.Peer.Install(ctx, bytes.NewReader(pkg))
	if err != nil {
		return DeploymentResult{Success: false, Error: err}, err
	}

	reporter.ReportStatus(DeploymentStatusUpdate{
		DeploymentID: deploymentID,
		Status:       StatusSuccess,
		Message:      fmt.Sprintf("Chaincode installed successfully %s", result.PackageId),
	})
	return DeploymentResult{Success: true, Logs: "Chaincode installed (stub)"}, nil
}

func ApproveChaincode(params FabricChaincodeApproveParams, reporter DeploymentStatusReporter) (DeploymentResult, error) {
	ctx := context.Background()
	deploymentID := fmt.Sprintf("approve-%s-%s", params.Name, params.Version)
	logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_APPROVE_START", audit.EventOutcomePending, deploymentID, map[string]interface{}{
		"name":    params.Name,
		"version": params.Version,
	})
	reporter.ReportStatus(DeploymentStatusUpdate{
		DeploymentID: deploymentID,
		Status:       StatusPending,
		Message:      "Chaincode approve pending",
	})
	if err := ValidateFabricChaincodeApproveParams(params); err != nil {
		logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_APPROVE_VALIDATION_FAILED", audit.EventOutcomeFailure, deploymentID, map[string]interface{}{
			"error": err.Error(),
		})
		reporter.ReportStatus(DeploymentStatusUpdate{
			DeploymentID: deploymentID,
			Status:       StatusFailed,
			Message:      "Validation failed",
			Error:        err,
		})
		return DeploymentResult{Success: false, Error: err}, err
	}
	logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_APPROVE_RUNNING", audit.EventOutcomePending, deploymentID, map[string]interface{}{
		"name":    params.Name,
		"version": params.Version,
	})
	reporter.ReportStatus(DeploymentStatusUpdate{
		DeploymentID: deploymentID,
		Status:       StatusRunning,
		Message:      "Approving chaincode...",
	})
	applicationPolicy, err := chaincode.NewApplicationPolicy(params.EndorsementPolicy, "")
	if err != nil {
		logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_APPROVE_VALIDATION_FAILED", audit.EventOutcomeFailure, deploymentID, map[string]interface{}{
			"error": err.Error(),
		})
		return DeploymentResult{Success: false, Error: err}, err
	}
	chaincodeDef := &chaincode.Definition{
		ChannelName:       params.ChannelID,
		PackageID:         params.PackageID,
		Name:              params.Name,
		Version:           params.Version,
		EndorsementPlugin: "escc",
		ValidationPlugin:  "vscc",
		Sequence:          int64(params.Sequence),
		InitRequired:      false,
		Collections:       nil,
		ApplicationPolicy: applicationPolicy,
	}
	err = params.Gateway.Approve(ctx, chaincodeDef)
	if err != nil {
		if strings.Contains(err.Error(), "redefine uncommitted") {
			logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_APPROVE_SUCCESS", audit.EventOutcomeSuccess, deploymentID, map[string]interface{}{
				"name":    params.Name,
				"version": params.Version,
				"result":  "Chaincode approved (stub)",
			})
		} else {
			logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_APPROVE_VALIDATION_FAILED", audit.EventOutcomeFailure, deploymentID, map[string]interface{}{
				"error": err.Error(),
			})
			return DeploymentResult{Success: false, Error: err}, err
		}
	}
	// TODO: Implement approve logic using fabric-admin-sdk
	logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_APPROVE_SUCCESS", audit.EventOutcomeSuccess, deploymentID, map[string]interface{}{
		"name":    params.Name,
		"version": params.Version,
		"result":  "Chaincode approved (stub)",
	})
	reporter.ReportStatus(DeploymentStatusUpdate{
		DeploymentID: deploymentID,
		Status:       StatusSuccess,
		Message:      "Chaincode approved successfully",
	})
	return DeploymentResult{Success: true, Logs: "Chaincode approved (stub)"}, nil
}

func CommitChaincode(params FabricChaincodeCommitParams, reporter DeploymentStatusReporter) (DeploymentResult, error) {
	ctx := context.Background()
	deploymentID := fmt.Sprintf("commit-%s-%s", params.Name, params.Version)
	logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_COMMIT_START", audit.EventOutcomePending, deploymentID, map[string]interface{}{
		"name":    params.Name,
		"version": params.Version,
	})
	reporter.ReportStatus(DeploymentStatusUpdate{
		DeploymentID: deploymentID,
		Status:       StatusPending,
		Message:      "Chaincode commit pending",
	})
	if err := ValidateFabricChaincodeCommitParams(params); err != nil {
		logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_COMMIT_VALIDATION_FAILED", audit.EventOutcomeFailure, deploymentID, map[string]interface{}{
			"error": err.Error(),
		})
		reporter.ReportStatus(DeploymentStatusUpdate{
			DeploymentID: deploymentID,
			Status:       StatusFailed,
			Message:      "Validation failed",
			Error:        err,
		})
		return DeploymentResult{Success: false, Error: err}, err
	}
	logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_COMMIT_RUNNING", audit.EventOutcomePending, deploymentID, map[string]interface{}{
		"name":    params.Name,
		"version": params.Version,
	})
	reporter.ReportStatus(DeploymentStatusUpdate{
		DeploymentID: deploymentID,
		Status:       StatusRunning,
		Message:      "Committing chaincode...",
	})
	applicationPolicy, err := chaincode.NewApplicationPolicy(params.EndorsementPolicy, "")
	if err != nil {
		logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_COMMIT_VALIDATION_FAILED", audit.EventOutcomeFailure, deploymentID, map[string]interface{}{
			"error": err.Error(),
		})
		return DeploymentResult{Success: false, Error: err}, err
	}

	chaincodeDef := &chaincode.Definition{
		ChannelName:       params.ChannelID,
		Name:              params.Name,
		Version:           params.Version,
		EndorsementPlugin: "escc",
		ValidationPlugin:  "vscc",
		Sequence:          int64(params.Sequence),
		InitRequired:      false,
		Collections:       nil,
		ApplicationPolicy: applicationPolicy,
	}
	err = params.Gateway.Commit(ctx, chaincodeDef)
	if err != nil {
		logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_COMMIT_VALIDATION_FAILED", audit.EventOutcomeFailure, deploymentID, map[string]interface{}{
			"error": err.Error(),
		})
		return DeploymentResult{Success: false, Error: err}, err
	}
	// TODO: Implement commit logic using fabric-admin-sdk
	logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_COMMIT_SUCCESS", audit.EventOutcomeSuccess, deploymentID, map[string]interface{}{
		"name":    params.Name,
		"version": params.Version,
		"result":  "Chaincode committed (stub)",
	})
	reporter.ReportStatus(DeploymentStatusUpdate{
		DeploymentID: deploymentID,
		Status:       StatusSuccess,
		Message:      "Chaincode committed successfully",
	})
	return DeploymentResult{Success: true, Logs: "Chaincode committed (stub)"}, nil
}

func DeployChaincode(params FabricChaincodeDeployParams, reporter DeploymentStatusReporter) (DeploymentResult, error) {
	ctx := context.Background()
	deploymentID := fmt.Sprintf("deploy-%s-%s", params.ApproveParams.Name, params.ApproveParams.Version)
	logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_DEPLOY_START", audit.EventOutcomePending, deploymentID, map[string]interface{}{
		"name":    params.ApproveParams.Name,
		"version": params.ApproveParams.Version,
	})
	reporter.ReportStatus(DeploymentStatusUpdate{
		DeploymentID: deploymentID,
		Status:       StatusPending,
		Message:      "Chaincode deploy pending",
	})
	if err := ValidateFabricChaincodeInstallParams(params.InstallParams); err != nil {
		logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_DEPLOY_INSTALL_VALIDATION_FAILED", audit.EventOutcomeFailure, deploymentID, map[string]interface{}{
			"error": err.Error(),
		})
		reporter.ReportStatus(DeploymentStatusUpdate{
			DeploymentID: deploymentID,
			Status:       StatusFailed,
			Message:      "Install validation failed",
			Error:        err,
		})
		return DeploymentResult{Success: false, Error: err}, err
	}
	if err := ValidateFabricChaincodeApproveParams(params.ApproveParams); err != nil {
		logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_DEPLOY_APPROVE_VALIDATION_FAILED", audit.EventOutcomeFailure, deploymentID, map[string]interface{}{
			"error": err.Error(),
		})
		reporter.ReportStatus(DeploymentStatusUpdate{
			DeploymentID: deploymentID,
			Status:       StatusFailed,
			Message:      "Approve validation failed",
			Error:        err,
		})
		return DeploymentResult{Success: false, Error: err}, err
	}
	if err := ValidateFabricChaincodeCommitParams(params.CommitParams); err != nil {
		logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_DEPLOY_COMMIT_VALIDATION_FAILED", audit.EventOutcomeFailure, deploymentID, map[string]interface{}{
			"error": err.Error(),
		})
		reporter.ReportStatus(DeploymentStatusUpdate{
			DeploymentID: deploymentID,
			Status:       StatusFailed,
			Message:      "Commit validation failed",
			Error:        err,
		})
		return DeploymentResult{Success: false, Error: err}, err
	}
	logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_DEPLOY_RUNNING", audit.EventOutcomePending, deploymentID, map[string]interface{}{
		"name":    params.ApproveParams.Name,
		"version": params.ApproveParams.Version,
	})
	reporter.ReportStatus(DeploymentStatusUpdate{
		DeploymentID: deploymentID,
		Status:       StatusRunning,
		Message:      "Deploying chaincode (install, approve, commit)...",
	})
	// TODO: Implement deploy logic using fabric-admin-sdk
	logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_DEPLOY_SUCCESS", audit.EventOutcomeSuccess, deploymentID, map[string]interface{}{
		"name":    params.ApproveParams.Name,
		"version": params.ApproveParams.Version,
		"result":  "Chaincode deployed (stub)",
	})
	reporter.ReportStatus(DeploymentStatusUpdate{
		DeploymentID: deploymentID,
		Status:       StatusSuccess,
		Message:      "Chaincode deployed successfully",
	})
	return DeploymentResult{Success: true, Logs: "Chaincode deployed (stub)"}, nil
}

// DockerChaincodeDeployer encapsulates Docker logic for chaincode deployment
// (inspired by the robust pattern from the provided example)
type DockerChaincodeDeployer struct {
	client *client.Client
}

// NewDockerChaincodeDeployer creates a new Docker chaincode deployer
func NewDockerChaincodeDeployer() *DockerChaincodeDeployer {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil
	}
	return &DockerChaincodeDeployer{
		client: cli,
	}
}

// findFreePort finds an available port on the host
func findFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, fmt.Errorf("failed to resolve TCP addr: %v", err)
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("failed to listen on TCP port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// pullImageIfNeeded pulls the Docker image if not present locally
func (d *DockerChaincodeDeployer) pullImageIfNeeded(ctx context.Context, imageName string) error {
	_, _, err := d.client.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		return nil // Image exists locally
	}
	reader, err := d.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %v", imageName, err)
	}
	defer reader.Close()
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("error while pulling image: %v", err)
	}
	return nil
}

// sanitizeContainerName replaces any character not in [a-zA-Z0-9_.-] with '-'
func sanitizeContainerName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_.-]`)
	return re.ReplaceAllString(name, "-")
}

// Deploy deploys a chaincode container using Docker
func (d *DockerChaincodeDeployer) Deploy(params DockerDeployParams, reporter DeploymentStatusReporter) (DeploymentResult, error) {
	ctx := context.Background()
	deploymentID := fmt.Sprintf("docker-%s-%s", params.DockerImage, params.PackageID)
	logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_DOCKER_DEPLOY_START", audit.EventOutcomePending, deploymentID, map[string]interface{}{
		"dockerImage":   params.DockerImage,
		"packageID":     params.PackageID,
		"hostPort":      params.HostPort,
		"containerPort": params.ContainerPort,
	})
	reporter.ReportStatus(DeploymentStatusUpdate{
		DeploymentID: deploymentID,
		Status:       StatusPending,
		Message:      "Chaincode Docker deployment pending",
	})

	// Pull image if needed
	if err := d.pullImageIfNeeded(ctx, params.DockerImage); err != nil {
		logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_DOCKER_DEPLOY_FAILED", audit.EventOutcomeFailure, deploymentID, map[string]interface{}{"error": err.Error()})
		reporter.ReportStatus(DeploymentStatusUpdate{
			DeploymentID: deploymentID,
			Status:       StatusFailed,
			Message:      "Failed to pull Docker image",
			Error:        err,
		})
		return DeploymentResult{Success: false, Error: err}, err
	}

	// Determine host port
	hostPort := params.HostPort
	if hostPort == "" {
		freePort, err := findFreePort()
		if err != nil {
			logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_DOCKER_DEPLOY_FAILED", audit.EventOutcomeFailure, deploymentID, map[string]interface{}{"error": err.Error()})
			reporter.ReportStatus(DeploymentStatusUpdate{
				DeploymentID: deploymentID,
				Status:       StatusFailed,
				Message:      "Failed to find free port",
				Error:        err,
			})
			return DeploymentResult{Success: false, Error: err}, err
		}
		hostPort = strconv.Itoa(freePort)
	}

	// Determine container port
	containerPort := params.ContainerPort
	if containerPort == "" {
		containerPort = "7052"
	}

	safePackageID := sanitizeContainerName(params.PackageID)
	containerName := fmt.Sprintf("chaincode-%s-%s", safePackageID, hostPort)
	// Remove existing container if it exists
	var containersToRemove []string
	if defID, ok := params.Labels["chainlaunch.chaincode.definition_id"]; ok {
		filterArgs := filters.NewArgs()
		filterArgs.Add("label", fmt.Sprintf("chainlaunch.chaincode.definition_id=%s", defID))
		labeledContainers, err := d.client.ContainerList(ctx, container.ListOptions{All: true, Filters: filterArgs})
		if err == nil {
			for _, c := range labeledContainers {
				containersToRemove = append(containersToRemove, c.ID)
			}
		}
	}
	// Fallback: also check by container name (legacy)
	containers, err := d.client.ContainerList(ctx, container.ListOptions{All: true})
	if err == nil {
		for _, c := range containers {
			for _, name := range c.Names {
				if name == "/"+containerName {
					containersToRemove = append(containersToRemove, c.ID)
				}
			}
		}
	}
	for _, id := range containersToRemove {
		_ = d.client.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
	}

	env := []string{
		fmt.Sprintf("CHAINCODE_ID=%s", params.PackageID),
		fmt.Sprintf("CORE_CHAINCODE_ID=%s", params.PackageID),
		fmt.Sprintf("CORE_CHAINCODE_ID_NAME=%s", params.PackageID),
	}
	// Chaincode address logic
	chaincodeAddress := params.ChaincodeAddress // Assume this field exists in params, or add as needed
	if params.HostPort != "" {
		chaincodeAddress = fmt.Sprintf("0.0.0.0:%s", params.ContainerPort)
	} else {
		chaincodeAddress = fmt.Sprintf("0.0.0.0:%s", containerPort)
	}
	// Use chaincodeAddress for deployment (set in params or pass to Docker, etc.)
	if chaincodeAddress != "" {
		env = append(env,
			fmt.Sprintf("CHAINCODE_SERVER_ADDRESS=%s", chaincodeAddress),
			fmt.Sprintf("CORE_CHAINCODE_ADDRESS=%s", chaincodeAddress),
		)
	}

	// Add custom environment variables
	for key, value := range params.EnvironmentVariables {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	exposedPort := nat.Port(fmt.Sprintf("%s/tcp", containerPort))
	config := &container.Config{
		Image:        params.DockerImage,
		Env:          env,
		Labels:       params.Labels,
		ExposedPorts: nat.PortSet{exposedPort: struct{}{}},
		Cmd:          []string{},
	}
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			exposedPort: []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: hostPort}},
		},
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
	}

	resp, err := d.client.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_DOCKER_DEPLOY_FAILED", audit.EventOutcomeFailure, deploymentID, map[string]interface{}{"error": err.Error()})
		reporter.ReportStatus(DeploymentStatusUpdate{
			DeploymentID: deploymentID,
			Status:       StatusFailed,
			Message:      "Failed to create Docker container",
			Error:        err,
		})
		return DeploymentResult{Success: false, Error: err}, err
	}

	if err := d.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_DOCKER_DEPLOY_FAILED", audit.EventOutcomeFailure, deploymentID, map[string]interface{}{"error": err.Error()})
		reporter.ReportStatus(DeploymentStatusUpdate{
			DeploymentID: deploymentID,
			Status:       StatusFailed,
			Message:      "Failed to start Docker container",
			Error:        err,
		})
		return DeploymentResult{Success: false, Error: err}, err
	}

	logFabricAuditEvent(ctx, "FABRIC_CHAINCODE_DOCKER_DEPLOY_SUCCESS", audit.EventOutcomeSuccess, deploymentID, map[string]interface{}{
		"dockerImage":   params.DockerImage,
		"packageID":     params.PackageID,
		"hostPort":      hostPort,
		"containerPort": containerPort,
		"container":     containerName,
	})
	reporter.ReportStatus(DeploymentStatusUpdate{
		DeploymentID: deploymentID,
		Status:       StatusSuccess,
		Message:      "Chaincode Docker container started successfully",
	})
	return DeploymentResult{
		Success:     true,
		Logs:        fmt.Sprintf("Container %s started with image %s (host port %s -> container port %s)", containerName, params.DockerImage, hostPort, containerPort),
		ChaincodeID: params.PackageID,
	}, nil
}

// DeployChaincodeWithDockerImage is a wrapper for DockerChaincodeDeployer.Deploy
func DeployChaincodeWithDockerImage(dockerImage, packageID, hostPort, containerPort string, reporter DeploymentStatusReporter) (DeploymentResult, error) {
	params := DockerDeployParams{
		DockerImage:   dockerImage,
		PackageID:     packageID,
		HostPort:      hostPort,
		ContainerPort: containerPort,
	}
	deployer := NewDockerChaincodeDeployer()
	return deployer.Deploy(params, reporter)
}

// DeployChaincodeWithDockerImageWithLabels is a wrapper for DockerChaincodeDeployer.Deploy that adds Docker labels
func DeployChaincodeWithDockerImageWithLabels(dockerImage, packageID, hostPort, containerPort string, labels map[string]string, reporter DeploymentStatusReporter) (DeploymentResult, error) {
	params := DockerDeployParams{
		DockerImage:   dockerImage,
		PackageID:     packageID,
		HostPort:      hostPort,
		ContainerPort: containerPort,
		Labels:        labels,
	}
	deployer := NewDockerChaincodeDeployer()
	return deployer.Deploy(params, reporter)
}

// DeployChaincodeWithDockerImageWithEnvironmentVariables is a wrapper for DockerChaincodeDeployer.Deploy that adds environment variables
func DeployChaincodeWithDockerImageWithEnvironmentVariables(dockerImage, packageID, hostPort, containerPort string, environmentVariables map[string]string, reporter DeploymentStatusReporter) (DeploymentResult, error) {
	params := DockerDeployParams{
		DockerImage:          dockerImage,
		PackageID:            packageID,
		HostPort:             hostPort,
		ContainerPort:        containerPort,
		EnvironmentVariables: environmentVariables,
	}
	deployer := NewDockerChaincodeDeployer()
	return deployer.Deploy(params, reporter)
}

// DeployChaincodeWithDockerImageWithLabelsAndEnvironmentVariables is a wrapper for DockerChaincodeDeployer.Deploy that adds both labels and environment variables
func DeployChaincodeWithDockerImageWithLabelsAndEnvironmentVariables(dockerImage, packageID, hostPort, containerPort string, labels map[string]string, environmentVariables map[string]string, reporter DeploymentStatusReporter) (DeploymentResult, error) {
	params := DockerDeployParams{
		DockerImage:          dockerImage,
		PackageID:            packageID,
		HostPort:             hostPort,
		ContainerPort:        containerPort,
		Labels:               labels,
		EnvironmentVariables: environmentVariables,
	}
	deployer := NewDockerChaincodeDeployer()
	return deployer.Deploy(params, reporter)
}

type fabricDeployer struct{}

// NewFabricDeployer returns a new instance of the Fabric deployer
func NewFabricDeployer() Deployer {
	return &fabricDeployer{}
}

// Install installs a chaincode package on a Fabric peer
func (d *fabricDeployer) Install(params FabricChaincodeInstallParams, reporter DeploymentStatusReporter) (DeploymentResult, error) {
	return InstallChaincode(params, reporter)
}

// Approve approves a chaincode definition for an organization
func (d *fabricDeployer) Approve(params FabricChaincodeApproveParams, reporter DeploymentStatusReporter) (DeploymentResult, error) {
	return ApproveChaincode(params, reporter)
}

// Commit commits a chaincode definition to the channel
func (d *fabricDeployer) Commit(params FabricChaincodeCommitParams, reporter DeploymentStatusReporter) (DeploymentResult, error) {
	return CommitChaincode(params, reporter)
}

// DeployFabricContract implements the Deployer interface for Fabric
func (d *fabricDeployer) DeployFabricContract(params FabricChaincodeDeployParams, reporter DeploymentStatusReporter) (DeploymentResult, error) {
	installResult, err := d.Install(params.InstallParams, reporter)
	if err != nil {
		return installResult, err
	}
	approveResult, err := d.Approve(params.ApproveParams, reporter)
	if err != nil {
		return approveResult, err
	}
	commitResult, err := d.Commit(params.CommitParams, reporter)
	if err != nil {
		return commitResult, err
	}
	return commitResult, nil
}

// DeployEVMContract is not supported for fabricDeployer
func (d *fabricDeployer) DeployEVMContract(params EVMParams, reporter DeploymentStatusReporter) (DeploymentResult, error) {
	return DeploymentResult{Success: false, Error: fmt.Errorf("DeployEVMContract not supported for Fabric")}, fmt.Errorf("DeployEVMContract not supported for Fabric")
}

// DockerChaincodeDockerDeployParams defines parameters for deploying chaincode using a Docker image.
// - DockerImage: the image to use
// - PackageID: the chaincode package ID
// - HostPort: the port to listen on the host (if empty, a free port is chosen)
// - ContainerPort: the port to map to inside the container (default "7052" if empty)
type DockerDeployParams struct {
	DockerImage          string            `json:"dockerImage"`
	PackageID            string            `json:"packageID"`
	HostPort             string            `json:"hostPort"`         // Host port to listen on
	ContainerPort        string            `json:"containerPort"`    // Container port to map to (default 7052)
	ChaincodeAddress     string            `json:"chaincodeAddress"` // Chaincode address to use
	Labels               map[string]string `json:"labels"`
	EnvironmentVariables map[string]string `json:"environmentVariables"` // Custom environment variables
}
