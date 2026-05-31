package azdevops

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client represents an Azure DevOps API client
type Client struct {
	org           string
	project       string
	tokenProvider TokenProvider
	baseURL       string
	httpClient    *http.Client
	userID        string
}

// GetOrg returns the organization name
func (c *Client) GetOrg() string { return c.org }

// GetProject returns the project name
func (c *Client) GetProject() string { return c.project }

// SetBaseURL overrides the base URL for the client (used by demo mode).
func (c *Client) SetBaseURL(url string) { c.baseURL = url }

// SetUserID sets the cached user ID, bypassing the connectionData API call (used by demo mode).
func (c *Client) SetUserID(id string) { c.userID = id }

// NewClient creates a Client authenticated with a Personal Access Token.
func NewClient(org, project, pat string) (*Client, error) {
	if org == "" {
		return nil, fmt.Errorf("organization cannot be empty")
	}
	if project == "" {
		return nil, fmt.Errorf("project cannot be empty")
	}
	if pat == "" {
		return nil, fmt.Errorf("PAT cannot be empty")
	}

	provider, err := NewPATTokenProvider(pat)
	if err != nil {
		return nil, err
	}
	return newClient(org, project, provider), nil
}

// NewClientWithProvider creates a Client using the supplied TokenProvider.
func NewClientWithProvider(org, project string, provider TokenProvider) (*Client, error) {
	if org == "" {
		return nil, fmt.Errorf("organization cannot be empty")
	}
	if project == "" {
		return nil, fmt.Errorf("project cannot be empty")
	}
	if provider == nil {
		return nil, fmt.Errorf("token provider cannot be nil")
	}
	return newClient(org, project, provider), nil
}

func newClient(org, project string, provider TokenProvider) *Client {
	return &Client{
		org:           org,
		project:       project,
		tokenProvider: provider,
		baseURL:       fmt.Sprintf("https://dev.azure.com/%s/%s/_apis", org, project),
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

// setAuthHeader sets the Authorization header using the token provider.
func (c *Client) setAuthHeader(req *http.Request) error {
	header, err := c.tokenProvider.AuthorizationHeader()
	if err != nil {
		return fmt.Errorf("failed to get auth header: %w", err)
	}
	req.Header.Set("Authorization", header)
	return nil
}

// get performs a GET request to the Azure DevOps API
func (c *Client) get(path string) ([]byte, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if err := c.setAuthHeader(req); err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.formatHTTPError(resp.StatusCode, body)
	}
	return body, nil
}

// put performs a PUT request to the Azure DevOps API
func (c *Client) put(path string, body io.Reader) ([]byte, error) {
	return c.doRequest("PUT", path, body)
}

// patch performs a PATCH request to the Azure DevOps API
func (c *Client) patch(path string, body io.Reader) ([]byte, error) {
	return c.doRequest("PATCH", path, body)
}

// post performs a POST request to the Azure DevOps API
func (c *Client) post(path string, body io.Reader) ([]byte, error) {
	return c.doRequest("POST", path, body)
}

// doRequestWithContentType performs an HTTP request with a custom Content-Type header.
func (c *Client) doRequestWithContentType(method, path string, body io.Reader, contentType string) ([]byte, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if err := c.setAuthHeader(req); err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.formatHTTPError(resp.StatusCode, respBody)
	}
	return respBody, nil
}

// doRequest performs an HTTP request with the given method and JSON Content-Type.
func (c *Client) doRequest(method, path string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if err := c.setAuthHeader(req); err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.formatHTTPError(resp.StatusCode, respBody)
	}
	return respBody, nil
}

// connectionDataResponse holds the response from the connection data API
type connectionDataResponse struct {
	AuthenticatedUser struct {
		ID string `json:"id"`
	} `json:"authenticatedUser"`
}

// GetCurrentUserID returns the authenticated user's ID, fetching and caching it on first call.
func (c *Client) GetCurrentUserID() (string, error) {
	if c.userID != "" {
		return c.userID, nil
	}

	url := fmt.Sprintf("https://dev.azure.com/%s/_apis/connectionData", c.org)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	if err := c.setAuthHeader(req); err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch connection data: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", c.formatHTTPError(resp.StatusCode, body)
	}

	var data connectionDataResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to parse connection data: %w", err)
	}

	if data.AuthenticatedUser.ID == "" {
		return "", fmt.Errorf("connection data did not contain a user ID")
	}

	c.userID = data.AuthenticatedUser.ID
	return c.userID, nil
}

// formatHTTPError creates a user-friendly error message based on the HTTP status code.
func (c *Client) formatHTTPError(statusCode int, _ []byte) error {
	switch statusCode {
	case http.StatusUnauthorized:
		if _, ok := c.tokenProvider.(*AzCLITokenProvider); ok {
			return fmt.Errorf("authentication failed (HTTP 401): az-cli token was rejected. " +
				"Run 'az login' to refresh your session")
		}
		return fmt.Errorf("authentication failed (HTTP 401): your PAT may be expired or invalid. " +
			"Please generate a new PAT in Azure DevOps and update your configuration")
	case http.StatusForbidden:
		if _, ok := c.tokenProvider.(*AzCLITokenProvider); ok {
			return fmt.Errorf("access denied (HTTP 403): your Azure account does not have sufficient " +
				"permissions. Required: Code (Read), Build (Read), Work Items (Read & Write)")
		}
		return fmt.Errorf("access denied (HTTP 403): your PAT does not have sufficient permissions. " +
			"Required scopes: Code (Read), Build (Read), Work Items (Read & Write)")
	case http.StatusNotFound:
		return fmt.Errorf("resource not found (HTTP 404): the requested resource does not exist. " +
			"Please verify your organization and project names are correct in your configuration")
	case http.StatusTooManyRequests:
		return fmt.Errorf("rate limit exceeded (HTTP 429): too many requests to Azure DevOps. " +
			"Please wait a few minutes before retrying")
	case http.StatusInternalServerError:
		return fmt.Errorf("server error (HTTP 500): Azure DevOps encountered an internal error. " +
			"This is usually temporary - please try again in a few moments")
	case http.StatusServiceUnavailable:
		return fmt.Errorf("service unavailable (HTTP 503): Azure DevOps is temporarily unavailable. " +
			"This is usually a temporary issue - please try again later")
	default:
		return fmt.Errorf("HTTP request failed with status %d", statusCode)
	}
}
