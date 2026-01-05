//go:build integration

package smb

import (
	"context"
	"idx/detect"
	"strings"
	"testing"
)

const (
	testHostname = "localhost"
	testPort     = 445
	testUser     = "testuser"
	testPassword = "testpass123"
	testShare    = "testshare"
)

func TestIntegration_SMBConnection(t *testing.T) {
	client, err := NewClient(testHostname, testPort, testUser, testPassword, "")
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
	client, err := NewClient(testHostname, testPort, testUser, testPassword, "")
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
	client, err := NewClient(testHostname, testPort, testUser, testPassword, "")
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
	client, err := NewClient(testHostname, testPort, testUser, testPassword, "")
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

	memoryKeys := make(map[string]bool)
	memory := detect.MemoryStore{
		Has: func(key string) bool {
			return memoryKeys[key]
		},
		Set: func(key string) {
			memoryKeys[key] = true
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
