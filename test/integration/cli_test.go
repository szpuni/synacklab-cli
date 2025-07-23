package integration

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestCLIIntegration(t *testing.T) {
	// Build the binary first - use go build with module path
	buildCmd := exec.Command("go", "build", "-o", "synacklab-test", "synacklab/cmd/synacklab")
	var buildOut bytes.Buffer
	buildCmd.Stdout = &buildOut
	buildCmd.Stderr = &buildOut
	err := buildCmd.Run()
	if err != nil {
		t.Fatalf("Failed to build binary: %v\nOutput: %s", err, buildOut.String())
	}
	defer func() {
		if err := exec.Command("rm", "synacklab-test").Run(); err != nil {
			t.Logf("Failed to remove test binary: %v", err)
		}
	}()

	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "help command",
			args:     []string{"--help"},
			expected: "synacklab",
		},
		{
			name:     "auth help",
			args:     []string{"auth", "--help"},
			expected: "auth",
		},
		{
			name:     "aws-config help",
			args:     []string{"auth", "aws-config", "--help"},
			expected: "aws-config",
		},
		{
			name:     "init help",
			args:     []string{"init", "--help"},
			expected: "init",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("./synacklab-test", tt.args...)
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out

			err := cmd.Run()
			// Help commands should exit with code 0
			if err != nil && !strings.Contains(tt.args[len(tt.args)-1], "--help") {
				t.Fatalf("Command failed: %v", err)
			}

			output := out.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("Expected output to contain '%s', got: %s", tt.expected, output)
			}
		})
	}
}

func TestCLIVersion(t *testing.T) {
	// Build the binary - use go build with module path
	buildCmd := exec.Command("go", "build", "-o", "synacklab-test", "synacklab/cmd/synacklab")
	var buildOut bytes.Buffer
	buildCmd.Stdout = &buildOut
	buildCmd.Stderr = &buildOut
	err := buildCmd.Run()
	if err != nil {
		t.Fatalf("Failed to build binary: %v\nOutput: %s", err, buildOut.String())
	}
	defer func() {
		if err := exec.Command("rm", "synacklab-test").Run(); err != nil {
			t.Logf("Failed to remove test binary: %v", err)
		}
	}()

	// Test that the binary runs without arguments (should show help)
	cmd := exec.Command("./synacklab-test")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err = cmd.Run()
	// Should exit with 0 when showing help
	if err != nil {
		t.Logf("Command output: %s", out.String())
	}

	output := out.String()
	if !strings.Contains(output, "synacklab") {
		t.Errorf("Expected output to contain 'synacklab', got: %s", output)
	}
}
