package gitlab

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// APIClient represents a GitLab API client.
type APIClient struct {
	BaseURL     string
	AccessToken string // Personal Access Token
	HTTPClient  *http.Client
}

// NewAPIClient creates a new GitLab API client.
// If no baseURL is provided, it defaults to "https://gitlab.com/api/v4".
func NewAPIClient(baseURL, accessToken string) (*APIClient, error) {
	if baseURL == "" {
		baseURL = "https://gitlab.com/api/v4" // Default GitLab API
	}
	return &APIClient{
		BaseURL:     strings.TrimSuffix(baseURL, "/"),
		AccessToken: accessToken,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// doRequest is a helper to make HTTP requests, adding authentication.
func (c *APIClient) doRequest(req *http.Request) (*http.Response, error) {
	if c.AccessToken != "" {
		req.Header.Set("PRIVATE-TOKEN", c.AccessToken)
	}
	// If no token, the request is made anonymously.

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return resp, err // Return resp even on error for status code checking
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, fmt.Errorf("API request to %s failed with status %s", req.URL.String(), resp.Status)
	}
	return resp, nil
}

// VerifyConnection checks if the client can connect and authenticate to the GitLab API.
func (c *APIClient) VerifyConnection(ctx context.Context) error {
	var endpointURL string

	if c.AccessToken != "" {
		// Test authenticated connection by fetching user details
		endpointURL = c.BaseURL + "/user"
		req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create GitLab API request for authenticated check: %w", err)
		}

		resp, err := c.doRequest(req)
		if err != nil {
			if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
				return fmt.Errorf("GitLab authentication failed (HTTP %d) for endpoint %s: %w", resp.StatusCode, endpointURL, err)
			}
			return fmt.Errorf("failed to verify GitLab authenticated connection to %s: %w", endpointURL, err)
		}
		defer resp.Body.Close()
		slog.Info("GitLab authenticated connection verified successfully.", "baseURL", c.BaseURL, "endpoint", endpointURL)
		return nil
	} else {
		// Test anonymous connection by fetching a public resource (e.g., GitLab version)
		// This endpoint usually does not require authentication.
		endpointURL = c.BaseURL + "/version"
		req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create GitLab API request for anonymous check: %w", err)
		}

		resp, err := c.doRequest(req)
		if err != nil {
			if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
				slog.Warn("Anonymous connection attempt to GitLab endpoint indicated authentication is required.", "endpoint", endpointURL, "statusCode", resp.StatusCode)
				return fmt.Errorf("anonymous connection to %s failed as authentication is required (HTTP %d): %w", endpointURL, resp.StatusCode, err)
			}
			return fmt.Errorf("failed to test GitLab anonymous connection to %s: %w", endpointURL, err)
		}
		defer resp.Body.Close()
		slog.Info("GitLab anonymous connection test to endpoint was successful (2xx).", "endpoint", endpointURL, "baseURL", c.BaseURL)
		return nil
	}
}
