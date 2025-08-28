# Implementation Plan

- [x] 1. Create authentication manager package
  - Create `internal/auth/manager.go` with AuthManager interface and implementation
  - Extract AWS SSO authentication logic from existing aws_config.go
  - Implement credential storage and session validation methods
  - Add session expiry detection and credential clearing functionality
  - Run tests and linters (golangci-lint run --no-config --timeout=5m) to make sure that code is fully functional
  - _Requirements: 1.1, 1.2, 4.4, 4.5_

- [x] 2. Implement interactive fuzzy finder
  - Create `pkg/fuzzy/interactive.go` with InteractiveFinder interface
  - Implement real-time filtering based on user input
  - Add keyboard navigation support (arrow keys, vim-style keys)
  - Implement terminal interaction handling for selection and cancellation
  - Add graceful fallback for unsupported terminals
  - Run tests and linters to make sure that code is fully functional
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7_

- [x] 3. Create AWS login command
  - Create `internal/cmd/aws_login.go` with new aws-login command implementation
  - Integrate with authentication manager for AWS SSO device flow
  - Implement proper error handling for authentication failures
  - Add command help text and flag definitions
  - Run tests and linters (golangci-lint run --no-config --timeout=5m) to make sure that code is fully functional
  - _Requirements: 1.1, 1.2, 4.1, 4.2, 4.3_

- [x] 4. Rename and enhance AWS context command
  - Rename `awsConfigCmd` to `awsCtxCmd` in `internal/cmd/aws_config.go`
  - Update command metadata (Use, Short, Long descriptions)
  - Implement auto-authentication flow when user is not authenticated
  - Replace number-based selection with interactive fuzzy finder
  - Run tests and linters (golangci-lint run --no-config --timeout=5m) to make sure that code is fully functional
  - _Requirements: 1.3, 1.4, 3.1, 3.3, 2.1-2.7_

- [x] 5. Update auth command registration
  - Modify `internal/cmd/auth.go` to register new aws-login and aws-ctx commands
  - Remove registration of old config command
  - Update auth command help text to reflect new subcommands
  - Fix auth to display Available Commands correctly and remove duplicates
  - Run tests and linters (golangci-lint run --no-config --timeout=5m) to make sure that code is fully functional
  - _Requirements: 3.2, 3.3, 3.4_

- [x] 6. Enhance fuzzy finder integration across commands
  - Update any existing commands that use fuzzy finder to use new interactive version
  - Ensure consistent metadata display for AWS profiles (account info, role info)
  - Implement consistent keyboard shortcuts across all selection scenarios
  - Run tests and linters (golangci-lint run --no-config --timeout=5m) to make sure that code is fully functional
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_

- [x] 7. Add comprehensive error handling
  - Implement network connectivity error detection and messaging
  - Add AWS SSO session expiry detection in authentication manager
  - Create user-friendly error messages with troubleshooting guidance
  - Add validation for AWS configuration parameters
  - Run tests and linters (golangci-lint run --no-config --timeout=5m) to make sure that code is fully functional
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

- [x] 8. Create unit tests for authentication manager
  - Write tests for AWS SSO authentication flow with mocked AWS SDK calls
  - Test credential storage, retrieval, and validation methods
  - Test session expiry detection and credential clearing
  - Test error handling scenarios (network failures, invalid tokens)
  - Run tests and linters (golangci-lint run --no-config --timeout=5m) to make sure that code is fully functional
  - _Requirements: 1.1, 1.2, 4.1, 4.2, 4.4, 4.5_

- [x] 9. Create unit tests for interactive fuzzy finder
  - Write tests for filtering algorithms and real-time updates
  - Test keyboard navigation logic and input handling
  - Test terminal interaction mocking and edge cases
  - Test graceful handling of empty lists and single items
  - Run tests and linters (golangci-lint run --no-config --timeout=5m) to make sure that code is fully functional
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7_

- [x] 10. Create unit tests for command handlers
  - Write tests for aws-login command execution and argument parsing
  - Test aws-ctx command integration with authentication manager
  - Test auto-authentication flow when user is not authenticated
  - Test error propagation and user-friendly error messaging
  - Run tests and linters (golangci-lint run --no-config --timeout=5m) to make sure that code is fully functional
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 3.1, 3.3, 4.1, 4.2, 4.3_

- [ ] 11. Create integration tests for authentication flow
  - Write end-to-end tests for complete aws-login command execution
  - Test aws-ctx command behavior with and without existing authentication
  - Test credential persistence across multiple command invocations
  - Test integration between authentication manager and command handlers
  - Run tests and linters (golangci-lint run --no-config --timeout=5m) to make sure that code is fully functional
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 4.4, 4.5_

- [x] 12. Implement automatic browser opening for AWS SSO authentication
  - Add browser opening functionality to authentication manager
  - Replace manual URL display and Enter key wait with automatic browser launch
  - Implement polling mechanism to check for authorization completion
  - Add cross-platform browser opening support (macOS, Linux, Windows)
  - Handle cases where browser opening fails with graceful fallback
  - Run tests and linters (golangci-lint run --no-config --timeout=5m) to make sure that code is fully functional
  - _Requirements: 1.2, 1.3_

- [x] 13. Replace custom fuzzy finder with fzf library integration
  - ✅ Add fzf library dependency (github.com/junegunn/fzf) to go.mod
  - ✅ Create new fzf-based finder implementation in pkg/fuzzy/fzf.go
  - ✅ Implement fzf library integration using temporary files and stdin/stdout redirection
  - ✅ Add fallback to simple selection when fzf execution fails
  - ✅ Update all commands (aws-ctx, eks-ctx) to use new fzf-based finder
  - ✅ Maintain existing FzfFinderInterface for compatibility
  - ✅ Run tests and linters (golangci-lint run --no-config --timeout=5m) to make sure that code is fully functional
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8_

- [x] 13.1. Add --no-auth flag to aws-ctx command
  - Add --no-auth flag definition to aws-ctx command
  - Modify aws-ctx logic to skip auto-authentication when flag is present
  - Return clear error message when not authenticated and --no-auth is used
  - Update command help text to document the new flag
  - Run tests and linters (golangci-lint run --no-config --timeout=5m) to make sure that code is fully functional
  - _Requirements: 1.7_

- [ ] 14. Create integration tests for enhanced authentication flow
  - Test complete aws-login command with automatic browser opening
  - Test polling mechanism for authorization completion
  - Test fallback behavior when browser opening fails
  - Test cross-platform compatibility for browser opening
  - Run tests and linters (golangci-lint run --no-config --timeout=5m) to make sure that code is fully functional
  - _Requirements: 1.1, 1.2, 1.3, 1.4_

- [ ] 15. Create integration tests for fzf-based fuzzy finder
  - Test fzf integration with real AWS profile data from config files
  - Test fzf filtering performance and accuracy with various input patterns
  - Test selection handling and result processing with fzf
  - Test fallback behavior when fzf is not available on the system
  - Test consistent behavior across different terminal environments
  - Run tests and linters (golangci-lint run --no-config --timeout=5m) to make sure that code is fully functional
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8, 5.1, 5.2, 5.3, 5.4, 5.5_