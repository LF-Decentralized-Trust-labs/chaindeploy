package service

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// validBesuVersion matches semantic versions like "24.1.0", "25.3.1-RC1"
var validBesuVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)?$`)

// BesuVersionVerificationRequest represents the request to verify a Besu version
type BesuVersionVerificationRequest struct {
	Version string `json:"version"`
}

// BesuVersionVerificationResponse represents the response of Besu version verification
type BesuVersionVerificationResponse struct {
	Success       bool   `json:"success"`
	Version       string `json:"version"`
	BinaryPath    string `json:"binaryPath,omitempty"`
	ActualVersion string `json:"actualVersion,omitempty"`
	Downloaded    bool   `json:"downloaded"`
	Error         string `json:"error,omitempty"`
}

// VerifyBesuVersion verifies that a Besu binary can be executed for the given version,
// downloading it if necessary
func (s *NodeService) VerifyBesuVersion(ctx context.Context, req BesuVersionVerificationRequest) (*BesuVersionVerificationResponse, error) {
	version := req.Version
	if version == "" {
		version = "24.1.0" // Default Besu version
	}

	if !validBesuVersion.MatchString(version) {
		return &BesuVersionVerificationResponse{
			Version: version,
			Error:   "invalid version format: must be semver (e.g. 24.1.0)",
		}, nil
	}

	s.logger.Info("Starting Besu version verification", "version", version)

	response := &BesuVersionVerificationResponse{
		Version:    version,
		Downloaded: false,
	}

	// Calculate expected binary path
	binDir := filepath.Join(s.configService.GetDataPath(), "bin/besu", version)
	binaryPath := filepath.Join(binDir, "bin", "besu")
	response.BinaryPath = binaryPath

	// Check if binary already exists
	binaryExists := false
	if _, err := os.Stat(binaryPath); err == nil {
		binaryExists = true
		s.logger.Info("Besu binary already exists", "path", binaryPath)
	}

	// If binary doesn't exist, download it
	if !binaryExists {
		s.logger.Info("Besu binary not found, downloading", "version", version)
		if err := s.downloadBesuBinary(version, binDir); err != nil {
			response.Error = fmt.Sprintf("Failed to download Besu binary: %v", err)
			return response, nil
		}
		response.Downloaded = true
	}

	// Verify the binary works by running --version
	actualVersion, err := s.getBesuActualVersion(binaryPath)
	if err != nil {
		response.Error = fmt.Sprintf("Failed to execute or verify Besu binary: %v", err)
		return response, nil
	}

	response.Success = true
	response.ActualVersion = actualVersion

	s.logger.Info("Besu version verification completed successfully",
		"requested_version", version,
		"actual_version", actualVersion,
		"binary_path", response.BinaryPath)

	return response, nil
}

// downloadBesuBinary downloads and installs a Besu binary for the specified version
func (s *NodeService) downloadBesuBinary(version, targetDir string) error {
	s.logger.Info("Downloading Besu binary", "version", version, "targetDir", targetDir)
	return s.downloadAndExtractBesu(version, targetDir)
}

// downloadAndExtractBesu downloads and extracts Besu binary
func (s *NodeService) downloadAndExtractBesu(version, binDir string) error {
	// Create directories
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Download URL from GitHub releases
	downloadURL := fmt.Sprintf("https://github.com/hyperledger/besu/releases/download/%s/besu-%s.zip", version, version)

	// Create temporary directory for download
	tmpDir, err := os.MkdirTemp("", "besu-download-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download archive
	archivePath := filepath.Join(tmpDir, "besu.zip")
	if err := s.downloadFile(downloadURL, archivePath); err != nil {
		return fmt.Errorf("failed to download Besu: %w", err)
	}

	// Extract archive
	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("failed to create extraction directory: %w", err)
	}

	if err := s.extractZip(archivePath, extractDir); err != nil {
		return fmt.Errorf("failed to extract Besu archive: %w", err)
	}

	// Source directory with all Besu files
	besuDir := filepath.Join(extractDir, fmt.Sprintf("besu-%s", version))

	// Copy entire directory structure
	if err := s.copyDir(besuDir, binDir); err != nil {
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

	s.logger.Info("Successfully downloaded and installed Besu", "version", version, "path", binDir)
	return nil
}

// downloadFile downloads a file from the given URL to the specified path
func (s *NodeService) downloadFile(url, destPath string) error {
	resp, err := http.Get(url) // #nosec G107 -- URL is constructed from known version string
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// extractZip extracts a zip file to a destination directory
func (s *NodeService) extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	// Extract files and folders
	for _, f := range r.File {
		// Check for directory traversal
		if strings.Contains(f.Name, "..") {
			continue
		}

		fpath := filepath.Join(dest, f.Name) // #nosec G305 -- checked for traversal above

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, f.FileInfo().Mode())
			continue
		}

		// Create directories for file
		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}

		// Extract file
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.FileInfo().Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

// copyDir recursively copies a directory tree
func (s *NodeService) copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from src
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Construct destination path
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			return err
		}

		// Copy permissions
		return os.Chmod(dstPath, info.Mode())
	})
}

// getBesuActualVersion runs besu --version and parses the actual version string
func (s *NodeService) getBesuActualVersion(binaryPath string) (string, error) {
	cmd := exec.Command(binaryPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run besu --version: %w", err)
	}

	// Parse version from output like "besu/v24.1.0/linux-x86_64/openjdk-java-21"
	outputStr := strings.TrimSpace(string(output))
	lines := strings.Split(outputStr, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "besu/") {
			parts := strings.Split(line, "/")
			if len(parts) >= 2 {
				version := strings.TrimPrefix(parts[1], "v")
				return version, nil
			}
		}
	}

	// Fallback: just return the first line as version info
	if len(lines) > 0 {
		return lines[0], nil
	}

	return outputStr, nil
}
