package github

import (
	"fmt"
	"strings"
	"time"
)

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
