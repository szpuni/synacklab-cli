package cmd

import (
	"path/filepath"
	"testing"

	"gopkg.in/ini.v1"
)

func TestSetDefaultProfile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config")

	// Create a test config with profiles
	cfg := ini.Empty()

	// Add a test profile
	profileSection, err := cfg.NewSection("profile test-profile")
	if err != nil {
		t.Fatalf("Failed to create profile section: %v", err)
	}

	profileSection.Key("sso_start_url").SetValue("https://test.awsapps.com/start")
	profileSection.Key("sso_region").SetValue("us-east-1")
	profileSection.Key("sso_account_id").SetValue("123456789012")
	profileSection.Key("sso_role_name").SetValue("TestRole")
	profileSection.Key("region").SetValue("us-west-2")
	profileSection.Key("output").SetValue("json")

	// Save the config
	err = cfg.SaveTo(configPath)
	if err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}

	// Test setDefaultProfile
	err = setDefaultProfile(cfg, "test-profile", configPath)
	if err != nil {
		t.Fatalf("setDefaultProfile failed: %v", err)
	}

	// Reload and verify the config
	updatedCfg, err := ini.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load updated config: %v", err)
	}

	defaultSection := updatedCfg.Section("default")

	tests := []struct {
		key      string
		expected string
	}{
		{"sso_start_url", "https://test.awsapps.com/start"},
		{"sso_region", "us-east-1"},
		{"sso_account_id", "123456789012"},
		{"sso_role_name", "TestRole"},
		{"region", "us-west-2"},
		{"output", "json"},
	}

	for _, test := range tests {
		actual := defaultSection.Key(test.key).String()
		if actual != test.expected {
			t.Errorf("Expected %s = %s, got %s", test.key, test.expected, actual)
		}
	}
}

func TestSetDefaultProfileExistingDefault(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config")

	// Create a test config with existing default and profiles
	cfg := ini.Empty()

	// Add existing default section
	defaultSection, err := cfg.NewSection("default")
	if err != nil {
		t.Fatalf("Failed to create default section: %v", err)
	}
	defaultSection.Key("region").SetValue("us-east-1")
	defaultSection.Key("output").SetValue("table")

	// Add a test profile
	profileSection, err := cfg.NewSection("profile new-profile")
	if err != nil {
		t.Fatalf("Failed to create profile section: %v", err)
	}

	profileSection.Key("sso_start_url").SetValue("https://new.awsapps.com/start")
	profileSection.Key("sso_region").SetValue("us-west-2")
	profileSection.Key("sso_account_id").SetValue("987654321098")
	profileSection.Key("sso_role_name").SetValue("NewRole")
	profileSection.Key("region").SetValue("us-west-1")
	profileSection.Key("output").SetValue("json")

	// Save the config
	err = cfg.SaveTo(configPath)
	if err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}

	// Test setDefaultProfile
	err = setDefaultProfile(cfg, "new-profile", configPath)
	if err != nil {
		t.Fatalf("setDefaultProfile failed: %v", err)
	}

	// Reload and verify the config
	updatedCfg, err := ini.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load updated config: %v", err)
	}

	defaultSection = updatedCfg.Section("default")

	// Check that default section was updated with new profile values
	if defaultSection.Key("sso_start_url").String() != "https://new.awsapps.com/start" {
		t.Errorf("Expected sso_start_url to be updated")
	}
	if defaultSection.Key("sso_account_id").String() != "987654321098" {
		t.Errorf("Expected sso_account_id to be updated")
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
