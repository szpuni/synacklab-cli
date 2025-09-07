# AWS SSO Authentication

Synacklab provides comprehensive AWS SSO (Single Sign-On) authentication and profile management capabilities. This guide covers all AWS-related features and workflows.

## Overview

AWS SSO integration in Synacklab includes:

- **Device Authorization Flow**: Secure browser-based authentication
- **Profile Synchronization**: Automatic sync of all AWS accounts and roles
- **Profile Management**: Interactive selection and configuration
- **Session Management**: Automatic token refresh and validation
- **Multi-Account Support**: Seamless switching between AWS accounts

## Prerequisites

- AWS SSO configured in your organization
- Access to AWS SSO portal URL
- Web browser for device authorization
- AWS CLI v2 (recommended for enhanced integration)

## Configuration

### Basic Configuration

Set up your AWS SSO configuration in `~/.synacklab/config.yaml`:

```yaml
aws:
  sso:
    start_url: "https://your-company.awsapps.com/start"
    region: "us-east-1"
```

### Advanced Configuration

```yaml
aws:
  sso:
    start_url: "https://your-company.awsapps.com/start"
    region: "us-east-1"
    session_timeout: 7200      # Session timeout in seconds
    default_region: "us-east-1" # Default region for profiles
    default_output: "json"     # Default output format
```

### Finding Your SSO Start URL

1. **From AWS SSO Portal**: Copy the URL from your bookmark or email
2. **From Existing Config**: Check `~/.aws/config` for `sso_start_url`
3. **From Administrator**: Ask your AWS administrator

Example URLs:
- `https://mycompany.awsapps.com/start`
- `https://d-1234567890.awsapps.com/start`

## Commands

### Authentication Commands

#### `synacklab auth aws-login`

Authenticate with AWS SSO using device authorization flow.

```bash
# Basic authentication
synacklab auth aws-login

# With custom timeout
synacklab auth aws-login --timeout 600
```

**Process:**
1. Initiates device authorization with AWS SSO
2. Opens browser to verification URL
3. Prompts for verification code entry
4. Stores session credentials securely
5. Validates authentication status

**Example Output:**
```
🚀 Starting AWS SSO authentication...
🔐 Authenticating with AWS SSO: https://mycompany.awsapps.com/start

🌐 Please visit: https://device.sso.us-east-1.amazonaws.com/
📋 And enter code: ABCD-1234

Press Enter after completing the authorization...

🎉 Authentication successful!
📍 SSO URL: https://mycompany.awsapps.com/start
🌍 Region: us-east-1
⏰ Session expires: 2024-12-07 15:30:45 PST
```

#### `synacklab auth sync`

Synchronize AWS SSO profiles to local configuration.

```bash
# Sync all profiles (default behavior)
synacklab auth sync

# Reset and replace all profiles
synacklab auth sync --reset

# Use custom configuration file
synacklab auth sync --config /path/to/config.yaml
```

**Features:**
- Discovers all AWS accounts and roles from SSO
- Creates profiles in `~/.aws/config`
- Preserves existing non-SSO profiles (unless `--reset`)
- Sanitizes profile names (lowercase, hyphens)
- Sorts profiles alphabetically

**Profile Naming Convention:**
- Format: `{account-name}-{role-name}`
- Example: `production-administratoraccess`
- Sanitization: Spaces and underscores become hyphens

**Example Output:**
```
🔄 Starting AWS SSO profile synchronization...
🔐 Authenticating with AWS SSO: https://mycompany.awsapps.com/start
📋 Found 12 profiles in AWS SSO
📊 Added 8 new profiles, updated 4 existing profiles
✅ Successfully synchronized 12 SSO profiles to AWS config
```

#### `synacklab auth aws-config`

Configure default AWS profile interactively.

```bash
# Interactive profile selection
synacklab auth aws-config

# Use custom configuration file
synacklab auth aws-config --config /path/to/config.yaml
```

**Features:**
- Lists all available AWS profiles
- Shows account ID and role name
- Interactive fuzzy search
- Updates `[default]` section in `~/.aws/config`

**Example Output:**
```
⚙️  Configuring AWS default profile...

📋 Available AWS profiles:
1. production-administratoraccess (Account: 123456789012, Role: AdministratorAccess)
2. development-poweruseraccess (Account: 987654321098, Role: PowerUserAccess)
3. staging-readonlyaccess (Account: 456789012345, Role: ReadOnlyAccess)

Select profile number to set as default: 1
✅ Successfully set 'production-administratoraccess' as the default AWS profile
```

### Profile Management

#### Generated AWS Configuration

Synacklab creates AWS profiles in the standard format:

```ini
[default]
sso_start_url = https://mycompany.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = AdministratorAccess
region = us-east-1
output = json

[profile production-administratoraccess]
sso_start_url = https://mycompany.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = AdministratorAccess
region = us-east-1
output = json

[profile development-poweruseraccess]
sso_start_url = https://mycompany.awsapps.com/start
sso_region = us-east-1
sso_account_id = 987654321098
sso_role_name = PowerUserAccess
region = us-east-1
output = json
```

#### Using AWS Profiles

After synchronization, use profiles with AWS CLI:

```bash
# Use default profile
aws s3 ls

# Use specific profile
aws s3 ls --profile production-administratoraccess

# Set profile for session
export AWS_PROFILE=production-administratoraccess
aws s3 ls
```

## Workflows

### Daily Authentication Workflow

```bash
# 1. Check authentication status
synacklab auth aws-login

# 2. Sync latest profiles (if needed)
synacklab auth sync

# 3. Set default profile for the day
synacklab auth aws-config

# 4. Verify AWS access
aws sts get-caller-identity
```

### Multi-Account Workflow

```bash
# Sync all accounts and roles
synacklab auth sync

# Switch to production account
export AWS_PROFILE=production-administratoraccess
aws sts get-caller-identity

# Switch to development account
export AWS_PROFILE=development-poweruseraccess
aws sts get-caller-identity

# Switch to staging with read-only access
export AWS_PROFILE=staging-readonlyaccess
aws s3 ls
```

### Automated Workflow

```bash
#!/bin/bash
# Daily AWS setup script

echo "Setting up AWS environment..."

# Authenticate (will use cached session if valid)
synacklab auth aws-login

# Sync profiles to get any new accounts/roles
synacklab auth sync

# Set production as default for the day
echo "1" | synacklab auth aws-config

echo "AWS environment ready!"
aws sts get-caller-identity
```

## Session Management

### Session Lifecycle

1. **Authentication**: Device flow creates session token
2. **Storage**: Token stored securely in `~/.synacklab/cache/`
3. **Validation**: Automatic validation before operations
4. **Refresh**: Automatic refresh when possible
5. **Expiration**: Clear expired tokens automatically

### Session Information

Check current session status:

```bash
# Authentication command shows session info
synacklab auth aws-login

# Example output includes:
# 📍 SSO URL: https://mycompany.awsapps.com/start
# 🌍 Region: us-east-1
# ⏰ Session expires: 2024-12-07 15:30:45 PST
```

### Manual Session Management

```bash
# Force re-authentication
rm -rf ~/.synacklab/cache/
synacklab auth aws-login

# Check AWS CLI session status
aws sts get-caller-identity

# Login to AWS SSO for CLI access
aws sso login --profile production-administratoraccess
```

## Integration with AWS CLI

### Compatibility

Synacklab-generated profiles are fully compatible with:
- AWS CLI v2 (recommended)
- AWS CLI v1 (basic support)
- AWS SDKs
- Third-party tools using AWS credentials

### Enhanced Integration

With AWS CLI v2:

```bash
# Automatic SSO login
aws sso login --profile production-administratoraccess

# List all SSO sessions
aws sso list-accounts

# Logout from SSO
aws sso logout
```

## Troubleshooting

### Common Issues

#### Authentication Timeout

```bash
# Increase timeout
synacklab auth aws-login --timeout 600

# Check network connectivity
curl -I https://device.sso.us-east-1.amazonaws.com/
```

#### No Profiles Found

```bash
# Verify SSO configuration
cat ~/.synacklab/config.yaml

# Check SSO permissions
# Contact AWS administrator to verify account access
```

#### Profile Sync Issues

```bash
# Clear cache and retry
rm -rf ~/.synacklab/cache/
synacklab auth aws-login
synacklab auth sync

# Check AWS config file
cat ~/.aws/config
```

#### Permission Errors

```bash
# Check file permissions
ls -la ~/.aws/config
chmod 644 ~/.aws/config

# Check directory permissions
ls -la ~/.aws/
chmod 755 ~/.aws/
```

### Debug Mode

Enable debug logging for troubleshooting:

```bash
export SYNACKLAB_LOG_LEVEL="debug"
synacklab auth aws-login
```

### Validation Commands

```bash
# Test AWS SSO connectivity
curl -I "https://your-company.awsapps.com/start"

# Validate AWS CLI integration
aws configure list-profiles

# Test profile access
aws sts get-caller-identity --profile production-administratoraccess
```

## Security Considerations

### Token Storage

- Session tokens stored in `~/.synacklab/cache/`
- Files protected with restrictive permissions (600)
- Tokens automatically expire based on SSO policy
- No long-term credentials stored

### Best Practices

1. **Regular Re-authentication**: Don't disable session expiration
2. **Secure Workstation**: Use on trusted devices only
3. **Profile Naming**: Avoid sensitive information in profile names
4. **Access Review**: Regularly review AWS SSO access grants
5. **Shared Systems**: Don't use on shared/public computers

### Compliance

- Supports AWS SSO audit logging
- No credential storage in plaintext
- Respects organizational SSO policies
- Compatible with MFA requirements

## Advanced Usage

### Custom Profile Configuration

Override default profile settings:

```yaml
aws:
  sso:
    start_url: "https://company.awsapps.com/start"
    region: "us-east-1"
    default_region: "us-west-2"  # Different default region
    default_output: "table"      # Different output format
```

### Scripting and Automation

```bash
#!/bin/bash
# Automated AWS profile setup

# Function to check authentication
check_auth() {
    if ! synacklab auth aws-login 2>/dev/null; then
        echo "Authentication required"
        synacklab auth aws-login
    fi
}

# Function to ensure profiles are current
sync_profiles() {
    echo "Syncing AWS profiles..."
    synacklab auth sync
}

# Main workflow
check_auth
sync_profiles

echo "AWS environment ready for automation"
```

### Integration with Other Tools

```bash
# Use with Terraform
export AWS_PROFILE=production-administratoraccess
terraform plan

# Use with kubectl (for EKS)
aws eks update-kubeconfig --region us-east-1 --name my-cluster

# Use with Docker (for ECR)
aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 123456789012.dkr.ecr.us-east-1.amazonaws.com
```

## Next Steps

- [Set up Kubernetes management](kubernetes.md)
- [Configure GitHub integration](github.md)
- [Review all commands](commands.md)
- [Check troubleshooting guide](troubleshooting.md)