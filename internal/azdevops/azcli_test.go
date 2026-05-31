// internal/azdevops/azcli_test.go
package azdevops

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// makeAzCLIProvider builds a provider with an injected commander for tests.
func makeAzCLIProvider(output string, cmdErr error) *AzCLITokenProvider {
	return &AzCLITokenProvider{
		run: func(name string, args ...string) ([]byte, error) {
			if cmdErr != nil {
				return nil, cmdErr
			}
			return []byte(output), nil
		},
	}
}

func azCLIJSON(token string, expiresAt time.Time) string {
	return fmt.Sprintf(`{"accessToken":%q,"expiresOn":%q,"tokenType":"Bearer"}`,
		token, expiresAt.Format("2006-01-02 15:04:05.000000"))
}

func TestAzCLITokenProvider_AuthorizationHeader_Success(t *testing.T) {
	provider := makeAzCLIProvider(azCLIJSON("eyJtest", time.Now().Add(1*time.Hour)), nil)

	header, err := provider.AuthorizationHeader()
	if err != nil {
		t.Fatalf("AuthorizationHeader() failed: %v", err)
	}

	if !strings.HasPrefix(header, "Bearer ") {
		t.Errorf("expected header to start with 'Bearer ', got %q", header)
	}
	if !strings.Contains(header, "eyJtest") {
		t.Errorf("expected header to contain token, got %q", header)
	}
}

func TestAzCLITokenProvider_CachesToken(t *testing.T) {
	callCount := 0
	provider := &AzCLITokenProvider{
		run: func(name string, args ...string) ([]byte, error) {
			callCount++
			return []byte(azCLIJSON("tok", time.Now().Add(1*time.Hour))), nil
		},
	}

	_, _ = provider.AuthorizationHeader()
	_, _ = provider.AuthorizationHeader()

	if callCount != 1 {
		t.Errorf("expected 1 az CLI invocation (cached), got %d", callCount)
	}
}

func TestAzCLITokenProvider_RefreshesNearExpiry(t *testing.T) {
	callCount := 0
	// Token expires in 3 minutes — inside the 5-minute refresh buffer
	provider := &AzCLITokenProvider{
		token:     "oldtoken",
		expiresAt: time.Now().Add(3 * time.Minute),
		run: func(name string, args ...string) ([]byte, error) {
			callCount++
			return []byte(azCLIJSON("newtoken", time.Now().Add(1*time.Hour))), nil
		},
	}

	header, err := provider.AuthorizationHeader()
	if err != nil {
		t.Fatalf("AuthorizationHeader() failed: %v", err)
	}

	if !strings.Contains(header, "newtoken") {
		t.Errorf("expected refreshed token, got %q", header)
	}
	if callCount != 1 {
		t.Errorf("expected 1 refresh call, got %d", callCount)
	}
}

func TestAzCLITokenProvider_AzNotAvailable(t *testing.T) {
	provider := makeAzCLIProvider("", fmt.Errorf("exec: az not found in PATH"))

	_, err := provider.AuthorizationHeader()
	if err == nil {
		t.Fatal("expected error when az CLI not available")
	}
	if !strings.Contains(err.Error(), "az login") {
		t.Errorf("expected error to mention 'az login', got %q", err.Error())
	}
}

func TestAzCLITokenProvider_EmptyAccessToken(t *testing.T) {
	provider := makeAzCLIProvider(
		fmt.Sprintf(`{"accessToken":"","expiresOn":%q,"tokenType":"Bearer"}`,
			time.Now().Add(1*time.Hour).Format("2006-01-02 15:04:05.000000")),
		nil,
	)

	_, err := provider.AuthorizationHeader()
	if err == nil {
		t.Fatal("expected error for empty access token")
	}
}

func TestAzCLITokenProvider_ImplementsInterface(t *testing.T) {
	var _ TokenProvider = NewAzCLITokenProvider() // compile-time check
}

func TestParseExpiresOn_SpaceFormat(t *testing.T) {
	got, err := parseExpiresOn("2024-06-15 14:30:00.000000")
	if err != nil {
		t.Fatalf("parseExpiresOn() failed: %v", err)
	}
	if got.Year() != 2024 || got.Month() != 6 || got.Day() != 15 {
		t.Errorf("unexpected date: %v", got)
	}
}

func TestParseExpiresOn_RFC3339Format(t *testing.T) {
	got, err := parseExpiresOn("2024-06-15T14:30:00Z")
	if err != nil {
		t.Fatalf("parseExpiresOn() failed: %v", err)
	}
	if got.Year() != 2024 || got.Month() != 6 || got.Day() != 15 {
		t.Errorf("unexpected date: %v", got)
	}
}

func TestParseExpiresOn_RFC3339WithOffset(t *testing.T) {
	got, err := parseExpiresOn("2024-06-15T14:30:00+02:00")
	if err != nil {
		t.Fatalf("parseExpiresOn() failed: %v", err)
	}
	if got.Year() != 2024 {
		t.Errorf("unexpected year: %v", got)
	}
}

func TestParseExpiresOn_UnknownFormat(t *testing.T) {
	_, err := parseExpiresOn("not-a-date")
	if err == nil {
		t.Error("expected error for unknown format")
	}
}
