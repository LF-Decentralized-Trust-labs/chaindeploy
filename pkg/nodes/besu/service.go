package besu

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

// getServiceName returns the systemd service name
func (b *LocalBesu) getServiceName() string {
	return fmt.Sprintf("besu-%s", strings.ReplaceAll(strings.ToLower(b.opts.ID), " ", "-"))
}

// getLaunchdServiceName returns the launchd service name
func (b *LocalBesu) getLaunchdServiceName() string {
	return fmt.Sprintf("dev.chainlaunch.besu.%s",
		strings.ReplaceAll(strings.ToLower(b.opts.ID), " ", "-"))
}

// getServiceFilePath returns the systemd service file path
func (b *LocalBesu) getServiceFilePath() string {
	return fmt.Sprintf("/etc/systemd/system/%s.service", b.getServiceName())
}

// getLaunchdPlistPath returns the launchd plist file path
func (b *LocalBesu) getLaunchdPlistPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "Library/LaunchAgents", b.getLaunchdServiceName()+".plist")
}

// startService starts the besu as a system service
func (b *LocalBesu) startService(cmd string, env map[string]string, dirPath, configDir string) (*StartServiceResponse, error) {
	// Write genesis file to config directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write genesis file
	genesisPath := filepath.Join(configDir, "genesis.json")
	if err := os.WriteFile(genesisPath, []byte(b.opts.GenesisFile), 0644); err != nil {
		return nil, fmt.Errorf("failed to write genesis file: %w", err)
	}

	// Write private key file
	keyPath := filepath.Join(configDir, "key")
	if err := os.WriteFile(keyPath, []byte(b.opts.NodePrivateKey), 0600); err != nil {
		return nil, fmt.Errorf("failed to write key file: %w", err)
	}

	platform := runtime.GOOS
	switch platform {
	case "linux":
		if err := b.createSystemdService(cmd, env, dirPath, genesisPath, keyPath); err != nil {
			return nil, fmt.Errorf("failed to create systemd service: %w", err)
		}
		if err := b.startSystemdService(); err != nil {
			return nil, fmt.Errorf("failed to start systemd service: %w", err)
		}
		return &StartServiceResponse{
			Mode:        "service",
			Type:        "systemd",
			ServiceName: b.getServiceName(),
		}, nil

	case "darwin":
		if err := b.createLaunchdService(cmd, env, dirPath, genesisPath, keyPath); err != nil {
			return nil, fmt.Errorf("failed to create launchd service: %w", err)
		}
		if err := b.startLaunchdService(); err != nil {
			return nil, fmt.Errorf("failed to start launchd service: %w", err)
		}
		return &StartServiceResponse{
			Mode:        "service",
			Type:        "launchd",
			ServiceName: b.getLaunchdServiceName(),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported platform for service mode: %s", platform)
	}
}

// createSystemdService creates a systemd service file
func (b *LocalBesu) createSystemdService(cmd string, env map[string]string, dirPath, genesisPath, keyPath string) error {
	var envStrings []string
	for k, v := range env {
		envStrings = append(envStrings, fmt.Sprintf("Environment=\"%s=%s\"", k, v))
	}

	tmpl := template.Must(template.New("systemd").Parse(`
[Unit]
Description=Hyperledger Besu Node - {{.ID}}
After=network.target

[Service]
Type=simple
WorkingDirectory={{.DirPath}}
ExecStart={{.Cmd}}
Restart=on-failure
RestartSec=10
LimitNOFILE=65536
StandardOutput=append:{{.LogPath}}
StandardError=append:{{.LogPath}}
{{range .EnvVars}}{{.}}
{{end}}

[Install]
WantedBy=multi-user.target
`))

	data := struct {
		ID      string
		DirPath string
		Cmd     string
		LogPath string
		EnvVars []string
	}{
		ID:      b.opts.ID,
		DirPath: dirPath,
		Cmd:     cmd,
		LogPath: b.GetStdOutPath(),
		EnvVars: envStrings,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.WriteFile(b.getServiceFilePath(), buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write systemd service file: %w", err)
	}

	return nil
}

// createLaunchdService creates a launchd service file
func (b *LocalBesu) createLaunchdService(cmd string, env map[string]string, dirPath, genesisPath, keyPath string) error {
	var envStrings []string
	for k, v := range env {
		envStrings = append(envStrings, fmt.Sprintf("<key>%s</key>\n    <string>%s</string>", k, v))
	}

	// Build command using the common builder

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
		ServiceName: b.getLaunchdServiceName(),
		Cmd:         cmd,
		LogPath:     filepath.Join(dirPath, b.getServiceName()+".log"),
		EnvVars:     envStrings,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.WriteFile(b.getLaunchdPlistPath(), buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write launchd service file: %w", err)
	}

	return nil
}

// startSystemdService starts the systemd service
func (b *LocalBesu) startSystemdService() error {
	if err := b.execSystemctl("daemon-reload"); err != nil {
		return err
	}
	if err := b.execSystemctl("enable", b.getServiceName()); err != nil {
		return err
	}
	if err := b.execSystemctl("start", b.getServiceName()); err != nil {
		return err
	}
	return b.execSystemctl("restart", b.getServiceName())
}

func (b *LocalBesu) GetStdOutPath() string {
	return filepath.Join(b.configService.GetDataPath(), "besu", strings.ReplaceAll(strings.ToLower(b.opts.ID), " ", "-"), b.getServiceName()+".log")
}

// startLaunchdService starts the launchd service
func (b *LocalBesu) startLaunchdService() error {
	cmd := exec.Command("launchctl", "load", b.getLaunchdPlistPath())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to load launchd service: %w", err)
	}

	cmd = exec.Command("launchctl", "start", b.getLaunchdServiceName())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start launchd service: %w", err)
	}

	return nil
}

// stopSystemdService stops the systemd service
func (b *LocalBesu) stopSystemdService() error {
	serviceName := b.getServiceName()

	// Stop the service
	if err := b.execSystemctl("stop", serviceName); err != nil {
		return fmt.Errorf("failed to stop systemd service: %w", err)
	}

	// Disable the service
	if err := b.execSystemctl("disable", serviceName); err != nil {
		b.logger.Warn("Failed to disable systemd service", "error", err)
	}

	// Remove the service file
	if err := os.Remove(b.getServiceFilePath()); err != nil {
		if !os.IsNotExist(err) {
			b.logger.Warn("Failed to remove service file", "error", err)
		}
	}

	// Reload systemd daemon
	if err := b.execSystemctl("daemon-reload"); err != nil {
		b.logger.Warn("Failed to reload systemd daemon", "error", err)
	}

	return nil
}

// stopLaunchdService stops the launchd service
func (b *LocalBesu) stopLaunchdService() error {
	// Stop the service
	stopCmd := exec.Command("launchctl", "stop", b.getLaunchdServiceName())
	if err := stopCmd.Run(); err != nil {
		b.logger.Warn("Failed to stop launchd service", "error", err)
	}

	// Unload the service
	unloadCmd := exec.Command("launchctl", "unload", b.getLaunchdPlistPath())
	if err := unloadCmd.Run(); err != nil {
		return fmt.Errorf("failed to unload launchd service: %w", err)
	}

	return nil
}

// execSystemctl executes a systemctl command
func (b *LocalBesu) execSystemctl(command string, args ...string) error {
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
