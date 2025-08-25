package cmd

import (
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
	Long: `Commands for managing authentication with various cloud providers.

Use the subcommands to authenticate with AWS SSO and manage your authentication contexts.`,
}

func init() {
	authCmd.AddCommand(awsLoginCmd)
	authCmd.AddCommand(awsCtxCmd)
	authCmd.AddCommand(awsSyncCmd)
	authCmd.AddCommand(eksConfigCmd)
	authCmd.AddCommand(eksCtxCmd)
}
