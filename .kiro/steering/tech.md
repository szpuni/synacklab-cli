# Technology Stack

## Language & Runtime
- **Go 1.21+** - Primary programming language
- Built as a single binary executable

## Key Dependencies
- **github.com/spf13/cobra** - CLI framework for command structure
- **github.com/aws/aws-sdk-go-v2** - AWS SDK for SSO operations
- **gopkg.in/ini.v1** - INI file parsing for AWS config files
- **gopkg.in/yaml.v3** - YAML configuration file support

## Build System
- **Make** - Primary build automation
- **GoReleaser** - Cross-platform release automation
- **golangci-lint** - Code linting and formatting

## Common Commands

### Development
```bash
# Install dependencies
make deps

# Format and lint code
make fmt
make vet
make lint

# Run tests (unit tests only)
make test

# Run tests with coverage
make test-coverage

# Build binary
make build

# Development build with debug info
make dev-build
```

### Testing
```bash
# Unit tests only
make test

# Integration tests (requires built binary)
make integration-test

# All tests
make test && make integration-test
```

### Release
```bash
# Cross-compile for all platforms
make build-all

# Clean build artifacts
make clean
```

## Code Quality
- Uses golangci-lint with custom configuration
- Enforces gofmt and goimports formatting
- Requires test coverage for new features
- CI/CD pipeline runs lint, test, build, and integration tests