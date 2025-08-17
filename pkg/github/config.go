package github

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
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
}

// Validate validates the repository configuration
func (r *RepositoryConfig) Validate() error {
	var validationErrors ValidationErrors

	if err := r.validateName(); err != nil {
		if valErr, ok := err.(*ValidationError); ok {
			validationErrors = append(validationErrors, *valErr)
		} else {
			validationErrors.Add("name", r.Name, err.Error())
		}
	}

	if err := r.validateDescription(); err != nil {
		validationErrors.Add("description", r.Description, err.Error())
	}

	if err := r.validateTopics(); err != nil {
		validationErrors.Add("topics", fmt.Sprintf("%v", r.Topics), err.Error())
	}

	if err := r.validateBranchRules(); err != nil {
		validationErrors.Add("branch_protection", "", err.Error())
	}

	if err := r.validateCollaborators(); err != nil {
		validationErrors.Add("collaborators", "", err.Error())
	}

	if err := r.validateTeams(); err != nil {
		validationErrors.Add("teams", "", err.Error())
	}

	if err := r.validateWebhooks(); err != nil {
		validationErrors.Add("webhooks", "", err.Error())
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

// validateName validates repository name according to GitHub rules
func (r *RepositoryConfig) validateName() error {
	if r.Name == "" {
		return &ValidationError{
			Field:   "name",
			Value:   r.Name,
			Message: "repository name is required",
		}
	}

	if len(r.Name) > 100 {
		return &ValidationError{
			Field:   "name",
			Value:   r.Name,
			Message: "repository name must be 100 characters or less",
		}
	}

	// GitHub repository name validation
	validName := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	if !validName.MatchString(r.Name) {
		return &ValidationError{
			Field:   "name",
			Value:   r.Name,
			Message: "repository name can only contain alphanumeric characters, periods, hyphens, and underscores",
		}
	}

	// Cannot start or end with period
	if strings.HasPrefix(r.Name, ".") || strings.HasSuffix(r.Name, ".") {
		return &ValidationError{
			Field:   "name",
			Value:   r.Name,
			Message: "repository name cannot start or end with a period",
		}
	}

	return nil
}

// validateDescription validates repository description
func (r *RepositoryConfig) validateDescription() error {
	if len(r.Description) > 350 {
		return fmt.Errorf("repository description must be 350 characters or less")
	}
	return nil
}

// validateTopics validates repository topics
func (r *RepositoryConfig) validateTopics() error {
	if len(r.Topics) > 20 {
		return fmt.Errorf("repository can have at most 20 topics")
	}

	for i, topic := range r.Topics {
		if len(topic) == 0 {
			return fmt.Errorf("topic %d cannot be empty", i+1)
		}
		if len(topic) > 50 {
			return fmt.Errorf("topic %d must be 50 characters or less", i+1)
		}
		// GitHub topic validation
		validTopic := regexp.MustCompile(`^[a-z0-9-]+$`)
		if !validTopic.MatchString(topic) {
			return fmt.Errorf("topic %d can only contain lowercase letters, numbers, and hyphens", i+1)
		}
	}

	return nil
}

// validateBranchRules validates branch protection rules
func (r *RepositoryConfig) validateBranchRules() error {
	for i, rule := range r.BranchRules {
		if rule.Pattern == "" {
			return fmt.Errorf("branch protection rule %d: pattern is required", i+1)
		}
		if rule.RequiredReviews < 0 || rule.RequiredReviews > 6 {
			return fmt.Errorf("branch protection rule %d: required reviews must be between 0 and 6", i+1)
		}
	}
	return nil
}

// validateCollaborators validates collaborator configurations
func (r *RepositoryConfig) validateCollaborators() error {
	for i, collab := range r.Collaborators {
		if collab.Username == "" {
			return fmt.Errorf("collaborator %d: username is required", i+1)
		}
		if err := validateGitHubUsername(collab.Username); err != nil {
			return fmt.Errorf("collaborator %d: %w", i+1, err)
		}
		if !isValidPermission(collab.Permission) {
			return fmt.Errorf("collaborator %d: permission must be one of: read, write, admin", i+1)
		}
	}
	return nil
}

// validateTeams validates team access configurations
func (r *RepositoryConfig) validateTeams() error {
	for i, team := range r.Teams {
		if team.TeamSlug == "" {
			return fmt.Errorf("team %d: team slug is required", i+1)
		}
		if err := validateGitHubTeamSlug(team.TeamSlug); err != nil {
			return fmt.Errorf("team %d: %w", i+1, err)
		}
		if !isValidPermission(team.Permission) {
			return fmt.Errorf("team %d: permission must be one of: read, write, admin", i+1)
		}
	}
	return nil
}

// validateWebhooks validates webhook configurations
func (r *RepositoryConfig) validateWebhooks() error {
	for i, webhook := range r.Webhooks {
		if webhook.URL == "" {
			return fmt.Errorf("webhook %d: URL is required", i+1)
		}
		parsedURL, err := url.Parse(webhook.URL)
		if err != nil {
			return fmt.Errorf("webhook %d: invalid URL format: %w", i+1, err)
		}
		// Require HTTP or HTTPS scheme for webhooks
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return fmt.Errorf("webhook %d: URL must use http or https scheme", i+1)
		}
		if parsedURL.Host == "" {
			return fmt.Errorf("webhook %d: URL must have a valid host", i+1)
		}
		if len(webhook.Events) == 0 {
			return fmt.Errorf("webhook %d: at least one event is required", i+1)
		}
		for j, event := range webhook.Events {
			if !isValidWebhookEvent(event) {
				return fmt.Errorf("webhook %d, event %d: invalid event type '%s'", i+1, j+1, event)
			}
		}
	}
	return nil
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
	if name == "" {
		return fmt.Errorf("repository name cannot be empty")
	}

	// Repository name length validation (GitHub allows 1-100 characters)
	if len(name) > 100 {
		return fmt.Errorf("repository name must be 100 characters or less")
	}

	// Repository name format validation
	// GitHub repository names can contain alphanumeric characters, hyphens, underscores, and periods
	// They cannot start with a period or hyphen
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "-") {
		return fmt.Errorf("repository name cannot start with a period or hyphen")
	}

	// Check for valid characters
	for _, char := range name {
		if !isValidRepoNameChar(char) {
			return fmt.Errorf("repository name can only contain alphanumeric characters, hyphens, underscores, and periods")
		}
	}

	// Repository names cannot end with .git
	if strings.HasSuffix(strings.ToLower(name), ".git") {
		return fmt.Errorf("repository name cannot end with .git")
	}

	return nil
}

// validateGitHubTopic validates a GitHub topic according to GitHub's rules
func validateGitHubTopic(topic string) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}

	// Topic length validation (GitHub allows 1-50 characters)
	if len(topic) > 50 {
		return fmt.Errorf("topic must be 50 characters or less")
	}

	// Topic format validation
	// GitHub topics can only contain lowercase letters, numbers, and hyphens
	// They cannot start or end with hyphens
	if strings.HasPrefix(topic, "-") || strings.HasSuffix(topic, "-") {
		return fmt.Errorf("topic cannot start or end with a hyphen")
	}

	// Check for valid characters (lowercase letters, numbers, and hyphens only)
	for _, char := range topic {
		if !isValidTopicChar(char) {
			return fmt.Errorf("topic can only contain lowercase letters, numbers, and hyphens")
		}
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
