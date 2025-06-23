package binaries

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/chainlaunch/chainlaunch/pkg/config"
)

const (
	DefaultVersion = "3.0.0"
	// Base URL for Hyperledger Fabric binary releases
	githubReleaseURL = "https://github.com/hyperledger/fabric/releases/download"
)

// BinaryType represents the type of binary (peer or orderer)
type BinaryType string

const (
	PeerBinary    BinaryType = "peer"
	OrdererBinary BinaryType = "orderer"
)

// BinaryDownloader handles downloading and managing Fabric binaries
type BinaryDownloader struct {
	configService *config.ConfigService
}

// NewBinaryDownloader creates a new BinaryDownloader instance
func NewBinaryDownloader(configService *config.ConfigService) (*BinaryDownloader, error) {
	binDir := filepath.Join(configService.GetDataPath(), "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create binary directory: %w", err)
	}
	return &BinaryDownloader{configService: configService}, nil
}

// GetBinaryPath returns the path to the binary, downloading it if necessary
func (d *BinaryDownloader) GetBinaryPath(binaryType BinaryType, version string) (string, error) {
	if version == "" {
		version = DefaultVersion
	}

	binDir := filepath.Join(d.configService.GetDataPath(), "bin")
	binaryName := string(binaryType)
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	versionDir := filepath.Join(binDir, version)
	binaryPath := filepath.Join(versionDir, "bin", binaryName)

	// Check if binary already exists
	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath, nil
	}

	// Create version directory
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create version directory: %w", err)
	}

	// Download and extract binaries
	if err := d.downloadAndExtractBinaries(version, versionDir); err != nil {
		return "", fmt.Errorf("failed to download and extract binaries: %w", err)
	}

	// Verify binary exists after extraction
	if _, err := os.Stat(binaryPath); err != nil {
		return "", fmt.Errorf("binary not found after extraction: %w", err)
	}

	return binaryPath, nil
}

// downloadAndExtractBinaries downloads and extracts the Fabric binaries
func (d *BinaryDownloader) downloadAndExtractBinaries(version, destDir string) error {
	// Validate destination directory
	absDestDir, err := filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for destination directory: %w", err)
	}

	// Construct download URL
	arch := runtime.GOARCH
	runtimeOs := runtime.GOOS
	filename := fmt.Sprintf("hyperledger-fabric-%s-%s-%s.tar.gz", runtimeOs, arch, version)
	url := fmt.Sprintf("%s/v%s/%s", githubReleaseURL, version, filename)

	// Create temporary file for download
	tmpFile, err := os.CreateTemp("", "fabric-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Download file
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download archive: HTTP %d", resp.StatusCode)
	}

	// Copy download to temp file
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("failed to save archive: %w", err)
	}

	// Rewind temp file for reading
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to rewind temp file: %w", err)
	}

	// Open gzip reader
	gzr, err := gzip.NewReader(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	// Create tar reader
	tr := tar.NewReader(gzr)

	// Extract files
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		// Skip if not a file
		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Sanitize the archive path to prevent zip slip
		targetPath, err := d.sanitizeArchivePath(header.Name, absDestDir)
		if err != nil {
			return fmt.Errorf("failed to sanitize archive path %s: %w", header.Name, err)
		}

		// Create directory structure
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory structure: %w", err)
		}

		// Create file
		f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		// Copy contents
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return fmt.Errorf("failed to write file: %w", err)
		}
		f.Close()

		// Make binary executable if in bin directory
		if strings.HasPrefix(header.Name, "bin/") {
			if err := os.Chmod(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to make binary executable: %w", err)
			}
		}
	}

	return nil
}

// sanitizeArchivePath ensures the archive entry path is safe and doesn't contain path traversal
func (d *BinaryDownloader) sanitizeArchivePath(archivePath, targetDir string) (string, error) {
	// Clean the path to remove any ".." or "." components
	cleanPath := filepath.Clean(archivePath)

	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("archive path contains path traversal attempt: %s", archivePath)
	}

	// Ensure the path doesn't start with a slash (absolute path)
	if filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("archive path is absolute, which is not allowed: %s", archivePath)
	}

	// Join with target directory and validate the result
	fullPath := filepath.Join(targetDir, cleanPath)

	// Ensure the resulting path is within the target directory
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute target directory: %w", err)
	}

	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute full path: %w", err)
	}

	// Check if the full path is within the target directory
	if !strings.HasPrefix(absFullPath, absTargetDir) {
		return "", fmt.Errorf("archive path would escape target directory: %s", archivePath)
	}

	return fullPath, nil
}
