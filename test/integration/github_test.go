//go:build integration
// +build integration

package integration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGitHubCommandStructure tests the basic GitHub command structure and help
func TestGitHubCommandStructure(t *testing.T) {
	binaryPath := getBinaryPath(t)

	tests := []struct {
		name     string
		args     []string
		contains []string
	}{
		{
			name: "github help",
			args: []string{"github", "--help"},
			contains: []string{
				"Commands for managing GitHub repositories",
				"apply",
				"validate",
				"YAML configuration files",
			},
		},
		{
			name: "github apply help",
			args: []string{"github", "apply", "--help"},
			contains: []string{
				"Apply repository configuration from a YAML file to GitHub",
				"--dry-run",
				"--owner",
				"Preview changes without applying them",
			},
		},
		{
			name: "github validate help",
			args: []string{"github", "validate", "--help"},
			contains: []string{
				"Validate a repository configuration file for syntax and logical errors",
				"--owner",
				"syntax errors",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.args...)
			output, err := cmd.CombinedOutput()

			if err != nil {
				t.Fatalf("Help command failed: %v\nOutput: %s", err, output)
			}

			outputStr := string(output)
			for _, expected := range tt.contains {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nFull output: %s", expected, outputStr)
				}
			}
		})
	}
}

// TestGitHubApplyDryRun tests the apply command with dry-run functionality
func TestGitHubApplyDryRun(t *testing.T) {
	binaryPath := getBinaryPath(t)

	// Create test configuration files
	testConfigs := createTestConfigs(t)
	defer cleanupTestConfigs(testConfigs)

	tests := []struct {
		name        string
		configFile  string
		args        []string
		expectError bool
		contains    []string
		notContains []string
	}{
		{
			name:       "valid config dry-run without auth",
			configFile: testConfigs["valid"],
			args:       []string{"github", "apply", "", "--dry-run", "--owner", "test-org"},
			contains: []string{
				"Authentication failed",
				"no GitHub token found",
			},
			notContains: []string{
				"Successfully applied",
			},
		},
		{
			name:        "missing config file",
			configFile:  "nonexistent.yaml",
			args:        []string{"github", "apply", "", "--dry-run"},
			expectError: true,
			contains: []string{
				"failed to load repository config",
			},
		},
		{
			name:        "invalid config file",
			configFile:  testConfigs["invalid"],
			args:        []string{"github", "apply", "", "--dry-run", "--owner", "test-org"},
			expectError: true,
			contains: []string{
				"configuration validation failed",
			},
		},
		{
			name:        "missing owner",
			configFile:  testConfigs["valid"],
			args:        []string{"github", "apply", "", "--dry-run"},
			expectError: true,
			contains: []string{
				"repository owner not specified",
			},
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
			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			t.Logf("Command: %s %v", binaryPath, args)
			t.Logf("Exit code: %v", err)
			t.Logf("Output: %s", outputStr)

			if tt.expectError && err == nil {
				t.Error("Expected command to fail, but it succeeded")
			}

			if !tt.expectError && err != nil {
				// For dry-run without authentication, we might get auth errors
				// but the command structure should still work
				if !strings.Contains(outputStr, "Authentication failed") &&
					!strings.Contains(outputStr, "failed to load synacklab config") &&
					!strings.Contains(outputStr, "no GitHub token found") {
					t.Errorf("Unexpected error: %v\nOutput: %s", err, outputStr)
				}
			}

			for _, expected := range tt.contains {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nFull output: %s", expected, outputStr)
				}
			}

			for _, notExpected := range tt.notContains {
				if strings.Contains(outputStr, notExpected) {
					t.Errorf("Expected output to NOT contain %q, but it did.\nFull output: %s", notExpected, outputStr)
				}
			}
		})
	}
}

// TestGitHubValidate tests the validate command functionality
func TestGitHubValidate(t *testing.T) {
	binaryPath := getBinaryPath(t)

	// Create test configuration files
	testConfigs := createTestConfigs(t)
	defer cleanupTestConfigs(testConfigs)

	tests := []struct {
		name        string
		configFile  string
		args        []string
		expectError bool
		contains    []string
	}{
		{
			name:       "valid config validation",
			configFile: testConfigs["valid"],
			args:       []string{"github", "validate", ""},
			contains: []string{
				"Validating configuration file",
				"YAML syntax and basic validation passed",
			},
		},
		{
			name:        "invalid config validation",
			configFile:  testConfigs["invalid"],
			args:        []string{"github", "validate", ""},
			expectError: true,
			contains: []string{
				"configuration validation failed",
			},
		},
		{
			name:        "missing config file",
			configFile:  "nonexistent.yaml",
			args:        []string{"github", "validate", ""},
			expectError: true,
			contains: []string{
				"failed to read config file",
			},
		},
		{
			name:        "malformed yaml",
			configFile:  testConfigs["malformed"],
			args:        []string{"github", "validate", ""},
			expectError: true,
			contains: []string{
				"failed to parse YAML",
			},
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
			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			t.Logf("Command: %s %v", binaryPath, args)
			t.Logf("Exit code: %v", err)
			t.Logf("Output: %s", outputStr)

			if tt.expectError && err == nil {
				t.Error("Expected command to fail, but it succeeded")
			}

			if !tt.expectError && err != nil {
				// For validation without authentication, we might get auth warnings
				// but basic validation should still work
				if !strings.Contains(outputStr, "Could not load synacklab config") &&
					!strings.Contains(outputStr, "GitHub authentication failed") &&
					!strings.Contains(outputStr, "Configuration file is valid") {
					t.Errorf("Unexpected error: %v\nOutput: %s", err, outputStr)
				}
			}

			for _, expected := range tt.contains {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nFull output: %s", expected, outputStr)
				}
			}
		})
	}
}

// TestGitHubAuthenticationFlow tests authentication scenarios
func TestGitHubAuthenticationFlow(t *testing.T) {
	binaryPath := getBinaryPath(t)
	testConfigs := createTestConfigs(t)
	defer cleanupTestConfigs(testConfigs)

	tests := []struct {
		name        string
		envVars     map[string]string
		args        []string
		contains    []string
		description string
	}{
		{
			name: "no authentication",
			envVars: map[string]string{
				"GITHUB_TOKEN": "",
			},
			args: []string{"github", "apply", testConfigs["valid"], "--dry-run", "--owner", "test-org"},
			contains: []string{
				"Authentication failed",
				"GITHUB_TOKEN environment variable",
			},
			description: "Should show authentication instructions when no token is provided",
		},
		{
			name: "invalid token format",
			envVars: map[string]string{
				"GITHUB_TOKEN": "invalid-token",
			},
			args: []string{"github", "validate", testConfigs["valid"], "--owner", "test-org"},
			contains: []string{
				"GitHub authentication failed",
				"Skipping GitHub API validation",
			},
			description: "Should handle invalid token gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.args...)

			// Set up environment variables
			env := os.Environ()
			for key, value := range tt.envVars {
				if value == "" {
					// Remove the environment variable
					env = removeEnvVar(env, key)
				} else {
					env = append(env, fmt.Sprintf("%s=%s", key, value))
				}
			}
			cmd.Env = env

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			t.Logf("Test: %s", tt.description)
			t.Logf("Command: %s %v", binaryPath, tt.args)
			t.Logf("Environment: %v", tt.envVars)
			t.Logf("Exit code: %v", err)
			t.Logf("Output: %s", outputStr)

			for _, expected := range tt.contains {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nFull output: %s", expected, outputStr)
				}
			}
		})
	}
}

// TestGitHubErrorScenarios tests various error scenarios
func TestGitHubErrorScenarios(t *testing.T) {
	binaryPath := getBinaryPath(t)

	tests := []struct {
		name        string
		args        []string
		expectError bool
		contains    []string
		description string
	}{
		{
			name:        "apply without arguments",
			args:        []string{"github", "apply"},
			expectError: true,
			contains: []string{
				"accepts 1 arg(s), received 0",
			},
			description: "Should require config file argument",
		},
		{
			name:        "validate without arguments",
			args:        []string{"github", "validate"},
			expectError: true,
			contains: []string{
				"accepts 1 arg(s), received 0",
			},
			description: "Should require config file argument",
		},
		{
			name:        "apply with too many arguments",
			args:        []string{"github", "apply", "config1.yaml", "config2.yaml"},
			expectError: true,
			contains: []string{
				"accepts 1 arg(s), received 2",
			},
			description: "Should reject multiple config files",
		},
		{
			name:        "validate with too many arguments",
			args:        []string{"github", "validate", "config1.yaml", "config2.yaml"},
			expectError: true,
			contains: []string{
				"accepts 1 arg(s), received 2",
			},
			description: "Should reject multiple config files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.args...)
			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			t.Logf("Test: %s", tt.description)
			t.Logf("Command: %s %v", binaryPath, tt.args)
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

// getBinaryPath returns the path to the synacklab binary for testing
func getBinaryPath(t *testing.T) string {
	// Use pre-built binary from CI or build locally
	binaryPath := os.Getenv("SYNACKLAB_BINARY")
	if binaryPath == "" {
		// Build the binary locally for local testing
		buildCmd := exec.Command("go", "build", "-o", "synacklab-test", "./cmd/synacklab")
		buildCmd.Dir = getProjectRoot()
		var buildOut bytes.Buffer
		buildCmd.Stdout = &buildOut
		buildCmd.Stderr = &buildOut
		err := buildCmd.Run()
		if err != nil {
			t.Fatalf("Failed to build binary: %v\nOutput: %s", err, buildOut.String())
		}
		binaryPath = filepath.Join(getProjectRoot(), "synacklab-test")

		// Schedule cleanup
		t.Cleanup(func() {
			if err := os.Remove(binaryPath); err != nil {
				t.Logf("Failed to remove test binary: %v", err)
			}
		})
	} else {
		// Convert relative path to absolute path from project root
		if !filepath.IsAbs(binaryPath) {
			projectRoot := getProjectRoot()
			binaryPath = filepath.Join(projectRoot, binaryPath)
		}
	}

	return binaryPath
}

// createTestConfigs creates temporary test configuration files
func createTestConfigs(t *testing.T) map[string]string {
	tempDir := t.TempDir()

	configs := map[string]string{
		"valid":     filepath.Join(tempDir, "valid-repo.yaml"),
		"invalid":   filepath.Join(tempDir, "invalid-repo.yaml"),
		"malformed": filepath.Join(tempDir, "malformed-repo.yaml"),
	}

	// Valid configuration
	validConfig := `name: test-repository
description: A test repository for integration testing
private: true
topics:
  - testing
  - integration
  - golang
features:
  issues: true
  wiki: false
  projects: true
  discussions: false
branch_protection:
  - pattern: main
    required_reviews: 2
    dismiss_stale_reviews: true
    require_code_owner_review: true
    required_status_checks:
      - ci/build
      - ci/test
    require_up_to_date: true
collaborators:
  - username: test-user
    permission: write
teams:
  - team: developers
    permission: write
  - team: admins
    permission: admin
webhooks:
  - url: https://example.com/webhook
    events:
      - push
      - pull_request
    active: true
`

	// Invalid configuration (invalid repository name)
	invalidConfig := `name: .invalid-repo-name.
description: This has an invalid repository name
private: true
collaborators:
  - username: ""
    permission: invalid-permission
branch_protection:
  - pattern: ""
    required_reviews: 10
webhooks:
  - url: not-a-valid-url
    events: []
`

	// Malformed YAML
	malformedConfig := `name: test-repo
description: This YAML is malformed
private: true
collaborators:
  - username: test-user
    permission: write
  - username: another-user
    # Missing permission field and invalid indentation
  permission: read
`

	// Write test files
	if err := os.WriteFile(configs["valid"], []byte(validConfig), 0644); err != nil {
		t.Fatalf("Failed to create valid test config: %v", err)
	}

	if err := os.WriteFile(configs["invalid"], []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("Failed to create invalid test config: %v", err)
	}

	if err := os.WriteFile(configs["malformed"], []byte(malformedConfig), 0644); err != nil {
		t.Fatalf("Failed to create malformed test config: %v", err)
	}

	return configs
}

// cleanupTestConfigs removes temporary test configuration files
func cleanupTestConfigs(configs map[string]string) {
	for _, configPath := range configs {
		if err := os.Remove(configPath); err != nil {
			// Don't fail the test for cleanup errors, just log them
			fmt.Printf("Warning: Failed to cleanup test config %s: %v\n", configPath, err)
		}
	}
}

// removeEnvVar removes an environment variable from the environment slice
func removeEnvVar(env []string, key string) []string {
	var result []string
	prefix := key + "="
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			result = append(result, e)
		}
	}
	return result
}

// TestGitHubMultiRepositoryValidation tests the enhanced validate command with multi-repository configurations
func TestGitHubMultiRepositoryValidation(t *testing.T) {
	binaryPath := getBinaryPath(t)
	tempDir := t.TempDir()

	// Create multi-repository test configurations
	multiRepoConfigs := createMultiRepoTestConfigs(t, tempDir)

	tests := []struct {
		name        string
		configFile  string
		args        []string
		expectError bool
		contains    []string
		description string
	}{
		{
			name:        "valid_multi_repo_config",
			configFile:  multiRepoConfigs["valid-multi"],
			args:        []string{"github", "validate"},
			expectError: false,
			contains: []string{
				"Configuration format: multi-repository",
				"Validating 3 repositories",
				"Configuration file is valid",
			},
			description: "Should validate valid multi-repository configuration",
		},
		{
			name:        "multi_repo_with_repos_filter",
			configFile:  multiRepoConfigs["valid-multi"],
			args:        []string{"github", "validate", "--repos", "service-a,service-b"},
			expectError: false,
			contains: []string{
				"Configuration format: multi-repository",
				"Validating 2 selected repositories from 3 total repositories",
				"Selected repositories: service-a, service-b",
				"Configuration file is valid",
			},
			description: "Should validate selected repositories from multi-repository configuration",
		},
		{
			name:        "multi_repo_with_invalid_filter",
			configFile:  multiRepoConfigs["valid-multi"],
			args:        []string{"github", "validate", "--repos", "nonexistent-repo"},
			expectError: true,
			contains: []string{
				"repositories not found in configuration: nonexistent-repo",
			},
			description: "Should fail when filtering for non-existent repositories",
		},
		{
			name:        "invalid_multi_repo_config",
			configFile:  multiRepoConfigs["invalid-multi"],
			args:        []string{"github", "validate"},
			expectError: true,
			contains: []string{
				"validation failed",
				"duplicate repository name",
				"repository name is required",
			},
			description: "Should fail validation for invalid multi-repository configuration",
		},
		{
			name:        "multi_repo_with_defaults",
			configFile:  multiRepoConfigs["multi-with-defaults"],
			args:        []string{"github", "validate"},
			expectError: false,
			contains: []string{
				"Configuration format: multi-repository",
				"Validating 2 repositories",
				"Configuration file is valid",
			},
			description: "Should validate multi-repository configuration with defaults",
		},
		{
			name:        "single_repo_backward_compatibility",
			configFile:  multiRepoConfigs["single-repo"],
			args:        []string{"github", "validate"},
			expectError: false,
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
			args := append(tt.args, tt.configFile)
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

// createMultiRepoTestConfigs creates test configuration files for multi-repository testing
func createMultiRepoTestConfigs(t *testing.T, tempDir string) map[string]string {
	configs := map[string]string{
		"valid-multi":         filepath.Join(tempDir, "valid-multi.yaml"),
		"invalid-multi":       filepath.Join(tempDir, "invalid-multi.yaml"),
		"multi-with-defaults": filepath.Join(tempDir, "multi-defaults.yaml"),
		"single-repo":         filepath.Join(tempDir, "single-repo.yaml"),
	}

	// Valid multi-repository configuration
	validMultiConfig := `version: "1.0"
repositories:
  - name: service-a
    description: Service A
    private: true
    topics:
      - golang
      - api
      - microservice
    features:
      issues: true
      wiki: false
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
  - name: service-b
    description: Service B
    private: false
    topics:
      - python
      - web
    features:
      issues: true
      projects: true
    webhooks:
      - url: https://example.com/webhook
        events:
          - push
          - pull_request
        active: true
  - name: frontend-app
    description: Frontend Application
    private: true
    topics:
      - react
      - frontend
    features:
      issues: true
      wiki: true
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

	// Multi-repository configuration with defaults
	multiWithDefaultsConfig := `version: "1.0"
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
    - team: platform-team
      permission: write
repositories:
  - name: service-with-defaults
    description: Service using defaults
  - name: service-with-overrides
    description: Service with overrides
    private: false
    topics:
      - custom
      - override
    branch_protection:
      - pattern: main
        required_reviews: 3
`

	// Single repository configuration for backward compatibility
	singleRepoConfig := `name: test-repo
description: Single repository configuration
private: true
topics:
  - testing
  - single
features:
  issues: true
  wiki: false
branch_protection:
  - pattern: main
    required_reviews: 1
collaborators:
  - username: test-user
    permission: write
`

	// Write test files
	testConfigs := map[string]string{
		"valid-multi":         validMultiConfig,
		"invalid-multi":       invalidMultiConfig,
		"multi-with-defaults": multiWithDefaultsConfig,
		"single-repo":         singleRepoConfig,
	}

	for key, content := range testConfigs {
		if err := os.WriteFile(configs[key], []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test config %s: %v", key, err)
		}
	}

	return configs
}
