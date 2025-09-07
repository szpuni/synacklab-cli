# Command Reference

Complete reference for all Synacklab CLI commands, options, and usage patterns.

## Command Structure

Synacklab uses a hierarchical command structure:

```
synacklab [global-options] <command> [command-options] [arguments]
```

### Global Options

Available with all commands:

- `--help, -h`: Show help information
- `--version, -v`: Show version information

## Root Commands

### `synacklab`

Show help and available commands.

```bash
synacklab
synacklab --help
synacklab --version
```

### `synacklab init`

Initialize Synacklab configuration file.

```bash
synacklab init
```

**Description:**
Creates a default configuration file at `~/.synacklab/config.yaml` with AWS SSO settings template.

**Behavior:**
- Prompts before overwriting existing configuration
- Creates directory structure if needed
- Sets appropriate file permissions (600)

**Example Output:**
```
✅ Configuration file created at: ~/.synacklab/config.yaml
📝 Please edit the file to customize your AWS SSO settings.
```

## Authentication Commands

### `synacklab auth`

Parent command for all authentication operations.

```bash
synacklab auth --help
```

**Subcommands:**
- `aws-login` - Authenticate with AWS SSO
- `sync` - Sync AWS SSO profiles
- `aws-config` - Configure default AWS profile
- `eks-config` - Configure EKS clusters
- `eks-ctx` - Switch Kubernetes contexts

### `synacklab auth aws-login`

Authenticate with AWS SSO using device authorization flow.

```bash
synacklab auth aws-login [options]
```

**Options:**
- `--timeout <seconds>`: Authentication timeout (default: 300)

**Examples:**
```bash
# Basic authentication
synacklab auth aws-login

# With extended timeout
synacklab auth aws-login --timeout 600
```

**Process:**
1. Loads configuration from `~/.synacklab/config.yaml`
2. Initiates AWS SSO device authorization
3. Opens browser for verification
4. Stores session credentials securely
5. Displays session information

### `synacklab auth sync`

Synchronize AWS SSO profiles to local configuration.

```bash
synacklab auth sync [options]
```

**Options:**
- `--config, -c <path>`: Path to configuration file
- `--reset`: Replace all profiles with AWS SSO profiles only

**Examples:**
```bash
# Sync profiles (preserve existing)
synacklab auth sync

# Reset and replace all profiles
synacklab auth sync --reset

# Use custom configuration
synacklab auth sync --config /path/to/config.yaml
```

**Behavior:**
- Fetches all AWS accounts and roles from SSO
- Creates profiles in `~/.aws/config`
- Preserves existing non-SSO profiles (unless `--reset`)
- Sanitizes profile names (lowercase, hyphens)

### `synacklab auth aws-config`

Configure default AWS profile interactively.

```bash
synacklab auth aws-config [options]
```

**Options:**
- `--config, -c <path>`: Path to configuration file

**Examples:**
```bash
# Interactive profile selection
synacklab auth aws-config

# Use custom configuration
synacklab auth aws-config --config /path/to/config.yaml
```

**Features:**
- Lists all available AWS profiles
- Shows account ID and role information
- Interactive selection with fuzzy search
- Updates `[default]` section in AWS config

### `synacklab auth eks-config`

Discover EKS clusters and update kubeconfig.

```bash
synacklab auth eks-config [options]
```

**Options:**
- `--region, -r <region>`: AWS region to search (searches all if not specified)
- `--dry-run`: Show what would be done without making changes

**Examples:**
```bash
# Discover clusters in all regions
synacklab auth eks-config

# Discover in specific region
synacklab auth eks-config --region us-west-2

# Preview changes
synacklab auth eks-config --dry-run
```

**Features:**
- Scans AWS regions for EKS clusters
- Adds clusters to `~/.kube/config`
- Configures AWS authentication
- Preserves existing kubeconfig entries

### `synacklab auth eks-ctx`

Switch Kubernetes contexts interactively.

```bash
synacklab auth eks-ctx [options]
```

**Options:**
- `--list, -l`: List all available contexts without switching
- `--filter, -f`: Use filtering mode for better search experience

**Examples:**
```bash
# Interactive context selection
synacklab auth eks-ctx

# List contexts
synacklab auth eks-ctx --list

# Use filtering mode
synacklab auth eks-ctx --filter
```

**Features:**
- Interactive fuzzy search through contexts
- Shows cluster, user, and namespace information
- Highlights current context
- Updates current-context in kubeconfig

## GitHub Commands

### `synacklab github`

Parent command for GitHub repository management.

```bash
synacklab github --help
```

**Subcommands:**
- `apply` - Apply repository configuration
- `validate` - Validate repository configuration

### `synacklab github apply`

Apply repository configuration to GitHub.

```bash
synacklab github apply <config-file.yaml> [options]
```

**Arguments:**
- `<config-file.yaml>`: Path to repository configuration file

**Options:**
- `--dry-run`: Preview changes without applying them
- `--owner <owner>`: Repository owner (organization or user)
- `--repos <repo1,repo2>`: Comma-separated list of repositories (multi-repo only)

**Examples:**
```bash
# Apply single repository
synacklab github apply my-repo.yaml --owner myorg

# Preview changes
synacklab github apply my-repo.yaml --owner myorg --dry-run

# Apply multi-repository configuration
synacklab github apply multi-repos.yaml --owner myorg

# Apply to specific repositories
synacklab github apply multi-repos.yaml --owner myorg --repos repo1,repo2
```

**Features:**
- Creates new repositories or updates existing ones
- Shows detailed change plans
- Supports single and multi-repository formats
- Handles batch operations with error reporting

### `synacklab github validate`

Validate repository configuration file.

```bash
synacklab github validate <config-file.yaml> [options]
```

**Arguments:**
- `<config-file.yaml>`: Path to repository configuration file

**Options:**
- `--owner <owner>`: Repository owner (organization or user)
- `--repos <repo1,repo2>`: Comma-separated list of repositories (multi-repo only)

**Examples:**
```bash
# Validate single repository
synacklab github validate my-repo.yaml --owner myorg

# Validate multi-repository configuration
synacklab github validate multi-repos.yaml --owner myorg

# Validate specific repositories
synacklab github validate multi-repos.yaml --repos repo1,repo2
```

**Validation Checks:**
- YAML syntax and structure
- Required fields and valid values
- GitHub user and team existence
- Repository permissions
- Configuration format compatibility

## Command Patterns

### Interactive vs Non-Interactive

Most commands support both modes:

**Interactive Mode (default):**
```bash
synacklab auth aws-config    # Shows interactive selection
synacklab auth eks-ctx       # Shows fuzzy finder
```

**Non-Interactive Mode:**
```bash
synacklab auth sync --reset  # No prompts, uses flags
synacklab github apply repo.yaml --dry-run  # No prompts
```

### Configuration File Usage

Commands that support configuration files:

```bash
# Use default configuration (~/.synacklab/config.yaml)
synacklab auth aws-login

# Use custom configuration file
synacklab auth aws-login --config /path/to/config.yaml

# Configuration file takes precedence over defaults
synacklab auth sync --config ./custom-config.yaml
```

### Dry-Run Mode

Commands that support dry-run for safety:

```bash
# Preview EKS cluster changes
synacklab auth eks-config --dry-run

# Preview GitHub repository changes
synacklab github apply repo.yaml --dry-run --owner myorg
```

### Batch Operations

Commands that support batch operations:

```bash
# Multi-repository operations
synacklab github apply multi-repos.yaml --owner myorg

# Selective batch operations
synacklab github apply multi-repos.yaml --repos repo1,repo2,repo3 --owner myorg

# Multi-region EKS discovery
synacklab auth eks-config --region us-east-1
synacklab auth eks-config --region us-west-2
```

## Exit Codes

Synacklab uses standard exit codes:

- `0`: Success
- `1`: General error
- `2`: Misuse of shell command
- `126`: Command invoked cannot execute
- `127`: Command not found
- `128+n`: Fatal error signal "n"

### Error Handling Examples

```bash
# Check command success
if synacklab auth aws-login; then
    echo "Authentication successful"
else
    echo "Authentication failed"
    exit 1
fi

# Use in scripts
synacklab github validate repo.yaml --owner myorg || {
    echo "Validation failed"
    exit 1
}
```

## Environment Variables

Commands respect these environment variables:

### Global Variables
- `SYNACKLAB_LOG_LEVEL`: Set log level (debug, info, warn, error)
- `SYNACKLAB_CONFIG`: Override default config file path
- `SYNACKLAB_TIMEOUT`: Default timeout for operations

### AWS Variables
- `AWS_PROFILE`: Default AWS profile to use
- `AWS_REGION`: Default AWS region
- `SYNACKLAB_AWS_SSO_START_URL`: Override SSO start URL
- `SYNACKLAB_AWS_SSO_REGION`: Override SSO region

### GitHub Variables
- `GITHUB_TOKEN`: GitHub Personal Access Token
- `SYNACKLAB_GITHUB_ORGANIZATION`: Default GitHub organization

### Examples

```bash
# Set debug logging
export SYNACKLAB_LOG_LEVEL=debug
synacklab auth aws-login

# Use custom config location
export SYNACKLAB_CONFIG=/path/to/config.yaml
synacklab auth sync

# Set GitHub token
export GITHUB_TOKEN=ghp_your_token_here
synacklab github validate repo.yaml
```

## Shell Completion

Enable shell completion for better CLI experience:

### Bash
```bash
# Add to ~/.bashrc
eval "$(synacklab completion bash)"

# Or generate completion file
synacklab completion bash > /etc/bash_completion.d/synacklab
```

### Zsh
```bash
# Add to ~/.zshrc
eval "$(synacklab completion zsh)"

# Or for oh-my-zsh
synacklab completion zsh > ~/.oh-my-zsh/completions/_synacklab
```

### Fish
```bash
synacklab completion fish > ~/.config/fish/completions/synacklab.fish
```

## Command Aliases

Common command aliases for efficiency:

```bash
# Add to ~/.bashrc or ~/.zshrc
alias slab='synacklab'
alias slab-auth='synacklab auth aws-login'
alias slab-sync='synacklab auth sync'
alias slab-ctx='synacklab auth eks-ctx'
alias slab-gh='synacklab github'
```

## Debugging Commands

### Enable Debug Output

```bash
# Set debug level
export SYNACKLAB_LOG_LEVEL=debug

# Run command with debug output
synacklab auth aws-login
```

### Verbose Mode

```bash
# Some commands support verbose output
synacklab github validate repo.yaml --verbose
```

### Configuration Debugging

```bash
# Check configuration loading
synacklab init --dry-run

# Validate configuration syntax
synacklab auth aws-login --dry-run
```

## Command Chaining

### Sequential Operations

```bash
# Complete AWS setup
synacklab auth aws-login && \
synacklab auth sync && \
synacklab auth aws-config

# EKS setup
synacklab auth eks-config && \
synacklab auth eks-ctx
```

### Conditional Operations

```bash
# Only sync if authentication succeeds
synacklab auth aws-login && synacklab auth sync

# Validate before applying
synacklab github validate repo.yaml --owner myorg && \
synacklab github apply repo.yaml --owner myorg
```

### Error Handling in Scripts

```bash
#!/bin/bash
set -e  # Exit on any error

echo "Setting up AWS environment..."
synacklab auth aws-login
synacklab auth sync
synacklab auth eks-config

echo "AWS environment ready!"
```

## Performance Considerations

### Command Optimization

```bash
# Use specific regions for faster EKS discovery
synacklab auth eks-config --region us-east-1

# Use selective repository operations
synacklab github apply multi-repos.yaml --repos critical-repo1,critical-repo2

# Cache authentication tokens
synacklab auth aws-login  # Tokens cached for reuse
```

### Parallel Operations

```bash
# Run multiple region discoveries in parallel
synacklab auth eks-config --region us-east-1 &
synacklab auth eks-config --region us-west-2 &
wait
```

## Next Steps

- [Review configuration options](config-reference.md)
- [Check troubleshooting guide](troubleshooting.md)
- [Browse examples](examples.md)
- [Read development guide](development.md)