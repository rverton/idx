package confluencedc

import (
	"context"
	"encoding/json"
	"idx/detect"
	"net/http"
	"net/http/httptest"
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
			baseURL:  "http://localhost:8090",
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
			baseURL:   "http://localhost:8090",
			apiToken:  "",
			wantErr:   true,
			errSubstr: "apiToken must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewAPIClient(tt.baseURL, tt.apiToken)
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
	client, err := NewAPIClient("http://localhost:8090/", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.BaseURL != "http://localhost:8090" {
		t.Errorf("expected trailing slash to be trimmed, got %q", client.BaseURL)
	}
}

func TestResolveSpaceKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/space" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		response := SpacesResponse{
			Results: []SpaceResult{
				{ID: 1, Key: "DS", Name: "Demo Space"},
				{ID: 2, Key: "TEST", Name: "Test Space"},
			},
			Size: 2,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, _ := NewAPIClient(server.URL, "token")

	tests := []struct {
		name      string
		spaceName string
		wantKey   string
		wantErr   bool
	}{
		{
			name:      "find by name",
			spaceName: "Demo Space",
			wantKey:   "DS",
			wantErr:   false,
		},
		{
			name:      "find by name case insensitive",
			spaceName: "demo space",
			wantKey:   "DS",
			wantErr:   false,
		},
		{
			name:      "find by key",
			spaceName: "TEST",
			wantKey:   "TEST",
			wantErr:   false,
		},
		{
			name:      "find by key case insensitive",
			spaceName: "test",
			wantKey:   "TEST",
			wantErr:   false,
		},
		{
			name:      "not found",
			spaceName: "NonExistent",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := client.resolveSpaceKey(context.Background(), tt.spaceName)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if key != tt.wantKey {
					t.Errorf("got key %q, want %q", key, tt.wantKey)
				}
			}
		})
	}
}

func TestListPagesInSpace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/rest/api/content") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		spaceKey := r.URL.Query().Get("spaceKey")
		if spaceKey != "TEST" {
			t.Errorf("expected spaceKey=TEST, got %s", spaceKey)
		}

		response := ContentResponse{
			Results: []ContentResult{
				{
					ID:    "123",
					Title: "Page One",
					Body: struct {
						Storage struct {
							Value          string `json:"value"`
							Representation string `json:"representation"`
						} `json:"storage"`
					}{
						Storage: struct {
							Value          string `json:"value"`
							Representation string `json:"representation"`
						}{
							Value: "<p>Content of page one</p>",
						},
					},
					Version: struct {
						Number int `json:"number"`
					}{Number: 1},
				},
				{
					ID:    "456",
					Title: "Page Two",
					Body: struct {
						Storage struct {
							Value          string `json:"value"`
							Representation string `json:"representation"`
						} `json:"storage"`
					}{
						Storage: struct {
							Value          string `json:"value"`
							Representation string `json:"representation"`
						}{
							Value: "<p>Content of page two</p>",
						},
					},
					Version: struct {
						Number int `json:"number"`
					}{Number: 3},
				},
			},
			Size: 2,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, _ := NewAPIClient(server.URL, "token")

	pages, err := client.listPagesInSpace(context.Background(), "TEST")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pages) != 2 {
		t.Errorf("expected 2 pages, got %d", len(pages))
	}

	if pages[0].ID != "123" || pages[0].Title != "Page One" {
		t.Errorf("unexpected first page: %+v", pages[0])
	}

	if pages[1].ID != "456" || pages[1].Title != "Page Two" {
		t.Errorf("unexpected second page: %+v", pages[1])
	}
}

func TestListPageVersions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/rest/experimental/content/123/version") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		response := VersionsResponse{
			Results: []VersionResult{
				{Number: 1},
				{Number: 2},
				{Number: 3},
			},
			Size: 3,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, _ := NewAPIClient(server.URL, "token")

	versions, err := client.listPageVersions(context.Background(), "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(versions))
	}

	expected := []int{1, 2, 3}
	for i, v := range versions {
		if v != expected[i] {
			t.Errorf("version[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestGetPageVersionContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/experimental/content/123/version/2" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		response := VersionContentResponse{
			Number: 2,
			Content: struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				Body  struct {
					Storage struct {
						Value string `json:"value"`
					} `json:"storage"`
				} `json:"body"`
			}{
				ID:    "123",
				Title: "Test Page",
				Body: struct {
					Storage struct {
						Value string `json:"value"`
					} `json:"storage"`
				}{
					Storage: struct {
						Value string `json:"value"`
					}{
						Value: "<p>Version 2 content</p>",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, _ := NewAPIClient(server.URL, "token")

	content, err := client.getPageVersionContent(context.Background(), "123", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if content.Number != 2 {
		t.Errorf("expected version 2, got %d", content.Number)
	}

	if content.Content.Body.Storage.Value != "<p>Version 2 content</p>" {
		t.Errorf("unexpected content: %s", content.Content.Body.Storage.Value)
	}
}

func TestVerifyConnection(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name:       "forbidden",
			statusCode: http.StatusForbidden,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/rest/api/space" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(SpacesResponse{})
			}))
			defer server.Close()

			client, _ := NewAPIClient(server.URL, "token")

			err := client.VerifyConnection(context.Background())
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestExplore_DisableHistorySearch(t *testing.T) {
	requestPaths := make([]string, 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)

		switch {
		case r.URL.Path == "/rest/api/space":
			response := SpacesResponse{
				Results: []SpaceResult{{ID: 1, Key: "TEST", Name: "Test Space"}},
				Size:    1,
			}
			json.NewEncoder(w).Encode(response)

		case strings.HasPrefix(r.URL.Path, "/rest/api/content"):
			response := ContentResponse{
				Results: []ContentResult{
					{
						ID:    "123",
						Title: "Test Page",
						Body: struct {
							Storage struct {
								Value          string `json:"value"`
								Representation string `json:"representation"`
							} `json:"storage"`
						}{
							Storage: struct {
								Value          string `json:"value"`
								Representation string `json:"representation"`
							}{
								Value: "<p>Test content with secret api_key=\"abc123def456ghi789jkl012mno345pq\"</p>",
							},
						},
						Version: struct {
							Number int `json:"number"`
						}{Number: 5},
					},
				},
				Size: 1,
			}
			json.NewEncoder(w).Encode(response)

		default:
			t.Errorf("unexpected request to: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	var contents []detect.Content
	analyze := func(content detect.Content) {
		contents = append(contents, content)
	}

	memory := detect.MemoryStore{
		Has: func(key string) bool { return false },
		Set: func(key string) {},
	}

	err := Explore(
		context.Background(),
		analyze,
		memory,
		"test-target",
		server.URL,
		"token",
		[]string{"Test Space"},
		true, // disableHistorySearch
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT have called experimental version API
	for _, path := range requestPaths {
		if strings.Contains(path, "/rest/experimental/") {
			t.Errorf("should not call experimental API when history search is disabled, but called: %s", path)
		}
	}

	if len(contents) != 1 {
		t.Errorf("expected 1 content, got %d", len(contents))
	}

	if len(contents) > 0 {
		if !strings.Contains(string(contents[0].Data), "api_key") {
			t.Error("expected content to contain the page data")
		}
		if contents[0].Key != "TEST:123:v5" {
			t.Errorf("unexpected key: %s", contents[0].Key)
		}
	}
}

func TestExplore_WithHistorySearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/space":
			response := SpacesResponse{
				Results: []SpaceResult{{ID: 1, Key: "TEST", Name: "Test Space"}},
				Size:    1,
			}
			json.NewEncoder(w).Encode(response)

		case strings.HasPrefix(r.URL.Path, "/rest/api/content"):
			response := ContentResponse{
				Results: []ContentResult{
					{
						ID:    "123",
						Title: "Test Page",
						Version: struct {
							Number int `json:"number"`
						}{Number: 2},
					},
				},
				Size: 1,
			}
			json.NewEncoder(w).Encode(response)

		case r.URL.Path == "/rest/experimental/content/123/version":
			response := VersionsResponse{
				Results: []VersionResult{{Number: 1}, {Number: 2}},
				Size:    2,
			}
			json.NewEncoder(w).Encode(response)

		case strings.HasPrefix(r.URL.Path, "/rest/experimental/content/123/version/"):
			version := strings.TrimPrefix(r.URL.Path, "/rest/experimental/content/123/version/")
			response := VersionContentResponse{
				Content: struct {
					ID    string `json:"id"`
					Title string `json:"title"`
					Body  struct {
						Storage struct {
							Value string `json:"value"`
						} `json:"storage"`
					} `json:"body"`
				}{
					ID:    "123",
					Title: "Test Page",
					Body: struct {
						Storage struct {
							Value string `json:"value"`
						} `json:"storage"`
					}{
						Storage: struct {
							Value string `json:"value"`
						}{
							Value: "<p>Content version " + version + "</p>",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	var contents []detect.Content
	analyze := func(content detect.Content) {
		contents = append(contents, content)
	}

	memory := detect.MemoryStore{
		Has: func(key string) bool { return false },
		Set: func(key string) {},
	}

	err := Explore(
		context.Background(),
		analyze,
		memory,
		"test-target",
		server.URL,
		"token",
		[]string{"Test Space"},
		false, // enable history search
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(contents) != 2 {
		t.Errorf("expected 2 contents (one per version), got %d", len(contents))
	}
}

func TestExplore_MemoryStoreSkipsAnalyzed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/space":
			json.NewEncoder(w).Encode(SpacesResponse{
				Results: []SpaceResult{{ID: 1, Key: "TEST", Name: "Test Space"}},
				Size:    1,
			})

		case strings.HasPrefix(r.URL.Path, "/rest/api/content"):
			json.NewEncoder(w).Encode(ContentResponse{
				Results: []ContentResult{
					{
						ID:    "123",
						Title: "Test Page",
						Body: struct {
							Storage struct {
								Value          string `json:"value"`
								Representation string `json:"representation"`
							} `json:"storage"`
						}{
							Storage: struct {
								Value          string `json:"value"`
								Representation string `json:"representation"`
							}{Value: "<p>Content</p>"},
						},
						Version: struct {
							Number int `json:"number"`
						}{Number: 1},
					},
				},
				Size: 1,
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	seenKeys := make(map[string]bool)
	memory := detect.MemoryStore{
		Has: func(key string) bool { return seenKeys[key] },
		Set: func(key string) { seenKeys[key] = true },
	}

	var firstRunContents []detect.Content
	err := Explore(
		context.Background(),
		func(c detect.Content) { firstRunContents = append(firstRunContents, c) },
		memory,
		"test-target",
		server.URL,
		"token",
		[]string{"Test Space"},
		true,
	)
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}

	if len(firstRunContents) != 1 {
		t.Fatalf("expected 1 content on first run, got %d", len(firstRunContents))
	}

	var secondRunContents []detect.Content
	err = Explore(
		context.Background(),
		func(c detect.Content) { secondRunContents = append(secondRunContents, c) },
		memory,
		"test-target",
		server.URL,
		"token",
		[]string{"Test Space"},
		true,
	)
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}

	if len(secondRunContents) != 0 {
		t.Errorf("expected 0 contents on second run (skipped by memory), got %d", len(secondRunContents))
	}
}

func TestExplore_ContentFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/space":
			json.NewEncoder(w).Encode(SpacesResponse{
				Results: []SpaceResult{{ID: 1, Key: "MYSPACE", Name: "My Space"}},
				Size:    1,
			})

		case strings.HasPrefix(r.URL.Path, "/rest/api/content"):
			json.NewEncoder(w).Encode(ContentResponse{
				Results: []ContentResult{
					{
						ID:    "456",
						Title: "Important Page",
						Body: struct {
							Storage struct {
								Value          string `json:"value"`
								Representation string `json:"representation"`
							} `json:"storage"`
						}{
							Storage: struct {
								Value          string `json:"value"`
								Representation string `json:"representation"`
							}{Value: "<p>Secret content here</p>"},
						},
						Version: struct {
							Number int `json:"number"`
						}{Number: 3},
					},
				},
				Size: 1,
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	var contents []detect.Content
	memory := detect.MemoryStore{
		Has: func(key string) bool { return false },
		Set: func(key string) {},
	}

	err := Explore(
		context.Background(),
		func(c detect.Content) { contents = append(contents, c) },
		memory,
		"test-target",
		server.URL,
		"token",
		[]string{"My Space"},
		true,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}

	c := contents[0]

	// Check Key format
	if c.Key != "MYSPACE:456:v3" {
		t.Errorf("unexpected Key: %s, want MYSPACE:456:v3", c.Key)
	}

	// Check Data
	if string(c.Data) != "<p>Secret content here</p>" {
		t.Errorf("unexpected Data: %s", string(c.Data))
	}

	// Check Location
	expectedLocation := []string{"confluence-dc", "MYSPACE", "456", "Important Page", "v3"}
	if len(c.Location) != len(expectedLocation) {
		t.Errorf("expected Location length %d, got %d", len(expectedLocation), len(c.Location))
	}
	for i, loc := range expectedLocation {
		if i < len(c.Location) && c.Location[i] != loc {
			t.Errorf("Location[%d] = %q, want %q", i, c.Location[i], loc)
		}
	}
}

func TestDoRequest_BearerAuth(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewAPIClient(server.URL, "my-secret-token")

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	_, err := client.doRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if authHeader != "Bearer my-secret-token" {
		t.Errorf("expected 'Bearer my-secret-token', got %q", authHeader)
	}
}
