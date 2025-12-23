package bitbucketcloud

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// APIClient represents a Bitbucket Cloud API client.
type APIClient struct {
	BaseURL    string
	Username   string
	ApiToken   string
	HTTPClient *http.Client
}

// NewAPIClient creates a new Bitbucket Cloud API client.
func NewAPIClient(username, apiToken string) (*APIClient, error) {
	if username == "" || apiToken == "" {
		return nil, fmt.Errorf("both username and apiToken must be provided for Bitbucket Cloud API client")
	}

	return &APIClient{
		BaseURL:  "https://api.bitbucket.org/2.0",
		Username: username,
		ApiToken: apiToken,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// doRequest is a helper to make HTTP requests, adding authentication if configured.
func (c *APIClient) doRequest(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(c.Username, c.ApiToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, fmt.Errorf("API request to %s failed with status %s", req.URL.String(), resp.Status)
	}
	return resp, nil
}

// VerifyConnection checks if the client can connect and authenticate to the Bitbucket Cloud API.
func (c *APIClient) VerifyConnection(ctx context.Context) error {
	baseURL := strings.TrimSuffix(c.BaseURL, "/")
	endpointURL := baseURL + "/user"

	req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create Bitbucket Cloud API request: %w", err)
	}

	slog.Debug("Bitbucket Cloud API Request", "method", req.Method, "url", req.URL.String())

	resp, err := c.doRequest(req)
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			return fmt.Errorf("bitbucket cloud authentication failed for user '%s' at %s (HTTP %d): %w", c.Username, endpointURL, resp.StatusCode, err)
		}
		return fmt.Errorf("failed to verify Bitbucket Cloud connection to %s: %w", endpointURL, err)
	}
	defer resp.Body.Close()

	slog.Debug("Bitbucket Cloud authenticated connection verified successfully.", "username", c.Username, "baseURL", c.BaseURL, "endpoint", endpointURL)

	return nil
}

type ResponseRepositories struct {
	Size     int    `json:"size"`
	Page     int    `json:"page"`
	Pagelen  int    `json:"pagelen"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Values   []struct {
		Type  string `json:"type"`
		Links struct {
			Self struct {
				Href string `json:"href"`
				Name string `json:"name"`
			} `json:"self"`
			HTML struct {
				Href string `json:"href"`
				Name string `json:"name"`
			} `json:"html"`
			Avatar struct {
				Href string `json:"href"`
				Name string `json:"name"`
			} `json:"avatar"`
			Pullrequests struct {
				Href string `json:"href"`
				Name string `json:"name"`
			} `json:"pullrequests"`
			Commits struct {
				Href string `json:"href"`
				Name string `json:"name"`
			} `json:"commits"`
			Forks struct {
				Href string `json:"href"`
				Name string `json:"name"`
			} `json:"forks"`
			Watchers struct {
				Href string `json:"href"`
				Name string `json:"name"`
			} `json:"watchers"`
			Downloads struct {
				Href string `json:"href"`
				Name string `json:"name"`
			} `json:"downloads"`
			Clone []struct {
				Href string `json:"href"`
				Name string `json:"name"`
			} `json:"clone"`
			Hooks struct {
				Href string `json:"href"`
				Name string `json:"name"`
			} `json:"hooks"`
		} `json:"links"`
		UUID      string `json:"uuid"`
		FullName  string `json:"full_name"`
		IsPrivate bool   `json:"is_private"`
		Scm       string `json:"scm"`
		Owner     struct {
			Type string `json:"type"`
		} `json:"owner"`
		Name        string `json:"name"`
		Description string `json:"description"`
		CreatedOn   string `json:"created_on"`
		UpdatedOn   string `json:"updated_on"`
		Size        int    `json:"size"`
		Language    string `json:"language"`
		HasIssues   bool   `json:"has_issues"`
		HasWiki     bool   `json:"has_wiki"`
		ForkPolicy  string `json:"fork_policy"`
		Project     struct {
			Type string `json:"type"`
		} `json:"project"`
		Mainbranch struct {
			Type string `json:"type"`
		} `json:"mainbranch"`
	} `json:"values"`
}

// ListRepositories lists all repositories for the given workspaces.
func (c *APIClient) ListRepositories(workspaces []string) ([]string, error) {
	var allRepos []string

	for _, workspace := range workspaces {
		endpointURL := fmt.Sprintf("%s/repositories/%s", c.BaseURL, workspace)

		req, err := http.NewRequest("GET", endpointURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create Bitbucket Cloud API request: %w", err)
		}

		resp, err := c.doRequest(req)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories for workspace %s: %w", workspace, err)
		}
		defer resp.Body.Close()

		var reposResponse ResponseRepositories
		if err := json.NewDecoder(resp.Body).Decode(&reposResponse); err != nil {
			return nil, fmt.Errorf("failed to decode repositories response for workspace %s: %w", workspace, err)
		}

		for _, repo := range reposResponse.Values {
			allRepos = append(allRepos, repo.FullName)
		}

	}

	return allRepos, nil
}
