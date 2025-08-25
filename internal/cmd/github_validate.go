package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"synacklab/pkg/config"
	"synacklab/pkg/github"
)

var githubValidateCmd = &cobra.Command{
	Use:   "validate <config-file.yaml>",
	Short: "Validate repository configuration file",
	Long: `Validate a repository configuration file for syntax and logical errors.

This command supports both single repository and multi-repository configuration formats
and performs comprehensive validation to catch errors before applying configurations.

VALIDATION CHECKS:

Offline Validation (always performed):
• YAML syntax errors and structure validation
• Required fields and valid values
• Configuration format detection and compatibility
• Duplicate repository names (multi-repository format)
• Configuration merging validation (defaults with repository overrides)

Online Validation (when authenticated):
• GitHub user existence validation
• Team existence validation (requires organization context)
• Repository permissions validation
• GitHub API connectivity and rate limit checks

MULTI-REPOSITORY FEATURES:

• Batch Validation: Validate all repositories in a single command
• Selective Validation: Use --repos flag to validate specific repositories only
• Comprehensive Reporting: Detailed validation results for each repository
• Merge Validation: Ensures global defaults merge correctly with repository settings
• Duplicate Detection: Identifies duplicate repository names within configuration

Examples:
  # Single repository validation
  synacklab github validate my-repo.yaml
  synacklab github validate my-repo.yaml --owner myorg

  # Multi-repository validation
  synacklab github validate multi-repos.yaml --owner myorg
  synacklab github validate multi-repos.yaml

  # Selective multi-repository validation
  synacklab github validate multi-repos.yaml --repos repo1,repo2
  synacklab github validate multi-repos.yaml --repos "user-service,payment-service" --owner myorg

  # Validation without authentication (offline only)
  synacklab github validate multi-repos.yaml
  # Note: Will skip user/team existence checks but validate syntax and structure

Configuration Examples:
  See examples/ directory for sample configurations and migration guide:
  • examples/github-simple-repo.yaml - Single repository format
  • examples/github-multi-repo-basic.yaml - Basic multi-repository format
  • examples/migration-single-to-multi.md - Migration guide`,
	Args: cobra.ExactArgs(1),
	RunE: runGitHubValidate,
}

func init() {
	githubValidateCmd.Flags().StringVar(&githubOwner, "owner", "", "Repository owner (organization or user) - required for team validation and permissions checks")
	githubValidateCmd.Flags().StringSliceVar(&githubRepos, "repos", nil, "Comma-separated list of repository names to validate from multi-repository configuration (e.g., --repos repo1,repo2)")
	githubCmd.AddCommand(githubValidateCmd)
}

func runGitHubValidate(_ *cobra.Command, args []string) error {
	configFile := args[0]

	fmt.Printf("🔍 Validating configuration file: %s\n", configFile)

	// Parse repository filter if provided
	var repoFilter []string
	if len(githubRepos) > 0 {
		// Remove empty strings from the filter
		for _, repo := range githubRepos {
			if strings.TrimSpace(repo) != "" {
				repoFilter = append(repoFilter, strings.TrimSpace(repo))
			}
		}
	}

	// Load configuration and detect format
	configData, format, err := github.LoadConfigFromFile(configFile)
	if err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	fmt.Printf("✓ YAML syntax and basic validation passed\n")
	fmt.Printf("📋 Configuration format: %s\n", format)

	// Handle different configuration formats
	switch format {
	case github.FormatSingleRepository:
		return runSingleRepositoryValidation(configData.(*github.RepositoryConfig), configFile)
	case github.FormatMultiRepository:
		return runMultiRepositoryValidation(configData.(*github.MultiRepositoryConfig), configFile, repoFilter)
	default:
		return fmt.Errorf("unsupported configuration format: %s", format)
	}
}

func runSingleRepositoryValidation(repoConfig *github.RepositoryConfig, configFile string) error {
	fmt.Printf("📦 Validating single repository: %s\n", repoConfig.Name)

	// Load synacklab configuration for GitHub API validation
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("⚠️  Could not load synacklab config: %v\n", err)
		fmt.Printf("   Skipping GitHub API validation (user/team existence checks)\n")
		fmt.Printf("\n✅ Configuration file is valid (offline validation only)\n")
		return nil
	}

	// Determine repository owner for team validation
	repoOwner := githubOwner
	if repoOwner == "" {
		if cfg.GitHub.Organization != "" {
			repoOwner = cfg.GitHub.Organization
		} else {
			fmt.Printf("⚠️  Repository owner not specified and no default organization configured\n")
			fmt.Printf("   Use --owner flag or set github.organization in config for team validation\n")
			fmt.Printf("   Skipping team existence validation\n")
		}
	}

	// Try to authenticate for GitHub API validation
	authManager := github.NewManager()
	tokenInfo, err := authManager.AuthenticateFromConfig(context.Background(), cfg)
	if err != nil {
		fmt.Printf("⚠️  GitHub authentication failed: %v\n", err)
		fmt.Printf("   Skipping GitHub API validation (user/team existence checks)\n")
		fmt.Printf("   To enable full validation, ensure GitHub authentication is configured:\n")
		fmt.Printf("%s\n", github.GetAuthInstructions())
		fmt.Printf("\n✅ Configuration file is valid (offline validation only)\n")
		return nil
	}

	fmt.Printf("✓ Authenticated as %s\n", tokenInfo.User)

	// Get GitHub token for API validation
	token, err := authManager.GetToken(cfg)
	if err != nil {
		fmt.Printf("⚠️  Could not get GitHub token: %v\n", err)
		fmt.Printf("   Skipping GitHub API validation\n")
		fmt.Printf("\n✅ Configuration file is valid (offline validation only)\n")
		return nil
	}

	// Create validator and perform GitHub API validation
	validator := github.NewValidator(token)

	fmt.Printf("🔍 Performing GitHub API validation...\n")

	// Validate collaborators exist
	if len(repoConfig.Collaborators) > 0 {
		fmt.Printf("   Checking collaborator usernames...\n")
		for _, collab := range repoConfig.Collaborators {
			fmt.Printf("     - %s", collab.Username)
		}
		fmt.Printf("\n")
	}

	// Validate teams exist (only if we have an owner)
	if len(repoConfig.Teams) > 0 && repoOwner != "" {
		fmt.Printf("   Checking team slugs in organization %s...\n", repoOwner)
		for _, team := range repoConfig.Teams {
			fmt.Printf("     - %s", team.TeamSlug)
		}
		fmt.Printf("\n")
	}

	// Perform the actual validation
	if err := validator.ValidateConfig(repoConfig, repoOwner); err != nil {
		return fmt.Errorf("GitHub API validation failed: %w", err)
	}

	fmt.Printf("✓ All users and teams exist\n")

	// Validate permissions for the repository if it exists
	if repoOwner != "" {
		fmt.Printf("🔍 Checking repository permissions...\n")
		if err := validator.ValidatePermissions(repoOwner, repoConfig.Name); err != nil {
			// Don't fail validation for permission issues, just warn
			fmt.Printf("⚠️  Permission check: %v\n", err)
			fmt.Printf("   This may prevent applying the configuration\n")
		} else {
			fmt.Printf("✓ Sufficient permissions for repository operations\n")
		}
	}

	fmt.Printf("\n✅ Configuration file is valid and ready to apply\n")

	// Provide helpful next steps
	fmt.Printf("\n💡 Next steps:\n")
	fmt.Printf("   • Apply configuration: synacklab github apply %s", configFile)
	if githubOwner != "" {
		fmt.Printf(" --owner %s", githubOwner)
	}
	fmt.Printf("\n")
	fmt.Printf("   • Preview changes: synacklab github apply %s --dry-run", configFile)
	if githubOwner != "" {
		fmt.Printf(" --owner %s", githubOwner)
	}
	fmt.Printf("\n")

	return nil
}

func runMultiRepositoryValidation(multiConfig *github.MultiRepositoryConfig, configFile string, repoFilter []string) error {
	totalRepos := len(multiConfig.Repositories)

	// Validate repository filter early
	if len(repoFilter) > 0 {
		// Create a set of available repository names
		availableRepos := make(map[string]bool)
		for _, repo := range multiConfig.Repositories {
			availableRepos[repo.Name] = true
		}

		// Check that all filtered repositories exist
		var invalidRepos []string
		for _, repoName := range repoFilter {
			if !availableRepos[repoName] {
				invalidRepos = append(invalidRepos, repoName)
			}
		}

		if len(invalidRepos) > 0 {
			return fmt.Errorf("repositories not found in configuration: %s", strings.Join(invalidRepos, ", "))
		}

		fmt.Printf("📦 Validating %d selected repositories from %d total repositories\n", len(repoFilter), totalRepos)
		fmt.Printf("🎯 Selected repositories: %s\n", strings.Join(repoFilter, ", "))
	} else {
		fmt.Printf("📦 Validating %d repositories\n", totalRepos)
	}

	// Load synacklab configuration for GitHub API validation
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("⚠️  Could not load synacklab config: %v\n", err)
		fmt.Printf("   Skipping GitHub API validation (user/team existence checks)\n")
		fmt.Printf("\n✅ Configuration file is valid (offline validation only)\n")
		return nil
	}

	// Determine repository owner for team validation
	repoOwner := githubOwner
	if repoOwner == "" {
		if cfg.GitHub.Organization != "" {
			repoOwner = cfg.GitHub.Organization
		} else {
			fmt.Printf("⚠️  Repository owner not specified and no default organization configured\n")
			fmt.Printf("   Use --owner flag or set github.organization in config for team validation\n")
			fmt.Printf("   Skipping team existence validation\n")
		}
	}

	// Try to authenticate for GitHub API validation
	authManager := github.NewManager()
	tokenInfo, err := authManager.AuthenticateFromConfig(context.Background(), cfg)
	if err != nil {
		fmt.Printf("⚠️  GitHub authentication failed: %v\n", err)
		fmt.Printf("   Skipping GitHub API validation (user/team existence checks)\n")
		fmt.Printf("   To enable full validation, ensure GitHub authentication is configured:\n")
		fmt.Printf("%s\n", github.GetAuthInstructions())
		fmt.Printf("\n✅ Configuration file is valid (offline validation only)\n")
		return nil
	}

	fmt.Printf("✓ Authenticated as %s\n", tokenInfo.User)

	// Get GitHub token for API validation
	token, err := authManager.GetToken(cfg)
	if err != nil {
		fmt.Printf("⚠️  Could not get GitHub token: %v\n", err)
		fmt.Printf("   Skipping GitHub API validation\n")
		fmt.Printf("\n✅ Configuration file is valid (offline validation only)\n")
		return nil
	}

	// Create multi-repository reconciler for validation
	client := github.NewClient(token)
	multiReconciler := github.NewMultiReconciler(client, repoOwner)

	fmt.Printf("🔍 Performing comprehensive multi-repository validation...\n")

	// Validate all repositories
	result, err := multiReconciler.ValidateAll(multiConfig, repoFilter)
	if err != nil {
		return fmt.Errorf("multi-repository validation failed: %w", err)
	}

	// Display validation results
	displayMultiRepositoryValidationResults(result)

	// Check if there were any validation failures
	if result.Summary.InvalidCount > 0 {
		return fmt.Errorf("validation failed for %d repositories", result.Summary.InvalidCount)
	}

	fmt.Printf("\n✅ All repositories are valid and ready to apply\n")

	// Provide helpful next steps
	fmt.Printf("\n💡 Next steps:\n")
	fmt.Printf("   • Apply configuration: synacklab github apply %s", configFile)
	if githubOwner != "" {
		fmt.Printf(" --owner %s", githubOwner)
	}
	if len(repoFilter) > 0 {
		fmt.Printf(" --repos %s", strings.Join(repoFilter, ","))
	}
	fmt.Printf("\n")
	fmt.Printf("   • Preview changes: synacklab github apply %s --dry-run", configFile)
	if githubOwner != "" {
		fmt.Printf(" --owner %s", githubOwner)
	}
	if len(repoFilter) > 0 {
		fmt.Printf(" --repos %s", strings.Join(repoFilter, ","))
	}
	fmt.Printf("\n")

	return nil
}

func displayMultiRepositoryValidationResults(result *github.MultiRepoValidationResult) {
	// Display summary
	fmt.Printf("\n📊 Validation Summary:\n")
	fmt.Printf("   Total repositories: %d\n", result.Summary.TotalRepositories)
	fmt.Printf("   ✅ Valid: %d\n", result.Summary.ValidCount)
	fmt.Printf("   ❌ Invalid: %d\n", result.Summary.InvalidCount)
	if result.Summary.WarningCount > 0 {
		fmt.Printf("   ⚠️  Warnings: %d\n", result.Summary.WarningCount)
	}

	// Display valid repositories
	if len(result.Valid) > 0 {
		fmt.Printf("\n✅ Valid repositories:\n")
		for _, repo := range result.Valid {
			fmt.Printf("   • %s", repo)
			if details, exists := result.Details[repo]; exists && len(details.Warnings) > 0 {
				fmt.Printf(" (%d warnings)", len(details.Warnings))
			}
			fmt.Printf("\n")
		}
	}

	// Display invalid repositories with detailed errors
	if len(result.Invalid) > 0 {
		fmt.Printf("\n❌ Invalid repositories:\n")
		for repo, err := range result.Invalid {
			fmt.Printf("   • %s: %v\n", repo, err)

			// Display detailed validation errors if available
			if details, exists := result.Details[repo]; exists {
				if len(details.Errors) > 0 {
					fmt.Printf("     Errors:\n")
					for _, validationErr := range details.Errors {
						if validationErr.Value != "" {
							fmt.Printf("       - %s (%s): %s\n", validationErr.Field, validationErr.Value, validationErr.Message)
						} else {
							fmt.Printf("       - %s: %s\n", validationErr.Field, validationErr.Message)
						}
					}
				}
			}
		}
	}

	// Display warnings for all repositories
	hasWarnings := false
	for repo, details := range result.Details {
		if len(details.Warnings) > 0 {
			if !hasWarnings {
				fmt.Printf("\n⚠️  Validation warnings:\n")
				hasWarnings = true
			}
			fmt.Printf("   • %s:\n", repo)
			for _, warning := range details.Warnings {
				if warning.Value != "" {
					fmt.Printf("     - %s (%s): %s\n", warning.Field, warning.Value, warning.Message)
				} else {
					fmt.Printf("     - %s: %s\n", warning.Field, warning.Message)
				}
			}
		}
	}
}
