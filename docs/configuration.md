# Configuration Guide

This guide covers all configuration options for Synacklab CLI, including file locations, formats, and environment variable overrides.

## Configuration File Locations

Synacklab uses a hierarchical configuration system:

### Primary Configuration
- **Location**: `~/.synacklab/config.yaml`
- **Created by**: `synacklab init`
- **Purpose**: Main configuration file for all Synacklab settings

### Command-Specific Configurations
- **Custom location**: Use `--config /path/to/config.yaml` with any command
- **Environment variables**: Override specific settings (see below)

## Configuration File Format

The configuration file uses YAML format with the following structure:

```yaml
# AWS SSO Configuration
aws:
  sso:
    start_url: "https://your-company.awsapps.com/start"
    region: "us-east-1"

# GitHub Configuration
github:
  token: "ghp_your_token_here"
  organization: "your-organization"  # optional

# Application Settings (optional)
app:
  log_level: "info"  # debug, info, warn, error
  timeout: 300       # seconds
```

## Configuration Sections

### AWS SSO Configuration

Controls AWS SSO authentication and profile management:

```yaml
aws:
  sso:
    # Required: Your AWS SSO start URL
    start_url: "https://your-company.awsapps.com/start"
    
    # Required: AWS region for SSO operations
    region: "us-east-1"
    
    # Optional: Default session timeout (seconds)
    session_timeout: 3600
    
    # Optional: Default region for AWS profiles
    default_region: "us-east-1"
    
    # Optional: Default output format for AWS profiles
    default_output: "json"
```

**Finding Your SSO Start URL:**
1. Check your AWS SSO portal URL
2. Look in existing `~/.aws/config` for `sso_start_url`
3. Ask your AWS administrator

### GitHub Configuration

Controls GitHub API access and repository management:

```yaml
github:
  # Required: GitHub Personal Access Token
  token: "ghp_your_token_here"
  
  # Optional: Default organization for repositories
  organization: "your-organization"
  
  # Optional: GitHub API base URL (for GitHub Enterprise)
  api_url: "https://api.github.com"
  
  # Optional: Request timeout (seconds)
  timeout: 30
  
  # Optional: Rate limit handling
  rate_limit:
    enabled: true
    max_retries: 3
    backoff_factor: 2
```

**GitHub Token Requirements:**
- **Scopes needed**: `repo`, `admin:org`, `admin:repo_hook`
- **Creation**: GitHub Settings → Developer settings → Personal access tokens
- **Security**: Store in environment variables for production use

### Application Settings

General application behavior:

```yaml
app:
  # Log level: debug, info, warn, error
  log_level: "info"
  
  # Default timeout for operations (seconds)
  timeout: 300
  
  # Enable colored output
  color: true
  
  # Cache settings
  cache:
    enabled: true
    ttl: 3600  # seconds
    
  # Fuzzy finder settings
  fuzzy:
    enabled: true
    case_sensitive: false
```

## Environment Variable Overrides

Override configuration values using environment variables:

### AWS SSO Variables
```bash
export SYNACKLAB_AWS_SSO_START_URL="https://company.awsapps.com/start"
export SYNACKLAB_AWS_SSO_REGION="us-west-2"
export SYNACKLAB_AWS_SESSION_TIMEOUT="7200"
```

### GitHub Variables
```bash
export GITHUB_TOKEN="ghp_your_token_here"
export SYNACKLAB_GITHUB_ORGANIZATION="myorg"
export SYNACKLAB_GITHUB_API_URL="https://github.company.com/api/v3"
```

### Application Variables
```bash
export SYNACKLAB_LOG_LEVEL="debug"
export SYNACKLAB_TIMEOUT="600"
export SYNACKLAB_COLOR="false"
```

## Configuration Validation

Synacklab validates configuration on startup:

### Automatic Validation
```bash
# Validates configuration automatically
synacklab auth aws-login
```

### Manual Validation
```bash
# Check configuration syntax
synacklab init --validate

# Test AWS SSO configuration
synacklab auth aws-login --dry-run

# Test GitHub configuration
synacklab github validate examples/github-simple-repo.yaml
```

## Configuration Examples

### Minimal Configuration

For basic AWS SSO usage:

```yaml
aws:
  sso:
    start_url: "https://company.awsapps.com/start"
    region: "us-east-1"
```

### Complete Configuration

For full feature usage:

```yaml
aws:
  sso:
    start_url: "https://company.awsapps.com/start"
    region: "us-east-1"
    session_timeout: 7200
    default_region: "us-east-1"
    default_output: "json"

github:
  token: "ghp_your_token_here"
  organization: "mycompany"
  timeout: 60
  rate_limit:
    enabled: true
    max_retries: 5

app:
  log_level: "info"
  timeout: 300
  color: true
  cache:
    enabled: true
    ttl: 3600
  fuzzy:
    enabled: true
    case_sensitive: false
```

### Multi-Environment Configuration

Using environment-specific configurations:

```bash
# Development
export SYNACKLAB_CONFIG="~/.synacklab/dev-config.yaml"

# Production
export SYNACKLAB_CONFIG="~/.synacklab/prod-config.yaml"

# Use specific config
synacklab auth sync --config ~/.synacklab/staging-config.yaml
```

## Security Best Practices

### Token Storage

**Recommended: Environment Variables**
```bash
# In ~/.bashrc or ~/.zshrc
export GITHUB_TOKEN="ghp_your_token_here"

# In configuration file
github:
  token: "${GITHUB_TOKEN}"
```

**Alternative: Secure File Permissions**
```bash
# Restrict config file access
chmod 600 ~/.synacklab/config.yaml

# Verify permissions
ls -la ~/.synacklab/config.yaml
```

### Token Rotation

```bash
# Update token in environment
export GITHUB_TOKEN="new_token_here"

# Or update configuration file
synacklab init  # Recreate with new values
```

## Configuration Migration

### From AWS CLI

Import existing AWS SSO configuration:

```bash
# Check existing AWS config
cat ~/.aws/config

# Extract SSO settings for Synacklab config
grep sso_ ~/.aws/config
```

### From Previous Versions

Update configuration format:

```bash
# Backup existing config
cp ~/.synacklab/config.yaml ~/.synacklab/config.yaml.backup

# Recreate with current format
synacklab init
```

## Troubleshooting Configuration

### Common Issues

**Configuration file not found:**
```bash
# Create default configuration
synacklab init

# Verify location
ls -la ~/.synacklab/config.yaml
```

**Invalid YAML syntax:**
```bash
# Validate YAML syntax
python -c "import yaml; yaml.safe_load(open('~/.synacklab/config.yaml'))"

# Or use online YAML validator
```

**Environment variables not working:**
```bash
# Check variable names (case sensitive)
env | grep SYNACKLAB

# Test variable expansion
echo $GITHUB_TOKEN
```

**Permission errors:**
```bash
# Fix file permissions
chmod 600 ~/.synacklab/config.yaml
chown $USER ~/.synacklab/config.yaml
```

### Debug Configuration Loading

```bash
# Enable debug logging
export SYNACKLAB_LOG_LEVEL="debug"
synacklab auth aws-login

# Check which config file is loaded
synacklab init --dry-run
```

## Advanced Configuration

### Custom Cache Directory

```yaml
app:
  cache:
    directory: "/custom/cache/path"
    enabled: true
    ttl: 7200
```

### Proxy Configuration

```yaml
app:
  proxy:
    http: "http://proxy.company.com:8080"
    https: "https://proxy.company.com:8080"
    no_proxy: "localhost,127.0.0.1,.company.com"
```

### Custom User Agent

```yaml
github:
  user_agent: "MyCompany-Synacklab/1.0"
```

## Next Steps

- [Set up AWS SSO authentication](aws-sso.md)
- [Configure GitHub integration](github.md)
- [Review command reference](commands.md)
- [Check troubleshooting guide](troubleshooting.md)