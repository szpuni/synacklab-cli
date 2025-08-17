package github

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-github/v66/github"
	"github.com/stretchr/testify/assert"
)

func TestMultiRepoError_Error(t *testing.T) {
	tests := []struct {
		name     string
		error    *MultiRepoError
		expected string
	}{
		{
			name: "simple error message",
			error: &MultiRepoError{
				Type:    ErrorTypePartialFailure,
				Message: "partial failure occurred",
			},
			expected: "partial failure occurred",
		},
		{
			name: "error with repository context",
			error: &MultiRepoError{
				Type:    ErrorTypeCompleteFailure,
				Message: "all repositories failed",
				RepositoryErrors: map[string]error{
					"repo1": errors.New("auth failed"),
					"repo2": errors.New("not found"),
				},
			},
			expected: "all repositories failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.error.Error())
		})
	}
}

func TestMultiRepoError_IsPartialFailure(t *testing.T) {
	tests := []struct {
		name     string
		error    *MultiRepoError
		expected bool
	}{
		{
			name: "partial failure",
			error: &MultiRepoError{
				PartialSuccess: true,
			},
			expected: true,
		},
		{
			name: "complete failure",
			error: &MultiRepoError{
				PartialSuccess: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.error.IsPartialFailure())
		})
	}
}

func TestMultiRepoError_GetExitCode(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		expected  int
	}{
		{
			name:      "authentication error",
			errorType: ErrorTypeAuth,
			expected:  1,
		},
		{
			name:      "complete failure",
			errorType: ErrorTypeCompleteFailure,
			expected:  2,
		},
		{
			name:      "partial failure",
			errorType: ErrorTypePartialFailure,
			expected:  3,
		},
		{
			name:      "validation error",
			errorType: ErrorTypeValidation,
			expected:  4,
		},
		{
			name:      "config format error",
			errorType: ErrorTypeConfigFormat,
			expected:  5,
		},
		{
			name:      "unknown error",
			errorType: ErrorTypeUnknown,
			expected:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &MultiRepoError{Type: tt.errorType}
			assert.Equal(t, tt.expected, err.GetExitCode())
		})
	}
}

func TestMultiRepoError_GetFailedRepositories(t *testing.T) {
	repoErrors := map[string]error{
		"repo1": errors.New("error1"),
		"repo2": errors.New("error2"),
		"repo3": errors.New("error3"),
	}

	err := &MultiRepoError{
		RepositoryErrors: repoErrors,
	}

	failed := err.GetFailedRepositories()
	assert.Len(t, failed, 3)
	assert.Contains(t, failed, "repo1")
	assert.Contains(t, failed, "repo2")
	assert.Contains(t, failed, "repo3")
}

func TestMultiRepoError_GetSucceededRepositories(t *testing.T) {
	result := &MultiRepoResult{
		Succeeded: []string{"repo1", "repo2"},
		Failed:    map[string]error{"repo3": errors.New("failed")},
	}

	err := &MultiRepoError{
		Result: result,
	}

	succeeded := err.GetSucceededRepositories()
	assert.Equal(t, []string{"repo1", "repo2"}, succeeded)
}

func TestMultiRepoError_HasActionableGuidance(t *testing.T) {
	tests := []struct {
		name     string
		error    *MultiRepoError
		expected bool
	}{
		{
			name: "has guidance",
			error: &MultiRepoError{
				ActionableGuidance: []ActionableGuidance{
					{Issue: "test issue", Suggestion: "test suggestion"},
				},
			},
			expected: true,
		},
		{
			name: "no guidance",
			error: &MultiRepoError{
				ActionableGuidance: []ActionableGuidance{},
			},
			expected: false,
		},
		{
			name: "nil guidance",
			error: &MultiRepoError{
				ActionableGuidance: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.error.HasActionableGuidance())
		})
	}
}

func TestMultiRepoError_GetGuidanceForSeverity(t *testing.T) {
	err := &MultiRepoError{
		ActionableGuidance: []ActionableGuidance{
			{Issue: "error1", Severity: "error"},
			{Issue: "warning1", Severity: "warning"},
			{Issue: "error2", Severity: "error"},
			{Issue: "info1", Severity: "info"},
		},
	}

	errorGuidance := err.GetGuidanceForSeverity("error")
	assert.Len(t, errorGuidance, 2)
	assert.Equal(t, "error1", errorGuidance[0].Issue)
	assert.Equal(t, "error2", errorGuidance[1].Issue)

	warningGuidance := err.GetGuidanceForSeverity("warning")
	assert.Len(t, warningGuidance, 1)
	assert.Equal(t, "warning1", warningGuidance[0].Issue)

	infoGuidance := err.GetGuidanceForSeverity("info")
	assert.Len(t, infoGuidance, 1)
	assert.Equal(t, "info1", infoGuidance[0].Issue)

	nonExistentGuidance := err.GetGuidanceForSeverity("nonexistent")
	assert.Len(t, nonExistentGuidance, 0)
}

func TestNewMultiRepoAuthError(t *testing.T) {
	message := "authentication failed"
	err := NewMultiRepoAuthError(message)

	assert.Equal(t, ErrorTypeAuth, err.Type)
	assert.Equal(t, message, err.Message)
	assert.True(t, err.HasActionableGuidance())
	assert.Len(t, err.ActionableGuidance, 1)
	assert.Equal(t, "error", err.ActionableGuidance[0].Severity)
	assert.Contains(t, err.ActionableGuidance[0].Commands, "export GITHUB_TOKEN=<your-token>")
}

func TestNewMultiRepoPartialFailureError(t *testing.T) {
	result := &MultiRepoResult{
		Succeeded: []string{"repo1", "repo2"},
		Failed:    map[string]error{"repo3": errors.New("failed")},
		Summary: MultiRepoSummary{
			SuccessCount: 2,
			FailureCount: 1,
			SkippedCount: 0,
		},
	}

	err := NewMultiRepoPartialFailureError(result)

	assert.Equal(t, ErrorTypePartialFailure, err.Type)
	assert.True(t, err.IsPartialFailure())
	assert.Equal(t, result, err.Result)
	assert.Contains(t, err.Message, "2 succeeded, 1 failed, 0 skipped")
}

func TestNewMultiRepoCompleteFailureError(t *testing.T) {
	result := &MultiRepoResult{
		Succeeded: []string{},
		Failed:    map[string]error{"repo1": errors.New("failed1"), "repo2": errors.New("failed2")},
		Summary: MultiRepoSummary{
			SuccessCount: 0,
			FailureCount: 2,
			SkippedCount: 1,
		},
	}

	err := NewMultiRepoCompleteFailureError(result)

	assert.Equal(t, ErrorTypeCompleteFailure, err.Type)
	assert.False(t, err.IsPartialFailure())
	assert.Equal(t, result, err.Result)
	assert.Contains(t, err.Message, "2 failed, 1 skipped")
}

func TestNewMultiRepoValidationError(t *testing.T) {
	message := "validation failed"
	repoErrors := map[string]error{
		"repo1": errors.New("invalid config"),
	}

	err := NewMultiRepoValidationError(message, repoErrors)

	assert.Equal(t, ErrorTypeValidation, err.Type)
	assert.Equal(t, message, err.Message)
	assert.Equal(t, repoErrors, err.RepositoryErrors)
	assert.False(t, err.PartialSuccess)
	assert.True(t, err.HasActionableGuidance())
}

func TestNewMultiRepoConfigFormatError(t *testing.T) {
	message := "invalid YAML format"
	err := NewMultiRepoConfigFormatError(message)

	assert.Equal(t, ErrorTypeConfigFormat, err.Type)
	assert.Equal(t, message, err.Message)
	assert.True(t, err.HasActionableGuidance())
	assert.Contains(t, err.ActionableGuidance[0].Commands, "yamllint <config-file>")
}

func TestIsAuthenticationError(t *testing.T) {
	tests := []struct {
		name     string
		error    error
		expected bool
	}{
		{
			name:     "multi repo auth error",
			error:    &MultiRepoError{Type: ErrorTypeAuth},
			expected: true,
		},
		{
			name:     "github auth error",
			error:    &Error{Type: ErrorTypeAuth},
			expected: true,
		},
		{
			name:     "multi repo non-auth error",
			error:    &MultiRepoError{Type: ErrorTypeValidation},
			expected: false,
		},
		{
			name:     "github non-auth error",
			error:    &Error{Type: ErrorTypeValidation},
			expected: false,
		},
		{
			name:     "generic error",
			error:    errors.New("generic error"),
			expected: false,
		},
		{
			name:     "nil error",
			error:    nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsAuthenticationError(tt.error))
		})
	}
}

func TestShouldFastFail(t *testing.T) {
	tests := []struct {
		name     string
		error    error
		expected bool
	}{
		{
			name:     "auth error should fast fail",
			error:    &MultiRepoError{Type: ErrorTypeAuth},
			expected: true,
		},
		{
			name:     "validation error should not fast fail",
			error:    &MultiRepoError{Type: ErrorTypeValidation},
			expected: false,
		},
		{
			name:     "generic error should not fast fail",
			error:    errors.New("generic error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ShouldFastFail(tt.error))
		})
	}
}

func TestGenerateActionableGuidance(t *testing.T) {
	tests := []struct {
		name       string
		repoErrors map[string]error
		result     *MultiRepoResult
		expected   int // expected number of guidance items
	}{
		{
			name: "authentication errors",
			repoErrors: map[string]error{
				"repo1": &Error{Type: ErrorTypeAuth, Message: "auth failed"},
				"repo2": &Error{Type: ErrorTypeAuth, Message: "token expired"},
			},
			expected: 1, // One guidance for auth errors
		},
		{
			name: "permission errors",
			repoErrors: map[string]error{
				"repo1": &Error{Type: ErrorTypePermission, Message: "insufficient permissions"},
			},
			expected: 1, // One guidance for permission errors
		},
		{
			name: "mixed errors",
			repoErrors: map[string]error{
				"repo1": &Error{Type: ErrorTypeAuth, Message: "auth failed"},
				"repo2": &Error{Type: ErrorTypePermission, Message: "permission denied"},
				"repo3": &Error{Type: ErrorTypeRateLimit, Message: "rate limit exceeded"},
			},
			expected: 3, // One guidance for each error type
		},
		{
			name: "partial failure with retry guidance",
			repoErrors: map[string]error{
				"repo1": &Error{Type: ErrorTypeNetwork, Message: "network error"},
			},
			result: &MultiRepoResult{
				Succeeded: []string{"repo2", "repo3"},
				Failed:    map[string]error{"repo1": &Error{Type: ErrorTypeNetwork}},
			},
			expected: 2, // Network error guidance + retry guidance
		},
		{
			name:       "no errors",
			repoErrors: map[string]error{},
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guidance := generateActionableGuidance(tt.repoErrors, tt.result)
			assert.Len(t, guidance, tt.expected)

			// Verify guidance has required fields
			for _, g := range guidance {
				assert.NotEmpty(t, g.Issue)
				assert.NotEmpty(t, g.Suggestion)
				assert.NotEmpty(t, g.Severity)
				assert.Contains(t, []string{"error", "warning", "info"}, g.Severity)
			}
		})
	}
}

func TestMultiReconciler_PerformAuthenticationCheck(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*MockAPIClient)
		expectError bool
		errorType   ErrorType
	}{
		{
			name: "successful authentication",
			setupMock: func(client *MockAPIClient) {
				client.On("GetRepository", "test-owner", "non-existent-repo-for-auth-check").Return(nil, &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusNotFound},
					Message:  "Not Found",
				})
			},
			expectError: false,
		},
		{
			name: "authentication failure",
			setupMock: func(client *MockAPIClient) {
				client.On("GetRepository", "test-owner", "non-existent-repo-for-auth-check").Return(nil, &github.ErrorResponse{
					Response: &http.Response{StatusCode: http.StatusUnauthorized},
					Message:  "Bad credentials",
				})
			},
			expectError: true,
			errorType:   ErrorTypeAuth,
		},
		{
			name: "network error",
			setupMock: func(client *MockAPIClient) {
				client.On("GetRepository", "test-owner", "non-existent-repo-for-auth-check").Return(nil, errors.New("connection timeout"))
			},
			expectError: true,
			errorType:   ErrorTypeNetwork,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &MockAPIClient{}
			tt.setupMock(client)

			reconciler := &multiReconciler{
				client: client,
				owner:  "test-owner",
			}

			err := reconciler.performAuthenticationCheck()

			if tt.expectError {
				assert.Error(t, err)
				if ghErr, ok := err.(*Error); ok {
					assert.Equal(t, tt.errorType, ghErr.Type)
				}
			} else {
				assert.NoError(t, err)
			}

			client.AssertExpectations(t)
		})
	}
}

func TestMultiReconciler_EnhanceRepositoryError(t *testing.T) {
	reconciler := &multiReconciler{
		owner: "test-owner",
	}

	tests := []struct {
		name         string
		repoName     string
		inputError   error
		expectedType ErrorType
		expectedMsg  string
	}{
		{
			name:     "enhance github error",
			repoName: "test-repo",
			inputError: &Error{
				Type:    ErrorTypeAuth,
				Message: "authentication failed",
			},
			expectedType: ErrorTypeAuth,
			expectedMsg:  "Repository test-repo: authentication failed",
		},
		{
			name:     "enhance partial failure error",
			repoName: "test-repo",
			inputError: &PartialFailureError{
				Message: "some operations failed",
			},
			expectedType: ErrorTypeRepositoryFailure,
			expectedMsg:  "Repository test-repo: some operations failed",
		},
		{
			name:         "enhance generic error",
			repoName:     "test-repo",
			inputError:   errors.New("generic error"),
			expectedType: ErrorTypeUnknown,
			expectedMsg:  "Repository test-repo: generic error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reconciler.enhanceRepositoryError(tt.repoName, tt.inputError)

			assert.Error(t, err)
			if ghErr, ok := err.(*Error); ok {
				assert.Equal(t, tt.expectedType, ghErr.Type)
				assert.Equal(t, tt.expectedMsg, ghErr.Message)
				assert.Equal(t, tt.repoName, ghErr.Resource)
			} else {
				t.Errorf("Expected *Error, got %T", err)
			}
		})
	}
}

func TestMultiReconciler_ApplyAll_AuthenticationFastFail(t *testing.T) {
	client := &MockAPIClient{}
	client.On("GetRepository", "test-owner", "non-existent-repo-for-auth-check").Return(nil, &github.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusUnauthorized},
		Message:  "Bad credentials",
	})

	reconciler := &multiReconciler{
		client: client,
		owner:  "test-owner",
		merger: NewConfigMerger(),
	}

	plans := map[string]*ReconciliationPlan{
		"repo1": {Repository: &RepositoryChange{Type: ChangeTypeCreate}},
	}

	result, err := reconciler.ApplyAll(plans)

	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Succeeded)
	assert.Empty(t, result.Failed)

	// Verify it's an authentication error
	if multiErr, ok := err.(*MultiRepoError); ok {
		assert.Equal(t, ErrorTypeAuth, multiErr.Type)
		assert.True(t, multiErr.HasActionableGuidance())
	} else {
		t.Errorf("Expected *MultiRepoError, got %T", err)
	}

	client.AssertExpectations(t)
}

func TestMultiReconciler_ValidateAll_AuthenticationFastFail(t *testing.T) {
	client := &MockAPIClient{}
	client.On("GetRepository", "test-owner", "non-existent-repo-for-auth-check").Return(nil, &github.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusUnauthorized},
		Message:  "Bad credentials",
	})

	reconciler := &multiReconciler{
		client: client,
		owner:  "test-owner",
		merger: NewConfigMerger(),
	}

	config := &MultiRepositoryConfig{
		Repositories: []RepositoryConfig{
			{Name: "test-repo"},
		},
	}

	result, err := reconciler.ValidateAll(config, nil)

	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Valid)
	assert.Empty(t, result.Invalid)

	// Verify it's an authentication error
	if multiErr, ok := err.(*MultiRepoError); ok {
		assert.Equal(t, ErrorTypeAuth, multiErr.Type)
		assert.True(t, multiErr.HasActionableGuidance())
	} else {
		t.Errorf("Expected *MultiRepoError, got %T", err)
	}

	client.AssertExpectations(t)
}

func TestMultiReconciler_PlanAll_AuthenticationFastFail(t *testing.T) {
	client := &MockAPIClient{}
	client.On("GetRepository", "test-owner", "non-existent-repo-for-auth-check").Return(nil, &github.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusUnauthorized},
		Message:  "Bad credentials",
	})

	reconciler := &multiReconciler{
		client: client,
		owner:  "test-owner",
		merger: NewConfigMerger(),
	}

	config := &MultiRepositoryConfig{
		Repositories: []RepositoryConfig{
			{Name: "test-repo"},
		},
	}

	plans, err := reconciler.PlanAll(config, nil)

	assert.Error(t, err)
	assert.Nil(t, plans)

	// Verify it's an authentication error
	if multiErr, ok := err.(*MultiRepoError); ok {
		assert.Equal(t, ErrorTypeAuth, multiErr.Type)
		assert.True(t, multiErr.HasActionableGuidance())
	} else {
		t.Errorf("Expected *MultiRepoError, got %T", err)
	}

	client.AssertExpectations(t)
}
