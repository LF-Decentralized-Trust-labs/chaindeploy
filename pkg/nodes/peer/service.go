package peer

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

// startService starts the peer as a system service
func (p *LocalPeer) startService(cmd string, env map[string]string, dirPath string) (*StartServiceResponse, error) {
	platform := runtime.GOOS
	switch platform {
	case "linux":
		if err := p.createSystemdService(cmd, env, dirPath); err != nil {
			return nil, fmt.Errorf("failed to create systemd service: %w", err)
		}
		if err := p.startSystemdService(); err != nil {
			return nil, fmt.Errorf("failed to start systemd service: %w", err)
		}
		return &StartServiceResponse{
			Mode:        "service",
			Type:        "systemd",
			ServiceName: p.getServiceName(),
		}, nil

	case "darwin":
		if err := p.createLaunchdService(cmd, env, dirPath); err != nil {
			return nil, fmt.Errorf("failed to create launchd service: %w", err)
		}
		if err := p.startLaunchdService(); err != nil {
			return nil, fmt.Errorf("failed to start launchd service: %w", err)
		}
		return &StartServiceResponse{
			Mode:        "service",
			Type:        "launchd",
			ServiceName: p.getLaunchdServiceName(),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported platform for service mode: %s", platform)
	}
}

// createSystemdService creates a systemd service file
func (p *LocalPeer) createSystemdService(cmd string, env map[string]string, dirPath string) error {
	var envStrings []string
	for k, v := range env {
		envStrings = append(envStrings, fmt.Sprintf("Environment=\"%s=%s\"", k, v))
	}

	tmpl := template.Must(template.New("systemd").Parse(`
[Unit]
Description=Hyperledger Fabric Peer - {{.ID}}
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
		ID:      p.opts.ID,
		DirPath: dirPath,
		Cmd:     cmd,
		LogPath: p.GetStdOutPath(),
		EnvVars: envStrings,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.WriteFile(p.getServiceFilePath(), buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write systemd service file: %w", err)
	}

	return nil
}

// createLaunchdService creates a launchd service file
func (p *LocalPeer) createLaunchdService(cmd string, env map[string]string, dirPath string) error {
	var envStrings []string
	for k, v := range env {
		envStrings = append(envStrings, fmt.Sprintf("<key>%s</key>\n    <string>%s</string>", k, v))
	}

	// Escape special XML characters in cmd
	cmd = strings.ReplaceAll(cmd, "&", "&amp;")
	cmd = strings.ReplaceAll(cmd, "<", "&lt;")
	cmd = strings.ReplaceAll(cmd, ">", "&gt;")
	cmd = strings.ReplaceAll(cmd, "'", "&apos;")
	cmd = strings.ReplaceAll(cmd, "\"", "&quot;")

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
		ServiceName: p.getLaunchdServiceName(),
		Cmd:         cmd,
		LogPath:     filepath.Join(dirPath, p.getServiceName()+".log"),
		EnvVars:     envStrings,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.WriteFile(p.getLaunchdPlistPath(), buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write launchd service file: %w", err)
	}

	return nil
}

// startSystemdService starts the systemd service
func (p *LocalPeer) startSystemdService() error {
	if err := p.execSystemctl("daemon-reload"); err != nil {
		return err
	}
	if err := p.execSystemctl("enable", p.getServiceName()); err != nil {
		return err
	}
	if err := p.execSystemctl("start", p.getServiceName()); err != nil {
		return err
	}
	return p.execSystemctl("restart", p.getServiceName())
}

// startLaunchdService starts the launchd service
func (p *LocalPeer) startLaunchdService() error {
	// Try to stop existing service first
	// _ = p.stopLaunchdService()

	cmd := exec.Command("launchctl", "load", p.getLaunchdPlistPath())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to load launchd service: %w", err)
	}

	cmd = exec.Command("launchctl", "start", p.getLaunchdServiceName())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start launchd service: %w", err)
	}

	return nil
}
