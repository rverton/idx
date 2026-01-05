package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// CloneRepository clones a git repository to a temporary directory with authentication.
// It returns the path to the cloned repository, a cleanup function, and any error encountered.
// The cleanup function removes the temporary directory and is safe to call multiple times.
func CloneRepository(ctx context.Context, url, username, password string) (repoPath string, cleanup func(), err error) {
	// Create temporary directory for the clone
	tempDir, err := os.MkdirTemp("", "idx-git-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create cleanup function with idempotent behavior
	var cleanupOnce sync.Once
	cleanup = func() {
		cleanupOnce.Do(func() {
			os.RemoveAll(tempDir)
		})
	}

	// Configure clone options
	cloneOpts := &git.CloneOptions{
		URL: url,
	}

	// Add authentication if credentials are provided
	if username != "" || password != "" {
		cloneOpts.Auth = &http.BasicAuth{
			Username: username,
			Password: password,
		}
	}

	// Clone the repository
	_, err = git.PlainCloneContext(ctx, tempDir, false, cloneOpts)
	if err != nil {
		// Clean up temp directory on failure
		cleanup()
		return "", nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	return tempDir, cleanup, nil
}

// FileChange represents a single file change within a commit.
type FileChange struct {
	CommitHash    string
	CommitMessage string
	AuthorName    string
	AuthorEmail   string
	FilePath      string
	Patch         string
}

// Additions returns only the added lines from the patch (lines starting with '+'),
// excluding the file header ('+++' lines). This filters out context lines and deletions.
func (fc FileChange) Additions() string {
	var additions []string
	for _, line := range strings.Split(fc.Patch, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			additions = append(additions, strings.TrimPrefix(line, "+"))
		}
	}
	return strings.Join(additions, "\n")
}

// IterateCommits opens a repository at the given path and iterates over all commits
// across all branches, calling the callback function for each file change found.
// This is similar to `git log -p --all`.
func IterateCommits(repoPath string, callback func(FileChange) error) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	branches, err := repo.Branches()
	if err != nil {
		return fmt.Errorf("failed to get branches: %w", err)
	}

	seenCommits := make(map[string]bool)

	err = branches.ForEach(func(ref *plumbing.Reference) error {
		commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
		if err != nil {
			return fmt.Errorf("failed to get commit log for branch %s: %w", ref.Name(), err)
		}

		return commitIter.ForEach(func(commit *object.Commit) error {
			if seenCommits[commit.Hash.String()] {
				return nil
			}
			seenCommits[commit.Hash.String()] = true

			var parentTree *object.Tree
			if commit.NumParents() > 0 {
				parent, err := commit.Parent(0)
				if err != nil {
					return fmt.Errorf("failed to get parent commit: %w", err)
				}
				parentTree, err = parent.Tree()
				if err != nil {
					return fmt.Errorf("failed to get parent tree: %w", err)
				}
			}

			currentTree, err := commit.Tree()
			if err != nil {
				return fmt.Errorf("failed to get commit tree: %w", err)
			}

			changes, err := parentTree.Diff(currentTree)
			if err != nil {
				return fmt.Errorf("failed to get diff: %w", err)
			}

			for _, change := range changes {
				patch, err := change.Patch()
				if err != nil {
					continue
				}

				fc := FileChange{
					CommitHash:    commit.Hash.String(),
					CommitMessage: commit.Message,
					AuthorName:    commit.Author.Name,
					AuthorEmail:   commit.Author.Email,
					FilePath:      change.To.Name,
					Patch:         patch.String(),
				}

				if fc.FilePath == "" {
					fc.FilePath = change.From.Name
				}

				if err := callback(fc); err != nil {
					return err
				}
			}

			return nil
		})
	})

	return err
}
