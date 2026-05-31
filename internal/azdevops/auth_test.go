// internal/azdevops/auth_test.go
package azdevops

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestPATTokenProvider_AuthorizationHeader(t *testing.T) {
	provider, err := NewPATTokenProvider("my-secret-pat")
	if err != nil {
		t.Fatalf("NewPATTokenProvider() failed: %v", err)
	}

	header, err := provider.AuthorizationHeader()
	if err != nil {
		t.Fatalf("AuthorizationHeader() failed: %v", err)
	}

	if !strings.HasPrefix(header, "Basic ") {
		t.Errorf("expected header to start with 'Basic ', got %q", header)
	}

	encoded := strings.TrimPrefix(header, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}

	if string(decoded) != ":my-secret-pat" {
		t.Errorf("decoded = %q, want %q", string(decoded), ":my-secret-pat")
	}
}

func TestNewPATTokenProvider_EmptyPAT(t *testing.T) {
	_, err := NewPATTokenProvider("")
	if err == nil {
		t.Error("expected error for empty PAT, got nil")
	}
}

func TestPATTokenProvider_ImplementsInterface(t *testing.T) {
	provider, _ := NewPATTokenProvider("tok")
	var _ TokenProvider = provider // compile-time check
}
