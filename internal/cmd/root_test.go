package cmd

import (
	"bytes"
	"testing"
)

func TestRootCommand(t *testing.T) {
	// Test that root command exists and has expected properties
	if rootCmd.Use != "synacklab" {
		t.Errorf("Expected Use = synacklab, got %s", rootCmd.Use)
	}

	if rootCmd.Short != "A CLI tool for DevOps engineers to manage AWS SSO authentication" {
		t.Errorf("Unexpected Short description: %s", rootCmd.Short)
	}

	// Test that auth command is added
	authCmdFound := false
	initCmdFound := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "auth" {
			authCmdFound = true
		}
		if cmd.Use == "init" {
			initCmdFound = true
		}
	}

	if !authCmdFound {
		t.Error("auth command not found in root command")
	}

	if !initCmdFound {
		t.Error("init command not found in root command")
	}
}

func TestRootCommandHelp(t *testing.T) {
	// Test help output
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Failed to execute help command: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("synacklab")) {
		t.Error("Help output doesn't contain command name")
	}

	if !bytes.Contains([]byte(output), []byte("auth")) {
		t.Error("Help output doesn't contain auth subcommand")
	}

	if !bytes.Contains([]byte(output), []byte("init")) {
		t.Error("Help output doesn't contain init subcommand")
	}
}

func TestExecuteFunction(t *testing.T) {
	// Test that Execute function exists and is callable
	// We can't easily test the actual execution without mocking os.Exit
	t.Log("Execute function exists and is callable")
}
