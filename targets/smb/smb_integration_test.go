//go:build integration

package smb

import (
	"context"
	"idx/detect"
	"strings"
	"testing"
	"time"
)

const (
	testHostname = "localhost"
	testPort     = 445
	testUser     = "testuser"
	testPassword = "testpass123"
	testShare    = "testshare"
)

func TestIntegration_SMBConnection(t *testing.T) {
	client, err := NewClient(testHostname, testPort, testUser, testPassword, "", 0)
	if err != nil {
		t.Fatalf("failed to connect to SMB server: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	if err := client.VerifyConnection(ctx); err != nil {
		t.Fatalf("failed to verify connection: %v", err)
	}
}

func TestIntegration_EnumerateShares(t *testing.T) {
	client, err := NewClient(testHostname, testPort, testUser, testPassword, "", 0)
	if err != nil {
		t.Fatalf("failed to connect to SMB server: %v", err)
	}
	defer client.Close()

	shares, err := client.EnumerateShares()
	if err != nil {
		t.Fatalf("failed to enumerate shares: %v", err)
	}

	found := false
	for _, share := range shares {
		if share == testShare {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected to find share %q, got shares: %v", testShare, shares)
	}
}

func TestIntegration_ListFiles(t *testing.T) {
	client, err := NewClient(testHostname, testPort, testUser, testPassword, "", 0)
	if err != nil {
		t.Fatalf("failed to connect to SMB server: %v", err)
	}
	defer client.Close()

	err = client.session.TreeConnect(testShare)
	if err != nil {
		t.Fatalf("failed to connect to share: %v", err)
	}
	defer client.session.TreeDisconnect(testShare)

	files, err := client.listFiles(testShare, "", 0, 10)
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	expectedFiles := map[string]bool{
		"foo.py":      false,
		"secrets.zip": false,
	}

	for _, file := range files {
		if _, ok := expectedFiles[file.Name]; ok {
			expectedFiles[file.Name] = true
		}
	}

	for name, found := range expectedFiles {
		if !found {
			t.Errorf("expected to find file %q in share", name)
		}
	}
}

func TestIntegration_ReadFile(t *testing.T) {
	client, err := NewClient(testHostname, testPort, testUser, testPassword, "", 0)
	if err != nil {
		t.Fatalf("failed to connect to SMB server: %v", err)
	}
	defer client.Close()

	err = client.session.TreeConnect(testShare)
	if err != nil {
		t.Fatalf("failed to connect to share: %v", err)
	}
	defer client.session.TreeDisconnect(testShare)

	data, err := client.readFile(testShare, "foo.py")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "postgresql") {
		t.Errorf("expected foo.py to contain 'postgresql', got: %s", content)
	}
}

func TestIntegration_Explore(t *testing.T) {
	ctx := context.Background()

	var analyzedContent []detect.Content
	var analyzedFilenames []string

	analyze := func(content detect.Content) {
		analyzedContent = append(analyzedContent, content)
	}

	analyzeFilename := func(filename, contentKey string, location []string) {
		analyzedFilenames = append(analyzedFilenames, filename)
	}

	memoryData := make(map[string]int64)
	memory := detect.MemoryStore{
		Has: func(key string) bool {
			_, exists := memoryData[key]
			return exists
		},
		Set: func(key string) {
			memoryData[key] = time.Now().Unix()
		},
		GetTimestamp: func(key string) (int64, bool) {
			ts, exists := memoryData[key]
			return ts, exists
		},
	}

	err := Explore(
		ctx,
		analyze,
		analyzeFilename,
		memory,
		"integration-test",
		testHostname,
		testPort,
		testUser,
		testPassword,
		"",
		10,
		0,             // folderCacheDepth=0 disables folder caching
		24*time.Hour,
		0,
	)
	if err != nil {
		t.Fatalf("Explore failed: %v", err)
	}

	foundFooPyContent := false
	for _, content := range analyzedContent {
		if strings.Contains(content.Key, "foo.py") {
			foundFooPyContent = true
			if !strings.Contains(string(content.Data), "postgresql") {
				t.Errorf("foo.py content should contain 'postgresql'")
			}
			break
		}
	}
	if !foundFooPyContent {
		t.Error("expected foo.py to be analyzed for content")
	}

	foundFooPy := false
	foundSecretsZip := false
	foundIdRsa := false // in secrets.zip

	for _, filename := range analyzedFilenames {
		switch filename {
		case "foo.py":
			foundFooPy = true
		case "secrets.zip":
			foundSecretsZip = true
		case "id_rsa":
			foundIdRsa = true
		}
	}

	if !foundFooPy {
		t.Error("expected foo.py filename to be analyzed")
	}
	if !foundSecretsZip {
		t.Error("expected secrets.zip filename to be analyzed")
	}
	if !foundIdRsa {
		t.Error("expected id_rsa (from secrets.zip) filename to be analyzed")
	}
}

func TestIntegration_ListFilesDeepNested(t *testing.T) {
	client, err := NewClient(testHostname, testPort, testUser, testPassword, "", 0)
	if err != nil {
		t.Fatalf("failed to connect to SMB server: %v", err)
	}
	defer client.Close()

	err = client.session.TreeConnect(testShare)
	if err != nil {
		t.Fatalf("failed to connect to share: %v", err)
	}
	defer client.session.TreeDisconnect(testShare)

	files, err := client.listFiles(testShare, "", 0, 10)
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	expectedFiles := map[string]bool{
		"shallow.txt": false,
		"deep.txt":    false,
		"deeper.txt":  false,
		".env":        false,
	}

	for _, file := range files {
		if _, ok := expectedFiles[file.Name]; ok {
			expectedFiles[file.Name] = true
		}
	}

	for name, found := range expectedFiles {
		if !found {
			t.Errorf("expected to find file %q in nested folders", name)
		}
	}
}

func TestIntegration_FolderCaching_SkipsDeepFolders(t *testing.T) {
	ctx := context.Background()

	var analyzedFilenames []string
	var memorySetKeys []string

	analyze := func(content detect.Content) {}

	analyzeFilename := func(filename, contentKey string, location []string) {
		analyzedFilenames = append(analyzedFilenames, filename)
	}

	now := time.Now()
	recentTimestamp := now.Add(-1 * time.Hour).Unix()

	memoryData := make(map[string]int64)
	// Pre-populate with a cached folder at depth 2
	memoryData["smb/cache-test/localhost/testshare/folder:level1/level2"] = recentTimestamp

	memory := detect.MemoryStore{
		Has: func(key string) bool {
			_, exists := memoryData[key]
			return exists
		},
		Set: func(key string) {
			memoryData[key] = now.Unix()
			memorySetKeys = append(memorySetKeys, key)
		},
		GetTimestamp: func(key string) (int64, bool) {
			ts, exists := memoryData[key]
			return ts, exists
		},
	}

	err := Explore(
		ctx,
		analyze,
		analyzeFilename,
		memory,
		"cache-test",
		testHostname,
		testPort,
		testUser,
		testPassword,
		"",
		10,
		2,             // folderCacheDepth=2
		24*time.Hour,  // rescan duration
		0,
	)
	if err != nil {
		t.Fatalf("Explore failed: %v", err)
	}

	// Files at depth < 2 should be analyzed (foo.py at depth 1, shallow.txt at depth 2)
	foundFooPy := false
	foundShallowTxt := false
	foundDeepTxt := false
	foundDeeperTxt := false

	for _, filename := range analyzedFilenames {
		switch filename {
		case "foo.py":
			foundFooPy = true
		case "shallow.txt":
			foundShallowTxt = true
		case "deep.txt":
			foundDeepTxt = true
		case "deeper.txt":
			foundDeeperTxt = true
		}
	}

	if !foundFooPy {
		t.Error("expected foo.py (depth 1) to be analyzed")
	}
	if !foundShallowTxt {
		t.Error("expected shallow.txt (depth 2) to be analyzed")
	}

	// Files at depth >= 2 under the cached folder should be SKIPPED
	// level1/level2/level3/deep.txt is at depth 4, under level1/level2 which is cached
	if foundDeepTxt {
		t.Error("deep.txt should be skipped because level1/level2 folder is cached")
	}
	if foundDeeperTxt {
		t.Error("deeper.txt should be skipped because level1/level2 folder is cached")
	}
}

func TestIntegration_FolderCaching_RescansExpiredFolders(t *testing.T) {
	ctx := context.Background()

	var analyzedFilenames []string

	analyze := func(content detect.Content) {}

	analyzeFilename := func(filename, contentKey string, location []string) {
		analyzedFilenames = append(analyzedFilenames, filename)
	}

	now := time.Now()
	// Old timestamp - folder cache expired
	expiredTimestamp := now.Add(-48 * time.Hour).Unix()

	memoryData := make(map[string]int64)
	// Pre-populate with an EXPIRED cached folder at depth 2
	memoryData["smb/rescan-test/localhost/testshare/folder:level1/level2"] = expiredTimestamp

	memory := detect.MemoryStore{
		Has: func(key string) bool {
			_, exists := memoryData[key]
			return exists
		},
		Set: func(key string) {
			memoryData[key] = now.Unix()
		},
		GetTimestamp: func(key string) (int64, bool) {
			ts, exists := memoryData[key]
			return ts, exists
		},
	}

	err := Explore(
		ctx,
		analyze,
		analyzeFilename,
		memory,
		"rescan-test",
		testHostname,
		testPort,
		testUser,
		testPassword,
		"",
		10,
		2,             // folderCacheDepth=2
		24*time.Hour,  // rescan duration (48h ago > 24h, so expired)
		0,
	)
	if err != nil {
		t.Fatalf("Explore failed: %v", err)
	}

	// Since the folder cache is expired, all files should be analyzed
	foundDeepTxt := false
	foundDeeperTxt := false

	for _, filename := range analyzedFilenames {
		switch filename {
		case "deep.txt":
			foundDeepTxt = true
		case "deeper.txt":
			foundDeeperTxt = true
		}
	}

	if !foundDeepTxt {
		t.Error("expected deep.txt to be analyzed after folder cache expired")
	}
	if !foundDeeperTxt {
		t.Error("expected deeper.txt to be analyzed after folder cache expired")
	}
}

func TestIntegration_FolderCaching_MemoryKeyFormats(t *testing.T) {
	ctx := context.Background()

	var memorySetKeys []string

	analyze := func(content detect.Content) {}
	analyzeFilename := func(filename, contentKey string, location []string) {}

	now := time.Now()
	memoryData := make(map[string]int64)

	memory := detect.MemoryStore{
		Has: func(key string) bool {
			_, exists := memoryData[key]
			return exists
		},
		Set: func(key string) {
			memoryData[key] = now.Unix()
			memorySetKeys = append(memorySetKeys, key)
		},
		GetTimestamp: func(key string) (int64, bool) {
			ts, exists := memoryData[key]
			return ts, exists
		},
	}

	err := Explore(
		ctx,
		analyze,
		analyzeFilename,
		memory,
		"key-format-test",
		testHostname,
		testPort,
		testUser,
		testPassword,
		"",
		10,
		2,             // folderCacheDepth=2
		24*time.Hour,
		0,
	)
	if err != nil {
		t.Fatalf("Explore failed: %v", err)
	}

	// Check that we have both file-level and folder-level memory keys
	hasFileKey := false
	hasFolderKey := false

	for _, key := range memorySetKeys {
		if strings.Contains(key, "/folder:") {
			hasFolderKey = true
		} else if strings.Contains(key, "foo.py") {
			// File keys at shallow depth should have timestamp format
			hasFileKey = true
		}
	}

	if !hasFileKey {
		t.Error("expected file-level memory keys for shallow files")
	}
	if !hasFolderKey {
		t.Error("expected folder-level memory keys for deep folders")
	}
}
