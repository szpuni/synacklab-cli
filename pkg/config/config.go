package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"synacklab/pkg/tunnel"
)

// Config represents the synacklab configuration
type Config struct {
	AWS    AWSConfig    `yaml:"aws"`
	GitHub GitHubConfig `yaml:"github"`
	Tunnel TunnelConfig `yaml:"tunnel,omitempty"`
	// LogLevel controls console log verbosity: "error", "warn", or "info"
	// (default "info" if empty). See pkg/log.ParseLevel.
	LogLevel string `yaml:"log_level,omitempty"`
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

// GitHubConfig represents GitHub-specific configuration
type GitHubConfig struct {
	Token        string `yaml:"token,omitempty"`
	Organization string `yaml:"organization,omitempty"`
}

// TunnelConfig represents `synacklab tunnel` configuration
type TunnelConfig struct {
	Namespace    string            `yaml:"namespace,omitempty"`
	Image        string            `yaml:"image,omitempty"`
	MaxDuration  string            `yaml:"max_duration,omitempty"`
	NodeSelector map[string]string `yaml:"node_selector,omitempty"`
	Databases    []TunnelDatabase  `yaml:"databases,omitempty"`
}

// TunnelDatabase is a named database target for `synacklab tunnel db`.
// Port 0 means the engine's default port.
type TunnelDatabase struct {
	Name   string `yaml:"name"`
	Engine string `yaml:"engine"`
	Host   string `yaml:"host"`
	Port   int    `yaml:"port,omitempty"`
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

	if err := os.WriteFile(path, data, 0600); err != nil {
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
	return c.ValidateAWS()
}

// ValidateAWS validates AWS-specific configuration
func (c *Config) ValidateAWS() error {
	if c.AWS.SSO.StartURL == "" {
		return fmt.Errorf("AWS SSO start URL is required. Please configure it in ~/.synacklab/config.yaml")
	}

	if c.AWS.SSO.Region == "" {
		return fmt.Errorf("AWS SSO region is required. Please configure it in ~/.synacklab/config.yaml")
	}

	return nil
}

// ValidateGitHub validates GitHub-specific configuration
func (c *Config) ValidateGitHub() error {
	if c.GitHub.Token == "" {
		return fmt.Errorf("GitHub token is required")
	}

	return nil
}

// ValidateDatabases validates tunnel.databases. It is checked only when a
// command uses them, so a broken entry never blocks an ad hoc --host tunnel.
func (t TunnelConfig) ValidateDatabases() error {
	seen := make(map[string]bool)
	for i, db := range t.Databases {
		if db.Name == "" {
			return fmt.Errorf("tunnel.databases[%d]: name is required", i)
		}
		if seen[db.Name] {
			return fmt.Errorf("tunnel.databases: duplicate database name %q", db.Name)
		}
		seen[db.Name] = true
		if _, err := db.Target(); err != nil {
			return fmt.Errorf("tunnel.databases: database %q: %w", db.Name, err)
		}
	}
	return nil
}

// Target returns the tunnel target for this database, with the engine's
// default port when Port is 0. It is the one definition of a valid target.
func (db TunnelDatabase) Target() (tunnel.Target, error) {
	port, err := tunnel.DefaultPort(db.Engine)
	if err != nil {
		return tunnel.Target{}, err
	}
	if db.Port != 0 {
		port = db.Port
	}
	target := tunnel.Target{Kind: db.Engine, Host: db.Host, Port: port}
	if err := target.Validate(); err != nil {
		return tunnel.Target{}, err
	}
	return target, nil
}

// ParseMaxDuration returns tunnel.max_duration parsed, or
// tunnel.DefaultMaxDuration if unset.
func (t TunnelConfig) ParseMaxDuration() (time.Duration, error) {
	if t.MaxDuration == "" {
		return tunnel.DefaultMaxDuration, nil
	}
	d, err := time.ParseDuration(t.MaxDuration)
	// It becomes the pod's activeDeadlineSeconds: whole seconds, at least 1.
	if err != nil || d < time.Second || d%time.Second != 0 {
		return 0, fmt.Errorf("tunnel.max_duration %q must be a whole number of seconds (at least 1s), such as 8h or 90m", t.MaxDuration)
	}
	return d, nil
}
