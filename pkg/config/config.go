package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the synacklab configuration
type Config struct {
	AWS AWSConfig `yaml:"aws"`
}

// AWSConfig represents AWS-specific configuration
type AWSConfig struct {
	SSO SSOConfig `yaml:"sso"`
}

// SSOConfig represents AWS SSO configuration
type SSOConfig struct {
	StartURL string `yaml:"start_url"`
	Region   string `yaml:"region"`
}

// LoadConfig loads configuration from the default location
func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	return LoadConfigFromPath(configPath)
}

// LoadConfigFromPath loads configuration from a specific path
func LoadConfigFromPath(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Config{}, nil // Return empty config if file doesn't exist
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// SaveConfig saves configuration to the default location
func (c *Config) SaveConfig() error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	return c.SaveConfigToPath(configPath)
}

// SaveConfigToPath saves configuration to a specific path
func (c *Config) SaveConfigToPath(path string) error {
	// Create config directory if it doesn't exist
	configDir := filepath.Dir(path)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConfigPath returns the default configuration file path
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".synacklab", "config.yaml"), nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.AWS.SSO.StartURL == "" {
		return fmt.Errorf("AWS SSO start URL is required")
	}

	if c.AWS.SSO.Region == "" {
		return fmt.Errorf("AWS SSO region is required")
	}

	return nil
}
