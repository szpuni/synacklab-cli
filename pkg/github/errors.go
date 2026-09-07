package github

import (
	"fmt"
	"net/http"
	"strings"

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
