# Multi-Repository Configuration Best Practices

This guide provides best practices for managing multi-repository configurations effectively and securely.

## Configuration Organization

### 1. Use Environment-Specific Configurations

Organize configurations by environment or purpose:

```
configs/
├── production-repos.yaml      # Production repositories
├── staging-repos.yaml         # Staging/testing repositories  
├── development-repos.yaml     # Development repositories
├── open-source-repos.yaml     # Public open-source projects
└── archived-repos.yaml        # Archived/legacy repositories
```

### 2. Logical Grouping by Team or Domain

Group related repositories together:

```yaml
# backend-services.yaml
version: "1.0"
defaults:
  private: true
  topics: ["backend", "microservice"]
  teams:
    - team: "backend-team"
      permission: "admin"

repositories:
  - name: "user-service"
  - name: "payment-service"
  - name: "notification-service"
```

```yaml
# frontend-applications.yaml  
version: "1.0"
defaults:
  private: false
  topics: ["frontend", "react"]
  teams:
    - team: "frontend-team"
      permission: "admin"

repositories:
  - name: "customer-portal"
  - name: "admin-dashboard"
  - name: "mobile-app"
```

## Security Best Practices

### 1. Secure Defaults

Always configure secure defaults:

```yaml
defaults:
  private: true                    # Private by default
  features:
    security_and_analysis:
      secret_scanning: true        # Enable secret scanning
      secret_scanning_push_protection: true
      dependabot_security_updates: true
      dependabot_alerts: true
  
  branch_protection:
    - pattern: "main"
      required_reviews: 2
      dismiss_stale_reviews: true
      require_code_owner_review: true
      enforce_admins: true         # Apply to administrators too
      required_status_checks:
        strict: true
        contexts:
          - "security/scan"        # Require security scans
          - "ci/test"
```

### 2. Principle of Least Privilege

Grant minimal necessary permissions:

```yaml
defaults:
  collaborators:
    - username: "security-team"
      permission: "read"           # Read-only for security audits
  
  teams:
    - team: "developers"
      permission: "write"          # Write access for developers
    - team: "leads"
      permission: "admin"          # Admin only for team leads
```

### 3. Sensitive Repository Handling

Use stricter settings for sensitive repositories:

```yaml
repositories:
  - name: "payment-service"
    description: "Payment processing (PCI compliant)"
    branch_protection:
      - pattern: "main"
        required_reviews: 3        # More reviews for sensitive code
        required_status_checks:
          contexts:
            - "security/pci-scan"
            - "security/vulnerability-scan"
            - "compliance/audit"
    teams:
      - team: "security-team"
        permission: "admin"        # Security team oversight
```

## Performance and Scalability

### 1. Efficient Defaults Usage

Maximize defaults to minimize configuration duplication:

```yaml
# Good: Extensive defaults
defaults:
  private: true
  topics: ["production", "microservice"]
  features:
    issues: true
    projects: true
  branch_protection:
    - pattern: "main"
      required_reviews: 2
  teams:
    - team: "devops"
      permission: "admin"

repositories:
  - name: "service-a"
    topics: ["golang"]           # Only specify differences
  - name: "service-b" 
    topics: ["python"]
```

```yaml
# Avoid: Repetitive configuration
repositories:
  - name: "service-a"
    private: true                # Repeated in every repo
    topics: ["production", "microservice", "golang"]
    features:
      issues: true               # Repeated configuration
      projects: true
    # ... repeated settings
```

### 2. Selective Operations

Use repository filtering for large configurations:

```bash
# Apply changes to specific repositories only
synacklab github apply large-config.yaml --repos service-a,service-b

# Validate specific repositories
synacklab github validate large-config.yaml --repos payment-service
```

### 3. Batch Processing Considerations

For very large repository sets (100+ repos):

```yaml
# Consider splitting into smaller configuration files
# Process in batches to avoid rate limiting

# critical-services.yaml (high priority)
repositories:
  - name: "payment-service"
  - name: "user-service"

# standard-services.yaml (normal priority)  
repositories:
  - name: "logging-service"
  - name: "monitoring-service"
```

## Configuration Management

### 1. Version Control

Always version control your configurations:

```bash
# Store configurations in a dedicated repository
git init infrastructure-configs
cd infrastructure-configs

mkdir github-configs
cp *.yaml github-configs/

git add .
git commit -m "Initial GitHub repository configurations"
```

### 2. Configuration Validation in CI/CD

Validate configurations automatically:

```yaml
# .github/workflows/validate-configs.yml
name: Validate GitHub Configurations
on: [push, pull_request]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Validate configurations
        run: |
          for config in github-configs/*.yaml; do
            synacklab github validate "$config"
          done
```

### 3. Change Management Process

Implement a review process for configuration changes:

```yaml
# .github/workflows/apply-configs.yml
name: Apply GitHub Configurations
on:
  push:
    branches: [main]
    paths: ['github-configs/**']

jobs:
  apply:
    runs-on: ubuntu-latest
    environment: production  # Require approval
    steps:
      - uses: actions/checkout@v3
      - name: Apply configurations
        run: |
          for config in github-configs/*.yaml; do
            synacklab github apply "$config"
          done
```

## Error Handling and Recovery

### 1. Graceful Degradation

Design configurations to handle partial failures:

```yaml
# Use independent repository configurations
# Failure in one repository won't affect others
repositories:
  - name: "critical-service"
    # Critical service configuration
  - name: "optional-service"  
    # Optional service configuration
```

### 2. Rollback Strategy

Maintain previous configurations for rollback:

```bash
# Backup before applying changes
cp current-config.yaml current-config.yaml.backup

# Apply new configuration
synacklab github apply new-config.yaml

# Rollback if needed
synacklab github apply current-config.yaml.backup
```

### 3. Monitoring and Alerting

Monitor configuration drift:

```bash
# Regular validation to detect drift
synacklab github validate production-repos.yaml --check-remote

# Alert on validation failures
if ! synacklab github validate production-repos.yaml; then
  echo "Configuration drift detected!" | mail -s "GitHub Config Alert" admin@company.com
fi
```

## Team Collaboration

### 1. Clear Ownership

Document repository ownership in configuration:

```yaml
repositories:
  - name: "user-service"
    description: "User management service (owned by: backend-team)"
    topics: ["backend", "users", "team:backend"]
    teams:
      - team: "backend-team"
        permission: "admin"
```

### 2. Standardized Topics

Use consistent topic taxonomy:

```yaml
defaults:
  topics:
    - "company:acme"           # Company identifier
    - "env:production"         # Environment
    - "type:service"           # Repository type

repositories:
  - name: "user-service"
    topics:
      - "lang:golang"          # Programming language
      - "domain:auth"          # Business domain
      - "team:backend"         # Owning team
```

### 3. Documentation Integration

Link to relevant documentation:

```yaml
repositories:
  - name: "payment-service"
    description: "Payment processing service - See: https://wiki.company.com/payments"
    topics: ["payments", "pci-compliant"]
```

## Testing and Validation

### 1. Dry-Run First

Always test changes with dry-run:

```bash
# Preview changes before applying
synacklab github apply config.yaml --dry-run

# Review output carefully
# Look for unexpected changes or deletions
```

### 2. Staged Rollouts

Apply changes incrementally:

```bash
# Start with non-critical repositories
synacklab github apply config.yaml --repos test-service,dev-tools

# Then apply to critical services
synacklab github apply config.yaml --repos payment-service,user-service
```

### 3. Validation Checklist

Before applying configurations:

- [ ] Configuration passes validation: `synacklab github validate config.yaml`
- [ ] Dry-run shows expected changes only
- [ ] No unexpected repository deletions or permission changes
- [ ] Security settings are appropriate for each repository
- [ ] Team permissions align with organizational structure
- [ ] Webhook URLs are accessible and correct
- [ ] Branch protection rules match CI/CD pipeline requirements

## Common Pitfalls to Avoid

### 1. Over-Permissive Defaults

```yaml
# Avoid: Too permissive defaults
defaults:
  private: false               # Dangerous default
  collaborators:
    - username: "external-user"
      permission: "admin"      # Too much access

# Better: Secure defaults with explicit overrides
defaults:
  private: true                # Secure by default
repositories:
  - name: "public-docs"
    private: false             # Explicit override for public repos
```

### 2. Inconsistent Naming

```yaml
# Avoid: Inconsistent naming
repositories:
  - name: "UserService"        # PascalCase
  - name: "payment_api"        # snake_case
  - name: "frontend-app"       # kebab-case

# Better: Consistent naming convention
repositories:
  - name: "user-service"       # All kebab-case
  - name: "payment-api"
  - name: "frontend-app"
```

### 3. Ignoring Rate Limits

```bash
# Avoid: Processing too many repositories at once
# This can hit GitHub API rate limits

# Better: Process in smaller batches
synacklab github apply large-config.yaml --repos batch1,batch2,batch3
sleep 60  # Wait between batches if needed
synacklab github apply large-config.yaml --repos batch4,batch5,batch6
```

### 4. Missing Validation

```yaml
# Avoid: Applying without validation
synacklab github apply config.yaml  # Risky!

# Better: Always validate first
synacklab github validate config.yaml && \
synacklab github apply config.yaml --dry-run && \
synacklab github apply config.yaml
```

By following these best practices, you can effectively manage large-scale GitHub repository configurations while maintaining security, consistency, and team collaboration.