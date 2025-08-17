# Implementation Plan

- [x] 1. Create multi-repository configuration models and format detection
  - Create `pkg/github/multi_config.go` with MultiRepositoryConfig and RepositoryDefaults structs
  - Implement ConfigDetector interface for format detection (single vs multi-repo)
  - Add YAML unmarshaling support for multi-repository format with validation
  - Write unit tests for configuration parsing and format detection
  - Ensure code passes golangci-lint checks and follows Go formatting standards
  - _Requirements: 1.1, 1.2, 1.6, 3.1, 3.2, 3.3_

- [x] 2. Implement configuration merging logic for defaults and overrides
  - Create ConfigMerger interface and implementation in `pkg/github/multi_config.go`
  - Implement deep merge logic for combining defaults with repository-specific settings
  - Handle merge strategies for different field types (override, append, deep merge)
  - Write unit tests for various merge scenarios and edge cases
  - Ensure code passes golangci-lint checks and follows Go formatting standards
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 6.6_

- [x] 3. Extend existing configuration loading to support both formats
  - Modify `pkg/github/config.go` LoadRepositoryConfigFromFile function to detect format
  - Add LoadMultiRepositoryConfigFromFile function for multi-repo configurations
  - Ensure backward compatibility with existing single repository configurations
  - Write unit tests for both configuration loading paths
  - Ensure code passes golangci-lint checks and follows Go formatting standards
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

- [x] 4. Create multi-repository reconciler and batch processing logic
  - Create `pkg/github/multi_reconciler.go` with MultiReconciler interface implementation
  - Implement PlanAll method to create reconciliation plans for multiple repositories
  - Add repository filtering logic for selective operations
  - Write unit tests for multi-repository planning with various scenarios
  - Ensure code passes golangci-lint checks and follows Go formatting standards
  - _Requirements: 2.1, 2.2, 2.3, 7.1, 7.2_

- [x] 5. Implement batch execution with error handling and result aggregation
  - Implement ApplyAll method in multi-reconciler for executing multiple repository changes
  - Add MultiRepoResult struct for collecting success/failure results
  - Implement graceful error handling that continues processing on individual failures
  - Write unit tests for batch execution with mixed success/failure scenarios
  - Ensure code passes golangci-lint checks and follows Go formatting standards
  - _Requirements: 2.4, 2.5, 2.6, 8.1, 8.2, 8.3, 8.6_

- [x] 6. Add multi-repository validation with comprehensive error reporting
  - Implement ValidateAll method in multi-reconciler for validating all repositories
  - Add duplicate repository name detection within configuration
  - Implement validation result aggregation with detailed error reporting
  - Write unit tests for multi-repository validation scenarios
  - Ensure code passes golangci-lint checks and follows Go formatting standards
  - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6_

- [x] 7. Enhance GitHub apply command to support multi-repository operations
  - Modify `internal/cmd/github_apply.go` to detect and handle multi-repository configurations
  - Add --repos flag for selective repository processing
  - Implement enhanced progress reporting for multiple repositories
  - Update command output formatting to show per-repository results
  - Write unit tests for enhanced apply command functionality
  - Ensure code passes golangci-lint checks and follows Go formatting standards
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 7.1, 7.2, 7.3_

- [x] 8. Enhance dry-run functionality for multi-repository preview
  - Extend dry-run mode in apply command to show changes across all repositories
  - Implement repository-grouped change display with clear identification
  - Add summary statistics for total changes across all repositories
  - Enhance destructive change highlighting for multi-repository context
  - Write unit tests for multi-repository dry-run scenarios
  - Ensure code passes golangci-lint checks and follows Go formatting standards
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 7.4_

- [x] 9. Enhance validate command for multi-repository configurations
  - Modify `internal/cmd/github_validate.go` to handle multi-repository configurations
  - Add repository count reporting and per-repository validation results
  - Implement selective validation with --repos flag support
  - Enhance error reporting to show validation issues per repository
  - Write unit tests for enhanced validate command functionality
  - Ensure code passes golangci-lint checks and follows Go formatting standards
  - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 7.5_

- [x] 10. Implement rate limiting and throttling for multi-repository operations
  - Create MultiRepoRateLimiter for managing GitHub API rate limits across repositories
  - Implement intelligent backoff and retry logic for rate limit handling
  - Add configurable concurrency limits for parallel repository processing
  - Write unit tests for rate limiting behavior under various load scenarios
  - Ensure code passes golangci-lint checks and follows Go formatting standards
  - _Requirements: 2.6, 8.5_

- [x] 11. Add comprehensive error handling for multi-repository scenarios
  - Implement MultiRepoError type for aggregating errors across repositories
  - Add partial success handling with appropriate exit codes
  - Implement authentication failure fast-fail before processing repositories
  - Enhance error messages with repository context and actionable guidance
  - Write unit tests for various error scenarios and recovery
  - Ensure code passes golangci-lint checks and follows Go formatting standards
  - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 8.6_

- [x] 12. Create example multi-repository configuration files
  - Create example YAML files demonstrating multi-repository format
  - Add examples showing global defaults with repository-specific overrides
  - Create migration examples from single to multi-repository format
  - Document configuration schema and best practices
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 4.1, 4.2, 4.3, 4.4, 4.5, 4.6_

- [x] 13. Add integration tests for multi-repository workflows
  - Create end-to-end tests for multi-repository apply, validate, and dry-run operations
  - Test selective repository operations with --repos flag
  - Add tests for backward compatibility with existing single repository configurations
  - Test error handling and partial failure scenarios with real GitHub API
  - Ensure integration tests pass golangci-lint checks and follow Go standards, run make test and make lint before you mark task complete
  - _Requirements: 2.1, 2.2, 2.3, 3.1, 3.2, 3.3, 7.1, 7.2, 7.3, 7.4, 7.5_

- [x] 14. Enhance command help and documentation for multi-repository features
  - Update command help text to describe multi-repository capabilities
  - Add examples of multi-repository usage in command documentation
  - Document the --repos flag and selective operation capabilities
  - Create migration guide from single to multi-repository configurations, run make test and make lint before you mark task complete
  - _Requirements: 3.5, 7.1, 7.2, 7.3, 7.4, 7.5_

- [x] 15. Optimize performance for large multi-repository configurations
  - Implement parallel processing for repository operations where safe
  - Add memory-efficient streaming for large configuration files
  - Optimize result aggregation and reporting for better performance
  - Add performance benchmarks and optimization tests
  - Ensure code passes golangci-lint checks and follows Go formatting standards, run make test and make lint before you mark task complete
  - _Requirements: 2.6, 8.5_