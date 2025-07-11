package versionmanagement

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// CommitChange commits all staged changes with the provided message using go-git.
func (m *DefaultManager) CommitChange(ctx context.Context, dir string, message string) error {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return fmt.Errorf("failed to open git repo: %w", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Add all changes
	if err := w.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return fmt.Errorf("failed to add changes: %w", err)
	}

	// Get author info from git config or use default
	cfg, err := repo.ConfigScoped(config.SystemScope)
	name := "AI Bot"
	email := "ai@localhost"
	if err == nil && cfg.User.Name != "" && cfg.User.Email != "" {
		name = cfg.User.Name
		email = cfg.User.Email
	}

	commit, err := w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  name,
			Email: email,
			When:  time.Now(),
		},
	})
	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			return nil
		}
		return fmt.Errorf("failed to commit: %w", err)
	}

	_ = commit
	return nil
}

// ListHistory returns the commit history.
func (m *DefaultManager) ListHistory(ctx context.Context, dir string) ([]VersionEntry, error) {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repo: %w", err)
	}
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}
	cIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, fmt.Errorf("failed to get log: %w", err)
	}
	defer cIter.Close()

	var history []VersionEntry
	err = cIter.ForEach(func(c *object.Commit) error {
		history = append(history, VersionEntry{
			ID:        c.Hash.String(),
			Author:    c.Author.Name,
			Timestamp: c.Author.When.Format(time.RFC3339),
			Message:   c.Message,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}
	return history, nil
}

// CheckoutVersion checks out the specified commit or branch.
func (m *DefaultManager) CheckoutVersion(ctx context.Context, dir string, versionID string) error {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return fmt.Errorf("failed to open git repo: %w", err)
	}
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	// Try as branch first
	branchRef := plumbing.NewBranchReferenceName(versionID)
	err = w.Checkout(&git.CheckoutOptions{
		Branch: branchRef,
		Force:  true,
	})
	if err == nil {
		return nil
	}
	// Try as commit hash
	commitHash := plumbing.NewHash(versionID)
	err = w.Checkout(&git.CheckoutOptions{
		Hash:  commitHash,
		Force: true,
	})
	if err != nil {
		return fmt.Errorf("failed to checkout version %s: %w", versionID, err)
	}
	return nil
}

// GetCurrentVersion returns the current commit info.
func (m *DefaultManager) GetCurrentVersion(ctx context.Context, dir string) (VersionEntry, error) {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return VersionEntry{}, fmt.Errorf("failed to open git repo: %w", err)
	}
	ref, err := repo.Head()
	if err != nil {
		return VersionEntry{}, fmt.Errorf("failed to get HEAD: %w", err)
	}
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return VersionEntry{}, fmt.Errorf("failed to get commit: %w", err)
	}
	return VersionEntry{
		ID:        commit.Hash.String(),
		Author:    commit.Author.Name,
		Timestamp: commit.Author.When.Format(time.RFC3339),
		Message:   commit.Message,
	}, nil
}

// DiffVersions returns the diff between two versions.
func (m *DefaultManager) DiffVersions(ctx context.Context, dir string, fromID, toID string) (string, error) {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return "", fmt.Errorf("failed to open git repo: %w", err)
	}
	fromHash := plumbing.NewHash(fromID)
	toHash := plumbing.NewHash(toID)
	fromCommit, err := repo.CommitObject(fromHash)
	if err != nil {
		return "", fmt.Errorf("failed to get from commit: %w", err)
	}
	toCommit, err := repo.CommitObject(toHash)
	if err != nil {
		return "", fmt.Errorf("failed to get to commit: %w", err)
	}
	fromTree, err := fromCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("failed to get from tree: %w", err)
	}
	toTree, err := toCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("failed to get to tree: %w", err)
	}
	diffs, err := fromTree.Diff(toTree)
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}
	var sb strings.Builder
	for _, patch := range diffs {
		patchObj, err := patch.Patch()
		if err != nil {
			return "", fmt.Errorf("failed to get patch: %w", err)
		}
		io.WriteString(&sb, patchObj.String())
	}
	return sb.String(), nil
}
