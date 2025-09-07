# GitHub Repository Management

Synacklab provides comprehensive GitHub repository management through declarative YAML configuration. This guide covers all GitHub-related features, from basic repository creation to advanced multi-repository operations.

## Overview

GitHub management in Synacklab includes:

- **Declarative Configuration**: Define repositories as code using YAML
- **Multi-Repository Support**: Manage multiple repositories in batch operations
- **Branch Protection**: Configure branch protection rules and policies
- **Access Control**: Manage collaborators and team permissions
- **Webhook Management**: Configure webhooks for CI/CD and notifications
- **Validation**: Comprehensive validation before applying changes
- **Dry-Run Mode**: Preview changes before applying them

## Prerequisites

- **GitHub Account**: Personal or organization account
- **GitHub Token**: Personal Access Token with appropriate permissions
- **Repository Permissions**: Admin access to repositories you want to manage

### GitHub Token Setup

Create a Personal Access Token with these scopes:

1. Go to GitHub Settings → Developer settings → Personal access tokens
2. Generate a new token with these scopes:
   - `repo` - Full control of private repositories
   - `admin:org` - Full control of orgs and teams
   - `admin:repo_hook` - Full control of repository hooks

3. Store the token securely:

```bash
# Environment variable (recommended)
export GITHUB_TOKEN="ghp_your_token_here"

# Or in configuration file
echo 'github:
  token: "ghp_your_token_here"
  organization: "your-org"' >> ~/.synacklab/config.yaml
```

## Configuration Formats

Synacklab supports two configuration formats:

### Single Repository Format

Traditional format for managing one repository per file:

```yaml
# my-repo.yaml
name: my-awesome-repo
description: "A single repository configuration"
private: false

features:
  issues: true
  wiki: true
  projects: false

branch_protection:
  - pattern: "main"
    required_status_checks:
      - "ci/build"
    required_reviews: 2
```

### Multi-Repository Format

Advanced format for managing multiple repositories with shared defaults:

```yaml
# multi-repos.yaml
version: "1.0"

# Global defaults applied to all repositories
defaults:
  private: true
  features:
    issues: true
    wiki: false
    projects: true
  
  branch_protection:
    - pattern: "main"
      required_reviews: 2
      require_code_owner_review: true

# Individual repositories
repositories:
  - name: frontend-app
    description: "React frontend application"
    topics: [react, frontend, typescript]
    
  - name: backend-api
    description: "REST API backend service"
    private: false  # Override default
    topics: [golang, api, backend]
```

## Commands

### Repository Validation

#### `synacklab github validate <config-file.yaml>`

Validate repository configuration files before applying changes.

```bash
# Validate single repository
synacklab github validate my-repo.yaml

# Validate with owner context
synacklab github validate my-repo.yaml --owner myorg

# Validate multi-repository configuration
synacklab github validate multi-repos.yaml --owner myorg

# Validate specific repositories only
synacklab github validate multi-repos.yaml --repos repo1,repo2
```

**Validation Checks:**
- YAML syntax and structure
- Required fields and valid values
- GitHub user and team existence
- Repository permissions
- Configuration format compatibility

**Example Output:**
```
🔍 Validating configuration file: my-repo.yaml
✓ YAML syntax and basic validation passed
📋 Configuration format: single-repository
📦 Validating single repository: my-awesome-repo
✓ Authenticated as myusername
🔍 Performing GitHub API validation...
   Checking collaborator usernames...
     - john-doe
     - jane-smith
✓ All users and teams exist
🔍 Checking repository permissions...
✓ Sufficient permissions for repository operations

✅ Configuration file is valid and ready to apply
```

### Repository Application

#### `synacklab github apply <config-file.yaml>`

Apply repository configuration to GitHub, creating or updating repositories.

```bash
# Apply single repository configuration
synacklab github apply my-repo.yaml

# Preview changes without applying
synacklab github apply my-repo.yaml --dry-run

# Apply with specific owner
synacklab github apply my-repo.yaml --owner myorg

# Apply multi-repository configuration
synacklab github apply multi-repos.yaml --owner myorg

# Apply to specific repositories only
synacklab github apply multi-repos.yaml --repos repo1,repo2 --owner myorg
```

**Features:**
- Creates new repositories or updates existing ones
- Reconciles actual state with desired configuration
- Shows detailed change plans before applying
- Supports batch operations across multiple repositories
- Handles failures gracefully with detailed reporting

**Example Output:**
```
✓ Authenticated as myusername
✓ Configuration validated

📋 Planned changes for myorg/my-awesome-repo:
  + Repository: CREATE new repository
    - Name: my-awesome-repo
    - Description: A single repository configuration
    - Private: false
  + Branch Protection: CREATE rule for main
    - Required reviews: 2
    - Required status checks: ci/build
  + Collaborator: ADD john-doe with write permission

Total changes: 3

Applying changes...
✅ Successfully applied changes to myorg/my-awesome-repo
🎉 Repository created: https://github.com/myorg/my-awesome-repo
```

## Configuration Reference

### Repository Settings

```yaml
# Basic repository information
name: "repository-name"          # Required: Repository name
description: "Repository description"  # Optional: Repository description
private: true                    # Required: true for private, false for public

# Repository topics for discoverability
topics:
  - golang
  - cli
  - devops

# Repository features
features:
  issues: true          # Enable GitHub Issues
  wiki: false          # Enable repository wiki
  projects: true       # Enable GitHub Projects
  discussions: false   # Enable GitHub Discussions
```

### Branch Protection Rules

```yaml
branch_protection:
  - pattern: "main"                    # Branch pattern (supports glob)
    required_status_checks:            # Required CI checks
      - "ci/build"
      - "ci/test"
      - "security/scan"
    require_up_to_date: true          # Require branches to be up to date
    required_reviews: 2               # Number of required reviews
    dismiss_stale_reviews: true       # Dismiss stale reviews on new commits
    require_code_owner_review: true   # Require code owner review
    restrict_pushes:                  # Restrict who can push
      - "admin-team"
      - "release-team"

  - pattern: "release/*"              # Protect release branches
    required_status_checks:
      - "ci/build"
    required_reviews: 1
    require_code_owner_review: true
```

### Access Control

```yaml
# Individual collaborator access
collaborators:
  - username: "john-doe"
    permission: "write"    # read, write, admin
  - username: "jane-smith"
    permission: "admin"

# Team-based access control (requires organization)
teams:
  - team: "backend-team"
    permission: "write"    # read, write, admin
  - team: "devops-team"
    permission: "admin"
```

### Webhook Configuration

```yaml
webhooks:
  - url: "https://ci.example.com/webhook/github"
    events:
      - "push"
      - "pull_request"
      - "release"
    secret: "${WEBHOOK_SECRET_CI}"    # Environment variable
    active: true

  - url: "https://hooks.slack.com/services/..."
    events:
      - "push"
      - "issues"
      - "pull_request"
    active: true
```

### Multi-Repository Configuration

```yaml
version: "1.0"

# Global defaults (optional)
defaults:
  private: true
  features:
    issues: true
    wiki: false
    projects: true
  
  # Default branch protection for all repos
  branch_protection:
    - pattern: "main"
      required_reviews: 2
      require_code_owner_review: true
      required_status_checks:
        - "ci/build"

# Individual repositories
repositories:
  - name: "frontend-app"
    description: "React frontend application"
    topics: [react, frontend, typescript]
    # Inherits defaults, can override any setting
    
  - name: "backend-api"
    description: "REST API backend service"
    private: false  # Override default
    topics: [golang, api, backend]
    
    # Repository-specific settings
    collaborators:
      - username: "backend-lead"
        permission: "admin"
    
    # Additional branch protection
    branch_protection:
      - pattern: "develop"
        required_reviews: 1
        required_status_checks:
          - "ci/build"
```

## Workflows

### Single Repository Workflow

```bash
# 1. Create repository configuration
cat > my-repo.yaml << EOF
name: my-new-project
description: "A new project repository"
private: true

features:
  issues: true
  projects: true

branch_protection:
  - pattern: "main"
    required_reviews: 2
    required_status_checks:
      - "ci/build"
EOF

# 2. Validate configuration
synacklab github validate my-repo.yaml --owner myorg

# 3. Preview changes
synacklab github apply my-repo.yaml --owner myorg --dry-run

# 4. Apply configuration
synacklab github apply my-repo.yaml --owner myorg
```

### Multi-Repository Workflow

```bash
# 1. Create multi-repository configuration
cat > team-repos.yaml << EOF
version: "1.0"

defaults:
  private: true
  features:
    issues: true
    wiki: false
  teams:
    - team: "development-team"
      permission: "write"

repositories:
  - name: "user-service"
    description: "User management microservice"
  - name: "payment-service"
    description: "Payment processing microservice"
  - name: "notification-service"
    description: "Notification delivery microservice"
EOF

# 2. Validate all repositories
synacklab github validate team-repos.yaml --owner myorg

# 3. Preview changes for all repositories
synacklab github apply team-repos.yaml --owner myorg --dry-run

# 4. Apply to specific repositories first
synacklab github apply team-repos.yaml --owner myorg --repos user-service,payment-service

# 5. Apply to remaining repositories
synacklab github apply team-repos.yaml --owner myorg --repos notification-service
```

### Repository Update Workflow

```bash
# 1. Modify existing configuration
# Edit my-repo.yaml to add new collaborators or change settings

# 2. Validate changes
synacklab github validate my-repo.yaml --owner myorg

# 3. Preview what will change
synacklab github apply my-repo.yaml --owner myorg --dry-run

# 4. Apply updates
synacklab github apply my-repo.yaml --owner myorg
```

### Batch Operations Workflow

```bash
# Apply configuration to multiple repositories
synacklab github apply multi-repos.yaml --owner myorg

# Apply to specific subset
synacklab github apply multi-repos.yaml --owner myorg --repos "frontend-*,backend-*"

# Validate specific repositories
synacklab github validate multi-repos.yaml --repos service1,service2,service3
```

## Advanced Features

### Environment Variable Substitution

Use environment variables in configuration files:

```yaml
webhooks:
  - url: "https://ci.example.com/webhook"
    secret: "${WEBHOOK_SECRET}"
    events: ["push", "pull_request"]

collaborators:
  - username: "${LEAD_DEVELOPER}"
    permission: "admin"
```

```bash
# Set environment variables
export WEBHOOK_SECRET="my-secret-key"
export LEAD_DEVELOPER="john-doe"

# Apply configuration
synacklab github apply my-repo.yaml --owner myorg
```

### Template Repositories

Create template repositories for consistent project setup:

```yaml
# template-repo.yaml
name: "project-template"
description: "Template repository for new projects"
private: false

features:
  issues: true
  wiki: true
  projects: true

branch_protection:
  - pattern: "main"
    required_reviews: 2
    require_code_owner_review: true
    required_status_checks:
      - "ci/build"
      - "ci/test"
      - "security/scan"

# Standard team access
teams:
  - team: "developers"
    permission: "write"
  - team: "leads"
    permission: "admin"
```

### Configuration Inheritance

Use defaults effectively in multi-repository configurations:

```yaml
version: "1.0"

# Global defaults
defaults:
  private: true
  features:
    issues: true
    wiki: false
    projects: true
    discussions: false
  
  # Standard branch protection
  branch_protection:
    - pattern: "main"
      required_reviews: 2
      require_code_owner_review: true
      required_status_checks:
        - "ci/build"
        - "ci/test"
  
  # Standard team access
  teams:
    - team: "developers"
      permission: "write"
    - team: "devops"
      permission: "admin"

repositories:
  # Microservices inherit all defaults
  - name: "user-service"
    description: "User management service"
    topics: [microservice, golang, users]
  
  - name: "payment-service"
    description: "Payment processing service"
    topics: [microservice, golang, payments]
  
  # Frontend app with custom settings
  - name: "web-app"
    description: "Main web application"
    topics: [frontend, react, typescript]
    private: false  # Override: make public
    
    # Additional branch protection
    branch_protection:
      - pattern: "develop"
        required_reviews: 1
        required_status_checks:
          - "ci/build"
  
  # Documentation repo with minimal protection
  - name: "documentation"
    description: "Project documentation"
    private: false
    features:
      wiki: true
      discussions: true
    
    # Override: simpler branch protection
    branch_protection:
      - pattern: "main"
        required_reviews: 1
```

## Troubleshooting

### Common Issues

#### Authentication Errors

```bash
# Check token permissions
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user

# Verify token scopes
curl -H "Authorization: token $GITHUB_TOKEN" -I https://api.github.com/user

# Test organization access
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/orgs/myorg
```

#### Validation Failures

```bash
# Check YAML syntax
python -c "import yaml; yaml.safe_load(open('my-repo.yaml'))"

# Validate with verbose output
synacklab github validate my-repo.yaml --owner myorg --verbose

# Check specific user existence
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/users/username
```

#### Permission Errors

```bash
# Check repository permissions
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/repos/owner/repo

# Verify team access (for organizations)
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/orgs/myorg/teams

# Test webhook permissions
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/repos/owner/repo/hooks
```

#### Rate Limiting

```bash
# Check rate limit status
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/rate_limit

# Use smaller batches for multi-repository operations
synacklab github apply multi-repos.yaml --repos repo1,repo2 --owner myorg
# Wait, then continue with more repositories
```

### Debug Mode

Enable debug logging for troubleshooting:

```bash
export SYNACKLAB_LOG_LEVEL="debug"
synacklab github apply my-repo.yaml --owner myorg --dry-run
```

### Validation Commands

```bash
# Test GitHub API connectivity
curl -I https://api.github.com

# Validate token
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user

# Check organization membership
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user/orgs
```

## Security Best Practices

### Token Management

1. **Use Environment Variables**: Store tokens in environment variables, not config files
2. **Rotate Regularly**: Rotate tokens periodically
3. **Minimal Scopes**: Use only required token scopes
4. **Secure Storage**: Use secure credential storage in CI/CD systems

### Repository Security

1. **Branch Protection**: Always protect main/master branches
2. **Required Reviews**: Require code reviews for all changes
3. **Status Checks**: Require CI/CD checks before merging
4. **Team Access**: Use teams instead of individual collaborators
5. **Audit Logging**: Enable GitHub audit logging for organizations

### Configuration Security

1. **Version Control**: Store configurations in Git repositories
2. **Code Review**: Review configuration changes like code
3. **Environment Separation**: Use separate configurations for different environments
4. **Secret Management**: Use environment variables for secrets

## Integration Examples

### CI/CD Integration

```yaml
# .github/workflows/repository-management.yml
name: Repository Management
on:
  push:
    paths:
      - 'repositories/*.yaml'

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Validate Repository Configurations
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          for config in repositories/*.yaml; do
            synacklab github validate "$config" --owner ${{ github.repository_owner }}
          done

  apply:
    needs: validate
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Apply Repository Configurations
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          for config in repositories/*.yaml; do
            synacklab github apply "$config" --owner ${{ github.repository_owner }}
          done
```

### Terraform Integration

```hcl
# Use Synacklab for repository management, Terraform for infrastructure
resource "github_repository" "managed_by_synacklab" {
  # This repository is managed by Synacklab
  lifecycle {
    ignore_changes = [
      description,
      private,
      has_issues,
      has_wiki,
      has_projects,
    ]
  }
}
```

## Next Steps

- [Review AWS SSO integration](aws-sso.md)
- [Check Kubernetes management](kubernetes.md)
- [Browse command reference](commands.md)
- [Explore configuration examples](examples.md)