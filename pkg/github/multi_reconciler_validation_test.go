package github

import (
	"strings"
	"testing"
	"time"
)

func TestMultiReconciler_ValidateAll(t *testing.T) {
	tests := []struct {
		name           string
		config         *MultiRepositoryConfig
		repoFilter     []string
		expectedValid  []string
		expectedErrors map[string]bool // map of repo names to whether they should have errors
		wantErr        bool
	}{
		{
			name: "valid multi-repository configuration",
			config: &MultiRepositoryConfig{
				Defaults: &RepositoryDefaults{
					Private: boolPtr(true),
					Topics:  []string{"production"},
				},
				Repositories: []RepositoryConfig{
					{
						Name:        "valid-repo-1",
						Description: "A valid repository",
						Topics:      []string{"golang"},
					},
					{
						Name:        "valid-repo-2",
						Description: "Another valid repository",
						Topics:      []string{"typescript"},
					},
				},
			},
			expectedValid:  []string{"valid-repo-1", "valid-repo-2"},
			expectedErrors: map[string]bool{},
			wantErr:        false,
		},
		{
			name: "mixed valid and invalid repositories",
			config: &MultiRepositoryConfig{
				Repositories: []RepositoryConfig{
					{
						Name:        "valid-repo",
						Description: "A valid repository",
					},
					{
						Name:        "", // Invalid: empty name
						Description: "Invalid repository with empty name",
					},
					{
						Name:        "invalid-repo-with-very-long-description-that-exceeds-the-maximum-allowed-length-of-350-characters-which-should-cause-validation-to-fail-because-github-has-strict-limits-on-repository-descriptions-and-we-need-to-enforce-these-limits-to-prevent-api-errors-when-creating-or-updating-repositories-on-github-platform-which-would-result-in-failed-operations",
						Description: "This description is way too long and should fail validation because it exceeds the 350 character limit that GitHub enforces for repository descriptions",
					},
				},
			},
			expectedValid: []string{"valid-repo"},
			expectedErrors: map[string]bool{
				"": true, // Empty name repo should have errors
				"invalid-repo-with-very-long-description-that-exceeds-the-maximum-allowed-length-of-350-characters-which-should-cause-validation-to-fail-because-github-has-strict-limits-on-repository-descriptions-and-we-need-to-enforce-these-limits-to-prevent-api-errors-when-creating-or-updating-repositories-on-github-platform-which-would-result-in-failed-operations": true,
			},
			wantErr: false,
		},
		{
			name: "repository with invalid topics",
			config: &MultiRepositoryConfig{
				Repositories: []RepositoryConfig{
					{
						Name:        "repo-with-invalid-topics",
						Description: "Repository with invalid topics",
						Topics:      []string{"valid-topic", "INVALID-TOPIC-WITH-UPPERCASE", "", "topic-with-very-long-name-that-exceeds-fifty-characters"},
					},
				},
			},
			expectedValid: []string{},
			expectedErrors: map[string]bool{
				"repo-with-invalid-topics": true,
			},
			wantErr: false,
		},
		{
			name: "repository with invalid collaborators",
			config: &MultiRepositoryConfig{
				Repositories: []RepositoryConfig{
					{
						Name:        "repo-with-invalid-collaborators",
						Description: "Repository with invalid collaborators",
						Collaborators: []Collaborator{
							{Username: "valid-user", Permission: "read"},
							{Username: "", Permission: "write"},                               // Invalid: empty username
							{Username: "user-with-invalid-permission", Permission: "invalid"}, // Invalid permission
						},
					},
				},
			},
			expectedValid: []string{},
			expectedErrors: map[string]bool{
				"repo-with-invalid-collaborators": true,
			},
			wantErr: false,
		},
		{
			name: "repository with invalid teams",
			config: &MultiRepositoryConfig{
				Repositories: []RepositoryConfig{
					{
						Name:        "repo-with-invalid-teams",
						Description: "Repository with invalid teams",
						Teams: []TeamAccess{
							{TeamSlug: "valid-team", Permission: "read"},
							{TeamSlug: "", Permission: "write"},                                 // Invalid: empty team slug
							{TeamSlug: "team-with-invalid-permission", Permission: "superuser"}, // Invalid permission
						},
					},
				},
			},
			expectedValid: []string{},
			expectedErrors: map[string]bool{
				"repo-with-invalid-teams": true,
			},
			wantErr: false,
		},
		{
			name: "repository with invalid webhooks",
			config: &MultiRepositoryConfig{
				Repositories: []RepositoryConfig{
					{
						Name:        "repo-with-invalid-webhooks",
						Description: "Repository with invalid webhooks",
						Webhooks: []Webhook{
							{URL: "https://example.com/webhook", Events: []string{"push"}},
							{URL: "", Events: []string{"push"}},                                      // Invalid: empty URL
							{URL: "https://example.com/webhook2", Events: []string{}},                // Invalid: no events
							{URL: "https://example.com/webhook3", Events: []string{"invalid-event"}}, // Invalid event
						},
					},
				},
			},
			expectedValid: []string{},
			expectedErrors: map[string]bool{
				"repo-with-invalid-webhooks": true,
			},
			wantErr: false,
		},
		{
			name: "filtered repositories",
			config: &MultiRepositoryConfig{
				Repositories: []RepositoryConfig{
					{
						Name:        "repo-1",
						Description: "First repository",
					},
					{
						Name:        "repo-2",
						Description: "Second repository",
					},
					{
						Name:        "", // Invalid repo that should be filtered out
						Description: "Invalid repository",
					},
				},
			},
			repoFilter:     []string{"repo-1", "repo-2"},
			expectedValid:  []string{"repo-1", "repo-2"},
			expectedErrors: map[string]bool{},
			wantErr:        false,
		},
		{
			name:    "nil configuration",
			config:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock client for testing
			client := newMockAPIClient()
			reconciler := NewMultiReconciler(client, "test-owner")

			result, err := reconciler.ValidateAll(tt.config, tt.repoFilter)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateAll() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateAll() unexpected error = %v", err)
				return
			}

			if result == nil {
				t.Errorf("ValidateAll() returned nil result")
				return
			}

			// Check valid repositories
			if len(result.Valid) != len(tt.expectedValid) {
				t.Errorf("ValidateAll() valid count = %d, want %d", len(result.Valid), len(tt.expectedValid))
			}

			for _, expectedRepo := range tt.expectedValid {
				found := false
				for _, validRepo := range result.Valid {
					if validRepo == expectedRepo {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ValidateAll() expected repository '%s' to be valid but it wasn't", expectedRepo)
				}
			}

			// Check invalid repositories
			for repoName, shouldHaveErrors := range tt.expectedErrors {
				if shouldHaveErrors {
					if _, hasError := result.Invalid[repoName]; !hasError {
						t.Errorf("ValidateAll() expected repository '%s' to have errors but it didn't", repoName)
					}
				} else {
					if _, hasError := result.Invalid[repoName]; hasError {
						t.Errorf("ValidateAll() expected repository '%s' to be valid but it had errors: %v", repoName, result.Invalid[repoName])
					}
				}
			}

			// Check summary statistics
			expectedTotalRepos := len(tt.config.Repositories)
			if tt.repoFilter != nil {
				expectedTotalRepos = len(tt.repoFilter)
			}

			if result.Summary.TotalRepositories != expectedTotalRepos {
				t.Errorf("ValidateAll() summary total repositories = %d, want %d", result.Summary.TotalRepositories, expectedTotalRepos)
			}

			if result.Summary.ValidCount != len(result.Valid) {
				t.Errorf("ValidateAll() summary valid count = %d, want %d", result.Summary.ValidCount, len(result.Valid))
			}

			if result.Summary.InvalidCount != len(result.Invalid) {
				t.Errorf("ValidateAll() summary invalid count = %d, want %d", result.Summary.InvalidCount, len(result.Invalid))
			}

			// Check that details are provided for all processed repositories
			processedRepos := len(result.Valid) + len(result.Invalid)
			if len(result.Details) != processedRepos {
				t.Errorf("ValidateAll() details count = %d, want %d", len(result.Details), processedRepos)
			}

			// Check that all details have timestamps
			for repoName, details := range result.Details {
				if details.ValidatedAt == "" {
					t.Errorf("ValidateAll() repository '%s' details missing ValidatedAt timestamp", repoName)
				}

				// Verify timestamp is valid RFC3339 format
				if _, err := time.Parse(time.RFC3339, details.ValidatedAt); err != nil {
					t.Errorf("ValidateAll() repository '%s' has invalid ValidatedAt timestamp: %v", repoName, err)
				}

				if details.RepositoryName != repoName {
					t.Errorf("ValidateAll() repository '%s' details has wrong repository name: %s", repoName, details.RepositoryName)
				}
			}
		})
	}
}

func TestMultiReconciler_ValidateAll_DuplicateRepositoryNames(t *testing.T) {
	config := &MultiRepositoryConfig{
		Repositories: []RepositoryConfig{
			{
				Name:        "duplicate-repo",
				Description: "First repository",
			},
			{
				Name:        "duplicate-repo", // Duplicate name
				Description: "Second repository with same name",
			},
			{
				Name:        "unique-repo",
				Description: "Unique repository",
			},
		},
	}

	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	// This should fail at the configuration level due to duplicate names
	_, err := reconciler.ValidateAll(config, nil)

	if err == nil {
		t.Errorf("ValidateAll() expected error for duplicate repository names but got none")
	}

	if !strings.Contains(err.Error(), "duplicate repository name") {
		t.Errorf("ValidateAll() error should mention duplicate repository names, got: %v", err)
	}
}

func TestMultiReconciler_ValidateAll_InvalidRepositoryFilterEnhanced(t *testing.T) {
	config := &MultiRepositoryConfig{
		Repositories: []RepositoryConfig{
			{
				Name:        "existing-repo",
				Description: "Repository that exists",
			},
		},
	}

	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	// Test with non-existent repository in filter
	_, err := reconciler.ValidateAll(config, []string{"non-existent-repo"})

	if err == nil {
		t.Errorf("ValidateAll() expected error for invalid repository filter but got none")
	}

	if !strings.Contains(err.Error(), "invalid repository filter") {
		t.Errorf("ValidateAll() error should mention invalid repository filter, got: %v", err)
	}
}

func TestMultiReconciler_ValidateAll_ValidationWarnings(t *testing.T) {
	config := &MultiRepositoryConfig{
		Repositories: []RepositoryConfig{
			{
				Name:        "internal-service", // Should trigger warning for public repo with sensitive name
				Description: "Internal service repository",
				Private:     false, // Public repo with sensitive name
			},
			{
				Name:        "service-without-protection",
				Description: "Service without branch protection",
				// No branch protection rules - should trigger warning
			},
			{
				Name:        "service-without-access",
				Description: "Service without access control",
				// No collaborators or teams - should trigger warning
			},
			{
				Name:        "service-with-insecure-webhook",
				Description: "Service with insecure webhook",
				Webhooks: []Webhook{
					{
						URL:    "https://example.com/webhook",
						Events: []string{"push"},
						// No secret - should trigger warning
					},
				},
			},
		},
	}

	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	result, err := reconciler.ValidateAll(config, nil)

	if err != nil {
		t.Errorf("ValidateAll() unexpected error = %v", err)
		return
	}

	// All repositories should be valid but have warnings
	if len(result.Valid) != 4 {
		t.Errorf("ValidateAll() expected 4 valid repositories, got %d", len(result.Valid))
	}

	if result.Summary.WarningCount == 0 {
		t.Errorf("ValidateAll() expected warnings but got none")
	}

	// Check specific warnings
	expectedWarnings := map[string][]string{
		"internal-service":              {"sensitive_name_public_repo"},
		"service-without-protection":    {"missing_main_branch_protection"},
		"service-without-access":        {"no_access_control"},
		"service-with-insecure-webhook": {"webhook_no_secret"},
	}

	for repoName, expectedCodes := range expectedWarnings {
		details, exists := result.Details[repoName]
		if !exists {
			t.Errorf("ValidateAll() missing details for repository '%s'", repoName)
			continue
		}

		if len(details.Warnings) == 0 {
			t.Errorf("ValidateAll() expected warnings for repository '%s' but got none", repoName)
			continue
		}

		for _, expectedCode := range expectedCodes {
			found := false
			for _, warning := range details.Warnings {
				if warning.Code == expectedCode {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ValidateAll() expected warning code '%s' for repository '%s' but didn't find it", expectedCode, repoName)
			}
		}
	}
}
