package files

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type FilesService struct {
}

func NewFilesService() *FilesService {
	return &FilesService{}
}

// Placeholder for project validation
func (s *FilesService) validateProject(project string) error {
	if project == "" {
		return errors.New("project is required")
	}
	// TODO: Implement real project existence check
	return nil
}

// sanitizeAndValidatePath ensures the path is safe and within the project scope
func (s *FilesService) sanitizeAndValidatePath(project, path string) (string, error) {
	if path == "" {
		path = "."
	}

	// Clean the path to remove any path traversal attempts
	cleanPath := filepath.Clean(path)

	// Prevent path traversal attacks
	if strings.Contains(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") || strings.HasPrefix(cleanPath, "\\") {
		return "", errors.New("invalid file path: contains path traversal or absolute path")
	}

	// Join with project path and get absolute path
	joinedPath := filepath.Join(project, cleanPath)
	absPath, err := filepath.Abs(joinedPath)
	if err != nil {
		return "", errors.New("invalid file path")
	}

	// Ensure the resolved path is within the project directory
	absProject, err := filepath.Abs(project)
	if err != nil {
		return "", errors.New("invalid project path")
	}

	// Check if the path is within the project scope
	if !strings.HasPrefix(absPath, absProject) {
		return "", errors.New("file path is outside the project scope")
	}

	return absPath, nil
}

func (s *FilesService) ListFiles(project, dir string) ([]string, error) {
	if err := s.validateProject(project); err != nil {
		return nil, err
	}

	// Sanitize and validate the path
	base, err := s.sanitizeAndValidatePath(project, dir)
	if err != nil {
		return nil, err
	}

	entries, err := ioutil.ReadDir(base)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

func (s *FilesService) ReadFile(project, path string) ([]byte, error) {
	if err := s.validateProject(project); err != nil {
		return nil, err
	}

	if path == "" {
		return nil, errors.New("path is required")
	}

	// Sanitize and validate the path
	absPath, err := s.sanitizeAndValidatePath(project, path)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadFile(absPath)
}

func (s *FilesService) WriteFile(project, path string, data []byte) error {
	if err := s.validateProject(project); err != nil {
		return err
	}

	if path == "" {
		return errors.New("path is required")
	}

	// Sanitize and validate the path
	absPath, err := s.sanitizeAndValidatePath(project, path)
	if err != nil {
		return err
	}

	// Ensure the directory exists before writing
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return ioutil.WriteFile(absPath, data, 0644)
}

func (s *FilesService) DeleteFile(project, path string) error {
	if err := s.validateProject(project); err != nil {
		return err
	}

	if path == "" {
		return errors.New("path is required")
	}

	// Sanitize and validate the path
	absPath, err := s.sanitizeAndValidatePath(project, path)
	if err != nil {
		return err
	}

	return os.Remove(absPath)
}

// ListEntries returns files, directories, and skipped directories in a given directory
func (s *FilesService) ListEntries(project, dir string) (files, directories, skipped []string, err error) {
	if err := s.validateProject(project); err != nil {
		return nil, nil, nil, err
	}

	// Sanitize and validate the path
	base, err := s.sanitizeAndValidatePath(project, dir)
	if err != nil {
		return nil, nil, nil, err
	}

	entries, err := ioutil.ReadDir(base)
	if err != nil {
		return nil, nil, nil, err
	}

	var filesOut, dirsOut, skippedOut []string
	const maxEntries = 1000
	skipList := map[string]struct{}{"node_modules": {}}

	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			if _, skip := skipList[name]; skip {
				skippedOut = append(skippedOut, name)
				continue
			}

			// Validate subdirectory path before reading
			subDirPath := filepath.Join(base, name)
			// Double-check the subdirectory is still within project scope
			absSubDir, err := filepath.Abs(subDirPath)
			if err != nil {
				skippedOut = append(skippedOut, name)
				continue
			}

			absProject, err := filepath.Abs(project)
			if err != nil || !strings.HasPrefix(absSubDir, absProject) {
				skippedOut = append(skippedOut, name)
				continue
			}

			dirEntries, err := ioutil.ReadDir(absSubDir)
			if err == nil && len(dirEntries) > maxEntries {
				skippedOut = append(skippedOut, name)
				continue
			}
			dirsOut = append(dirsOut, name)
		} else {
			filesOut = append(filesOut, entry.Name())
		}
	}
	return filesOut, dirsOut, skippedOut, nil
}
