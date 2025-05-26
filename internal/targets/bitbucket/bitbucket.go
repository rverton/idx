package bitbucket

import (
	"context" // Added context for VerifyConnection
	"fmt"
	"log/slog" // Added slog for logging in VerifyConnection
	"net/http"
	"strings"
	"time"
)

// APIClient represents a Bitbucket API client.
type APIClient struct {
	BaseURL    string
	Username   string
	Password   string // Or AppPassword
	HTTPClient *http.Client
}

// NewAPIClient creates a new Bitbucket API client.
func NewAPIClient(baseURL, username, password string) (*APIClient, error) {
	if baseURL == "" {
		baseURL = "https://api.bitbucket.org/2.0" // Default Bitbucket Cloud API
	}

	return &APIClient{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// doRequest is a helper to make HTTP requests, adding authentication if configured.
func (c *APIClient) doRequest(req *http.Request) (*http.Response, error) {
	if c.Username != "" && c.Password != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}
	// If no username/password, the request is made anonymously.

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return resp, err // Return resp even on error for status code checking
	}

	// Check for non-2xx status codes and wrap them in an error
	// Allow callers to handle specific status codes if needed.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, fmt.Errorf("API request to %s failed with status %s", req.URL.String(), resp.Status)
	}
	return resp, nil
}

// GetRepositories fetches repositories for the authenticated user.
// This is a placeholder and would need actual implementation.
func (c *APIClient) GetRepositories() (string, error) {
	// Example of a request, actual implementation will vary
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/repositories/%s", c.BaseURL, c.Username), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute GetRepositories request: %w", err)
	}
	defer resp.Body.Close()

	// Placeholder: actual body reading and parsing would happen here.
	// For now, we just confirm the request was successful.

	return fmt.Sprintf("Fetching repositories for %s from %s (implementation pending)", c.Username, c.BaseURL), nil
}

// VerifyConnection checks if the client can connect and authenticate to the Bitbucket API.
func (c *APIClient) VerifyConnection(ctx context.Context) error {
	isCloud := strings.Contains(c.BaseURL, "api.bitbucket.org")

	if c.Username != "" && c.Password != "" {
		// Test (b): Authenticated connection
		var endpointURL string
		if isCloud {
			endpointURL = strings.TrimSuffix(c.BaseURL, "/") + "/user"
		} else {
			// For Bitbucket Server, verify credentials by fetching the user's own details
			endpointURL = strings.TrimSuffix(c.BaseURL, "/") + "/rest/api/1.0/users/" + c.Username
			// Note: Bitbucket Server also often uses /rest/api/latest/ or similar,
			// but /rest/api/1.0/ is a common stable one. Adjust if specific versions are targeted.
		}

		req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create Bitbucket API request for authenticated check: %w", err)
		}

		resp, err := c.doRequest(req) // doRequest will add auth
		if err != nil {
			// err from doRequest might already include status code if it's a non-2xx
			// We are particularly interested in 401/403 for auth failure.
			if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
				return fmt.Errorf("bitbucket authentication failed for user '%s' at %s (HTTP %d): %w", c.Username, endpointURL, resp.StatusCode, err)
			}
			return fmt.Errorf("failed to verify Bitbucket authenticated connection to %s: %w", endpointURL, err)
		}
		defer resp.Body.Close()

		slog.Info("Bitbucket authenticated connection verified successfully.", "username", c.Username, "baseURL", c.BaseURL, "endpoint", endpointURL)
		return nil

	} else {
		// Test (a): Anonymous connection
		var endpointURL string
		if isCloud {
			endpointURL = strings.TrimSuffix(c.BaseURL, "/") + "/repositories/atlassian?pagelen=1"
		} else {
			endpointURL = strings.TrimSuffix(c.BaseURL, "/") + "/rest/api/1.0/projects?limit=1"
		}

		req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create Bitbucket API request for anonymous check: %w", err)
		}

		resp, err := c.doRequest(req) // Makes unauthenticated call
		if err != nil {
			if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
				slog.Warn("Anonymous connection attempt to Bitbucket endpoint indicated authentication is required.", "endpoint", endpointURL, "statusCode", resp.StatusCode)
				// This is not an "error" for the test itself, but indicates the endpoint requires auth.
				// The function's contract is to test if an *anonymous connection* to *this specific endpoint* is successful (2xx).
				// So a 401/403 is a valid outcome of testing "can I connect anonymously to this?" - the answer is "no, auth needed".
				// We return an error to signal the connection attempt (getting a 2xx) failed.
				return fmt.Errorf("anonymous connection to %s failed as authentication is required (HTTP %d): %w", endpointURL, resp.StatusCode, err)
			}
			return fmt.Errorf("failed to test Bitbucket anonymous connection to %s: %w", endpointURL, err)
		}
		defer resp.Body.Close()

		return nil
	}
}

// Add more Bitbucket specific functions here (e.g., GetPullRequests, GetIssues)
