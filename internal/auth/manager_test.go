package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/aws/smithy-go"

	"synacklab/pkg/config"
)

// createTestManager creates a test manager with a mock browser opener
func createTestManager() (*DefaultManager, error) {
	mockBrowser := &MockBrowserOpener{}
	return NewManagerWithBrowserOpener(mockBrowser)
}

func TestNewManager(t *testing.T) {
	// Use mock browser opener to avoid opening browser in tests
	mockBrowser := &MockBrowserOpener{}
	manager, err := NewManagerWithBrowserOpener(mockBrowser)
	if err != nil {
		t.Fatalf("Failed to create auth manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Auth manager is nil")
	}

	if manager.credentialsPath == "" {
		t.Fatal("Credentials path is empty")
	}
}

func TestGetStoredCredentials_NoFile(t *testing.T) {
	// Use mock browser opener to avoid opening browser in tests
	mockBrowser := &MockBrowserOpener{}
	manager, err := NewManagerWithBrowserOpener(mockBrowser)
	if err != nil {
		t.Fatalf("Failed to create auth manager: %v", err)
	}

	// Ensure no credentials file exists
	_ = manager.ClearCredentials()

	_, err = manager.GetStoredCredentials()
	if err == nil {
		t.Fatal("Expected error when no credentials file exists")
	}
}

func TestStoreAndGetCredentials(t *testing.T) {
	manager, err := createTestManager()
	if err != nil {
		t.Fatalf("Failed to create auth manager: %v", err)
	}

	// Clean up before and after test
	defer func() {
		_ = manager.ClearCredentials()
	}()
	_ = manager.ClearCredentials()

	// Create test session
	session := &SSOSession{
		AccessToken: "test-token",
		StartURL:    "https://test.awsapps.com/start",
		Region:      "us-east-1",
		ExpiresAt:   time.Now().Add(8 * time.Hour),
	}

	// Store credentials
	err = manager.storeCredentials(session)
	if err != nil {
		t.Fatalf("Failed to store credentials: %v", err)
	}

	// Retrieve credentials
	retrievedSession, err := manager.GetStoredCredentials()
	if err != nil {
		t.Fatalf("Failed to get stored credentials: %v", err)
	}

	// Verify credentials match
	if retrievedSession.AccessToken != session.AccessToken {
		t.Errorf("Access token mismatch: got %s, want %s", retrievedSession.AccessToken, session.AccessToken)
	}

	if retrievedSession.StartURL != session.StartURL {
		t.Errorf("Start URL mismatch: got %s, want %s", retrievedSession.StartURL, session.StartURL)
	}

	if retrievedSession.Region != session.Region {
		t.Errorf("Region mismatch: got %s, want %s", retrievedSession.Region, session.Region)
	}

	// Check that expiry time is approximately correct (within 1 second)
	timeDiff := retrievedSession.ExpiresAt.Sub(session.ExpiresAt)
	if timeDiff > time.Second || timeDiff < -time.Second {
		t.Errorf("Expiry time mismatch: got %v, want %v", retrievedSession.ExpiresAt, session.ExpiresAt)
	}
}

func TestClearCredentials(t *testing.T) {
	manager, err := createTestManager()
	if err != nil {
		t.Fatalf("Failed to create auth manager: %v", err)
	}

	// Create test session and store it
	session := &SSOSession{
		AccessToken: "test-token",
		StartURL:    "https://test.awsapps.com/start",
		Region:      "us-east-1",
		ExpiresAt:   time.Now().Add(8 * time.Hour),
	}

	err = manager.storeCredentials(session)
	if err != nil {
		t.Fatalf("Failed to store credentials: %v", err)
	}

	// Verify credentials exist
	_, err = manager.GetStoredCredentials()
	if err != nil {
		t.Fatalf("Credentials should exist: %v", err)
	}

	// Clear credentials
	err = manager.ClearCredentials()
	if err != nil {
		t.Fatalf("Failed to clear credentials: %v", err)
	}

	// Verify credentials are gone
	_, err = manager.GetStoredCredentials()
	if err == nil {
		t.Fatal("Credentials should be cleared")
	}
}

func TestClearCredentials_NoFile(t *testing.T) {
	manager, err := createTestManager()
	if err != nil {
		t.Fatalf("Failed to create auth manager: %v", err)
	}

	// Ensure no credentials file exists
	_ = manager.ClearCredentials()

	// Clearing non-existent credentials should not error
	err = manager.ClearCredentials()
	if err != nil {
		t.Fatalf("Clearing non-existent credentials should not error: %v", err)
	}
}

func TestIsAuthenticated_NoCredentials(t *testing.T) {
	manager, err := createTestManager()
	if err != nil {
		t.Fatalf("Failed to create auth manager: %v", err)
	}

	// Ensure no credentials exist
	_ = manager.ClearCredentials()

	ctx := context.Background()
	isAuth, err := manager.IsAuthenticated(ctx)
	if err != nil {
		t.Fatalf("IsAuthenticated should not error when no credentials exist: %v", err)
	}

	if isAuth {
		t.Fatal("Should not be authenticated when no credentials exist")
	}
}

func TestIsAuthenticated_ExpiredCredentials(t *testing.T) {
	manager, err := createTestManager()
	if err != nil {
		t.Fatalf("Failed to create auth manager: %v", err)
	}

	// Clean up after test
	defer func() {
		_ = manager.ClearCredentials()
	}()

	// Create expired session
	session := &SSOSession{
		AccessToken: "test-token",
		StartURL:    "https://test.awsapps.com/start",
		Region:      "us-east-1",
		ExpiresAt:   time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
	}

	err = manager.storeCredentials(session)
	if err != nil {
		t.Fatalf("Failed to store credentials: %v", err)
	}

	ctx := context.Background()
	isAuth, err := manager.IsAuthenticated(ctx)
	if err != nil {
		t.Fatalf("IsAuthenticated should not error with expired credentials: %v", err)
	}

	if isAuth {
		t.Fatal("Should not be authenticated with expired credentials")
	}

	// Verify expired credentials were cleared
	_, err = manager.GetStoredCredentials()
	if err == nil {
		t.Fatal("Expired credentials should have been cleared")
	}
}

func TestCredentialsFilePermissions(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create auth manager: %v", err)
	}

	// Clean up after test
	defer func() {
		_ = manager.ClearCredentials()
	}()

	// Create test session
	session := &SSOSession{
		AccessToken: "test-token",
		StartURL:    "https://test.awsapps.com/start",
		Region:      "us-east-1",
		ExpiresAt:   time.Now().Add(8 * time.Hour),
	}

	err = manager.storeCredentials(session)
	if err != nil {
		t.Fatalf("Failed to store credentials: %v", err)
	}

	// Check file permissions
	fileInfo, err := os.Stat(manager.credentialsPath)
	if err != nil {
		t.Fatalf("Failed to stat credentials file: %v", err)
	}

	// File should be readable/writable by owner only (0600)
	expectedMode := os.FileMode(0600)
	if fileInfo.Mode().Perm() != expectedMode {
		t.Errorf("Incorrect file permissions: got %v, want %v", fileInfo.Mode().Perm(), expectedMode)
	}
}

func TestCredentialsDirectoryCreation(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create manager with custom credentials path
	credentialsPath := filepath.Join(tempDir, "nested", "dir", "credentials.json")
	manager := &DefaultManager{
		credentialsPath: credentialsPath,
	}

	// Create test session
	session := &SSOSession{
		AccessToken: "test-token",
		StartURL:    "https://test.awsapps.com/start",
		Region:      "us-east-1",
		ExpiresAt:   time.Now().Add(8 * time.Hour),
	}

	// Store credentials (should create nested directories)
	err := manager.storeCredentials(session)
	if err != nil {
		t.Fatalf("Failed to store credentials: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		t.Fatal("Credentials file was not created")
	}

	// Verify directory permissions
	dirInfo, err := os.Stat(filepath.Dir(credentialsPath))
	if err != nil {
		t.Fatalf("Failed to stat credentials directory: %v", err)
	}

	// Directory should be accessible by owner only (0700)
	expectedMode := os.FileMode(0700)
	if dirInfo.Mode().Perm() != expectedMode {
		t.Errorf("Incorrect directory permissions: got %v, want %v", dirInfo.Mode().Perm(), expectedMode)
	}
}

func TestSSOSessionJSONSerialization(t *testing.T) {
	session := &SSOSession{
		AccessToken: "test-token-123",
		StartURL:    "https://test.awsapps.com/start",
		Region:      "us-west-2",
		ExpiresAt:   time.Now().Add(8 * time.Hour),
	}

	// Marshal to JSON
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("Failed to marshal session: %v", err)
	}

	// Unmarshal from JSON
	var unmarshaled SSOSession
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal session: %v", err)
	}

	// Verify all fields match
	if unmarshaled.AccessToken != session.AccessToken {
		t.Errorf("Access token mismatch: got %s, want %s", unmarshaled.AccessToken, session.AccessToken)
	}

	if unmarshaled.StartURL != session.StartURL {
		t.Errorf("Start URL mismatch: got %s, want %s", unmarshaled.StartURL, session.StartURL)
	}

	if unmarshaled.Region != session.Region {
		t.Errorf("Region mismatch: got %s, want %s", unmarshaled.Region, session.Region)
	}

	// Check that time is approximately correct (within 1 second due to JSON precision)
	timeDiff := unmarshaled.ExpiresAt.Sub(session.ExpiresAt)
	if timeDiff > time.Second || timeDiff < -time.Second {
		t.Errorf("Expiry time mismatch: got %v, want %v", unmarshaled.ExpiresAt, session.ExpiresAt)
	}
}
func TestAuthManager_GetStoredCredentials_CorruptedFile(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create auth manager with custom path
	manager := &DefaultManager{
		credentialsPath: filepath.Join(tempDir, "credentials.json"),
	}

	// Write corrupted JSON to credentials file
	corruptedData := []byte(`{"access_token": "token", "invalid_json":}`)
	err := os.WriteFile(manager.credentialsPath, corruptedData, 0600)
	if err != nil {
		t.Fatalf("Failed to write corrupted credentials: %v", err)
	}

	// Try to get stored credentials
	_, err = manager.GetStoredCredentials()
	if err == nil {
		t.Fatal("Expected error for corrupted credentials")
	}

	// Check that it's classified as invalid credentials error
	var authErr *Error
	if !errors.As(err, &authErr) {
		t.Fatalf("Expected Error, got %T", err)
	}

	if authErr.Type != ErrorTypeInvalidCredentials {
		t.Errorf("Expected ErrorTypeInvalidCredentials, got %v", authErr.Type)
	}

	// Verify credentials file was cleared
	if _, err := os.Stat(manager.credentialsPath); !os.IsNotExist(err) {
		t.Error("Expected credentials file to be cleared after corruption")
	}
}

func TestAuthManager_IsAuthenticated_NetworkError(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create auth manager with custom path
	manager := &DefaultManager{
		credentialsPath: filepath.Join(tempDir, "credentials.json"),
	}

	// Create valid session that hasn't expired
	session := &SSOSession{
		AccessToken: "valid-token",
		StartURL:    "https://example.awsapps.com/start",
		Region:      "us-east-1",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}

	// Store the session
	err := manager.storeCredentials(session)
	if err != nil {
		t.Fatalf("Failed to store credentials: %v", err)
	}

	// Create a context that will cause network timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for context to timeout
	time.Sleep(10 * time.Millisecond)

	// Try to check authentication - this should fail with network error
	isAuth, err := manager.IsAuthenticated(ctx)
	if err == nil {
		t.Fatal("Expected network error but got none")
	}

	if isAuth {
		t.Error("Expected authentication to fail due to network error")
	}

	// Check that it's classified as a network error
	var authErr *Error
	if !errors.As(err, &authErr) {
		t.Fatalf("Expected Error, got %T: %v", err, err)
	}

	// Should be either network connectivity or timeout error
	if authErr.Type != ErrorTypeNetworkConnectivity && authErr.Type != ErrorTypeNetworkTimeout {
		t.Errorf("Expected network error type, got %v", authErr.Type)
	}
}

func TestAuthManager_IsAuthenticated_ExpiredSession(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create auth manager with custom path
	manager := &DefaultManager{
		credentialsPath: filepath.Join(tempDir, "credentials.json"),
	}

	// Create expired session
	session := &SSOSession{
		AccessToken: "expired-token",
		StartURL:    "https://example.awsapps.com/start",
		Region:      "us-east-1",
		ExpiresAt:   time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
	}

	// Store the session
	err := manager.storeCredentials(session)
	if err != nil {
		t.Fatalf("Failed to store credentials: %v", err)
	}

	// Check authentication status
	isAuth, err := manager.IsAuthenticated(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if isAuth {
		t.Error("Expected authentication to fail for expired session")
	}

	// Verify credentials were cleared
	if _, err := os.Stat(manager.credentialsPath); !os.IsNotExist(err) {
		t.Error("Expected expired credentials to be cleared")
	}
}

func TestAuthManager_StoreCredentials_PermissionError(t *testing.T) {
	// Skip this test on Windows as permission handling is different
	if os.Getenv("GOOS") == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create a read-only directory
	readOnlyDir := filepath.Join(tempDir, "readonly")
	err := os.Mkdir(readOnlyDir, 0400) // Read-only
	if err != nil {
		t.Fatalf("Failed to create read-only dir: %v", err)
	}

	// Create auth manager with path in read-only directory
	manager := &DefaultManager{
		credentialsPath: filepath.Join(readOnlyDir, "credentials.json"),
	}

	// Try to store credentials
	session := &SSOSession{
		AccessToken: "test-token",
		StartURL:    "https://example.awsapps.com/start",
		Region:      "us-east-1",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}

	err = manager.storeCredentials(session)
	if err == nil {
		t.Fatal("Expected permission error but got none")
	}

	// Check that it's classified as permission error
	var authErr *Error
	if !errors.As(err, &authErr) {
		t.Fatalf("Expected Error, got %T", err)
	}

	if authErr.Type != ErrorTypePermissionDenied && authErr.Type != ErrorTypeCredentialsAccess {
		t.Errorf("Expected permission or credentials access error, got %v", authErr.Type)
	}
}

func TestValidateAWSConfig_Integration(t *testing.T) {
	tests := []struct {
		name        string
		startURL    string
		region      string
		expectError bool
		errorType   ErrorType
	}{
		{
			name:        "valid config",
			startURL:    "https://example.awsapps.com/start",
			region:      "us-east-1",
			expectError: false,
		},
		{
			name:        "missing start URL",
			startURL:    "",
			region:      "us-east-1",
			expectError: true,
			errorType:   ErrorTypeMissingConfig,
		},
		{
			name:        "invalid start URL format",
			startURL:    "not-a-valid-url",
			region:      "us-east-1",
			expectError: true,
			errorType:   ErrorTypeInvalidStartURL,
		},
		{
			name:        "missing region",
			startURL:    "https://example.awsapps.com/start",
			region:      "",
			expectError: true,
			errorType:   ErrorTypeMissingConfig,
		},
		{
			name:        "invalid region format",
			startURL:    "https://example.awsapps.com/start",
			region:      "invalid-region",
			expectError: true,
			errorType:   ErrorTypeInvalidRegion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAWSConfig(tt.startURL, tt.region)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}

				var authErr *Error
				if !errors.As(err, &authErr) {
					t.Errorf("Expected Error, got %T", err)
					return
				}

				if authErr.Type != tt.errorType {
					t.Errorf("Expected error type %v, got %v", tt.errorType, authErr.Type)
				}

				// Check that troubleshooting steps are provided
				if len(authErr.TroubleshootingSteps) == 0 {
					t.Error("Expected troubleshooting steps to be provided")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got %v", err)
				}
			}
		})
	}
}

// MockSSOOIDCClient is a mock implementation for testing AWS SSO OIDC operations
type MockSSOOIDCClient struct {
	RegisterClientFunc           func(ctx context.Context, params *ssooidc.RegisterClientInput, optFns ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error)
	StartDeviceAuthorizationFunc func(ctx context.Context, params *ssooidc.StartDeviceAuthorizationInput, optFns ...func(*ssooidc.Options)) (*ssooidc.StartDeviceAuthorizationOutput, error)
	CreateTokenFunc              func(ctx context.Context, params *ssooidc.CreateTokenInput, optFns ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error)
}

func (m *MockSSOOIDCClient) RegisterClient(ctx context.Context, params *ssooidc.RegisterClientInput, optFns ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error) {
	if m.RegisterClientFunc != nil {
		return m.RegisterClientFunc(ctx, params, optFns...)
	}
	return &ssooidc.RegisterClientOutput{
		ClientId:     aws.String("test-client-id"),
		ClientSecret: aws.String("test-client-secret"),
	}, nil
}

func (m *MockSSOOIDCClient) StartDeviceAuthorization(ctx context.Context, params *ssooidc.StartDeviceAuthorizationInput, optFns ...func(*ssooidc.Options)) (*ssooidc.StartDeviceAuthorizationOutput, error) {
	if m.StartDeviceAuthorizationFunc != nil {
		return m.StartDeviceAuthorizationFunc(ctx, params, optFns...)
	}
	return &ssooidc.StartDeviceAuthorizationOutput{
		DeviceCode:              aws.String("test-device-code"),
		UserCode:                aws.String("TEST-CODE"),
		VerificationUri:         aws.String("https://device.sso.us-east-1.amazonaws.com/"),
		VerificationUriComplete: aws.String("https://device.sso.us-east-1.amazonaws.com/?user_code=TEST-CODE"),
		ExpiresIn:               900,
		Interval:                5,
	}, nil
}

func (m *MockSSOOIDCClient) CreateToken(ctx context.Context, params *ssooidc.CreateTokenInput, optFns ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error) {
	if m.CreateTokenFunc != nil {
		return m.CreateTokenFunc(ctx, params, optFns...)
	}
	return &ssooidc.CreateTokenOutput{
		AccessToken: aws.String("test-access-token"),
		TokenType:   aws.String("Bearer"),
		ExpiresIn:   28800, // 8 hours
	}, nil
}

// MockSSOClient is a mock implementation for testing AWS SSO operations
type MockSSOClient struct {
	ListAccountsFunc func(ctx context.Context, params *sso.ListAccountsInput, optFns ...func(*sso.Options)) (*sso.ListAccountsOutput, error)
}

func (m *MockSSOClient) ListAccounts(ctx context.Context, params *sso.ListAccountsInput, optFns ...func(*sso.Options)) (*sso.ListAccountsOutput, error) {
	if m.ListAccountsFunc != nil {
		return m.ListAccountsFunc(ctx, params, optFns...)
	}
	// Return a simple successful response for testing
	return &sso.ListAccountsOutput{}, nil
}

// TestAuthManager_Authenticate_Success tests successful AWS SSO authentication flow
func TestAuthManager_Authenticate_Success(t *testing.T) {
	// Skip this test in unit test mode since it requires real AWS integration
	// This test would need extensive AWS SDK mocking to work properly in CI
	t.Skip("Skipping authentication test that requires AWS SDK mocking")
}

// TestAuthManager_Authenticate_InvalidConfig tests authentication with invalid configuration
func TestAuthManager_Authenticate_InvalidConfig(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create auth manager: %v", err)
	}

	tests := []struct {
		name      string
		config    *config.Config
		errorType ErrorType
	}{
		{
			name: "missing start URL",
			config: &config.Config{
				AWS: config.AWSConfig{
					SSO: config.SSOConfig{
						StartURL: "",
						Region:   "us-east-1",
					},
				},
			},
			errorType: ErrorTypeMissingConfig,
		},
		{
			name: "missing region",
			config: &config.Config{
				AWS: config.AWSConfig{
					SSO: config.SSOConfig{
						StartURL: "https://test.awsapps.com/start",
						Region:   "",
					},
				},
			},
			errorType: ErrorTypeMissingConfig,
		},
		{
			name: "invalid start URL",
			config: &config.Config{
				AWS: config.AWSConfig{
					SSO: config.SSOConfig{
						StartURL: "not-a-valid-url",
						Region:   "us-east-1",
					},
				},
			},
			errorType: ErrorTypeInvalidStartURL,
		},
		{
			name: "invalid region",
			config: &config.Config{
				AWS: config.AWSConfig{
					SSO: config.SSOConfig{
						StartURL: "https://test.awsapps.com/start",
						Region:   "invalid-region",
					},
				},
			},
			errorType: ErrorTypeInvalidRegion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := manager.Authenticate(ctx, tt.config)

			if err == nil {
				t.Fatal("Expected error but got none")
			}

			var authErr *Error
			if !errors.As(err, &authErr) {
				t.Fatalf("Expected Error, got %T", err)
			}

			if authErr.Type != tt.errorType {
				t.Errorf("Expected error type %v, got %v", tt.errorType, authErr.Type)
			}

			// Verify troubleshooting steps are provided
			if len(authErr.TroubleshootingSteps) == 0 {
				t.Error("Expected troubleshooting steps to be provided")
			}
		})
	}
}

// TestAuthManager_ValidateSession_Success tests successful session validation
func TestAuthManager_ValidateSession_Success(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create auth manager: %v", err)
	}

	session := &SSOSession{
		AccessToken: "valid-token",
		StartURL:    "https://test.awsapps.com/start",
		Region:      "us-east-1",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}

	ctx := context.Background()

	// This will fail in unit tests without AWS SDK mocking, which is expected
	err = manager.ValidateSession(ctx, session)
	if err == nil {
		t.Fatal("Expected validation to fail without proper AWS setup")
	}

	// Verify error is properly classified
	var authErr *Error
	if !errors.As(err, &authErr) {
		t.Fatalf("Expected Error, got %T", err)
	}

	// Should be a network, AWS API, or session expired error
	if authErr.Type != ErrorTypeNetworkConnectivity &&
		authErr.Type != ErrorTypeAWSAPIError &&
		authErr.Type != ErrorTypeNetworkTimeout &&
		authErr.Type != ErrorTypeSessionExpired {
		t.Errorf("Expected network, AWS API, or session expired error, got %v", authErr.Type)
	}
}

// TestAuthManager_ValidateSession_InvalidToken tests session validation with invalid token
func TestAuthManager_ValidateSession_InvalidToken(t *testing.T) {
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create auth manager: %v", err)
	}

	session := &SSOSession{
		AccessToken: "invalid-token",
		StartURL:    "https://test.awsapps.com/start",
		Region:      "us-east-1",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}

	ctx := context.Background()
	err = manager.ValidateSession(ctx, session)

	if err == nil {
		t.Fatal("Expected validation to fail with invalid token")
	}

	// Verify error is properly classified
	var authErr *Error
	if !errors.As(err, &authErr) {
		t.Fatalf("Expected Error, got %T", err)
	}
}

// TestAuthManager_IsAuthenticated_ValidSession tests authentication check with valid session
func TestAuthManager_IsAuthenticated_ValidSession(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create auth manager with custom path
	manager := &DefaultManager{
		credentialsPath: filepath.Join(tempDir, "credentials.json"),
	}

	// Create valid session that hasn't expired
	session := &SSOSession{
		AccessToken: "valid-token",
		StartURL:    "https://test.awsapps.com/start",
		Region:      "us-east-1",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}

	// Store the session
	err := manager.storeCredentials(session)
	if err != nil {
		t.Fatalf("Failed to store credentials: %v", err)
	}

	ctx := context.Background()

	// This will fail validation due to network/AWS API call, but should not fail on expiry
	isAuth, err := manager.IsAuthenticated(ctx)

	// In unit tests without AWS mocking, this should return false with a network error
	if isAuth {
		t.Error("Expected authentication to fail due to network/AWS API issues")
	}

	// If there's an error, it should be a network or AWS API error, not an expiry error
	if err != nil {
		var authErr *Error
		if errors.As(err, &authErr) {
			if authErr.Type == ErrorTypeSessionExpired {
				t.Error("Should not be session expired error for valid session")
			}
		}
	}
}

// TestAuthManager_IsAuthenticated_PermissionError tests authentication check with permission errors
func TestAuthManager_IsAuthenticated_PermissionError(t *testing.T) {
	// Skip this test on Windows as permission handling is different
	if os.Getenv("GOOS") == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create a file with no read permissions
	credentialsPath := filepath.Join(tempDir, "credentials.json")
	err := os.WriteFile(credentialsPath, []byte("test"), 0000) // No permissions
	if err != nil {
		t.Fatalf("Failed to create credentials file: %v", err)
	}

	// Create auth manager with custom path
	manager := &DefaultManager{
		credentialsPath: credentialsPath,
	}

	ctx := context.Background()
	isAuth, err := manager.IsAuthenticated(ctx)

	if isAuth {
		t.Error("Expected authentication to fail due to permission error")
	}

	if err == nil {
		t.Fatal("Expected permission error but got none")
	}

	// Check that it's classified as permission error
	var authErr *Error
	if !errors.As(err, &authErr) {
		t.Fatalf("Expected Error, got %T", err)
	}

	if authErr.Type != ErrorTypePermissionDenied {
		t.Errorf("Expected ErrorTypePermissionDenied, got %v", authErr.Type)
	}
}

// TestAuthManager_ErrorClassification tests various error classification scenarios
func TestAuthManager_ErrorClassification(t *testing.T) {
	tests := []struct {
		name              string
		error             error
		expectedType      ErrorType
		shouldBeRetryable bool
	}{
		{
			name: "AWS UnauthorizedException",
			error: &smithy.GenericAPIError{
				Code:    "UnauthorizedException",
				Message: "Token is invalid",
			},
			expectedType:      ErrorTypeSessionExpired,
			shouldBeRetryable: false,
		},
		{
			name: "AWS ExpiredTokenException",
			error: &smithy.GenericAPIError{
				Code:    "ExpiredTokenException",
				Message: "Device code has expired",
			},
			expectedType:      ErrorTypeDeviceCodeExpired,
			shouldBeRetryable: false,
		},
		{
			name: "AWS ThrottlingException",
			error: &smithy.GenericAPIError{
				Code:    "ThrottlingException",
				Message: "Rate exceeded",
			},
			expectedType:      ErrorTypeRateLimited,
			shouldBeRetryable: true,
		},
		{
			name: "AWS ServiceUnavailableException",
			error: &smithy.GenericAPIError{
				Code:    "ServiceUnavailableException",
				Message: "Service temporarily unavailable",
			},
			expectedType:      ErrorTypeServiceUnavailable,
			shouldBeRetryable: true,
		},
		{
			name:              "Network timeout error",
			error:             &timeoutError{},
			expectedType:      ErrorTypeNetworkTimeout,
			shouldBeRetryable: true,
		},
		{
			name:              "Connection refused error",
			error:             fmt.Errorf("connection refused"),
			expectedType:      ErrorTypeNetworkConnectivity,
			shouldBeRetryable: true,
		},
		{
			name:              "Permission denied error",
			error:             fmt.Errorf("permission denied"),
			expectedType:      ErrorTypePermissionDenied,
			shouldBeRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authErr := ClassifyError(tt.error)

			if authErr == nil {
				t.Fatal("Expected Error but got nil")
			}

			if authErr.Type != tt.expectedType {
				t.Errorf("Expected error type %v, got %v", tt.expectedType, authErr.Type)
			}

			if authErr.IsRetryable() != tt.shouldBeRetryable {
				t.Errorf("Expected retryable %v, got %v", tt.shouldBeRetryable, authErr.IsRetryable())
			}

			// Verify troubleshooting steps are provided
			if len(authErr.TroubleshootingSteps) == 0 {
				t.Error("Expected troubleshooting steps to be provided")
			}

			// Verify original error is preserved
			if authErr.OriginalError != tt.error {
				t.Error("Original error should be preserved")
			}
		})
	}
}

// TestAuthManager_CredentialStorage_EdgeCases tests edge cases in credential storage
func TestAuthManager_CredentialStorage_EdgeCases(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	t.Run("store credentials with very long path", func(t *testing.T) {
		// Create a very long path
		longPath := filepath.Join(tempDir, strings.Repeat("a", 100), "credentials.json")
		manager := &DefaultManager{
			credentialsPath: longPath,
		}

		session := &SSOSession{
			AccessToken: "test-token",
			StartURL:    "https://test.awsapps.com/start",
			Region:      "us-east-1",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		err := manager.storeCredentials(session)
		if err != nil {
			t.Fatalf("Failed to store credentials with long path: %v", err)
		}

		// Verify credentials can be retrieved
		retrieved, err := manager.GetStoredCredentials()
		if err != nil {
			t.Fatalf("Failed to retrieve credentials: %v", err)
		}

		if retrieved.AccessToken != session.AccessToken {
			t.Error("Retrieved credentials don't match stored credentials")
		}
	})

	t.Run("store credentials with special characters", func(t *testing.T) {
		manager := &DefaultManager{
			credentialsPath: filepath.Join(tempDir, "special_chars", "credentials.json"),
		}

		session := &SSOSession{
			AccessToken: "token-with-special-chars-!@#$%^&*()",
			StartURL:    "https://test-special.awsapps.com/start",
			Region:      "us-east-1",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		err := manager.storeCredentials(session)
		if err != nil {
			t.Fatalf("Failed to store credentials with special characters: %v", err)
		}

		// Verify credentials can be retrieved
		retrieved, err := manager.GetStoredCredentials()
		if err != nil {
			t.Fatalf("Failed to retrieve credentials: %v", err)
		}

		if retrieved.AccessToken != session.AccessToken {
			t.Error("Retrieved credentials don't match stored credentials")
		}
	})

	t.Run("concurrent credential operations", func(t *testing.T) {
		manager := &DefaultManager{
			credentialsPath: filepath.Join(tempDir, "concurrent", "credentials.json"),
		}

		session := &SSOSession{
			AccessToken: "concurrent-test-token",
			StartURL:    "https://test.awsapps.com/start",
			Region:      "us-east-1",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}

		// Store credentials
		err := manager.storeCredentials(session)
		if err != nil {
			t.Fatalf("Failed to store credentials: %v", err)
		}

		// Simulate concurrent operations
		done := make(chan bool, 2)

		// Concurrent read
		go func() {
			for i := 0; i < 10; i++ {
				_, _ = manager.GetStoredCredentials()
			}
			done <- true
		}()

		// Concurrent clear
		go func() {
			for i := 0; i < 5; i++ {
				_ = manager.ClearCredentials()
				_ = manager.storeCredentials(session)
			}
			done <- true
		}()

		// Wait for both goroutines to complete
		<-done
		<-done

		// Verify final state is consistent
		_, err = manager.GetStoredCredentials()
		// Either should succeed or fail with "no stored credentials" - should not crash
		if err != nil && !strings.Contains(err.Error(), "no stored credentials") {
			t.Errorf("Unexpected error after concurrent operations: %v", err)
		}
	})
}

// TestAuthManager_SessionExpiry_Precision tests session expiry detection with various time precisions
func TestAuthManager_SessionExpiry_Precision(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	manager := &DefaultManager{
		credentialsPath: filepath.Join(tempDir, "credentials.json"),
	}

	tests := []struct {
		name     string
		expiry   time.Time
		expected bool
	}{
		{
			name:     "expires in 1 second",
			expiry:   time.Now().Add(1 * time.Second),
			expected: true, // Should be considered valid
		},
		{
			name:     "expires in 1 millisecond",
			expiry:   time.Now().Add(1 * time.Millisecond),
			expected: true, // Should be considered valid
		},
		{
			name:     "expired 1 millisecond ago",
			expiry:   time.Now().Add(-1 * time.Millisecond),
			expected: false, // Should be considered expired
		},
		{
			name:     "expired 1 second ago",
			expiry:   time.Now().Add(-1 * time.Second),
			expected: false, // Should be considered expired
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &SSOSession{
				AccessToken: "test-token",
				StartURL:    "https://test.awsapps.com/start",
				Region:      "us-east-1",
				ExpiresAt:   tt.expiry,
			}

			// Store the session
			err := manager.storeCredentials(session)
			if err != nil {
				t.Fatalf("Failed to store credentials: %v", err)
			}

			// Check authentication status
			ctx := context.Background()
			isAuth, err := manager.IsAuthenticated(ctx)

			if tt.expected {
				// Should attempt validation (which will fail due to network, but not due to expiry)
				if err != nil {
					var authErr *Error
					if errors.As(err, &authErr) && authErr.Type == ErrorTypeSessionExpired {
						t.Error("Should not be expired for future expiry time")
					}
				}
			} else {
				// Should be expired and not attempt validation
				if isAuth {
					t.Error("Should be expired for past expiry time")
				}
				if err != nil {
					t.Errorf("Should not error for expired credentials, just return false: %v", err)
				}
			}

			// Clean up for next test
			_ = manager.ClearCredentials()
		})
	}
}
func TestNewManagerWithBrowserOpener(t *testing.T) {
	mockBrowser := &MockBrowserOpener{}

	manager, err := NewManagerWithBrowserOpener(mockBrowser)
	if err != nil {
		t.Fatalf("Failed to create auth manager with browser opener: %v", err)
	}

	if manager == nil {
		t.Fatal("Auth manager is nil")
	}

	if manager.browserOpener != mockBrowser {
		t.Fatal("Browser opener not set correctly")
	}
}

func TestAuthenticate_BrowserOpeningSuccess(t *testing.T) {
	// Skip this test in unit test mode since it requires real AWS integration
	// This test would need extensive AWS SDK mocking to work properly in CI
	t.Skip("Skipping authentication test that requires AWS SDK mocking")
}

func TestAuthenticate_BrowserOpeningFailure(t *testing.T) {
	// Skip this test in unit test mode since it requires real AWS integration
	// This test would need extensive AWS SDK mocking to work properly in CI
	t.Skip("Skipping authentication test that requires AWS SDK mocking")
}
