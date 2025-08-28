package auth

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/aws/smithy-go"
)

// ErrorType represents different types of authentication errors
type ErrorType string

const (
	// Network-related errors
	ErrorTypeNetworkConnectivity ErrorType = "network_connectivity"
	ErrorTypeNetworkTimeout      ErrorType = "network_timeout"
	ErrorTypeDNSResolution       ErrorType = "dns_resolution"

	// Authentication-related errors
	ErrorTypeSessionExpired       ErrorType = "session_expired"
	ErrorTypeInvalidCredentials   ErrorType = "invalid_credentials"
	ErrorTypeAuthorizationFailed  ErrorType = "authorization_failed"
	ErrorTypeDeviceCodeExpired    ErrorType = "device_code_expired"
	ErrorTypeAuthorizationPending ErrorType = "authorization_pending"
	ErrorTypeSlowDown             ErrorType = "slow_down"
	ErrorTypeExpiredToken         ErrorType = "expired_token"
	ErrorTypeAccessDenied         ErrorType = "access_denied"

	// Configuration-related errors
	ErrorTypeInvalidConfig   ErrorType = "invalid_config"
	ErrorTypeMissingConfig   ErrorType = "missing_config"
	ErrorTypeInvalidStartURL ErrorType = "invalid_start_url"
	ErrorTypeInvalidRegion   ErrorType = "invalid_region"

	// File system errors
	ErrorTypeCredentialsAccess ErrorType = "credentials_access"
	ErrorTypePermissionDenied  ErrorType = "permission_denied"

	// AWS API errors
	ErrorTypeAWSAPIError        ErrorType = "aws_api_error"
	ErrorTypeRateLimited        ErrorType = "rate_limited"
	ErrorTypeServiceUnavailable ErrorType = "service_unavailable"
)

// Error represents a structured authentication error with troubleshooting guidance
type Error struct {
	Type                 ErrorType      `json:"type"`
	Message              string         `json:"message"`
	OriginalError        error          `json:"-"`
	TroubleshootingSteps []string       `json:"troubleshooting_steps"`
	RetryAfter           *time.Duration `json:"retry_after,omitempty"`
}

// Error implements the error interface
func (e *Error) Error() string {
	return e.Message
}

// Unwrap returns the original error for error unwrapping
func (e *Error) Unwrap() error {
	return e.OriginalError
}

// IsRetryable returns true if the error is retryable
func (e *Error) IsRetryable() bool {
	switch e.Type {
	case ErrorTypeNetworkTimeout, ErrorTypeNetworkConnectivity, ErrorTypeRateLimited, ErrorTypeServiceUnavailable, ErrorTypeAuthorizationPending, ErrorTypeSlowDown:
		return true
	default:
		return false
	}
}

// GetTroubleshootingMessage returns a formatted troubleshooting message
func (e *Error) GetTroubleshootingMessage() string {
	if len(e.TroubleshootingSteps) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nTroubleshooting steps:\n")
	for i, step := range e.TroubleshootingSteps {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
	}
	return sb.String()
}

// ClassifyError analyzes an error and returns a structured Error
func ClassifyError(err error) *Error {
	if err == nil {
		return nil
	}

	// Check if it's already an Error
	var authErr *Error
	if errors.As(err, &authErr) {
		return authErr
	}

	// File system errors (check before network errors as they can overlap)
	if isFileSystemError(err) {
		return classifyFileSystemError(err)
	}

	// AWS SDK errors
	if isAWSError(err) {
		return classifyAWSError(err)
	}

	// Network connectivity errors
	if isNetworkError(err) {
		return classifyNetworkError(err)
	}

	// Configuration errors
	if isConfigurationError(err) {
		return classifyConfigurationError(err)
	}

	// Default to generic error
	return &Error{
		Type:          ErrorTypeAWSAPIError,
		Message:       fmt.Sprintf("Authentication failed: %v", err),
		OriginalError: err,
		TroubleshootingSteps: []string{
			"Check your internet connection",
			"Verify your AWS SSO configuration",
			"Try running the command again",
		},
	}
}

// isNetworkError checks if the error is network-related
func isNetworkError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	var syscallErr *net.AddrError
	if errors.As(err, &syscallErr) {
		return true
	}

	// Check for common network error strings
	errStr := strings.ToLower(err.Error())
	networkKeywords := []string{
		"connection refused",
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

// classifyNetworkError creates a specific network error
func classifyNetworkError(err error) *Error {
	errStr := strings.ToLower(err.Error())

	// DNS resolution errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) || strings.Contains(errStr, "no such host") {
		return &Error{
			Type:          ErrorTypeDNSResolution,
			Message:       "DNS resolution failed - unable to resolve AWS SSO hostname",
			OriginalError: err,
			TroubleshootingSteps: []string{
				"Check your internet connection",
				"Verify your DNS settings",
				"Try using a different DNS server (e.g., 8.8.8.8)",
				"Check if you're behind a corporate firewall",
				"Verify the AWS SSO start URL is correct",
			},
		}
	}

	// Timeout errors
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &Error{
			Type:          ErrorTypeNetworkTimeout,
			Message:       "Network timeout - request took too long to complete",
			OriginalError: err,
			TroubleshootingSteps: []string{
				"Check your internet connection speed",
				"Try again in a few moments",
				"Increase the timeout with --timeout flag",
				"Check if you're behind a slow proxy or VPN",
			},
			RetryAfter: func() *time.Duration { d := 30 * time.Second; return &d }(),
		}
	}

	// Connection refused
	if strings.Contains(errStr, "connection refused") {
		return &Error{
			Type:          ErrorTypeNetworkConnectivity,
			Message:       "Connection refused - unable to connect to AWS SSO service",
			OriginalError: err,
			TroubleshootingSteps: []string{
				"Check your internet connection",
				"Verify you can access AWS services from your network",
				"Check if you're behind a firewall that blocks AWS endpoints",
				"Try connecting from a different network",
			},
		}
	}

	// Generic network error
	return &Error{
		Type:          ErrorTypeNetworkConnectivity,
		Message:       "Network connectivity issue - unable to reach AWS SSO service",
		OriginalError: err,
		TroubleshootingSteps: []string{
			"Check your internet connection",
			"Verify network connectivity to AWS services",
			"Check firewall and proxy settings",
			"Try again in a few moments",
		},
		RetryAfter: func() *time.Duration { d := 15 * time.Second; return &d }(),
	}
}

// isAWSError checks if the error is from AWS SDK
func isAWSError(err error) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr)
}

// classifyAWSError creates a specific AWS error
func classifyAWSError(err error) *Error {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return nil
	}

	errorCode := apiErr.ErrorCode()
	errorMessage := apiErr.ErrorMessage()

	switch errorCode {
	case "UnauthorizedException", "InvalidTokenException":
		return &Error{
			Type:          ErrorTypeSessionExpired,
			Message:       "AWS SSO session has expired or is invalid",
			OriginalError: err,
			TroubleshootingSteps: []string{
				"Run 'synacklab auth aws-login' to re-authenticate",
				"Check that your AWS SSO session hasn't expired",
				"Verify your AWS SSO configuration is correct",
			},
		}

	case "ExpiredTokenException":
		return &Error{
			Type:          ErrorTypeDeviceCodeExpired,
			Message:       "Device authorization code has expired",
			OriginalError: err,
			TroubleshootingSteps: []string{
				"Start the authentication process again",
				"Complete the browser authorization more quickly",
				"Check if there are any browser issues preventing authorization",
			},
		}

	case "AuthorizationPendingException":
		return &Error{
			Type:          ErrorTypeAuthorizationPending,
			Message:       "Authorization is still pending - waiting for user to complete browser authorization",
			OriginalError: err,
			TroubleshootingSteps: []string{
				"Complete the authorization in your browser",
				"Check if the browser opened correctly",
				"Manually visit the authorization URL if needed",
			},
		}

	case "SlowDownException":
		return &Error{
			Type:          ErrorTypeSlowDown,
			Message:       "Polling too frequently - slowing down requests",
			OriginalError: err,
			TroubleshootingSteps: []string{
				"The system will automatically adjust polling frequency",
				"Please wait for the authorization to complete",
			},
		}

	case "AccessDeniedException":
		return &Error{
			Type:          ErrorTypeAccessDenied,
			Message:       "Access denied - insufficient permissions for AWS SSO",
			OriginalError: err,
			TroubleshootingSteps: []string{
				"Verify you have permission to use AWS SSO",
				"Check with your AWS administrator about SSO access",
				"Ensure your user account is properly configured in AWS SSO",
			},
		}

	case "ThrottlingException", "TooManyRequestsException":
		return &Error{
			Type:          ErrorTypeRateLimited,
			Message:       "Rate limited by AWS - too many requests",
			OriginalError: err,
			TroubleshootingSteps: []string{
				"Wait a few minutes before trying again",
				"Avoid making multiple concurrent authentication requests",
			},
			RetryAfter: func() *time.Duration { d := 60 * time.Second; return &d }(),
		}

	case "ServiceUnavailableException", "InternalServerException":
		return &Error{
			Type:          ErrorTypeServiceUnavailable,
			Message:       "AWS SSO service is temporarily unavailable",
			OriginalError: err,
			TroubleshootingSteps: []string{
				"Wait a few minutes and try again",
				"Check AWS service health status",
				"Try again later if the issue persists",
			},
			RetryAfter: func() *time.Duration { d := 120 * time.Second; return &d }(),
		}

	default:
		return &Error{
			Type:          ErrorTypeAWSAPIError,
			Message:       fmt.Sprintf("AWS API error: %s", errorMessage),
			OriginalError: err,
			TroubleshootingSteps: []string{
				"Check your AWS SSO configuration",
				"Verify your internet connection",
				"Try running the command again",
				"Check AWS service status if the issue persists",
			},
		}
	}
}

// isFileSystemError checks if the error is file system related
func isFileSystemError(err error) bool {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return true
	}

	var syscallErr syscall.Errno
	if errors.As(err, &syscallErr) {
		return true
	}

	errStr := strings.ToLower(err.Error())
	fsKeywords := []string{
		"permission denied",
		"no such file or directory",
		"file exists",
		"directory not empty",
		"read-only file system",
	}

	for _, keyword := range fsKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}

	return false
}

// classifyFileSystemError creates a specific file system error
func classifyFileSystemError(err error) *Error {
	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "permission denied") {
		return &Error{
			Type:          ErrorTypePermissionDenied,
			Message:       "Permission denied - unable to access credentials file",
			OriginalError: err,
			TroubleshootingSteps: []string{
				"Check file permissions on ~/.synacklab directory",
				"Ensure you have write access to your home directory",
				"Try running with appropriate permissions",
				"Check if the directory is owned by another user",
			},
		}
	}

	return &Error{
		Type:          ErrorTypeCredentialsAccess,
		Message:       "Unable to access credentials storage",
		OriginalError: err,
		TroubleshootingSteps: []string{
			"Check if ~/.synacklab directory exists and is accessible",
			"Verify file system permissions",
			"Ensure sufficient disk space is available",
		},
	}
}

// isConfigurationError checks if the error is configuration-related
func isConfigurationError(err error) bool {
	errStr := strings.ToLower(err.Error())
	configKeywords := []string{
		"start url",
		"region",
		"configuration",
		"config file",
		"invalid url",
	}

	for _, keyword := range configKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}

	return false
}

// classifyConfigurationError creates a specific configuration error
func classifyConfigurationError(err error) *Error {
	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "start url") {
		return &Error{
			Type:          ErrorTypeInvalidStartURL,
			Message:       "Invalid AWS SSO start URL configuration",
			OriginalError: err,
			TroubleshootingSteps: []string{
				"Check your ~/.synacklab/config.yaml file",
				"Verify the AWS SSO start URL is correct",
				"Ensure the URL format is: https://your-sso-portal.awsapps.com/start",
				"Contact your AWS administrator for the correct SSO URL",
			},
		}
	}

	if strings.Contains(errStr, "region") {
		return &Error{
			Type:          ErrorTypeInvalidRegion,
			Message:       "Invalid AWS region configuration",
			OriginalError: err,
			TroubleshootingSteps: []string{
				"Check your ~/.synacklab/config.yaml file",
				"Verify the AWS region is correct (e.g., us-east-1, eu-west-1)",
				"Ensure the region matches your AWS SSO configuration",
			},
		}
	}

	return &Error{
		Type:          ErrorTypeInvalidConfig,
		Message:       "Invalid configuration",
		OriginalError: err,
		TroubleshootingSteps: []string{
			"Check your ~/.synacklab/config.yaml file",
			"Verify all required configuration values are set",
			"Refer to the configuration documentation",
		},
	}
}

// ValidateAWSConfig validates AWS configuration and returns structured errors
func ValidateAWSConfig(startURL, region string) error {
	var validationErrors []string

	// Validate start URL
	if startURL == "" {
		return &Error{
			Type:    ErrorTypeMissingConfig,
			Message: "AWS SSO start URL is not configured",
			TroubleshootingSteps: []string{
				"Add AWS SSO configuration to ~/.synacklab/config.yaml",
				"Set the start_url field to your AWS SSO portal URL",
				"Example: https://your-sso-portal.awsapps.com/start",
				"Contact your AWS administrator for the correct URL",
			},
		}
	}

	// Validate URL format
	parsedURL, err := url.Parse(startURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return &Error{
			Type:          ErrorTypeInvalidStartURL,
			Message:       "AWS SSO start URL format is invalid",
			OriginalError: err,
			TroubleshootingSteps: []string{
				"Check the start URL format in ~/.synacklab/config.yaml",
				"Ensure it's a valid HTTPS URL",
				"Example: https://your-sso-portal.awsapps.com/start",
			},
		}
	}

	// Validate region
	if region == "" {
		return &Error{
			Type:    ErrorTypeMissingConfig,
			Message: "AWS SSO region is not configured",
			TroubleshootingSteps: []string{
				"Add AWS SSO region to ~/.synacklab/config.yaml",
				"Set the region field to your AWS SSO region",
				"Example: us-east-1, eu-west-1, ap-southeast-1",
			},
		}
	}

	// Validate region format (basic check)
	if !isValidAWSRegion(region) {
		validationErrors = append(validationErrors, fmt.Sprintf("Invalid AWS region format: %s", region))
	}

	if len(validationErrors) > 0 {
		return &Error{
			Type:    ErrorTypeInvalidRegion,
			Message: strings.Join(validationErrors, "; "),
			TroubleshootingSteps: []string{
				"Check the region format in ~/.synacklab/config.yaml",
				"Use a valid AWS region code (e.g., us-east-1, eu-west-1)",
				"Refer to AWS documentation for valid region codes",
			},
		}
	}

	return nil
}

// isValidAWSRegion performs basic validation of AWS region format
func isValidAWSRegion(region string) bool {
	// Basic regex pattern for AWS regions: us-east-1, eu-west-2, ap-southeast-1, etc.
	// This is a simple check - AWS SDK will do the authoritative validation
	if len(region) < 9 || len(region) > 15 {
		return false
	}

	parts := strings.Split(region, "-")
	return len(parts) >= 3
}
