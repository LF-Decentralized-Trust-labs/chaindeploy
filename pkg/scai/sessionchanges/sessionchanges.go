package sessionchanges

import (
	"sync"
)

// Tracker tracks file changes for a specific session
type Tracker struct {
	mu            sync.Mutex
	modifiedFiles map[string]struct{}
}

// NewTracker creates a new session tracker
func NewTracker() *Tracker {
	return &Tracker{
		modifiedFiles: make(map[string]struct{}),
	}
}

// RegisterChange registers a file as changed (created/modified/deleted) during the session.
func (t *Tracker) RegisterChange(filePath string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.modifiedFiles[filePath] = struct{}{}
}

// GetAndResetChanges returns the list of changed files and resets the tracker.
func (t *Tracker) GetAndResetChanges() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	files := make([]string, 0, len(t.modifiedFiles))
	for f := range t.modifiedFiles {
		files = append(files, f)
	}
	t.modifiedFiles = make(map[string]struct{})
	return files
}

// For backward compatibility, maintain the global tracker
var (
	globalMu            sync.Mutex
	globalModifiedFiles = make(map[string]struct{})
)

// RegisterChange registers a file as changed in the global tracker
func RegisterChange(filePath string) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalModifiedFiles[filePath] = struct{}{}
}

// GetAndResetChanges returns the list of changed files from the global tracker and resets it
func GetAndResetChanges() []string {
	globalMu.Lock()
	defer globalMu.Unlock()
	files := make([]string, 0, len(globalModifiedFiles))
	for f := range globalModifiedFiles {
		files = append(files, f)
	}
	globalModifiedFiles = make(map[string]struct{})
	return files
}
