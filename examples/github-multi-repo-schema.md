# Multi-Repository Configuration Schema

This document describes the complete schema for multi-repository GitHub configurations.

## Root Schema

```yaml
version: string                    # Configuration format version (required: "1.0")
defaults: RepositoryDefaults      # Global defaults (optional)
repositories: []RepositoryConfig  # List of repositories (required)
```

## RepositoryDefaults Schema

Global defaults that apply to all repositories unless overridden at the repository level.

```yaml
defaults:
  # Basic repository settings
  description: string             # Default description template
  private: boolean               # Default visibility (true = private, false = public)
  topics: []string              # Default topics applied to all repositories
  
  # Repository features
  features: RepositoryFeatures   # Default feature settings
  
  # Access control
  collaborators: []Collaborator  # Default collaborators for all repositories
  teams: []TeamAccess           # Default team access for all repositories
  
  # Branch protection
  branch_protection: []BranchProtectionRule  # Default branch protection rules
  
  # Webhooks
  webhooks: []Webhook           # Default webhooks for all repositories
```

## RepositoryConfig Schema

Configuration for individual repositories. All fields are optional and will inherit from defaults if not specified.

```yaml
repositories:
  - name: string                 # Repository name (required, unique within configuration)
    description: string          # Repository description
    private: boolean            # Repository visibility
    topics: []string           # Repository topics
    
    # Repository features
    features: RepositoryFeatures
    
    # Access control
    collaborators: []Collaborator
    teams: []TeamAccess
    
    # Branch protection
    branch_protection: []BranchProtectionRule
    
    # Webhooks
    webhooks: []Webhook
```

## RepositoryFeatures Schema

```yaml
features:
  issues: boolean               # Enable/disable issues (default: true)
  wiki: boolean                # Enable/disable wiki (default: false)
  projects: boolean            # Enable/disable projects (default: false)
  discussions: boolean         # Enable/disable discussions (default: false)
  
  # Security and analysis features
  security_and_analysis:
    secret_scanning: boolean                    # Enable secret scanning
    secret_scanning_push_protection: boolean   # Enable push protection
    dependabot_security_updates: boolean       # Enable Dependabot security updates
    dependabot_alerts: boolean                 # Enable Dependabot alerts
```

## Collaborator Schema

```yaml
collaborators:
  - username: string           # GitHub username (required)
    permission: string         # Permission level (required)
```

### Permission Levels
- `read` - Read access to repository
- `triage` - Read access + manage issues and pull requests
- `write` - Read/write access to repository
- `maintain` - Write access + manage repository settings
- `admin` - Full administrative access

## TeamAccess Schema

```yaml
teams:
  - team: string              # Team name within organization (required)
    permission: string        # Permission level (required)
```

### Team Permission Levels
- `pull` - Read access to repository
- `triage` - Read access + manage issues and pull requests  
- `push` - Read/write access to repository
- `maintain` - Push access + manage repository settings
- `admin` - Full administrative access

## BranchProtectionRule Schema

```yaml
branch_protection:
  - pattern: string                    # Branch name pattern (required, e.g., "main", "release/*")
    required_reviews: integer          # Number of required reviews (0-6)
    dismiss_stale_reviews: boolean     # Dismiss stale reviews on new commits
    require_code_owner_review: boolean # Require review from code owners
    
    # Status checks
    required_status_checks:
      strict: boolean                  # Require branches to be up to date
      contexts: []string              # Required status check contexts
    
    # Push restrictions
    restrictions:
      users: []string                 # Users who can push to protected branch
      teams: []string                 # Teams who can push to protected branch
      apps: []string                  # Apps who can push to protected branch
    
    # Additional settings
    enforce_admins: boolean           # Enforce restrictions for administrators
    allow_force_pushes: boolean       # Allow force pushes
    allow_deletions: boolean          # Allow branch deletions
```

## Webhook Schema

```yaml
webhooks:
  - url: string                # Webhook URL (required)
    events: []string          # List of events to trigger webhook (required)
    active: boolean           # Whether webhook is active (default: true)
    content_type: string      # Content type: "json" or "form" (default: "json")
    secret: string            # Webhook secret for validation (optional)
    insecure_ssl: boolean     # Allow insecure SSL (default: false)
```

### Webhook Events
Common webhook events include:
- `push` - Repository push
- `pull_request` - Pull request activity
- `issues` - Issue activity
- `release` - Release activity
- `create` - Branch/tag creation
- `delete` - Branch/tag deletion
- `fork` - Repository fork
- `watch` - Repository watch/star
- `deployment` - Deployment activity
- `deployment_status` - Deployment status updates

## Configuration Merging Rules

When a repository configuration is processed, settings are merged with defaults using these rules:

### Override Strategy (Default)
Repository settings completely replace default settings:
- `private`, `description` - Repository value overrides default
- `topics` - Repository topics replace default topics entirely
- `collaborators` - Repository collaborators replace default collaborators entirely
- `teams` - Repository teams replace default teams entirely

### Deep Merge Strategy
Complex objects are merged recursively:
- `features` - Individual feature flags are merged
- `security_and_analysis` - Individual security settings are merged

### Array Replacement Strategy
Arrays are replaced entirely, not merged:
- `branch_protection` - Repository rules replace default rules entirely
- `webhooks` - Repository webhooks replace default webhooks entirely

## Best Practices

### 1. Use Meaningful Defaults
```yaml
defaults:
  private: true                    # Secure by default
  topics: ["production", "team"]   # Consistent tagging
  features:
    issues: true                   # Enable issue tracking
    wiki: false                    # Disable unused features
```

### 2. Consistent Naming Conventions
```yaml
repositories:
  - name: "user-service"          # Use kebab-case
  - name: "payment-api"           # Be descriptive
  - name: "frontend-dashboard"    # Include component type
```

### 3. Logical Topic Hierarchies
```yaml
defaults:
  topics: ["company", "production"]  # Company-wide tags

repositories:
  - name: "user-service"
    topics: ["golang", "microservice", "users"]  # Specific tags
```

### 4. Security-First Configuration
```yaml
defaults:
  private: true                    # Private by default
  features:
    security_and_analysis:
      secret_scanning: true        # Enable security features
      dependabot_security_updates: true
  branch_protection:
    - pattern: "main"
      required_reviews: 2          # Require reviews
      require_code_owner_review: true
```

### 5. Environment-Specific Configurations
```yaml
# production-repos.yaml
defaults:
  private: true
  topics: ["production"]
  branch_protection:
    - pattern: "main"
      required_reviews: 3          # Stricter for production

# development-repos.yaml  
defaults:
  private: false
  topics: ["development"]
  branch_protection:
    - pattern: "main"
      required_reviews: 1          # More relaxed for development
```

## Validation Rules

The configuration is validated according to these rules:

### Required Fields
- `version` must be specified at root level
- `repositories` array must contain at least one repository
- Each repository must have a unique `name`
- Webhook `url` and `events` are required when webhooks are specified

### Format Validation
- Repository names must be valid GitHub repository names
- Permission levels must be from allowed values
- Webhook events must be valid GitHub webhook events
- Branch patterns must be valid glob patterns

### Logical Validation
- Collaborator usernames and team names are checked for existence
- Required status check contexts should match actual CI/CD pipeline contexts
- Webhook URLs should be accessible endpoints

## Example Validation Errors

```yaml
# Invalid configuration examples

# Error: Missing required version
repositories:
  - name: "test-repo"

# Error: Duplicate repository names
version: "1.0"
repositories:
  - name: "my-repo"
  - name: "my-repo"  # Duplicate!

# Error: Invalid permission level
version: "1.0"
repositories:
  - name: "test-repo"
    collaborators:
      - username: "user1"
        permission: "invalid"  # Should be read/write/admin/etc.
```