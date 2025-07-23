package cmd

import (
	"fmt"
	"os"

	"synacklab/pkg/config"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize synacklab configuration",
	Long:  "Create a default configuration file for synacklab",
	RunE:  runInit,
}

func runInit(_ *cobra.Command, _ []string) error {
	configPath, err := config.GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("⚠️  Configuration file already exists at: %s\n", configPath)
		fmt.Print("Do you want to overwrite it? (y/N): ")
		var response string
		_, _ = fmt.Scanln(&response) // Ignore error for user input
		if response != "y" && response != "Y" {
			fmt.Println("Configuration initialization cancelled.")
			return nil
		}
	}

	// Create default configuration
	defaultConfig := &config.Config{
		AWS: config.AWSConfig{
			SSO: config.SSOConfig{
				StartURL: "https://your-company.awsapps.com/start",
				Region:   "us-east-1",
			},
		},
	}

	// Save configuration
	if err := defaultConfig.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("✅ Configuration file created at: %s\n", configPath)
	fmt.Println("📝 Please edit the file to customize your AWS SSO settings.")

	return nil
}
