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

func (s *DirsService) ListDirs(project, dir string) ([]string, error) {
	if err := s.validateProject(project); err != nil {
		return nil, err
	}
	if dir == "" {
		dir = "."
	}
	// Scope to project root
	base := filepath.Join(project, dir)
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
	// Normalize and validate the directory name
	cleanDir := filepath.Clean(dir)
	if strings.Contains(cleanDir, "..") || strings.Contains(cleanDir, "/") || strings.Contains(cleanDir, "\\") {
		return errors.New("invalid directory name")
	}
	// Ensure the resolved path is within the project directory
	base := filepath.Join(project, cleanDir)
	absBase, err := filepath.Abs(base)
	if err != nil || !strings.HasPrefix(absBase, filepath.Clean(project)) {
		return errors.New("directory path is outside the project scope")
	}
	return os.MkdirAll(absBase, 0755)
}

func (s *DirsService) DeleteDir(project, dir string) error {
	if err := s.validateProject(project); err != nil {
		return err
	}
	if dir == "" {
		return errors.New("dir is required")
	}
	base := filepath.Join(project, dir)
	return os.RemoveAll(base)
}

// ListEntries returns files, directories, and skipped directories in a given directory
func (s *DirsService) ListEntries(project, dir string) (files, directories, skipped []string, err error) {
	if err := s.validateProject(project); err != nil {
		return nil, nil, nil, err
	}
	if dir == "" {
		dir = "."
	}
	base := filepath.Join(project, dir)
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
			dirPath := filepath.Join(base, name)
			dirEntries, err := os.ReadDir(dirPath)
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
