---
inclusion: always
---

# Technology Stack & Development Guidelines

## Language & Runtime
- **Go 1.21+** - Primary language, use modern Go idioms and features
- Single binary executable - no external runtime dependencies
- Cross-platform support (Linux, macOS, Windows)

## Core Dependencies
- **github.com/spf13/cobra** - CLI framework, use for all command implementations
- **github.com/aws/aws-sdk-go-v2** - AWS SDK, prefer v2 over v1 for new code
- **gopkg.in/ini.v1** - AWS config file parsing only
- **gopkg.in/yaml.v3** - Configuration files, prefer over JSON

## Development Workflow
Use Make targets for all development tasks:
```bash
make deps      # Install/update dependencies
make fmt       # Format code (gofmt + goimports)
make lint      # Run golangci-lint
make test      # Unit tests only
make build     # Production binary
```

## Code Style Requirements
- **Formatting**: Always run `make fmt` before commits
- **Linting**: Code must pass `golangci-lint` with project config
- **Error handling**: Use `fmt.Errorf("context: %w", err)` for wrapping
- **Interfaces**: Define interfaces in consuming packages, not implementing packages
- **Testing**: Table-driven tests preferred, mock external dependencies

## Build & Release
- **Local builds**: Use `make build` for single platform
- **Cross-compilation**: Use `make build-all` for all platforms  
- **Releases**: GoReleaser handles automated releases via CI
- **Integration tests**: Require built binary, run with `make integration-test`

## Performance Guidelines
- Commands should complete in <2 seconds for typical operations
- Use context.Context for cancellation and timeouts
- Implement rate limiting for external API calls
- Cache authentication tokens appropriately

## Security Requirements
- Never store credentials in plaintext
- Use AWS SSO tokens exclusively
- Validate all user inputs
- Follow principle of least privilege for AWS permissions