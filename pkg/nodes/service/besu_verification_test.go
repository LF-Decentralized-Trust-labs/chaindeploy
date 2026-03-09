package service

import (
	"os"
	"path/filepath"
	"testing"
)

// --- BesuVersionVerificationRequest/Response struct tests ---

func TestBesuVersionVerificationRequest_Fields(t *testing.T) {
	req := BesuVersionVerificationRequest{Version: "24.3.0"}
	if req.Version != "24.3.0" {
		t.Errorf("expected Version '24.3.0', got %q", req.Version)
	}
}

func TestBesuVersionVerificationResponse_Fields(t *testing.T) {
	resp := BesuVersionVerificationResponse{
		Success:       true,
		Version:       "24.3.0",
		BinaryPath:    "/path/to/besu",
		ActualVersion: "24.3.0",
		Downloaded:    false,
	}
	if !resp.Success {
		t.Error("expected Success true")
	}
	if resp.Version != "24.3.0" {
		t.Errorf("expected Version '24.3.0', got %q", resp.Version)
	}
	if resp.Downloaded {
		t.Error("expected Downloaded false")
	}
}

func TestBesuVersionVerificationResponse_ErrorField(t *testing.T) {
	resp := BesuVersionVerificationResponse{
		Success: false,
		Version: "99.0.0",
		Error:   "binary not found",
	}
	if resp.Success {
		t.Error("expected Success false")
	}
	if resp.Error != "binary not found" {
		t.Errorf("expected Error 'binary not found', got %q", resp.Error)
	}
}

// --- copyDir tests (uses temp dirs) ---

func TestCopyDir_BasicCopy(t *testing.T) {
	// Create source directory with files
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create a file in source
	testContent := []byte("hello world")
	if err := os.WriteFile(filepath.Join(srcDir, "test.txt"), testContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory with a file
	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a NodeService with nil fields (copyDir doesn't use them)
	s := &NodeService{}

	if err := s.copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify files exist in destination
	content, err := os.ReadFile(filepath.Join(dstDir, "test.txt"))
	if err != nil {
		t.Fatalf("expected test.txt in dest, got error: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(content))
	}

	nestedContent, err := os.ReadFile(filepath.Join(dstDir, "subdir", "nested.txt"))
	if err != nil {
		t.Fatalf("expected nested.txt in dest, got error: %v", err)
	}
	if string(nestedContent) != "nested" {
		t.Errorf("expected 'nested', got %q", string(nestedContent))
	}
}

func TestCopyDir_EmptySource(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	s := &NodeService{}
	if err := s.copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir should handle empty source, got: %v", err)
	}
}

func TestCopyDir_NonExistentSource(t *testing.T) {
	dstDir := t.TempDir()

	s := &NodeService{}
	err := s.copyDir("/nonexistent/path", dstDir)
	if err == nil {
		t.Fatal("copyDir should fail with nonexistent source")
	}
}

// --- extractZip tests ---

func TestExtractZip_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a non-zip file
	badFile := filepath.Join(tmpDir, "bad.zip")
	if err := os.WriteFile(badFile, []byte("not a zip"), 0644); err != nil {
		t.Fatal(err)
	}

	s := &NodeService{}
	err := s.extractZip(badFile, tmpDir)
	if err == nil {
		t.Fatal("extractZip should fail with invalid zip file")
	}
}

func TestExtractZip_NonExistentFile(t *testing.T) {
	s := &NodeService{}
	err := s.extractZip("/nonexistent/file.zip", "/tmp")
	if err == nil {
		t.Fatal("extractZip should fail with nonexistent file")
	}
}

// --- downloadFile tests ---

func TestDownloadFile_InvalidURL(t *testing.T) {
	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "test.bin")

	s := &NodeService{}
	err := s.downloadFile("http://invalid-host-that-does-not-exist.local/file.zip", dest)
	if err == nil {
		t.Fatal("downloadFile should fail with invalid URL")
	}
}
