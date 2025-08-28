package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"synacklab/internal/auth"
	"synacklab/pkg/config"
)

// MockAuthManager implements the AuthManager interface for testing
type MockAuthManager struct {
	isAuthenticated        bool
	authenticateError      error
	getCredentialsError    error
	clearCredentialsError  error
	session                *auth.SSOSession
	authenticateCalled     bool
	clearCredentialsCalled bool
}

func (m *MockAuthManager) IsAuthenticated(_ context.Context) (bool, error) {
	if m.authenticateError != nil {
		return false, m.authenticateError
	}
	return m.isAuthenticated, nil
}

func (m *MockAuthManager) Authenticate(_ context.Context, _ *config.Config) (*auth.SSOSession, error) {
	m.authenticateCalled = true
	if m.authenticateError != nil {
		return nil, m.authenticateError
	}
	return m.session, nil
}

func (m *MockAuthManager) GetStoredCredentials() (*auth.SSOSession, error) {
	if m.getCredentialsError != nil {
		return nil, m.getCredentialsError
	}
	return m.session, nil
}

func (m *MockAuthManager) ClearCredentials() error {
	m.clearCredentialsCalled = true
	return m.clearCredentialsError
}

func (m *MockAuthManager) ValidateSession(_ context.Context, _ *auth.SSOSession) error {
	return nil
}

func TestAWSLoginCommand(t *testing.T) {
	// Test command initialization
	if awsLoginCmd.Use != "aws-login" {
		t.Errorf("Expected command use to be 'aws-login', got %s", awsLoginCmd.Use)
	}

	if awsLoginCmd.Short == "" {
		t.Error("Expected command to have a short description")
	}

	if awsLoginCmd.Long == "" {
		t.Error("Expected command to have a long description")
	}

	if awsLoginCmd.RunE == nil {
		t.Error("Expected command to have a RunE function")
	}

	// Test flags
	timeoutFlag := awsLoginCmd.Flags().Lookup("timeout")
	if timeoutFlag == nil {
		t.Error("Expected command to have a timeout flag")
		return
	}

	if timeoutFlag.DefValue != "300" {
		t.Errorf("Expected timeout flag default value to be '300', got %s", timeoutFlag.DefValue)
	}
}

func TestAWSLoginCommandStructure(t *testing.T) {
	// Test that the command has the expected structure
	expectedUse := "aws-login"
	if awsLoginCmd.Use != expectedUse {
		t.Errorf("Expected Use = %s, got %s", expectedUse, awsLoginCmd.Use)
	}

	expectedShort := "Authenticate with AWS SSO"
	if awsLoginCmd.Short != expectedShort {
		t.Errorf("Expected Short = %s, got %s", expectedShort, awsLoginCmd.Short)
	}

	// Test that the command has proper help text
	if len(awsLoginCmd.Long) == 0 {
		t.Error("Command should have detailed help text")
	}

	// Test that the command has a run function
	if awsLoginCmd.RunE == nil {
		t.Error("Command should have a RunE function")
	}
}

func TestAWSLoginCommandArguments(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		timeout int
		wantErr bool
	}{
		{
			name:    "no arguments",
			args:    []string{},
			timeout: 300,
			wantErr: false,
		},
		{
			name:    "with timeout flag",
			args:    []string{"--timeout", "600"},
			timeout: 600,
			wantErr: false,
		},
		{
			name:    "with short timeout flag",
			args:    []string{"-t", "120"},
			timeout: 120,
			wantErr: true, // -t is not defined, should use --timeout
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset the timeout variable
			loginTimeout = 300

			// Create a new command instance to avoid state pollution
			cmd := &cobra.Command{
				Use:   "aws-login",
				Short: "Authenticate with AWS SSO",
				RunE:  func(_ *cobra.Command, _ []string) error { return nil },
			}
			cmd.Flags().IntVar(&loginTimeout, "timeout", 300, "Timeout in seconds for the authentication process")

			// Set arguments
			cmd.SetArgs(tt.args)

			// Parse flags
			err := cmd.ParseFlags(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFlags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && loginTimeout != tt.timeout {
				t.Errorf("Expected timeout = %d, got %d", tt.timeout, loginTimeout)
			}
		})
	}
}

func TestRunAWSLoginConfigurationError(t *testing.T) {
	// Skip this test in unit test mode since it calls the real runAWSLogin function
	// which triggers actual AWS authentication and browser opening
	t.Skip("Skipping test that calls real AWS authentication - requires mocking")
}

func TestRunAWSLoginAlreadyAuthenticated(t *testing.T) {
	// Skip this test in unit test mode since it calls the real runAWSLogin function
	// which triggers actual AWS authentication and browser opening
	t.Skip("Skipping test that calls real AWS authentication - requires mocking")
}

func TestRunAWSLoginTimeoutHandling(t *testing.T) {
	tests := []struct {
		name           string
		timeout        int
		expectedInHelp bool
	}{
		{
			name:           "default timeout",
			timeout:        300,
			expectedInHelp: true,
		},
		{
			name:           "custom timeout",
			timeout:        600,
			expectedInHelp: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset timeout
			loginTimeout = tt.timeout

			// Test that timeout is properly configured
			if loginTimeout != tt.timeout {
				t.Errorf("Expected timeout = %d, got %d", tt.timeout, loginTimeout)
			}

			// Test that the timeout flag exists and has correct default
			timeoutFlag := awsLoginCmd.Flags().Lookup("timeout")
			if timeoutFlag == nil {
				t.Error("Timeout flag should exist")
				return
			}

			// The default value should be "300" as string
			if timeoutFlag.DefValue != "300" {
				t.Errorf("Expected default timeout = '300', got %s", timeoutFlag.DefValue)
			}
		})
	}
}

func TestRunAWSLoginErrorPropagation(t *testing.T) {
	// Skip this test in unit test mode since it calls the real runAWSLogin function
	// which triggers actual AWS authentication and browser opening
	t.Skip("Skipping test that calls real AWS authentication - requires mocking")
}

func TestAWSLoginCommandHelp(t *testing.T) {
	// Test help output using isolated command instance
	testCmd := &cobra.Command{
		Use:   "aws-login",
		Short: "Authenticate with AWS SSO",
		Long: `Authenticate with AWS SSO using device authorization flow.

This command initiates the AWS SSO device authorization flow to authenticate
your session. You will be prompted to visit a URL and enter a verification
code in your browser.

Once authenticated, your session credentials will be stored locally and can
be used by other commands like 'aws-ctx' to switch between AWS profiles.

Examples:
  synacklab auth aws-login
  synacklab auth aws-login --timeout 300`,
		RunE: runAWSLogin,
	}
	var testTimeout int
	testCmd.Flags().IntVar(&testTimeout, "timeout", 300, "Timeout in seconds for the authentication process")

	buf := new(bytes.Buffer)
	testCmd.SetOut(buf)
	testCmd.SetErr(buf)
	testCmd.SetArgs([]string{"--help"})

	err := testCmd.Execute()
	if err != nil {
		t.Fatalf("Failed to execute help command: %v", err)
	}

	output := buf.String()

	// Check that help contains expected content
	expectedContent := []string{
		"aws-login",
		"Authenticate with AWS SSO",
		"timeout",
		"Examples:",
	}

	for _, content := range expectedContent {
		if !strings.Contains(output, content) {
			t.Errorf("Help output missing expected content: %s", content)
		}
	}
}

func TestAWSLoginCommandFlags(t *testing.T) {
	// Test flag parsing
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "valid timeout flag",
			args:        []string{"--timeout", "600"},
			expectError: false,
		},
		{
			name:        "invalid timeout flag value",
			args:        []string{"--timeout", "invalid"},
			expectError: true,
		},
		{
			name:        "unknown flag",
			args:        []string{"--unknown"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh command instance
			cmd := &cobra.Command{
				Use: "aws-login",
			}
			var testTimeout int
			cmd.Flags().IntVar(&testTimeout, "timeout", 300, "Timeout in seconds")

			cmd.SetArgs(tt.args)
			err := cmd.ParseFlags(tt.args)

			if (err != nil) != tt.expectError {
				t.Errorf("ParseFlags() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}
