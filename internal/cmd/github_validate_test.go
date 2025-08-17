package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"synacklab/pkg/github"
)

func TestValidateCmd_FileNotFound(t *testing.T) {
	err := runGitHubValidate(githubValidateCmd, []string{"nonexistent.yaml"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestValidateCmd_SingleRepository(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test.yaml")
	config := `name: test-repo
description: A test repository
private: true
topics:
  - golang
  - api
features:
  issues: true
  wiki: false
`
	err := os.WriteFile(configFile, []byte(config), 0644)
	require.NoError(t, err)

	// Test that the configuration loads without error (offline validation)
	_, err = github.LoadRepositoryConfigFromFile(configFile)
	assert.NoError(t, err)
}

func TestValidateCmd_MultiRepository(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "multi-test.yaml")
	config := `version: "1.0"
defaults:
  private: true
  topics:
    - production
  features:
    issues: true
    wiki: false
repositories:
  - name: service-a
    description: Service A
    topics:
      - golang
      - api
  - name: service-b
    description: Service B
    private: false
    topics:
      - python
      - web
`
	err := os.WriteFile(configFile, []byte(config), 0644)
	require.NoError(t, err)

	// Test that the multi-repository configuration loads without error
	multiConfig, err := github.LoadMultiRepositoryConfigFromFile(configFile)
	assert.NoError(t, err)
	assert.Len(t, multiConfig.Repositories, 2)
	assert.Equal(t, "service-a", multiConfig.Repositories[0].Name)
	assert.Equal(t, "service-b", multiConfig.Repositories[1].Name)
}

func TestValidateCmd_MultiRepositoryWithInvalidConfig(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "invalid-multi.yaml")
	config := `version: "1.0"
repositories:
  - name: ""
    description: Empty name repository
  - name: service-a
    description: Valid repository
  - name: service-a
    description: Duplicate name
`
	err := os.WriteFile(configFile, []byte(config), 0644)
	require.NoError(t, err)

	// Test that validation catches the errors
	_, err = github.LoadMultiRepositoryConfigFromFile(configFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestValidateCmd_RepositoryFilter(t *testing.T) {
	tests := []struct {
		name          string
		repoFilter    string
		expectedRepos []string
	}{
		{
			name:          "single repository filter",
			repoFilter:    "service-a",
			expectedRepos: []string{"service-a"},
		},
		{
			name:          "multiple repositories filter",
			repoFilter:    "service-a,service-b",
			expectedRepos: []string{"service-a", "service-b"},
		},
		{
			name:          "filter with spaces",
			repoFilter:    "service-a, service-b",
			expectedRepos: []string{"service-a", "service-b"},
		},
		{
			name:          "empty filter",
			repoFilter:    "",
			expectedRepos: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse repository filter
			var repoFilter []string
			if tt.repoFilter != "" {
				repoFilter = strings.Split(strings.ReplaceAll(tt.repoFilter, " ", ""), ",")
				// Remove empty strings
				var filtered []string
				for _, repo := range repoFilter {
					if repo != "" {
						filtered = append(filtered, repo)
					}
				}
				repoFilter = filtered
			}

			assert.Equal(t, tt.expectedRepos, repoFilter)
		})
	}
}

func TestValidateCmd_ConfigFormatDetection(t *testing.T) {
	tests := []struct {
		name           string
		config         string
		expectedFormat github.ConfigFormat
	}{
		{
			name: "single repository format",
			config: `name: test-repo
description: A test repository
private: true`,
			expectedFormat: github.FormatSingleRepository,
		},
		{
			name: "multi repository format with repositories array",
			config: `repositories:
  - name: repo1
    description: Repository 1
  - name: repo2
    description: Repository 2`,
			expectedFormat: github.FormatMultiRepository,
		},
		{
			name: "multi repository format with defaults",
			config: `defaults:
  private: true
repositories:
  - name: repo1
    description: Repository 1`,
			expectedFormat: github.FormatMultiRepository,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "test.yaml")
			err := os.WriteFile(configFile, []byte(tt.config), 0644)
			require.NoError(t, err)

			_, format, err := github.LoadConfigFromFile(configFile)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedFormat, format)
		})
	}
}

func TestValidateCmd_MultiRepositoryValidationDetails(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "validation-test.yaml")
	config := `version: "1.0"
defaults:
  private: true
  topics:
    - production
repositories:
  - name: valid-repo
    description: A valid repository
    topics:
      - golang
      - api
  - name: ""
    description: Repository with empty name
  - name: invalid-topic-repo
    description: Repository with invalid topic
    topics:
      - "this-topic-is-way-too-long-and-exceeds-the-fifty-character-limit-for-github-topics"
`
	err := os.WriteFile(configFile, []byte(config), 0644)
	require.NoError(t, err)

	// Test that validation catches specific errors
	_, err = github.LoadMultiRepositoryConfigFromFile(configFile)
	assert.Error(t, err)

	// Check that the error contains information about the validation failures
	errorMsg := err.Error()
	assert.Contains(t, errorMsg, "validation failed")
}

func TestValidateCmd_SingleRepositoryBackwardCompatibility(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "legacy.yaml")
	config := `name: legacy-repo
description: Legacy single repository configuration
private: false
topics:
  - legacy
  - api
features:
  issues: true
  wiki: true
  projects: false
branch_protection:
  - pattern: main
    required_reviews: 2
    dismiss_stale_reviews: true
collaborators:
  - username: developer1
    permission: write
teams:
  - team: backend-team
    permission: admin
webhooks:
  - url: https://example.com/webhook
    events:
      - push
      - pull_request
    active: true
`
	err := os.WriteFile(configFile, []byte(config), 0644)
	require.NoError(t, err)

	// Test that legacy configuration still works
	repoConfig, err := github.LoadRepositoryConfigFromFile(configFile)
	assert.NoError(t, err)
	assert.Equal(t, "legacy-repo", repoConfig.Name)
	assert.Equal(t, "Legacy single repository configuration", repoConfig.Description)
	assert.False(t, repoConfig.Private)
	assert.Len(t, repoConfig.Topics, 2)
	assert.Len(t, repoConfig.BranchRules, 1)
	assert.Len(t, repoConfig.Collaborators, 1)
	assert.Len(t, repoConfig.Teams, 1)
	assert.Len(t, repoConfig.Webhooks, 1)
}

func TestValidateCmd_MultiRepositoryWithDefaults(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "defaults-test.yaml")
	config := `version: "1.0"
defaults:
  private: true
  topics:
    - production
    - microservice
  features:
    issues: true
    wiki: false
  branch_protection:
    - pattern: main
      required_reviews: 2
      dismiss_stale_reviews: true
  collaborators:
    - username: devops-team
      permission: admin
  teams:
    - team: backend-team
      permission: write
  webhooks:
    - url: https://ci.example.com/webhook
      events:
        - push
        - pull_request
      active: true
repositories:
  - name: service-a
    description: Service A with defaults
  - name: service-b
    description: Service B with overrides
    private: false
    topics:
      - python
      - web
    branch_protection:
      - pattern: main
        required_reviews: 3
`
	err := os.WriteFile(configFile, []byte(config), 0644)
	require.NoError(t, err)

	// Test that multi-repository configuration with defaults loads correctly
	multiConfig, err := github.LoadMultiRepositoryConfigFromFile(configFile)
	assert.NoError(t, err)
	assert.NotNil(t, multiConfig.Defaults)
	assert.True(t, *multiConfig.Defaults.Private)
	assert.Len(t, multiConfig.Defaults.Topics, 2)
	assert.Len(t, multiConfig.Defaults.BranchRules, 1)
	assert.Len(t, multiConfig.Defaults.Collaborators, 1)
	assert.Len(t, multiConfig.Defaults.Teams, 1)
	assert.Len(t, multiConfig.Defaults.Webhooks, 1)
	assert.Len(t, multiConfig.Repositories, 2)
}
