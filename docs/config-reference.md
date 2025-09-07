# Configuration Reference

Complete reference for all Synacklab configuration options, file formats, and environment variable overrides.

## Configuration File Location

Synacklab uses a YAML configuration file located at:

- **Default**: `~/.synacklab/config.yaml`
- **Custom**: Specify with `--config` flag or `SYNACKLAB_CONFIG` environment variable

## File Structure

The configuration file uses YAML format with these top-level sections:

```yaml
# AWS SSO Configuration
aws:
  sso:
    # AWS SSO settings

# GitHub Configuration  
github:
  # GitHub API settings

# Application Settings
app:
  # General application behavior
```

## AWS Configuration

### AWS SSO Settings

Configure AWS SSO authentication and profile management:

```yaml
aws:
  sso:
    # Required: AWS SSO start URL
    start_url: "https://your-company.awsapps.com/start"
    
    # Required: AWS region for SSO operations
    region: "us-east-1"
    
    # Optional: Session timeout in seconds (default: 3600)
    session_timeout: 7200
    
    # Optional: Default region for AWS profiles (default: same as sso.region)
    default_region: "us-east-1"
    
    # Optional: Default output format for AWS profiles (default: "json")
    default_output: "json"
    
    # Optional: Profile name prefix (default: none)
    profile_prefix: "company-"
    
    # Optional: Custom profile naming template
    profile_template: "{account_name}-{role_name}"
```

#### Configuration Details

**start_url** (required)
- Your organization's AWS SSO portal URL
- Format: `https://{subdomain}.awsapps.com/start`
- Example: `https://mycompany.awsapps.com/start`

**region** (required)
- AWS region where your SSO is configured
- Must be a valid AWS region identifier
- Example: `us-east-1`, `eu-west-1`, `ap-southeast-1`

**session_timeout** (optional)
- Session timeout in seconds
- Default: 3600 (1 hour)
- Range: 900-43200 (15 minutes to 12 hours)

**default_region** (optional)
- Default region for generated AWS profiles
- Defaults to the same value as `sso.region`
- Can be overridden per profile

**default_output** (optional)
- Default output format for AWS CLI
- Options: `json`, `text`, `table`, `yaml`, `yaml-stream`
- Default: `json`

**profile_prefix** (optional)
- Prefix added to all generated profile names
- Useful for distinguishing between different organizations
- Example: `company-` results in `company-production-admin`

**profile_template** (optional)
- Template for profile name generation
- Variables: `{account_name}`, `{role_name}`, `{account_id}`
- Default: `{account_name}-{role_name}`

### Environment Variable Overrides

Override AWS configuration using environment variables:

```bash
# AWS SSO settings
export SYNACKLAB_AWS_SSO_START_URL="https://company.awsapps.com/start"
export SYNACKLAB_AWS_SSO_REGION="us-west-2"
export SYNACKLAB_AWS_SESSION_TIMEOUT="7200"
export SYNACKLAB_AWS_DEFAULT_REGION="us-west-2"
export SYNACKLAB_AWS_DEFAULT_OUTPUT="table"
export SYNACKLAB_AWS_PROFILE_PREFIX="dev-"
```

## GitHub Configuration

### GitHub API Settings

Configure GitHub API access and repository management:

```yaml
github:
  # Required: GitHub Personal Access Token
  token: "ghp_your_token_here"
  
  # Optional: Default organization for repositories
  organization: "your-organization"
  
  # Optional: GitHub API base URL (default: "https://api.github.com")
  api_url: "https://api.github.com"
  
  # Optional: GitHub web URL (default: "https://github.com")
  web_url: "https://github.com"
  
  # Optional: Request timeout in seconds (default: 30)
  timeout: 60
  
  # Optional: User agent string
  user_agent: "Synacklab-CLI/1.0"
  
  # Optional: Rate limiting configuration
  rate_limit:
    enabled: true
    max_retries: 5
    backoff_factor: 2
    max_backoff: 300
  
  # Optional: Default repository settings
  defaults:
    private: true
    auto_init: false
    gitignore_template: ""
    license_template: ""
```

#### Configuration Details

**token** (required)
- GitHub Personal Access Token
- Required scopes: `repo`, `admin:org`, `admin:repo_hook`
- Can be set via `GITHUB_TOKEN` environment variable

**organization** (optional)
- Default GitHub organization for repository operations
- Used when `--owner` flag is not specified
- Can be username for personal repositories

**api_url** (optional)
- GitHub API base URL
- Default: `https://api.github.com`
- For GitHub Enterprise: `https://github.company.com/api/v3`

**web_url** (optional)
- GitHub web interface URL
- Default: `https://github.com`
- For GitHub Enterprise: `https://github.company.com`

**timeout** (optional)
- HTTP request timeout in seconds
- Default: 30
- Range: 5-300

**rate_limit** (optional)
- Configure GitHub API rate limit handling
- `enabled`: Enable automatic rate limit handling
- `max_retries`: Maximum number of retries
- `backoff_factor`: Exponential backoff multiplier
- `max_backoff`: Maximum backoff time in seconds

### Environment Variable Overrides

Override GitHub configuration using environment variables:

```bash
# GitHub settings
export GITHUB_TOKEN="ghp_your_token_here"
export SYNACKLAB_GITHUB_ORGANIZATION="myorg"
export SYNACKLAB_GITHUB_API_URL="https://github.company.com/api/v3"
export SYNACKLAB_GITHUB_WEB_URL="https://github.company.com"
export SYNACKLAB_GITHUB_TIMEOUT="60"
export SYNACKLAB_GITHUB_USER_AGENT="MyCompany-Synacklab/1.0"
```

## Application Configuration

### General Application Settings

Configure general application behavior:

```yaml
app:
  # Optional: Log level (default: "info")
  log_level: "info"
  
  # Optional: Default timeout for operations in seconds (default: 300)
  timeout: 600
  
  # Optional: Enable colored output (default: true)
  color: true
  
  # Optional: Enable progress indicators (default: true)
  progress: true
  
  # Optional: Cache configuration
  cache:
    enabled: true
    directory: "~/.synacklab/cache"
    ttl: 3600
    max_size: "100MB"
  
  # Optional: Fuzzy finder configuration
  fuzzy:
    enabled: true
    case_sensitive: false
    algorithm: "fzf"
  
  # Optional: HTTP proxy configuration
  proxy:
    http: "http://proxy.company.com:8080"
    https: "https://proxy.company.com:8080"
    no_proxy: "localhost,127.0.0.1,.company.com"
  
  # Optional: TLS configuration
  tls:
    insecure_skip_verify: false
    ca_bundle: "/path/to/ca-bundle.pem"
```

#### Configuration Details

**log_level** (optional)
- Logging verbosity level
- Options: `debug`, `info`, `warn`, `error`
- Default: `info`

**timeout** (optional)
- Default timeout for operations in seconds
- Default: 300 (5 minutes)
- Range: 30-3600

**color** (optional)
- Enable colored terminal output
- Default: `true`
- Automatically disabled for non-TTY output

**progress** (optional)
- Enable progress indicators and spinners
- Default: `true`
- Automatically disabled for non-TTY output

**cache** (optional)
- Configure local caching behavior
- `enabled`: Enable/disable caching
- `directory`: Cache directory path
- `ttl`: Time-to-live in seconds
- `max_size`: Maximum cache size

**fuzzy** (optional)
- Configure fuzzy finder behavior
- `enabled`: Enable/disable fuzzy finding
- `case_sensitive`: Case-sensitive matching
- `algorithm`: Fuzzy matching algorithm

**proxy** (optional)
- HTTP proxy configuration
- `http`: HTTP proxy URL
- `https`: HTTPS proxy URL
- `no_proxy`: Comma-separated list of hosts to bypass proxy

**tls** (optional)
- TLS/SSL configuration
- `insecure_skip_verify`: Skip TLS certificate verification
- `ca_bundle`: Path to custom CA certificate bundle

### Environment Variable Overrides

Override application configuration using environment variables:

```bash
# Application settings
export SYNACKLAB_LOG_LEVEL="debug"
export SYNACKLAB_TIMEOUT="600"
export SYNACKLAB_COLOR="false"
export SYNACKLAB_PROGRESS="false"

# Cache settings
export SYNACKLAB_CACHE_ENABLED="true"
export SYNACKLAB_CACHE_DIRECTORY="/tmp/synacklab-cache"
export SYNACKLAB_CACHE_TTL="7200"
export SYNACKLAB_CACHE_MAX_SIZE="200MB"

# Proxy settings
export HTTP_PROXY="http://proxy.company.com:8080"
export HTTPS_PROXY="https://proxy.company.com:8080"
export NO_PROXY="localhost,127.0.0.1,.company.com"

# TLS settings
export SYNACKLAB_TLS_INSECURE_SKIP_VERIFY="false"
export SYNACKLAB_TLS_CA_BUNDLE="/etc/ssl/certs/ca-bundle.pem"
```

## Configuration Examples

### Minimal Configuration

Basic setup for AWS SSO only:

```yaml
aws:
  sso:
    start_url: "https://mycompany.awsapps.com/start"
    region: "us-east-1"
```

### Complete Configuration

Full configuration with all features:

```yaml
aws:
  sso:
    start_url: "https://mycompany.awsapps.com/start"
    region: "us-east-1"
    session_timeout: 7200
    default_region: "us-east-1"
    default_output: "json"
    profile_prefix: "company-"

github:
  token: "ghp_your_token_here"
  organization: "mycompany"
  timeout: 60
  rate_limit:
    enabled: true
    max_retries: 5
    backoff_factor: 2

app:
  log_level: "info"
  timeout: 600
  color: true
  cache:
    enabled: true
    ttl: 7200
  fuzzy:
    enabled: true
    case_sensitive: false
```

### Enterprise Configuration

Configuration for GitHub Enterprise and corporate proxy:

```yaml
aws:
  sso:
    start_url: "https://company.awsapps.com/start"
    region: "us-east-1"

github:
  token: "${GITHUB_TOKEN}"
  organization: "mycompany"
  api_url: "https://github.company.com/api/v3"
  web_url: "https://github.company.com"
  user_agent: "MyCompany-Synacklab/1.0"

app:
  proxy:
    http: "http://proxy.company.com:8080"
    https: "https://proxy.company.com:8080"
    no_proxy: "localhost,127.0.0.1,.company.com"
  tls:
    ca_bundle: "/etc/ssl/certs/company-ca.pem"
```

### Development Configuration

Configuration optimized for development:

```yaml
aws:
  sso:
    start_url: "https://dev-company.awsapps.com/start"
    region: "us-west-2"
    profile_prefix: "dev-"

github:
  token: "${GITHUB_TOKEN}"
  organization: "mycompany-dev"

app:
  log_level: "debug"
  timeout: 120
  cache:
    enabled: false  # Disable caching for development
```

## Configuration Validation

### Automatic Validation

Synacklab automatically validates configuration on startup:

```bash
# Configuration is validated when commands run
synacklab auth aws-login

# Validation errors are reported clearly
synacklab github validate repo.yaml
```

### Manual Validation

Validate configuration manually:

```bash
# Test configuration loading
synacklab init --dry-run

# Validate AWS SSO configuration
synacklab auth aws-login --dry-run

# Validate GitHub configuration
synacklab github validate examples/github-simple-repo.yaml
```

### Common Validation Errors

**Invalid YAML syntax:**
```
Error: yaml: line 5: found character that cannot start any token
```

**Missing required fields:**
```
Error: aws.sso.start_url is required
```

**Invalid values:**
```
Error: aws.sso.region must be a valid AWS region
```

## Configuration Migration

### From Previous Versions

Update configuration format for newer versions:

```bash
# Backup existing configuration
cp ~/.synacklab/config.yaml ~/.synacklab/config.yaml.backup

# Recreate with current format
synacklab init

# Manually migrate custom settings
```

### From AWS CLI

Import existing AWS SSO configuration:

```bash
# Check existing AWS configuration
cat ~/.aws/config | grep sso_

# Extract values for Synacklab configuration
# sso_start_url -> aws.sso.start_url
# sso_region -> aws.sso.region
```

## Security Considerations

### Token Storage

**Recommended: Environment Variables**
```yaml
github:
  token: "${GITHUB_TOKEN}"
```

**Alternative: Secure File Permissions**
```bash
chmod 600 ~/.synacklab/config.yaml
```

### Configuration File Security

```bash
# Set restrictive permissions
chmod 600 ~/.synacklab/config.yaml
chown $USER ~/.synacklab/config.yaml

# Verify permissions
ls -la ~/.synacklab/config.yaml
# Should show: -rw------- 1 user user
```

### Environment Variable Security

```bash
# Use secure environment variable management
# In ~/.bashrc (not recommended for production)
export GITHUB_TOKEN="ghp_token_here"

# Better: Use secret management systems
# AWS Secrets Manager, HashiCorp Vault, etc.
```

## Troubleshooting Configuration

### Configuration Loading Issues

```bash
# Check configuration file exists
ls -la ~/.synacklab/config.yaml

# Validate YAML syntax
python -c "import yaml; yaml.safe_load(open('~/.synacklab/config.yaml'))"

# Check file permissions
ls -la ~/.synacklab/config.yaml
```

### Environment Variable Issues

```bash
# List all Synacklab environment variables
env | grep SYNACKLAB

# Check specific variables
echo $GITHUB_TOKEN
echo $SYNACKLAB_AWS_SSO_START_URL
```

### Debug Configuration Loading

```bash
# Enable debug logging
export SYNACKLAB_LOG_LEVEL=debug

# Run command to see configuration loading
synacklab auth aws-login
```

## Advanced Configuration

### Configuration Profiles

Use different configurations for different environments:

```bash
# Development configuration
export SYNACKLAB_CONFIG=~/.synacklab/dev-config.yaml
synacklab auth aws-login

# Production configuration
export SYNACKLAB_CONFIG=~/.synacklab/prod-config.yaml
synacklab auth aws-login

# Staging configuration
synacklab auth aws-login --config ~/.synacklab/staging-config.yaml
```

### Dynamic Configuration

Use environment variables for dynamic configuration:

```yaml
aws:
  sso:
    start_url: "${AWS_SSO_START_URL}"
    region: "${AWS_SSO_REGION:-us-east-1}"

github:
  token: "${GITHUB_TOKEN}"
  organization: "${GITHUB_ORG:-mycompany}"

app:
  log_level: "${LOG_LEVEL:-info}"
```

### Configuration Templates

Create configuration templates for teams:

```yaml
# template-config.yaml
aws:
  sso:
    start_url: "REPLACE_WITH_YOUR_SSO_URL"
    region: "REPLACE_WITH_YOUR_REGION"

github:
  token: "${GITHUB_TOKEN}"
  organization: "REPLACE_WITH_YOUR_ORG"

app:
  log_level: "info"
  timeout: 300
```

## Next Steps

- [Review command reference](commands.md)
- [Check troubleshooting guide](troubleshooting.md)
- [Browse examples](examples.md)
- [Read development guide](development.md)