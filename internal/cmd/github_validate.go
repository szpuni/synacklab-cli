package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"synacklab/pkg/config"
	"synacklab/pkg/github"
)

var githubValidateCmd = &cobra.Command{
	Use:   "validate <config-file.yaml>",
	Short: "Validate repository configuration file",
	Long: `Validate a repository configuration file for syntax and logical errors.

This command checks the configuration file for:
- YAML syntax errors
- Required fields and valid values
- GitHub-specific validation rules (usernames, team slugs, URLs)
- Existence of referenced users and teams (requires authentication)

The validation includes both offline checks (syntax, format, required fields)
and online checks (user/team existence, permissions) when authentication is available.

Examples:
  # Validate configuration file
  synacklab github validate my-repo.yaml

  # Validate with specific owner context
  synacklab github validate my-repo.yaml --owner myorg`,
	Args: cobra.ExactArgs(1),
	RunE: runGitHubValidate,
}

func init() {
	githubValidateCmd.Flags().StringVar(&githubOwner, "owner", "", "Repository owner (organization or user) for team validation")
	githubCmd.AddCommand(githubValidateCmd)
}

func runGitHubValidate(_ *cobra.Command, args []string) error {
	configFile := args[0]

	fmt.Printf("🔍 Validating configuration file: %s\n", configFile)

	// Load repository configuration from file
	repoConfig, err := github.LoadRepositoryConfigFromFile(configFile)
	if err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	fmt.Printf("✓ YAML syntax and basic validation passed\n")

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
	authManager := github.NewAuthManager()
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
