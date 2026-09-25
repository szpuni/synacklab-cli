package github

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// RepositoryConfig represents a complete repository configuration
type RepositoryConfig struct {
	Name          string                 `yaml:"name" validate:"required,min=1,max=100"`
	Description   string                 `yaml:"description,omitempty" validate:"max=350"`
	Private       bool                   `yaml:"private"`
	Topics        []string               `yaml:"topics,omitempty" validate:"max=20,dive,min=1,max=50"`
	Features      RepositoryFeatures     `yaml:"features,omitempty"`
	BranchRules   []BranchProtectionRule `yaml:"branch_protection,omitempty" validate:"dive"`
	Collaborators []Collaborator         `yaml:"collaborators,omitempty" validate:"dive"`
	Teams         []TeamAccess           `yaml:"teams,omitempty" validate:"dive"`
	Webhooks      []Webhook              `yaml:"webhooks,omitempty" validate:"dive"`
}

// BranchProtectionRule defines branch protection settings in configuration
type BranchProtectionRule struct {
	Pattern                string   `yaml:"pattern" validate:"required,min=1"`
	RequiredStatusChecks   []string `yaml:"required_status_checks,omitempty"`
	RequireUpToDate        bool     `yaml:"require_up_to_date"`
	RequiredReviews        int      `yaml:"required_reviews" validate:"min=0,max=6"`
	DismissStaleReviews    bool     `yaml:"dismiss_stale_reviews"`
	RequireCodeOwnerReview bool     `yaml:"require_code_owner_review"`
	RestrictPushes         []string `yaml:"restrict_pushes,omitempty"`
	// EnforceAdmins controls whether branch protection rules also apply to repository
	// admins. A pointer so an omitted value can default to true (the historical,
	// always-on behavior) while an explicit `enforce_admins: false` can opt out.
	EnforceAdmins *bool `yaml:"enforce_admins,omitempty"`
}

// EnforceAdminsEnabled resolves the effective EnforceAdmins setting, defaulting to
// true when unset so existing configs keep their current (enforced) behavior.
func (r BranchProtectionRule) EnforceAdminsEnabled() bool {
	return r.EnforceAdmins == nil || *r.EnforceAdmins
}

// Validate checks the configuration against GitHub's constraints. It is
// the one set of rules for a repository config: single-repo and multi-repo
// validation both use it. On failure it returns an ErrorTypeValidation
// *Error whose Cause is a ValidationErrors listing every problem found, each
// under its field path (e.g. "topics[2]", "webhooks[0].url").
func (r *RepositoryConfig) Validate() error {
	var errs ValidationErrors

	if err := validateGitHubRepositoryName(r.Name); err != nil {
		errs.Add("name", r.Name, err.Error())
	}
	if len(r.Description) > 350 {
		errs.Add("description", "", "repository description must be 350 characters or less")
	}

	if len(r.Topics) > 20 {
		errs.Add("topics", "", "repository can have at most 20 topics")
	}
	for i, topic := range r.Topics {
		if err := validateGitHubTopic(topic); err != nil {
			errs.Add(fmt.Sprintf("topics[%d]", i), topic, fmt.Sprintf("topic %d %s", i+1, err))
		}
	}

	for i, rule := range r.BranchRules {
		if rule.Pattern == "" {
			errs.Add(fmt.Sprintf("branch_protection[%d].pattern", i), "", fmt.Sprintf("branch protection rule %d: pattern is required", i+1))
		}
		if rule.RequiredReviews < 0 || rule.RequiredReviews > 6 {
			errs.Add(fmt.Sprintf("branch_protection[%d].required_reviews", i), strconv.Itoa(rule.RequiredReviews),
				fmt.Sprintf("branch protection rule %d: required reviews must be between 0 and 6", i+1))
		}
	}

	for i, collab := range r.Collaborators {
		field := fmt.Sprintf("collaborators[%d]", i)
		if collab.Username == "" {
			errs.Add(field+".username", "", fmt.Sprintf("collaborator %d: username is required", i+1))
		} else if err := validateGitHubUsername(collab.Username); err != nil {
			errs.Add(field+".username", collab.Username, fmt.Sprintf("collaborator %d: %v", i+1, err))
		}
		if !isValidPermission(collab.Permission) {
			errs.Add(field+".permission", collab.Permission, fmt.Sprintf("collaborator %d: permission must be one of: read, write, admin", i+1))
		}
	}

	for i, team := range r.Teams {
		field := fmt.Sprintf("teams[%d]", i)
		if team.TeamSlug == "" {
			errs.Add(field+".team", "", fmt.Sprintf("team %d: team slug is required", i+1))
		} else if err := validateGitHubTeamSlug(team.TeamSlug); err != nil {
			errs.Add(field+".team", team.TeamSlug, fmt.Sprintf("team %d: %v", i+1, err))
		}
		if !isValidPermission(team.Permission) {
			errs.Add(field+".permission", team.Permission, fmt.Sprintf("team %d: permission must be one of: read, write, admin", i+1))
		}
	}

	for i, webhook := range r.Webhooks {
		field := fmt.Sprintf("webhooks[%d]", i)
		if msg := webhookURLProblem(webhook.URL); msg != "" {
			errs.Add(field+".url", "", fmt.Sprintf("webhook %d: %s", i+1, msg))
		}
		if len(webhook.Events) == 0 {
			errs.Add(field+".events", "", fmt.Sprintf("webhook %d: at least one event is required", i+1))
		}
		for j, event := range webhook.Events {
			if !isValidWebhookEvent(event) {
				errs.Add(fmt.Sprintf("%s.events[%d]", field, j), event, fmt.Sprintf("webhook %d, event %d: invalid event type '%s'", i+1, j+1, event))
			}
		}
	}

	if errs.HasErrors() {
		return &Error{
			Type:      ErrorTypeValidation,
			Message:   errs.Error(),
			Cause:     errs,
			Retryable: false,
		}
	}
	return nil
}

// webhookURLProblem describes what is wrong with a webhook URL, or returns
// "" if it is usable. The URL itself is never echoed back, since it may
// carry credentials.
func webhookURLProblem(rawURL string) string {
	if rawURL == "" {
		return "URL is required"
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "invalid URL format"
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "URL must use http or https scheme"
	}
	if parsed.Host == "" {
		return "URL must have a valid host"
	}
	return ""
}

// isValidPermission checks if the permission level is valid
func isValidPermission(permission string) bool {
	validPermissions := map[string]bool{
		"read":  true,
		"write": true,
		"admin": true,
	}
	return validPermissions[permission]
}

// isValidWebhookEvent checks if the webhook event is valid
func isValidWebhookEvent(event string) bool {
	validEvents := map[string]bool{
		"push":                        true,
		"pull_request":                true,
		"issues":                      true,
		"issue_comment":               true,
		"pull_request_review":         true,
		"pull_request_review_comment": true,
		"commit_comment":              true,
		"create":                      true,
		"delete":                      true,
		"deployment":                  true,
		"deployment_status":           true,
		"fork":                        true,
		"gollum":                      true,
		"member":                      true,
		"membership":                  true,
		"milestone":                   true,
		"organization":                true,
		"page_build":                  true,
		"project":                     true,
		"project_card":                true,
		"project_column":              true,
		"public":                      true,
		"release":                     true,
		"repository":                  true,
		"status":                      true,
		"team":                        true,
		"team_add":                    true,
		"watch":                       true,
	}
	return validEvents[event]
}

// validateGitHubUsername validates a GitHub username according to GitHub's rules
func validateGitHubUsername(username string) error {
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	if len(username) > 39 {
		return fmt.Errorf("username must be 39 characters or less")
	}

	// GitHub username validation rules:
	// - May only contain alphanumeric characters or single hyphens
	// - Cannot begin or end with a hyphen
	// - Cannot contain consecutive hyphens
	validUsername := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)
	if !validUsername.MatchString(username) {
		return fmt.Errorf("username '%s' is invalid: must contain only alphanumeric characters and single hyphens, cannot start or end with hyphen", username)
	}

	// Check for consecutive hyphens
	if strings.Contains(username, "--") {
		return fmt.Errorf("username '%s' is invalid: cannot contain consecutive hyphens", username)
	}

	return nil
}

// validateGitHubTeamSlug validates a GitHub team slug according to GitHub's rules
func validateGitHubTeamSlug(teamSlug string) error {
	if teamSlug == "" {
		return fmt.Errorf("team slug cannot be empty")
	}

	if len(teamSlug) > 100 {
		return fmt.Errorf("team slug must be 100 characters or less")
	}

	// GitHub team slug validation rules:
	// - May only contain lowercase alphanumeric characters, hyphens, and underscores
	// - Must start with an alphanumeric character
	validTeamSlug := regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	if !validTeamSlug.MatchString(teamSlug) {
		return fmt.Errorf("team slug '%s' is invalid: must contain only lowercase alphanumeric characters, hyphens, and underscores, and start with alphanumeric character", teamSlug)
	}

	return nil
}

// LoadRepositoryConfig loads repository configuration from YAML file
func LoadRepositoryConfig(data []byte) (*RepositoryConfig, error) {
	var config RepositoryConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// LoadRepositoryConfigFromFile loads repository configuration from a file
// This function now detects the format and handles both single and multi-repository configurations
// For backward compatibility, it returns a single RepositoryConfig even for multi-repo files
func LoadRepositoryConfigFromFile(filename string) (*RepositoryConfig, error) {
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
	case FormatSingleRepository:
		return LoadRepositoryConfig(data)
	case FormatMultiRepository:
		// For backward compatibility, if this is a multi-repo config but only has one repository,
		// return that single repository. Otherwise, return an error indicating multi-repo format.
		multiConfig, err := detector.LoadMultiRepo(data)
		if err != nil {
			return nil, fmt.Errorf("failed to load multi-repository config: %w", err)
		}

		if len(multiConfig.Repositories) == 1 {
			// Apply defaults to the single repository if present
			if multiConfig.Defaults != nil {
				merger := NewConfigMerger()
				merged, err := merger.MergeDefaults(multiConfig.Defaults, &multiConfig.Repositories[0])
				if err != nil {
					return nil, fmt.Errorf("failed to merge defaults: %w", err)
				}
				return merged, nil
			}
			return &multiConfig.Repositories[0], nil
		}

		return nil, fmt.Errorf("multi-repository configuration detected with %d repositories. Use LoadMultiRepositoryConfigFromFile or LoadConfigFromFile instead", len(multiConfig.Repositories))
	default:
		return nil, fmt.Errorf("unsupported config format: %s", format)
	}
}

// validateGitHubRepositoryName validates a GitHub repository name according to GitHub's rules
func validateGitHubRepositoryName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("repository name is required")
	case len(name) > 100:
		return fmt.Errorf("repository name must be 100 characters or less")
	case strings.IndexFunc(name, func(c rune) bool { return !isValidRepoNameChar(c) }) >= 0:
		return fmt.Errorf("repository name can only contain alphanumeric characters, periods, hyphens, and underscores")
	case strings.HasPrefix(name, ".") || strings.HasSuffix(name, "."):
		return fmt.Errorf("repository name cannot start or end with a period")
	case strings.HasPrefix(name, "-"):
		return fmt.Errorf("repository name cannot start with a hyphen")
	case strings.HasSuffix(strings.ToLower(name), ".git"):
		return fmt.Errorf("repository name cannot end with .git")
	}
	return nil
}

// validateGitHubTopic validates a GitHub topic according to GitHub's rules.
// Its message is phrased to follow a "topic N" prefix.
func validateGitHubTopic(topic string) error {
	switch {
	case topic == "":
		return fmt.Errorf("cannot be empty")
	case len(topic) > 50:
		return fmt.Errorf("must be 50 characters or less")
	case strings.IndexFunc(topic, func(c rune) bool { return !isValidTopicChar(c) }) >= 0:
		return fmt.Errorf("can only contain lowercase letters, numbers, and hyphens")
	case strings.HasPrefix(topic, "-") || strings.HasSuffix(topic, "-"):
		return fmt.Errorf("cannot start or end with a hyphen")
	}
	return nil
}

// isValidRepoNameChar checks if a character is valid for repository names
func isValidRepoNameChar(char rune) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.'
}

// isValidTopicChar checks if a character is valid for topics
func isValidTopicChar(char rune) bool {
	return (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-'
}
