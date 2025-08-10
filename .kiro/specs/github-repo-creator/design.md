# Design Document

## Overview

The GitHub Repository Reconciler extends synacklab with declarative GitHub repository management capabilities. The feature implements Infrastructure as Code principles, where YAML configuration files serve as the source of truth for repository state. The system compares desired configuration with actual GitHub repository state and reconciles differences through the GitHub REST API.

The design follows synacklab's existing patterns: Cobra CLI framework for commands, YAML configuration management, and a pkg/internal architecture separation. The reconciler uses a three-phase approach: validate configuration, plan changes, and apply changes.

## Architecture

### High-Level Components

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   CLI Commands  │───▶│  GitHub Service  │───▶│  GitHub API     │
│                 │    │                  │    │                 │
│ • apply         │    │ • Reconciler     │    │ • REST API v4   │
│ • validate      │    │ • State Manager  │    │ • GraphQL API   │
│ • dry-run       │    │ • Config Parser  │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                       │
         ▼                       ▼
┌─────────────────┐    ┌──────────────────┐
│ Config Models   │    │   Auth Manager   │
│                 │    │                  │
│ • Repository    │    │ • Token Provider │
│ • Branch Rules  │    │ • Permissions    │
│ • Access Control│    │                  │
└─────────────────┘    └──────────────────┘
```

### Package Structure

```
internal/cmd/
├── github.go              # GitHub command group
├── github_apply.go        # Apply command implementation
├── github_validate.go     # Validate command implementation

pkg/github/
├── config.go              # Configuration models and parsing
├── client.go              # GitHub API client wrapper
├── reconciler.go          # State reconciliation logic
├── auth.go                # Authentication management
└── types.go               # GitHub resource type definitions

pkg/config/
├── github.go              # GitHub-specific config extensions
```

## Components and Interfaces

### Configuration Models

The configuration system extends the existing `pkg/config` package with GitHub-specific structures:

```go
// GitHubConfig extends the main Config struct
type GitHubConfig struct {
    Token        string `yaml:"token,omitempty"`
    Organization string `yaml:"organization,omitempty"`
}

// RepositoryConfig represents a complete repository configuration
type RepositoryConfig struct {
    Name         string                 `yaml:"name"`
    Description  string                 `yaml:"description,omitempty"`
    Private      bool                   `yaml:"private"`
    Topics       []string               `yaml:"topics,omitempty"`
    Features     RepositoryFeatures     `yaml:"features,omitempty"`
    BranchRules  []BranchProtectionRule `yaml:"branch_protection,omitempty"`
    Collaborators []Collaborator        `yaml:"collaborators,omitempty"`
    Teams        []TeamAccess           `yaml:"teams,omitempty"`
    Webhooks     []Webhook              `yaml:"webhooks,omitempty"`
}

// BranchProtectionRule defines branch protection settings
type BranchProtectionRule struct {
    Pattern                string   `yaml:"pattern"`
    RequiredStatusChecks   []string `yaml:"required_status_checks,omitempty"`
    RequireUpToDate        bool     `yaml:"require_up_to_date"`
    RequiredReviews        int      `yaml:"required_reviews"`
    DismissStaleReviews    bool     `yaml:"dismiss_stale_reviews"`
    RequireCodeOwnerReview bool     `yaml:"require_code_owner_review"`
    RestrictPushes         []string `yaml:"restrict_pushes,omitempty"`
}
```

### GitHub Client Interface

```go
type GitHubClient interface {
    // Repository operations
    GetRepository(owner, name string) (*Repository, error)
    CreateRepository(config RepositoryConfig) (*Repository, error)
    UpdateRepository(owner, name string, config RepositoryConfig) error
    
    // Branch protection operations
    GetBranchProtection(owner, name, branch string) (*BranchProtection, error)
    UpdateBranchProtection(owner, name, branch string, rules BranchProtectionRule) error
    
    // Collaborator operations
    ListCollaborators(owner, name string) ([]Collaborator, error)
    AddCollaborator(owner, name, username string, permission string) error
    RemoveCollaborator(owner, name, username string) error
    
    // Team operations
    ListTeamAccess(owner, name string) ([]TeamAccess, error)
    AddTeamAccess(owner, name string, team TeamAccess) error
    UpdateTeamAccess(owner, name string, team TeamAccess) error
    RemoveTeamAccess(owner, name, teamSlug string) error
    
    // Webhook operations
    ListWebhooks(owner, name string) ([]Webhook, error)
    CreateWebhook(owner, name string, webhook Webhook) error
    UpdateWebhook(owner, name string, webhookID int64, webhook Webhook) error
    DeleteWebhook(owner, name string, webhookID int64) error
}
```

### Reconciler Interface

```go
type Reconciler interface {
    Plan(config RepositoryConfig) (*ReconciliationPlan, error)
    Apply(plan *ReconciliationPlan) error
    Validate(config RepositoryConfig) error
}

type ReconciliationPlan struct {
    Repository    *RepositoryChange    `json:"repository,omitempty"`
    BranchRules   []BranchRuleChange   `json:"branch_rules,omitempty"`
    Collaborators []CollaboratorChange `json:"collaborators,omitempty"`
    Teams         []TeamChange         `json:"teams,omitempty"`
    Webhooks      []WebhookChange      `json:"webhooks,omitempty"`
}

type ChangeType string

const (
    ChangeTypeCreate ChangeType = "create"
    ChangeTypeUpdate ChangeType = "update"
    ChangeTypeDelete ChangeType = "delete"
)
```

## Data Models

### Core Repository Model

```go
type Repository struct {
    ID          int64  `json:"id"`
    Name        string `json:"name"`
    FullName    string `json:"full_name"`
    Description string `json:"description"`
    Private     bool   `json:"private"`
    Topics      []string `json:"topics"`
    Features    RepositoryFeatures `json:"features"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type RepositoryFeatures struct {
    Issues      bool `json:"has_issues" yaml:"issues"`
    Wiki        bool `json:"has_wiki" yaml:"wiki"`
    Projects    bool `json:"has_projects" yaml:"projects"`
    Discussions bool `json:"has_discussions" yaml:"discussions"`
}
```

### Access Control Models

```go
type Collaborator struct {
    Username   string `yaml:"username"`
    Permission string `yaml:"permission"` // read, write, admin
}

type TeamAccess struct {
    TeamSlug   string `yaml:"team"`
    Permission string `yaml:"permission"` // read, write, admin
}

type Webhook struct {
    URL    string   `yaml:"url"`
    Events []string `yaml:"events"`
    Secret string   `yaml:"secret,omitempty"`
    Active bool     `yaml:"active"`
}
```

## Error Handling

### Error Types

```go
type GitHubError struct {
    Type    ErrorType
    Message string
    Cause   error
}

type ErrorType string

const (
    ErrorTypeAuth         ErrorType = "authentication"
    ErrorTypePermission   ErrorType = "permission"
    ErrorTypeNotFound     ErrorType = "not_found"
    ErrorTypeValidation   ErrorType = "validation"
    ErrorTypeRateLimit    ErrorType = "rate_limit"
    ErrorTypeNetwork      ErrorType = "network"
)
```

### Error Handling Strategy

1. **Authentication Errors**: Provide clear instructions for token setup
2. **Permission Errors**: Specify required GitHub permissions
3. **Rate Limiting**: Implement exponential backoff with retry logic
4. **Validation Errors**: Return detailed field-level validation messages
5. **Network Errors**: Retry transient failures, fail fast on permanent errors

### Validation Rules

- Repository names must follow GitHub naming conventions
- Branch protection patterns must be valid glob patterns
- Collaborator usernames must exist on GitHub
- Team slugs must exist in the organization
- Webhook URLs must be valid and accessible
- Permission levels must be valid GitHub permission strings

## Testing Strategy

### Unit Testing

1. **Configuration Parsing**: Test YAML parsing and validation
2. **GitHub Client**: Mock GitHub API responses for all operations
3. **Reconciler Logic**: Test state comparison and change planning
4. **Error Handling**: Test all error scenarios and recovery

### Integration Testing

1. **GitHub API Integration**: Test against GitHub API with test repositories
2. **End-to-End Workflows**: Test complete apply/validate/dry-run flows
3. **Authentication**: Test token-based authentication flows
4. **Rate Limiting**: Test rate limit handling and backoff

### Test Data Management

```go
// Test fixtures for consistent testing
type TestFixtures struct {
    ValidConfig     RepositoryConfig
    InvalidConfig   RepositoryConfig
    ExistingRepo    Repository
    MockResponses   map[string]interface{}
}
```

### Testing Approach

1. Use `testify` for assertions and mocking
2. Create GitHub test organization for integration tests
3. Use environment variables for test configuration
4. Implement cleanup procedures for test repositories
5. Test both success and failure scenarios comprehensively

The testing strategy ensures reliability across different GitHub configurations and handles edge cases like partial failures, network issues, and permission changes.