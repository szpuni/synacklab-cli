package cmd

import (
	"bytes"
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

Available commands:
  sync   - Sync AWS SSO profiles to local configuration
  config - Set default AWS profile from existing profiles`

	if authCmd.Long != expectedLong {
		t.Errorf("Unexpected Long description: %s", authCmd.Long)
	}

	// Test that config and sync commands are added
	configCmdFound := false
	syncCmdFound := false
	for _, cmd := range authCmd.Commands() {
		if cmd.Use == "config" {
			configCmdFound = true
		}
		if cmd.Use == "sync" {
			syncCmdFound = true
		}
	}

	if !configCmdFound {
		t.Error("config command not found in auth command")
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

Available commands:
  sync   - Sync AWS SSO profiles to local configuration
  config - Set default AWS profile from existing profiles`,
	}
	cmd.AddCommand(awsConfigCmd)
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

	if !bytes.Contains([]byte(output), []byte("config")) {
		t.Errorf("Help output doesn't contain config subcommand. Output: %s", output)
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

	// Check that config and sync commands exist as subcommands
	configFound := false
	syncFound := false
	for _, cmd := range authCmd.Commands() {
		if cmd.Use == "config" {
			configFound = true
		}
		if cmd.Use == "sync" {
			syncFound = true
		}
	}

	if !configFound {
		t.Error("config command not found as subcommand of auth")
	}

	if !syncFound {
		t.Error("sync command not found as subcommand of auth")
	}
}
