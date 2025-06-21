package projects

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadProjectAsZip(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-project")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a mock project structure
	projectDir := filepath.Join(tempDir, "test-project")
	err = os.MkdirAll(projectDir, 0755)
	require.NoError(t, err)

	// Create some test files
	testFiles := map[string]string{
		"main.go":               "package main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}",
		"go.mod":                "module test-project\n\ngo 1.21\n",
		"README.md":             "# Test Project\n\nThis is a test project.",
		"config.json":           `{"name": "test", "version": "1.0.0"}`,
		".env":                  "API_KEY=test123",
		"node_modules/test":     "should be ignored",
		".vscode/settings.json": `{"editor.formatOnSave": true}`,
		"tmp/temp.txt":          "temporary file",
		"build/output":          "build artifact",
	}

	// Create the test files and directories
	for filePath, content := range testFiles {
		fullPath := filepath.Join(projectDir, filePath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)

		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Test the zip creation logic directly
	var buf bytes.Buffer
	err = createProjectZip(context.Background(), projectDir, &buf)
	require.NoError(t, err)
	assert.NotEmpty(t, buf.Bytes())

	// Verify the zip file contains the expected files
	zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)

	// Check that included files are present
	expectedFiles := []string{
		"main.go",
		"go.mod",
		"README.md",
		"config.json",
		".env",
	}

	// Check that ignored files are not present
	ignoredFiles := []string{
		"node_modules/test",
		".vscode/settings.json",
		"tmp/temp.txt",
		"build/output",
	}

	// Create a map of files in the zip
	zipFiles := make(map[string]bool)
	for _, file := range zipReader.File {
		zipFiles[file.Name] = true
	}

	// Verify expected files are included
	for _, expectedFile := range expectedFiles {
		assert.True(t, zipFiles[expectedFile], "Expected file %s to be in zip", expectedFile)
	}

	// Verify ignored files are not included
	for _, ignoredFile := range ignoredFiles {
		assert.False(t, zipFiles[ignoredFile], "Expected file %s to be ignored", ignoredFile)
	}

	// Verify we can read the content of a file from the zip
	for _, file := range zipReader.File {
		if file.Name == "main.go" {
			rc, err := file.Open()
			require.NoError(t, err)
			defer rc.Close()

			content, err := io.ReadAll(rc)
			require.NoError(t, err)
			assert.Contains(t, string(content), "package main")
			break
		}
	}
}

// createProjectZip is a helper function that extracts the zip creation logic for testing
func createProjectZip(ctx context.Context, projectDir string, writer io.Writer) error {
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
	err := filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
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
			return err
		}

		// Open and read the file
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// Copy file content to zip
		_, err = io.Copy(zipFile, file)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Close the zip writer to finalize the zip file
	if err := zipWriter.Close(); err != nil {
		return err
	}

	return nil
}
