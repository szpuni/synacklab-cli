package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/ini.v1"
)

var (
	configFile  string
	interactive bool
)

var awsConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure AWS SSO default profile",
	Long: `Set a default AWS profile from existing SSO profiles.
This command lists available profiles from ~/.aws/config and allows you to select one as the default.
Run 'synacklab auth sync' first to ensure your profiles are up to date.`,
	RunE: runAWSConfig,
}

func init() {
	awsConfigCmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to configuration file")
	awsConfigCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Force interactive mode even with config file")
}

func runAWSConfig(_ *cobra.Command, _ []string) error {
	fmt.Println("⚙️  Configuring AWS default profile...")

	// Check if AWS config exists
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".aws", "config")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("❌ No AWS config file found at ~/.aws/config")
		fmt.Println("💡 Run 'synacklab auth sync' first to create profiles from AWS SSO")
		return nil
	}

	// Load existing AWS config
	cfg, err := ini.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Find all profile sections
	var profiles []string
	for _, section := range cfg.Sections() {
		if section.Name() != "DEFAULT" && section.Name() != "default" {
			// Remove "profile " prefix if present
			profileName := section.Name()
			if len(profileName) > 8 && profileName[:8] == "profile " {
				profileName = profileName[8:]
			}
			profiles = append(profiles, profileName)
		}
	}

	if len(profiles) == 0 {
		fmt.Println("❌ No profiles found in AWS config")
		fmt.Println("💡 Run 'synacklab auth sync' first to create profiles from AWS SSO")
		return nil
	}

	// Display available profiles
	fmt.Println("\n📋 Available AWS profiles:")
	for i, profile := range profiles {
		sectionName := fmt.Sprintf("profile %s", profile)
		section := cfg.Section(sectionName)

		accountID := section.Key("sso_account_id").String()
		roleName := section.Key("sso_role_name").String()

		fmt.Printf("%d. %s", i+1, profile)
		if accountID != "" && roleName != "" {
			fmt.Printf(" (Account: %s, Role: %s)", accountID, roleName)
		}
		fmt.Println()
	}

	// Let user choose profile
	fmt.Print("\nSelect profile number to set as default: ")
	var choice int
	if _, err := fmt.Scanln(&choice); err != nil {
		return fmt.Errorf("failed to read selection: %w", err)
	}

	if choice < 1 || choice > len(profiles) {
		return fmt.Errorf("invalid selection")
	}

	selectedProfile := profiles[choice-1]

	// Copy selected profile configuration to default section
	err = setDefaultProfile(cfg, selectedProfile, configPath)
	if err != nil {
		return fmt.Errorf("failed to set default profile: %w", err)
	}

	fmt.Printf("✅ Successfully set '%s' as the default AWS profile\n", selectedProfile)
	return nil
}

func setDefaultProfile(cfg *ini.File, profileName, configPath string) error {
	// Get the selected profile section
	profileSectionName := fmt.Sprintf("profile %s", profileName)
	profileSection := cfg.Section(profileSectionName)

	if profileSection == nil {
		return fmt.Errorf("profile section not found: %s", profileSectionName)
	}

	// Create or get default section
	var defaultSection *ini.Section
	if cfg.HasSection("default") {
		defaultSection = cfg.Section("default")
	} else {
		var err error
		defaultSection, err = cfg.NewSection("default")
		if err != nil {
			return fmt.Errorf("failed to create default section: %w", err)
		}
	}

	// Copy all keys from profile to default
	for _, key := range profileSection.Keys() {
		defaultSection.Key(key.Name()).SetValue(key.Value())
	}

	// Save the configuration
	return cfg.SaveTo(configPath)
}
