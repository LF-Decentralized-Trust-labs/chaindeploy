package besu

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/chainlaunch/chainlaunch/pkg/config"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/types"
	settingsservice "github.com/chainlaunch/chainlaunch/pkg/settings/service"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// LocalBesu represents a local Besu node
type LocalBesu struct {
	opts            StartBesuOpts
	mode            string
	nodeID          int64
	NetworkConfig   types.BesuNetworkConfig
	logger          *logger.Logger
	configService   *config.ConfigService
	settingsService *settingsservice.SettingsService
}

// NewLocalBesu creates a new LocalBesu instance
func NewLocalBesu(
	opts StartBesuOpts,
	mode string,
	nodeID int64,
	logger *logger.Logger,
	configService *config.ConfigService,
	settingsService *settingsservice.SettingsService,
	networkConfig types.BesuNetworkConfig,
) *LocalBesu {
	return &LocalBesu{
		opts:            opts,
		mode:            mode,
		nodeID:          nodeID,
		logger:          logger,
		configService:   configService,
		settingsService: settingsService,
		NetworkConfig:   networkConfig,
	}
}

// Start starts the Besu node
func (b *LocalBesu) Start() (interface{}, error) {
	b.logger.Info("Starting Besu node", "opts", b.opts)

	// Create necessary directories
	chainlaunchDir := b.configService.GetDataPath()

	slugifiedID := strings.ReplaceAll(strings.ToLower(b.opts.ID), " ", "-")
	dirPath := filepath.Join(chainlaunchDir, "besu", slugifiedID)
	dataDir := filepath.Join(dirPath, "data")
	configDir := filepath.Join(dirPath, "config")
	binDir := filepath.Join(chainlaunchDir, "bin/besu", b.opts.Version)

	// Create directories
	for _, dir := range []string{dataDir, configDir, binDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Install Besu if not exists
	if err := b.installBesu(); err != nil {
		return nil, fmt.Errorf("failed to install Besu: %w", err)
	}

	// Log which binary will be used for the service
	b.logServiceBinaryPath()

	// Verify binary exists and is executable
	if err := b.verifyBinary(); err != nil {
		return nil, fmt.Errorf("binary verification failed: %w", err)
	}

	// Write genesis file to config directory
	genesisPath := filepath.Join(configDir, "genesis.json")
	if err := os.WriteFile(genesisPath, []byte(b.opts.GenesisFile), 0644); err != nil {
		return nil, fmt.Errorf("failed to write genesis file: %w", err)
	}

	// Check prerequisites based on mode
	if err := b.checkPrerequisites(); err != nil {
		return nil, fmt.Errorf("prerequisites check failed: %w", err)
	}

	// Build command and environment
	cmd := b.buildCommand(dataDir, genesisPath, configDir)

	switch b.mode {
	case "service":
		env := b.buildEnvironment()
		return b.startService(cmd, env, dirPath, configDir)
	case "docker":
		env := b.buildDockerEnvironment()
		return b.startDocker(env, dataDir, configDir)
	default:
		return nil, fmt.Errorf("invalid mode: %s", b.mode)
	}
}

// GetBinaryPathInfo returns information about the Besu binary paths
func (b *LocalBesu) GetBinaryPathInfo() map[string]string {
	info := map[string]string{
		"downloaded_path": b.GetBinaryPath(),
		"service_path":    b.GetServiceBinaryPath(),
		"version":         b.opts.Version,
		"platform":        runtime.GOOS,
		"arch":            runtime.GOARCH,
	}

	// Check if downloaded binary exists
	if _, err := os.Stat(info["downloaded_path"]); err == nil {
		info["downloaded_exists"] = "true"
	} else {
		info["downloaded_exists"] = "false"
	}

	// Check if service binary exists
	if _, err := os.Stat(info["service_path"]); err == nil {
		info["service_exists"] = "true"
	} else {
		info["service_exists"] = "false"
	}

	return info
}

// logServiceBinaryPath logs which binary path will be used for the service
func (b *LocalBesu) logServiceBinaryPath() {
	binaryPath := b.GetServiceBinaryPath()
	b.logger.Info("Service will use Besu binary",
		"path", binaryPath,
		"nodeID", b.opts.ID,
		"version", b.opts.Version)
}

// GetBinaryPath returns the path to the Besu binary
func (b *LocalBesu) GetBinaryPath() string {
	binDir := filepath.Join(b.configService.GetDataPath(), "bin/besu", b.opts.Version)
	return filepath.Join(binDir, "bin", "besu")
}

// GetServiceBinaryPath returns the path to the Besu binary that should be used in service configuration
// This method determines the correct binary path after installation, considering different locations
func (b *LocalBesu) GetServiceBinaryPath() string {
	// First, try the downloaded binary path
	downloadedPath := b.GetBinaryPath()
	if _, err := os.Stat(downloadedPath); err == nil {
		// Check if it's executable
		if err := exec.Command(downloadedPath, "--version").Run(); err == nil {
			b.logger.Info("Using downloaded Besu binary for service", "path", downloadedPath)
			return downloadedPath
		}
	}

	// Check if besu is in PATH
	if path, err := exec.LookPath("besu"); err == nil {
		if err := exec.Command(path, "--version").Run(); err == nil {
			b.logger.Info("Using Besu binary from PATH for service", "path", path)
			return path
		}
	}

	// Fallback to downloaded path (will be handled by error checking)
	b.logger.Warn("No suitable Besu binary found, using downloaded path", "path", downloadedPath)
	return downloadedPath
}

// verifyBinary verifies that the Besu binary exists and is executable
func (b *LocalBesu) verifyBinary() error {
	besuBinary := b.GetServiceBinaryPath()

	// Check if binary exists
	if _, err := os.Stat(besuBinary); os.IsNotExist(err) {
		return fmt.Errorf("besu binary %s not found", besuBinary)
	}

	// Check if binary is executable
	cmd := exec.Command(besuBinary, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("besu binary %s is not executable or failed to run: %w\nOutput: %s", besuBinary, err, string(output))
	}

	b.logger.Info("Besu binary verified", "path", besuBinary)
	return nil
}

// checkPrerequisites checks if required software is installed
func (b *LocalBesu) checkPrerequisites() error {
	switch b.mode {
	case "service":
		// Only require "java" binary to be available in PATH
		cmd := exec.Command("java", "-version")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("java is not installed or not in PATH: %w\nOutput: %s", err, string(output))
		}

	case "docker":
		// Check Docker installation using Docker API client
		cli, err := client.NewClientWithOpts(
			client.FromEnv,
			client.WithAPIVersionNegotiation(),
		)
		if err != nil {
			return fmt.Errorf("failed to create Docker client: %w", err)
		}
		defer cli.Close()

		// Ping Docker daemon to verify connectivity
		ctx := context.Background()
		if _, err := cli.Ping(ctx); err != nil {
			return fmt.Errorf("docker daemon is not running or not accessible: %w", err)
		}
	}

	return nil
}

// buildCommand builds the command to start Besu
func (b *LocalBesu) buildCommand(dataDir string, genesisPath string, configDir string) string {
	// Use the service binary path which determines the correct binary after installation
	besuBinary := b.GetServiceBinaryPath()

	keyPath := filepath.Join(configDir, "key")

	cmd := []string{
		besuBinary,
		fmt.Sprintf("--data-path=%s", dataDir),
		fmt.Sprintf("--genesis-file=%s", genesisPath),
		"--rpc-http-enabled",
		"--rpc-http-api=ETH,NET,QBFT",
		"--rpc-http-cors-origins=all",
		"--rpc-http-host=0.0.0.0",
		fmt.Sprintf("--rpc-http-port=%s", b.opts.RPCPort),
		"--min-gas-price=1000000000",
		fmt.Sprintf("--network-id=%d", b.opts.ChainID),
		"--host-allowlist=*",
		fmt.Sprintf("--node-private-key-file=%s", keyPath),
		fmt.Sprintf("--metrics-enabled=%t", b.opts.MetricsEnabled),
		"--metrics-host=0.0.0.0",
		fmt.Sprintf("--metrics-port=%d", b.opts.MetricsPort),
		fmt.Sprintf("--metrics-protocol=%s", b.opts.MetricsProtocol),

		"--p2p-enabled=true",
		fmt.Sprintf("--p2p-host=%s", b.opts.P2PHost),
		fmt.Sprintf("--p2p-port=%s", b.opts.P2PPort),
		"--nat-method=NONE",
		"--discovery-enabled=true",
		"--profile=ENTERPRISE",
	}

	// Add bootnodes if specified
	if len(b.opts.BootNodes) > 0 {
		cmd = append(cmd, fmt.Sprintf("--bootnodes=%s", strings.Join(b.opts.BootNodes, ",")))
	}

	return strings.Join(cmd, " ")
}

// buildEnvironment builds the environment variables for Besu
func (b *LocalBesu) buildEnvironment() map[string]string {
	env := make(map[string]string)

	// Add custom environment variables from opts
	for k, v := range b.opts.Env {
		env[k] = v
	}

	// Add required environment variables
	env["JAVA_OPTS"] = "-Xmx4g"

	// Add JAVA_HOME if it exists
	if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
		env["JAVA_HOME"] = javaHome

		// Add Java binary directory to PATH
		currentPath := os.Getenv("PATH")
		javaBinPath := filepath.Join(javaHome, "bin")
		env["PATH"] = javaBinPath + string(os.PathListSeparator) + currentPath
	}

	return env
}

// buildDockerEnvironment builds the environment variables for Besu in Docker
func (b *LocalBesu) buildDockerEnvironment() map[string]string {
	env := make(map[string]string)

	// Add custom environment variables from opts
	for k, v := range b.opts.Env {
		env[k] = v
	}

	// Add Java options
	env["JAVA_OPTS"] = "-Xmx4g"

	return env
}

// Stop stops the Besu node
func (b *LocalBesu) Stop() error {
	b.logger.Info("Stopping Besu node", "opts", b.opts)

	switch b.mode {
	case "service":
		platform := runtime.GOOS
		switch platform {
		case "linux":
			return b.stopSystemdService()
		case "darwin":
			return b.stopLaunchdService()
		default:
			return fmt.Errorf("unsupported platform for service mode: %s", platform)
		}
	case "docker":
		return b.stopDocker()
	default:
		return fmt.Errorf("invalid mode: %s", b.mode)
	}
}

func (b *LocalBesu) installBesu() error {
	besuBinary := b.GetBinaryPath()

	// Check if binary already exists
	if _, err := os.Stat(besuBinary); err == nil {
		b.logger.Info("Besu binary already exists", "path", besuBinary)
		return nil
	}

	b.logger.Info("Installing Besu binary", "version", b.opts.Version, "platform", runtime.GOOS)

	err := b.downloadBesu(b.opts.Version)
	if err != nil {
		return err
	}

	b.logger.Info("Besu downloaded and installed", "path", besuBinary)
	return nil
}

func (b *LocalBesu) downloadBesu(version string) error {
	b.logger.Info("Downloading Besu", "version", version)
	// Construct download URL from GitHub releases
	downloadURL := fmt.Sprintf("https://github.com/hyperledger/besu/releases/download/%s/besu-%s.zip",
		version, version)

	// Create temporary directory for download
	tmpDir, err := os.MkdirTemp("", "besu-download-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download archive
	archivePath := filepath.Join(tmpDir, "besu.zip")
	if err := downloadFile(downloadURL, archivePath); err != nil {
		return fmt.Errorf("failed to download Besu: %w", err)
	}

	// Extract archive
	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("failed to create extraction directory: %w", err)
	}

	if err := extractZip(archivePath, extractDir); err != nil {
		return fmt.Errorf("failed to extract Besu archive: %w", err)
	}

	// Source directory with all Besu files
	besuDir := filepath.Join(extractDir, fmt.Sprintf("besu-%s", version))

	// Create the target directory structure
	binDir := filepath.Join(b.configService.GetDataPath(), "bin/besu", version)
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Copy entire directory structure
	if err := copyDir(besuDir, binDir); err != nil {
		return fmt.Errorf("failed to copy Besu directory: %w", err)
	}

	// Ensure executables have correct permissions
	executablePaths := []string{
		filepath.Join(binDir, "bin", "besu"),
		filepath.Join(binDir, "bin", "besu-entry.sh"),
		filepath.Join(binDir, "bin", "besu-untuned"),
		filepath.Join(binDir, "bin", "evmtool"),
	}

	for _, execPath := range executablePaths {
		if _, err := os.Stat(execPath); err == nil {
			if err := os.Chmod(execPath, 0755); err != nil {
				return fmt.Errorf("failed to set executable permissions for %s: %w", execPath, err)
			}
		}
	}

	b.logger.Info("Successfully downloaded and installed Besu", "version", version, "path", binDir)
	return nil
}

// downloadFile downloads a file from the given URL to the specified path
func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// extractZip extracts a zip file to the specified directory
func extractZip(zipPath, extractDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		// Construct the full path for the extracted file
		path := filepath.Join(extractDir, file.Name)

		// Check for ZipSlip vulnerability
		if !strings.HasPrefix(path, filepath.Clean(extractDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			// Create directory
			if err := os.MkdirAll(path, file.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", path, err)
			}
			continue
		}

		// Create parent directories if they don't exist
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %w", path, err)
		}

		// Open the file in the zip
		fileReader, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open file in zip %s: %w", file.Name, err)
		}

		// Create the file
		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			fileReader.Close()
			return fmt.Errorf("failed to create file %s: %w", path, err)
		}

		// Copy the file contents
		_, err = io.Copy(outFile, fileReader)
		fileReader.Close()
		outFile.Close()
		if err != nil {
			return fmt.Errorf("failed to copy file contents for %s: %w", path, err)
		}
	}

	return nil
}

// copyDir recursively copies a directory structure
func copyDir(src string, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy directory %s: %w", srcPath, err)
			}
		} else {
			// Copy file
			input, err := os.ReadFile(srcPath)
			if err != nil {
				return fmt.Errorf("failed to read source file %s: %w", srcPath, err)
			}

			// Preserve original file mode
			srcInfo, err := os.Stat(srcPath)
			if err != nil {
				return fmt.Errorf("failed to get source file info %s: %w", srcPath, err)
			}

			if err := os.WriteFile(dstPath, input, srcInfo.Mode()); err != nil {
				return fmt.Errorf("failed to write destination file %s: %w", dstPath, err)
			}
		}
	}

	return nil
}

// TailLogs tails the logs of the besu service
func (b *LocalBesu) TailLogs(ctx context.Context, tail int, follow bool) (<-chan string, error) {
	logChan := make(chan string, 100)

	if b.mode == "docker" {
		slugifiedID := strings.ReplaceAll(strings.ToLower(b.opts.ID), " ", "-")
		containerName := fmt.Sprintf("besu-%s", slugifiedID) // Adjust if you have a helper for container name
		go func() {
			defer close(logChan)
			cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err != nil {
				b.logger.Error("Failed to create docker client", "error", err)
				return
			}
			defer cli.Close()

			options := container.LogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Follow:     follow,
				Details:    true,
				Tail:       fmt.Sprintf("%d", tail),
			}
			reader, err := cli.ContainerLogs(ctx, containerName, options)
			if err != nil {
				b.logger.Error("Failed to get docker logs", "error", err)
				return
			}
			defer reader.Close()

			header := make([]byte, 8)
			for {
				_, err := io.ReadFull(reader, header)
				if err != nil {
					if err != io.EOF {
						b.logger.Error("Failed to read docker log header", "error", err)
					}
					return
				}
				length := int(uint32(header[4])<<24 | uint32(header[5])<<16 | uint32(header[6])<<8 | uint32(header[7]))
				if length == 0 {
					continue
				}
				payload := make([]byte, length)
				_, err = io.ReadFull(reader, payload)
				if err != nil {
					if err != io.EOF {
						b.logger.Error("Failed to read docker log payload", "error", err)
					}
					return
				}
				select {
				case <-ctx.Done():
					return
				case logChan <- string(payload):
				}
			}
		}()
		return logChan, nil
	}

	// Get log file path based on ID
	slugifiedID := strings.ReplaceAll(strings.ToLower(b.opts.ID), " ", "-")
	logPath := filepath.Join(b.configService.GetDataPath(), "besu", slugifiedID, b.getServiceName()+".log")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		close(logChan)
		return logChan, fmt.Errorf("log file does not exist: %s", logPath)
	}

	go func() {
		defer close(logChan)

		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			if follow {
				cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
					"Get-Content", "-Encoding", "UTF8", "-Path", logPath, "-Tail", fmt.Sprintf("%d", tail), "-Wait")
			} else {
				cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
					"Get-Content", "-Encoding", "UTF8", "-Path", logPath, "-Tail", fmt.Sprintf("%d", tail))
			}
		} else {
			env := os.Environ()
			env = append(env, "LC_ALL=en_US.UTF-8")
			if follow {
				cmd = exec.Command("tail", "-n", fmt.Sprintf("%d", tail), "-f", logPath)
			} else {
				cmd = exec.Command("tail", "-n", fmt.Sprintf("%d", tail), logPath)
			}
			cmd.Env = env
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			b.logger.Error("Failed to create stdout pipe", "error", err)
			return
		}

		if err := cmd.Start(); err != nil {
			b.logger.Error("Failed to start tail command", "error", err)
			return
		}

		scanner := bufio.NewScanner(transform.NewReader(stdout, unicode.UTF8.NewDecoder()))
		scanner.Split(bufio.ScanLines)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				cmd.Process.Kill()
				return
			case logChan <- scanner.Text() + "\n":
			}
		}

		if err := cmd.Wait(); err != nil {
			if ctx.Err() == nil {
				b.logger.Error("Tail command failed", "error", err)
			}
		}
	}()

	return logChan, nil
}
