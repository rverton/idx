// API Documentation: https://docs.atlassian.com/software/jira/docs/api/REST/9.14.0/

package jiradc

import (
	"context"
	"encoding/json"
	"fmt"
	"idx/detect"
	"idx/tools"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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
		return nil, fmt.Errorf("baseURL is required for Jira Data Center")
	}

	if apiToken == "" {
		return nil, fmt.Errorf("apiToken must be provided for Jira Data Center API client")
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
	req.Header.Set("Authorization", "Bearer "+c.ApiToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
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
	endpointURL := c.BaseURL + "/rest/api/2/myself"

	req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create Jira DC API request: %w", err)
	}

	slog.Debug("Jira DC API Request", "method", req.Method, "url", req.URL.String())

	resp, err := c.doRequest(req)
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			return fmt.Errorf("jira data center authentication failed at %s (HTTP %d): %w", endpointURL, resp.StatusCode, err)
		}
		return fmt.Errorf("failed to verify Jira DC connection to %s: %w", endpointURL, err)
	}
	defer resp.Body.Close()

	slog.Debug("Jira Data Center authenticated connection verified successfully.", "baseURL", c.BaseURL)

	return nil
}

type SearchResponse struct {
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
	Issues     []Issue `json:"issues"`
}

type Issue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Fields struct {
		Summary     string `json:"summary"`
		Description string `json:"description"`
		Updated     string `json:"updated"`
		Project     struct {
			Key string `json:"key"`
		} `json:"project"`
		Attachment []Attachment `json:"attachment"`
	} `json:"fields"`
}

type CommentsResponse struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Comments   []Comment `json:"comments"`
}

type Comment struct {
	ID      string `json:"id"`
	Body    string `json:"body"`
	Updated string `json:"updated"`
	Author  struct {
		DisplayName string `json:"displayName"`
	} `json:"author"`
}

type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mimeType"`
	Size     int    `json:"size"`
	Content  string `json:"content"`
}

func (c *APIClient) searchIssues(ctx context.Context, projectKey string) ([]Issue, error) {
	var allIssues []Issue
	startAt := 0
	maxResults := 100

	jql := fmt.Sprintf("project = %s", projectKey)

	for {
		endpointURL := fmt.Sprintf("%s/rest/api/2/search?jql=%s&startAt=%d&maxResults=%d&fields=summary,description,updated,project,attachment",
			c.BaseURL, url.QueryEscape(jql), startAt, maxResults)

		req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.doRequest(req)
		if err != nil {
			return nil, fmt.Errorf("failed to search issues for project %s: %w", projectKey, err)
		}
		defer resp.Body.Close()

		var searchResponse SearchResponse
		if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
			return nil, fmt.Errorf("failed to decode search response for project %s: %w", projectKey, err)
		}

		allIssues = append(allIssues, searchResponse.Issues...)

		if startAt+len(searchResponse.Issues) >= searchResponse.Total {
			break
		}
		startAt += maxResults
	}

	return allIssues, nil
}

func (c *APIClient) getComments(ctx context.Context, issueKey string) ([]Comment, error) {
	var allComments []Comment
	startAt := 0
	maxResults := 100

	for {
		endpointURL := fmt.Sprintf("%s/rest/api/2/issue/%s/comment?startAt=%d&maxResults=%d",
			c.BaseURL, issueKey, startAt, maxResults)

		req, err := http.NewRequestWithContext(ctx, "GET", endpointURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.doRequest(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get comments for issue %s: %w", issueKey, err)
		}
		defer resp.Body.Close()

		var commentsResponse CommentsResponse
		if err := json.NewDecoder(resp.Body).Decode(&commentsResponse); err != nil {
			return nil, fmt.Errorf("failed to decode comments response for issue %s: %w", issueKey, err)
		}

		allComments = append(allComments, commentsResponse.Comments...)

		if startAt+len(commentsResponse.Comments) >= commentsResponse.Total {
			break
		}
		startAt += maxResults
	}

	return allComments, nil
}

const maxAttachmentSize = 1 * 1024 * 1024 // 1MB

func (c *APIClient) downloadAttachment(ctx context.Context, contentURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", contentURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download attachment: %w", err)
	}
	defer resp.Body.Close()

	// Limit download to first 1MB
	limitedReader := io.LimitReader(resp.Body, maxAttachmentSize)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read attachment content: %w", err)
	}

	return data, nil
}

func isTextMimeType(mimeType string) bool {
	textTypes := []string{
		"text/",
		"application/json",
		"application/xml",
		"application/javascript",
		"application/x-sh",
		"application/x-yaml",
		"application/sql",
		"application/x-iwork-keynote-sffkey", // .key files
	}
	for _, t := range textTypes {
		if strings.HasPrefix(mimeType, t) {
			return true
		}
	}
	return false
}

func Explore(
	ctx context.Context,
	analyze func(content detect.Content),
	analyzeFilename func(filename, contentKey string, location []string),
	memory detect.MemoryStore,
	name string,
	baseURL string,
	apiToken string,
	projectKeys []string,
) error {
	client, err := NewAPIClient(baseURL, apiToken)
	if err != nil {
		return fmt.Errorf("jira-dc: %w", err)
	}

	for _, projectKey := range projectKeys {
		issues, err := client.searchIssues(ctx, projectKey)
		if err != nil {
			slog.Error("jira-dc: failed to search issues", "project", projectKey, "error", err)
			continue
		}

		slog.Info("jira-dc issues", "target", name, "project", projectKey, "count", len(issues))

		for _, issue := range issues {
			// Analyze issue description
			descMemoryKey := fmt.Sprintf("jira-dc/%s/%s/%s/description/%s", name, projectKey, issue.Key, issue.Fields.Updated)
			if !memory.Has(descMemoryKey) && issue.Fields.Description != "" {
				content := detect.Content{
					Key:  fmt.Sprintf("%s:%s:description", projectKey, issue.Key),
					Data: []byte(issue.Fields.Description),
					Location: []string{
						"jira-dc",
						projectKey,
						issue.Key,
						issue.Fields.Summary,
						"description",
					},
				}
				analyze(content)
				memory.Set(descMemoryKey)
				slog.Debug("analyzed issue description", "project", projectKey, "issue", issue.Key)
			}

			// Analyze issue comments
			comments, err := client.getComments(ctx, issue.Key)
			if err != nil {
				slog.Error("jira-dc: failed to get comments", "issue", issue.Key, "error", err)
				continue
			}

			for _, comment := range comments {
				commentMemoryKey := fmt.Sprintf("jira-dc/%s/%s/%s/comment/%s/%s", name, projectKey, issue.Key, comment.ID, comment.Updated)
				if memory.Has(commentMemoryKey) {
					continue
				}

				if comment.Body == "" {
					continue
				}

				content := detect.Content{
					Key:  fmt.Sprintf("%s:%s:comment:%s", projectKey, issue.Key, comment.ID),
					Data: []byte(comment.Body),
					Location: []string{
						"jira-dc",
						projectKey,
						issue.Key,
						issue.Fields.Summary,
						"comment",
						comment.ID,
					},
				}
				analyze(content)
				memory.Set(commentMemoryKey)
				slog.Debug("analyzed issue comment", "project", projectKey, "issue", issue.Key, "comment", comment.ID)
			}

			// Analyze issue attachments
			for _, attachment := range issue.Fields.Attachment {
				attachmentMemoryKey := fmt.Sprintf("jira-dc/%s/%s/%s/attachment/%s", name, projectKey, issue.Key, attachment.ID)
				if memory.Has(attachmentMemoryKey) {
					continue
				}

				location := []string{
					"jira-dc",
					projectKey,
					issue.Key,
					issue.Fields.Summary,
					"attachment",
					attachment.Filename,
				}
				contentKey := fmt.Sprintf("%s:%s:attachment:%s", projectKey, issue.Key, attachment.ID)

				// Check filename against rules
				analyzeFilename(attachment.Filename, contentKey, location)

				// Check if it's an archive - extract and analyze filenames within
				if tools.IsArchive(attachment.MimeType, attachment.Filename) {
					data, err := client.downloadAttachment(ctx, attachment.Content)
					if err != nil {
						slog.Error("jira-dc: failed to download archive", "issue", issue.Key, "filename", attachment.Filename, "error", err)
					} else {
						archivedFiles := tools.ExtractArchiveFilenames(data, attachment.MimeType, attachment.Filename)
						slog.Debug("jira-dc: extracted filenames from archive", "issue", issue.Key, "archive", attachment.Filename, "count", len(archivedFiles))

						for _, archivedFile := range archivedFiles {
							archivedLocation := []string{
								"jira-dc",
								projectKey,
								issue.Key,
								issue.Fields.Summary,
								"attachment",
								attachment.Filename,
								archivedFile,
							}
							analyzeFilename(archivedFile, contentKey, archivedLocation)
						}
					}
				} else if isTextMimeType(attachment.MimeType) {
					// Analyze content for text files
					data, err := client.downloadAttachment(ctx, attachment.Content)
					if err != nil {
						slog.Error("jira-dc: failed to download attachment", "issue", issue.Key, "filename", attachment.Filename, "error", err)
					} else {
						content := detect.Content{
							Key:      contentKey,
							Data:     data,
							Location: location,
						}
						analyze(content)
						slog.Debug("analyzed issue attachment content", "project", projectKey, "issue", issue.Key, "filename", attachment.Filename)
					}
				}

				memory.Set(attachmentMemoryKey)
			}
		}
	}

	return nil
}
