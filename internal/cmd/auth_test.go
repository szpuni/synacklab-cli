package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestAuthCommand(t *testing.T) {
	// Test that auth command exists and has expected properties
	if authCmd.Use != "auth" {
		t.Errorf("Expected Use = auth, got %s", authCmd.Use)
	}

	if authCmd.Short != "Authentication commands" {
		t.Errorf("Unexpected Short description: %s", authCmd.Short)
	}

	expectedLong := `Commands for managing authentication with various cloud providers.

Use the subcommands to authenticate with AWS SSO and manage your authentication contexts.`

	if authCmd.Long != expectedLong {
		t.Errorf("Unexpected Long description: %s", authCmd.Long)
	}

	// Test that aws-login, aws-ctx and sync commands are added
	awsLoginCmdFound := false
	awsCtxCmdFound := false
	syncCmdFound := false
	for _, cmd := range authCmd.Commands() {
		if cmd.Use == "aws-login" {
			awsLoginCmdFound = true
		}
		if cmd.Use == "aws-ctx" {
			awsCtxCmdFound = true
		}
		if cmd.Use == "sync" {
			syncCmdFound = true
		}
	}

	if !awsLoginCmdFound {
		t.Error("aws-login command not found in auth command")
	}

	if !awsCtxCmdFound {
		t.Error("aws-ctx command not found in auth command")
	}

	if !syncCmdFound {
		t.Error("sync command not found in auth command")
	}
}

func TestAuthCommandHelp(t *testing.T) {
	// Create a new command instance to avoid interference
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
		Long: `Commands for managing authentication with various cloud providers.

Use the subcommands to authenticate with AWS SSO and manage your authentication contexts.`,
	}
	cmd.AddCommand(awsLoginCmd)
	cmd.AddCommand(awsCtxCmd)
	cmd.AddCommand(awsSyncCmd)

	// Test help output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Failed to execute auth help command: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("auth")) {
		t.Errorf("Help output doesn't contain command name. Output: %s", output)
	}

	if !bytes.Contains([]byte(output), []byte("aws-login")) {
		t.Errorf("Help output doesn't contain aws-login subcommand. Output: %s", output)
	}

	if !bytes.Contains([]byte(output), []byte("aws-ctx")) {
		t.Errorf("Help output doesn't contain aws-ctx subcommand. Output: %s", output)
	}

	if !bytes.Contains([]byte(output), []byte("sync")) {
		t.Errorf("Help output doesn't contain sync subcommand. Output: %s", output)
	}
}

func TestAuthCommandStructure(t *testing.T) {
	// Verify the command structure
	if len(authCmd.Commands()) == 0 {
		t.Error("Auth command has no subcommands")
	}

	// Check that aws-login, aws-ctx and sync commands exist as subcommands
	awsLoginFound := false
	awsCtxFound := false
	syncFound := false
	for _, cmd := range authCmd.Commands() {
		if cmd.Use == "aws-login" {
			awsLoginFound = true
		}
		if cmd.Use == "aws-ctx" {
			awsCtxFound = true
		}
		if cmd.Use == "sync" {
			syncFound = true
		}
	}

	if !awsLoginFound {
		t.Error("aws-login command not found as subcommand of auth")
	}

	if !awsCtxFound {
		t.Error("aws-ctx command not found as subcommand of auth")
	}

	if !syncFound {
		t.Error("sync command not found as subcommand of auth")
	}
}

func TestAuthCommandRegistration(t *testing.T) {
	// Test that all expected commands are properly registered
	expectedCommands := map[string]bool{
		"aws-login":  false,
		"aws-ctx":    false,
		"sync":       false,
		"eks-config": false,
		"eks-ctx":    false,
	}

	// Check which commands are actually registered
	for _, cmd := range authCmd.Commands() {
		if _, exists := expectedCommands[cmd.Use]; exists {
			expectedCommands[cmd.Use] = true
		}
	}

	// Verify critical commands are registered
	criticalCommands := []string{"aws-login", "aws-ctx"}
	for _, cmdName := range criticalCommands {
		if !expectedCommands[cmdName] {
			t.Errorf("Critical command '%s' not registered in auth command", cmdName)
		}
	}

	// Log all registered commands for debugging
	var registeredCommands []string
	for _, cmd := range authCmd.Commands() {
		registeredCommands = append(registeredCommands, cmd.Use)
	}
	t.Logf("Registered auth subcommands: %v", registeredCommands)
}

func TestAuthCommandSubcommandIntegration(t *testing.T) {
	// Test that subcommands are properly integrated
	tests := []struct {
		name        string
		subcommand  string
		shouldExist bool
	}{
		{
			name:        "aws-login command exists",
			subcommand:  "aws-login",
			shouldExist: true,
		},
		{
			name:        "aws-ctx command exists",
			subcommand:  "aws-ctx",
			shouldExist: true,
		},
		{
			name:        "sync command exists",
			subcommand:  "sync",
			shouldExist: true,
		},
		{
			name:        "non-existent command",
			subcommand:  "non-existent",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := false
			for _, cmd := range authCmd.Commands() {
				if cmd.Use == tt.subcommand {
					found = true
					break
				}
			}

			if found != tt.shouldExist {
				t.Errorf("Command '%s' existence = %v, want %v", tt.subcommand, found, tt.shouldExist)
			}
		})
	}
}

func TestAuthCommandErrorHandling(t *testing.T) {
	// Test that auth command handles invalid subcommands gracefully
	// Create a separate command instance to avoid interference with the global one
	testAuthCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
	}
	testAuthCmd.AddCommand(awsLoginCmd)
	testAuthCmd.AddCommand(awsCtxCmd)

	buf := new(bytes.Buffer)
	testAuthCmd.SetOut(buf)
	testAuthCmd.SetErr(buf)
	testAuthCmd.SetArgs([]string{"invalid-subcommand"})

	err := testAuthCmd.Execute()
	if err == nil {
		t.Error("Expected error for invalid subcommand")
	}
}

func TestAuthCommandHelpContent(t *testing.T) {
	// Test that help content is comprehensive and user-friendly
	// Create a separate command instance to test in isolation
	testAuthCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
		Long: `Commands for managing authentication with various cloud providers.

Use the subcommands to authenticate with AWS SSO and manage your authentication contexts.`,
	}
	testAuthCmd.AddCommand(awsLoginCmd)
	testAuthCmd.AddCommand(awsCtxCmd)

	buf := new(bytes.Buffer)
	testAuthCmd.SetOut(buf)
	testAuthCmd.SetErr(buf)
	testAuthCmd.SetArgs([]string{"--help"})

	err := testAuthCmd.Execute()
	if err != nil {
		t.Fatalf("Failed to execute auth help: %v", err)
	}

	output := buf.String()

	// Check for user-friendly content
	expectedContent := []string{
		"auth",
		"Available Commands:",
		"aws-login",
		"aws-ctx",
	}

	for _, content := range expectedContent {
		if !strings.Contains(output, content) {
			t.Errorf("Help output missing expected content: %s", content)
		}
	}
}

func TestAuthCommandSubcommandHelp(t *testing.T) {
	// Test that subcommands have proper help integration
	tests := []struct {
		name       string
		subcommand string
		args       []string
	}{
		{
			name:       "aws-login help",
			subcommand: "aws-login",
			args:       []string{"aws-login", "--help"},
		},
		{
			name:       "aws-ctx help",
			subcommand: "aws-ctx",
			args:       []string{"aws-ctx", "--help"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a separate command instance to test in isolation
			testAuthCmd := &cobra.Command{
				Use:   "auth",
				Short: "Authentication commands",
			}
			testAuthCmd.AddCommand(awsLoginCmd)
			testAuthCmd.AddCommand(awsCtxCmd)

			buf := new(bytes.Buffer)
			testAuthCmd.SetOut(buf)
			testAuthCmd.SetErr(buf)
			testAuthCmd.SetArgs(tt.args)

			err := testAuthCmd.Execute()
			if err != nil {
				t.Errorf("Failed to execute %s help: %v", tt.subcommand, err)
			}

			output := buf.String()
			if !strings.Contains(output, tt.subcommand) {
				t.Errorf("Help output for %s should contain command name", tt.subcommand)
			}

			// Reset args
			authCmd.SetArgs([]string{})
		})
	}
}

func TestAuthCommandUserExperience(t *testing.T) {
	// Test user experience aspects of the auth command

	// Test that command descriptions are clear and helpful
	if len(authCmd.Short) < 10 {
		t.Error("Short description should be descriptive")
	}

	if len(authCmd.Long) < 50 {
		t.Error("Long description should be comprehensive")
	}

	// Test that subcommands have consistent naming
	for _, cmd := range authCmd.Commands() {
		if len(cmd.Use) == 0 {
			t.Error("Subcommand should have a name")
		}

		if len(cmd.Short) == 0 {
			t.Errorf("Subcommand '%s' should have a short description", cmd.Use)
		}

		// AWS commands should follow naming convention
		if strings.HasPrefix(cmd.Use, "aws-") {
			if !strings.Contains(cmd.Short, "AWS") {
				t.Errorf("AWS command '%s' should mention AWS in description", cmd.Use)
			}
		}
	}
}

func TestAuthCommandAvailableCommands(t *testing.T) {
	// Test that the auth command properly displays available commands
	// Create a separate command instance to test in isolation
	testAuthCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
		Long: `Commands for managing authentication with various cloud providers.

Use the subcommands to authenticate with AWS SSO and manage your authentication contexts.`,
	}
	testAuthCmd.AddCommand(awsLoginCmd)
	testAuthCmd.AddCommand(awsCtxCmd)

	buf := new(bytes.Buffer)
	testAuthCmd.SetOut(buf)
	testAuthCmd.SetErr(buf)
	testAuthCmd.SetArgs([]string{"--help"})

	err := testAuthCmd.Execute()
	if err != nil {
		t.Fatalf("Failed to execute auth help: %v", err)
	}

	output := buf.String()

	// Should show available commands section
	if !strings.Contains(output, "Available Commands:") {
		t.Error("Help should show Available Commands section")
	}

	// Should not show duplicates
	awsLoginCount := strings.Count(output, "aws-login")
	if awsLoginCount > 2 { // Once in available commands, once in usage
		t.Errorf("aws-login appears too many times (%d) in help output", awsLoginCount)
	}

	awsCtxCount := strings.Count(output, "aws-ctx")
	if awsCtxCount > 2 { // Once in available commands, once in usage
		t.Errorf("aws-ctx appears too many times (%d) in help output", awsCtxCount)
	}
}
