//go:build integration && github_e2e
// +build integration,github_e2e

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestGitHubMultiRepositoryE2EApply tests end-to-end multi-repository apply functionality
func TestGitHubMultiRepositoryE2EApply(t *testing.T) {
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

	// Create unique test repository names
	timestamp := time.Now().Unix()
	testRepoNames := []string{
		fmt.Sprintf("synacklab-multi-test-1-%d", timestamp),
		fmt.Sprintf("synacklab-multi-test-2-%d", timestamp),
		fmt.Sprintf("synacklab-multi-test-3-%d", timestamp),
	}

	// Create multi-repository test configuration
	testConfig := createE2EMultiRepoTestConfig(t, testRepoNames)
	defer os.Remove(testConfig)

	// Ensure cleanup of all test repositories
	defer func() {
		for _, repoName := range testRepoNames {
			cleanupTestRepository(t, token, testOrg, repoName)
		}
	}()

	t.Run("multi_repo_dry_run_shows_all_planned_changes", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "github", "apply", testConfig, "--dry-run", "--owner", testOrg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Multi-repo dry-run output: %s", outputStr)

		if err != nil {
			t.Fatalf("Multi-repo dry-run failed: %v\nOutput: %s", err, outputStr)
		}

		expectedContents := []string{
			"Configuration format: multi-repository",
			"Processing 3 repositories",
			"Dry-run mode",
			"CREATE new repository",
			testRepoNames[0],
			testRepoNames[1],
			testRepoNames[2],
			"No changes were applied",
		}

		for _, expected := range expectedContents {
			if !strings.Contains(outputStr, expected) {
				t.Errorf("Expected multi-repo dry-run output to contain %q, but it didn't", expected)
			}
		}
	})

	t.Run("multi_repo_selective_dry_run", func(t *testing.T) {
		selectedRepos := fmt.Sprintf("%s,%s", testRepoNames[0], testRepoNames[1])
		cmd := exec.Command(binaryPath, "github", "apply", testConfig, "--dry-run", "--owner", testOrg, "--repos", selectedRepos)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Multi-repo selective dry-run output: %s", outputStr)

		if err != nil {
			t.Fatalf("Multi-repo selective dry-run failed: %v\nOutput: %s", err, outputStr)
		}

		expectedContents := []string{
			"Configuration format: multi-repository",
			"Processing 2 selected repositories from 3 total repositories",
			fmt.Sprintf("Selected repositories: %s, %s", testRepoNames[0], testRepoNames[1]),
			"Dry-run mode",
			testRepoNames[0],
			testRepoNames[1],
			"No changes were applied",
		}

		notExpectedContents := []string{
			testRepoNames[2],
		}

		for _, expected := range expectedContents {
			if !strings.Contains(outputStr, expected) {
				t.Errorf("Expected selective dry-run output to contain %q, but it didn't", expected)
			}
		}

		for _, notExpected := range notExpectedContents {
			if strings.Contains(outputStr, notExpected) {
				t.Errorf("Expected selective dry-run output to NOT contain %q, but it did", notExpected)
			}
		}
	})

	t.Run("multi_repo_apply_creates_all_repositories", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "github", "apply", testConfig, "--owner", testOrg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Multi-repo apply output: %s", outputStr)

		if err != nil {
			t.Fatalf("Multi-repo apply failed: %v\nOutput: %s", err, outputStr)
		}

		expectedContents := []string{
			"Configuration format: multi-repository",
			"Processing 3 repositories",
			"Successfully applied changes",
			"Summary:",
			"✓ 3 repositories processed successfully",
			"✗ 0 repositories failed",
		}

		for _, expected := range expectedContents {
			if !strings.Contains(outputStr, expected) {
				t.Errorf("Expected multi-repo apply output to contain %q, but it didn't", expected)
			}
		}

		// Verify all repositories were actually created
		for _, repoName := range testRepoNames {
			verifyRepositoryExists(t, token, testOrg, repoName)
		}
	})

	t.Run("multi_repo_selective_apply", func(t *testing.T) {
		// First, delete one repository to test selective re-creation
		cleanupTestRepository(t, token, testOrg, testRepoNames[1])

		// Wait a moment for deletion to complete
		time.Sleep(2 * time.Second)

		selectedRepos := testRepoNames[1]
		cmd := exec.Command(binaryPath, "github", "apply", testConfig, "--owner", testOrg, "--repos", selectedRepos)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Multi-repo selective apply output: %s", outputStr)

		if err != nil {
			t.Fatalf("Multi-repo selective apply failed: %v\nOutput: %s", err, outputStr)
		}

		expectedContents := []string{
			"Configuration format: multi-repository",
			"Processing 1 selected repositories from 3 total repositories",
			fmt.Sprintf("Selected repositories: %s", testRepoNames[1]),
			"Successfully applied changes",
			"✓ 1 repositories processed successfully",
		}

		for _, expected := range expectedContents {
			if !strings.Contains(outputStr, expected) {
				t.Errorf("Expected selective apply output to contain %q, but it didn't", expected)
			}
		}

		// Verify the selected repository was re-created
		verifyRepositoryExists(t, token, testOrg, testRepoNames[1])
	})

	t.Run("multi_repo_second_apply_shows_no_changes", func(t *testing.T) {
		// Wait a moment to ensure repositories are fully created
		time.Sleep(3 * time.Second)

		cmd := exec.Command(binaryPath, "github", "apply", testConfig, "--owner", testOrg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Multi-repo second apply output: %s", outputStr)

		if err != nil {
			t.Fatalf("Multi-repo second apply failed: %v\nOutput: %s", err, outputStr)
		}

		expectedContents := []string{
			"Configuration format: multi-repository",
			"Processing 3 repositories",
			"Successfully applied changes",
			"already up to date",
		}

		for _, expected := range expectedContents {
			if !strings.Contains(outputStr, expected) {
				t.Errorf("Expected second apply output to contain %q, but it didn't", expected)
			}
		}
	})
}

// TestGitHubMultiRepositoryE2EValidate tests end-to-end multi-repository validate functionality
func TestGitHubMultiRepositoryE2EValidate(t *testing.T) {
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

	// Create test configuration with validation scenarios
	testConfig := createE2EMultiRepoValidationTestConfig(t)
	defer os.Remove(testConfig)

	t.Run("multi_repo_validate_with_github_api", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "github", "validate", testConfig, "--owner", testOrg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Multi-repo validate output: %s", outputStr)

		// Validation might fail due to non-existent users/teams, which is expected
		// We're mainly testing that the command structure works for multi-repo
		expectedContents := []string{
			"Configuration format: multi-repository",
			"Validating 3 repositories",
			"YAML syntax and basic validation passed",
			"Authenticated as",
			"Performing GitHub API validation",
		}

		for _, expected := range expectedContents {
			if !strings.Contains(outputStr, expected) {
				t.Errorf("Expected multi-repo validate output to contain %q, but it didn't", expected)
			}
		}
	})

	t.Run("multi_repo_selective_validate", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "github", "validate", testConfig, "--owner", testOrg, "--repos", "service-1,service-2")
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Multi-repo selective validate output: %s", outputStr)

		expectedContents := []string{
			"Configuration format: multi-repository",
			"Validating 2 selected repositories from 3 total repositories",
			"Selected repositories: service-1, service-2",
			"YAML syntax and basic validation passed",
			"Authenticated as",
		}

		for _, expected := range expectedContents {
			if !strings.Contains(outputStr, expected) {
				t.Errorf("Expected selective validate output to contain %q, but it didn't", expected)
			}
		}
	})
}

// TestGitHubMultiRepositoryE2EPartialFailure tests partial failure scenarios with real API
func TestGitHubMultiRepositoryE2EPartialFailure(t *testing.T) {
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

	// Create unique test repository names
	timestamp := time.Now().Unix()
	testRepoNames := []string{
		fmt.Sprintf("synacklab-partial-test-1-%d", timestamp),
		fmt.Sprintf("synacklab-partial-test-2-%d", timestamp),
	}

	// Create configuration with one valid and one invalid repository
	testConfig := createE2EPartialFailureTestConfig(t, testRepoNames)
	defer os.Remove(testConfig)

	// Ensure cleanup of test repositories
	defer func() {
		for _, repoName := range testRepoNames {
			cleanupTestRepository(t, token, testOrg, repoName)
		}
	}()

	t.Run("multi_repo_partial_failure_handling", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "github", "apply", testConfig, "--owner", testOrg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Multi-repo partial failure output: %s", outputStr)

		// We expect this to have a non-zero exit code due to partial failure
		if err == nil {
			t.Log("Expected partial failure, but command succeeded completely")
		}

		expectedContents := []string{
			"Configuration format: multi-repository",
			"Processing 2 repositories",
			"Summary:",
		}

		for _, expected := range expectedContents {
			if !strings.Contains(outputStr, expected) {
				t.Errorf("Expected partial failure output to contain %q, but it didn't", expected)
			}
		}

		// At least one repository should succeed
		if strings.Contains(outputStr, "✓ 0 repositories processed successfully") {
			t.Error("Expected at least one repository to succeed")
		}
	})
}

// TestGitHubMultiRepositoryE2EUpdate tests updating existing repositories in multi-repo config
func TestGitHubMultiRepositoryE2EUpdate(t *testing.T) {
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

	// Create unique test repository names
	timestamp := time.Now().Unix()
	testRepoNames := []string{
		fmt.Sprintf("synacklab-update-test-1-%d", timestamp),
		fmt.Sprintf("synacklab-update-test-2-%d", timestamp),
	}

	// Create initial and updated configurations
	initialConfig := createE2EMultiRepoInitialConfig(t, testRepoNames)
	updatedConfig := createE2EMultiRepoUpdatedConfig(t, testRepoNames)
	defer os.Remove(initialConfig)
	defer os.Remove(updatedConfig)

	// Ensure cleanup of test repositories
	defer func() {
		for _, repoName := range testRepoNames {
			cleanupTestRepository(t, token, testOrg, repoName)
		}
	}()

	t.Run("create_initial_multi_repo_configuration", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "github", "apply", initialConfig, "--owner", testOrg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Initial multi-repo creation output: %s", outputStr)

		if err != nil {
			t.Fatalf("Initial multi-repo creation failed: %v\nOutput: %s", err, outputStr)
		}

		// Verify repositories were created
		for _, repoName := range testRepoNames {
			verifyRepositoryExists(t, token, testOrg, repoName)
		}
	})

	t.Run("update_multi_repo_shows_changes", func(t *testing.T) {
		// Wait a moment to ensure repositories are fully created
		time.Sleep(3 * time.Second)

		cmd := exec.Command(binaryPath, "github", "apply", updatedConfig, "--dry-run", "--owner", testOrg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Multi-repo update dry-run output: %s", outputStr)

		if err != nil {
			t.Fatalf("Multi-repo update dry-run failed: %v\nOutput: %s", err, outputStr)
		}

		expectedContents := []string{
			"Configuration format: multi-repository",
			"Processing 2 repositories",
			"Dry-run mode",
			"UPDATE repository settings",
		}

		for _, expected := range expectedContents {
			if !strings.Contains(outputStr, expected) {
				t.Errorf("Expected update dry-run output to contain %q, but it didn't", expected)
			}
		}
	})

	t.Run("apply_multi_repo_updates", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "github", "apply", updatedConfig, "--owner", testOrg)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GITHUB_TOKEN=%s", token))

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		t.Logf("Multi-repo update apply output: %s", outputStr)

		if err != nil {
			t.Fatalf("Multi-repo update apply failed: %v\nOutput: %s", err, outputStr)
		}

		expectedContents := []string{
			"Configuration format: multi-repository",
			"Processing 2 repositories",
			"Successfully applied changes",
		}

		for _, expected := range expectedContents {
			if !strings.Contains(outputStr, expected) {
				t.Errorf("Expected update apply output to contain %q, but it didn't", expected)
			}
		}
	})
}

// createE2EMultiRepoTestConfig creates a multi-repository test configuration for E2E testing
func createE2EMultiRepoTestConfig(t *testing.T, repoNames []string) string {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "e2e-multi-repo-test-config.yaml")

	config := fmt.Sprintf(`version: "1.0"

defaults:
  private: true
  topics:
    - testing
    - synacklab
    - integration
    - multi-repo
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

repositories:
  - name: %s
    description: First test repository for multi-repo E2E testing
    topics:
      - golang
      - api
      - first

  - name: %s
    description: Second test repository for multi-repo E2E testing
    private: false
    topics:
      - python
      - web
      - second
    features:
      issues: true
      wiki: true

  - name: %s
    description: Third test repository for multi-repo E2E testing
    topics:
      - react
      - frontend
      - third
    branch_protection:
      - pattern: main
        required_reviews: 2
        dismiss_stale_reviews: true
`, repoNames[0], repoNames[1], repoNames[2])

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create E2E multi-repo test config: %v", err)
	}

	return configPath
}

// createE2EMultiRepoValidationTestConfig creates a multi-repository configuration for validation testing
func createE2EMultiRepoValidationTestConfig(t *testing.T) string {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "e2e-multi-repo-validation-config.yaml")

	config := `version: "1.0"

defaults:
  private: true
  topics:
    - testing
    - validation
  features:
    issues: true
    wiki: false
  collaborators:
    - username: nonexistent-user-12345
      permission: read
  teams:
    - team: nonexistent-team
      permission: write

repositories:
  - name: service-1
    description: First service for validation testing
    topics:
      - golang
      - service

  - name: service-2
    description: Second service for validation testing
    topics:
      - python
      - service
    collaborators:
      - username: another-nonexistent-user
        permission: write

  - name: service-3
    description: Third service for validation testing
    topics:
      - nodejs
      - service
    teams:
      - team: another-nonexistent-team
        permission: admin
    webhooks:
      - url: https://example.com/webhook
        events:
          - push
          - pull_request
        active: true
`

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create E2E multi-repo validation config: %v", err)
	}

	return configPath
}

// createE2EPartialFailureTestConfig creates a configuration designed to test partial failures
func createE2EPartialFailureTestConfig(t *testing.T, repoNames []string) string {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "e2e-partial-failure-config.yaml")

	config := fmt.Sprintf(`version: "1.0"

repositories:
  - name: %s
    description: Valid repository that should succeed
    private: true
    topics:
      - testing
      - valid
    features:
      issues: true
      wiki: false

  - name: %s
    description: Repository with potentially problematic settings
    private: true
    topics:
      - testing
      - problematic
    features:
      issues: true
      wiki: false
    collaborators:
      - username: definitely-nonexistent-user-12345
        permission: admin
    teams:
      - team: definitely-nonexistent-team-12345
        permission: write
`, repoNames[0], repoNames[1])

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create E2E partial failure config: %v", err)
	}

	return configPath
}

// createE2EMultiRepoInitialConfig creates initial configuration for update testing
func createE2EMultiRepoInitialConfig(t *testing.T, repoNames []string) string {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "e2e-multi-repo-initial-config.yaml")

	config := fmt.Sprintf(`version: "1.0"

repositories:
  - name: %s
    description: Initial description for first repository
    private: true
    topics:
      - testing
      - initial
    features:
      issues: true
      wiki: false

  - name: %s
    description: Initial description for second repository
    private: true
    topics:
      - testing
      - initial
    features:
      issues: true
      wiki: false
`, repoNames[0], repoNames[1])

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create E2E multi-repo initial config: %v", err)
	}

	return configPath
}

// createE2EMultiRepoUpdatedConfig creates updated configuration for update testing
func createE2EMultiRepoUpdatedConfig(t *testing.T, repoNames []string) string {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "e2e-multi-repo-updated-config.yaml")

	config := fmt.Sprintf(`version: "1.0"

repositories:
  - name: %s
    description: Updated description for first repository - changed for testing updates
    private: true
    topics:
      - testing
      - updated
      - modified
    features:
      issues: true
      wiki: true

  - name: %s
    description: Updated description for second repository - also changed for testing
    private: false
    topics:
      - testing
      - updated
      - public
    features:
      issues: true
      wiki: true
      projects: true
`, repoNames[0], repoNames[1])

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to create E2E multi-repo updated config: %v", err)
	}

	return configPath
}
