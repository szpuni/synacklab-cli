package cmd

import (
	"github.com/spf13/cobra"
)

var githubCmd = &cobra.Command{
	Use:   "github",
	Short: "GitHub repository management commands",
	Long: `Commands for managing GitHub repositories using declarative configuration.

Available commands:
  apply    - Apply repository configuration to GitHub
  validate - Validate repository configuration file
  
Supports both single repository and multi-repository configuration formats:

Single Repository Format:
  Define one repository per YAML file with direct configuration fields.
  
Multi-Repository Format:
  Define multiple repositories in a single YAML file with optional global defaults.
  Use the --repos flag to selectively operate on specific repositories.

Use YAML configuration files to define repository settings, branch protection rules,
collaborators, teams, and webhooks. The system will reconcile the actual repository
state with your desired configuration.

Examples:
  # Single repository operations
  synacklab github apply single-repo.yaml
  synacklab github validate single-repo.yaml
  
  # Multi-repository operations
  synacklab github apply multi-repos.yaml
  synacklab github apply multi-repos.yaml --repos repo1,repo2
  synacklab github validate multi-repos.yaml --repos repo1,repo2`,
}

func init() {
	// Subcommands are added in their respective files
}
