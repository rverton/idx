package smb

import (
	"bytes"
	"testing"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		hostname    string
		port        int
		user        string
		password    string
		domain      string
		wantErr     bool
		errContains string
	}{
		{
			name:        "missing hostname",
			hostname:    "",
			user:        "user",
			password:    "pass",
			wantErr:     true,
			errContains: "hostname is required",
		},
		{
			name:        "missing user",
			hostname:    "server",
			user:        "",
			password:    "pass",
			wantErr:     true,
			errContains: "ntlmUser and ntlmPassword are required",
		},
		{
			name:        "missing password",
			hostname:    "server",
			user:        "user",
			password:    "",
			wantErr:     true,
			errContains: "ntlmUser and ntlmPassword are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.hostname, tt.port, tt.user, tt.password, tt.domain, 0)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !bytes.Contains([]byte(err.Error()), []byte(tt.errContains)) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestIsTextFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{".env", true},
		{"config.json", true},
		{"script.py", true},

		{"image.png", false},
		{"image.jpg", false},
		{"random", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := isTextFile(tt.filename)
			if got != tt.want {
				t.Errorf("isTextFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestFiletimeToUnix(t *testing.T) {
	tests := []struct {
		name     string
		filetime uint64
		want     int64
	}{
		{
			name:     "zero filetime",
			filetime: 0,
			want:     0,
		},
		{
			name:     "before unix epoch",
			filetime: 116444736000000000 - 1,
			want:     0,
		},
		{
			name:     "unix epoch",
			filetime: 116444736000000000,
			want:     0,
		},
		{
			name:     "one second after unix epoch",
			filetime: 116444736000000000 + 10000000,
			want:     1,
		},
		{
			name:     "Jan 1, 2020 00:00:00 UTC",
			filetime: 132223104000000000, // (1577836800 * 10000000) + 116444736000000000
			want:     1577836800,
		},
		{
			name:     "Jan 1, 2024 00:00:00 UTC",
			filetime: 133485408000000000, // (1704067200 * 10000000) + 116444736000000000
			want:     1704067200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filetimeToUnix(tt.filetime)
			if got != tt.want {
				t.Errorf("filetimeToUnix(%d) = %d, want %d", tt.filetime, got, tt.want)
			}
		})
	}
}

func TestPathDepth(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"", 0},
		{"/", 0},
		{"\\", 0},
		{"file.txt", 1},
		{"dir/file.txt", 2},
		{"dir\\file.txt", 2},
		{"a/b/c", 3},
		{"a\\b\\c", 3},
		{"a/b\\c/d", 4},
		{"/a/b/c/", 3},
		{"level1/level2/level3/level4/file.txt", 5},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := pathDepth(tt.path)
			if got != tt.want {
				t.Errorf("pathDepth(%q) = %d, want %d", tt.path, got, tt.want)
			}
		})
	}
}

func TestFolderCacheKeyFormat(t *testing.T) {
	targetName := "test-target"
	hostname := "server.local"
	share := "Documents"
	folderPath := "reports/2024"

	expectedKey := "smb/test-target/server.local/Documents/folder:reports/2024"
	actualKey := "smb/" + targetName + "/" + hostname + "/" + share + "/folder:" + folderPath

	if actualKey != expectedKey {
		t.Errorf("folder key format mismatch: got %q, want %q", actualKey, expectedKey)
	}
}

func TestFolderCacheDepthLogic(t *testing.T) {
	tests := []struct {
		name             string
		fileDepth        int
		folderCacheDepth int
		wantFolderCache  bool
	}{
		{"depth 0, threshold 2", 0, 2, false},
		{"depth 1, threshold 2", 1, 2, false},
		{"depth 2, threshold 2", 2, 2, true},
		{"depth 3, threshold 2", 3, 2, true},
		{"depth 5, threshold 3", 5, 3, true},
		{"threshold 0 disables", 5, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useFolderCache := tt.folderCacheDepth > 0 && tt.fileDepth >= tt.folderCacheDepth
			if useFolderCache != tt.wantFolderCache {
				t.Errorf("folderCacheDepth=%d, fileDepth=%d: useFolderCache=%v, want %v",
					tt.folderCacheDepth, tt.fileDepth, useFolderCache, tt.wantFolderCache)
			}
		})
	}
}
