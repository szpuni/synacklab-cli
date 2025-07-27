package cmd

import (
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
	Long: `Commands for managing authentication with various cloud providers.

Available commands:
  sync   - Sync AWS SSO profiles to local configuration
  config - Set default AWS profile from existing profiles`,
}

func init() {
	authCmd.AddCommand(awsConfigCmd)
	authCmd.AddCommand(awsSyncCmd)
}
