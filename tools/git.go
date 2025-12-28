package tools

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/go-git/go-git/v5"
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
