// API Documentation: https://developer.atlassian.com/server/confluence/confluence-server-rest-api/
// Version History API: https://support.atlassian.com/confluence/kb/how-to-retrieve-all-previous-versions-of-content-using-the-rest-api/

package confluencedc

import (
	"context"
	"encoding/json"
	"fmt"
	"idx/detect"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type APIClient struct {
	BaseURL    string
	ApiToken   string
	HTTPClient *http.Client
}

func NewAPIClient(baseURL, apiToken string) (*APIClient, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is required for Confluence Data Center")
	}

	if apiToken == "" {
		return nil, fmt.Errorf("apiToken must be provided for Confluence Data Center API client")
	}

	return &APIClient{
		BaseURL:  strings.TrimSuffix(baseURL, "/"),
		ApiToken: apiToken,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (c *APIClient) doRequest(req *http.Request) (*http.Response, error) {
	// Confluence DC uses Bearer token authentication for PATs
	req.Header.Set("Authorization", "Bearer "+c.ApiToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to read error body for more context
		var errMsg string
		if resp.Body != nil {
			body := make([]byte, 512)
			n, _ := resp.Body.Read(body)
			if n > 0 {
				errMsg = string(body[:n])
			}
		}
		if errMsg != "" {
			return resp, fmt.Errorf("API request to %s failed with status %s: %s", req.URL.String(), resp.Status, errMsg)
		}
		return resp, fmt.Errorf("API request to %s failed with status %s", req.URL.String(), resp.Status)
	}
	return resp, nil
}

func (c *APIClient) VerifyConnection(ctx context.Context) error {
	endpointURL := c.BaseURL + "/rest/api/space?limit=1"

	req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create Confluence DC API request: %w", err)
	}

	slog.Debug("Confluence DC API Request", "method", req.Method, "url", req.URL.String())

	resp, err := c.doRequest(req)
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			return fmt.Errorf("confluence data center authentication failed at %s (HTTP %d): %w", endpointURL, resp.StatusCode, err)
		}
		return fmt.Errorf("failed to verify Confluence DC connection to %s: %w", endpointURL, err)
	}
	defer resp.Body.Close()

	slog.Debug("Confluence Data Center authenticated connection verified successfully.", "baseURL", c.BaseURL)

	return nil
}

type SpacesResponse struct {
	Results []SpaceResult `json:"results"`
	Start   int           `json:"start"`
	Limit   int           `json:"limit"`
	Size    int           `json:"size"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
}

type SpaceResult struct {
	ID   int    `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

func (c *APIClient) listSpaces(ctx context.Context) ([]SpaceResult, error) {
	var allSpaces []SpaceResult
	start := 0
	limit := 100

	for {
		endpointURL := fmt.Sprintf("%s/rest/api/space?start=%d&limit=%d", c.BaseURL, start, limit)

		req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.doRequest(req)
		if err != nil {
			return nil, fmt.Errorf("failed to list spaces: %w", err)
		}
		defer resp.Body.Close()

		var spacesResponse SpacesResponse
		if err := json.NewDecoder(resp.Body).Decode(&spacesResponse); err != nil {
			return nil, fmt.Errorf("failed to decode spaces response: %w", err)
		}

		allSpaces = append(allSpaces, spacesResponse.Results...)

		if spacesResponse.Links.Next == "" || spacesResponse.Size < limit {
			break
		}
		start += limit
	}

	return allSpaces, nil
}

func (c *APIClient) resolveSpaceKey(ctx context.Context, spaceName string) (string, error) {
	spaces, err := c.listSpaces(ctx)
	if err != nil {
		return "", err
	}

	for _, space := range spaces {
		if strings.EqualFold(space.Name, spaceName) || strings.EqualFold(space.Key, spaceName) {
			return space.Key, nil
		}
	}

	return "", fmt.Errorf("space not found: %s", spaceName)
}

type ContentResponse struct {
	Results []ContentResult `json:"results"`
	Start   int             `json:"start"`
	Limit   int             `json:"limit"`
	Size    int             `json:"size"`
	Links   struct {
		Next string `json:"next"`
		Base string `json:"base"`
	} `json:"_links"`
}

type ContentResult struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Title  string `json:"title"`
	Space  struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"space"`
	Body struct {
		Storage struct {
			Value          string `json:"value"`
			Representation string `json:"representation"`
		} `json:"storage"`
	} `json:"body"`
	Version struct {
		Number int `json:"number"`
	} `json:"version"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

func (c *APIClient) listPagesInSpace(ctx context.Context, spaceKey string) ([]ContentResult, error) {
	var allPages []ContentResult
	start := 0
	limit := 100

	for {
		endpointURL := fmt.Sprintf("%s/rest/api/content?spaceKey=%s&type=page&expand=body.storage,version,space&start=%d&limit=%d",
			c.BaseURL, spaceKey, start, limit)

		req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.doRequest(req)
		if err != nil {
			return nil, fmt.Errorf("failed to list pages for space %s: %w", spaceKey, err)
		}
		defer resp.Body.Close()

		var contentResponse ContentResponse
		if err := json.NewDecoder(resp.Body).Decode(&contentResponse); err != nil {
			return nil, fmt.Errorf("failed to decode content response for space %s: %w", spaceKey, err)
		}

		allPages = append(allPages, contentResponse.Results...)

		if contentResponse.Links.Next == "" || contentResponse.Size < limit {
			break
		}
		start += limit
	}

	return allPages, nil
}

type VersionsResponse struct {
	Results []VersionResult `json:"results"`
	Start   int             `json:"start"`
	Limit   int             `json:"limit"`
	Size    int             `json:"size"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
}

type VersionResult struct {
	Number int `json:"number"`
}

func (c *APIClient) listPageVersions(ctx context.Context, pageID string) ([]int, error) {
	var versions []int
	start := 0
	limit := 100

	for {
		endpointURL := fmt.Sprintf("%s/rest/experimental/content/%s/version?start=%d&limit=%d",
			c.BaseURL, pageID, start, limit)

		req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.doRequest(req)
		if err != nil {
			return nil, fmt.Errorf("failed to list versions for page %s: %w", pageID, err)
		}
		defer resp.Body.Close()

		var versionsResponse VersionsResponse
		if err := json.NewDecoder(resp.Body).Decode(&versionsResponse); err != nil {
			return nil, fmt.Errorf("failed to decode versions response for page %s: %w", pageID, err)
		}

		for _, v := range versionsResponse.Results {
			versions = append(versions, v.Number)
		}

		if versionsResponse.Links.Next == "" || versionsResponse.Size < limit {
			break
		}
		start += limit
	}

	return versions, nil
}

type VersionContentResponse struct {
	Content struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Body  struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
	} `json:"content"`
	Number int `json:"number"`
}

func (c *APIClient) getPageVersionContent(ctx context.Context, pageID string, version int) (*VersionContentResponse, error) {
	endpointURL := fmt.Sprintf("%s/rest/experimental/content/%s/version/%d?expand=content.body.storage",
		c.BaseURL, pageID, version)

	req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get version %d for page %s: %w", version, pageID, err)
	}
	defer resp.Body.Close()

	var versionContent VersionContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&versionContent); err != nil {
		return nil, fmt.Errorf("failed to decode version content for page %s version %d: %w", pageID, version, err)
	}

	return &versionContent, nil
}

func Explore(
	ctx context.Context,
	analyze func(content detect.Content),
	memory detect.MemoryStore,
	name string,
	baseURL string,
	apiToken string,
	spaceNames []string,
	disableHistorySearch bool,
) error {
	client, err := NewAPIClient(baseURL, apiToken)
	if err != nil {
		return fmt.Errorf("confluence-dc: %w", err)
	}

	for _, spaceName := range spaceNames {
		spaceKey, err := client.resolveSpaceKey(ctx, spaceName)
		if err != nil {
			slog.Error("confluence-dc: failed to resolve space", "space", spaceName, "error", err)
			continue
		}

		slog.Debug("confluence-dc: resolved space", "name", spaceName, "key", spaceKey)

		pages, err := client.listPagesInSpace(ctx, spaceKey)
		if err != nil {
			slog.Error("confluence-dc: failed to list pages", "space", spaceKey, "error", err)
			continue
		}

		slog.Info("confluence-dc pages", "target", name, "space", spaceKey, "count", len(pages))

		for _, page := range pages {
			var versions []int

			if disableHistorySearch {
				versions = []int{page.Version.Number}
			} else {
				versions, err = client.listPageVersions(ctx, page.ID)
				if err != nil {
					slog.Error("confluence-dc: failed to list versions", "page", page.Title, "error", err)
					continue
				}
				slog.Debug("confluence-dc: page versions", "page", page.Title, "versions", len(versions))
			}

			for _, version := range versions {
				memoryKey := fmt.Sprintf("confluence-dc/%s/%s/%s/v%d", name, spaceKey, page.ID, version)

				if memory.Has(memoryKey) {
					slog.Debug("skipping already analyzed page version", "space", spaceKey, "page", page.Title, "version", version)
					continue
				}

				var pageContent string
				if disableHistorySearch {
					pageContent = page.Body.Storage.Value
				} else {
					versionContent, err := client.getPageVersionContent(ctx, page.ID, version)
					if err != nil {
						slog.Error("confluence-dc: failed to get version content", "page", page.Title, "version", version, "error", err)
						continue
					}
					pageContent = versionContent.Content.Body.Storage.Value
				}

				if pageContent == "" {
					continue
				}

				content := detect.Content{
					Key:  fmt.Sprintf("%s:%s:v%d", spaceKey, page.ID, version),
					Data: []byte(pageContent),
					Location: []string{
						"confluence-dc",
						spaceKey,
						page.ID,
						page.Title,
						fmt.Sprintf("v%d", version),
					},
				}
				analyze(content)
				memory.Set(memoryKey)

				slog.Debug("analyzed page", "space", spaceKey, "page", page.Title, "version", version)
			}
		}
	}

	return nil
}
