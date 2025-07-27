package cmd

import (
	"testing"
)

func TestSanitizeProfileName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Production Account", "production-account"},
		{"Dev_Environment", "dev-environment"},
		{"Test-123", "test-123"},
		{"STAGING", "staging"},
		{"Account@#$%", "account"},
		{"Multi  Space  Name", "multi-space-name"},
		{"", ""},
	}

	for _, test := range tests {
		result := sanitizeProfileName(test.input)
		if result != test.expected {
			t.Errorf("sanitizeProfileName(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestAWSProfile(t *testing.T) {
	profile := AWSProfile{
		Name:      "test-profile",
		AccountID: "123456789012",
		RoleName:  "TestRole",
		Region:    "us-east-1",
	}

	if profile.Name != "test-profile" {
		t.Errorf("Expected profile name 'test-profile', got %s", profile.Name)
	}

	if profile.AccountID != "123456789012" {
		t.Errorf("Expected account ID '123456789012', got %s", profile.AccountID)
	}
}
