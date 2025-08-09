package github

import (
	"testing"
)

func TestValidator_ValidateConfig(t *testing.T) {
	// Note: These tests would require mocking the GitHub API client
	// For now, we'll test the basic structure and error handling

	tests := []struct {
		name    string
		config  *RepositoryConfig
		owner   string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config basic validation",
			config: &RepositoryConfig{
				Name:        "test-repo",
				Description: "A test repository",
				Private:     true,
			},
			owner:   "testorg",
			wantErr: false, // Would fail with real API, but basic validation should pass
		},
		{
			name: "invalid config fails basic validation",
			config: &RepositoryConfig{
				Name: "", // Invalid name
			},
			owner:   "testorg",
			wantErr: true,
			errMsg:  "repository name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a validator with a dummy token
			// In real usage, this would need a valid GitHub token
			v := NewValidator("dummy-token")

			err := tt.config.Validate() // Test basic validation first
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateConfig() expected error but got none")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateConfig() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateConfig() unexpected error = %v", err)
				}
			}

			// Note: Full API validation tests would require mocking the GitHub client
			// or integration tests with a real GitHub token and test organization
			_ = v // Prevent unused variable error
		})
	}
}

func TestNewValidator(t *testing.T) {
	validator := NewValidator("test-token")
	if validator == nil {
		t.Errorf("NewValidator() returned nil")
		return
	}
	if validator.client == nil {
		t.Errorf("NewValidator() client is nil")
	}
	if validator.ctx == nil {
		t.Errorf("NewValidator() context is nil")
	}
}

func TestValidateGitHubUsernameIntegration(t *testing.T) {
	// Test the username validation function with various edge cases
	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{"valid short username", "a", false},
		{"valid long username", "abcdefghijklmnopqrstuvwxyz1234567890123", false}, // 39 chars
		{"too long username", "abcdefghijklmnopqrstuvwxyz12345678901234", true},   // 40 chars
		{"username with valid hyphen", "test-user", false},
		{"username starting with hyphen", "-test", true},
		{"username ending with hyphen", "test-", true},
		{"username with consecutive hyphens", "test--user", true},
		{"username with underscore", "test_user", true},
		{"username with space", "test user", true},
		{"empty username", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitHubUsername(tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGitHubUsername() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateGitHubTeamSlugIntegration(t *testing.T) {
	// Test the team slug validation function with various edge cases
	tests := []struct {
		name     string
		teamSlug string
		wantErr  bool
	}{
		{"valid short team slug", "a", false},
		{"valid team slug with hyphen", "dev-team", false},
		{"valid team slug with underscore", "dev_team", false},
		{"valid team slug with numbers", "team123", false},
		{"valid mixed team slug", "dev-team_123", false},
		{"team slug with uppercase", "Dev-Team", true},
		{"team slug starting with hyphen", "-dev", true},
		{"team slug starting with underscore", "_dev", true},
		{"team slug with space", "dev team", true},
		{"team slug with special chars", "dev@team", true},
		{"empty team slug", "", true},
		{"too long team slug", "a" + string(make([]byte, 100)), true}, // 101 chars
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitHubTeamSlug(tt.teamSlug)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGitHubTeamSlug() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
