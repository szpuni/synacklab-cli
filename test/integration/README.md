# Integration Tests

This directory contains integration tests for the synacklab CLI tool.

## Test Categories

### Basic Integration Tests

These tests verify the CLI command structure and basic functionality without requiring external services:

```bash
# Run basic integration tests
make integration-test

# Or run directly with Go
go test -tags integration ./test/integration/
```

### GitHub End-to-End Tests

These tests require a real GitHub organization and valid authentication to test the complete GitHub repository management workflow.

#### Prerequisites

1. **GitHub Token**: Create a GitHub personal access token with the following permissions:
   - `repo` (Full control of private repositories)
   - `admin:org` (Full control of orgs and teams, read and write org projects)
   - `delete_repo` (Delete repositories)

2. **Test Organization**: You need access to a GitHub organization where you can create and delete test repositories.

#### Environment Variables

Set the following environment variables:

```bash
export GITHUB_TOKEN="your_github_token_here"
export GITHUB_TEST_ORG="your-test-org-name"
export GITHUB_E2E_TESTS="true"
```

#### Running E2E Tests

```bash
# Run GitHub E2E tests
go test -tags "integration,github_e2e" ./test/integration/

# Run specific E2E test
go test -tags "integration,github_e2e" -run TestGitHubE2EApply ./test/integration/
```

## Test Structure

### github_test.go

Basic integration tests that verify:
- Command structure and help text
- Configuration file validation
- Dry-run functionality
- Error handling scenarios
- Authentication flow (without making API calls)

### github_e2e_test.go

End-to-end tests that verify:
- Complete apply workflow (create repository)
- Repository updates and reconciliation
- Validation with GitHub API checks
- Cleanup procedures

## Test Data

The tests create temporary configuration files with various scenarios:
- Valid repository configurations
- Invalid configurations (for error testing)
- Malformed YAML files

## Cleanup

The E2E tests automatically clean up any test repositories they create. If a test fails and doesn't clean up properly, you may need to manually delete test repositories from your GitHub organization.

Test repositories are named with timestamps: `synacklab-test-{timestamp}` or `synacklab-update-test-{timestamp}`.

## CI/CD Integration

The basic integration tests run in CI/CD pipelines. The E2E tests are designed to run in environments where GitHub credentials are available, such as:

- Local development with proper environment variables
- CI/CD pipelines with GitHub secrets configured
- Staging environments with test organization access

## Troubleshooting

### Authentication Issues

If you see authentication errors:
1. Verify your `GITHUB_TOKEN` is valid and has the required permissions
2. Check that the token hasn't expired
3. Ensure the test organization exists and you have admin access

### Permission Issues

If you see permission errors:
1. Verify your token has `repo` and `admin:org` permissions
2. Check that you have admin access to the test organization
3. Ensure the `delete_repo` permission is granted for cleanup

### Rate Limiting

GitHub API has rate limits. If you encounter rate limiting:
1. Wait for the rate limit to reset
2. Consider using a GitHub App token instead of a personal access token
3. Run tests less frequently during development

## Adding New Tests

When adding new integration tests:

1. **Basic tests** (github_test.go): Use build tag `integration`
2. **E2E tests** (github_e2e_test.go): Use build tags `integration,github_e2e`
3. Always include cleanup procedures for any resources created
4. Use descriptive test names and include documentation
5. Test both success and failure scenarios