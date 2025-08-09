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
  
Use YAML configuration files to define repository settings, branch protection rules,
collaborators, teams, and webhooks. The system will reconcile the actual repository
state with your desired configuration.`,
}

func init() {
	// Subcommands are added in their respective files
}
