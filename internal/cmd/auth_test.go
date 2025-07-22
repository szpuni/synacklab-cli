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

	if authCmd.Long != "Commands for managing authentication with various cloud providers" {
		t.Errorf("Unexpected Long description: %s", authCmd.Long)
	}

	// Test that aws-config command is added
	awsConfigCmdFound := false
	for _, cmd := range authCmd.Commands() {
		if cmd.Use == "aws-config" {
			awsConfigCmdFound = true
			break
		}
	}

	if !awsConfigCmdFound {
		t.Error("aws-config command not found in auth command")
	}
}

func TestAuthCommandHelp(t *testing.T) {
	// Create a new command instance to avoid interference
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
		Long:  "Commands for managing authentication with various cloud providers",
	}
	cmd.AddCommand(awsConfigCmd)

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

	if !bytes.Contains([]byte(output), []byte("aws-config")) {
		t.Errorf("Help output doesn't contain aws-config subcommand. Output: %s", output)
	}
}

func TestAuthCommandStructure(t *testing.T) {
	// Verify the command structure
	if len(authCmd.Commands()) == 0 {
		t.Error("Auth command has no subcommands")
	}

	// Check that aws-config command exists as a subcommand
	awsConfigFound := false
	for _, cmd := range authCmd.Commands() {
		if cmd.Use == "aws-config" {
			awsConfigFound = true
			// The parent relationship is set when the command is added to root
			// so we just verify the command exists in the auth command's children
			break
		}
	}

	if !awsConfigFound {
		t.Error("aws-config command not found as subcommand of auth")
	}
}
