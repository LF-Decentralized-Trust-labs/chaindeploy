package versionmanagement

import (
	"context"
	"io"
	"time"

	"bytes"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// VersionEntry represents a single version (commit) in the project history.
type VersionEntry struct {
	ID        string
	Author    string
	Timestamp string
	Message   string
}

// VersionManager defines the interface for versioning operations.
type VersionManager interface {
	CommitChange(ctx context.Context, dir string, message string) error
	ListHistory(ctx context.Context, dir string) ([]VersionEntry, error)
	CheckoutVersion(ctx context.Context, dir string, versionID string) error
	GetCurrentVersion(ctx context.Context, dir string) (VersionEntry, error)
	DiffVersions(ctx context.Context, dir string, fromID, toID string) (string, error)
}

// DefaultManager is the default implementation using Git as backend.
type DefaultManager struct{}

func NewDefaultManager() *DefaultManager {
	return &DefaultManager{}
}

// InitRepo initializes a git repository in the given directory if it does not exist.
func InitRepo(dir string) (*git.Repository, error) {
	return git.PlainInit(dir, false)
}

type CommitWithFileChanges struct {
	Hash      string
	Author    string
	Timestamp string
	Message   string
	Added     []string
	Removed   []string
	Modified  []string
	Parent    *string
}

// ListCommitsWithFileChanges returns a list of commits for the given directory, with file changes for each commit.
func ListCommitsWithFileChanges(ctx context.Context, repoDir string, maxCommits int) ([]CommitWithFileChanges, error) {
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		return nil, err
	}
	ref, err := repo.Head()
	if err != nil {
		return nil, err
	}
	cIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, err
	}
	defer cIter.Close()

	var commits []CommitWithFileChanges
	count := 0
	err = cIter.ForEach(func(c *object.Commit) error {
		if maxCommits > 0 && count >= maxCommits {
			return io.EOF
		}
		var added, removed, modified []string
		var parentHash *string
		if c.NumParents() > 0 {
			p := c.ParentHashes[0].String()
			parentHash = &p
			parent, err := c.Parent(0)
			if err == nil {
				patch, err := parent.Patch(c)
				if err == nil {
					parentTree, err := parent.Tree()
					if err != nil {
						return err
					}
					currTree, err := c.Tree()
					if err != nil {
						return err
					}
					for _, stat := range patch.Stats() {
						// Check file existence in parent and current tree
						_, errParent := parentTree.File(stat.Name)
						_, errCurr := currTree.File(stat.Name)
						if errParent != nil && errCurr == nil {
							// Not in parent, in current: Added
							added = append(added, stat.Name)
						} else if errParent == nil && errCurr != nil {
							// In parent, not in current: Removed
							removed = append(removed, stat.Name)
						} else if errParent == nil && errCurr == nil {
							// In both: Modified
							modified = append(modified, stat.Name)
						}
					}
				}
			}
		}
		commits = append(commits, CommitWithFileChanges{
			Hash:      c.Hash.String(),
			Author:    c.Author.Name,
			Timestamp: c.Author.When.Format(time.RFC3339),
			Message:   c.Message,
			Added:     added,
			Removed:   removed,
			Modified:  modified,
			Parent:    parentHash,
		})
		count++
		return nil
	})
	if err != nil && err != io.EOF {
		return nil, err
	}
	return commits, nil
}

// GetFileDiffBetweenCommits returns the diff of a file between two commits in the given repo directory.
func GetFileDiffBetweenCommits(ctx context.Context, repoDir, filePath, fromHash, toHash string) (string, error) {
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		return "", err
	}
	fromCommit, err := repo.CommitObject(plumbing.NewHash(fromHash))
	if err != nil {
		return "", err
	}
	toCommit, err := repo.CommitObject(plumbing.NewHash(toHash))
	if err != nil {
		return "", err
	}
	fromTree, err := fromCommit.Tree()
	if err != nil {
		return "", err
	}
	toTree, err := toCommit.Tree()
	if err != nil {
		return "", err
	}
	fromEntry, _ := fromTree.File(filePath)
	toEntry, _ := toTree.File(filePath)
	var fromContent, toContent string
	if fromEntry != nil {
		fromContent, _ = fromEntry.Contents()
	}
	if toEntry != nil {
		toContent, _ = toEntry.Contents()
	}
	// Use a simple diff (unified format)
	udiff := UnifiedDiff(fromContent, toContent, filePath, fromHash, toHash)
	return udiff, nil
}

// UnifiedDiff returns a unified diff string for two file contents.
func UnifiedDiff(a, b, file, fromHash, toHash string) string {
	// This is a simple implementation; for more advanced, use a diff library
	if a == b {
		return "No changes."
	}
	var buf bytes.Buffer
	buf.WriteString("--- " + file + " (" + fromHash + ")\n")
	buf.WriteString("+++ " + file + " (" + toHash + ")\n")
	buf.WriteString("@@ ... @@\n")
	buf.WriteString(b)
	return buf.String()
}

// GetFileAtCommit returns the contents of a file at a specific commit hash in the given repo directory.
func GetFileAtCommit(ctx context.Context, repoDir, filePath, commitHash string) (string, error) {
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		return "", err
	}
	commit, err := repo.CommitObject(plumbing.NewHash(commitHash))
	if err != nil {
		return "", err
	}
	tree, err := commit.Tree()
	if err != nil {
		return "", err
	}
	file, err := tree.File(filePath)
	if err != nil {
		return "", err
	}
	content, err := file.Contents()
	if err != nil {
		return "", err
	}
	return content, nil
}
