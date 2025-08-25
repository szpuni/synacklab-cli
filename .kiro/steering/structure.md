---
inclusion: always
---

# Project Structure & Architecture

## Package Organization Rules

### Core Directories
- `cmd/synacklab/` - Entry point only, minimal main.go that calls internal/cmd.Execute()
- `internal/cmd/` - All CLI command implementations (private, not exportable)
- `pkg/` - Public packages that could be imported by external projects
- `test/integration/` - End-to-end tests requiring built binary

### Command Implementation Pattern
- Use Cobra CLI framework exclusively
- Root command in `internal/cmd/root.go`
- Each command group gets its own file (auth.go, github.go, etc.)
- Commands should be thin wrappers around pkg/ functionality
- Always include both interactive and non-interactive modes

### File Placement Rules
- New CLI commands: Add to `internal/cmd/`
- Reusable business logic: Add to `pkg/`
- Unit tests: Place alongside implementation (`*_test.go`)
- Integration tests: Place in `test/integration/`
- Configuration structs: Place in `pkg/config/`

## Testing Requirements

### Test Organization
- Unit tests must be co-located with implementation files
- Integration tests require `// +build integration` build tag
- Use table-driven tests for multiple scenarios
- Mock external dependencies (AWS APIs, GitHub APIs)

### Test Execution Order
1. Unit tests run first (`make test`)
2. Integration tests require built binary (`make integration-test`)
3. CI pipeline enforces this order

## Configuration Conventions

### File Locations
- User config: `~/.synacklab/config.yaml`
- AWS config updates: `~/.aws/config` (standard AWS CLI location)
- Example configs: `config.example.yaml` in project root

### Configuration Rules
- All config must be validated on load with clear error messages
- Support both YAML and environment variable overrides
- Never store credentials - use AWS SSO tokens only
- Provide working examples for all configuration options

## Code Organization Patterns

### Error Handling
- Use custom error types in `pkg/` packages
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- CLI commands should handle errors gracefully with user-friendly messages

### Dependency Injection
- Pass dependencies as parameters, not globals
- Use interfaces for external dependencies (AWS SDK, GitHub API)
- Keep main.go minimal - business logic belongs in pkg/

### Naming Standards
- Go standard conventions (PascalCase exported, camelCase unexported)
- CLI commands use kebab-case (`aws-config`, not `awsConfig`)
- Package names are lowercase, single word when possible
- Test files end with `_test.go`