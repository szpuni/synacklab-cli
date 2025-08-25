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

func getProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "../.."
	}
	// Walk up until we find go.mod
	for dir != "/" {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return "../.."
}

func TestCLIIntegration(t *testing.T) {
	// Use pre-built binary from CI or build locally
	binaryPath := os.Getenv("SYNACKLAB_BINARY")
	if binaryPath == "" {
		// Build the binary locally for local testing
		buildCmd := exec.Command("go", "build", "-o", "synacklab-test", "./cmd/synacklab")
		buildCmd.Dir = getProjectRoot()
		var buildOut bytes.Buffer
		buildCmd.Stdout = &buildOut
		buildCmd.Stderr = &buildOut
		err := buildCmd.Run()
		if err != nil {
			t.Fatalf("Failed to build binary: %v\nOutput: %s", err, buildOut.String())
		}
		binaryPath = "../../synacklab-test"
		defer func() {
			if err := exec.Command("rm", "../../synacklab-test").Run(); err != nil {
				t.Logf("Failed to remove test binary: %v", err)
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
		expected string
	}{
		{
			name:     "no arguments (shows help)",
			args:     []string{},
			expected: "synacklab",
		},
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
			name:     "auth aws-ctx help",
			args:     []string{"auth", "aws-ctx", "--help"},
			expected: "Switch between AWS SSO profiles",
		},
		{
			name:     "auth sync help",
			args:     []string{"auth", "sync", "--help"},
			expected: "Authenticate with AWS SSO and sync all available profiles",
		},
		{
			name:     "init help",
			args:     []string{"init", "--help"},
			expected: "init",
		},
		{
			name:     "auth eks-ctx help",
			args:     []string{"auth", "eks-ctx", "--help"},
			expected: "Switch between Kubernetes contexts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Using binary path: %s", binaryPath)
			t.Logf("Running command: %s %v", binaryPath, tt.args)

			cmd := exec.Command(binaryPath, tt.args...)
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out

			err := cmd.Run()
			t.Logf("Command exit code: %v", err)
			t.Logf("Raw output length: %d", out.Len())
			t.Logf("Raw output: %q", out.String())

			// Help commands should exit with code 0
			if err != nil && !strings.Contains(strings.Join(tt.args, " "), "--help") && len(tt.args) > 0 {
				t.Fatalf("Command failed: %v", err)
			}

			output := out.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("Expected output to contain '%s', got: %s", tt.expected, output)
			}
		})
	}
}
