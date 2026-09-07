package github

import (
	"fmt"
	"os"
	"runtime"

	"gopkg.in/yaml.v3"
)

// ConfigFormat represents the detected configuration format
type ConfigFormat int

const (
	FormatSingleRepository ConfigFormat = iota
	FormatMultiRepository
)

// String returns the string representation of ConfigFormat
func (f ConfigFormat) String() string {
	switch f {
	case FormatSingleRepository:
		return "single-repository"
	case FormatMultiRepository:
		return "multi-repository"
	default:
		return "unknown"
	}
}

// MultiRepositoryConfig represents a multi-repository configuration
type MultiRepositoryConfig struct {
	// Version of the configuration format
	Version string `yaml:"version,omitempty"`

	// Global defaults applied to all repositories
	Defaults *RepositoryDefaults `yaml:"defaults,omitempty"`

	// List of repositories to manage
	Repositories []RepositoryConfig `yaml:"repositories" validate:"required,min=1,dive"`
}

// RepositoryDefaults defines default settings for all repositories
type RepositoryDefaults struct {
	Description   string                 `yaml:"description,omitempty" validate:"max=350"`
	Private       *bool                  `yaml:"private,omitempty"`
	Topics        []string               `yaml:"topics,omitempty" validate:"max=20,dive,min=1,max=50"`
	Features      *RepositoryFeatures    `yaml:"features,omitempty"`
	BranchRules   []BranchProtectionRule `yaml:"branch_protection,omitempty" validate:"dive"`
	Collaborators []Collaborator         `yaml:"collaborators,omitempty" validate:"dive"`
	Teams         []TeamAccess           `yaml:"teams,omitempty" validate:"dive"`
	Webhooks      []Webhook              `yaml:"webhooks,omitempty" validate:"dive"`
}

// ConfigDetector detects and loads appropriate configuration format
type ConfigDetector interface {
	DetectFormat(data []byte) (ConfigFormat, error)
	LoadSingleRepo(data []byte) (*RepositoryConfig, error)
	LoadMultiRepo(data []byte) (*MultiRepositoryConfig, error)
}

// DefaultConfigDetector implements ConfigDetector interface
type DefaultConfigDetector struct{}

// NewConfigDetector creates a new DefaultConfigDetector
func NewConfigDetector() ConfigDetector {
	return &DefaultConfigDetector{}
}

// DetectFormat detects whether the YAML data represents a single or multi-repository configuration
func (d *DefaultConfigDetector) DetectFormat(data []byte) (ConfigFormat, error) {
	// Parse as generic map to inspect structure
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return FormatSingleRepository, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Check for multi-repository indicators
	if _, hasRepositories := raw["repositories"]; hasRepositories {
		return FormatMultiRepository, nil
	}

	// Check for single repository indicators
	if _, hasName := raw["name"]; hasName {
		return FormatSingleRepository, nil
	}

	// If neither repositories array nor name field is present, try to determine
	// based on other fields. If it has defaults but no name, it's likely multi-repo
	if _, hasDefaults := raw["defaults"]; hasDefaults {
		return FormatMultiRepository, nil
	}

	// Default to single repository format for backward compatibility
	return FormatSingleRepository, nil
}

// LoadSingleRepo loads a single repository configuration
func (d *DefaultConfigDetector) LoadSingleRepo(data []byte) (*RepositoryConfig, error) {
	return LoadRepositoryConfig(data)
}

// LoadMultiRepo loads a multi-repository configuration
func (d *DefaultConfigDetector) LoadMultiRepo(data []byte) (*MultiRepositoryConfig, error) {
	var config MultiRepositoryConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse multi-repository YAML: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("multi-repository configuration validation failed: %w", err)
	}

	return &config, nil
}

// Validate validates the multi-repository configuration
func (m *MultiRepositoryConfig) Validate() error {
	var validationErrors ValidationErrors

	// Validate that we have at least one repository
	if len(m.Repositories) == 0 {
		validationErrors.Add("repositories", "", "at least one repository must be defined")
	}

	// Validate defaults if present
	if m.Defaults != nil {
		if err := m.validateDefaults(); err != nil {
			validationErrors.Add("defaults", "", err.Error())
		}
	}

	// Check for duplicate repository names
	repoNames := make(map[string]bool)
	for i, repo := range m.Repositories {
		if repo.Name == "" {
			validationErrors.Add(fmt.Sprintf("repositories[%d].name", i), "", "repository name is required")
			continue
		}

		if repoNames[repo.Name] {
			validationErrors.Add(fmt.Sprintf("repositories[%d].name", i), repo.Name, "duplicate repository name")
		}
		repoNames[repo.Name] = true

		// Validate each repository configuration
		if err := repo.Validate(); err != nil {
			validationErrors.Add(fmt.Sprintf("repositories[%d]", i), repo.Name, err.Error())
		}
	}

	if validationErrors.HasErrors() {
		return &Error{
			Type:      ErrorTypeValidation,
			Message:   validationErrors.Error(),
			Cause:     validationErrors,
			Retryable: false,
		}
	}

	return nil
}

// validateDefaults validates the defaults configuration
func (m *MultiRepositoryConfig) validateDefaults() error {
	defaults := m.Defaults

	// Validate description length
	if len(defaults.Description) > 350 {
		return fmt.Errorf("default description must be 350 characters or less")
	}

	// Validate topics
	if len(defaults.Topics) > 20 {
		return fmt.Errorf("default topics can have at most 20 items")
	}
	for i, topic := range defaults.Topics {
		if len(topic) == 0 {
			return fmt.Errorf("default topic %d cannot be empty", i+1)
		}
		if len(topic) > 50 {
			return fmt.Errorf("default topic %d must be 50 characters or less", i+1)
		}
	}

	// Validate branch protection rules
	for i, rule := range defaults.BranchRules {
		if rule.Pattern == "" {
			return fmt.Errorf("default branch protection rule %d: pattern is required", i+1)
		}
		if rule.RequiredReviews < 0 || rule.RequiredReviews > 6 {
			return fmt.Errorf("default branch protection rule %d: required reviews must be between 0 and 6", i+1)
		}
	}

	// Validate collaborators
	for i, collab := range defaults.Collaborators {
		if collab.Username == "" {
			return fmt.Errorf("default collaborator %d: username is required", i+1)
		}
		if err := validateGitHubUsername(collab.Username); err != nil {
			return fmt.Errorf("default collaborator %d: %w", i+1, err)
		}
		if !isValidPermission(collab.Permission) {
			return fmt.Errorf("default collaborator %d: permission must be one of: read, write, admin", i+1)
		}
	}

	// Validate teams
	for i, team := range defaults.Teams {
		if team.TeamSlug == "" {
			return fmt.Errorf("default team %d: team slug is required", i+1)
		}
		if err := validateGitHubTeamSlug(team.TeamSlug); err != nil {
			return fmt.Errorf("default team %d: %w", i+1, err)
		}
		if !isValidPermission(team.Permission) {
			return fmt.Errorf("default team %d: permission must be one of: read, write, admin", i+1)
		}
	}

	// Validate webhooks
	for i, webhook := range defaults.Webhooks {
		if webhook.URL == "" {
			return fmt.Errorf("default webhook %d: URL is required", i+1)
		}
		if len(webhook.Events) == 0 {
			return fmt.Errorf("default webhook %d: at least one event is required", i+1)
		}
		for j, event := range webhook.Events {
			if !isValidWebhookEvent(event) {
				return fmt.Errorf("default webhook %d, event %d: invalid event type '%s'", i+1, j+1, event)
			}
		}
	}

	return nil
}

// LoadMultiRepositoryConfig loads multi-repository configuration from YAML data
func LoadMultiRepositoryConfig(data []byte) (*MultiRepositoryConfig, error) {
	detector := NewConfigDetector()
	return detector.LoadMultiRepo(data)
}

// LoadMultiRepositoryConfigFromFile loads multi-repository configuration from a file with memory optimization
func LoadMultiRepositoryConfigFromFile(filename string) (*MultiRepositoryConfig, error) {
	// Check file size to determine loading strategy
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to stat config file: %w", err)
	}

	// For large files (>10MB), use streaming approach
	if fileInfo.Size() > 10*1024*1024 {
		return LoadMultiRepositoryConfigFromFileStreaming(filename)
	}

	// For smaller files, use the standard approach
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Detect configuration format
	detector := NewConfigDetector()
	format, err := detector.DetectFormat(data)
	if err != nil {
		return nil, fmt.Errorf("failed to detect config format: %w", err)
	}

	switch format {
	case FormatMultiRepository:
		return detector.LoadMultiRepo(data)
	case FormatSingleRepository:
		// Convert single repository config to multi-repository format
		singleConfig, err := detector.LoadSingleRepo(data)
		if err != nil {
			return nil, fmt.Errorf("failed to load single repository config: %w", err)
		}

		return &MultiRepositoryConfig{
			Repositories: []RepositoryConfig{*singleConfig},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported config format: %s", format)
	}
}

// LoadMultiRepositoryConfigFromFileStreaming loads large configuration files using streaming
func LoadMultiRepositoryConfigFromFileStreaming(filename string) (*MultiRepositoryConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Log error but don't override the main function's return value
			fmt.Fprintf(os.Stderr, "Warning: failed to close file: %v\n", err)
		}
	}()

	// Use streaming YAML decoder for memory efficiency
	decoder := yaml.NewDecoder(file)

	var config MultiRepositoryConfig
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode streaming YAML: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("streaming configuration validation failed: %w", err)
	}

	return &config, nil
}

// StreamingConfigProcessor processes large configurations in batches to reduce memory usage
type StreamingConfigProcessor struct {
	batchSize int
	processor func([]RepositoryConfig) error
}

// NewStreamingConfigProcessor creates a new streaming configuration processor
func NewStreamingConfigProcessor(batchSize int, processor func([]RepositoryConfig) error) *StreamingConfigProcessor {
	if batchSize <= 0 {
		batchSize = 50 // Default batch size
	}
	return &StreamingConfigProcessor{
		batchSize: batchSize,
		processor: processor,
	}
}

// ProcessConfig processes a configuration in batches to reduce memory usage
func (scp *StreamingConfigProcessor) ProcessConfig(config *MultiRepositoryConfig) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	repositories := config.Repositories
	totalRepos := len(repositories)

	// Process repositories in batches
	for i := 0; i < totalRepos; i += scp.batchSize {
		end := minInt(i+scp.batchSize, totalRepos)
		batch := repositories[i:end]

		if err := scp.processor(batch); err != nil {
			return fmt.Errorf("batch processing failed at repositories %d-%d: %w", i, end-1, err)
		}

		// Optional: trigger garbage collection after each batch for large configurations
		if totalRepos > 1000 && (i+scp.batchSize)%500 == 0 {
			// Allow GC to clean up processed batches
			// This is optional and may impact performance, but helps with memory usage
			runtime.GC()
		}
	}

	return nil
}

// LoadConfigFromFile loads either single or multi-repository configuration from a file
func LoadConfigFromFile(filename string) (any, ConfigFormat, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, FormatSingleRepository, fmt.Errorf("failed to read config file: %w", err)
	}

	detector := NewConfigDetector()
	format, err := detector.DetectFormat(data)
	if err != nil {
		return nil, FormatSingleRepository, fmt.Errorf("failed to detect config format: %w", err)
	}

	switch format {
	case FormatSingleRepository:
		config, err := detector.LoadSingleRepo(data)
		return config, format, err
	case FormatMultiRepository:
		config, err := detector.LoadMultiRepo(data)
		return config, format, err
	default:
		return nil, format, fmt.Errorf("unsupported config format: %s", format)
	}
}
