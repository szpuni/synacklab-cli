package github

import (
	"fmt"
)

type MergeStrategy int

const (
	MergeStrategyOverride  MergeStrategy = iota // Repository settings override defaults
	MergeStrategyAppend                         // Repository settings append to defaults
	MergeStrategyDeepMerge                      // Deep merge for complex objects
)

// ConfigMerger merges global defaults with repository-specific settings
type ConfigMerger interface {
	MergeDefaults(defaults *RepositoryDefaults, repo *RepositoryConfig) (*RepositoryConfig, error)
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
				EnforceAdmins:          rule.EnforceAdmins,
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
