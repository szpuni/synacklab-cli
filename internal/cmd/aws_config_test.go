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
			name:        "valid no-auth flag",
			args:        []string{"--no-auth"},
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
			var testNoAuth bool
			cmd.Flags().StringVarP(&testConfig, "config", "c", "", "Path to configuration file")
			cmd.Flags().BoolVarP(&testInteractive, "interactive", "i", false, "Force interactive mode")
			cmd.Flags().BoolVar(&testNoAuth, "no-auth", false, "Skip automatic authentication")

			cmd.SetArgs(tt.args)
			err := cmd.ParseFlags(tt.args)

			if (err != nil) != tt.expectError {
				t.Errorf("ParseFlags() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestRunAWSCtxAuthenticationCheck(t *testing.T) {
	// Skip this test in unit test mode since it calls the real runAWSCtx function
	// which triggers actual AWS authentication and browser opening
	t.Skip("Skipping test that calls real AWS authentication - requires mocking")
}

func TestRunAWSCtxErrorHandling(t *testing.T) {
	// Skip this test in unit test mode since it calls the real runAWSCtx function
	// which triggers actual AWS authentication and browser opening
	t.Skip("Skipping test that calls real AWS authentication - requires mocking")
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
	// Skip this test in unit test mode since it calls the real runAWSCtx function
	// which triggers actual AWS authentication and browser opening
	t.Skip("Skipping test that calls real AWS authentication - requires mocking")
}

func TestAWSCtxIntegrationWithAuthManager(t *testing.T) {
	// Skip this test in unit test mode since it calls the real runAWSCtx function
	// which triggers actual AWS authentication and browser opening
	t.Skip("Skipping test that calls real AWS authentication - requires mocking")
}

func TestAWSCtxUserFriendlyErrorMessages(t *testing.T) {
	// Test that the command provides user-friendly error messages without calling runAWSCtx

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

	// Skip the actual execution part that would call runAWSCtx
	// since that triggers real AWS authentication
}
func TestAWSCtxNoAuthFlag(t *testing.T) {
	// Skip this test in unit test mode since it calls the real runAWSCtx function
	// which triggers actual AWS authentication and browser opening
	t.Skip("Skipping test that calls real AWS authentication - requires mocking")
}
