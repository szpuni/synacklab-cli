//go:build integration
// +build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGitHubMultiRepositoryBasicFunctionality tests basic multi-repository functionality
func TestGitHubMultiRepositoryBasicFunctionality(t *testing.T) {
	binaryPath := getBinaryPath(t)
	tempDir := t.TempDir()

	// Create multi-repository test configurations
	multiRepoConfigs := createBasicMultiRepoTestConfigs(t, tempDir)

	tests := []struct {
		name        string
		configFile  string
		args        []string
		expectError bool
		contains    []string
		description string
	}{
		{
			name:        "multi_repo_apply_requires_auth",
			configFile:  multiRepoConfigs["valid-multi"],
			args:        []string{"github", "apply", "", "--dry-run", "--owner", "test-org"},
			expectError: true,
			contains: []string{
				"Authentication failed",
				"no GitHub token found",
			},
			description: "Should require authentication for multi-repo operations",
		},
		{
			name:        "multi_repo_missing_owner",
			configFile:  multiRepoConfigs["valid-multi"],
			args:        []string{"github", "apply", "", "--dry-run"},
			expectError: true,
			contains: []string{
				"repository owner not specified",
			},
			description: "Should require owner for multi-repository operations",
		},
		{
			name:        "multi_repo_invalid_config",
			configFile:  multiRepoConfigs["invalid-multi"],
			args:        []string{"github", "apply", "", "--dry-run", "--owner", "test-org"},
			expectError: true,
			contains: []string{
				"configuration validation failed",
			},
			description: "Should fail validation for invalid multi-repository configuration",
		},
		{
			name:        "single_repo_backward_compatibility",
			configFile:  multiRepoConfigs["single-repo"],
			args:        []string{"github", "apply", "", "--dry-run", "--owner", "test-org"},
			expectError: true,
			contains: []string{
				"Authentication failed",
				"no GitHub token found",
			},
			description: "Should require authentication for single repository format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace placeholder with actual config file path
			args := make([]string, len(tt.args))
			copy(args, tt.args)
			for i, arg := range args {
				if arg == "" && i > 0 && args[i-1] == "apply" {
					args[i] = tt.configFile
				}
			}

			cmd := exec.Command(binaryPath, args...)
			cmd.Env = removeEnvVar(os.Environ(), "GITHUB_TOKEN") // Ensure no token

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			t.Logf("Test: %s", tt.description)
			t.Logf("Command: %s %v", binaryPath, args)
			t.Logf("Exit code: %v", err)
			t.Logf("Output: %s", outputStr)

			if tt.expectError && err == nil {
				t.Error("Expected command to fail, but it succeeded")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected command to succeed, but it failed: %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nFull output: %s", expected, outputStr)
				}
			}
		})
	}
}

// TestGitHubMultiRepositoryValidationIntegration tests the validate command with multi-repository configurations
func TestGitHubMultiRepositoryValidationIntegration(t *testing.T) {
	binaryPath := getBinaryPath(t)
	tempDir := t.TempDir()

	// Create multi-repository test configurations
	multiRepoConfigs := createBasicMultiRepoTestConfigs(t, tempDir)

	tests := []struct {
		name        string
		configFile  string
		args        []string
		expectError bool
		contains    []string
		description string
	}{
		{
			name:       "multi_repo_validate_valid_config",
			configFile: multiRepoConfigs["valid-multi"],
			args:       []string{"github", "validate", ""},
			contains: []string{
				"Configuration format: multi-repository",
				"Validating 3 repositories",
				"Configuration file is valid",
			},
			description: "Should validate valid multi-repository configuration",
		},
		{
			name:       "multi_repo_validate_with_repos_filter",
			configFile: multiRepoConfigs["valid-multi"],
			args:       []string{"github", "validate", "", "--repos", "service-a,service-b"},
			contains: []string{
				"Configuration format: multi-repository",
				"Validating 2 selected repositories from 3 total repositories",
				"Selected repositories: service-a, service-b",
				"Configuration file is valid",
			},
			description: "Should validate selected repositories from multi-repository configuration",
		},
		{
			name:        "multi_repo_validate_invalid_filter",
			configFile:  multiRepoConfigs["valid-multi"],
			args:        []string{"github", "validate", "", "--repos", "nonexistent-repo"},
			expectError: true,
			contains: []string{
				"repositories not found in configuration: nonexistent-repo",
			},
			description: "Should fail when filtering for non-existent repositories",
		},
		{
			name:        "multi_repo_validate_invalid_config",
			configFile:  multiRepoConfigs["invalid-multi"],
			args:        []string{"github", "validate", ""},
			expectError: true,
			contains: []string{
				"configuration validation failed",
				"duplicate repository name",
			},
			description: "Should fail validation for invalid multi-repository configuration",
		},
		{
			name:       "single_repo_validate_backward_compatibility",
			configFile: multiRepoConfigs["single-repo"],
			args:       []string{"github", "validate", ""},
			contains: []string{
				"Configuration format: single-repository",
				"Validating single repository: test-repo",
				"Configuration file is valid",
			},
			description: "Should maintain backward compatibility with single repository format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace placeholder with actual config file path
			args := make([]string, len(tt.args))
			copy(args, tt.args)
			for i, arg := range args {
				if arg == "" && i > 0 && args[i-1] == "validate" {
					args[i] = tt.configFile
				}
			}

			cmd := exec.Command(binaryPath, args...)
			cmd.Env = removeEnvVar(os.Environ(), "GITHUB_TOKEN") // Ensure no token for offline validation

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			t.Logf("Test: %s", tt.description)
			t.Logf("Command: %s %v", binaryPath, args)
			t.Logf("Exit code: %v", err)
			t.Logf("Output: %s", outputStr)

			if tt.expectError && err == nil {
				t.Error("Expected command to fail, but it succeeded")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected command to succeed, but it failed: %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nFull output: %s", expected, outputStr)
				}
			}
		})
	}
}

// TestGitHubMultiRepositoryErrorHandlingIntegration tests error handling scenarios
func TestGitHubMultiRepositoryErrorHandlingIntegration(t *testing.T) {
	binaryPath := getBinaryPath(t)
	tempDir := t.TempDir()

	// Create test configurations for error scenarios
	errorConfigs := createErrorTestConfigs(t, tempDir)

	tests := []struct {
		name        string
		configFile  string
		args        []string
		expectError bool
		contains    []string
		description string
	}{
		{
			name:        "malformed_multi_repo_yaml",
			configFile:  errorConfigs["malformed-yaml"],
			args:        []string{"github", "validate", ""},
			expectError: true,
			contains: []string{
				"failed to parse YAML",
			},
			description: "Should handle malformed YAML gracefully",
		},
		{
			name:        "empty_repositories_array",
			configFile:  errorConfigs["empty-repos"],
			args:        []string{"github", "validate", ""},
			expectError: true,
			contains: []string{
				"configuration validation failed",
			},
			description: "Should fail when repositories array is empty",
		},
		{
			name:        "invalid_repository_names",
			configFile:  errorConfigs["invalid-names"],
			args:        []string{"github", "validate", ""},
			expectError: true,
			contains: []string{
				"configuration validation failed",
				"repository name cannot start or end with a period",
			},
			description: "Should fail validation for invalid repository names",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace placeholder with actual config file path
			args := make([]string, len(tt.args))
			copy(args, tt.args)
			for i, arg := range args {
				if arg == "" && i > 0 && args[i-1] == "validate" {
					args[i] = tt.configFile
				}
			}

			cmd := exec.Command(binaryPath, args...)
			cmd.Env = removeEnvVar(os.Environ(), "GITHUB_TOKEN")

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			t.Logf("Test: %s", tt.description)
			t.Logf("Command: %s %v", binaryPath, args)
			t.Logf("Exit code: %v", err)
			t.Logf("Output: %s", outputStr)

			if tt.expectError && err == nil {
				t.Error("Expected command to fail, but it succeeded")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected command to succeed, but it failed: %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nFull output: %s", expected, outputStr)
				}
			}
		})
	}
}

// createBasicMultiRepoTestConfigs creates basic test configuration files for multi-repository testing
func createBasicMultiRepoTestConfigs(t *testing.T, tempDir string) map[string]string {
	configs := map[string]string{
		"valid-multi":   filepath.Join(tempDir, "valid-multi.yaml"),
		"invalid-multi": filepath.Join(tempDir, "invalid-multi.yaml"),
		"single-repo":   filepath.Join(tempDir, "single-repo.yaml"),
	}

	// Valid multi-repository configuration
	validMultiConfig := `version: "1.0"
repositories:
  - name: service-a
    description: Service A for integration testing
    private: true
    topics:
      - golang
      - api
    features:
      issues: true
      wiki: false

  - name: service-b
    description: Service B for integration testing
    private: false
    topics:
      - python
      - web
    features:
      issues: true
      wiki: true

  - name: frontend-app
    description: Frontend Application for integration testing
    private: true
    topics:
      - react
      - frontend
    features:
      issues: true
      wiki: false
`

	// Invalid multi-repository configuration (duplicate names)
	invalidMultiConfig := `version: "1.0"
repositories:
  - name: service-a
    description: First service A
    private: true
  - name: service-a
    description: Duplicate service A
    private: false
  - name: ""
    description: Empty name repository
`

	// Single repository configuration for backward compatibility
	singleRepoConfig := `name: test-repo
description: Single repository configuration for backward compatibility testing
private: true
topics:
  - testing
  - single
features:
  issues: true
  wiki: false
`

	// Write test files
	testConfigs := map[string]string{
		"valid-multi":   validMultiConfig,
		"invalid-multi": invalidMultiConfig,
		"single-repo":   singleRepoConfig,
	}

	for key, content := range testConfigs {
		if err := os.WriteFile(configs[key], []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test config %s: %v", key, err)
		}
	}

	return configs
}

// createErrorTestConfigs creates test configuration files for error scenario testing
func createErrorTestConfigs(t *testing.T, tempDir string) map[string]string {
	configs := map[string]string{
		"malformed-yaml": filepath.Join(tempDir, "malformed.yaml"),
		"empty-repos":    filepath.Join(tempDir, "empty-repos.yaml"),
		"invalid-names":  filepath.Join(tempDir, "invalid-names.yaml"),
	}

	// Malformed YAML
	malformedYaml := `version: "1.0"
repositories:
  - name: test-repo
    description: This YAML is malformed
    private: true
    topics:
      - testing
    features:
      issues: true
      wiki: false
    # This line will cause YAML parsing to fail
    invalid_yaml_structure: [unclosed bracket`

	// Empty repositories array
	emptyReposConfig := `version: "1.0"
repositories: []
`

	// Invalid repository names
	invalidNamesConfig := `version: "1.0"
repositories:
  - name: ".invalid-start"
    description: Repository name starting with dot
  - name: "invalid-end."
    description: Repository name ending with dot
`

	// Write test files
	testConfigs := map[string]string{
		"malformed-yaml": malformedYaml,
		"empty-repos":    emptyReposConfig,
		"invalid-names":  invalidNamesConfig,
	}

	for key, content := range testConfigs {
		if err := os.WriteFile(configs[key], []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create error test config %s: %v", key, err)
		}
	}

	return configs
}
