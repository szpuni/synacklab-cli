# Synacklab CLI

A command-line tool for DevOps engineers to manage AWS SSO authentication and profile configuration.

## Features

- 🔐 AWS SSO authentication with device authorization flow
- 🔄 Sync all AWS SSO profiles to local configuration
- 📋 List and select from available AWS profiles
- ⚙️ Set default profile in `.aws/config`
- 🔧 Merge or replace existing profiles
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

3. **Sync AWS SSO profiles**:
   ```bash
   synacklab auth sync
   ```

4. **Set default profile** (optional):
   ```bash
   synacklab auth config
   ```

## Usage

### Sync AWS SSO Profiles

```bash
synacklab auth sync
```

This command will:
1. Load configuration from `~/.synacklab/config.yaml` (if exists) or prompt for input
2. Authenticate with AWS SSO using device authorization
3. Fetch all available AWS profiles (accounts + roles) from SSO
4. Create or update profiles in your `~/.aws/config` file
5. Preserve existing non-SSO profiles by default

#### Sync Options

- `--config, -c`: Path to configuration file
- `--reset`: Replace all profiles with AWS SSO profiles only (removes existing profiles)

### Configure Default AWS Profile

```bash
synacklab auth config
```

This command will:
1. List all available profiles from your `~/.aws/config` file
2. Allow you to select a profile to set as default
3. Update the `[default]` section in your AWS config

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

### Examples

#### Sync AWS SSO Profiles

```bash
$ synacklab auth sync
🔄 Starting AWS SSO profile synchronization...
🔐 Authenticating with AWS SSO: https://my-company.awsapps.com/start

🌐 Please visit: https://device.sso.us-west-2.amazonaws.com/
📋 And enter code: ABCD-1234

Press Enter after completing the authorization...
📋 Found 5 profiles in AWS SSO
📊 Added 3 new profiles, updated 2 existing profiles
✅ Successfully synchronized 5 SSO profiles to AWS config
```

#### Reset All Profiles

```bash
$ synacklab auth sync --reset
🔄 Starting AWS SSO profile synchronization...
🔐 Authenticating with AWS SSO: https://my-company.awsapps.com/start
🔄 Resetting AWS config file
✅ Successfully replaced AWS config with 5 SSO profiles
```

#### Set Default Profile

```bash
$ synacklab auth config
⚙️  Configuring AWS default profile...

📋 Available AWS profiles:
1. production-administratoraccess (Account: 123456789012, Role: AdministratorAccess)
2. development-poweruseraccess (Account: 987654321098, Role: PowerUserAccess)
3. staging-readonlyaccess (Account: 456789012345, Role: ReadOnlyAccess)

Select profile number to set as default: 1
✅ Successfully set 'production-administratoraccess' as the default AWS profile
```

## Commands

- `synacklab` - Show help and available commands
- `synacklab init` - Initialize synacklab configuration file
- `synacklab auth` - Authentication commands
- `synacklab auth sync` - Sync AWS SSO profiles to local configuration
- `synacklab auth config` - Set default AWS profile from existing profiles

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

The tool updates your `~/.aws/config` file with profiles for each account/role combination:

```ini
[default]
sso_start_url = https://your-company.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = AdministratorAccess
region = us-east-1
output = json

[profile production-administratoraccess]
sso_start_url = https://your-company.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = AdministratorAccess
region = us-east-1
output = json

[profile development-poweruseraccess]
sso_start_url = https://your-company.awsapps.com/start
sso_region = us-east-1
sso_account_id = 987654321098
sso_role_name = PowerUserAccess
region = us-east-1
output = json
```

Profile names are automatically sanitized (lowercase, spaces/underscores become hyphens).

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