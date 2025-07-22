# Synacklab CLI

A command-line tool for DevOps engineers to manage AWS SSO authentication and profile configuration.

## Features

- 🔐 AWS SSO authentication
- 📋 List available AWS profiles
- ⚙️ Set default profile in `.aws/config`
- 🚀 Built with Go and Cobra framework

## Installation

### Prerequisites

- Go 1.21 or later
- AWS CLI configured (optional)

### Build from source

```bash
git clone <repository-url>
cd synacklab
go mod tidy
go build -o synacklab
```

### Install globally

```bash
go install
```

## Quick Start

1. **Initialize configuration** (optional):
   ```bash
   synacklab init
   ```
   This creates `~/.synacklab/config.yaml` with default values.

2. **Edit configuration** (if using config file):
   ```bash
   # Edit ~/.synacklab/config.yaml with your SSO details
   aws:
     sso:
       start_url: "https://your-company.awsapps.com/start"
       region: "us-east-1"
   ```

3. **Configure AWS SSO**:
   ```bash
   synacklab auth aws-config
   ```

## Usage

### Configure AWS SSO Authentication

```bash
synacklab auth aws-config
```

This command will:
1. Load configuration from `~/.synacklab/config.yaml` (if exists) or prompt for input
2. Authenticate with AWS SSO using device authorization
3. List all available AWS profiles (accounts + roles)
4. Allow you to select a profile to set as default
5. Update your `~/.aws/config` file with the selected profile
6. Save SSO configuration for future use

#### Using Configuration File

Create a configuration file at `~/.synacklab/config.yaml`:

```yaml
aws:
  sso:
    start_url: "https://your-company.awsapps.com/start"
    region: "us-east-1"
```

Or use a custom configuration file:

```bash
synacklab auth aws-config --config /path/to/config.yaml
```

#### Command Options

- `--config, -c`: Path to configuration file
- `--interactive, -i`: Force interactive mode even with config file

### Example

```bash
$ synacklab auth aws-config
🔐 Starting AWS SSO authentication...
Enter your AWS SSO start URL: https://my-company.awsapps.com/start
Enter your SSO region (default: us-east-1): us-west-2

🌐 Please visit: https://device.sso.us-west-2.amazonaws.com/
📋 And enter code: ABCD-1234

Press Enter after completing the authorization...

📋 Available AWS profiles:
1. Production-AdministratorAccess (Account: 123456789012, Role: AdministratorAccess)
2. Development-PowerUserAccess (Account: 987654321098, Role: PowerUserAccess)
3. Staging-ReadOnlyAccess (Account: 456789012345, Role: ReadOnlyAccess)

Select profile number to set as default: 1
✅ Successfully configured profile 'Production-AdministratorAccess' as default
```

## Commands

- `synacklab` - Show help and available commands
- `synacklab init` - Initialize synacklab configuration file
- `synacklab auth` - Authentication commands
- `synacklab auth aws-config` - Configure AWS SSO authentication

## Configuration

### Synacklab Configuration

The tool can use a configuration file at `~/.synacklab/config.yaml` to store your SSO settings:

```yaml
aws:
  sso:
    start_url: "https://your-company.awsapps.com/start"
    region: "us-east-1"
```

Copy `config.example.yaml` to `~/.synacklab/config.yaml` and customize it for your environment.

### AWS Configuration

The tool updates your `~/.aws/config` file with the following format:

```ini
[default]
sso_start_url = https://your-company.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = AdministratorAccess
region = us-east-1
output = json
```

## Development

### Project Structure

```
synacklab/
├── cmd/
│   └── synacklab/       # Application entry point
│       └── main.go
├── internal/
│   └── cmd/             # Internal command implementations
│       ├── root.go      # Root command and CLI setup
│       ├── auth.go      # Auth command group
│       ├── aws_config.go # AWS SSO configuration logic
│       ├── init.go      # Init command
│       └── *_test.go    # Unit tests
├── pkg/
│   └── config/          # Public configuration package
│       ├── config.go
│       └── config_test.go
├── test/
│   └── integration/     # Integration tests
│       └── cli_test.go
├── go.mod               # Go module dependencies
├── Makefile             # Build automation
└── README.md            # This file
```

### Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/aws/aws-sdk-go-v2` - AWS SDK for Go v2
- `gopkg.in/ini.v1` - INI file parser

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

See LICENSE file for details.