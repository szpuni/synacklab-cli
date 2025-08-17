package github

import (
	"strings"
	"testing"
)

func TestConfigFormat_String(t *testing.T) {
	tests := []struct {
		format   ConfigFormat
		expected string
	}{
		{FormatSingleRepository, "single-repository"},
		{FormatMultiRepository, "multi-repository"},
		{ConfigFormat(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.format.String(); got != tt.expected {
				t.Errorf("ConfigFormat.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDefaultConfigDetector_DetectFormat(t *testing.T) {
	detector := NewConfigDetector()

	tests := []struct {
		name     string
		yaml     string
		expected ConfigFormat
		wantErr  bool
	}{
		{
			name: "single repository with name",
			yaml: `
name: test-repo
description: Test repository
private: true
`,
			expected: FormatSingleRepository,
			wantErr:  false,
		},
		{
			name: "multi repository with repositories array",
			yaml: `
repositories:
  - name: repo1
    description: First repo
  - name: repo2
    description: Second repo
`,
			expected: FormatMultiRepository,
			wantErr:  false,
		},
		{
			name: "multi repository with defaults only",
			yaml: `
defaults:
  private: true
  description: Default description
repositories:
  - name: repo1
`,
			expected: FormatMultiRepository,
			wantErr:  false,
		},
		{
			name: "defaults without repositories (should be multi-repo)",
			yaml: `
defaults:
  private: true
  description: Default description
`,
			expected: FormatMultiRepository,
			wantErr:  false,
		},
		{
			name: "empty config (defaults to single-repo)",
			yaml: `
description: Some description
private: true
`,
			expected: FormatSingleRepository,
			wantErr:  false,
		},
		{
			name:     "invalid YAML",
			yaml:     `invalid: yaml: content: [`,
			expected: FormatSingleRepository,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format, err := detector.DetectFormat([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if format != tt.expected {
				t.Errorf("DetectFormat() = %v, want %v", format, tt.expected)
			}
		})
	}
}

func TestDefaultConfigDetector_LoadSingleRepo(t *testing.T) {
	detector := NewConfigDetector()

	validYAML := `
name: test-repo
description: Test repository
private: true
topics:
  - golang
  - api
`

	config, err := detector.LoadSingleRepo([]byte(validYAML))
	if err != nil {
		t.Fatalf("LoadSingleRepo() error = %v", err)
	}

	if config.Name != "test-repo" {
		t.Errorf("LoadSingleRepo() name = %v, want test-repo", config.Name)
	}
	if config.Description != "Test repository" {
		t.Errorf("LoadSingleRepo() description = %v, want Test repository", config.Description)
	}
	if !config.Private {
		t.Errorf("LoadSingleRepo() private = %v, want true", config.Private)
	}
}

func TestDefaultConfigDetector_LoadMultiRepo(t *testing.T) {
	detector := NewConfigDetector()

	validYAML := `
version: "1.0"
defaults:
  private: true
  description: Default description
  topics:
    - production
repositories:
  - name: repo1
    description: First repository
  - name: repo2
    description: Second repository
    private: false
`

	config, err := detector.LoadMultiRepo([]byte(validYAML))
	if err != nil {
		t.Fatalf("LoadMultiRepo() error = %v", err)
	}

	if config.Version != "1.0" {
		t.Errorf("LoadMultiRepo() version = %v, want 1.0", config.Version)
	}

	if config.Defaults == nil {
		t.Fatal("LoadMultiRepo() defaults is nil")
	}
	if *config.Defaults.Private != true {
		t.Errorf("LoadMultiRepo() defaults.private = %v, want true", *config.Defaults.Private)
	}

	if len(config.Repositories) != 2 {
		t.Fatalf("LoadMultiRepo() repositories count = %v, want 2", len(config.Repositories))
	}

	if config.Repositories[0].Name != "repo1" {
		t.Errorf("LoadMultiRepo() repositories[0].name = %v, want repo1", config.Repositories[0].Name)
	}
	if config.Repositories[1].Name != "repo2" {
		t.Errorf("LoadMultiRepo() repositories[1].name = %v, want repo2", config.Repositories[1].Name)
	}
}

func TestMultiRepositoryConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  MultiRepositoryConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: MultiRepositoryConfig{
				Repositories: []RepositoryConfig{
					{Name: "repo1", Description: "First repo"},
					{Name: "repo2", Description: "Second repo"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty repositories",
			config: MultiRepositoryConfig{
				Repositories: []RepositoryConfig{},
			},
			wantErr: true,
			errMsg:  "at least one repository must be defined",
		},
		{
			name: "duplicate repository names",
			config: MultiRepositoryConfig{
				Repositories: []RepositoryConfig{
					{Name: "repo1", Description: "First repo"},
					{Name: "repo1", Description: "Duplicate repo"},
				},
			},
			wantErr: true,
			errMsg:  "duplicate repository name",
		},
		{
			name: "repository without name",
			config: MultiRepositoryConfig{
				Repositories: []RepositoryConfig{
					{Description: "Repo without name"},
				},
			},
			wantErr: true,
			errMsg:  "repository name is required",
		},
		{
			name: "invalid defaults",
			config: MultiRepositoryConfig{
				Defaults: &RepositoryDefaults{
					Description: strings.Repeat("a", 351), // Too long
				},
				Repositories: []RepositoryConfig{
					{Name: "repo1", Description: "Valid repo"},
				},
			},
			wantErr: true,
			errMsg:  "default description must be 350 characters or less",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("MultiRepositoryConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("MultiRepositoryConfig.Validate() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestMultiRepositoryConfig_validateDefaults(t *testing.T) {
	tests := []struct {
		name     string
		defaults RepositoryDefaults
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid defaults",
			defaults: RepositoryDefaults{
				Description: "Valid description",
				Private:     boolPtr(true),
				Topics:      []string{"golang", "api"},
			},
			wantErr: false,
		},
		{
			name: "description too long",
			defaults: RepositoryDefaults{
				Description: strings.Repeat("a", 351),
			},
			wantErr: true,
			errMsg:  "default description must be 350 characters or less",
		},
		{
			name: "too many topics",
			defaults: RepositoryDefaults{
				Topics: make([]string, 21), // Too many topics
			},
			wantErr: true,
			errMsg:  "default topics can have at most 20 items",
		},
		{
			name: "empty topic",
			defaults: RepositoryDefaults{
				Topics: []string{"valid", ""},
			},
			wantErr: true,
			errMsg:  "default topic 2 cannot be empty",
		},
		{
			name: "topic too long",
			defaults: RepositoryDefaults{
				Topics: []string{strings.Repeat("a", 51)},
			},
			wantErr: true,
			errMsg:  "default topic 1 must be 50 characters or less",
		},
		{
			name: "invalid branch protection rule",
			defaults: RepositoryDefaults{
				BranchRules: []BranchProtectionRule{
					{Pattern: ""}, // Empty pattern
				},
			},
			wantErr: true,
			errMsg:  "default branch protection rule 1: pattern is required",
		},
		{
			name: "invalid required reviews",
			defaults: RepositoryDefaults{
				BranchRules: []BranchProtectionRule{
					{Pattern: "main", RequiredReviews: 7}, // Too many reviews
				},
			},
			wantErr: true,
			errMsg:  "default branch protection rule 1: required reviews must be between 0 and 6",
		},
		{
			name: "invalid collaborator",
			defaults: RepositoryDefaults{
				Collaborators: []Collaborator{
					{Username: "", Permission: "read"}, // Empty username
				},
			},
			wantErr: true,
			errMsg:  "default collaborator 1: username is required",
		},
		{
			name: "invalid team",
			defaults: RepositoryDefaults{
				Teams: []TeamAccess{
					{TeamSlug: "", Permission: "read"}, // Empty team slug
				},
			},
			wantErr: true,
			errMsg:  "default team 1: team slug is required",
		},
		{
			name: "invalid webhook",
			defaults: RepositoryDefaults{
				Webhooks: []Webhook{
					{URL: "", Events: []string{"push"}}, // Empty URL
				},
			},
			wantErr: true,
			errMsg:  "default webhook 1: URL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := MultiRepositoryConfig{
				Defaults: &tt.defaults,
				Repositories: []RepositoryConfig{
					{Name: "test-repo"},
				},
			}

			err := config.validateDefaults()
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDefaults() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("validateDefaults() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// This test would require creating temporary files, which is more complex
	// For now, we'll test the core logic through the detector methods
	detector := NewConfigDetector()

	singleRepoYAML := `
name: test-repo
description: Test repository
`

	multiRepoYAML := `
repositories:
  - name: repo1
    description: First repo
`

	// Test single repository detection and loading
	format, err := detector.DetectFormat([]byte(singleRepoYAML))
	if err != nil {
		t.Fatalf("DetectFormat() error = %v", err)
	}
	if format != FormatSingleRepository {
		t.Errorf("DetectFormat() = %v, want %v", format, FormatSingleRepository)
	}

	singleConfig, err := detector.LoadSingleRepo([]byte(singleRepoYAML))
	if err != nil {
		t.Fatalf("LoadSingleRepo() error = %v", err)
	}
	if singleConfig.Name != "test-repo" {
		t.Errorf("LoadSingleRepo() name = %v, want test-repo", singleConfig.Name)
	}

	// Test multi repository detection and loading
	format, err = detector.DetectFormat([]byte(multiRepoYAML))
	if err != nil {
		t.Fatalf("DetectFormat() error = %v", err)
	}
	if format != FormatMultiRepository {
		t.Errorf("DetectFormat() = %v, want %v", format, FormatMultiRepository)
	}

	multiConfig, err := detector.LoadMultiRepo([]byte(multiRepoYAML))
	if err != nil {
		t.Fatalf("LoadMultiRepo() error = %v", err)
	}
	if len(multiConfig.Repositories) != 1 {
		t.Errorf("LoadMultiRepo() repositories count = %v, want 1", len(multiConfig.Repositories))
	}
}

func TestDefaultConfigMerger_MergeDefaults(t *testing.T) {
	merger := NewConfigMerger()

	tests := []struct {
		name     string
		defaults *RepositoryDefaults
		repo     *RepositoryConfig
		expected *RepositoryConfig
	}{
		{
			name:     "nil defaults",
			defaults: nil,
			repo: &RepositoryConfig{
				Name:        "test-repo",
				Description: "Test repo",
				Private:     true,
			},
			expected: &RepositoryConfig{
				Name:        "test-repo",
				Description: "Test repo",
				Private:     true,
			},
		},
		{
			name: "merge description from defaults",
			defaults: &RepositoryDefaults{
				Description: "Default description",
				Private:     boolPtr(true),
			},
			repo: &RepositoryConfig{
				Name: "test-repo",
			},
			expected: &RepositoryConfig{
				Name:        "test-repo",
				Description: "Default description",
				Private:     true,
			},
		},
		{
			name: "repository overrides defaults",
			defaults: &RepositoryDefaults{
				Description: "Default description",
				Private:     boolPtr(false), // Default is false
			},
			repo: &RepositoryConfig{
				Name:        "test-repo",
				Description: "Repo description",
				Private:     true, // Repository explicitly sets true
			},
			expected: &RepositoryConfig{
				Name:        "test-repo",
				Description: "Repo description",
				Private:     true, // Should keep repository value
			},
		},
		{
			name: "merge topics from defaults",
			defaults: &RepositoryDefaults{
				Topics: []string{"golang", "api"},
			},
			repo: &RepositoryConfig{
				Name: "test-repo",
			},
			expected: &RepositoryConfig{
				Name:   "test-repo",
				Topics: []string{"golang", "api"},
			},
		},
		{
			name: "repository topics override defaults",
			defaults: &RepositoryDefaults{
				Topics: []string{"golang", "api"},
			},
			repo: &RepositoryConfig{
				Name:   "test-repo",
				Topics: []string{"javascript", "frontend"},
			},
			expected: &RepositoryConfig{
				Name:   "test-repo",
				Topics: []string{"javascript", "frontend"},
			},
		},
		{
			name: "merge features from defaults",
			defaults: &RepositoryDefaults{
				Features: &RepositoryFeatures{
					Issues:   true,
					Wiki:     false,
					Projects: true,
				},
			},
			repo: &RepositoryConfig{
				Name: "test-repo",
			},
			expected: &RepositoryConfig{
				Name: "test-repo",
				Features: RepositoryFeatures{
					Issues:   true,
					Wiki:     false,
					Projects: true,
				},
			},
		},
		{
			name: "merge branch protection rules from defaults",
			defaults: &RepositoryDefaults{
				BranchRules: []BranchProtectionRule{
					{
						Pattern:         "main",
						RequiredReviews: 2,
						RequireUpToDate: true,
					},
				},
			},
			repo: &RepositoryConfig{
				Name: "test-repo",
			},
			expected: &RepositoryConfig{
				Name: "test-repo",
				BranchRules: []BranchProtectionRule{
					{
						Pattern:         "main",
						RequiredReviews: 2,
						RequireUpToDate: true,
					},
				},
			},
		},
		{
			name: "merge collaborators from defaults",
			defaults: &RepositoryDefaults{
				Collaborators: []Collaborator{
					{Username: "admin-user", Permission: "admin"},
					{Username: "dev-user", Permission: "write"},
				},
			},
			repo: &RepositoryConfig{
				Name: "test-repo",
			},
			expected: &RepositoryConfig{
				Name: "test-repo",
				Collaborators: []Collaborator{
					{Username: "admin-user", Permission: "admin"},
					{Username: "dev-user", Permission: "write"},
				},
			},
		},
		{
			name: "merge teams from defaults",
			defaults: &RepositoryDefaults{
				Teams: []TeamAccess{
					{TeamSlug: "backend-team", Permission: "write"},
					{TeamSlug: "devops-team", Permission: "admin"},
				},
			},
			repo: &RepositoryConfig{
				Name: "test-repo",
			},
			expected: &RepositoryConfig{
				Name: "test-repo",
				Teams: []TeamAccess{
					{TeamSlug: "backend-team", Permission: "write"},
					{TeamSlug: "devops-team", Permission: "admin"},
				},
			},
		},
		{
			name: "merge webhooks from defaults",
			defaults: &RepositoryDefaults{
				Webhooks: []Webhook{
					{
						URL:    "https://ci.example.com/webhook",
						Events: []string{"push", "pull_request"},
						Active: true,
					},
				},
			},
			repo: &RepositoryConfig{
				Name: "test-repo",
			},
			expected: &RepositoryConfig{
				Name: "test-repo",
				Webhooks: []Webhook{
					{
						URL:    "https://ci.example.com/webhook",
						Events: []string{"push", "pull_request"},
						Active: true,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := merger.MergeDefaults(tt.defaults, tt.repo)
			if err != nil {
				t.Fatalf("MergeDefaults() error = %v", err)
			}

			if result.Name != tt.expected.Name {
				t.Errorf("MergeDefaults() name = %v, want %v", result.Name, tt.expected.Name)
			}
			if result.Description != tt.expected.Description {
				t.Errorf("MergeDefaults() description = %v, want %v", result.Description, tt.expected.Description)
			}
			if result.Private != tt.expected.Private {
				t.Errorf("MergeDefaults() private = %v, want %v", result.Private, tt.expected.Private)
			}
			if len(result.Topics) != len(tt.expected.Topics) {
				t.Errorf("MergeDefaults() topics length = %v, want %v", len(result.Topics), len(tt.expected.Topics))
			}
			for i, topic := range result.Topics {
				if i < len(tt.expected.Topics) && topic != tt.expected.Topics[i] {
					t.Errorf("MergeDefaults() topics[%d] = %v, want %v", i, topic, tt.expected.Topics[i])
				}
			}

			// Test branch rules
			if len(result.BranchRules) != len(tt.expected.BranchRules) {
				t.Errorf("MergeDefaults() branch rules length = %v, want %v", len(result.BranchRules), len(tt.expected.BranchRules))
			}
			for i, rule := range result.BranchRules {
				if i < len(tt.expected.BranchRules) {
					expected := tt.expected.BranchRules[i]
					if rule.Pattern != expected.Pattern {
						t.Errorf("MergeDefaults() branch rule[%d].pattern = %v, want %v", i, rule.Pattern, expected.Pattern)
					}
					if rule.RequiredReviews != expected.RequiredReviews {
						t.Errorf("MergeDefaults() branch rule[%d].required_reviews = %v, want %v", i, rule.RequiredReviews, expected.RequiredReviews)
					}
				}
			}

			// Test collaborators
			if len(result.Collaborators) != len(tt.expected.Collaborators) {
				t.Errorf("MergeDefaults() collaborators length = %v, want %v", len(result.Collaborators), len(tt.expected.Collaborators))
			}
			for i, collab := range result.Collaborators {
				if i < len(tt.expected.Collaborators) {
					expected := tt.expected.Collaborators[i]
					if collab.Username != expected.Username {
						t.Errorf("MergeDefaults() collaborator[%d].username = %v, want %v", i, collab.Username, expected.Username)
					}
					if collab.Permission != expected.Permission {
						t.Errorf("MergeDefaults() collaborator[%d].permission = %v, want %v", i, collab.Permission, expected.Permission)
					}
				}
			}

			// Test teams
			if len(result.Teams) != len(tt.expected.Teams) {
				t.Errorf("MergeDefaults() teams length = %v, want %v", len(result.Teams), len(tt.expected.Teams))
			}
			for i, team := range result.Teams {
				if i < len(tt.expected.Teams) {
					expected := tt.expected.Teams[i]
					if team.TeamSlug != expected.TeamSlug {
						t.Errorf("MergeDefaults() team[%d].team_slug = %v, want %v", i, team.TeamSlug, expected.TeamSlug)
					}
					if team.Permission != expected.Permission {
						t.Errorf("MergeDefaults() team[%d].permission = %v, want %v", i, team.Permission, expected.Permission)
					}
				}
			}

			// Test webhooks
			if len(result.Webhooks) != len(tt.expected.Webhooks) {
				t.Errorf("MergeDefaults() webhooks length = %v, want %v", len(result.Webhooks), len(tt.expected.Webhooks))
			}
			for i, webhook := range result.Webhooks {
				if i < len(tt.expected.Webhooks) {
					expected := tt.expected.Webhooks[i]
					if webhook.URL != expected.URL {
						t.Errorf("MergeDefaults() webhook[%d].url = %v, want %v", i, webhook.URL, expected.URL)
					}
					if webhook.Active != expected.Active {
						t.Errorf("MergeDefaults() webhook[%d].active = %v, want %v", i, webhook.Active, expected.Active)
					}
					if len(webhook.Events) != len(expected.Events) {
						t.Errorf("MergeDefaults() webhook[%d].events length = %v, want %v", i, len(webhook.Events), len(expected.Events))
					}
				}
			}
		})
	}
}

func TestDefaultConfigMerger_ValidateMergedConfig(t *testing.T) {
	merger := NewConfigMerger()

	validConfig := &RepositoryConfig{
		Name:        "test-repo",
		Description: "Test repository",
	}

	err := merger.ValidateMergedConfig(validConfig)
	if err != nil {
		t.Errorf("ValidateMergedConfig() error = %v, want nil", err)
	}

	invalidConfig := &RepositoryConfig{
		Name: "", // Invalid: empty name
	}

	err = merger.ValidateMergedConfig(invalidConfig)
	if err == nil {
		t.Error("ValidateMergedConfig() error = nil, want error for invalid config")
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
func TestDefaultConfigMerger_MergeStrategies(t *testing.T) {
	tests := []struct {
		name     string
		strategy MergeStrategy
		field    string
		defaults *RepositoryDefaults
		repo     *RepositoryConfig
		validate func(t *testing.T, result *RepositoryConfig)
	}{
		{
			name:     "topics override strategy",
			strategy: MergeStrategyOverride,
			field:    "topics",
			defaults: &RepositoryDefaults{
				Topics: []string{"golang", "api"},
			},
			repo: &RepositoryConfig{
				Name:   "test-repo",
				Topics: []string{"javascript", "frontend"},
			},
			validate: func(t *testing.T, result *RepositoryConfig) {
				t.Helper()
				// Repository topics should be kept (override strategy)
				expected := []string{"javascript", "frontend"}
				if len(result.Topics) != len(expected) {
					t.Errorf("Topics length = %v, want %v", len(result.Topics), len(expected))
				}
				for i, topic := range result.Topics {
					if i < len(expected) && topic != expected[i] {
						t.Errorf("Topics[%d] = %v, want %v", i, topic, expected[i])
					}
				}
			},
		},
		{
			name:     "topics append strategy",
			strategy: MergeStrategyAppend,
			field:    "topics",
			defaults: &RepositoryDefaults{
				Topics: []string{"golang", "api"},
			},
			repo: &RepositoryConfig{
				Name:   "test-repo",
				Topics: []string{"javascript", "frontend"},
			},
			validate: func(t *testing.T, result *RepositoryConfig) {
				t.Helper()
				// Should have all topics without duplicates
				expected := []string{"javascript", "frontend", "golang", "api"}
				if len(result.Topics) != len(expected) {
					t.Errorf("Topics length = %v, want %v", len(result.Topics), len(expected))
				}
				topicSet := make(map[string]bool)
				for _, topic := range result.Topics {
					topicSet[topic] = true
				}
				for _, expectedTopic := range expected {
					if !topicSet[expectedTopic] {
						t.Errorf("Missing expected topic: %v", expectedTopic)
					}
				}
			},
		},
		{
			name:     "topics append strategy with duplicates",
			strategy: MergeStrategyAppend,
			field:    "topics",
			defaults: &RepositoryDefaults{
				Topics: []string{"golang", "api", "javascript"},
			},
			repo: &RepositoryConfig{
				Name:   "test-repo",
				Topics: []string{"javascript", "frontend"},
			},
			validate: func(t *testing.T, result *RepositoryConfig) {
				t.Helper()
				// Should have unique topics only
				expected := []string{"javascript", "frontend", "golang", "api"}
				if len(result.Topics) != len(expected) {
					t.Errorf("Topics length = %v, want %v", len(result.Topics), len(expected))
				}
				// Check for duplicates
				topicSet := make(map[string]int)
				for _, topic := range result.Topics {
					topicSet[topic]++
				}
				for topic, count := range topicSet {
					if count > 1 {
						t.Errorf("Duplicate topic found: %v (count: %d)", topic, count)
					}
				}
			},
		},
		{
			name:     "collaborators append strategy",
			strategy: MergeStrategyAppend,
			field:    "collaborators",
			defaults: &RepositoryDefaults{
				Collaborators: []Collaborator{
					{Username: "admin-user", Permission: "admin"},
					{Username: "dev-user", Permission: "write"},
				},
			},
			repo: &RepositoryConfig{
				Name: "test-repo",
				Collaborators: []Collaborator{
					{Username: "repo-user", Permission: "read"},
				},
			},
			validate: func(t *testing.T, result *RepositoryConfig) {
				t.Helper()
				expected := 3 // repo-user + admin-user + dev-user
				if len(result.Collaborators) != expected {
					t.Errorf("Collaborators length = %v, want %v", len(result.Collaborators), expected)
				}
				userSet := make(map[string]bool)
				for _, collab := range result.Collaborators {
					userSet[collab.Username] = true
				}
				expectedUsers := []string{"repo-user", "admin-user", "dev-user"}
				for _, user := range expectedUsers {
					if !userSet[user] {
						t.Errorf("Missing expected collaborator: %v", user)
					}
				}
			},
		},
		{
			name:     "branch rules deep merge strategy",
			strategy: MergeStrategyDeepMerge,
			field:    "branch_rules",
			defaults: &RepositoryDefaults{
				BranchRules: []BranchProtectionRule{
					{
						Pattern:                "main",
						RequiredReviews:        2,
						RequiredStatusChecks:   []string{"ci/build", "ci/test"},
						RequireUpToDate:        true,
						DismissStaleReviews:    true,
						RequireCodeOwnerReview: true,
					},
					{
						Pattern:         "develop",
						RequiredReviews: 1,
					},
				},
			},
			repo: &RepositoryConfig{
				Name: "test-repo",
				BranchRules: []BranchProtectionRule{
					{
						Pattern:              "main",
						RequiredReviews:      3,                         // Should override default
						RequiredStatusChecks: []string{"security/scan"}, // Should merge with defaults
					},
				},
			},
			validate: func(t *testing.T, result *RepositoryConfig) {
				t.Helper()
				if len(result.BranchRules) != 2 {
					t.Errorf("BranchRules length = %v, want 2", len(result.BranchRules))
					return
				}

				// Find main branch rule
				var mainRule *BranchProtectionRule
				var developRule *BranchProtectionRule
				for i := range result.BranchRules {
					switch result.BranchRules[i].Pattern {
					case "main":
						mainRule = &result.BranchRules[i]
					case "develop":
						developRule = &result.BranchRules[i]
					}
				}

				if mainRule == nil {
					t.Error("Main branch rule not found")
					return
				}
				if developRule == nil {
					t.Error("Develop branch rule not found")
					return
				}

				// Main rule should have merged status checks
				expectedChecks := []string{"security/scan", "ci/build", "ci/test"}
				if len(mainRule.RequiredStatusChecks) != len(expectedChecks) {
					t.Errorf("Main rule status checks length = %v, want %v", len(mainRule.RequiredStatusChecks), len(expectedChecks))
				}

				// Repository values should override defaults for numeric/boolean fields
				if mainRule.RequiredReviews != 3 {
					t.Errorf("Main rule required reviews = %v, want 3", mainRule.RequiredReviews)
				}

				// Develop rule should be from defaults only
				if developRule.RequiredReviews != 1 {
					t.Errorf("Develop rule required reviews = %v, want 1", developRule.RequiredReviews)
				}
			},
		},
		{
			name:     "webhooks deep merge strategy",
			strategy: MergeStrategyDeepMerge,
			field:    "webhooks",
			defaults: &RepositoryDefaults{
				Webhooks: []Webhook{
					{
						URL:    "https://ci.example.com/webhook",
						Events: []string{"push", "pull_request"},
						Active: true,
					},
				},
			},
			repo: &RepositoryConfig{
				Name: "test-repo",
				Webhooks: []Webhook{
					{
						URL:    "https://ci.example.com/webhook",
						Events: []string{"issues", "pull_request_review"},
						Active: false, // Should override default
					},
				},
			},
			validate: func(t *testing.T, result *RepositoryConfig) {
				t.Helper()
				if len(result.Webhooks) != 1 {
					t.Errorf("Webhooks length = %v, want 1", len(result.Webhooks))
					return
				}

				webhook := result.Webhooks[0]
				// Events should be merged
				expectedEvents := []string{"issues", "pull_request_review", "push", "pull_request"}
				if len(webhook.Events) != len(expectedEvents) {
					t.Errorf("Webhook events length = %v, want %v", len(webhook.Events), len(expectedEvents))
				}

				// Active should keep repository value (false)
				if webhook.Active != false {
					t.Errorf("Webhook active = %v, want false", webhook.Active)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merger := NewConfigMerger()
			merger.SetMergeStrategy(tt.field, tt.strategy)

			result, err := merger.MergeDefaults(tt.defaults, tt.repo)
			if err != nil {
				t.Fatalf("MergeDefaults() error = %v", err)
			}

			tt.validate(t, result)
		})
	}
}

func TestDefaultConfigMerger_DeepCopy(t *testing.T) {
	merger := &DefaultConfigMerger{}

	original := &RepositoryConfig{
		Name:        "test-repo",
		Description: "Test repository",
		Private:     true,
		Topics:      []string{"golang", "api"},
		Features: RepositoryFeatures{
			Issues:   true,
			Wiki:     false,
			Projects: true,
		},
		BranchRules: []BranchProtectionRule{
			{
				Pattern:              "main",
				RequiredReviews:      2,
				RequiredStatusChecks: []string{"ci/build", "ci/test"},
				RestrictPushes:       []string{"admin"},
			},
		},
		Collaborators: []Collaborator{
			{Username: "user1", Permission: "admin"},
		},
		Teams: []TeamAccess{
			{TeamSlug: "team1", Permission: "write"},
		},
		Webhooks: []Webhook{
			{
				URL:    "https://example.com/webhook",
				Events: []string{"push", "pull_request"},
				Active: true,
			},
		},
	}

	copied, err := merger.deepCopyRepositoryConfig(original)
	if err != nil {
		t.Fatalf("deepCopyRepositoryConfig() error = %v", err)
	}

	// Verify deep copy worked
	if copied == original {
		t.Error("deepCopyRepositoryConfig() returned same pointer")
	}

	// Verify values are equal
	if copied.Name != original.Name {
		t.Errorf("Name = %v, want %v", copied.Name, original.Name)
	}
	if copied.Description != original.Description {
		t.Errorf("Description = %v, want %v", copied.Description, original.Description)
	}
	if copied.Private != original.Private {
		t.Errorf("Private = %v, want %v", copied.Private, original.Private)
	}

	// Verify slices are different pointers but same content
	if &copied.Topics == &original.Topics {
		t.Error("Topics slice has same pointer")
	}
	if len(copied.Topics) != len(original.Topics) {
		t.Errorf("Topics length = %v, want %v", len(copied.Topics), len(original.Topics))
	}

	// Modify copied to ensure independence
	copied.Topics[0] = "modified"
	if original.Topics[0] == "modified" {
		t.Error("Modifying copied Topics affected original")
	}

	// Test branch rules deep copy
	if &copied.BranchRules == &original.BranchRules {
		t.Error("BranchRules slice has same pointer")
	}
	if &copied.BranchRules[0].RequiredStatusChecks == &original.BranchRules[0].RequiredStatusChecks {
		t.Error("BranchRules RequiredStatusChecks slice has same pointer")
	}

	// Test webhooks deep copy
	if &copied.Webhooks == &original.Webhooks {
		t.Error("Webhooks slice has same pointer")
	}
	if &copied.Webhooks[0].Events == &original.Webhooks[0].Events {
		t.Error("Webhook Events slice has same pointer")
	}
}

func TestDefaultConfigMerger_ErrorHandling(t *testing.T) {
	merger := &DefaultConfigMerger{}

	// Test nil repository config
	_, err := merger.deepCopyRepositoryConfig(nil)
	if err == nil {
		t.Error("deepCopyRepositoryConfig() with nil config should return error")
	}

	// Test merge with nil repository (should not happen in practice)
	defaults := &RepositoryDefaults{
		Description: "Default description",
	}

	// This should work fine as MergeDefaults handles nil repo by creating a copy
	result, err := merger.MergeDefaults(defaults, &RepositoryConfig{Name: "test"})
	if err != nil {
		t.Errorf("MergeDefaults() error = %v", err)
	}
	if result.Description != "Default description" {
		t.Errorf("Description = %v, want 'Default description'", result.Description)
	}
}

func TestDefaultConfigMerger_SetMergeStrategy(t *testing.T) {
	merger := &DefaultConfigMerger{}

	// Test setting strategy on empty map
	merger.SetMergeStrategy("topics", MergeStrategyAppend)
	if merger.strategies["topics"] != MergeStrategyAppend {
		t.Errorf("Strategy for topics = %v, want %v", merger.strategies["topics"], MergeStrategyAppend)
	}

	// Test overriding existing strategy
	merger.SetMergeStrategy("topics", MergeStrategyDeepMerge)
	if merger.strategies["topics"] != MergeStrategyDeepMerge {
		t.Errorf("Strategy for topics = %v, want %v", merger.strategies["topics"], MergeStrategyDeepMerge)
	}
}

func TestMergeStrategy_EdgeCases(t *testing.T) {
	merger := NewConfigMerger()

	// Test empty defaults with non-empty repository
	defaults := &RepositoryDefaults{}
	repo := &RepositoryConfig{
		Name:   "test-repo",
		Topics: []string{"existing"},
	}

	result, err := merger.MergeDefaults(defaults, repo)
	if err != nil {
		t.Fatalf("MergeDefaults() error = %v", err)
	}

	if len(result.Topics) != 1 || result.Topics[0] != "existing" {
		t.Errorf("Topics = %v, want ['existing']", result.Topics)
	}

	// Test complex merge scenario
	defaults = &RepositoryDefaults{
		Description: "Default description",
		Private:     boolPtr(true),
		Topics:      []string{"default-topic"},
		Features: &RepositoryFeatures{
			Issues:   true,
			Wiki:     false,
			Projects: true,
		},
		BranchRules: []BranchProtectionRule{
			{Pattern: "main", RequiredReviews: 2},
		},
		Collaborators: []Collaborator{
			{Username: "default-user", Permission: "read"},
		},
		Teams: []TeamAccess{
			{TeamSlug: "default-team", Permission: "write"},
		},
		Webhooks: []Webhook{
			{URL: "https://default.com", Events: []string{"push"}, Active: true},
		},
	}

	repo = &RepositoryConfig{
		Name: "complex-repo",
		// Leave other fields empty to test defaults application
	}

	result, err = merger.MergeDefaults(defaults, repo)
	if err != nil {
		t.Fatalf("MergeDefaults() error = %v", err)
	}

	// Verify all defaults were applied
	if result.Description != "Default description" {
		t.Errorf("Description = %v, want 'Default description'", result.Description)
	}
	if !result.Private {
		t.Errorf("Private = %v, want true", result.Private)
	}
	if len(result.Topics) != 1 || result.Topics[0] != "default-topic" {
		t.Errorf("Topics = %v, want ['default-topic']", result.Topics)
	}
	if !result.Features.Issues {
		t.Errorf("Features.Issues = %v, want true", result.Features.Issues)
	}
	if len(result.BranchRules) != 1 {
		t.Errorf("BranchRules length = %v, want 1", len(result.BranchRules))
	}
	if len(result.Collaborators) != 1 {
		t.Errorf("Collaborators length = %v, want 1", len(result.Collaborators))
	}
	if len(result.Teams) != 1 {
		t.Errorf("Teams length = %v, want 1", len(result.Teams))
	}
	if len(result.Webhooks) != 1 {
		t.Errorf("Webhooks length = %v, want 1", len(result.Webhooks))
	}
}
