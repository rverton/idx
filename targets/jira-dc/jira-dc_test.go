package jiradc

import (
	"strings"
	"testing"
)

func TestNewAPIClient(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		apiToken  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "valid credentials",
			baseURL:  "http://localhost:8080",
			apiToken: "token123",
			wantErr:  false,
		},
		{
			name:      "missing baseURL",
			baseURL:   "",
			apiToken:  "token123",
			wantErr:   true,
			errSubstr: "baseURL is required",
		},
		{
			name:      "missing apiToken",
			baseURL:   "http://localhost:8080",
			apiToken:  "",
			wantErr:   true,
			errSubstr: "apiToken must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewAPIClient(tt.baseURL, tt.apiToken, 0)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("expected error containing %q, got %q", tt.errSubstr, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if client == nil {
					t.Error("expected client, got nil")
				}
			}
		})
	}
}

func TestNewAPIClient_TrimsTrailingSlash(t *testing.T) {
	client, err := NewAPIClient("http://localhost:8080/", "token123", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.BaseURL != "http://localhost:8080" {
		t.Errorf("expected BaseURL without trailing slash, got %q", client.BaseURL)
	}
}

func TestIsTextMimeType(t *testing.T) {
	tests := []struct {
		mimeType string
		want     bool
	}{
		// Text types - should match
		{"text/plain", true},
		{"text/html", true},
		{"text/css", true},
		{"text/csv", true},
		{"text/xml", true},
		{"text/markdown", true},

		// Application types that are text-based - should match
		{"application/json", true},
		{"application/xml", true},
		{"application/javascript", true},
		{"application/x-sh", true},
		{"application/x-yaml", true},
		{"application/sql", true},
		{"application/x-iwork-keynote-sffkey", true},

		// Binary types - should not match
		{"application/octet-stream", false},
		{"application/pdf", false},
		{"application/zip", false},
		{"application/gzip", false},
		{"application/x-tar", false},

		// Image types - should not match
		{"image/png", false},
		{"image/jpeg", false},
		{"image/gif", false},
		{"image/svg+xml", false},

		// Video/audio types - should not match
		{"video/mp4", false},
		{"audio/mpeg", false},

		// Empty and edge cases
		{"", false},
		{"unknown/type", false},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			got := isTextMimeType(tt.mimeType)
			if got != tt.want {
				t.Errorf("isTextMimeType(%q) = %v, want %v", tt.mimeType, got, tt.want)
			}
		})
	}
}
