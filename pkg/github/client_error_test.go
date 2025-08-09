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

func TestClient_ErrorHandling(t *testing.T) {
	t.Run("authentication error handling", func(t *testing.T) {
		authErr := &github.ErrorResponse{
			Response: &http.Response{StatusCode: http.StatusUnauthorized},
			Message:  "Bad credentials",
		}

		wrappedErr := WrapGitHubError(authErr, "repository test/repo")

		require.NotNil(t, wrappedErr)
		assert.Equal(t, ErrorTypeAuth, wrappedErr.Type)
		assert.False(t, wrappedErr.IsRetryable())
		assert.Contains(t, wrappedErr.Message, "Authentication failed")
	})

	t.Run("network error handling", func(t *testing.T) {
		networkErr := &github.ErrorResponse{
			Response: &http.Response{StatusCode: http.StatusInternalServerError},
			Message:  "Internal Server Error",
		}

		wrappedErr := WrapGitHubError(networkErr, "repository test/repo")

		require.NotNil(t, wrappedErr)
		assert.Equal(t, ErrorTypeNetwork, wrappedErr.Type)
		assert.True(t, wrappedErr.IsRetryable())
		assert.Contains(t, wrappedErr.Message, "GitHub API is temporarily unavailable")
	})

	t.Run("validation error handling", func(t *testing.T) {
		validationErr := &github.ErrorResponse{
			Response: &http.Response{StatusCode: http.StatusUnprocessableEntity},
			Message:  "Validation Failed",
			Errors: []github.Error{
				{Field: "name", Message: "is required"},
			},
		}

		wrappedErr := WrapGitHubError(validationErr, "repository test/repo")

		require.NotNil(t, wrappedErr)
		assert.Equal(t, ErrorTypeValidation, wrappedErr.Type)
		assert.False(t, wrappedErr.IsRetryable())
		assert.Contains(t, wrappedErr.Message, "Validation failed")
		assert.Contains(t, wrappedErr.Message, "name: is required")
	})
}

func TestClient_PartialFailureScenarios(t *testing.T) {
	t.Run("repository creation succeeds but collaborator addition fails", func(t *testing.T) {
		// This test demonstrates the PartialFailureError functionality
		succeeded := []string{"repository"}
		failed := map[string]error{
			"collaborator user1": NewGitHubError(ErrorTypeNotFound, "user not found", nil),
		}

		err := NewPartialFailureError(succeeded, failed)

		assert.Contains(t, err.Error(), "1 operations succeeded, 1 failed")
		assert.Equal(t, []string{"repository"}, err.GetSucceededOperations())
		assert.Equal(t, []string{"collaborator user1"}, err.GetFailedOperations())
	})
}

func TestClient_RateLimitHandling(t *testing.T) {
	t.Run("rate limit error triggers appropriate wait", func(t *testing.T) {
		// Create a rate limit error
		rateLimitErr := &github.RateLimitError{
			Rate: github.Rate{
				Limit:     5000,
				Remaining: 0,
				Reset:     github.Timestamp{Time: time.Now().Add(time.Second)},
			},
		}

		wrappedErr := WrapGitHubError(rateLimitErr, "test resource")

		require.NotNil(t, wrappedErr)
		assert.Equal(t, ErrorTypeRateLimit, wrappedErr.Type)
		assert.True(t, wrappedErr.IsRetryable())
		assert.Contains(t, wrappedErr.Message, "Rate limit exceeded")
	})
}

func TestClient_ErrorMessageQuality(t *testing.T) {
	tests := []struct {
		name             string
		inputError       error
		resource         string
		expectedMsg      string
		expectedGuidance string
	}{
		{
			name: "authentication error provides token guidance",
			inputError: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusUnauthorized},
				Message:  "Bad credentials",
			},
			resource:         "repository test/repo",
			expectedMsg:      "Authentication failed",
			expectedGuidance: "GitHub token",
		},
		{
			name: "permission error provides scope guidance for repository",
			inputError: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusForbidden},
				Message:  "Forbidden",
			},
			resource:         "repository test/repo",
			expectedMsg:      "Insufficient permissions",
			expectedGuidance: "Required scopes: repo",
		},
		{
			name: "not found error provides specific guidance for user",
			inputError: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "Not Found",
			},
			resource:         "user testuser",
			expectedMsg:      "User not found",
			expectedGuidance: "verify the username",
		},
		{
			name: "validation error provides field-level details",
			inputError: &github.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusUnprocessableEntity},
				Message:  "Validation Failed",
				Errors: []github.Error{
					{Field: "name", Message: "is too long", Code: "too_long"},
					{Field: "description", Message: "is required", Code: "missing"},
				},
			},
			resource:         "repository test/repo",
			expectedMsg:      "Validation failed",
			expectedGuidance: "name: is too long; description: is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WrapGitHubError(tt.inputError, tt.resource)

			require.NotNil(t, err)
			assert.Contains(t, err.Message, tt.expectedMsg)
			assert.Contains(t, err.Message, tt.expectedGuidance)
		})
	}
}

func TestClient_NetworkErrorDetection(t *testing.T) {
	networkErrors := []string{
		"dial tcp: connection refused",
		"dial tcp: connection timeout",
		"dial tcp: no such host",
		"read tcp: i/o timeout",
		"network is unreachable",
	}

	for _, errMsg := range networkErrors {
		t.Run(errMsg, func(t *testing.T) {
			err := errors.New(errMsg)
			wrappedErr := WrapGitHubError(err, "test resource")

			require.NotNil(t, wrappedErr)
			assert.Equal(t, ErrorTypeNetwork, wrappedErr.Type)
			assert.True(t, wrappedErr.IsRetryable())
			assert.Contains(t, wrappedErr.Message, "Network error occurred")
		})
	}
}
