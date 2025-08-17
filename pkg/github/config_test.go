package github

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  RepositoryConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: RepositoryConfig{
				Name:        "test-repo",
				Description: "A test repository",
				Private:     true,
				Topics:      []string{"go", "cli", "test"},
				Features: RepositoryFeatures{
					Issues:      true,
					Wiki:        false,
					Projects:    true,
					Discussions: false,
				},
				BranchRules: []BranchProtectionRule{
					{
						Pattern:         "main",
						RequiredReviews: 2,
					},
				},
				Collaborators: []Collaborator{
					{
						Username:   "testuser",
						Permission: "write",
					},
				},
				Teams: []TeamAccess{
					{
						TeamSlug:   "dev-team",
						Permission: "admin",
					},
				},
				Webhooks: []Webhook{
					{
						URL:    "https://example.com/webhook",
						Events: []string{"push", "pull_request"},
						Active: true,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty name",
			config: RepositoryConfig{
				Name: "",
			},
			wantErr: true,
			errMsg:  "repository name is required",
		},
		{
			name: "name too long",
			config: RepositoryConfig{
				Name: "this-is-a-very-long-repository-name-that-exceeds-the-maximum-allowed-length-of-one-hundred-characters-and-should-fail-validation",
			},
			wantErr: true,
			errMsg:  "repository name must be 100 characters or less",
		},
		{
			name: "invalid name characters",
			config: RepositoryConfig{
				Name: "test repo with spaces",
			},
			wantErr: true,
			errMsg:  "repository name can only contain alphanumeric characters, periods, hyphens, and underscores",
		},
		{
			name: "name starts with period",
			config: RepositoryConfig{
				Name: ".test-repo",
			},
			wantErr: true,
			errMsg:  "repository name cannot start or end with a period",
		},
		{
			name: "description too long",
			config: RepositoryConfig{
				Name:        "test-repo",
				Description: "This is a very long description that exceeds the maximum allowed length of 350 characters. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.",
			},
			wantErr: true,
			errMsg:  "repository description must be 350 characters or less",
		},
		{
			name: "too many topics",
			config: RepositoryConfig{
				Name:   "test-repo",
				Topics: make([]string, 21), // 21 topics, max is 20
			},
			wantErr: true,
			errMsg:  "repository can have at most 20 topics",
		},
		{
			name: "empty topic",
			config: RepositoryConfig{
				Name:   "test-repo",
				Topics: []string{"valid-topic", ""},
			},
			wantErr: true,
			errMsg:  "topic 2 cannot be empty",
		},
		{
			name: "topic too long",
			config: RepositoryConfig{
				Name:   "test-repo",
				Topics: []string{"this-is-a-very-long-topic-name-that-exceeds-fifty-characters"},
			},
			wantErr: true,
			errMsg:  "topic 1 must be 50 characters or less",
		},
		{
			name: "invalid topic characters",
			config: RepositoryConfig{
				Name:   "test-repo",
				Topics: []string{"Invalid_Topic"},
			},
			wantErr: true,
			errMsg:  "topic 1 can only contain lowercase letters, numbers, and hyphens",
		},
		{
			name: "empty branch protection pattern",
			config: RepositoryConfig{
				Name: "test-repo",
				BranchRules: []BranchProtectionRule{
					{
						Pattern: "",
					},
				},
			},
			wantErr: true,
			errMsg:  "branch protection rule 1: pattern is required",
		},
		{
			name: "invalid required reviews count",
			config: RepositoryConfig{
				Name: "test-repo",
				BranchRules: []BranchProtectionRule{
					{
						Pattern:         "main",
						RequiredReviews: 10,
					},
				},
			},
			wantErr: true,
			errMsg:  "branch protection rule 1: required reviews must be between 0 and 6",
		},
		{
			name: "empty collaborator username",
			config: RepositoryConfig{
				Name: "test-repo",
				Collaborators: []Collaborator{
					{
						Username:   "",
						Permission: "read",
					},
				},
			},
			wantErr: true,
			errMsg:  "collaborator 1: username is required",
		},
		{
			name: "invalid collaborator username",
			config: RepositoryConfig{
				Name: "test-repo",
				Collaborators: []Collaborator{
					{
						Username:   "test_user",
						Permission: "read",
					},
				},
			},
			wantErr: true,
			errMsg:  "collaborator 1: username 'test_user' is invalid",
		},
		{
			name: "invalid collaborator permission",
			config: RepositoryConfig{
				Name: "test-repo",
				Collaborators: []Collaborator{
					{
						Username:   "testuser",
						Permission: "invalid",
					},
				},
			},
			wantErr: true,
			errMsg:  "collaborator 1: permission must be one of: read, write, admin",
		},
		{
			name: "empty team slug",
			config: RepositoryConfig{
				Name: "test-repo",
				Teams: []TeamAccess{
					{
						TeamSlug:   "",
						Permission: "read",
					},
				},
			},
			wantErr: true,
			errMsg:  "team 1: team slug is required",
		},
		{
			name: "invalid team slug",
			config: RepositoryConfig{
				Name: "test-repo",
				Teams: []TeamAccess{
					{
						TeamSlug:   "Dev-Team",
						Permission: "read",
					},
				},
			},
			wantErr: true,
			errMsg:  "team 1: team slug 'Dev-Team' is invalid",
		},
		{
			name: "invalid team permission",
			config: RepositoryConfig{
				Name: "test-repo",
				Teams: []TeamAccess{
					{
						TeamSlug:   "dev-team",
						Permission: "invalid",
					},
				},
			},
			wantErr: true,
			errMsg:  "team 1: permission must be one of: read, write, admin",
		},
		{
			name: "empty webhook URL",
			config: RepositoryConfig{
				Name: "test-repo",
				Webhooks: []Webhook{
					{
						URL:    "",
						Events: []string{"push"},
					},
				},
			},
			wantErr: true,
			errMsg:  "webhook 1: URL is required",
		},
		{
			name: "invalid webhook URL scheme",
			config: RepositoryConfig{
				Name: "test-repo",
				Webhooks: []Webhook{
					{
						URL:    "ftp://example.com/webhook",
						Events: []string{"push"},
					},
				},
			},
			wantErr: true,
			errMsg:  "webhook 1: URL must use http or https scheme",
		},
		{
			name: "webhook URL without host",
			config: RepositoryConfig{
				Name: "test-repo",
				Webhooks: []Webhook{
					{
						URL:    "https://",
						Events: []string{"push"},
					},
				},
			},
			wantErr: true,
			errMsg:  "webhook 1: URL must have a valid host",
		},
		{
			name: "webhook with no events",
			config: RepositoryConfig{
				Name: "test-repo",
				Webhooks: []Webhook{
					{
						URL:    "https://example.com/webhook",
						Events: []string{},
					},
				},
			},
			wantErr: true,
			errMsg:  "webhook 1: at least one event is required",
		},
		{
			name: "webhook with invalid event",
			config: RepositoryConfig{
				Name: "test-repo",
				Webhooks: []Webhook{
					{
						URL:    "https://example.com/webhook",
						Events: []string{"push", "invalid_event"},
					},
				},
			},
			wantErr: true,
			errMsg:  "webhook 1, event 2: invalid event type 'invalid_event'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("RepositoryConfig.Validate() expected error but got none")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("RepositoryConfig.Validate() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("RepositoryConfig.Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestLoadRepositoryConfig(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid YAML",
			yaml: `
name: test-repo
description: A test repository
private: true
topics:
  - go
  - cli
features:
  issues: true
  wiki: false
  projects: true
  discussions: false
branch_protection:
  - pattern: main
    required_reviews: 2
    require_up_to_date: true
collaborators:
  - username: testuser
    permission: write
teams:
  - team: dev-team
    permission: admin
webhooks:
  - url: https://example.com/webhook
    events:
      - push
      - pull_request
    active: true
`,
			wantErr: false,
		},
		{
			name: "invalid YAML syntax",
			yaml: `
name: test-repo
description: A test repository
private: true
topics:
  - go
  - cli
invalid_yaml: [
`,
			wantErr: true,
			errMsg:  "failed to parse YAML",
		},
		{
			name: "invalid config",
			yaml: `
name: ""
description: A test repository
`,
			wantErr: true,
			errMsg:  "configuration validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadRepositoryConfig([]byte(tt.yaml))
			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadRepositoryConfig() expected error but got none")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("LoadRepositoryConfig() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("LoadRepositoryConfig() unexpected error = %v", err)
					return
				}
				if config == nil {
					t.Errorf("LoadRepositoryConfig() returned nil config")
				}
			}
		})
	}
}

func TestLoadRepositoryConfigFromFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Test valid config file
	validConfigPath := filepath.Join(tempDir, "valid_config.yaml")
	validConfig := `
name: test-repo
description: A test repository
private: true
topics:
  - go
  - cli
`
	if err := os.WriteFile(validConfigPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("Failed to write valid config file: %v", err)
	}

	config, err := LoadRepositoryConfigFromFile(validConfigPath)
	if err != nil {
		t.Errorf("LoadRepositoryConfigFromFile() unexpected error = %v", err)
	}
	if config == nil {
		t.Errorf("LoadRepositoryConfigFromFile() returned nil config")
		return
	}
	if config.Name != "test-repo" {
		t.Errorf("LoadRepositoryConfigFromFile() config.Name = %v, want %v", config.Name, "test-repo")
	}

	// Test non-existent file
	_, err = LoadRepositoryConfigFromFile(filepath.Join(tempDir, "nonexistent.yaml"))
	if err == nil {
		t.Errorf("LoadRepositoryConfigFromFile() expected error for non-existent file")
	}
	if !contains(err.Error(), "failed to read config file") {
		t.Errorf("LoadRepositoryConfigFromFile() error = %v, want error containing 'failed to read config file'", err)
	}
}

func TestIsValidPermission(t *testing.T) {
	tests := []struct {
		permission string
		want       bool
	}{
		{"read", true},
		{"write", true},
		{"admin", true},
		{"invalid", false},
		{"", false},
		{"READ", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.permission, func(t *testing.T) {
			if got := isValidPermission(tt.permission); got != tt.want {
				t.Errorf("isValidPermission(%v) = %v, want %v", tt.permission, got, tt.want)
			}
		})
	}
}

func TestIsValidWebhookEvent(t *testing.T) {
	tests := []struct {
		event string
		want  bool
	}{
		{"push", true},
		{"pull_request", true},
		{"issues", true},
		{"invalid_event", false},
		{"", false},
		{"PUSH", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			if got := isValidWebhookEvent(tt.event); got != tt.want {
				t.Errorf("isValidWebhookEvent(%v) = %v, want %v", tt.event, got, tt.want)
			}
		})
	}
}

func TestValidateGitHubUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid username",
			username: "testuser",
			wantErr:  false,
		},
		{
			name:     "valid username with numbers",
			username: "test123",
			wantErr:  false,
		},
		{
			name:     "valid username with hyphen",
			username: "test-user",
			wantErr:  false,
		},
		{
			name:     "valid single character",
			username: "a",
			wantErr:  false,
		},
		{
			name:     "empty username",
			username: "",
			wantErr:  true,
			errMsg:   "username cannot be empty",
		},
		{
			name:     "username too long",
			username: "this-is-a-very-long-username-that-exceeds-39-characters",
			wantErr:  true,
			errMsg:   "username must be 39 characters or less",
		},
		{
			name:     "username starts with hyphen",
			username: "-testuser",
			wantErr:  true,
			errMsg:   "username '-testuser' is invalid",
		},
		{
			name:     "username ends with hyphen",
			username: "testuser-",
			wantErr:  true,
			errMsg:   "username 'testuser-' is invalid",
		},
		{
			name:     "username with consecutive hyphens",
			username: "test--user",
			wantErr:  true,
			errMsg:   "username 'test--user' is invalid: cannot contain consecutive hyphens",
		},
		{
			name:     "username with invalid characters",
			username: "test_user",
			wantErr:  true,
			errMsg:   "username 'test_user' is invalid",
		},
		{
			name:     "username with spaces",
			username: "test user",
			wantErr:  true,
			errMsg:   "username 'test user' is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitHubUsername(tt.username)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateGitHubUsername() expected error but got none")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("validateGitHubUsername() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateGitHubUsername() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateGitHubTeamSlug(t *testing.T) {
	tests := []struct {
		name     string
		teamSlug string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid team slug",
			teamSlug: "dev-team",
			wantErr:  false,
		},
		{
			name:     "valid team slug with numbers",
			teamSlug: "team123",
			wantErr:  false,
		},
		{
			name:     "valid team slug with underscores",
			teamSlug: "dev_team",
			wantErr:  false,
		},
		{
			name:     "valid team slug mixed",
			teamSlug: "dev-team_123",
			wantErr:  false,
		},
		{
			name:     "valid single character",
			teamSlug: "a",
			wantErr:  false,
		},
		{
			name:     "empty team slug",
			teamSlug: "",
			wantErr:  true,
			errMsg:   "team slug cannot be empty",
		},
		{
			name:     "team slug too long",
			teamSlug: "this-is-a-very-long-team-slug-that-exceeds-the-maximum-allowed-length-of-one-hundred-characters-and-should-fail",
			wantErr:  true,
			errMsg:   "team slug must be 100 characters or less",
		},
		{
			name:     "team slug starts with hyphen",
			teamSlug: "-dev-team",
			wantErr:  true,
			errMsg:   "team slug '-dev-team' is invalid",
		},
		{
			name:     "team slug starts with underscore",
			teamSlug: "_dev-team",
			wantErr:  true,
			errMsg:   "team slug '_dev-team' is invalid",
		},
		{
			name:     "team slug with uppercase",
			teamSlug: "Dev-Team",
			wantErr:  true,
			errMsg:   "team slug 'Dev-Team' is invalid",
		},
		{
			name:     "team slug with spaces",
			teamSlug: "dev team",
			wantErr:  true,
			errMsg:   "team slug 'dev team' is invalid",
		},
		{
			name:     "team slug with invalid characters",
			teamSlug: "dev@team",
			wantErr:  true,
			errMsg:   "team slug 'dev@team' is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitHubTeamSlug(tt.teamSlug)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateGitHubTeamSlug() expected error but got none")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("validateGitHubTeamSlug() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateGitHubTeamSlug() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestLoadRepositoryConfigFromFile_Enhanced(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		configYAML  string
		filename    string
		wantErr     bool
		errMsg      string
		wantName    string
		wantPrivate bool
	}{
		{
			name: "single repository format",
			configYAML: `
name: single-repo
description: A single repository
private: true
topics:
  - go
  - cli
`,
			filename:    "single_repo.yaml",
			wantErr:     false,
			wantName:    "single-repo",
			wantPrivate: true,
		},
		{
			name: "multi-repository format with single repo",
			configYAML: `
version: "1.0"
defaults:
  private: true
  topics:
    - default-topic
repositories:
  - name: multi-single-repo
    description: A repository from multi-repo config
    topics:
      - specific-topic
`,
			filename:    "multi_single_repo.yaml",
			wantErr:     false,
			wantName:    "multi-single-repo",
			wantPrivate: true,
		},
		{
			name: "multi-repository format with multiple repos",
			configYAML: `
version: "1.0"
repositories:
  - name: repo1
    description: First repository
  - name: repo2
    description: Second repository
`,
			filename: "multi_repos.yaml",
			wantErr:  true,
			errMsg:   "multi-repository configuration detected with 2 repositories",
		},
		{
			name: "multi-repository format with defaults only",
			configYAML: `
version: "1.0"
defaults:
  private: false
  topics:
    - default-topic
repositories:
  - name: defaults-repo
`,
			filename:    "defaults_repo.yaml",
			wantErr:     false,
			wantName:    "defaults-repo",
			wantPrivate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(tempDir, tt.filename)
			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			config, err := LoadRepositoryConfigFromFile(configPath)
			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadRepositoryConfigFromFile() expected error but got none")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("LoadRepositoryConfigFromFile() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("LoadRepositoryConfigFromFile() unexpected error = %v", err)
					return
				}
				if config == nil {
					t.Errorf("LoadRepositoryConfigFromFile() returned nil config")
					return
				}
				if config.Name != tt.wantName {
					t.Errorf("LoadRepositoryConfigFromFile() config.Name = %v, want %v", config.Name, tt.wantName)
				}
				if config.Private != tt.wantPrivate {
					t.Errorf("LoadRepositoryConfigFromFile() config.Private = %v, want %v", config.Private, tt.wantPrivate)
				}
			}
		})
	}
}

func TestLoadMultiRepositoryConfigFromFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	tests := []struct {
		name       string
		configYAML string
		filename   string
		wantErr    bool
		errMsg     string
		wantRepos  int
	}{
		{
			name: "multi-repository format",
			configYAML: `
version: "1.0"
defaults:
  private: true
  topics:
    - default-topic
repositories:
  - name: repo1
    description: First repository
  - name: repo2
    description: Second repository
    private: false
`,
			filename:  "multi_repos.yaml",
			wantErr:   false,
			wantRepos: 2,
		},
		{
			name: "single repository format converted",
			configYAML: `
name: single-repo
description: A single repository
private: true
topics:
  - go
  - cli
`,
			filename:  "single_converted.yaml",
			wantErr:   false,
			wantRepos: 1,
		},
		{
			name: "invalid multi-repository format",
			configYAML: `
version: "1.0"
repositories:
  - name: ""
    description: Invalid repository
`,
			filename: "invalid_multi.yaml",
			wantErr:  true,
			errMsg:   "multi-repository configuration validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(tempDir, tt.filename)
			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			config, err := LoadMultiRepositoryConfigFromFile(configPath)
			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadMultiRepositoryConfigFromFile() expected error but got none")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("LoadMultiRepositoryConfigFromFile() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("LoadMultiRepositoryConfigFromFile() unexpected error = %v", err)
					return
				}
				if config == nil {
					t.Errorf("LoadMultiRepositoryConfigFromFile() returned nil config")
					return
				}
				if len(config.Repositories) != tt.wantRepos {
					t.Errorf("LoadMultiRepositoryConfigFromFile() len(config.Repositories) = %v, want %v", len(config.Repositories), tt.wantRepos)
				}
			}
		})
	}
}

func TestLoadRepositoryConfigFromFile_BackwardCompatibility(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Test that existing single repository configs still work exactly as before
	originalConfig := `
name: backward-compat-repo
description: Testing backward compatibility
private: false
topics:
  - go
  - test
features:
  issues: true
  wiki: false
  projects: true
  discussions: false
branch_protection:
  - pattern: main
    required_reviews: 1
    require_up_to_date: true
collaborators:
  - username: testuser
    permission: write
teams:
  - team: dev-team
    permission: admin
webhooks:
  - url: https://example.com/webhook
    events:
      - push
      - pull_request
    active: true
`

	configPath := filepath.Join(tempDir, "backward_compat.yaml")
	if err := os.WriteFile(configPath, []byte(originalConfig), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config, err := LoadRepositoryConfigFromFile(configPath)
	if err != nil {
		t.Errorf("LoadRepositoryConfigFromFile() backward compatibility failed: %v", err)
		return
	}

	// Verify all fields are loaded correctly
	if config.Name != "backward-compat-repo" {
		t.Errorf("Name = %v, want %v", config.Name, "backward-compat-repo")
	}
	if config.Description != "Testing backward compatibility" {
		t.Errorf("Description = %v, want %v", config.Description, "Testing backward compatibility")
	}
	if config.Private != false {
		t.Errorf("Private = %v, want %v", config.Private, false)
	}
	if len(config.Topics) != 2 || config.Topics[0] != "go" || config.Topics[1] != "test" {
		t.Errorf("Topics = %v, want %v", config.Topics, []string{"go", "test"})
	}
	if !config.Features.Issues {
		t.Errorf("Features.Issues = %v, want %v", config.Features.Issues, true)
	}
	if config.Features.Wiki {
		t.Errorf("Features.Wiki = %v, want %v", config.Features.Wiki, false)
	}
	if len(config.BranchRules) != 1 || config.BranchRules[0].Pattern != "main" {
		t.Errorf("BranchRules = %v, want pattern 'main'", config.BranchRules)
	}
	if len(config.Collaborators) != 1 || config.Collaborators[0].Username != "testuser" {
		t.Errorf("Collaborators = %v, want username 'testuser'", config.Collaborators)
	}
	if len(config.Teams) != 1 || config.Teams[0].TeamSlug != "dev-team" {
		t.Errorf("Teams = %v, want team 'dev-team'", config.Teams)
	}
	if len(config.Webhooks) != 1 || config.Webhooks[0].URL != "https://example.com/webhook" {
		t.Errorf("Webhooks = %v, want URL 'https://example.com/webhook'", config.Webhooks)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
