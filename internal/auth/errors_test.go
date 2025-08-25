package auth

import (
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/aws/smithy-go"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name              string
		inputError        error
		expectedType      ErrorType
		expectedRetryable bool
	}{
		{
			name:              "nil error",
			inputError:        nil,
			expectedType:      "",
			expectedRetryable: false,
		},
		{
			name:              "network timeout error",
			inputError:        &net.OpError{Op: "dial", Net: "tcp", Err: &timeoutError{}},
			expectedType:      ErrorTypeNetworkTimeout,
			expectedRetryable: true,
		},
		{
			name:              "DNS resolution error",
			inputError:        &net.DNSError{Name: "example.com", IsNotFound: true},
			expectedType:      ErrorTypeDNSResolution,
			expectedRetryable: false,
		},
		{
			name:              "connection refused error",
			inputError:        fmt.Errorf("dial tcp: connection refused"),
			expectedType:      ErrorTypeNetworkConnectivity,
			expectedRetryable: true,
		},
		{
			name:              "permission denied error",
			inputError:        &os.PathError{Op: "open", Path: "/test", Err: syscall.EACCES},
			expectedType:      ErrorTypePermissionDenied,
			expectedRetryable: false,
		},
		{
			name:              "AWS unauthorized error",
			inputError:        &mockAPIError{code: "UnauthorizedException", message: "Token expired"},
			expectedType:      ErrorTypeSessionExpired,
			expectedRetryable: false,
		},
		{
			name:              "AWS throttling error",
			inputError:        &mockAPIError{code: "ThrottlingException", message: "Rate exceeded"},
			expectedType:      ErrorTypeRateLimited,
			expectedRetryable: true,
		},
		{
			name:              "generic error",
			inputError:        fmt.Errorf("some generic error"),
			expectedType:      ErrorTypeAWSAPIError,
			expectedRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyError(tt.inputError)

			if tt.inputError == nil {
				if result != nil {
					t.Errorf("Expected nil result for nil error, got %v", result)
				}
				return
			}

			if result == nil {
				t.Fatalf("Expected non-nil result for error %v", tt.inputError)
			}

			if result.Type != tt.expectedType {
				t.Errorf("Expected error type %v, got %v", tt.expectedType, result.Type)
			}

			if result.IsRetryable() != tt.expectedRetryable {
				t.Errorf("Expected retryable %v, got %v", tt.expectedRetryable, result.IsRetryable())
			}

			if result.OriginalError != tt.inputError {
				t.Errorf("Expected original error to be preserved")
			}
		})
	}
}

func TestAuthError_Error(t *testing.T) {
	authErr := &Error{
		Type:    ErrorTypeNetworkTimeout,
		Message: "Network timeout occurred",
	}

	if authErr.Error() != "Network timeout occurred" {
		t.Errorf("Expected error message 'Network timeout occurred', got '%s'", authErr.Error())
	}
}

func TestAuthError_Unwrap(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	authErr := &Error{
		Type:          ErrorTypeNetworkTimeout,
		Message:       "Network timeout occurred",
		OriginalError: originalErr,
	}

	if authErr.Unwrap() != originalErr {
		t.Errorf("Expected unwrapped error to be original error")
	}
}

func TestAuthError_GetTroubleshootingMessage(t *testing.T) {
	tests := []struct {
		name     string
		authErr  *Error
		expected string
	}{
		{
			name: "with troubleshooting steps",
			authErr: &Error{
				Type:    ErrorTypeNetworkTimeout,
				Message: "Network timeout",
				TroubleshootingSteps: []string{
					"Check your connection",
					"Try again later",
				},
			},
			expected: "\nTroubleshooting steps:\n1. Check your connection\n2. Try again later\n",
		},
		{
			name: "without troubleshooting steps",
			authErr: &Error{
				Type:    ErrorTypeNetworkTimeout,
				Message: "Network timeout",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.authErr.GetTroubleshootingMessage()
			if result != tt.expected {
				t.Errorf("Expected troubleshooting message:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestValidateAWSConfig(t *testing.T) {
	tests := []struct {
		name         string
		startURL     string
		region       string
		expectedType ErrorType
		expectError  bool
	}{
		{
			name:        "valid configuration",
			startURL:    "https://example.awsapps.com/start",
			region:      "us-east-1",
			expectError: false,
		},
		{
			name:         "missing start URL",
			startURL:     "",
			region:       "us-east-1",
			expectedType: ErrorTypeMissingConfig,
			expectError:  true,
		},
		{
			name:         "invalid start URL",
			startURL:     "not-a-url",
			region:       "us-east-1",
			expectedType: ErrorTypeInvalidStartURL,
			expectError:  true,
		},
		{
			name:         "missing region",
			startURL:     "https://example.awsapps.com/start",
			region:       "",
			expectedType: ErrorTypeMissingConfig,
			expectError:  true,
		},
		{
			name:         "invalid region format",
			startURL:     "https://example.awsapps.com/start",
			region:       "invalid",
			expectedType: ErrorTypeInvalidRegion,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAWSConfig(tt.startURL, tt.region)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}

				var authErr *Error
				if !errors.As(err, &authErr) {
					t.Errorf("Expected Error but got %T", err)
					return
				}

				if authErr.Type != tt.expectedType {
					t.Errorf("Expected error type %v, got %v", tt.expectedType, authErr.Type)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got %v", err)
				}
			}
		})
	}
}

func TestIsValidAWSRegion(t *testing.T) {
	tests := []struct {
		name     string
		region   string
		expected bool
	}{
		{
			name:     "valid region us-east-1",
			region:   "us-east-1",
			expected: true,
		},
		{
			name:     "valid region eu-west-2",
			region:   "eu-west-2",
			expected: true,
		},
		{
			name:     "valid region ap-southeast-1",
			region:   "ap-southeast-1",
			expected: true,
		},
		{
			name:     "invalid region - too short",
			region:   "us-east",
			expected: false,
		},
		{
			name:     "invalid region - no dashes",
			region:   "useast1",
			expected: false,
		},
		{
			name:     "invalid region - too long",
			region:   "us-east-1-extra-long",
			expected: false,
		},
		{
			name:     "empty region",
			region:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidAWSRegion(tt.region)
			if result != tt.expected {
				t.Errorf("Expected %v for region %s, got %v", tt.expected, tt.region, result)
			}
		})
	}
}

// Mock types for testing

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

type mockAPIError struct {
	code    string
	message string
}

func (e *mockAPIError) Error() string {
	return fmt.Sprintf("%s: %s", e.code, e.message)
}

func (e *mockAPIError) ErrorCode() string {
	return e.code
}

func (e *mockAPIError) ErrorMessage() string {
	return e.message
}

func (e *mockAPIError) ErrorFault() smithy.ErrorFault {
	return smithy.FaultClient
}
