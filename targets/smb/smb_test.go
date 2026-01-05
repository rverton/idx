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
			_, err := NewClient(tt.hostname, tt.port, tt.user, tt.password, tt.domain)
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
