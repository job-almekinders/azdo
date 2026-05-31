// internal/azdevops/auth.go
package azdevops

import (
	"encoding/base64"
	"fmt"
)

// TokenProvider returns an Authorization header value for each HTTP request.
type TokenProvider interface {
	AuthorizationHeader() (string, error)
}

// PATTokenProvider implements TokenProvider using a Personal Access Token.
type PATTokenProvider struct {
	pat string
}

// NewPATTokenProvider creates a PATTokenProvider. Returns error if pat is empty.
func NewPATTokenProvider(pat string) (*PATTokenProvider, error) {
	if pat == "" {
		return nil, fmt.Errorf("PAT cannot be empty")
	}
	return &PATTokenProvider{pat: pat}, nil
}

// AuthorizationHeader returns a Basic auth header in Azure DevOps format (:{PAT}).
func (p *PATTokenProvider) AuthorizationHeader() (string, error) {
	encoded := base64.StdEncoding.EncodeToString([]byte(":" + p.pat))
	return "Basic " + encoded, nil
}
