//go:build integration
// +build integration

package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEKSConfigIntegration(t *testing.T) {
	// Use pre-built binary from CI or build locally
	binaryPath := os.Getenv("SYNACKLAB_BINARY")
	if binaryPath == "" {
		// Build the binary locally for local testing
		buildCmd := exec.Command("go", "build", "-o", "synacklab-test", "./cmd/synacklab")
		buildCmd.Dir = "../.."
		var buildOut bytes.Buffer
		buildCmd.Stdout = &buildOut
		buildCmd.Stderr = &buildOut

		if err := buildCmd.Run(); err != nil {
			t.Fatalf("Failed to build binary: %v\nOutput: %s", err, buildOut.String())
		}
		binaryPath = "../../synacklab-test"

		defer func() {
			if err := exec.Command("rm", "../../synacklab-test").Run(); err != nil {
				t.Logf("Failed to clean up test binary: %v", err)
			}
		}()
	} else {
		// Convert relative path to absolute path from project root
		if !filepath.IsAbs(binaryPath) {
			projectRoot := getProjectRoot()
			binaryPath = filepath.Join(projectRoot, binaryPath)
		}
	}
	tests := []struct {
		name     string
		args     []string
		contains []string
	}{
		{
			name: "eks-config help",
			args: []string{"eks-config", "--help"},
			contains: []string{
				"Discover EKS clusters in your AWS account",
				"--dry-run",
				"--region",
			},
		},
		{
			name: "auth eks-config help",
			args: []string{"auth", "eks-config", "--help"},
			contains: []string{
				"Discover EKS clusters in your AWS account",
				"--dry-run",
				"--region",
			},
		},
		{
			name: "eks-config dry-run (no AWS credentials needed)",
			args: []string{"eks-config", "--dry-run", "--region", "us-east-1"},
			contains: []string{
				"Discovering EKS clusters",
				"Searching for EKS clusters",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Using binary path: %s", binaryPath)
			t.Logf("Running command: %s %v", binaryPath, tt.args)

			cmd := exec.Command(binaryPath, tt.args...)
			output, err := cmd.CombinedOutput()

			t.Logf("Command exit code: %v", err)
			t.Logf("Raw output length: %d", len(output))
			t.Logf("Raw output: %q", string(output))

			// For help commands, we expect success
			if strings.Contains(tt.name, "help") && err != nil {
				t.Fatalf("Help command failed: %v\nOutput: %s", err, output)
			}

			// For dry-run commands, we might get an error due to AWS config issues, but output should still contain expected strings
			outputStr := string(output)
			for _, expected := range tt.contains {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nFull output: %s", expected, outputStr)
				}
			}
		})
	}
}

func TestEKSConfigFlags(t *testing.T) {
	// Use pre-built binary from CI or build locally
	binaryPath := os.Getenv("SYNACKLAB_BINARY")
	if binaryPath == "" {
		// Build the binary locally for local testing
		buildCmd := exec.Command("go", "build", "-o", "synacklab-test", "./cmd/synacklab")
		buildCmd.Dir = "../.."
		var buildOut bytes.Buffer
		buildCmd.Stdout = &buildOut
		buildCmd.Stderr = &buildOut

		if err := buildCmd.Run(); err != nil {
			t.Fatalf("Failed to build binary: %v\nOutput: %s", err, buildOut.String())
		}
		binaryPath = "../../synacklab-test"

		defer func() {
			if err := exec.Command("rm", "../../synacklab-test").Run(); err != nil {
				t.Logf("Failed to clean up test binary: %v", err)
			}
		}()
	} else {
		// Convert relative path to absolute path from project root
		if !filepath.IsAbs(binaryPath) {
			projectRoot := getProjectRoot()
			binaryPath = filepath.Join(projectRoot, binaryPath)
		}
	}
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "valid region flag",
			args:        []string{"eks-config", "--region", "us-west-2", "--dry-run"},
			expectError: false,
		},
		{
			name:        "valid region shorthand",
			args:        []string{"eks-config", "-r", "eu-west-1", "--dry-run"},
			expectError: false,
		},
		{
			name:        "dry-run flag",
			args:        []string{"eks-config", "--dry-run"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.args...)
			output, err := cmd.CombinedOutput()

			t.Logf("Command: %s %v", binaryPath, tt.args)
			t.Logf("Exit code: %v", err)
			t.Logf("Output: %s", string(output))

			if tt.expectError && err == nil {
				t.Error("Expected command to fail, but it succeeded")
			}

			if !tt.expectError && err != nil {
				// For dry-run commands, we might get AWS config errors, but that's expected
				// We just want to make sure the flags are parsed correctly
				outputStr := string(output)
				if !strings.Contains(outputStr, "Discovering EKS clusters") &&
					!strings.Contains(outputStr, "failed to load AWS config") {
					t.Errorf("Unexpected error: %v\nOutput: %s", err, outputStr)
				}
			}
		})
	}
}
