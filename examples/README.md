# GitHub Repository Configuration Examples

This directory contains example YAML configuration files demonstrating various GitHub repository management scenarios using synacklab.

## Quick Start

1. **Set up authentication**:
   ```bash
   export GITHUB_TOKEN="your_github_token"
   ```

2. **Validate an example**:
   ```bash
   synacklab github validate examples/github-simple-repo.yaml
   ```

3. **Apply with dry-run**:
   ```bash
   synacklab github apply examples/github-simple-repo.yaml --dry-run
   ```

4. **Apply the configuration**:
   ```bash
   synacklab github apply examples/github-simple-repo.yaml
   ```

## Example Files

### [`github-simple-repo.yaml`](./github-simple-repo.yaml)
**Use case**: Basic repository setup with minimal configuration

**Features demonstrated**:
- Basic repository settings (name, description, visibility)
- Repository topics for discoverability
- Simple feature toggles (issues, wiki, projects, discussions)

**Best for**: Getting started, simple projects, personal repositories

### [`github-complete-repo.yaml`](./github-complete-repo.yaml)
**Use case**: Comprehensive repository with all features configured

**Features demonstrated**:
- Complete repository configuration
- Multiple branch protection rules with different settings
- Individual collaborator and team-based access control
- Multiple webhooks with different event configurations
- Environment variable substitution for secrets

**Best for**: Production repositories, complex projects, learning all features

### [`github-team-repo.yaml`](./github-team-repo.yaml)
**Use case**: Repository optimized for team collaboration

**Features demonstrated**:
- Team-focused access control
- Collaboration-friendly branch protection
- Multiple branch patterns (main, feature branches)
- Team communication webhooks

**Best for**: Team projects, collaborative development, organization repositories

### [`github-open-source.yaml`](./github-open-source.yaml)
**Use case**: Public open source project configuration

**Features demonstrated**:
- Public repository settings
- Community-friendly features (all enabled)
- Balanced branch protection for open source
- Community notification webhooks
- Maintainer and contributor team structure

**Best for**: Open source projects, community-driven development

### [`github-config-reference.yaml`](./github-config-reference.yaml)
**Use case**: Complete configuration reference with detailed documentation

**Features demonstrated**:
- Every available configuration option
- Inline documentation for each setting
- Best practices and validation rules
- Environment variable usage examples

**Best for**: Reference documentation, understanding all options

## Configuration Patterns

### Basic Repository Pattern

```yaml
name: "repository-name"
description: "Repository description"
private: false
topics: ["topic1", "topic2"]
features:
  issues: true
  wiki: false
  projects: false
  discussions: false
```

### Branch Protection Pattern

```yaml
branch_protection:
  - pattern: "main"
    required_status_checks: ["ci/build", "ci/test"]
    require_up_to_date: true
    required_reviews: 2
    dismiss_stale_reviews: true
    require_code_owner_review: true
```

### Access Control Pattern

```yaml
# Team-based (recommended)
teams:
  - team: "developers"
    permission: "write"
  - team: "admins"
    permission: "admin"

# Individual collaborators
collaborators:
  - username: "external-contributor"
    permission: "read"
```

### Webhook Pattern

```yaml
webhooks:
  - url: "https://ci.example.com/webhook"
    events: ["push", "pull_request"]
    secret: "${WEBHOOK_SECRET}"
    active: true
```

## Common Use Cases

### 1. Creating a New Project Repository

**File**: `github-simple-repo.yaml` (modify as needed)

```bash
# Copy and customize
cp examples/github-simple-repo.yaml my-project.yaml
# Edit my-project.yaml with your settings
synacklab github apply my-project.yaml
```

### 2. Migrating Existing Repository to Code

1. Create configuration file based on current repository settings
2. Use `github-complete-repo.yaml` as a template
3. Apply with `--dry-run` first to verify changes
4. Apply the configuration

### 3. Standardizing Team Repositories

**File**: `github-team-repo.yaml` (customize for your team)

```bash
# Create team-specific template
cp examples/github-team-repo.yaml team-template.yaml
# Customize team names, permissions, and policies
# Apply to multiple repositories
```

### 4. Setting Up Open Source Project

**File**: `github-open-source.yaml`

```bash
# Customize for your project
cp examples/github-open-source.yaml my-oss-project.yaml
# Update project-specific settings
synacklab github apply my-oss-project.yaml
```

## Environment Variables

Many examples use environment variable substitution for sensitive data:

```bash
# Webhook secrets
export WEBHOOK_SECRET_CI="your-ci-webhook-secret"
export WEBHOOK_SECRET_SECURITY="your-security-webhook-secret"

# GitHub token (required)
export GITHUB_TOKEN="your-github-token"
```

## Validation and Testing

### Validate Configuration

```bash
# Validate syntax and basic rules
synacklab github validate examples/github-complete-repo.yaml

# Validate with GitHub API checks
synacklab github validate examples/github-complete-repo.yaml --check-remote
```

### Preview Changes

```bash
# See what would be changed without applying
synacklab github apply examples/github-team-repo.yaml --dry-run
```

### Test with Temporary Repository

```bash
# Create a test repository first
cp examples/github-simple-repo.yaml test-repo.yaml
# Change name to something like "test-synacklab-repo"
synacklab github apply test-repo.yaml
# Test your changes
# Delete test repository when done
```

## Customization Guide

### 1. Repository Naming

```yaml
# Use descriptive, consistent names
name: "project-backend-api"     # Good
name: "proj1"                   # Avoid

# Follow your organization's naming conventions
name: "team-service-component"  # Example pattern
```

### 2. Branch Protection Strategy

```yaml
# Production repositories - strict protection
branch_protection:
  - pattern: "main"
    required_reviews: 2
    require_code_owner_review: true
    dismiss_stale_reviews: true

# Development repositories - lighter protection
branch_protection:
  - pattern: "main"
    required_reviews: 1
    require_code_owner_review: false
```

### 3. Access Control Strategy

```yaml
# Prefer team-based access
teams:
  - team: "backend-developers"
    permission: "write"

# Use individual access sparingly
collaborators:
  - username: "external-consultant"
    permission: "read"
```

### 4. Webhook Configuration

```yaml
# Use environment variables for secrets
webhooks:
  - url: "https://ci.company.com/webhook"
    secret: "${CI_WEBHOOK_SECRET}"  # From environment
    events: ["push", "pull_request"]
```

## Best Practices

1. **Start Simple**: Begin with `github-simple-repo.yaml` and add features as needed
2. **Use Templates**: Create organization-specific templates based on these examples
3. **Version Control**: Store your configuration files in Git repositories
4. **Environment Variables**: Use environment variables for all secrets
5. **Validate First**: Always validate configurations before applying
6. **Dry Run**: Use `--dry-run` to preview changes
7. **Team Standards**: Establish consistent patterns across your organization
8. **Documentation**: Document any customizations or organization-specific patterns

## Troubleshooting

### Common Issues

1. **Invalid YAML syntax**: Use a YAML validator or `synacklab github validate`
2. **Missing permissions**: Ensure your GitHub token has required scopes
3. **Team not found**: Verify team slugs exist in your organization
4. **User not found**: Check that usernames are correct and users exist

### Getting Help

- Review the [authentication guide](../docs/github-authentication.md)
- Check the [configuration reference](./github-config-reference.yaml)
- Use `synacklab github validate` for detailed error messages
- Test with `--dry-run` before applying changes

## Contributing

When adding new examples:

1. Follow the existing naming pattern
2. Include comprehensive inline comments
3. Demonstrate specific use cases or patterns
4. Update this README with the new example
5. Test the example thoroughly before submitting