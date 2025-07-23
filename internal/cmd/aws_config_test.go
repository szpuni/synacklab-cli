package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/ini.v1"
)

func TestUpdateAWSConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	t.Setenv("HOME", tempDir)

	// Test profile
	profile := AWSProfile{
		Name:      "test-profile",
		AccountID: "123456789012",
		RoleName:  "TestRole",
		Region:    "us-west-2",
	}

	ssoStartURL := "https://test.awsapps.com/start"
	ssoRegion := "us-east-1"

	// Test updateAWSConfig
	err := updateAWSConfig(profile, ssoStartURL, ssoRegion)
	if err != nil {
		t.Fatalf("updateAWSConfig failed: %v", err)
	}

	// Verify the config file was created and has correct content
	configPath := filepath.Join(tempDir, ".aws", "config")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load and verify config content
	cfg, err := ini.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config file: %v", err)
	}

	defaultSection := cfg.Section("default")

	tests := []struct {
		key      string
		expected string
	}{
		{"sso_start_url", ssoStartURL},
		{"sso_region", ssoRegion},
		{"sso_account_id", profile.AccountID},
		{"sso_role_name", profile.RoleName},
		{"region", profile.Region},
		{"output", "json"},
	}

	for _, test := range tests {
		actual := defaultSection.Key(test.key).String()
		if actual != test.expected {
			t.Errorf("Expected %s = %s, got %s", test.key, test.expected, actual)
		}
	}
}

func TestUpdateAWSConfigExistingFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	t.Setenv("HOME", tempDir)

	// Create existing config file
	awsDir := filepath.Join(tempDir, ".aws")
	err := os.MkdirAll(awsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .aws directory: %v", err)
	}

	configPath := filepath.Join(awsDir, "config")
	existingConfig := `[default]
region = us-east-1
output = table

[profile existing]
region = eu-west-1
`
	err = os.WriteFile(configPath, []byte(existingConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to create existing config: %v", err)
	}

	// Test profile
	profile := AWSProfile{
		Name:      "new-profile",
		AccountID: "987654321098",
		RoleName:  "NewRole",
		Region:    "us-west-1",
	}

	ssoStartURL := "https://new.awsapps.com/start"
	ssoRegion := "us-west-2"

	// Update config
	err = updateAWSConfig(profile, ssoStartURL, ssoRegion)
	if err != nil {
		t.Fatalf("updateAWSConfig failed: %v", err)
	}

	// Verify the config file was updated correctly
	cfg, err := ini.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load updated config file: %v", err)
	}

	// Check default section was updated
	defaultSection := cfg.Section("default")
	if defaultSection.Key("sso_start_url").String() != ssoStartURL {
		t.Errorf("Expected sso_start_url = %s, got %s", ssoStartURL, defaultSection.Key("sso_start_url").String())
	}

	// Check existing profile section still exists
	existingSection := cfg.Section("profile existing")
	if existingSection.Key("region").String() != "eu-west-1" {
		t.Error("Existing profile section was modified or removed")
	}
}

func TestAWSProfileStruct(t *testing.T) {
	profile := AWSProfile{
		Name:      "test-profile",
		AccountID: "123456789012",
		RoleName:  "TestRole",
		Region:    "us-east-1",
	}

	if profile.Name != "test-profile" {
		t.Errorf("Expected Name = test-profile, got %s", profile.Name)
	}
	if profile.AccountID != "123456789012" {
		t.Errorf("Expected AccountID = 123456789012, got %s", profile.AccountID)
	}
	if profile.RoleName != "TestRole" {
		t.Errorf("Expected RoleName = TestRole, got %s", profile.RoleName)
	}
	if profile.Region != "us-east-1" {
		t.Errorf("Expected Region = us-east-1, got %s", profile.Region)
	}
}
