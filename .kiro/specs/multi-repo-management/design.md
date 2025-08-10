# Design Document

## Overview

The Multi-Repository Management feature extends the existing GitHub Repository Reconciler to support managing multiple repositories from a single YAML configuration file. The design maintains full backward compatibility with single repository configurations while introducing new capabilities for bulk repository management, global defaults, and selective operations.

The system implements a dual-format approach where configuration files can contain either a single repository definition (existing format) or a multi-repository definition with optional global defaults. The reconciliation engine processes repositories independently, enabling parallel operations and graceful failure handling.

## Architecture

### High-Level Components

```
┌─────────────────────┐    ┌──────────────────────┐    ┌─────────────────┐
│   CLI Commands      │───▶│  Multi-Repo Service  │───▶│  GitHub API     │
│                     │    │                      │    │                 │
│ • apply (enhanced)  │    │ • Multi-Reconciler   │    │ • REST API v4   │
│ • validate (enhanced)│   │ • Config Merger      │    │ • GraphQL API   │
│ • dry-run (enhanced)│    │ • Batch Processor    │    │                 │
│ • --repos filter    │    │ • Progress Reporter  │    │                 │
└─────────────────────┘    └──────────────────────┘    └─────────────────┘
         │                           │
         ▼                           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Enhanced Config     │    │   Existing Services  │
│                     │    │                      │
│ • Multi-Repo Format │    │ • Single Reconciler │
│ • Global Defaults   │    │ • GitHub Client     │
│ • Format Detection  │    │ • Auth Manager      │
└─────────────────────┘    └──────────────────────┘
```

### Package Structure Extensions

```
pkg/github/
├── config.go              # Enhanced with multi-repo support
├── multi_config.go        # New: Multi-repository configuration
├── reconciler.go          # Existing single-repo reconciler
├── multi_reconciler.go    # New: Multi-repository reconciler
├── client.go              # Existing GitHub API client
├── auth.go                # Existing authentication
└── types.go               # Enhanced with multi-repo types

internal/cmd/
├── github_apply.go        # Enhanced for multi-repo support
├── github_validate.go     # Enhanced for multi-repo support
└── github.go              # Enhanced command group
```

## Components and Interfaces

### Enhanced Configuration Models

The configuration system supports both single and multi-repository formats through format detection:

```go
// MultiRepositoryConfig represents a multi-repository configuration
type MultiRepositoryConfig struct {
    // Global defaults applied to all repositories
    Defaults *RepositoryDefaults `yaml:"defaults,omitempty"`
    
    // List of repositories to manage
    Repositories []RepositoryConfig `yaml:"repositories"`
    
    // Metadata for the configuration
    Version string `yaml:"version,omitempty"`
}

// RepositoryDefaults defines default settings for all repositories
type RepositoryDefaults struct {
    Description   string                 `yaml:"description,omitempty"`
    Private       *bool                  `yaml:"private,omitempty"`
    Topics        []string               `yaml:"topics,omitempty"`
    Features      *RepositoryFeatures    `yaml:"features,omitempty"`
    BranchRules   []BranchProtectionRule `yaml:"branch_protection,omitempty"`
    Collaborators []Collaborator         `yaml:"collaborators,omitempty"`
    Teams         []TeamAccess           `yaml:"teams,omitempty"`
    Webhooks      []Webhook              `yaml:"webhooks,omitempty"`
}

// ConfigFormat represents the detected configuration format
type ConfigFormat int

const (
    FormatSingleRepository ConfigFormat = iota
    FormatMultiRepository
)

// ConfigDetector detects and loads appropriate configuration format
type ConfigDetector interface {
    DetectFormat(data []byte) (ConfigFormat, error)
    LoadSingleRepo(data []byte) (*RepositoryConfig, error)
    LoadMultiRepo(data []byte) (*MultiRepositoryConfig, error)
}
```

### Multi-Repository Reconciler Interface

```go
// MultiReconciler manages multiple repositories
type MultiReconciler interface {
    // Plan creates reconciliation plans for all or selected repositories
    PlanAll(config *MultiRepositoryConfig, repoFilter []string) (map[string]*ReconciliationPlan, error)
    
    // Apply executes reconciliation plans for multiple repositories
    ApplyAll(plans map[string]*ReconciliationPlan) (*MultiRepoResult, error)
    
    // Validate validates all repository configurations
    ValidateAll(config *MultiRepositoryConfig, repoFilter []string) (*MultiRepoValidationResult, error)
}

// MultiRepoResult contains results from multi-repository operations
type MultiRepoResult struct {
    Succeeded []string                    `json:"succeeded"`
    Failed    map[string]error           `json:"failed"`
    Skipped   []string                   `json:"skipped"`
    Summary   MultiRepoSummary           `json:"summary"`
}

// MultiRepoSummary provides aggregate statistics
type MultiRepoSummary struct {
    TotalRepositories int `json:"total_repositories"`
    SuccessCount      int `json:"success_count"`
    FailureCount      int `json:"failure_count"`
    SkippedCount      int `json:"skipped_count"`
    TotalChanges      int `json:"total_changes"`
}
```

### Configuration Merger Interface

```go
// ConfigMerger merges global defaults with repository-specific settings
type ConfigMerger interface {
    MergeDefaults(defaults *RepositoryDefaults, repo *RepositoryConfig) (*RepositoryConfig, error)
    ValidateMergedConfig(merged *RepositoryConfig) error
}

// MergeStrategy defines how defaults are merged with repository settings
type MergeStrategy int

const (
    MergeStrategyOverride MergeStrategy = iota // Repository settings override defaults
    MergeStrategyAppend                        // Repository settings append to defaults
    MergeStrategyDeepMerge                     // Deep merge for complex objects
)
```

## Data Models

### Multi-Repository Configuration Schema

```yaml
# Multi-repository configuration format
version: "1.0"

# Global defaults applied to all repositories
defaults:
  private: true
  topics:
    - production
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
  collaborators:
    - username: "devops-team"
      permission: "admin"
  teams:
    - team: "backend-team"
      permission: "write"
  webhooks:
    - url: "https://ci.example.com/webhook/github"
      events: ["push", "pull_request"]
      active: true

# Repository definitions
repositories:
  - name: "user-service"
    description: "User management microservice"
    topics:
      - golang
      - api
    # Inherits defaults, adds specific topics
    
  - name: "payment-service"
    description: "Payment processing microservice"
    private: false  # Overrides default
    topics:
      - golang
      - payment
      - api
    branch_protection:
      - pattern: "main"
        required_reviews: 3  # Stricter than default
        required_status_checks:
          - "security/scan"
    # Overrides branch protection, inherits other defaults
    
  - name: "frontend-app"
    description: "React frontend application"
    topics:
      - react
      - frontend
    teams:
      - team: "frontend-team"
        permission: "admin"
      - team: "backend-team"
        permission: "read"
    # Overrides teams, inherits other defaults
```

### Backward Compatibility Schema

```yaml
# Single repository format (existing) - still supported
name: "my-repo"
description: "My repository"
private: true
topics:
  - production
features:
  issues: true
  wiki: true
branch_protection:
  - pattern: "main"
    required_reviews: 2
```

## Error Handling

### Multi-Repository Error Types

```go
// MultiRepoError represents errors in multi-repository operations
type MultiRepoError struct {
    Type         ErrorType            `json:"type"`
    Message      string               `json:"message"`
    RepositoryErrors map[string]error `json:"repository_errors"`
    PartialSuccess   bool             `json:"partial_success"`
}

// ErrorType extensions for multi-repository operations
const (
    ErrorTypeConfigFormat     ErrorType = "config_format"
    ErrorTypePartialFailure   ErrorType = "partial_failure"
    ErrorTypeDuplicateRepo    ErrorType = "duplicate_repository"
    ErrorTypeRepoNotFound     ErrorType = "repository_not_found"
    ErrorTypeMergeConflict    ErrorType = "merge_conflict"
)
```

### Error Handling Strategy

1. **Format Detection Errors**: Clear messages about expected formats with examples
2. **Partial Failures**: Continue processing other repositories, collect all errors
3. **Rate Limiting**: Implement intelligent backoff across multiple repositories
4. **Validation Errors**: Report all validation issues across all repositories
5. **Merge Conflicts**: Detailed reporting of default/override conflicts

### Graceful Degradation

- If one repository fails, continue with others
- Collect and report all errors at the end
- Provide partial success indicators
- Enable retry of failed repositories only

## Testing Strategy

### Unit Testing Extensions

1. **Configuration Detection**: Test format detection with various YAML structures
2. **Config Merging**: Test default/override merging with complex scenarios
3. **Multi-Reconciler**: Test batch processing with success/failure combinations
4. **Error Handling**: Test partial failure scenarios and error aggregation

### Integration Testing Extensions

1. **Multi-Repository Workflows**: Test complete apply/validate/dry-run with multiple repos
2. **Selective Operations**: Test repository filtering functionality
3. **Rate Limiting**: Test behavior under GitHub API rate limits
4. **Backward Compatibility**: Ensure existing single-repo configs work unchanged

### Test Data Management

```go
// Enhanced test fixtures for multi-repository testing
type MultiRepoTestFixtures struct {
    ValidMultiConfig     MultiRepositoryConfig
    InvalidMultiConfig   MultiRepositoryConfig
    MixedSuccessConfig   MultiRepositoryConfig
    BackwardCompatConfig RepositoryConfig
    MockMultiResponses   map[string]map[string]interface{}
}
```

## Implementation Approach

### Phase 1: Configuration Enhancement
- Extend configuration models to support multi-repository format
- Implement format detection and backward compatibility
- Add configuration merging logic for defaults and overrides

### Phase 2: Multi-Repository Reconciler
- Create multi-repository reconciler that orchestrates single-repo reconcilers
- Implement batch processing with error collection
- Add progress reporting and result aggregation

### Phase 3: CLI Enhancement
- Extend existing commands to support multi-repository operations
- Add repository filtering capabilities
- Enhance output formatting for multi-repository results

### Phase 4: Advanced Features
- Implement parallel processing for improved performance
- Add selective retry capabilities
- Enhance rate limiting and throttling

## Performance Considerations

### Parallel Processing
- Process repositories concurrently where possible
- Respect GitHub API rate limits with intelligent throttling
- Implement configurable concurrency limits

### Memory Management
- Stream process large configurations
- Avoid loading all repository states simultaneously
- Implement efficient result aggregation

### Rate Limiting Strategy
```go
// RateLimiter manages GitHub API rate limits across multiple repositories
type RateLimiter interface {
    Wait(ctx context.Context) error
    UpdateLimits(remaining, resetTime int)
    GetDelay() time.Duration
}

// Implement exponential backoff with jitter for multiple repositories
type MultiRepoRateLimiter struct {
    baseDelay    time.Duration
    maxDelay     time.Duration
    backoffFactor float64
    jitter       float64
}
```

The design ensures scalability while maintaining the reliability and user experience of the existing single-repository functionality.