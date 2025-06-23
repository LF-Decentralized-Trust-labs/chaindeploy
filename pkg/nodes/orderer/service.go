package orderer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// startService starts the orderer as a system service
func (o *LocalOrderer) startService(cmd string, env map[string]string, dirPath string) (*StartServiceResponse, error) {
	platform := runtime.GOOS
	switch platform {
	case "linux":
		if err := o.createSystemdService(cmd, env, dirPath); err != nil {
			return nil, fmt.Errorf("failed to create systemd service: %w", err)
		}
		if err := o.startSystemdService(); err != nil {
			return nil, fmt.Errorf("failed to start systemd service: %w", err)
		}
		return &StartServiceResponse{
			Mode:        "service",
			Type:        "systemd",
			ServiceName: o.getServiceName(),
		}, nil

	case "darwin":
		if err := o.createLaunchdService(cmd, env, dirPath); err != nil {
			return nil, fmt.Errorf("failed to create launchd service: %w", err)
		}
		if err := o.startLaunchdService(); err != nil {
			return nil, fmt.Errorf("failed to start launchd service: %w", err)
		}
		return &StartServiceResponse{
			Mode:        "service",
			Type:        "launchd",
			ServiceName: o.getLaunchdServiceName(),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported platform for service mode: %s", platform)
	}
}

// createSystemdService creates a systemd service file
func (o *LocalOrderer) createSystemdService(cmd string, env map[string]string, dirPath string) error {
	var envStrings []string
	for k, v := range env {
		envStrings = append(envStrings, fmt.Sprintf("Environment=\"%s=%s\"", k, v))
	}

	tmpl := template.Must(template.New("systemd").Parse(`
[Unit]
Description=Hyperledger Fabric Orderer - {{.ID}}
After=network.target

[Service]
Type=simple
WorkingDirectory={{.DirPath}}
ExecStart={{.Cmd}}
Restart=on-failure
RestartSec=10
LimitNOFILE=65536
{{range .EnvVars}}{{.}}
{{end}}

[Install]
WantedBy=multi-user.target
`))

	data := struct {
		ID      string
		DirPath string
		Cmd     string
		EnvVars []string
	}{
		ID:      o.opts.ID,
		DirPath: dirPath,
		Cmd:     cmd,
		EnvVars: envStrings,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.WriteFile(o.getServiceFilePath(), buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write systemd service file: %w", err)
	}

	return nil
}

// createLaunchdService creates a launchd service file
func (o *LocalOrderer) createLaunchdService(cmd string, env map[string]string, dirPath string) error {
	var envStrings []string
	for k, v := range env {
		envStrings = append(envStrings, fmt.Sprintf("<key>%s</key>\n    <string>%s</string>", k, v))
	}

	tmpl := template.Must(template.New("launchd").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>{{.ServiceName}}</string>
  <key>ProgramArguments</key>
  <array>
      <string>/bin/bash</string>
      <string>-c</string>
      <string>{{.Cmd}}</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>StandardOutPath</key>
  <string>{{.LogPath}}</string>
  <key>StandardErrorPath</key>
  <string>{{.LogPath}}</string>
  <key>EnvironmentVariables</key>
  <dict>
    {{range .EnvVars}}{{.}}
    {{end}}
  </dict>
</dict>
</plist>`))

	data := struct {
		ServiceName string
		Cmd         string
		LogPath     string
		EnvVars     []string
	}{
		ServiceName: o.getLaunchdServiceName(),
		Cmd:         cmd,
		LogPath:     filepath.Join(dirPath, o.getServiceName()+".log"),
		EnvVars:     envStrings,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.WriteFile(o.getLaunchdPlistPath(), buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write launchd service file: %w", err)
	}

	return nil
}

// startSystemdService starts the systemd service
func (o *LocalOrderer) startSystemdService() error {
	if err := o.execSystemctl("daemon-reload"); err != nil {
		return err
	}
	if err := o.execSystemctl("enable", o.getServiceName()); err != nil {
		return err
	}
	if err := o.execSystemctl("start", o.getServiceName()); err != nil {
		return err
	}
	return o.execSystemctl("restart", o.getServiceName())
}

// startLaunchdService starts the launchd service
func (o *LocalOrderer) startLaunchdService() error {
	cmd := exec.Command("launchctl", "load", o.getLaunchdPlistPath())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to load launchd service: %w", err)
	}

	cmd = exec.Command("launchctl", "start", o.getLaunchdServiceName())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start launchd service: %w", err)
	}

	return nil
}

// stopSystemdService stops the systemd service
func (o *LocalOrderer) stopSystemdService() error {
	serviceName := o.getServiceName()

	// Stop the service
	if err := o.execSystemctl("stop", serviceName); err != nil {
		return fmt.Errorf("failed to stop systemd service: %w", err)
	}

	// Disable the service
	if err := o.execSystemctl("disable", serviceName); err != nil {
		o.logger.Warn("Failed to disable systemd service", "error", err)
	}

	// Remove the service file
	if err := os.Remove(o.getServiceFilePath()); err != nil {
		if !os.IsNotExist(err) {
			o.logger.Warn("Failed to remove service file", "error", err)
		}
	}

	// Reload systemd daemon
	if err := o.execSystemctl("daemon-reload"); err != nil {
		o.logger.Warn("Failed to reload systemd daemon", "error", err)
	}

	return nil
}

// stopLaunchdService stops the launchd service
func (o *LocalOrderer) stopLaunchdService() error {
	// Stop the service
	stopCmd := exec.Command("launchctl", "stop", o.getLaunchdServiceName())
	if err := stopCmd.Run(); err != nil {
		o.logger.Warn("Failed to stop launchd service", "error", err)
	}

	// Unload the service
	unloadCmd := exec.Command("launchctl", "unload", o.getLaunchdPlistPath())
	if err := unloadCmd.Run(); err != nil {
		return fmt.Errorf("failed to unload launchd service: %w", err)
	}

	return nil
}

// execSystemctl executes a systemctl command
func (o *LocalOrderer) execSystemctl(command string, args ...string) error {
	cmdArgs := append([]string{command}, args...)

	// Check if sudo is available
	sudoPath, err := exec.LookPath("sudo")
	if err == nil {
		// sudo is available, use it
		cmdArgs = append([]string{"systemctl"}, cmdArgs...)
		cmd := exec.Command(sudoPath, cmdArgs...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("systemctl %s failed: %w", command, err)
		}
	} else {
		// sudo is not available, run directly
		cmd := exec.Command("systemctl", cmdArgs...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("systemctl %s failed: %w", command, err)
		}
	}

	return nil
}

// getContainerName returns the docker container name for the orderer
func (o *LocalOrderer) getContainerName() string {
	return strings.ReplaceAll(strings.ToLower(o.opts.ID), " ", "-")
}

// startDocker starts the orderer in a docker container
func (o *LocalOrderer) startDocker(env map[string]string, mspConfigPath, dataConfigPath string) (*StartDockerResponse, error) {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	// Pull the image first
	imageName := fmt.Sprintf("hyperledger/fabric-orderer:%s", o.opts.Version)
	reader, err := cli.ImagePull(context.Background(), imageName, image.PullOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()
	_, err = io.Copy(io.Discard, reader) // Wait for pull to complete
	if err != nil {
		return nil, fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}

	containerName := o.getContainerName()

	// Helper to extract port from address (host:port or just :port)
	extractPort := func(addr string) string {
		parts := strings.Split(addr, ":")
		if len(parts) > 1 {
			return parts[len(parts)-1]
		}
		return addr
	}

	listenPort := extractPort(o.opts.ListenAddress)
	adminPort := extractPort(o.opts.AdminListenAddress)
	operationsPort := extractPort(o.opts.OperationsListenAddress)

	// Configure port bindings
	portBindings := map[nat.Port][]nat.PortBinding{
		nat.Port(listenPort):     {{HostIP: "0.0.0.0", HostPort: listenPort}},
		nat.Port(adminPort):      {{HostIP: "0.0.0.0", HostPort: adminPort}},
		nat.Port(operationsPort): {{HostIP: "0.0.0.0", HostPort: operationsPort}},
	}

	// Configure volume bindings
	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: mspConfigPath,
			Target: "/etc/hyperledger/fabric/msp",
		},
		{
			Type:   mount.TypeBind,
			Source: dataConfigPath,
			Target: "/var/hyperledger/production",
		},
	}
	containerConfig := &container.Config{
		Image:        imageName,
		Cmd:          []string{"orderer"},
		Env:          mapToEnvSlice(env),
		ExposedPorts: map[nat.Port]struct{}{},
	}
	for port := range portBindings {
		containerConfig.ExposedPorts[port] = struct{}{}
	}
	// Create container
	resp, err := cli.ContainerCreate(context.Background(),
		containerConfig,
		&container.HostConfig{
			PortBindings: portBindings,
			Mounts:       mounts,
		},
		nil,
		nil,
		containerName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := cli.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	return &StartDockerResponse{
		Mode:          "docker",
		ContainerName: containerName,
	}, nil
}

func mapToEnvSlice(m map[string]string) []string {
	var env []string
	for k, v := range m {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

func (o *LocalOrderer) stopDocker() error {
	containerName := o.getContainerName()

	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	ctx := context.Background()

	// Stop and remove container
	if err := cli.ContainerRemove(ctx, containerName, container.RemoveOptions{
		Force: true,
	}); err != nil {
		o.logger.Warn("Failed to remove docker container", "error", err)
		// Don't return error as container might not exist
	}

	return nil
}
