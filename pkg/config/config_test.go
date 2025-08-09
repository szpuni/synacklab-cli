package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create test config file
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `aws:
  sso:
    start_url: "https://test.awsapps.com/start"
    region: "us-west-2"
github:
  token: "ghp_test_token"
  organization: "test-org"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Load config
	config, err := LoadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify AWS config values
	if config.AWS.SSO.StartURL != "https://test.awsapps.com/start" {
		t.Errorf("Expected StartURL = https://test.awsapps.com/start, got %s", config.AWS.SSO.StartURL)
	}

	if config.AWS.SSO.Region != "us-west-2" {
		t.Errorf("Expected Region = us-west-2, got %s", config.AWS.SSO.Region)
	}

	// Verify GitHub config values
	if config.GitHub.Token != "ghp_test_token" {
		t.Errorf("Expected GitHub Token = ghp_test_token, got %s", config.GitHub.Token)
	}

	if config.GitHub.Organization != "test-org" {
		t.Errorf("Expected GitHub Organization = test-org, got %s", config.GitHub.Organization)
	}
}

func TestLoadConfigNonExistent(t *testing.T) {
	// Test loading non-existent config file
	config, err := LoadConfigFromPath("/non/existent/path")
	if err != nil {
		t.Fatalf("Expected no error for non-existent config, got: %v", err)
	}

	// Should return empty config
	if config.AWS.SSO.StartURL != "" {
		t.Error("Expected empty StartURL for non-existent config")
	}
}

func TestSaveConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	configPath := filepath.Join(tempDir, "config.yaml")

	// Create and save config
	config := &Config{
		AWS: AWSConfig{
			SSO: SSOConfig{
				StartURL: "https://save-test.awsapps.com/start",
				Region:   "eu-west-1",
			},
		},
		GitHub: GitHubConfig{
			Token:        "ghp_save_test_token",
			Organization: "save-test-org",
		},
	}

	err := config.SaveConfigToPath(configPath)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load and verify saved config
	loadedConfig, err := LoadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loadedConfig.AWS.SSO.StartURL != config.AWS.SSO.StartURL {
		t.Errorf("Expected StartURL = %s, got %s", config.AWS.SSO.StartURL, loadedConfig.AWS.SSO.StartURL)
	}

	if loadedConfig.AWS.SSO.Region != config.AWS.SSO.Region {
		t.Errorf("Expected Region = %s, got %s", config.AWS.SSO.Region, loadedConfig.AWS.SSO.Region)
	}

	if loadedConfig.GitHub.Token != config.GitHub.Token {
		t.Errorf("Expected GitHub Token = %s, got %s", config.GitHub.Token, loadedConfig.GitHub.Token)
	}

	if loadedConfig.GitHub.Organization != config.GitHub.Organization {
		t.Errorf("Expected GitHub Organization = %s, got %s", config.GitHub.Organization, loadedConfig.GitHub.Organization)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				AWS: AWSConfig{
					SSO: SSOConfig{
						StartURL: "https://test.awsapps.com/start",
						Region:   "us-east-1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing start URL",
			config: Config{
				AWS: AWSConfig{
					SSO: SSOConfig{
						Region: "us-east-1",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing region",
			config: Config{
				AWS: AWSConfig{
					SSO: SSOConfig{
						StartURL: "https://test.awsapps.com/start",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetConfigPath(t *testing.T) {
	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath() failed: %v", err)
	}

	if path == "" {
		t.Error("GetConfigPath() returned empty path")
	}

	// Should contain .synacklab directory
	if !filepath.IsAbs(path) {
		t.Error("GetConfigPath() should return absolute path")
	}
}

func TestValidateGitHub(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid GitHub config",
			config: Config{
				GitHub: GitHubConfig{
					Token:        "ghp_test_token",
					Organization: "test-org",
				},
			},
			wantErr: false,
		},
		{
			name: "missing GitHub token",
			config: Config{
				GitHub: GitHubConfig{
					Organization: "test-org",
				},
			},
			wantErr: true,
			errMsg:  "GitHub token is required",
		},
		{
			name: "empty GitHub token",
			config: Config{
				GitHub: GitHubConfig{
					Token:        "",
					Organization: "test-org",
				},
			},
			wantErr: true,
			errMsg:  "GitHub token is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateGitHub()
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateGitHub() expected error but got none")
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("ValidateGitHub() error = %v, want %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateGitHub() unexpected error = %v", err)
				}
			}
		})
	}
}
