package github

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"

	"synacklab/pkg/config"
)

// AuthManager handles GitHub authentication
type AuthManager struct {
	client *github.Client
	token  string
}

// NewAuthManager creates a new authentication manager
func NewAuthManager() *AuthManager {
	return &AuthManager{}
}

// GetToken retrieves the GitHub token from environment variable or config file
func (am *AuthManager) GetToken(cfg *config.Config) (string, error) {
	// First, check environment variable
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return strings.TrimSpace(token), nil
	}

	// Then check config file
	if cfg != nil && cfg.GitHub.Token != "" {
		return strings.TrimSpace(cfg.GitHub.Token), nil
	}

	return "", fmt.Errorf("no GitHub token found: set GITHUB_TOKEN environment variable or configure token in ~/.synacklab/config.yaml")
}

// Authenticate sets up the GitHub client with the provided token
func (am *AuthManager) Authenticate(token string) error {
	if token == "" {
		return fmt.Errorf("GitHub token cannot be empty")
	}

	// Create OAuth2 token source
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	// Create GitHub client
	am.client = github.NewClient(tc)
	am.token = token

	return nil
}

// ValidateToken validates the GitHub token and checks permissions
func (am *AuthManager) ValidateToken(ctx context.Context) (*TokenInfo, error) {
	if am.client == nil {
		return nil, fmt.Errorf("not authenticated: call Authenticate() first")
	}

	// Get the authenticated user to validate the token
	user, _, err := am.client.Users.Get(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to validate GitHub token: %w", err)
	}

	// Get token scopes from the response headers
	// Note: This requires making an API call to get the scopes
	_, resp, err := am.client.Users.Get(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get token scopes: %w", err)
	}

	scopes := []string{}
	if scopeHeader := resp.Header.Get("X-OAuth-Scopes"); scopeHeader != "" {
		scopes = strings.Split(strings.ReplaceAll(scopeHeader, " ", ""), ",")
	}

	tokenInfo := &TokenInfo{
		User:   user.GetLogin(),
		Scopes: scopes,
	}

	// Validate required permissions
	if err := am.validatePermissions(tokenInfo.Scopes); err != nil {
		return tokenInfo, err
	}

	return tokenInfo, nil
}

// validatePermissions checks if the token has required permissions
func (am *AuthManager) validatePermissions(scopes []string) error {
	requiredScopes := []string{"repo"}
	scopeMap := make(map[string]bool)

	for _, scope := range scopes {
		scopeMap[scope] = true
	}

	var missingScopes []string
	for _, required := range requiredScopes {
		if !scopeMap[required] {
			missingScopes = append(missingScopes, required)
		}
	}

	if len(missingScopes) > 0 {
		return fmt.Errorf("GitHub token missing required permissions: %s. Please ensure your token has the following scopes: %s",
			strings.Join(missingScopes, ", "), strings.Join(requiredScopes, ", "))
	}

	return nil
}

// GetClient returns the authenticated GitHub client
func (am *AuthManager) GetClient() *github.Client {
	return am.client
}

// TokenInfo contains information about the authenticated token
type TokenInfo struct {
	User   string   `json:"user"`
	Scopes []string `json:"scopes"`
}

// AuthenticateFromConfig is a convenience method that handles the full authentication flow
func (am *AuthManager) AuthenticateFromConfig(ctx context.Context, cfg *config.Config) (*TokenInfo, error) {
	// Get token from environment or config
	token, err := am.GetToken(cfg)
	if err != nil {
		return nil, err
	}

	// Authenticate with the token
	if err := am.Authenticate(token); err != nil {
		return nil, err
	}

	// Validate token and permissions
	tokenInfo, err := am.ValidateToken(ctx)
	if err != nil {
		return nil, err
	}

	return tokenInfo, nil
}

// GetAuthInstructions returns instructions for setting up GitHub authentication
func GetAuthInstructions() string {
	return `GitHub authentication is required. Please set up authentication using one of the following methods:

1. Environment Variable (Recommended for CI/CD):
   export GITHUB_TOKEN="your_personal_access_token"

2. Configuration File:
   Add the following to ~/.synacklab/config.yaml:
   
   github:
     token: "your_personal_access_token"

To create a personal access token:
1. Go to GitHub Settings > Developer settings > Personal access tokens
2. Click "Generate new token (classic)"
3. Select the following scopes:
   - repo (Full control of private repositories)
4. Copy the generated token and use it with one of the methods above

Note: The token must have 'repo' scope to manage repositories.`
}
