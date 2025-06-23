package dirs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type DirsService struct {
	Root string
	// Add DB or project service reference here if needed for project validation
}

func NewDirsService(root string) *DirsService {
	return &DirsService{Root: root}
}

// Placeholder for project validation
func (s *DirsService) validateProject(project string) error {
	if project == "" {
		return errors.New("project is required")
	}
	// TODO: Implement real project existence check
	return nil
}

// sanitizeAndValidatePath ensures the path is safe and within the project scope
func (s *DirsService) sanitizeAndValidatePath(project, dir string) (string, error) {
	if dir == "" {
		dir = "."
	}

	// Clean the directory path to remove any path traversal attempts
	cleanDir := filepath.Clean(dir)

	// Prevent path traversal attacks
	if strings.Contains(cleanDir, "..") || strings.HasPrefix(cleanDir, "/") || strings.HasPrefix(cleanDir, "\\") {
		return "", errors.New("invalid directory path: contains path traversal or absolute path")
	}

	// Join with project path and get absolute path
	joinedPath := filepath.Join(project, cleanDir)
	absPath, err := filepath.Abs(joinedPath)
	if err != nil {
		return "", errors.New("invalid directory path")
	}

	// Ensure the resolved path is within the project directory
	absProject, err := filepath.Abs(project)
	if err != nil {
		return "", errors.New("invalid project path")
	}

	// Check if the path is within the project scope
	if !strings.HasPrefix(absPath, absProject) {
		return "", errors.New("directory path is outside the project scope")
	}

	return absPath, nil
}

func (s *DirsService) ListDirs(project, dir string) ([]string, error) {
	if err := s.validateProject(project); err != nil {
		return nil, err
	}

	// Sanitize and validate the path
	base, err := s.sanitizeAndValidatePath(project, dir)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs, nil
}

func (s *DirsService) CreateDir(project, dir string) error {
	if err := s.validateProject(project); err != nil {
		return err
	}

	if dir == "" {
		return errors.New("dir is required")
	}

	// Sanitize and validate the path
	absPath, err := s.sanitizeAndValidatePath(project, dir)
	if err != nil {
		return err
	}

	return os.MkdirAll(absPath, 0755)
}

func (s *DirsService) DeleteDir(project, dir string) error {
	if err := s.validateProject(project); err != nil {
		return err
	}

	if dir == "" {
		return errors.New("dir is required")
	}

	// Sanitize and validate the path
	absPath, err := s.sanitizeAndValidatePath(project, dir)
	if err != nil {
		return err
	}

	return os.RemoveAll(absPath)
}

// ListEntries returns files, directories, and skipped directories in a given directory
func (s *DirsService) ListEntries(project, dir string) (files, directories, skipped []string, err error) {
	if err := s.validateProject(project); err != nil {
		return nil, nil, nil, err
	}

	// Sanitize and validate the path
	base, err := s.sanitizeAndValidatePath(project, dir)
	if err != nil {
		return nil, nil, nil, err
	}

	entries, err := os.ReadDir(base)
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

			dirEntries, err := os.ReadDir(absSubDir)
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
