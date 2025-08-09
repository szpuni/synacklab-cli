//go:build integration && github_e2e
// +build integration,github_e2e

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
)

// TestGitHubE2EApply tests end-to-end apply functionality with a real GitHub organization
// This test requires:
// - GITHUB_TOKEN environment variable with appropriate permissions
// - GITHUB_TEST_ORG environment variable with test organization name
// - The token must have admin access to the test organization
func TestGitHubE2EApply(t *testing.T) {
	// Skip if not running E2E tests
	if os.Getenv("GITHUB_E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Set GITHUB_E2E_TESTS=true to run.")
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set, skipping E2E tests")
	}

	testOrg := os.Getenv("GITHUB_TEST_ORG")
	if testOrg == "" {
		t.Skip("GITHUB_TEST_ORG not set, skipping E2E tests")
	}

	binaryPath := getBinaryPath(t)

	// Create a unique test repository name
	timestamp := time.Now().Unix()
	testRepoName := fmt.Sprintf("synacklab-test-%d", timestamp)

	// Create test configuration
	testConfig := createE2ETestConfig(t, testRepoName)
	defer os.Remove(testConfig)

	// Ensure cleanup of test repository
	defer func() {
		cleanupTestRepository(t, token, testOrg, testRepoName)
	}()

	t.Run("dry-run shows planned changes", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "github", "apply", testConfig, "--dry-run", "--owner", testOrg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Dry-run output: %s", outputStr)

		if err != nil {
			t.Fatalf("Dry-run failed: %v\nOutput: %s", err, outputStr)
		}

		expectedContents := []string{
			"Dry-run mode",
			"CREATE new repository",
			testRepoName,
			"No changes were applied",
		}

		for _, expected := range expectedContents {
			if !strings.Contains(outputStr, expected) {
				t.Errorf("Expected dry-run output to contain %q, but it didn't", expected)
			}
		}
	})

	t.Run("apply creates repository", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "github", "apply", testConfig, "--owner", testOrg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Apply output: %s", outputStr)

		if err != nil {
			t.Fatalf("Apply failed: %v\nOutput: %s", err, outputStr)
		}

		expectedContents := []string{
			"Successfully applied changes",
			"Repository created",
			fmt.Sprintf("https://github.com/%s/%s", testOrg, testRepoName),
		}

		for _, expected := range expectedContents {
			if !strings.Contains(outputStr, expected) {
				t.Errorf("Expected apply output to contain %q, but it didn't", expected)
			}
		}

		// Verify repository was actually created
		verifyRepositoryExists(t, token, testOrg, testRepoName)
	})

	t.Run("second apply shows no changes needed", func(t *testing.T) {
		// Wait a moment to ensure repository is fully created
		time.Sleep(2 * time.Second)

		cmd := exec.Command(binaryPath, "github", "apply", testConfig, "--owner", testOrg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Second apply output: %s", outputStr)

		if err != nil {
			t.Fatalf("Second apply failed: %v\nOutput: %s", err, outputStr)
		}

		if !strings.Contains(outputStr, "already up to date") {
			t.Errorf("Expected second apply to show no changes needed, but got: %s", outputStr)
		}
	})
}

// TestGitHubE2EValidate tests end-to-end validate functionality
func TestGitHubE2EValidate(t *testing.T) {
	// Skip if not running E2E tests
	if os.Getenv("GITHUB_E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Set GITHUB_E2E_TESTS=true to run.")
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set, skipping E2E tests")
	}

	testOrg := os.Getenv("GITHUB_TEST_ORG")
	if testOrg == "" {
		t.Skip("GITHUB_TEST_ORG not set, skipping E2E tests")
	}

	binaryPath := getBinaryPath(t)

	// Create test configuration with valid users/teams
	testConfig := createE2EValidationTestConfig(t)
	defer os.Remove(testConfig)

	t.Run("validate with GitHub API checks", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "github", "validate", testConfig, "--owner", testOrg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Validate output: %s", outputStr)

		if err != nil {
			// Validation might fail due to non-existent users/teams, which is expected
			// We're mainly testing that the command structure works
			if !strings.Contains(outputStr, "GitHub API validation failed") {
				t.Fatalf("Unexpected validation error: %v\nOutput: %s", err, outputStr)
			}
		}

		expectedContents := []string{
			"Validating configuration file",
			"YAML syntax and basic validation passed",
			"Authenticated as",
			"Performing GitHub API validation",
		}

		for _, expected := range expectedContents {
			if !strings.Contains(outputStr, expected) {
				t.Errorf("Expected validate output to contain %q, but it didn't", expected)
			}
		}
	})
}

// TestGitHubE2ERepositoryUpdate tests updating an existing repository
func TestGitHubE2ERepositoryUpdate(t *testing.T) {
	// Skip if not running E2E tests
	if os.Getenv("GITHUB_E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Set GITHUB_E2E_TESTS=true to run.")
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set, skipping E2E tests")
	}

	testOrg := os.Getenv("GITHUB_TEST_ORG")
	if testOrg == "" {
		t.Skip("GITHUB_TEST_ORG not set, skipping E2E tests")
	}

	binaryPath := getBinaryPath(t)

	// Create a unique test repository name
	timestamp := time.Now().Unix()
	testRepoName := fmt.Sprintf("synacklab-update-test-%d", timestamp)

	// Create initial test configuration
	initialConfig := createE2ETestConfig(t, testRepoName)
	defer os.Remove(initialConfig)

	// Create updated test configuration
	updatedConfig := createE2EUpdatedTestConfig(t, testRepoName)
	defer os.Remove(updatedConfig)

	// Ensure cleanup of test repository
	defer func() {
		cleanupTestRepository(t, token, testOrg, testRepoName)
	}()

	t.Run("create initial repository", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "github", "apply", initialConfig, "--owner", testOrg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Initial repository creation failed: %v\nOutput: %s", err, output)
		}

		// Verify repository was created
		verifyRepositoryExists(t, token, testOrg, testRepoName)
	})

	t.Run("update repository shows changes", func(t *testing.T) {
		// Wait a moment to ensure repository is fully created
		time.Sleep(2 * time.Second)

		cmd := exec.Command(binaryPath, "github", "apply", updatedConfig, "--dry-run", "--owner", testOrg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Update dry-run output: %s", outputStr)

		if err != nil {
			t.Fatalf("Update dry-run failed: %v\nOutput: %s", err, outputStr)
		}

		expectedContents := []string{
			"Dry-run mode",
			"UPDATE repository settings",
			"Description:",
		}

		for _, expected := range expectedContents {
			if !strings.Contains(outputStr, expected) {
				t.Errorf("Expected update dry-run output to contain %q, but it didn't", expected)
			}
		}
	})

	t.Run("apply repository update", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "github", "apply", updatedConfig, "--owner", testOrg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Update apply output: %s", outputStr)

		if err != nil {
			t.Fatalf("Update apply failed: %v\nOutput: %s", err, outputStr)
		}

		if !strings.Contains(outputStr, "Successfully applied changes") {
			t.Errorf("Expected successful update, but got: %s", outputStr)
		}
	})
}

// createE2ETestConfig creates a test configuration file for E2E testing
func createE2ETestConfig(t *testing.T, repoName string) string {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "e2e-test-config.yaml")

	config := fmt.Sprintf(`name: %s
description: End-to-end test repository created by synacklab integration tests
private: true
topics:
  - testing
  - synacklab
  - integration
features:
  issues: true
  wiki: false
  projects: false
  discussions: false
branch_protection:
  - pattern: main
    required_reviews: 1
    dismiss_stale_reviews: true
    require_code_owner_review: false
    required_status_checks: []
    require_up_to_date: false
`, repoName)

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create E2E test config: %v", err)
	}

	return configPath
}

// createE2EUpdatedTestConfig creates an updated test configuration for testing updates
func createE2EUpdatedTestConfig(t *testing.T, repoName string) string {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "e2e-updated-test-config.yaml")

	config := fmt.Sprintf(`name: %s
description: Updated end-to-end test repository - description changed for testing updates
private: true
topics:
  - testing
  - synacklab
  - integration
  - updated
features:
  issues: true
  wiki: true
  projects: false
  discussions: false
branch_protection:
  - pattern: main
    required_reviews: 2
    dismiss_stale_reviews: true
    require_code_owner_review: false
    required_status_checks: []
    require_up_to_date: false
`, repoName)

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create E2E updated test config: %v", err)
	}

	return configPath
}

// createE2EValidationTestConfig creates a test configuration for validation testing
func createE2EValidationTestConfig(t *testing.T) string {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "e2e-validation-test-config.yaml")

	config := `name: validation-test-repo
description: Repository for testing validation functionality
private: true
topics:
  - testing
  - validation
features:
  issues: true
  wiki: false
  projects: false
  discussions: false
collaborators:
  - username: nonexistent-user-12345
    permission: read
teams:
  - team: nonexistent-team
    permission: write
webhooks:
  - url: https://example.com/webhook
    events:
      - push
      - pull_request
    active: true
`

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create E2E validation test config: %v", err)
	}

	return configPath
}

// verifyRepositoryExists verifies that a repository exists using the GitHub API
func verifyRepositoryExists(t *testing.T, token, owner, repoName string) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	repo, _, err := client.Repositories.Get(ctx, owner, repoName)
	if err != nil {
		t.Fatalf("Failed to verify repository exists: %v", err)
	}

	if repo.GetName() != repoName {
		t.Errorf("Repository name mismatch: expected %s, got %s", repoName, repo.GetName())
	}

	t.Logf("✓ Verified repository exists: %s/%s", owner, repoName)
}

// cleanupTestRepository removes a test repository
func cleanupTestRepository(t *testing.T, token, owner, repoName string) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// Check if repository exists before trying to delete
	_, resp, err := client.Repositories.Get(ctx, owner, repoName)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			// Repository doesn't exist, nothing to clean up
			t.Logf("Repository %s/%s doesn't exist, no cleanup needed", owner, repoName)
			return
		}
		t.Logf("Warning: Failed to check if repository exists for cleanup: %v", err)
		return
	}

	// Repository exists, delete it
	_, err = client.Repositories.Delete(ctx, owner, repoName)
	if err != nil {
		t.Logf("Warning: Failed to cleanup test repository %s/%s: %v", owner, repoName, err)
		return
	}

	t.Logf("✓ Cleaned up test repository: %s/%s", owner, repoName)
}
