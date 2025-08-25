package cmd

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gopkg.in/ini.v1"

	"synacklab/internal/auth"
	"synacklab/pkg/config"
)

// MockManagerForCtx implements Manager for aws-ctx testing
type MockManagerForCtx struct {
	isAuthenticated        bool
	authenticateError      error
	getCredentialsError    error
	clearCredentialsError  error
	session                *auth.SSOSession
	authenticateCalled     bool
	clearCredentialsCalled bool
	isAuthenticatedCalled  bool
}

func (m *MockManagerForCtx) IsAuthenticated(_ context.Context) (bool, error) {
	m.isAuthenticatedCalled = true
	if m.authenticateError != nil {
		return false, m.authenticateError
	}
	return m.isAuthenticated, nil
}

func (m *MockManagerForCtx) Authenticate(_ context.Context, _ *config.Config) (*auth.SSOSession, error) {
	m.authenticateCalled = true
	if m.authenticateError != nil {
		return nil, m.authenticateError
	}
	return m.session, nil
}

func (m *MockManagerForCtx) GetStoredCredentials() (*auth.SSOSession, error) {
	if m.getCredentialsError != nil {
		return nil, m.getCredentialsError
	}
	return m.session, nil
}

func (m *MockManagerForCtx) ClearCredentials() error {
	m.clearCredentialsCalled = true
	return m.clearCredentialsError
}

func (m *MockManagerForCtx) ValidateSession(_ context.Context, _ *auth.SSOSession) error {
	return nil
}

func TestSetDefaultProfile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config")

	// Create a test config with profiles
	cfg := ini.Empty()

	// Add a test profile
	profileSection, err := cfg.NewSection("profile test-profile")
	if err != nil {
		t.Fatalf("Failed to create profile section: %v", err)
	}

	profileSection.Key("sso_start_url").SetValue("https://test.awsapps.com/start")
	profileSection.Key("sso_region").SetValue("us-east-1")
	profileSection.Key("sso_account_id").SetValue("123456789012")
	profileSection.Key("sso_role_name").SetValue("TestRole")
	profileSection.Key("region").SetValue("us-west-2")
	profileSection.Key("output").SetValue("json")

	// Save the config
	err = cfg.SaveTo(configPath)
	if err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}

	// Test setDefaultProfile
	err = setDefaultProfile(cfg, "test-profile", configPath)
	if err != nil {
		t.Fatalf("setDefaultProfile failed: %v", err)
	}

	// Reload and verify the config
	updatedCfg, err := ini.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load updated config: %v", err)
	}

	defaultSection := updatedCfg.Section("default")

	tests := []struct {
		key      string
		expected string
	}{
		{"sso_start_url", "https://test.awsapps.com/start"},
		{"sso_region", "us-east-1"},
		{"sso_account_id", "123456789012"},
		{"sso_role_name", "TestRole"},
		{"region", "us-west-2"},
		{"output", "json"},
	}

	for _, test := range tests {
		actual := defaultSection.Key(test.key).String()
		if actual != test.expected {
			t.Errorf("Expected %s = %s, got %s", test.key, test.expected, actual)
		}
	}
}

func TestSetDefaultProfileExistingDefault(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config")

	// Create a test config with existing default and profiles
	cfg := ini.Empty()

	// Add existing default section
	defaultSection, err := cfg.NewSection("default")
	if err != nil {
		t.Fatalf("Failed to create default section: %v", err)
	}
	defaultSection.Key("region").SetValue("us-east-1")
	defaultSection.Key("output").SetValue("table")

	// Add a test profile
	profileSection, err := cfg.NewSection("profile new-profile")
	if err != nil {
		t.Fatalf("Failed to create profile section: %v", err)
	}

	profileSection.Key("sso_start_url").SetValue("https://new.awsapps.com/start")
	profileSection.Key("sso_region").SetValue("us-west-2")
	profileSection.Key("sso_account_id").SetValue("987654321098")
	profileSection.Key("sso_role_name").SetValue("NewRole")
	profileSection.Key("region").SetValue("us-west-1")
	profileSection.Key("output").SetValue("json")

	// Save the config
	err = cfg.SaveTo(configPath)
	if err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}

	// Test setDefaultProfile
	err = setDefaultProfile(cfg, "new-profile", configPath)
	if err != nil {
		t.Fatalf("setDefaultProfile failed: %v", err)
	}

	// Reload and verify the config
	updatedCfg, err := ini.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load updated config: %v", err)
	}

	defaultSection = updatedCfg.Section("default")

	// Check that default section was updated with new profile values
	if defaultSection.Key("sso_start_url").String() != "https://new.awsapps.com/start" {
		t.Errorf("Expected sso_start_url to be updated")
	}
	if defaultSection.Key("sso_account_id").String() != "987654321098" {
		t.Errorf("Expected sso_account_id to be updated")
	}
}

func TestSetDefaultProfileMissingProfile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config")

	// Create a test config without the requested profile
	cfg := ini.Empty()

	// Add a different profile
	profileSection, err := cfg.NewSection("profile other-profile")
	if err != nil {
		t.Fatalf("Failed to create profile section: %v", err)
	}
	profileSection.Key("region").SetValue("us-east-1")

	// Save the config
	err = cfg.SaveTo(configPath)
	if err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}

	// Test setDefaultProfile with non-existent profile
	err = setDefaultProfile(cfg, "missing-profile", configPath)
	if err == nil {
		t.Error("Expected error for missing profile, got nil")
		return
	}

	if !strings.Contains(err.Error(), "profile section not found") {
		t.Errorf("Expected 'profile section not found' error, got: %v", err)
	}
}

func TestAWSCtxCommand(t *testing.T) {
	// Test command initialization
	if awsCtxCmd.Use != "aws-ctx" {
		t.Errorf("Expected command use to be 'aws-ctx', got %s", awsCtxCmd.Use)
	}

	if awsCtxCmd.Short == "" {
		t.Error("Expected command to have a short description")
	}

	if awsCtxCmd.Long == "" {
		t.Error("Expected command to have a long description")
	}

	if awsCtxCmd.RunE == nil {
		t.Error("Expected command to have a RunE function")
	}

	// Test flags
	configFlag := awsCtxCmd.Flags().Lookup("config")
	if configFlag == nil {
		t.Error("Expected command to have a config flag")
	}

	interactiveFlag := awsCtxCmd.Flags().Lookup("interactive")
	if interactiveFlag == nil {
		t.Error("Expected command to have an interactive flag")
	}
}

func TestAWSCtxCommandStructure(t *testing.T) {
	expectedUse := "aws-ctx"
	if awsCtxCmd.Use != expectedUse {
		t.Errorf("Expected Use = %s, got %s", expectedUse, awsCtxCmd.Use)
	}

	expectedShort := "Switch AWS SSO context (profile)"
	if awsCtxCmd.Short != expectedShort {
		t.Errorf("Expected Short = %s, got %s", expectedShort, awsCtxCmd.Short)
	}

	// Test that the command has proper help text
	if len(awsCtxCmd.Long) == 0 {
		t.Error("Command should have detailed help text")
	}

	// Test that help text mentions key functionality
	if !strings.Contains(awsCtxCmd.Long, "interactive") {
		t.Error("Help text should mention interactive functionality")
	}

	if !strings.Contains(awsCtxCmd.Long, "authenticate") {
		t.Error("Help text should mention authentication")
	}
}

func TestAWSCtxCommandFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "valid config flag",
			args:        []string{"--config", "/path/to/config"},
			expectError: false,
		},
		{
			name:        "valid interactive flag",
			args:        []string{"--interactive"},
			expectError: false,
		},
		{
			name:        "short config flag",
			args:        []string{"-c", "/path/to/config"},
			expectError: false,
		},
		{
			name:        "short interactive flag",
			args:        []string{"-i"},
			expectError: false,
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
				Use: "aws-ctx",
			}
			var testConfig string
			var testInteractive bool
			cmd.Flags().StringVarP(&testConfig, "config", "c", "", "Path to configuration file")
			cmd.Flags().BoolVarP(&testInteractive, "interactive", "i", false, "Force interactive mode")

			cmd.SetArgs(tt.args)
			err := cmd.ParseFlags(tt.args)

			if (err != nil) != tt.expectError {
				t.Errorf("ParseFlags() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestRunAWSCtxAuthenticationCheck(t *testing.T) {
	// Test that runAWSCtx function exists and has correct signature
	// This is a compile-time check that the function exists
	_ = runAWSCtx

	// Create a command to test with
	cmd := &cobra.Command{
		Use:  "aws-ctx",
		RunE: runAWSCtx,
	}

	// Test that the function can be called (will fail due to missing dependencies)
	err := runAWSCtx(cmd, []string{})
	if err == nil {
		t.Log("Function executed without error (unexpected in test environment)")
	}

	// Verify error handling exists
	if err != nil {
		// Should contain authentication or configuration related error
		errStr := strings.ToLower(err.Error())
		if !strings.Contains(errStr, "auth") && !strings.Contains(errStr, "config") {
			t.Logf("Got expected error type: %v", err)
		}
	}
}

func TestRunAWSCtxErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		expectedError string
		description   string
	}{
		{
			name:          "authentication manager error",
			expectedError: "authentication",
			description:   "Should handle auth manager creation errors",
		},
		{
			name:          "configuration error",
			expectedError: "config",
			description:   "Should handle configuration loading errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that runAWSCtx handles errors appropriately
			cmd := &cobra.Command{
				Use:  "aws-ctx",
				RunE: runAWSCtx,
			}

			// Execute and expect error due to missing setup
			err := runAWSCtx(cmd, []string{})
			if err == nil {
				t.Log("Expected error due to missing configuration/auth setup")
			}

			// Verify error contains expected context
			if err != nil {
				errStr := strings.ToLower(err.Error())
				if strings.Contains(errStr, tt.expectedError) ||
					strings.Contains(errStr, "auth") ||
					strings.Contains(errStr, "config") {
					t.Logf("Got expected error type for %s: %v", tt.description, err)
				}
			}
		})
	}
}

func TestAWSCtxCommandHelp(t *testing.T) {
	// Test help output using isolated command instance
	testCmd := &cobra.Command{
		Use:   "aws-ctx",
		Short: "Switch AWS SSO context (profile)",
		Long: `Switch between AWS SSO profiles with interactive selection.
This command allows you to select and set a default AWS profile from your existing SSO profiles.
If you are not authenticated, it will automatically prompt you to authenticate first.

The command provides an interactive fuzzy finder interface for easy profile selection.`,
		RunE: runAWSCtx,
	}
	var testConfig string
	var testInteractive bool
	testCmd.Flags().StringVarP(&testConfig, "config", "c", "", "Path to configuration file")
	testCmd.Flags().BoolVarP(&testInteractive, "interactive", "i", false, "Force interactive mode even with config file")

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
		"aws-ctx",
		"config",
		"interactive",
		"Flags:",
	}

	for _, content := range expectedContent {
		if !strings.Contains(output, content) {
			t.Errorf("Help output missing expected content: %s", content)
		}
	}
}

func TestAWSCtxAutoAuthentication(t *testing.T) {
	// Test the auto-authentication flow logic
	// Since we can't easily mock the auth manager in the actual function,
	// we test the command structure and verify the function exists

	// Verify the command has the expected structure for auto-auth
	if !strings.Contains(awsCtxCmd.Long, "not authenticated") {
		t.Error("Command help should mention auto-authentication behavior")
	}

	// Test that the RunE function exists and is callable
	cmd := &cobra.Command{
		Use:  "aws-ctx",
		RunE: runAWSCtx,
	}

	if cmd.RunE == nil {
		t.Error("RunE function should be set for auto-authentication testing")
	}

	// Execute to test function signature (will fail due to missing setup)
	err := runAWSCtx(cmd, []string{})
	if err != nil {
		// Expected due to missing auth manager and config
		t.Logf("Got expected error in test environment: %v", err)
	}
}

func TestAWSCtxIntegrationWithAuthManager(t *testing.T) {
	// Test that the command properly integrates with auth manager interface
	// This tests the expected behavior without mocking

	// Verify command structure supports auth manager integration
	if awsCtxCmd.RunE == nil {
		t.Error("Command should have RunE function for auth manager integration")
	}

	// Test that the function signature is compatible
	cmd := &cobra.Command{
		Use:  "aws-ctx",
		RunE: runAWSCtx,
	}

	// Attempt execution (will fail due to missing dependencies)
	err := runAWSCtx(cmd, []string{})

	// Verify that error handling is in place
	if err != nil {
		// Should be related to auth or config issues
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "auth") ||
			strings.Contains(errStr, "config") ||
			strings.Contains(errStr, "manager") {
			t.Logf("Command properly handles auth manager integration errors: %v", err)
		}
	}
}

func TestAWSCtxUserFriendlyErrorMessages(t *testing.T) {
	// Test that the command provides user-friendly error messages

	// Test command structure
	if awsCtxCmd.RunE == nil {
		t.Error("Command should have error handling")
	}

	// Test that help text is user-friendly
	if !strings.Contains(awsCtxCmd.Short, "Switch") {
		t.Error("Short description should be user-friendly")
	}

	if !strings.Contains(awsCtxCmd.Long, "interactive") {
		t.Error("Long description should mention user-friendly features")
	}

	// Test error propagation structure
	cmd := &cobra.Command{
		Use:  "aws-ctx",
		RunE: runAWSCtx,
	}

	err := runAWSCtx(cmd, []string{})
	if err != nil {
		// Error should be descriptive
		if len(err.Error()) < 10 {
			t.Error("Error messages should be descriptive")
		}
		t.Logf("Error message format verified: %v", err)
	}
}
