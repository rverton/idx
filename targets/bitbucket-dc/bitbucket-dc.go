// API Documentation: https://docs.atlassian.com/bitbucket-server/rest/5.15.0/bitbucket-rest.html

package bitbucketdc

import (
	"context"
	"encoding/json"
	"fmt"
	"idx/detect"
	"idx/tools"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// APIClient represents a Bitbucket Data Center/Server API client.
type APIClient struct {
	BaseURL    string
	Username   string
	ApiToken   string
	HTTPClient *http.Client
}

// NewAPIClient creates a new Bitbucket Data Center/Server API client.
func NewAPIClient(baseURL, username, apiToken string) (*APIClient, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is required for Bitbucket Data Center")
	}

	if username == "" || apiToken == "" {
		return nil, fmt.Errorf("both username and apiToken must be provided for Bitbucket Data Center API client")
	}

	return &APIClient{
		BaseURL:  baseURL,
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

// VerifyConnection checks if the client can connect and authenticate to the Bitbucket Data Center API.
func (c *APIClient) VerifyConnection(ctx context.Context) error {
	baseURL := strings.TrimSuffix(c.BaseURL, "/")
	endpointURL := baseURL + "/rest/api/1.0/users/" + c.Username

	req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create Bitbucket DC API request: %w", err)
	}

	slog.Debug("Bitbucket DC API Request", "method", req.Method, "url", req.URL.String())

	resp, err := c.doRequest(req)
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			return fmt.Errorf("bitbucket data center authentication failed for user '%s' at %s (HTTP %d): %w", c.Username, endpointURL, resp.StatusCode, err)
		}
		return fmt.Errorf("failed to verify Bitbucket DC connection to %s: %w", endpointURL, err)
	}
	defer resp.Body.Close()

	slog.Info("Bitbucket Data Center authenticated connection verified successfully.", "username", c.Username, "baseURL", c.BaseURL, "endpoint", endpointURL)

	return nil
}

func (c *APIClient) RepoURL(projectKey, repoSlug string) string {
	baseURL := strings.TrimSuffix(c.BaseURL, "/")
	return fmt.Sprintf("%s/scm/%s/%s.git", baseURL, strings.ToLower(projectKey), repoSlug)
}

type PagedResponse struct {
	Size          int  `json:"size"`
	Limit         int  `json:"limit"`
	IsLastPage    bool `json:"isLastPage"`
	Start         int  `json:"start"`
	NextPageStart int  `json:"nextPageStart"`
}

// GET /rest/api/1.0/projects
type ProjectsResponse struct {
	PagedResponse
	Values []struct {
		Key    string `json:"key"`
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Public bool   `json:"public"`
		Type   string `json:"type"`
	} `json:"values"`
}

// GET /rest/api/1.0/projects/{projectKey}/repos
type RepositoriesResponse struct {
	PagedResponse
	Values []struct {
		Slug    string `json:"slug"`
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Project struct {
			Key string `json:"key"`
		} `json:"project"`
		Public bool `json:"public"`
	} `json:"values"`
}

func (c *APIClient) listProjects(ctx context.Context) ([]string, error) {
	var allProjects []string
	baseURL := strings.TrimSuffix(c.BaseURL, "/")
	start := 0

	for {
		endpointURL := fmt.Sprintf("%s/rest/api/1.0/projects?start=%d&limit=100", baseURL, start)

		req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.doRequest(req)
		if err != nil {
			return nil, fmt.Errorf("failed to list projects: %w", err)
		}
		defer resp.Body.Close()

		var projectsResponse ProjectsResponse
		if err := json.NewDecoder(resp.Body).Decode(&projectsResponse); err != nil {
			return nil, fmt.Errorf("failed to decode projects response: %w", err)
		}

		for _, project := range projectsResponse.Values {
			allProjects = append(allProjects, project.Key)
		}

		if projectsResponse.IsLastPage {
			break
		}
		start = projectsResponse.NextPageStart
	}

	return allProjects, nil
}

func (c *APIClient) listRepositories(ctx context.Context, projectKey string) ([]string, error) {
	var allRepos []string
	baseURL := strings.TrimSuffix(c.BaseURL, "/")
	start := 0

	for {
		endpointURL := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos?start=%d&limit=100", baseURL, projectKey, start)

		req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.doRequest(req)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories for project %s: %w", projectKey, err)
		}
		defer resp.Body.Close()

		var reposResponse RepositoriesResponse
		if err := json.NewDecoder(resp.Body).Decode(&reposResponse); err != nil {
			return nil, fmt.Errorf("failed to decode repositories response: %w", err)
		}

		for _, repo := range reposResponse.Values {
			allRepos = append(allRepos, repo.Slug)
		}

		if reposResponse.IsLastPage {
			break
		}
		start = reposResponse.NextPageStart
	}

	return allRepos, nil
}

func Explore(
	ctx context.Context,
	analyze func(content detect.Content),
	memory detect.MemoryStore,
	name string,
	baseURL string,
	username string,
	apiToken string,
) error {
	client, err := NewAPIClient(baseURL, username, apiToken)
	if err != nil {
		return fmt.Errorf("bitbucket-dc: %w", err)
	}

	projects, err := client.listProjects(ctx)
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	slog.Info("bitbucket-dc projects", "target", name, "count", len(projects), "projects", projects)

	for _, projectKey := range projects {
		repos, err := client.listRepositories(ctx, projectKey)
		if err != nil {
			slog.Error("bitbucket-dc: failed to list repositories", "project", projectKey, "error", err)
			continue
		}

		slog.Info("bitbucket-dc repositories", "target", name, "project", projectKey, "count", len(repos))

		for _, repoSlug := range repos {
			repoPath, cleanup, err := tools.CloneRepository(
				ctx,
				client.RepoURL(projectKey, repoSlug),
				username,
				apiToken,
			)
			if err != nil {
				slog.Error("bitbucket-dc: failed to clone repository", "project", projectKey, "repository", repoSlug, "error", err)
				continue
			}
			defer cleanup()

			repoFullName := fmt.Sprintf("%s/%s", projectKey, repoSlug)
			slog.Info("bitbucket-dc: cloned repository", "repository", repoFullName, "path", repoPath)

			analyseRepo(repoPath, repoFullName, name, analyze, memory)
		}
	}

	return nil
}

func analyseRepo(path, repo, targetName string, analyze func(content detect.Content), memory detect.MemoryStore) {
	var lastCommit string
	err := tools.IterateCommits(path, func(fc tools.FileChange) error {
		memoryKey := fmt.Sprintf("bitbucket-dc/%s/%s/%s", targetName, repo, fc.CommitHash)

		if fc.CommitHash != lastCommit {
			if memory.Has(memoryKey) {
				slog.Debug("skipping already analyzed commit", "repo", repo, "commit", fc.CommitHash[:8])
				return nil
			}
			lastCommit = fc.CommitHash
		}

		additions := fc.Additions()
		if additions == "" {
			return nil
		}

		content := detect.Content{
			Key:  fmt.Sprintf("%s:%s:%s", repo, fc.CommitHash[:8], fc.FilePath),
			Data: []byte(additions),
			Location: []string{
				"bitbucket-dc",
				repo,
				fc.CommitHash,
				fc.FilePath,
			},
		}
		analyze(content)
		memory.Set(memoryKey)
		return nil
	})
	if err != nil {
		slog.Error("failed to iterate commits", "repo", repo, "error", err)
	}
}
