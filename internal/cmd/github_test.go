package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestGitHubCommand(t *testing.T) {
	// Test that GitHub command exists and has expected properties
	if githubCmd.Use != "github" {
		t.Errorf("Expected Use = github, got %s", githubCmd.Use)
	}

	if githubCmd.Short != "GitHub repository management commands" {
		t.Errorf("Unexpected Short description: %s", githubCmd.Short)
	}

	// Test that Long description contains expected content
	expectedLongContent := []string{
		"Commands for managing GitHub repositories",
		"declarative configuration",
		"apply",
		"validate",
		"YAML configuration files",
	}

	for _, content := range expectedLongContent {
		if !bytes.Contains([]byte(githubCmd.Long), []byte(content)) {
			t.Errorf("Long description missing expected content: %s", content)
		}
	}
}

func TestGitHubCommandRegistration(t *testing.T) {
	// Test that GitHub command is registered with root command
	githubCmdFound := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "github" {
			githubCmdFound = true
			break
		}
	}

	if !githubCmdFound {
		t.Error("github command not found in root command")
	}
}

func TestGitHubCommandHelp(t *testing.T) {
	// Test help output for GitHub command by executing it through root
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"github", "--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Failed to execute GitHub help command: %v", err)
	}

	output := buf.String()

	// Check that help output contains expected content
	expectedContent := []string{
		"Commands for managing GitHub repositories",
		"declarative configuration",
		"apply",
		"validate",
		"YAML configuration files",
	}

	for _, content := range expectedContent {
		if !bytes.Contains([]byte(output), []byte(content)) {
			t.Errorf("Help output missing expected content: %s", content)
		}
	}
}

func TestGitHubCommandStructure(t *testing.T) {
	// Test that GitHub command has the correct structure
	// Note: The command will have a parent when registered with root, which is expected

	// Test that the command is runnable (should show help when no subcommand provided)
	if githubCmd.Runnable() {
		t.Error("GitHub command should not be directly runnable without subcommands")
	}

	// Test that subcommands can be added (this will be used when apply/validate are implemented)
	initialSubcommandCount := len(githubCmd.Commands())

	// Create a dummy subcommand for testing
	dummyCmd := &cobra.Command{
		Use:   "dummy",
		Short: "Dummy command for testing",
	}

	githubCmd.AddCommand(dummyCmd)

	if len(githubCmd.Commands()) != initialSubcommandCount+1 {
		t.Error("Failed to add subcommand to GitHub command")
	}

	// Clean up the dummy command
	githubCmd.RemoveCommand(dummyCmd)

	if len(githubCmd.Commands()) != initialSubcommandCount {
		t.Error("Failed to remove dummy subcommand from GitHub command")
	}
}

func TestRootCommandIncludesGitHub(t *testing.T) {
	// Test that root command help includes GitHub command
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Failed to execute root help command: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("github")) {
		t.Error("Root help output doesn't contain github subcommand")
	}
}
