package bitbucketdc

import (
	"context"
	"fmt"
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
