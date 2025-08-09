package github

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-github/v66/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *GitHubError
		expected string
	}{
		{
			name: "error with resource",
			err: &GitHubError{
				Type:     ErrorTypeAuth,
				Message:  "invalid token",
				Resource: "repository test/repo",
			},
			expected: "authentication error for repository test/repo: invalid token",
		},
		{
			name: "error without resource",
			err: &GitHubError{
				Type:    ErrorTypeValidation,
				Message: "validation failed",
			},
			expected: "validation error: validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestGitHubError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &GitHubError{
		Type:    ErrorTypeNetwork,
		Message: "network error",
		Cause:   cause,
	}

	assert.Equal(t, cause, err.Unwrap())
}

func TestGitHubError_IsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		retryable bool
		expected  bool
	}{
		{
			name:      "rate limit error is retryable",
			errorType: ErrorTypeRateLimit,
			retryable: true,
			expected:  true,
		},
		{
			name:      "network error is retryable",
			errorType: ErrorTypeNetwork,
			retryable: true,
			expected:  true,
		},
		{
			name:      "auth error is not retryable",
			errorType: ErrorTypeAuth,
			retryable: false,
			expected:  false,
		},
		{
			name:      "validation error is not retryable",
			errorType: ErrorTypeValidation,
			retryable: false,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &GitHubError{
				Type:      tt.errorType,
				Retryable: tt.retryable,
			}
			assert.Equal(t, tt.expected, err.IsRetryable())
		})
	}
}

func TestNewGitHubError(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewGitHubError(ErrorTypeAuth, "authentication failed", cause)

	assert.Equal(t, ErrorTypeAuth, err.Type)
	assert.Equal(t, "authentication failed", err.Message)
	assert.Equal(t, cause, err.Cause)
	assert.False(t, err.Retryable)
}

func TestWrapGitHubError(t *testing.T) {
	tests := []struct {
		name         string
		inputError   error
		resource     string
		expectedType ErrorType
		expectedMsg  string
	}{
		{
			name:         "nil error returns nil",
			inputError:   nil,
			resource:     "test",
			expectedType: "",
			expectedMsg:  "",
		},
		{
			name: "already GitHubError returns as-is",
			inputError: &GitHubError{
				Type:    ErrorTypeAuth,
				Message: "auth error",
			},
			resource:     "repository test/repo",
			expectedType: ErrorTypeAuth,
			expectedMsg:  "auth error",
		},
		{
			name: "401 unauthorized error",
			inputError: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusUnauthorized},
				Message:  "Bad credentials",
			},
			resource:     "repository test/repo",
			expectedType: ErrorTypeAuth,
			expectedMsg:  "Authentication failed. Please check your GitHub token",
		},
		{
			name: "403 forbidden error",
			inputError: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusForbidden},
				Message:  "Forbidden",
			},
			resource:     "repository test/repo",
			expectedType: ErrorTypePermission,
			expectedMsg:  "Insufficient permissions. Your token may not have the required scopes. Required scopes: repo (for private repos) or public_repo (for public repos)",
		},
		{
			name: "404 not found error",
			inputError: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
			},
			resource:     "repository test/repo",
			expectedType: ErrorTypeNotFound,
			expectedMsg:  "Repository not found. Check the repository name and your access permissions",
		},
		{
			name: "409 conflict error",
			inputError: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusConflict},
				Message:  "Repository already exists",
			},
			resource:     "repository test/repo",
			expectedType: ErrorTypeConflict,
			expectedMsg:  "Resource already exists with the same name",
		},
		{
			name: "422 validation error",
			inputError: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusUnprocessableEntity},
				Message:  "Validation Failed",
				Errors: []github.Error{
					{Field: "name", Message: "is required", Code: "missing_field"},
					{Message: "Repository name is invalid"},
				},
			},
			resource:     "repository test/repo",
			expectedType: ErrorTypeValidation,
			expectedMsg:  "Validation failed: name: is required; Repository name is invalid",
		},
		{
			name: "500 server error",
			inputError: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusInternalServerError},
				Message:  "Internal Server Error",
			},
			resource:     "repository test/repo",
			expectedType: ErrorTypeNetwork,
			expectedMsg:  "GitHub API is temporarily unavailable. Please try again later",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapGitHubError(tt.inputError, tt.resource)

			if tt.inputError == nil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.Equal(t, tt.expectedType, result.Type)
			assert.Contains(t, result.Message, tt.expectedMsg)
			assert.Equal(t, tt.resource, result.Resource)
		})
	}
}

func TestIsNetworkError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "connection refused",
			err:      errors.New("dial tcp: connection refused"),
			expected: true,
		},
		{
			name:     "connection timeout",
			err:      errors.New("dial tcp: connection timeout"),
			expected: true,
		},
		{
			name:     "no such host",
			err:      errors.New("dial tcp: no such host"),
			expected: true,
		},
		{
			name:     "i/o timeout",
			err:      errors.New("read tcp: i/o timeout"),
			expected: true,
		},
		{
			name:     "regular error",
			err:      errors.New("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isNetworkError(tt.err))
		})
	}
}

func TestWithRetry(t *testing.T) {
	t.Run("successful operation on first try", func(t *testing.T) {
		callCount := 0
		operation := func() error {
			callCount++
			return nil
		}

		err := WithRetry(operation, DefaultRetryConfig())
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("successful operation after retries", func(t *testing.T) {
		callCount := 0
		operation := func() error {
			callCount++
			if callCount < 3 {
				return &GitHubError{
					Type:      ErrorTypeNetwork,
					Message:   "network error",
					Retryable: true,
				}
			}
			return nil
		}

		err := WithRetry(operation, DefaultRetryConfig())
		assert.NoError(t, err)
		assert.Equal(t, 3, callCount)
	})

	t.Run("non-retryable error fails immediately", func(t *testing.T) {
		callCount := 0
		operation := func() error {
			callCount++
			return &GitHubError{
				Type:      ErrorTypeAuth,
				Message:   "auth error",
				Retryable: false,
			}
		}

		err := WithRetry(operation, DefaultRetryConfig())
		assert.Error(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("exhausts max retries", func(t *testing.T) {
		callCount := 0
		operation := func() error {
			callCount++
			return &GitHubError{
				Type:      ErrorTypeNetwork,
				Message:   "network error",
				Retryable: true,
			}
		}

		config := &RetryConfig{
			MaxRetries:    2,
			InitialDelay:  time.Millisecond,
			MaxDelay:      time.Millisecond * 10,
			BackoffFactor: 2.0,
		}

		err := WithRetry(operation, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "operation failed after 2 retries")
		assert.Equal(t, 3, callCount) // Initial attempt + 2 retries
	})

	t.Run("rate limit error with reset time", func(t *testing.T) {
		callCount := 0
		resetTime := time.Now().Add(time.Millisecond * 50)

		operation := func() error {
			callCount++
			if callCount == 1 {
				rateLimitErr := &github.RateLimitError{
					Rate: github.Rate{
						Reset: github.Timestamp{Time: resetTime},
					},
				}
				return &GitHubError{
					Type:      ErrorTypeRateLimit,
					Message:   "rate limit exceeded",
					Cause:     rateLimitErr,
					Retryable: true,
				}
			}
			return nil
		}

		start := time.Now()
		err := WithRetry(operation, DefaultRetryConfig())
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.Equal(t, 2, callCount)
		// Should have waited for the rate limit reset
		assert.True(t, duration >= time.Millisecond*40)
	})
}

func TestValidationError(t *testing.T) {
	t.Run("error with value", func(t *testing.T) {
		err := &ValidationError{
			Field:   "name",
			Value:   "invalid-name",
			Message: "name is invalid",
		}
		expected := "validation error for field 'name' (value: invalid-name): name is invalid"
		assert.Equal(t, expected, err.Error())
	})

	t.Run("error without value", func(t *testing.T) {
		err := &ValidationError{
			Field:   "description",
			Message: "description is too long",
		}
		expected := "validation error for field 'description': description is too long"
		assert.Equal(t, expected, err.Error())
	})
}

func TestValidationErrors(t *testing.T) {
	t.Run("empty validation errors", func(t *testing.T) {
		var errors ValidationErrors
		assert.Equal(t, "validation failed", errors.Error())
		assert.False(t, errors.HasErrors())
	})

	t.Run("single validation error", func(t *testing.T) {
		var errors ValidationErrors
		errors.Add("name", "test", "name is invalid")

		assert.True(t, errors.HasErrors())
		assert.Contains(t, errors.Error(), "validation error for field 'name'")
	})

	t.Run("multiple validation errors", func(t *testing.T) {
		var errors ValidationErrors
		errors.Add("name", "test", "name is invalid")
		errors.Add("description", "", "description is required")

		assert.True(t, errors.HasErrors())
		assert.Contains(t, errors.Error(), "validation failed with 2 errors")
		assert.Contains(t, errors.Error(), "name is invalid")
		assert.Contains(t, errors.Error(), "description is required")
	})
}

func TestPartialFailureError(t *testing.T) {
	succeeded := []string{"repository", "branch protection"}
	failed := map[string]error{
		"collaborator user1":          errors.New("user not found"),
		"webhook https://example.com": errors.New("webhook creation failed"),
	}

	err := NewPartialFailureError(succeeded, failed)

	assert.Contains(t, err.Error(), "2 operations succeeded, 2 failed")
	assert.Equal(t, succeeded, err.GetSucceededOperations())

	failedOps := err.GetFailedOperations()
	assert.Len(t, failedOps, 2)
	assert.Contains(t, failedOps, "collaborator user1")
	assert.Contains(t, failedOps, "webhook https://example.com")
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, time.Second, config.InitialDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.BackoffFactor)
	assert.Contains(t, config.RetryableErrors, ErrorTypeRateLimit)
	assert.Contains(t, config.RetryableErrors, ErrorTypeNetwork)
}

func TestIsRetryableErrorType(t *testing.T) {
	tests := []struct {
		errorType ErrorType
		expected  bool
	}{
		{ErrorTypeRateLimit, true},
		{ErrorTypeNetwork, true},
		{ErrorTypeAuth, false},
		{ErrorTypePermission, false},
		{ErrorTypeNotFound, false},
		{ErrorTypeValidation, false},
		{ErrorTypeConflict, false},
		{ErrorTypeUnknown, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.errorType), func(t *testing.T) {
			assert.Equal(t, tt.expected, isRetryableErrorType(tt.errorType))
		})
	}
}
