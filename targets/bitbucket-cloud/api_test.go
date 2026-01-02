package bitbucketcloud

import (
	"idx/detect"
	"os"
	"path/filepath"
	"strings"
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

	// Create file and commit
	file1 := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(file1, []byte("password=secret123"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := worktree.Add("secret.txt"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	_, err = worktree.Commit("add secrets", &git.CommitOptions{
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

func noopMemoryStore() detect.MemoryStore {
	return detect.MemoryStore{
		Has: func(key string) bool { return false },
		Set: func(key string) {},
	}
}

func TestAnalyseRepo_CallsAnalyze(t *testing.T) {
	repoPath := setupTestRepo(t)
	repoName := "workspace/test-repo"

	var contents []detect.Content
	analyze := func(content detect.Content) {
		contents = append(contents, content)
	}

	analyseRepo(repoPath, repoName, "test-target", analyze, noopMemoryStore())

	if len(contents) == 0 {
		t.Error("expected analyze to be called at least once")
	}
}

func TestAnalyseRepo_ContentFormat(t *testing.T) {
	repoPath := setupTestRepo(t)
	repoName := "workspace/test-repo"

	var contents []detect.Content
	analyze := func(content detect.Content) {
		contents = append(contents, content)
	}

	analyseRepo(repoPath, repoName, "test-target", analyze, noopMemoryStore())

	if len(contents) == 0 {
		t.Fatal("expected at least one content item")
	}

	content := contents[0]

	// Verify Key format: repo:shortHash:filePath
	if !strings.HasPrefix(content.Key, repoName+":") {
		t.Errorf("expected Key to start with %q, got %q", repoName+":", content.Key)
	}
	parts := strings.Split(content.Key, ":")
	if len(parts) != 3 {
		t.Errorf("expected Key to have 3 parts separated by ':', got %d parts in %q", len(parts), content.Key)
	}
	if len(parts) >= 2 && len(parts[1]) != 8 {
		t.Errorf("expected short hash (8 chars) in Key, got %d chars", len(parts[1]))
	}

	// Verify Data contains added content (not full patch)
	if len(content.Data) == 0 {
		t.Error("expected Data to contain added content")
	}
	if !strings.Contains(string(content.Data), "password=secret123") {
		t.Errorf("expected Data to contain added content 'password=secret123', got %q", string(content.Data))
	}

	// Verify Location format
	if len(content.Location) != 4 {
		t.Errorf("expected Location to have 4 elements, got %d", len(content.Location))
	}
	if content.Location[0] != "bitbucket-cloud" {
		t.Errorf("expected Location[0] to be 'bitbucket-cloud', got %q", content.Location[0])
	}
	if content.Location[1] != repoName {
		t.Errorf("expected Location[1] to be %q, got %q", repoName, content.Location[1])
	}
	if len(content.Location[2]) != 40 {
		t.Errorf("expected Location[2] to be full commit hash (40 chars), got %d chars", len(content.Location[2]))
	}
	if content.Location[3] != "secret.txt" {
		t.Errorf("expected Location[3] to be 'secret.txt', got %q", content.Location[3])
	}
}

func TestAnalyseRepo_InvalidPath(t *testing.T) {
	var contents []detect.Content
	analyze := func(content detect.Content) {
		contents = append(contents, content)
	}

	// Should not panic, just log error
	analyseRepo("/nonexistent/path", "repo", "test-target", analyze, noopMemoryStore())

	if len(contents) != 0 {
		t.Error("expected no content for invalid path")
	}
}

func TestAnalyseRepo_MemoryStoreSkipsAnalyzed(t *testing.T) {
	repoPath := setupTestRepo(t)
	repoName := "workspace/test-repo"

	seenKeys := make(map[string]bool)
	memory := detect.MemoryStore{
		Has: func(key string) bool {
			return seenKeys[key]
		},
		Set: func(key string) {
			seenKeys[key] = true
		},
	}

	var firstRunContents []detect.Content
	analyze := func(content detect.Content) {
		firstRunContents = append(firstRunContents, content)
	}

	analyseRepo(repoPath, repoName, "test-target", analyze, memory)

	if len(firstRunContents) == 0 {
		t.Fatal("expected at least one content item on first run")
	}

	var secondRunContents []detect.Content
	analyze2 := func(content detect.Content) {
		secondRunContents = append(secondRunContents, content)
	}

	analyseRepo(repoPath, repoName, "test-target", analyze2, memory)

	if len(secondRunContents) != 0 {
		t.Errorf("expected no content on second run (should be skipped by memory), got %d items", len(secondRunContents))
	}
}
