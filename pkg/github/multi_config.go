package github

import (
	"fmt"
	"os"
	"reflect"
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

// MergeStrategy defines how defaults are merged with repository settings
type MergeStrategy int

const (
	MergeStrategyOverride  MergeStrategy = iota // Repository settings override defaults
	MergeStrategyAppend                         // Repository settings append to defaults
	MergeStrategyDeepMerge                      // Deep merge for complex objects
)

// ConfigMerger merges global defaults with repository-specific settings
type ConfigMerger interface {
	MergeDefaults(defaults *RepositoryDefaults, repo *RepositoryConfig) (*RepositoryConfig, error)
	ValidateMergedConfig(merged *RepositoryConfig) error
	SetMergeStrategy(field string, strategy MergeStrategy)
}

// DefaultConfigMerger implements ConfigMerger interface
type DefaultConfigMerger struct {
	strategies map[string]MergeStrategy
}

// NewConfigMerger creates a new DefaultConfigMerger
func NewConfigMerger() ConfigMerger {
	return &DefaultConfigMerger{
		strategies: map[string]MergeStrategy{
			"topics":        MergeStrategyOverride,
			"collaborators": MergeStrategyOverride,
			"teams":         MergeStrategyOverride,
			"webhooks":      MergeStrategyOverride,
			"branch_rules":  MergeStrategyOverride,
		},
	}
}

// SetMergeStrategy sets the merge strategy for a specific field
func (m *DefaultConfigMerger) SetMergeStrategy(field string, strategy MergeStrategy) {
	if m.strategies == nil {
		m.strategies = make(map[string]MergeStrategy)
	}
	m.strategies[field] = strategy
}

// MergeDefaults merges global defaults with repository-specific settings
func (m *DefaultConfigMerger) MergeDefaults(defaults *RepositoryDefaults, repo *RepositoryConfig) (*RepositoryConfig, error) {
	if defaults == nil {
		return m.deepCopyRepositoryConfig(repo)
	}

	// Create a deep copy of the repository config to avoid modifying the original
	merged, err := m.deepCopyRepositoryConfig(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to copy repository config: %w", err)
	}

	// Apply defaults where repository config is empty/default
	if merged.Description == "" && defaults.Description != "" {
		merged.Description = defaults.Description
	}

	// Handle Private field - apply default only if repo value is false and default is true
	// This handles the limitation of using bool instead of *bool
	if defaults.Private != nil && !repo.Private && *defaults.Private {
		merged.Private = *defaults.Private
	}

	// Merge topics based on strategy
	if err := m.mergeTopics(defaults.Topics, &merged.Topics); err != nil {
		return nil, fmt.Errorf("failed to merge topics: %w", err)
	}

	// Merge features with deep merge logic
	if err := m.mergeFeatures(defaults.Features, &merged.Features); err != nil {
		return nil, fmt.Errorf("failed to merge features: %w", err)
	}

	// Merge branch rules based on strategy
	if err := m.mergeBranchRules(defaults.BranchRules, &merged.BranchRules); err != nil {
		return nil, fmt.Errorf("failed to merge branch rules: %w", err)
	}

	// Merge collaborators based on strategy
	if err := m.mergeCollaborators(defaults.Collaborators, &merged.Collaborators); err != nil {
		return nil, fmt.Errorf("failed to merge collaborators: %w", err)
	}

	// Merge teams based on strategy
	if err := m.mergeTeams(defaults.Teams, &merged.Teams); err != nil {
		return nil, fmt.Errorf("failed to merge teams: %w", err)
	}

	// Merge webhooks based on strategy
	if err := m.mergeWebhooks(defaults.Webhooks, &merged.Webhooks); err != nil {
		return nil, fmt.Errorf("failed to merge webhooks: %w", err)
	}

	return merged, nil
}

// deepCopyRepositoryConfig creates a deep copy of a RepositoryConfig
func (m *DefaultConfigMerger) deepCopyRepositoryConfig(repo *RepositoryConfig) (*RepositoryConfig, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository config cannot be nil")
	}

	merged := &RepositoryConfig{
		Name:        repo.Name,
		Description: repo.Description,
		Private:     repo.Private,
		Features:    repo.Features,
	}

	// Deep copy slices to avoid shared references
	if repo.Topics != nil {
		merged.Topics = make([]string, len(repo.Topics))
		copy(merged.Topics, repo.Topics)
	}

	if repo.BranchRules != nil {
		merged.BranchRules = make([]BranchProtectionRule, len(repo.BranchRules))
		for i, rule := range repo.BranchRules {
			merged.BranchRules[i] = BranchProtectionRule{
				Pattern:                rule.Pattern,
				RequireUpToDate:        rule.RequireUpToDate,
				RequiredReviews:        rule.RequiredReviews,
				DismissStaleReviews:    rule.DismissStaleReviews,
				RequireCodeOwnerReview: rule.RequireCodeOwnerReview,
			}
			if rule.RequiredStatusChecks != nil {
				merged.BranchRules[i].RequiredStatusChecks = make([]string, len(rule.RequiredStatusChecks))
				copy(merged.BranchRules[i].RequiredStatusChecks, rule.RequiredStatusChecks)
			}
			if rule.RestrictPushes != nil {
				merged.BranchRules[i].RestrictPushes = make([]string, len(rule.RestrictPushes))
				copy(merged.BranchRules[i].RestrictPushes, rule.RestrictPushes)
			}
		}
	}

	if repo.Collaborators != nil {
		merged.Collaborators = make([]Collaborator, len(repo.Collaborators))
		copy(merged.Collaborators, repo.Collaborators)
	}

	if repo.Teams != nil {
		merged.Teams = make([]TeamAccess, len(repo.Teams))
		copy(merged.Teams, repo.Teams)
	}

	if repo.Webhooks != nil {
		merged.Webhooks = make([]Webhook, len(repo.Webhooks))
		for i, webhook := range repo.Webhooks {
			merged.Webhooks[i] = Webhook{
				ID:     webhook.ID,
				URL:    webhook.URL,
				Secret: webhook.Secret,
				Active: webhook.Active,
			}
			if webhook.Events != nil {
				merged.Webhooks[i].Events = make([]string, len(webhook.Events))
				copy(merged.Webhooks[i].Events, webhook.Events)
			}
		}
	}

	return merged, nil
}

// mergeTopics merges topic arrays based on the configured strategy
func (m *DefaultConfigMerger) mergeTopics(defaultTopics []string, repoTopics *[]string) error {
	if len(defaultTopics) == 0 {
		return nil
	}

	strategy := m.strategies["topics"]
	switch strategy {
	case MergeStrategyOverride:
		// Use defaults only if repository has no topics
		if len(*repoTopics) == 0 {
			*repoTopics = make([]string, len(defaultTopics))
			copy(*repoTopics, defaultTopics)
		}
	case MergeStrategyAppend:
		// Append defaults to repository topics, avoiding duplicates
		topicSet := make(map[string]bool)
		for _, topic := range *repoTopics {
			topicSet[topic] = true
		}
		for _, topic := range defaultTopics {
			if !topicSet[topic] {
				*repoTopics = append(*repoTopics, topic)
				topicSet[topic] = true
			}
		}
	case MergeStrategyDeepMerge:
		// For topics, deep merge is the same as append
		return m.mergeTopics(defaultTopics, repoTopics)
	}

	return nil
}

// mergeFeatures merges repository features with deep merge logic
func (m *DefaultConfigMerger) mergeFeatures(defaultFeatures *RepositoryFeatures, repoFeatures *RepositoryFeatures) error {
	if defaultFeatures == nil {
		return nil
	}

	// If repository features is zero value, use defaults entirely
	if isZeroValue(reflect.ValueOf(*repoFeatures)) {
		*repoFeatures = *defaultFeatures
		return nil
	}

	// Deep merge individual feature flags - only apply defaults for zero values
	if isZeroValue(reflect.ValueOf(repoFeatures.Issues)) && !isZeroValue(reflect.ValueOf(defaultFeatures.Issues)) {
		repoFeatures.Issues = defaultFeatures.Issues
	}
	if isZeroValue(reflect.ValueOf(repoFeatures.Wiki)) && !isZeroValue(reflect.ValueOf(defaultFeatures.Wiki)) {
		repoFeatures.Wiki = defaultFeatures.Wiki
	}
	if isZeroValue(reflect.ValueOf(repoFeatures.Projects)) && !isZeroValue(reflect.ValueOf(defaultFeatures.Projects)) {
		repoFeatures.Projects = defaultFeatures.Projects
	}
	if isZeroValue(reflect.ValueOf(repoFeatures.Discussions)) && !isZeroValue(reflect.ValueOf(defaultFeatures.Discussions)) {
		repoFeatures.Discussions = defaultFeatures.Discussions
	}

	return nil
}

// mergeBranchRules merges branch protection rules based on the configured strategy
func (m *DefaultConfigMerger) mergeBranchRules(defaultRules []BranchProtectionRule, repoRules *[]BranchProtectionRule) error {
	if len(defaultRules) == 0 {
		return nil
	}

	strategy := m.strategies["branch_rules"]
	switch strategy {
	case MergeStrategyOverride:
		// Use defaults only if repository has no rules
		if len(*repoRules) == 0 {
			*repoRules = make([]BranchProtectionRule, len(defaultRules))
			for i, rule := range defaultRules {
				(*repoRules)[i] = m.copyBranchRule(rule)
			}
		}
	case MergeStrategyAppend:
		// Append defaults to repository rules, avoiding duplicate patterns
		patternSet := make(map[string]bool)
		for _, rule := range *repoRules {
			patternSet[rule.Pattern] = true
		}
		for _, rule := range defaultRules {
			if !patternSet[rule.Pattern] {
				*repoRules = append(*repoRules, m.copyBranchRule(rule))
				patternSet[rule.Pattern] = true
			}
		}
	case MergeStrategyDeepMerge:
		// Deep merge rules by pattern
		ruleMap := make(map[string]*BranchProtectionRule)
		for i := range *repoRules {
			ruleMap[(*repoRules)[i].Pattern] = &(*repoRules)[i]
		}
		for _, defaultRule := range defaultRules {
			if existingRule, exists := ruleMap[defaultRule.Pattern]; exists {
				// Merge the rules
				m.mergeBranchRule(defaultRule, existingRule)
			} else {
				// Add new rule
				*repoRules = append(*repoRules, m.copyBranchRule(defaultRule))
			}
		}
	}

	return nil
}

// copyBranchRule creates a deep copy of a BranchProtectionRule
func (m *DefaultConfigMerger) copyBranchRule(rule BranchProtectionRule) BranchProtectionRule {
	copied := BranchProtectionRule{
		Pattern:                rule.Pattern,
		RequireUpToDate:        rule.RequireUpToDate,
		RequiredReviews:        rule.RequiredReviews,
		DismissStaleReviews:    rule.DismissStaleReviews,
		RequireCodeOwnerReview: rule.RequireCodeOwnerReview,
	}
	if rule.RequiredStatusChecks != nil {
		copied.RequiredStatusChecks = make([]string, len(rule.RequiredStatusChecks))
		copy(copied.RequiredStatusChecks, rule.RequiredStatusChecks)
	}
	if rule.RestrictPushes != nil {
		copied.RestrictPushes = make([]string, len(rule.RestrictPushes))
		copy(copied.RestrictPushes, rule.RestrictPushes)
	}
	return copied
}

// mergeBranchRule merges a default rule into an existing rule
func (m *DefaultConfigMerger) mergeBranchRule(defaultRule BranchProtectionRule, existingRule *BranchProtectionRule) {
	// Merge required status checks
	if len(defaultRule.RequiredStatusChecks) > 0 {
		checkSet := make(map[string]bool)
		for _, check := range existingRule.RequiredStatusChecks {
			checkSet[check] = true
		}
		for _, check := range defaultRule.RequiredStatusChecks {
			if !checkSet[check] {
				existingRule.RequiredStatusChecks = append(existingRule.RequiredStatusChecks, check)
				checkSet[check] = true
			}
		}
	}

	// Merge restrict pushes
	if len(defaultRule.RestrictPushes) > 0 {
		pushSet := make(map[string]bool)
		for _, push := range existingRule.RestrictPushes {
			pushSet[push] = true
		}
		for _, push := range defaultRule.RestrictPushes {
			if !pushSet[push] {
				existingRule.RestrictPushes = append(existingRule.RestrictPushes, push)
				pushSet[push] = true
			}
		}
	}

	// For boolean and numeric fields, keep existing values (repository overrides defaults)
	// Only apply defaults if existing values are zero/default
	if existingRule.RequiredReviews == 0 && defaultRule.RequiredReviews > 0 {
		existingRule.RequiredReviews = defaultRule.RequiredReviews
	}
	if !existingRule.RequireUpToDate && defaultRule.RequireUpToDate {
		existingRule.RequireUpToDate = defaultRule.RequireUpToDate
	}
	if !existingRule.DismissStaleReviews && defaultRule.DismissStaleReviews {
		existingRule.DismissStaleReviews = defaultRule.DismissStaleReviews
	}
	if !existingRule.RequireCodeOwnerReview && defaultRule.RequireCodeOwnerReview {
		existingRule.RequireCodeOwnerReview = defaultRule.RequireCodeOwnerReview
	}
}

// mergeCollaborators merges collaborator arrays based on the configured strategy
func (m *DefaultConfigMerger) mergeCollaborators(defaultCollaborators []Collaborator, repoCollaborators *[]Collaborator) error {
	if len(defaultCollaborators) == 0 {
		return nil
	}

	strategy := m.strategies["collaborators"]
	switch strategy {
	case MergeStrategyOverride:
		// Use defaults only if repository has no collaborators
		if len(*repoCollaborators) == 0 {
			*repoCollaborators = make([]Collaborator, len(defaultCollaborators))
			copy(*repoCollaborators, defaultCollaborators)
		}
	case MergeStrategyAppend:
		// Append defaults to repository collaborators, avoiding duplicates
		collabSet := make(map[string]bool)
		for _, collab := range *repoCollaborators {
			collabSet[collab.Username] = true
		}
		for _, collab := range defaultCollaborators {
			if !collabSet[collab.Username] {
				*repoCollaborators = append(*repoCollaborators, collab)
				collabSet[collab.Username] = true
			}
		}
	case MergeStrategyDeepMerge:
		// Deep merge collaborators by username, repository permissions override defaults
		collabMap := make(map[string]*Collaborator)
		for i := range *repoCollaborators {
			collabMap[(*repoCollaborators)[i].Username] = &(*repoCollaborators)[i]
		}
		for _, defaultCollab := range defaultCollaborators {
			if _, exists := collabMap[defaultCollab.Username]; !exists {
				*repoCollaborators = append(*repoCollaborators, defaultCollab)
			}
			// If collaborator exists, keep repository permission (no merge needed)
		}
	}

	return nil
}

// mergeTeams merges team access arrays based on the configured strategy
func (m *DefaultConfigMerger) mergeTeams(defaultTeams []TeamAccess, repoTeams *[]TeamAccess) error {
	if len(defaultTeams) == 0 {
		return nil
	}

	strategy := m.strategies["teams"]
	switch strategy {
	case MergeStrategyOverride:
		// Use defaults only if repository has no teams
		if len(*repoTeams) == 0 {
			*repoTeams = make([]TeamAccess, len(defaultTeams))
			copy(*repoTeams, defaultTeams)
		}
	case MergeStrategyAppend:
		// Append defaults to repository teams, avoiding duplicates
		teamSet := make(map[string]bool)
		for _, team := range *repoTeams {
			teamSet[team.TeamSlug] = true
		}
		for _, team := range defaultTeams {
			if !teamSet[team.TeamSlug] {
				*repoTeams = append(*repoTeams, team)
				teamSet[team.TeamSlug] = true
			}
		}
	case MergeStrategyDeepMerge:
		// Deep merge teams by slug, repository permissions override defaults
		teamMap := make(map[string]*TeamAccess)
		for i := range *repoTeams {
			teamMap[(*repoTeams)[i].TeamSlug] = &(*repoTeams)[i]
		}
		for _, defaultTeam := range defaultTeams {
			if _, exists := teamMap[defaultTeam.TeamSlug]; !exists {
				*repoTeams = append(*repoTeams, defaultTeam)
			}
			// If team exists, keep repository permission (no merge needed)
		}
	}

	return nil
}

// mergeWebhooks merges webhook arrays based on the configured strategy
func (m *DefaultConfigMerger) mergeWebhooks(defaultWebhooks []Webhook, repoWebhooks *[]Webhook) error {
	if len(defaultWebhooks) == 0 {
		return nil
	}

	strategy := m.strategies["webhooks"]
	switch strategy {
	case MergeStrategyOverride:
		// Use defaults only if repository has no webhooks
		if len(*repoWebhooks) == 0 {
			*repoWebhooks = make([]Webhook, len(defaultWebhooks))
			for i, webhook := range defaultWebhooks {
				(*repoWebhooks)[i] = m.copyWebhook(webhook)
			}
		}
	case MergeStrategyAppend:
		// Append defaults to repository webhooks, avoiding duplicate URLs
		webhookSet := make(map[string]bool)
		for _, webhook := range *repoWebhooks {
			webhookSet[webhook.URL] = true
		}
		for _, webhook := range defaultWebhooks {
			if !webhookSet[webhook.URL] {
				*repoWebhooks = append(*repoWebhooks, m.copyWebhook(webhook))
				webhookSet[webhook.URL] = true
			}
		}
	case MergeStrategyDeepMerge:
		// Deep merge webhooks by URL
		webhookMap := make(map[string]*Webhook)
		for i := range *repoWebhooks {
			webhookMap[(*repoWebhooks)[i].URL] = &(*repoWebhooks)[i]
		}
		for _, defaultWebhook := range defaultWebhooks {
			if existingWebhook, exists := webhookMap[defaultWebhook.URL]; exists {
				// Merge events
				eventSet := make(map[string]bool)
				for _, event := range existingWebhook.Events {
					eventSet[event] = true
				}
				for _, event := range defaultWebhook.Events {
					if !eventSet[event] {
						existingWebhook.Events = append(existingWebhook.Events, event)
						eventSet[event] = true
					}
				}
				// Keep repository settings for other fields (Active, Secret)
			} else {
				*repoWebhooks = append(*repoWebhooks, m.copyWebhook(defaultWebhook))
			}
		}
	}

	return nil
}

// copyWebhook creates a deep copy of a Webhook
func (m *DefaultConfigMerger) copyWebhook(webhook Webhook) Webhook {
	copied := Webhook{
		ID:     webhook.ID,
		URL:    webhook.URL,
		Secret: webhook.Secret,
		Active: webhook.Active,
	}
	if webhook.Events != nil {
		copied.Events = make([]string, len(webhook.Events))
		copy(copied.Events, webhook.Events)
	}
	return copied
}

// ValidateMergedConfig validates the merged configuration
func (m *DefaultConfigMerger) ValidateMergedConfig(merged *RepositoryConfig) error {
	return merged.Validate()
}

// isZeroValue checks if a reflect.Value represents the zero value for its type
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.String:
		return v.String() == ""
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Struct:
		return v.Interface() == reflect.Zero(v.Type()).Interface()
	default:
		return false
	}
}
