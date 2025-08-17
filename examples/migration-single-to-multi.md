# Migration Guide: Single to Multi-Repository Configuration

This guide demonstrates how to migrate from single repository configurations to multi-repository format.

## Before: Single Repository Configuration

```yaml
# Original single repository format (github-simple-repo.yaml)
name: "my-service"
description: "My microservice application"
private: true
topics:
  - golang
  - microservice
  - api

features:
  issues: true
  wiki: false
  projects: true
  discussions: false

branch_protection:
  - pattern: "main"
    required_reviews: 2
    dismiss_stale_reviews: true
    require_code_owner_review: true
    required_status_checks:
      strict: true
      contexts:
        - "ci/build"
        - "ci/test"

collaborators:
  - username: "team-lead"
    permission: "admin"

teams:
  - team: "backend-team"
    permission: "write"
  - team: "devops-team"
    permission: "admin"

webhooks:
  - url: "https://ci.example.com/webhook/github"
    events: ["push", "pull_request"]
    active: true
```

## After: Multi-Repository Configuration

### Option 1: Direct Migration (Single Repository in Multi-Repo Format)

```yaml
# Migrated to multi-repository format
version: "1.0"

repositories:
  - name: "my-service"
    description: "My microservice application"
    private: true
    topics:
      - golang
      - microservice
      - api
    
    features:
      issues: true
      wiki: false
      projects: true
      discussions: false
    
    branch_protection:
      - pattern: "main"
        required_reviews: 2
        dismiss_stale_reviews: true
        require_code_owner_review: true
        required_status_checks:
          strict: true
          contexts:
            - "ci/build"
            - "ci/test"
    
    collaborators:
      - username: "team-lead"
        permission: "admin"
    
    teams:
      - team: "backend-team"
        permission: "write"
      - team: "devops-team"
        permission: "admin"
    
    webhooks:
      - url: "https://ci.example.com/webhook/github"
        events: ["push", "pull_request"]
        active: true
```

### Option 2: Migration with Defaults (Recommended for Multiple Services)

```yaml
# Optimized multi-repository format with defaults
version: "1.0"

# Extract common settings as defaults
defaults:
  private: true
  topics:
    - microservice
  
  features:
    issues: true
    wiki: false
    projects: true
    discussions: false
  
  branch_protection:
    - pattern: "main"
      required_reviews: 2
      dismiss_stale_reviews: true
      require_code_owner_review: true
      required_status_checks:
        strict: true
        contexts:
          - "ci/build"
          - "ci/test"
  
  collaborators:
    - username: "team-lead"
      permission: "admin"
  
  teams:
    - team: "backend-team"
      permission: "write"
    - team: "devops-team"
      permission: "admin"
  
  webhooks:
    - url: "https://ci.example.com/webhook/github"
      events: ["push", "pull_request"]
      active: true

repositories:
  - name: "my-service"
    description: "My microservice application"
    topics:
      - golang
      - api
    # Inherits all defaults, adds specific topics
  
  # Easy to add more repositories with consistent settings
  - name: "another-service"
    description: "Another microservice"
    topics:
      - python
      - worker
```

## Migration Steps

### Step 1: Backup Current Configuration
```bash
# Backup your existing configuration
cp github-simple-repo.yaml github-simple-repo.yaml.backup
```

### Step 2: Create Multi-Repository Structure
```bash
# Create new multi-repository configuration
cat > github-multi-repo.yaml << 'EOF'
version: "1.0"
repositories:
  - name: "existing-repo-name"
    # ... copy existing configuration here
EOF
```

### Step 3: Test the Migration
```bash
# Validate the new configuration
synacklab github validate github-multi-repo.yaml --owner myorg

# Dry-run to see what changes (should be none for exact migration)
synacklab github apply github-multi-repo.yaml --dry-run --owner myorg
```

### Step 4: Apply the New Configuration
```bash
# Apply the migrated configuration
synacklab github apply github-multi-repo.yaml --owner myorg
```

### Step 5: Test Selective Operations (Optional)
```bash
# Test selective repository operations
synacklab github validate github-multi-repo.yaml --repos existing-repo-name --owner myorg
synacklab github apply github-multi-repo.yaml --repos existing-repo-name --dry-run --owner myorg
```

## Selective Repository Operations with --repos Flag

The multi-repository format supports selective operations using the `--repos` flag, allowing you to work with specific repositories from your configuration.

### Basic Usage
```bash
# Operate on a single repository
synacklab github apply multi-repos.yaml --repos my-service

# Operate on multiple repositories
synacklab github apply multi-repos.yaml --repos "user-service,payment-service,order-service"

# Validate specific repositories
synacklab github validate multi-repos.yaml --repos user-service,payment-service
```

### Use Cases for Selective Operations

#### 1. Gradual Rollout
```bash
# Test changes on a single repository first
synacklab github apply multi-repos.yaml --repos user-service --dry-run
synacklab github apply multi-repos.yaml --repos user-service

# Then apply to remaining repositories
synacklab github apply multi-repos.yaml --repos "payment-service,order-service"
```

#### 2. Team-Specific Operations
```bash
# Apply changes only to frontend repositories
synacklab github apply multi-repos.yaml --repos "frontend-app,admin-dashboard"

# Apply changes only to backend services
synacklab github apply multi-repos.yaml --repos "user-service,payment-service,notification-service"
```

#### 3. Environment-Specific Deployments
```bash
# Apply to staging repositories first
synacklab github apply multi-repos.yaml --repos "user-service-staging,payment-service-staging"

# Then apply to production repositories
synacklab github apply multi-repos.yaml --repos "user-service,payment-service"
```

#### 4. Troubleshooting and Recovery
```bash
# Validate only problematic repositories
synacklab github validate multi-repos.yaml --repos problematic-repo

# Re-apply configuration to specific repositories that failed
synacklab github apply multi-repos.yaml --repos "failed-repo1,failed-repo2"
```

### Repository Name Validation
The system validates that all repository names specified in `--repos` exist in the configuration:

```bash
# This will fail if 'nonexistent-repo' is not in the configuration
synacklab github apply multi-repos.yaml --repos "user-service,nonexistent-repo"
# Error: repositories not found in configuration: nonexistent-repo
```

### Combining with Other Flags
```bash
# Selective dry-run with specific owner
synacklab github apply multi-repos.yaml --repos user-service --dry-run --owner myorg

# Selective validation with owner context for team validation
synacklab github validate multi-repos.yaml --repos "user-service,payment-service" --owner myorg
```

## Migration Checklist

- [ ] Backup original configuration file
- [ ] Convert single repository to multi-repository format
- [ ] Add `version: "1.0"` field
- [ ] Wrap repository configuration in `repositories` array
- [ ] Consider extracting common settings to `defaults` section
- [ ] Validate new configuration with `synacklab github validate`
- [ ] Test with dry-run to ensure no unexpected changes
- [ ] Test selective operations with `--repos` flag
- [ ] Apply new configuration
- [ ] Verify repository settings in GitHub UI
- [ ] Update CI/CD pipelines to use new configuration file
- [ ] Update documentation and team processes
- [ ] Train team on selective operation capabilities

## Common Migration Patterns

### Pattern 1: Multiple Similar Services
If you have multiple similar services, extract common settings to defaults:

```yaml
defaults:
  private: true
  topics: ["microservice", "production"]
  features:
    issues: true
    projects: true
  # ... other common settings

repositories:
  - name: "user-service"
    description: "User management service"
    topics: ["golang", "users"]
  
  - name: "order-service"
    description: "Order processing service"
    topics: ["golang", "orders"]
```

### Pattern 2: Different Service Types
For different types of services, use selective overrides:

```yaml
defaults:
  private: true
  features:
    issues: true

repositories:
  - name: "backend-api"
    topics: ["golang", "api"]
    teams:
      - team: "backend-team"
        permission: "admin"
  
  - name: "frontend-app"
    topics: ["react", "frontend"]
    teams:
      - team: "frontend-team"
        permission: "admin"
```

### Pattern 3: Gradual Migration
Migrate one repository at a time by adding to the multi-repository configuration:

```yaml
version: "1.0"

repositories:
  # Start with one migrated repository
  - name: "first-service"
    # ... configuration
  
  # Add more repositories over time
  # - name: "second-service"
  #   # ... configuration
```

## Advanced Multi-Repository Features

### Error Handling and Partial Failures
The multi-repository system handles failures gracefully:

```bash
# If some repositories fail, others continue processing
synacklab github apply multi-repos.yaml
# Output shows success/failure status for each repository
# Exit code indicates partial failure if any repositories failed
```

### Rate Limiting and Performance
The system automatically handles GitHub API rate limits:

- Intelligent rate limiting across multiple repositories
- Exponential backoff and retry logic
- Parallel processing where safe
- Progress reporting for long-running operations

### Comprehensive Reporting
Multi-repository operations provide detailed reporting:

```bash
# Example output from multi-repository apply
✅ Successfully applied changes to 3 repositories

✅ Successful repositories:
  • myorg/user-service: https://github.com/myorg/user-service
  • myorg/payment-service: https://github.com/myorg/payment-service
  • myorg/order-service: https://github.com/myorg/order-service

📊 Summary:
  • Total repositories: 3
  • Successful: 3
  • Failed: 0
  • Total changes applied: 12
```

## Best Practices for Multi-Repository Management

### 1. Use Meaningful Repository Names
```yaml
repositories:
  # Good: Clear, descriptive names
  - name: "user-authentication-service"
  - name: "payment-processing-api"
  - name: "order-management-system"
  
  # Avoid: Generic or unclear names
  # - name: "service1"
  # - name: "api"
```

### 2. Organize with Consistent Defaults
```yaml
# Extract common organizational policies
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
  teams:
    - team: "devops-team"
      permission: "admin"
```

### 3. Use Selective Operations for Safety
```bash
# Always test critical changes on a subset first
synacklab github apply multi-repos.yaml --repos "test-service" --dry-run
synacklab github apply multi-repos.yaml --repos "test-service"

# Then apply to remaining repositories
synacklab github apply multi-repos.yaml --repos "prod-service1,prod-service2"
```

### 4. Version Control Your Configurations
```bash
# Keep configurations in version control
git add multi-repos.yaml
git commit -m "Add payment-service to multi-repo configuration"

# Tag important configuration changes
git tag -a v1.2.0 -m "Updated branch protection rules for all services"
```

### 5. Document Repository Groupings
```yaml
# Use comments to document repository groupings
repositories:
  # Core API Services
  - name: "user-service"
    # ...
  - name: "auth-service"
    # ...
  
  # Data Processing Services
  - name: "analytics-service"
    # ...
  - name: "reporting-service"
    # ...
```