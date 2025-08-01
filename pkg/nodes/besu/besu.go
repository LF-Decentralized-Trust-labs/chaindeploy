package besu

import (
	"bufio"
	"context"
	"fmt"
	"io"
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

// checkPrerequisites checks if required software is installed
func (b *LocalBesu) checkPrerequisites() error {
	switch b.mode {
	case "service":
		// Check Java installation
		javaHome := os.Getenv("JAVA_HOME")
		if javaHome == "" {
			return fmt.Errorf("JAVA_HOME environment variable is not set")
		}

		// Verify JAVA_HOME directory exists
		if _, err := os.Stat(javaHome); os.IsNotExist(err) {
			return fmt.Errorf("JAVA_HOME directory does not exist: %s", javaHome)
		}

		// Check Java version
		javaCmd := filepath.Join(javaHome, "bin", "java")
		cmd := exec.Command(javaCmd, "-version")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to check Java version: %w\nOutput: %s", err, string(output))
		}

		// Check if java binary exists in PATH as fallback
		if err := exec.Command("java", "-version").Run(); err != nil {
			return fmt.Errorf("Java is not installed or not in PATH: %w", err)
		}

		// Check Besu installation
		if err := exec.Command("besu", "--version").Run(); err != nil {
			return fmt.Errorf("Besu is not installed: %w", err)
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
			return fmt.Errorf("Docker daemon is not running or not accessible: %w", err)
		}
	}

	return nil
}

// buildCommand builds the command to start Besu
func (b *LocalBesu) buildCommand(dataDir string, genesisPath string, configDir string) string {
	var besuBinary string
	if runtime.GOOS == "darwin" {
		if runtime.GOARCH == "arm64" {
			besuBinary = "/opt/homebrew/opt/besu/bin/besu"
		} else {
			besuBinary = "/usr/local/opt/besu/bin/besu"
		}
	} else {
		besuBinary = filepath.Join(b.configService.GetDataPath(), "bin/besu", b.opts.Version, "besu")
	}

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
	if runtime.GOOS == "darwin" {
		return b.installBesuMacOS()
	}
	return b.downloadBesu(b.opts.Version) // existing Linux download method
}

func (b *LocalBesu) downloadBesu(binDir string) error {
	// Construct download URL from GitHub releases
	downloadURL := fmt.Sprintf("https://github.com/hyperledger/besu/releases/download/%s/besu-%s.zip",
		b.opts.Version, b.opts.Version)

	// Create temporary directory for download
	tmpDir, err := os.MkdirTemp("", "besu-download-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download archive
	archivePath := filepath.Join(tmpDir, "besu.zip")
	cmd := exec.Command("curl", "-L", "-o", archivePath, downloadURL)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download Besu: %w", err)
	}

	// Extract archive
	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("failed to create extraction directory: %w", err)
	}

	unzipCmd := exec.Command("unzip", archivePath, "-d", extractDir)
	if err := unzipCmd.Run(); err != nil {
		return fmt.Errorf("failed to extract Besu archive: %w", err)
	}

	// Source directory with all Besu files
	besuDir := filepath.Join(extractDir, fmt.Sprintf("besu-%s", b.opts.Version))

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
		if err := os.Chmod(execPath, 0755); err != nil {
			return fmt.Errorf("failed to set executable permissions for %s: %w", execPath, err)
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

func (b *LocalBesu) installBesuMacOS() error {
	// Check if brew is installed
	if _, err := exec.LookPath("brew"); err != nil {
		return fmt.Errorf("homebrew is not installed: %w", err)
	}

	// Add hyperledger/besu tap if not already added
	tapCmd := exec.Command("brew", "tap", "hyperledger/besu")
	if err := tapCmd.Run(); err != nil {
		return fmt.Errorf("failed to tap hyperledger/besu: %w", err)
	}

	// Check if besu is already installed
	checkCmd := exec.Command("brew", "list", "hyperledger/besu/besu")
	if checkCmd.Run() == nil {
		// Besu is installed, check version
		versionCmd := exec.Command("besu", "--version")
		output, err := versionCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get installed Besu version: %w", err)
		}

		// Parse installed version
		installedVersion := strings.TrimSpace(string(output))
		if strings.Contains(installedVersion, b.opts.Version) {
			// Correct version is already installed
			return nil
		}

		// Uninstall current version if it's different
		uninstallCmd := exec.Command("brew", "uninstall", "hyperledger/besu/besu")
		if err := uninstallCmd.Run(); err != nil {
			return fmt.Errorf("failed to uninstall existing Besu version: %w", err)
		}
	}

	// Install specific version
	installCmd := exec.Command("brew", "install", "hyperledger/besu/besu")
	if output, err := installCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install Besu %s: %w\nOutput: %s", b.opts.Version, err, string(output))
	}

	// Create symlink to our bin directory
	binDir := filepath.Join(b.configService.GetDataPath(), "bin/besu", b.opts.Version)
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	brewPrefix := "/usr/local/opt/besu/bin/besu"
	if runtime.GOARCH == "arm64" {
		brewPrefix = "/opt/homebrew/opt/besu/bin/besu"
	}

	targetBinary := filepath.Join(binDir, "besu")
	if err := os.Symlink(brewPrefix, targetBinary); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create symlink: %w", err)
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
