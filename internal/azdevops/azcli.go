// internal/azdevops/azcli.go
package azdevops

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

const (
	// azDevOpsResourceID is the fixed Azure AD app ID for Azure DevOps.
	azDevOpsResourceID = "499b84ac-1321-427f-aa17-267ca6975798"
	tokenRefreshBuffer = 5 * time.Minute
)

type azCLIResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresOn   string `json:"expiresOn"`
	TokenType   string `json:"tokenType"`
}

// commander is injectable for testing without real az CLI calls.
type commander func(name string, args ...string) ([]byte, error)

// AzCLITokenProvider fetches Bearer tokens via `az account get-access-token`,
// caching the result and refreshing proactively before expiry.
type AzCLITokenProvider struct {
	mu        sync.Mutex
	token     string
	expiresAt time.Time
	run       commander
}

// NewAzCLITokenProvider creates a provider that calls the real az CLI.
func NewAzCLITokenProvider() *AzCLITokenProvider {
	return &AzCLITokenProvider{
		run: func(name string, args ...string) ([]byte, error) {
			return exec.Command(name, args...).Output()
		},
	}
}

// AuthorizationHeader returns a Bearer token, refreshing via az CLI when near expiry.
func (a *AzCLITokenProvider) AuthorizationHeader() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.token != "" && time.Now().Before(a.expiresAt.Add(-tokenRefreshBuffer)) {
		return "Bearer " + a.token, nil
	}

	token, expiresAt, err := a.fetchToken()
	if err != nil {
		return "", err
	}

	a.token = token
	a.expiresAt = expiresAt
	return "Bearer " + a.token, nil
}

func (a *AzCLITokenProvider) fetchToken() (string, time.Time, error) {
	out, err := a.run("az", "account", "get-access-token",
		"--resource", azDevOpsResourceID,
		"--output", "json")
	if err != nil {
		return "", time.Time{}, fmt.Errorf(
			"az CLI not available or not logged in: run 'az login' first: %w", err)
	}

	var resp azCLIResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to parse az CLI response: %w", err)
	}

	if resp.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("az CLI returned empty access token")
	}

	expiresAt, err := parseExpiresOn(resp.ExpiresOn)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to parse token expiry %q: %w", resp.ExpiresOn, err)
	}

	return resp.AccessToken, expiresAt, nil
}

// parseExpiresOn handles both the space-separated format ("2024-01-01 12:00:00.000000")
// used by older az CLI versions and ISO 8601 used by newer ones.
// The space-separated format uses local wall-clock time and must be parsed with time.Local.
func parseExpiresOn(s string) (time.Time, error) {
	// Space-separated format from older az CLI versions uses local wall-clock time.
	if t, err := time.ParseInLocation("2006-01-02 15:04:05.999999", s, time.Local); err == nil {
		return t, nil
	}
	// ISO 8601 / RFC3339 formats include timezone offset — parse directly.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognized expiresOn format: %q", s)
}
