# Synacklab CLI

A command-line tool for DevOps engineers to manage AWS SSO authentication and profile configuration.

## Features

- 🔐 AWS SSO authentication with device authorization flow
- 🔄 Sync all AWS SSO profiles to local configuration
- 📋 List and select from available AWS profiles
- ⚙️ Set default profile in `.aws/config`
- 🔧 Merge or replace existing profiles
- 🐙 GitHub repository management with declarative configuration
- 🛡️ Branch protection rules and access control management
- 🔗 Webhook configuration and team-based permissions
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
- `synacklab github` - GitHub repository management commands
- `synacklab github apply` - Apply repository configuration from YAML file
- `synacklab github validate` - Validate repository configuration file

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

## GitHub Repository Management

Synacklab provides declarative GitHub repository management using YAML configuration files. This feature allows you to define repository settings, branch protection rules, access control, and webhooks as code, enabling Infrastructure as Code practices for GitHub repository management.

### Quick Start

1. **Set up GitHub authentication**:
   ```bash
   export GITHUB_TOKEN="your_github_token"
   ```
   Or configure in `~/.synacklab/config.yaml`:
   ```yaml
   github:
     token: "your_github_token"
     organization: "your-org"  # optional
   ```

2. **Create a repository configuration file**:
   ```yaml
   # my-repo.yaml
   name: my-awesome-repo
   description: "An example repository managed by synacklab"
   private: false
   
   features:
     issues: true
     wiki: true
     projects: false
     discussions: false
   
   branch_protection:
     - pattern: "main"
       required_status_checks:
         - "ci/build"
         - "ci/test"
       required_reviews: 2
       require_code_owner_review: true
   ```

3. **Apply the configuration**:
   ```bash
   synacklab github apply my-repo.yaml
   ```

4. **Validate configuration before applying**:
   ```bash
   synacklab github validate my-repo.yaml
   ```

### GitHub Commands

#### Apply Repository Configuration

```bash
synacklab github apply <config-file.yaml>
```

Creates a new repository or updates an existing repository to match the configuration file.

**Options:**
- `--dry-run`: Preview changes without applying them
- `--config, -c`: Path to synacklab configuration file

**Examples:**

```bash
# Apply configuration to create/update repository
synacklab github apply examples/github-simple-repo.yaml

# Preview changes without applying
synacklab github apply examples/github-complete-repo.yaml --dry-run

# Use custom synacklab config file
synacklab github apply my-repo.yaml --config /path/to/config.yaml
```

#### Validate Repository Configuration

```bash
synacklab github validate <config-file.yaml>
```

Validates the repository configuration file for syntax errors, required fields, and GitHub-specific constraints.

**Examples:**

```bash
# Validate a configuration file
synacklab github validate examples/github-complete-repo.yaml

# Validate with detailed output
synacklab github validate my-repo.yaml --verbose
```

### Configuration File Format

Repository configuration files use YAML format with the following structure:

#### Basic Repository Settings

```yaml
# Repository name (required)
name: "my-repository"

# Repository description (optional)
description: "A brief description of the repository"

# Repository visibility (required)
private: true  # true for private, false for public

# Repository topics for discoverability (optional)
topics:
  - "golang"
  - "cli"
  - "devops"

# Repository features (optional)
features:
  issues: true      # Enable GitHub Issues
  wiki: false       # Enable repository wiki
  projects: true    # Enable GitHub Projects
  discussions: false # Enable GitHub Discussions
```

#### Branch Protection Rules

```yaml
branch_protection:
  - pattern: "main"  # Branch pattern (supports glob patterns)
    required_status_checks:
      - "ci/build"
      - "ci/test"
    require_up_to_date: true
    required_reviews: 2
    dismiss_stale_reviews: true
    require_code_owner_review: true
    restrict_pushes:
      - "admin-team"
```

#### Access Control

```yaml
# Individual collaborator access
collaborators:
  - username: "john-doe"
    permission: "write"  # read, write, admin

# Team-based access control
teams:
  - team: "backend-team"
    permission: "write"
  - team: "devops-team"
    permission: "admin"
```

#### Webhooks

```yaml
webhooks:
  - url: "https://ci.example.com/webhook/github"
    events:
      - "push"
      - "pull_request"
      - "release"
    secret: "${WEBHOOK_SECRET}"  # Environment variable substitution
    active: true
```

### Authentication Setup

#### GitHub Token Authentication

1. **Create a GitHub Personal Access Token**:
   - Go to GitHub Settings → Developer settings → Personal access tokens
   - Generate a new token with the following scopes:
     - `repo` (Full control of private repositories)
     - `admin:org` (Full control of orgs and teams, read and write org projects)
     - `admin:repo_hook` (Full control of repository hooks)

2. **Configure the token**:

   **Option 1: Environment Variable (Recommended)**
   ```bash
   export GITHUB_TOKEN="ghp_your_token_here"
   ```

   **Option 2: Configuration File**
   ```yaml
   # ~/.synacklab/config.yaml
   github:
     token: "ghp_your_token_here"
     organization: "your-organization"  # optional, for organization repositories
   ```

#### Required Permissions

Your GitHub token needs the following permissions:

- **Repository permissions**: Create, read, update repositories
- **Administration permissions**: Manage repository settings, branch protection
- **Organization permissions**: Manage team access (for organization repositories)
- **Webhook permissions**: Create and manage webhooks

### Example Configurations

The `examples/` directory contains several example configuration files:

- **`examples/github-simple-repo.yaml`**: Basic repository with minimal configuration
- **`examples/github-complete-repo.yaml`**: Comprehensive example showing all features
- **`examples/github-team-repo.yaml`**: Team-focused repository with collaboration features
- **`examples/github-open-source.yaml`**: Open source project configuration
- **`examples/github-config-reference.yaml`**: Complete reference with inline documentation

### Common Workflows

#### Creating a New Repository

1. Create a configuration file:
   ```yaml
   name: new-project
   description: "A new project repository"
   private: true
   
   features:
     issues: true
     projects: true
   
   teams:
     - team: "development-team"
       permission: "write"
   ```

2. Apply the configuration:
   ```bash
   synacklab github apply new-project.yaml
   ```

#### Updating Repository Settings

1. Modify your existing configuration file
2. Preview changes:
   ```bash
   synacklab github apply my-repo.yaml --dry-run
   ```
3. Apply changes:
   ```bash
   synacklab github apply my-repo.yaml
   ```

#### Managing Multiple Repositories

Create separate configuration files for each repository and apply them individually, or use a script to apply multiple configurations:

```bash
#!/bin/bash
for config in configs/*.yaml; do
  echo "Applying $config..."
  synacklab github apply "$config"
done
```

### Best Practices

1. **Use version control**: Store configuration files in Git repositories
2. **Environment variables**: Use environment variables for secrets (webhook secrets, tokens)
3. **Team-based access**: Prefer team-based access over individual collaborators
4. **Branch protection**: Always protect main/master branches in production repositories
5. **Validation**: Always validate configurations before applying
6. **Dry-run**: Use `--dry-run` to preview changes before applying
7. **Documentation**: Use meaningful descriptions and topics for repositories

### Troubleshooting

#### Authentication Issues

```bash
# Check if token is set
echo $GITHUB_TOKEN

# Validate token permissions
synacklab github validate --check-auth
```

#### Permission Errors

Ensure your GitHub token has the required scopes:
- `repo` for repository management
- `admin:org` for organization and team management
- `admin:repo_hook` for webhook management

#### Configuration Validation

```bash
# Validate configuration syntax
synacklab github validate my-repo.yaml

# Check for common issues
synacklab github validate my-repo.yaml --verbose
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