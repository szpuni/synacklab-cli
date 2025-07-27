# Project Structure

## Directory Organization

```
synacklab/
├── cmd/synacklab/           # Application entry point
│   └── main.go             # Main function - calls internal/cmd.Execute()
├── internal/cmd/           # Internal command implementations (not exported)
│   ├── root.go            # Root command and CLI setup
│   ├── auth.go            # Auth command group
│   ├── aws_config.go      # AWS SSO configuration logic
│   ├── init.go            # Init command implementation
│   └── *_test.go          # Unit tests alongside implementation
├── pkg/config/            # Public configuration package (exportable)
│   ├── config.go          # Configuration structs and loading
│   └── config_test.go     # Configuration tests
├── test/integration/      # Integration tests
│   └── cli_test.go        # End-to-end CLI testing
├── bin/                   # Build output directory
├── .github/workflows/     # CI/CD pipeline definitions
├── go.mod                 # Go module definition
├── Makefile              # Build automation
└── config.example.yaml   # Example configuration file
```

## Architecture Patterns

### Command Structure
- Uses **Cobra CLI framework** for command hierarchy
- Root command in `internal/cmd/root.go` with subcommands
- Each command group has its own file (e.g., `auth.go` for auth commands)
- Main entry point is minimal - delegates to internal packages

### Package Organization
- **cmd/** - Application entry points only
- **internal/** - Private packages not meant for external use
- **pkg/** - Public packages that could be imported by other projects
- **test/** - Integration and end-to-end tests

### Testing Strategy
- Unit tests alongside implementation files (`*_test.go`)
- Integration tests in separate `test/` directory
- Use build tags for integration tests (`-tags integration`)
- CI runs unit tests first, then integration tests with built binary

### Configuration
- YAML configuration files supported
- Example config provided as `config.example.yaml`
- User config stored in `~/.synacklab/config.yaml`
- AWS config updates written to `~/.aws/config`

## Naming Conventions
- Go standard naming (PascalCase for exported, camelCase for unexported)
- Test files end with `_test.go`
- Integration tests use build tag `// +build integration`
- Binary name matches module name (`synacklab`)