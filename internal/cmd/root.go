package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "synacklab",
	Short: "A CLI tool for DevOps engineers to manage AWS SSO authentication",
	Long: `Synacklab is a command-line tool designed for DevOps engineers to simplify
AWS SSO authentication and profile management. It helps you authenticate with AWS SSO,
list available profiles, and set default configurations.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(initCmd)
}
