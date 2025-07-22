package cmd

import (
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
	Long:  "Commands for managing authentication with various cloud providers",
}

func init() {
	authCmd.AddCommand(awsConfigCmd)
}
