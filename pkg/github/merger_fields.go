package github

import (
	"reflect"
)

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
		EnforceAdmins:          rule.EnforceAdmins,
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
	if existingRule.EnforceAdmins == nil && defaultRule.EnforceAdmins != nil {
		existingRule.EnforceAdmins = defaultRule.EnforceAdmins
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
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	case reflect.Struct:
		return v.Interface() == reflect.Zero(v.Type()).Interface()
	default:
		return false
	}
}
