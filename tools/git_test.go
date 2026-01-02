package tools

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	// Create first file and commit
	file1 := filepath.Join(dir, "file1.txt")
	if err := os.WriteFile(file1, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if _, err := worktree.Add("file1.txt"); err != nil {
		t.Fatalf("failed to add file1: %v", err)
	}
	_, err = worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Create second file and commit
	file2 := filepath.Join(dir, "file2.txt")
	if err := os.WriteFile(file2, []byte("second file"), 0644); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}
	if _, err := worktree.Add("file2.txt"); err != nil {
		t.Fatalf("failed to add file2: %v", err)
	}
	_, err = worktree.Commit("add file2", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	return dir
}

func TestIterateCommits_ValidRepo(t *testing.T) {
	repoPath := setupTestRepo(t)

	var changes []FileChange
	err := IterateCommits(repoPath, func(fc FileChange) error {
		changes = append(changes, fc)
		return nil
	})
	if err != nil {
		t.Fatalf("IterateCommits failed: %v", err)
	}

	if len(changes) != 2 {
		t.Errorf("expected 2 file changes, got %d", len(changes))
	}

	// Verify first change (most recent commit first)
	found := false
	for _, fc := range changes {
		if fc.FilePath == "file2.txt" {
			found = true
			if fc.AuthorName != "Test User" {
				t.Errorf("expected author 'Test User', got %q", fc.AuthorName)
			}
			if fc.AuthorEmail != "test@example.com" {
				t.Errorf("expected email 'test@example.com', got %q", fc.AuthorEmail)
			}
			if len(fc.CommitHash) != 40 {
				t.Errorf("expected 40-char commit hash, got %d chars", len(fc.CommitHash))
			}
			if fc.Patch == "" {
				t.Error("expected non-empty patch")
			}
		}
	}
	if !found {
		t.Error("expected to find file2.txt in changes")
	}
}

func TestIterateCommits_EmptyRepo(t *testing.T) {
	dir := t.TempDir()

	_, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	err = IterateCommits(dir, func(fc FileChange) error {
		t.Error("callback should not be called for empty repo")
		return nil
	})

	if err == nil {
		t.Error("expected error for empty repo (no HEAD)")
	}
}

func TestIterateCommits_InvalidPath(t *testing.T) {
	err := IterateCommits("/nonexistent/path", func(fc FileChange) error {
		t.Error("callback should not be called for invalid path")
		return nil
	})

	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestIterateCommits_CallbackError(t *testing.T) {
	repoPath := setupTestRepo(t)

	expectedErr := errors.New("callback error")
	callCount := 0

	err := IterateCommits(repoPath, func(fc FileChange) error {
		callCount++
		return expectedErr
	})

	if err == nil {
		t.Error("expected error from callback to propagate")
	}

	if callCount != 1 {
		t.Errorf("expected callback to be called once before error, got %d", callCount)
	}
}

func TestIterateCommits_DeletedFile(t *testing.T) {
	dir := t.TempDir()

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	// Create and commit a file
	filePath := filepath.Join(dir, "to_delete.txt")
	if err := os.WriteFile(filePath, []byte("will be deleted"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := worktree.Add("to_delete.txt"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	_, err = worktree.Commit("add file", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Delete and commit
	if err := os.Remove(filePath); err != nil {
		t.Fatalf("failed to remove file: %v", err)
	}
	if _, err := worktree.Remove("to_delete.txt"); err != nil {
		t.Fatalf("failed to stage removal: %v", err)
	}
	_, err = worktree.Commit("delete file", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit deletion: %v", err)
	}

	var deletionFound bool
	err = IterateCommits(dir, func(fc FileChange) error {
		if fc.FilePath == "to_delete.txt" && fc.CommitMessage == "delete file" {
			deletionFound = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("IterateCommits failed: %v", err)
	}

	if !deletionFound {
		t.Error("expected to find deletion change with correct FilePath")
	}
}

func TestFileChange_Additions(t *testing.T) {
	tests := []struct {
		name     string
		patch    string
		expected string
	}{
		{
			name: "extracts only added lines",
			patch: `diff --git a/file.txt b/file.txt
--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,4 @@
 context line
+added line
 another context
-removed line`,
			expected: "added line",
		},
		{
			name: "multiple additions",
			patch: `diff --git a/file.txt b/file.txt
+++ b/file.txt
@@ -1,2 +1,4 @@
+first addition
 context
+second addition`,
			expected: "first addition\nsecond addition",
		},
		{
			name: "ignores +++ header",
			patch: `diff --git a/file.txt b/file.txt
+++ b/file.txt
+real addition`,
			expected: "real addition",
		},
		{
			name: "empty patch returns empty string",
			patch: `diff --git a/file.txt b/file.txt
--- a/file.txt
+++ b/file.txt
@@ -1 +1 @@
-old line`,
			expected: "",
		},
		{
			name: "preserves content after + prefix",
			patch: `+password=secret123
+api_key="AKIAIOSFODNN7EXAMPLE"`,
			expected: "password=secret123\napi_key=\"AKIAIOSFODNN7EXAMPLE\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := FileChange{Patch: tt.patch}
			got := fc.Additions()
			if got != tt.expected {
				t.Errorf("Additions() = %q, want %q", got, tt.expected)
			}
		})
	}
}
