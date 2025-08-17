package github

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v66/github"
)

// ErrorType represents different categories of GitHub API errors
type ErrorType string

const (
	ErrorTypeAuth              ErrorType = "authentication"
	ErrorTypePermission        ErrorType = "permission"
	ErrorTypeNotFound          ErrorType = "not_found"
	ErrorTypeValidation        ErrorType = "validation"
	ErrorTypeRateLimit         ErrorType = "rate_limit"
	ErrorTypeNetwork           ErrorType = "network"
	ErrorTypeConflict          ErrorType = "conflict"
	ErrorTypeUnknown           ErrorType = "unknown"
	ErrorTypePartialFailure    ErrorType = "partial_failure"
	ErrorTypeCompleteFailure   ErrorType = "complete_failure"
	ErrorTypeRepositoryFailure ErrorType = "repository_failure"
	ErrorTypeConfigFormat      ErrorType = "config_format"
	ErrorTypeDuplicateRepo     ErrorType = "duplicate_repository"
	ErrorTypeRepoNotFound      ErrorType = "repository_not_found"
	ErrorTypeMergeConflict     ErrorType = "merge_conflict"
	ErrorTypeMultiRepoFailure  ErrorType = "multi_repo_failure"
)

// Error represents a structured error from GitHub operations
type Error struct {
	Type      ErrorType `json:"type"`
	Message   string    `json:"message"`
	Cause     error     `json:"-"`
	Resource  string    `json:"resource,omitempty"`
	Field     string    `json:"field,omitempty"`
	Code      string    `json:"code,omitempty"`
	Retryable bool      `json:"retryable"`
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Resource != "" {
		return fmt.Sprintf("%s error for %s: %s", e.Type, e.Resource, e.Message)
	}
	return fmt.Sprintf("%s error: %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether the error is retryable
func (e *Error) IsRetryable() bool {
	return e.Retryable
}

// NewGitHubError creates a new Error with the specified type and message
func NewGitHubError(errorType ErrorType, message string, cause error) *Error {
	return &Error{
		Type:      errorType,
		Message:   message,
		Cause:     cause,
		Retryable: isRetryableErrorType(errorType),
	}
}

// WrapGitHubError wraps a GitHub API error into our structured error type
func WrapGitHubError(err error, resource string) *Error {
	if err == nil {
		return nil
	}

	// If it's already a Error, return as-is
	if ghErr, ok := err.(*Error); ok {
		if ghErr.Resource == "" {
			ghErr.Resource = resource
		}
		return ghErr
	}

	// Handle GitHub API errors
	if ghErr, ok := err.(*github.ErrorResponse); ok {
		return parseGitHubAPIError(ghErr, resource)
	}

	// Handle HTTP errors
	if httpErr, ok := err.(*github.RateLimitError); ok {
		return &Error{
			Type:      ErrorTypeRateLimit,
			Message:   fmt.Sprintf("Rate limit exceeded. Reset at %v", httpErr.Rate.Reset.Time),
			Cause:     err,
			Resource:  resource,
			Retryable: true,
		}
	}

	// Handle network/connection errors
	if isNetworkError(err) {
		return &Error{
			Type:      ErrorTypeNetwork,
			Message:   "Network error occurred. Please check your connection and try again",
			Cause:     err,
			Resource:  resource,
			Retryable: true,
		}
	}

	// Default to unknown error
	return &Error{
		Type:      ErrorTypeUnknown,
		Message:   err.Error(),
		Cause:     err,
		Resource:  resource,
		Retryable: false,
	}
}

// parseGitHubAPIError parses GitHub API error responses into structured errors
func parseGitHubAPIError(ghErr *github.ErrorResponse, resource string) *Error {
	baseErr := &Error{
		Resource: resource,
		Cause:    ghErr,
	}

	switch ghErr.Response.StatusCode {
	case http.StatusUnauthorized:
		baseErr.Type = ErrorTypeAuth
		baseErr.Message = "Authentication failed. Please check your GitHub token"
		baseErr.Retryable = false

		// Provide specific guidance based on error message
		if strings.Contains(ghErr.Message, "token") {
			baseErr.Message = "Invalid or expired GitHub token. Please update your GITHUB_TOKEN environment variable or configuration"
		}

	case http.StatusForbidden:
		if strings.Contains(ghErr.Message, "rate limit") {
			baseErr.Type = ErrorTypeRateLimit
			baseErr.Message = "GitHub API rate limit exceeded. Please wait before retrying"
			baseErr.Retryable = true
		} else {
			baseErr.Type = ErrorTypePermission
			baseErr.Message = "Insufficient permissions. Your token may not have the required scopes"
			baseErr.Retryable = false

			// Provide specific permission guidance
			if strings.Contains(resource, "repository") {
				baseErr.Message += ". Required scopes: repo (for private repos) or public_repo (for public repos)"
			}
		}

	case http.StatusNotFound:
		baseErr.Type = ErrorTypeNotFound
		baseErr.Retryable = false

		if strings.Contains(resource, "repository") {
			baseErr.Message = "Repository not found. Check the repository name and your access permissions"
		} else if strings.Contains(resource, "user") {
			baseErr.Message = "User not found. Please verify the username is correct"
		} else if strings.Contains(resource, "team") {
			baseErr.Message = "Team not found. Please verify the team slug and organization"
		} else {
			baseErr.Message = "Resource not found"
		}

	case http.StatusConflict:
		baseErr.Type = ErrorTypeConflict
		baseErr.Message = "Resource conflict occurred"
		baseErr.Retryable = false

		if strings.Contains(ghErr.Message, "already exists") {
			baseErr.Message = "Resource already exists with the same name"
		}

	case http.StatusUnprocessableEntity:
		baseErr.Type = ErrorTypeValidation
		baseErr.Message = "Validation failed"
		baseErr.Retryable = false

		// Parse validation errors from GitHub response
		if len(ghErr.Errors) > 0 {
			var validationErrors []string
			for _, err := range ghErr.Errors {
				if err.Field != "" {
					validationErrors = append(validationErrors, fmt.Sprintf("%s: %s", err.Field, err.Message))
					// Set field info on the first error
					if baseErr.Field == "" {
						baseErr.Field = err.Field
						baseErr.Code = err.Code
					}
				} else {
					validationErrors = append(validationErrors, err.Message)
				}
			}
			baseErr.Message = fmt.Sprintf("Validation failed: %s", strings.Join(validationErrors, "; "))
		}

	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		baseErr.Type = ErrorTypeNetwork
		baseErr.Message = "GitHub API is temporarily unavailable. Please try again later"
		baseErr.Retryable = true

	default:
		baseErr.Type = ErrorTypeUnknown
		baseErr.Message = ghErr.Message
		baseErr.Retryable = ghErr.Response.StatusCode >= 500
	}

	return baseErr
}

// isNetworkError checks if an error is a network-related error
func isNetworkError(err error) bool {
	errStr := strings.ToLower(err.Error())
	networkKeywords := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"network is unreachable",
		"no such host",
		"timeout",
		"dial tcp",
		"i/o timeout",
	}

	for _, keyword := range networkKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}
	return false
}

// isRetryableErrorType determines if an error type is generally retryable
func isRetryableErrorType(errorType ErrorType) bool {
	switch errorType {
	case ErrorTypeRateLimit, ErrorTypeNetwork:
		return true
	default:
		return false
	}
}

// RetryConfig defines configuration for retry logic
type RetryConfig struct {
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	RetryableErrors []ErrorType
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []ErrorType{
			ErrorTypeRateLimit,
			ErrorTypeNetwork,
		},
	}
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation func() error

// minDuration returns the minimum of two durations
func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// WithRetry executes an operation with retry logic
func WithRetry(operation RetryableOperation, config *RetryConfig) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(delay)

			// Exponential backoff with jitter
			delay = time.Duration(float64(delay) * config.BackoffFactor)
			delay = minDuration(delay, config.MaxDelay)
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if ghErr, ok := err.(*Error); ok {
			if !ghErr.IsRetryable() {
				return err
			}

			// Special handling for rate limit errors
			if ghErr.Type == ErrorTypeRateLimit {
				if rateLimitErr, ok := ghErr.Cause.(*github.RateLimitError); ok {
					// Wait until rate limit resets
					resetTime := rateLimitErr.Rate.Reset.Time
					waitTime := time.Until(resetTime)
					if waitTime > 0 && waitTime < 5*time.Minute {
						time.Sleep(waitTime)
						continue
					}
				}
			}
		} else {
			// For non-GitHubError types, don't retry
			return err
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", config.MaxRetries, lastErr)
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value,omitempty"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("validation error for field '%s' (value: %s): %s", e.Field, e.Value, e.Message)
	}
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

// Error implements the error interface
func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "validation failed"
	}

	if len(e) == 1 {
		return e[0].Error()
	}

	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return fmt.Sprintf("validation failed with %d errors: %s", len(e), strings.Join(messages, "; "))
}

// Add adds a validation error to the collection
func (e *ValidationErrors) Add(field, value, message string) {
	*e = append(*e, ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	})
}

// HasErrors returns true if there are validation errors
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// PartialFailureError represents an error where some operations succeeded and others failed
type PartialFailureError struct {
	Succeeded []string         `json:"succeeded"`
	Failed    map[string]error `json:"failed"`
	Message   string           `json:"message"`
}

// Error implements the error interface
func (e *PartialFailureError) Error() string {
	if e.Message != "" {
		return e.Message
	}

	return fmt.Sprintf("partial failure: %d succeeded, %d failed", len(e.Succeeded), len(e.Failed))
}

// NewPartialFailureError creates a new partial failure error
func NewPartialFailureError(succeeded []string, failed map[string]error) *PartialFailureError {
	message := fmt.Sprintf("Operation completed with partial success: %d operations succeeded, %d failed",
		len(succeeded), len(failed))

	return &PartialFailureError{
		Succeeded: succeeded,
		Failed:    failed,
		Message:   message,
	}
}

// GetFailedOperations returns a list of failed operation descriptions
func (e *PartialFailureError) GetFailedOperations() []string {
	var operations []string
	for op := range e.Failed {
		operations = append(operations, op)
	}
	return operations
}

// GetSucceededOperations returns a list of successful operation descriptions
func (e *PartialFailureError) GetSucceededOperations() []string {
	return e.Succeeded
}

// MultiRepoError represents comprehensive errors in multi-repository operations
type MultiRepoError struct {
	Type               ErrorType            `json:"type"`
	Message            string               `json:"message"`
	RepositoryErrors   map[string]error     `json:"repository_errors"`
	PartialSuccess     bool                 `json:"partial_success"`
	Result             *MultiRepoResult     `json:"result,omitempty"`
	Context            *MultiRepoContext    `json:"context,omitempty"`
	ActionableGuidance []ActionableGuidance `json:"actionable_guidance,omitempty"`
}

// MultiRepoContext provides context about the multi-repository operation
type MultiRepoContext struct {
	TotalRepositories int      `json:"total_repositories"`
	ProcessedRepos    []string `json:"processed_repos"`
	SkippedRepos      []string `json:"skipped_repos"`
	OperationType     string   `json:"operation_type"`
	StartTime         string   `json:"start_time"`
	EndTime           string   `json:"end_time"`
}

// ActionableGuidance provides specific guidance for resolving errors
type ActionableGuidance struct {
	Issue      string   `json:"issue"`
	Suggestion string   `json:"suggestion"`
	Commands   []string `json:"commands,omitempty"`
	References []string `json:"references,omitempty"`
	Severity   string   `json:"severity"` // "error", "warning", "info"
}

// MultiRepoResult contains results from multi-repository operations
type MultiRepoResult struct {
	Succeeded []string         `json:"succeeded"`
	Failed    map[string]error `json:"failed"`
	Skipped   []string         `json:"skipped"`
	Summary   MultiRepoSummary `json:"summary"`
}

// MultiRepoSummary provides aggregate statistics
type MultiRepoSummary struct {
	TotalRepositories int `json:"total_repositories"`
	SuccessCount      int `json:"success_count"`
	FailureCount      int `json:"failure_count"`
	SkippedCount      int `json:"skipped_count"`
	TotalChanges      int `json:"total_changes"`
}

// Error implements the error interface
func (e *MultiRepoError) Error() string {
	return e.Message
}

// Unwrap returns the underlying error if there's only one repository error
func (e *MultiRepoError) Unwrap() error {
	if len(e.RepositoryErrors) == 1 {
		for _, err := range e.RepositoryErrors {
			return err
		}
	}
	return nil
}

// IsPartialFailure returns true if some repositories succeeded
func (e *MultiRepoError) IsPartialFailure() bool {
	return e.PartialSuccess
}

// GetFailedRepositories returns a list of failed repository names
func (e *MultiRepoError) GetFailedRepositories() []string {
	var repos []string
	for repo := range e.RepositoryErrors {
		repos = append(repos, repo)
	}
	return repos
}

// GetSucceededRepositories returns a list of successful repository names
func (e *MultiRepoError) GetSucceededRepositories() []string {
	if e.Result != nil {
		return e.Result.Succeeded
	}
	return []string{}
}

// GetSkippedRepositories returns a list of skipped repository names
func (e *MultiRepoError) GetSkippedRepositories() []string {
	if e.Result != nil {
		return e.Result.Skipped
	}
	return []string{}
}

// GetExitCode returns the appropriate exit code based on the error type and results
func (e *MultiRepoError) GetExitCode() int {
	switch e.Type {
	case ErrorTypeAuth:
		return 1 // Authentication failure - fast fail
	case ErrorTypeCompleteFailure:
		return 2 // Complete failure - all repositories failed
	case ErrorTypePartialFailure:
		return 3 // Partial failure - some succeeded, some failed
	case ErrorTypeValidation:
		return 4 // Validation errors
	case ErrorTypeConfigFormat:
		return 5 // Configuration format errors
	default:
		return 1 // General error
	}
}

// HasActionableGuidance returns true if the error has actionable guidance
func (e *MultiRepoError) HasActionableGuidance() bool {
	return len(e.ActionableGuidance) > 0
}

// GetGuidanceForSeverity returns guidance filtered by severity level
func (e *MultiRepoError) GetGuidanceForSeverity(severity string) []ActionableGuidance {
	var filtered []ActionableGuidance
	for _, guidance := range e.ActionableGuidance {
		if guidance.Severity == severity {
			filtered = append(filtered, guidance)
		}
	}
	return filtered
}

// NewMultiRepoError creates a new comprehensive multi-repository error
func NewMultiRepoError(errorType ErrorType, message string, repoErrors map[string]error, result *MultiRepoResult) *MultiRepoError {
	partialSuccess := result != nil && len(result.Succeeded) > 0 && len(result.Failed) > 0

	multiErr := &MultiRepoError{
		Type:             errorType,
		Message:          message,
		RepositoryErrors: repoErrors,
		PartialSuccess:   partialSuccess,
		Result:           result,
		Context:          buildMultiRepoContext(result, string(errorType)),
	}

	// Add actionable guidance based on error patterns
	multiErr.ActionableGuidance = generateActionableGuidance(repoErrors, result)

	return multiErr
}

// NewMultiRepoAuthError creates a new authentication error for multi-repository operations
func NewMultiRepoAuthError(message string) *MultiRepoError {
	return &MultiRepoError{
		Type:    ErrorTypeAuth,
		Message: message,
		ActionableGuidance: []ActionableGuidance{
			{
				Issue:      "Authentication failed",
				Suggestion: "Check your GitHub token and ensure it has the required permissions",
				Commands: []string{
					"export GITHUB_TOKEN=<your-token>",
					"gh auth status",
				},
				References: []string{
					"https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token",
				},
				Severity: "error",
			},
		},
	}
}

// NewMultiRepoPartialFailureError creates a new partial failure error for multi-repository operations
func NewMultiRepoPartialFailureError(result *MultiRepoResult) *MultiRepoError {
	message := fmt.Sprintf("Multi-repository operation completed with partial success: %d succeeded, %d failed, %d skipped",
		result.Summary.SuccessCount, result.Summary.FailureCount, result.Summary.SkippedCount)

	return NewMultiRepoError(ErrorTypePartialFailure, message, result.Failed, result)
}

// NewMultiRepoCompleteFailureError creates a new complete failure error for multi-repository operations
func NewMultiRepoCompleteFailureError(result *MultiRepoResult) *MultiRepoError {
	message := fmt.Sprintf("Multi-repository operation failed completely: %d failed, %d skipped",
		result.Summary.FailureCount, result.Summary.SkippedCount)

	multiErr := &MultiRepoError{
		Type:             ErrorTypeCompleteFailure,
		Message:          message,
		RepositoryErrors: result.Failed,
		PartialSuccess:   false, // Explicitly set to false for complete failure
		Result:           result,
		Context:          buildMultiRepoContext(result, string(ErrorTypeCompleteFailure)),
	}

	// Add actionable guidance based on error patterns
	multiErr.ActionableGuidance = generateActionableGuidance(result.Failed, result)

	return multiErr
}

// NewMultiRepoValidationError creates a validation error for multi-repository operations
func NewMultiRepoValidationError(message string, repoErrors map[string]error) *MultiRepoError {
	return &MultiRepoError{
		Type:             ErrorTypeValidation,
		Message:          message,
		RepositoryErrors: repoErrors,
		PartialSuccess:   false,
		ActionableGuidance: []ActionableGuidance{
			{
				Issue:      "Configuration validation failed",
				Suggestion: "Review and fix the validation errors in your configuration file",
				Commands: []string{
					"synacklab github validate <config-file>",
				},
				Severity: "error",
			},
		},
	}
}

// NewMultiRepoConfigFormatError creates a configuration format error
func NewMultiRepoConfigFormatError(message string) *MultiRepoError {
	return &MultiRepoError{
		Type:    ErrorTypeConfigFormat,
		Message: message,
		ActionableGuidance: []ActionableGuidance{
			{
				Issue:      "Configuration format is invalid",
				Suggestion: "Check your YAML syntax and ensure the configuration follows the expected format",
				Commands: []string{
					"yamllint <config-file>",
				},
				References: []string{
					"https://yaml.org/spec/1.2/spec.html",
				},
				Severity: "error",
			},
		},
	}
}

// buildMultiRepoContext creates context information for multi-repository operations
func buildMultiRepoContext(result *MultiRepoResult, operationType string) *MultiRepoContext {
	if result == nil {
		return &MultiRepoContext{
			OperationType: operationType,
			StartTime:     time.Now().UTC().Format(time.RFC3339),
		}
	}

	var processedRepos []string
	processedRepos = append(processedRepos, result.Succeeded...)
	for repo := range result.Failed {
		processedRepos = append(processedRepos, repo)
	}

	return &MultiRepoContext{
		TotalRepositories: result.Summary.TotalRepositories,
		ProcessedRepos:    processedRepos,
		SkippedRepos:      result.Skipped,
		OperationType:     operationType,
		StartTime:         time.Now().UTC().Format(time.RFC3339),
		EndTime:           time.Now().UTC().Format(time.RFC3339),
	}
}

// generateActionableGuidance generates actionable guidance based on error patterns
func generateActionableGuidance(repoErrors map[string]error, result *MultiRepoResult) []ActionableGuidance {
	var guidance []ActionableGuidance
	errorPatterns := make(map[ErrorType][]string)

	// Analyze error patterns
	for repo, err := range repoErrors {
		if ghErr, ok := err.(*Error); ok {
			errorPatterns[ghErr.Type] = append(errorPatterns[ghErr.Type], repo)
		}
	}

	// Generate guidance based on error patterns
	for errorType, repos := range errorPatterns {
		switch errorType {
		case ErrorTypeAuth:
			guidance = append(guidance, ActionableGuidance{
				Issue:      fmt.Sprintf("Authentication failed for %d repositories", len(repos)),
				Suggestion: "Verify your GitHub token has the required permissions and is not expired",
				Commands: []string{
					"gh auth status",
					"gh auth refresh",
				},
				References: []string{
					"https://docs.github.com/en/authentication",
				},
				Severity: "error",
			})

		case ErrorTypePermission:
			guidance = append(guidance, ActionableGuidance{
				Issue:      fmt.Sprintf("Permission denied for %d repositories: %s", len(repos), strings.Join(repos, ", ")),
				Suggestion: "Ensure your GitHub token has the required scopes (repo for private repos, public_repo for public repos)",
				Commands: []string{
					"gh auth status --show-token",
				},
				References: []string{
					"https://docs.github.com/en/developers/apps/building-oauth-apps/scopes-for-oauth-apps",
				},
				Severity: "error",
			})

		case ErrorTypeNotFound:
			guidance = append(guidance, ActionableGuidance{
				Issue:      fmt.Sprintf("Resources not found for %d repositories: %s", len(repos), strings.Join(repos, ", ")),
				Suggestion: "Verify repository names, user names, and team slugs are correct and accessible",
				Severity:   "error",
			})

		case ErrorTypeRateLimit:
			guidance = append(guidance, ActionableGuidance{
				Issue:      fmt.Sprintf("Rate limit exceeded for %d repositories", len(repos)),
				Suggestion: "Wait for rate limit to reset or use a GitHub App token for higher limits",
				Commands: []string{
					"gh api rate_limit",
				},
				References: []string{
					"https://docs.github.com/en/rest/overview/resources-in-the-rest-api#rate-limiting",
				},
				Severity: "warning",
			})

		case ErrorTypeValidation:
			guidance = append(guidance, ActionableGuidance{
				Issue:      fmt.Sprintf("Validation failed for %d repositories: %s", len(repos), strings.Join(repos, ", ")),
				Suggestion: "Review configuration for these repositories and fix validation errors",
				Commands: []string{
					"synacklab github validate <config-file> --repos " + strings.Join(repos, ","),
				},
				Severity: "error",
			})

		case ErrorTypeNetwork:
			guidance = append(guidance, ActionableGuidance{
				Issue:      fmt.Sprintf("Network errors for %d repositories", len(repos)),
				Suggestion: "Check your internet connection and GitHub API status",
				References: []string{
					"https://www.githubstatus.com/",
				},
				Severity: "warning",
			})
		}
	}

	// Add retry guidance for partial failures
	if result != nil && len(result.Failed) > 0 && len(result.Succeeded) > 0 {
		failedRepos := make([]string, 0, len(result.Failed))
		for repo := range result.Failed {
			failedRepos = append(failedRepos, repo)
		}

		guidance = append(guidance, ActionableGuidance{
			Issue:      "Partial failure occurred",
			Suggestion: "Retry the operation for only the failed repositories",
			Commands: []string{
				"synacklab github apply <config-file> --repos " + strings.Join(failedRepos, ","),
			},
			Severity: "info",
		})
	}

	return guidance
}

// IsAuthenticationError checks if the error is an authentication error that should cause fast-fail
func IsAuthenticationError(err error) bool {
	if multiErr, ok := err.(*MultiRepoError); ok {
		return multiErr.Type == ErrorTypeAuth
	}
	if ghErr, ok := err.(*Error); ok {
		return ghErr.Type == ErrorTypeAuth
	}
	return false
}

// ShouldFastFail determines if an error should cause immediate failure before processing repositories
func ShouldFastFail(err error) bool {
	return IsAuthenticationError(err)
}
