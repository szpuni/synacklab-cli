# GitHub Authentication and Permissions Guide

This guide covers setting up authentication and understanding the required permissions for using synacklab's GitHub repository management features.

## Table of Contents

- [Authentication Methods](#authentication-methods)
- [Creating a GitHub Personal Access Token](#creating-a-github-personal-access-token)
- [Required Token Permissions](#required-token-permissions)
- [Configuration Options](#configuration-options)
- [Organization vs Personal Repositories](#organization-vs-personal-repositories)
- [Security Best Practices](#security-best-practices)
- [Troubleshooting](#troubleshooting)

## Authentication Methods

Synacklab supports GitHub authentication through Personal Access Tokens (PATs). The token can be provided in two ways:

1. **Environment Variable** (Recommended)
2. **Configuration File**

### Environment Variable Authentication

Set the `GITHUB_TOKEN` environment variable:

```bash
# For current session
export GITHUB_TOKEN="ghp_your_token_here"

# For permanent setup (add to ~/.bashrc, ~/.zshrc, etc.)
echo 'export GITHUB_TOKEN="ghp_your_token_here"' >> ~/.bashrc
source ~/.bashrc
```

### Configuration File Authentication

Add the token to your synacklab configuration file:

```yaml
# ~/.synacklab/config.yaml
github:
  token: "ghp_your_token_here"
  organization: "your-organization"  # Optional: default organization
```

**Note**: Environment variables take precedence over configuration file settings.

## Creating a GitHub Personal Access Token

### Step 1: Access Token Settings

1. Log in to GitHub and go to **Settings** (click your profile picture → Settings)
2. In the left sidebar, click **Developer settings**
3. Click **Personal access tokens** → **Tokens (classic)**
4. Click **Generate new token** → **Generate new token (classic)**

### Step 2: Configure Token Settings

1. **Note**: Give your token a descriptive name (e.g., "synacklab-cli-token")
2. **Expiration**: Choose an appropriate expiration date
   - For production use: 90 days or 1 year
   - For testing: 30 days
3. **Select scopes**: Choose the required permissions (see next section)

### Step 3: Generate and Save Token

1. Click **Generate token**
2. **Important**: Copy the token immediately - you won't be able to see it again
3. Store the token securely (password manager, secure notes, etc.)

## Required Token Permissions

Your GitHub token must have the following scopes for full functionality:

### Essential Scopes

#### `repo` - Full control of private repositories
- **Required for**: Creating, reading, updating repositories
- **Includes**:
  - `repo:status` - Access commit status
  - `repo_deployment` - Access deployment status
  - `public_repo` - Access public repositories
  - `repo:invite` - Access repository invitations
  - `security_events` - Read and write security events

#### `admin:repo_hook` - Full control of repository hooks
- **Required for**: Managing webhooks
- **Includes**:
  - `write:repo_hook` - Write repository hooks
  - `read:repo_hook` - Read repository hooks

### Organization Scopes (Required for Organization Repositories)

#### `admin:org` - Full control of orgs and teams
- **Required for**: Managing team access to repositories
- **Includes**:
  - `write:org` - Write org and team membership
  - `read:org` - Read org and team membership
  - `manage_runners:org` - Manage org runners and runner groups

### Optional Scopes

#### `user` - Update ALL user data
- **Required for**: Validating user existence in collaborator management
- **Includes**:
  - `read:user` - Read user profile data
  - `user:email` - Access user email addresses

### Minimal Scope Configuration

For basic repository management without team features:

```
✅ repo
✅ admin:repo_hook
❌ admin:org (only if not managing teams)
❌ user (only if not validating users)
```

For full organizational repository management:

```
✅ repo
✅ admin:repo_hook
✅ admin:org
✅ user
```

## Configuration Options

### Basic Configuration

```yaml
# ~/.synacklab/config.yaml
github:
  token: "ghp_your_token_here"
```

### Advanced Configuration

```yaml
# ~/.synacklab/config.yaml
github:
  token: "ghp_your_token_here"
  organization: "my-company"        # Default organization for repositories
  api_url: "https://api.github.com" # GitHub API URL (for GitHub Enterprise)
  timeout: 30                       # API request timeout in seconds
```

### Environment Variable Override

Environment variables override configuration file settings:

```bash
# These environment variables take precedence
export GITHUB_TOKEN="ghp_different_token"
export GITHUB_ORGANIZATION="different-org"
export GITHUB_API_URL="https://github.enterprise.com/api/v3"
```

## Organization vs Personal Repositories

### Personal Repositories

For repositories under your personal GitHub account:

- **Required scopes**: `repo`, `admin:repo_hook`
- **Configuration**: No organization setting needed
- **Repository format**: `username/repository-name`

```yaml
# Configuration for personal repositories
github:
  token: "ghp_your_token_here"
```

### Organization Repositories

For repositories under a GitHub organization:

- **Required scopes**: `repo`, `admin:repo_hook`, `admin:org`
- **Configuration**: Set organization in config or environment
- **Repository format**: `organization/repository-name`
- **Additional requirements**: You must be an organization member with appropriate permissions

```yaml
# Configuration for organization repositories
github:
  token: "ghp_your_token_here"
  organization: "my-company"
```

### Organization Permissions

Within the organization, your account needs:

1. **Repository creation permissions** (if creating new repositories)
2. **Team management permissions** (if managing team access)
3. **Webhook management permissions** (if configuring webhooks)

## Security Best Practices

### Token Security

1. **Use environment variables**: Avoid storing tokens in configuration files
2. **Limit token scope**: Only grant necessary permissions
3. **Set expiration dates**: Use reasonable expiration periods
4. **Rotate tokens regularly**: Replace tokens periodically
5. **Monitor token usage**: Review token activity in GitHub settings

### Access Control

1. **Principle of least privilege**: Grant minimum required permissions
2. **Separate tokens for different purposes**: Use different tokens for different tools/environments
3. **Audit token permissions**: Regularly review and update token scopes
4. **Revoke unused tokens**: Remove tokens that are no longer needed

### Configuration File Security

If using configuration files:

```bash
# Set restrictive permissions on config file
chmod 600 ~/.synacklab/config.yaml

# Ensure config directory is secure
chmod 700 ~/.synacklab/
```

### Environment Variable Security

```bash
# Add to shell profile with restricted access
echo 'export GITHUB_TOKEN="ghp_your_token_here"' >> ~/.bashrc
chmod 600 ~/.bashrc
```

## Troubleshooting

### Authentication Failures

#### Error: "Bad credentials"

**Cause**: Invalid or expired token
**Solution**:
1. Verify token is correctly set: `echo $GITHUB_TOKEN`
2. Check token hasn't expired in GitHub settings
3. Regenerate token if necessary

#### Error: "Not Found" when accessing repository

**Cause**: Token lacks repository access permissions
**Solution**:
1. Verify repository exists and is accessible
2. Check token has `repo` scope
3. For organization repos, ensure you're a member

### Permission Errors

#### Error: "Resource not accessible by integration"

**Cause**: Token lacks required permissions for the operation
**Solution**:
1. Review required scopes for your use case
2. Update token permissions in GitHub settings
3. For organization features, ensure `admin:org` scope

#### Error: "Must have admin rights to Repository"

**Cause**: Token lacks administrative access to repository
**Solution**:
1. Ensure you have admin rights to the repository
2. For organization repos, check your organization role
3. Contact repository/organization admin for access

### Configuration Issues

#### Error: "No GitHub token found"

**Cause**: Token not configured in environment or config file
**Solution**:
1. Set `GITHUB_TOKEN` environment variable
2. Or add token to `~/.synacklab/config.yaml`
3. Verify configuration file path and format

#### Error: "Invalid configuration format"

**Cause**: Malformed YAML in configuration file
**Solution**:
1. Validate YAML syntax
2. Check indentation (use spaces, not tabs)
3. Verify required fields are present

### API Rate Limiting

#### Error: "API rate limit exceeded"

**Cause**: Too many API requests in short time period
**Solution**:
1. Wait for rate limit reset (shown in error message)
2. Reduce frequency of operations
3. Consider using GitHub Apps for higher rate limits

### Network Issues

#### Error: "Connection timeout" or "Network unreachable"

**Cause**: Network connectivity issues
**Solution**:
1. Check internet connection
2. Verify GitHub API is accessible
3. Check firewall/proxy settings
4. For GitHub Enterprise, verify API URL configuration

## Getting Help

If you continue to experience authentication issues:

1. **Check GitHub Status**: Visit [githubstatus.com](https://githubstatus.com)
2. **Review Token Permissions**: Ensure all required scopes are enabled
3. **Test with GitHub CLI**: Verify token works with `gh auth status`
4. **Contact Support**: Reach out to your organization's GitHub administrator

## Example: Complete Setup

Here's a complete example of setting up authentication:

```bash
# 1. Create and export token
export GITHUB_TOKEN="ghp_your_token_here"

# 2. Create synacklab config (optional)
mkdir -p ~/.synacklab
cat > ~/.synacklab/config.yaml << EOF
github:
  organization: "my-company"
EOF

# 3. Test authentication
synacklab github validate examples/github-simple-repo.yaml

# 4. Apply a configuration
synacklab github apply examples/github-simple-repo.yaml --dry-run
```

This setup provides secure, functional authentication for all synacklab GitHub features.