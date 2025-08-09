package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"synacklab/pkg/config"
)

func TestNewAuthManager(t *testing.T) {
	am := NewAuthManager()
	assert.NotNil(t, am)
	assert.Nil(t, am.client)
	assert.Empty(t, am.token)
}

func TestAuthManager_GetToken(t *testing.T) {
	tests := []struct {
		name        string
		envToken    string
		config      *config.Config
		expected    string
		expectError bool
	}{
		{
			name:     "token from environment variable",
			envToken: "env_token_123",
			config:   nil,
			expected: "env_token_123",
		},
		{
			name:     "token from config file",
			envToken: "",
			config: &config.Config{
				GitHub: config.GitHubConfig{
					Token: "config_token_456",
				},
			},
			expected: "config_token_456",
		},
		{
			name:     "environment variable takes precedence",
			envToken: "env_token_123",
			config: &config.Config{
				GitHub: config.GitHubConfig{
					Token: "config_token_456",
				},
			},
			expected: "env_token_123",
		},
		{
			name:        "no token available",
			envToken:    "",
			config:      &config.Config{},
			expectError: true,
		},
		{
			name:        "nil config and no env token",
			envToken:    "",
			config:      nil,
			expectError: true,
		},
		{
			name:     "token with whitespace is trimmed",
			envToken: "  token_with_spaces  ",
			config:   nil,
			expected: "token_with_spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envToken != "" {
				t.Setenv("GITHUB_TOKEN", tt.envToken)
			}

			am := NewAuthManager()
			token, err := am.GetToken(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "no GitHub token found")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, token)
			}
		})
	}
}

func TestAuthManager_Authenticate(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		expectError bool
	}{
		{
			name:  "valid token",
			token: "valid_token_123",
		},
		{
			name:        "empty token",
			token:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			am := NewAuthManager()
			err := am.Authenticate(tt.token)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "GitHub token cannot be empty")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, am.client)
				assert.Equal(t, tt.token, am.token)
			}
		})
	}
}

func TestAuthManager_ValidateToken(t *testing.T) {
	tests := []struct {
		name           string
		setupServer    func() *httptest.Server
		expectError    bool
		expectedUser   string
		expectedScopes []string
	}{
		{
			name: "valid token with required scopes",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/user" {
						w.Header().Set("X-OAuth-Scopes", "repo,user")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{"login": "testuser"}`))
					}
				}))
			},
			expectedUser:   "testuser",
			expectedScopes: []string{"repo", "user"},
		},
		{
			name: "valid token with missing required scopes",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/user" {
						w.Header().Set("X-OAuth-Scopes", "user")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{"login": "testuser"}`))
					}
				}))
			},
			expectError:    true,
			expectedUser:   "testuser",
			expectedScopes: []string{"user"},
		},
		{
			name: "invalid token",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/user" {
						w.WriteHeader(http.StatusUnauthorized)
						w.Write([]byte(`{"message": "Bad credentials"}`))
					}
				}))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			am := NewAuthManager()
			err := am.Authenticate("test_token")
			require.NoError(t, err)

			// Override the client's base URL to use our test server
			baseURL, _ := url.Parse(server.URL + "/")
			am.client.BaseURL = baseURL

			ctx := context.Background()
			tokenInfo, err := am.ValidateToken(ctx)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedUser != "" {
					// Even with permission errors, we should get user info
					assert.NotNil(t, tokenInfo)
					assert.Equal(t, tt.expectedUser, tokenInfo.User)
					assert.Equal(t, tt.expectedScopes, tokenInfo.Scopes)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tokenInfo)
				assert.Equal(t, tt.expectedUser, tokenInfo.User)
				assert.Equal(t, tt.expectedScopes, tokenInfo.Scopes)
			}
		})
	}
}

func TestAuthManager_ValidateToken_NotAuthenticated(t *testing.T) {
	am := NewAuthManager()
	ctx := context.Background()

	tokenInfo, err := am.ValidateToken(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not authenticated")
	assert.Nil(t, tokenInfo)
}

func TestAuthManager_validatePermissions(t *testing.T) {
	tests := []struct {
		name        string
		scopes      []string
		expectError bool
	}{
		{
			name:   "has required repo scope",
			scopes: []string{"repo", "user"},
		},
		{
			name:   "has only repo scope",
			scopes: []string{"repo"},
		},
		{
			name:        "missing repo scope",
			scopes:      []string{"user", "gist"},
			expectError: true,
		},
		{
			name:        "no scopes",
			scopes:      []string{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			am := NewAuthManager()
			err := am.validatePermissions(tt.scopes)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "missing required permissions")
				assert.Contains(t, err.Error(), "repo")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuthManager_GetClient(t *testing.T) {
	am := NewAuthManager()

	// Before authentication
	assert.Nil(t, am.GetClient())

	// After authentication
	err := am.Authenticate("test_token")
	require.NoError(t, err)
	assert.NotNil(t, am.GetClient())
}

func TestAuthManager_AuthenticateFromConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			w.Header().Set("X-OAuth-Scopes", "repo,user")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"login": "testuser"}`))
		}
	}))
	defer server.Close()

	tests := []struct {
		name         string
		envToken     string
		config       *config.Config
		expectError  bool
		expectedUser string
	}{
		{
			name:         "successful authentication from env",
			envToken:     "valid_token",
			config:       &config.Config{},
			expectedUser: "testuser",
		},
		{
			name:     "successful authentication from config",
			envToken: "",
			config: &config.Config{
				GitHub: config.GitHubConfig{
					Token: "valid_token",
				},
			},
			expectedUser: "testuser",
		},
		{
			name:        "no token available",
			envToken:    "",
			config:      &config.Config{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envToken != "" {
				t.Setenv("GITHUB_TOKEN", tt.envToken)
			}

			am := NewAuthManager()
			ctx := context.Background()

			tokenInfo, err := am.AuthenticateFromConfig(ctx, tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, tokenInfo)
			} else {
				// Override the client's base URL to use our test server after authentication
				baseURL, _ := url.Parse(server.URL + "/")
				am.client.BaseURL = baseURL

				// Re-validate with the test server
				tokenInfo, err = am.ValidateToken(ctx)
				assert.NoError(t, err)
				assert.NotNil(t, tokenInfo)
				assert.Equal(t, tt.expectedUser, tokenInfo.User)
			}
		})
	}
}

func TestGetAuthInstructions(t *testing.T) {
	instructions := GetAuthInstructions()

	assert.NotEmpty(t, instructions)
	assert.Contains(t, instructions, "GITHUB_TOKEN")
	assert.Contains(t, instructions, "config.yaml")
	assert.Contains(t, instructions, "repo")
	assert.Contains(t, instructions, "Personal access tokens")
}

func TestTokenInfo(t *testing.T) {
	tokenInfo := &TokenInfo{
		User:   "testuser",
		Scopes: []string{"repo", "user"},
	}

	assert.Equal(t, "testuser", tokenInfo.User)
	assert.Equal(t, []string{"repo", "user"}, tokenInfo.Scopes)
}
