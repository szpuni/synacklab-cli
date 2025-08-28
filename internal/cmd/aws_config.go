package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"synacklab/internal/auth"
	"synacklab/pkg/config"
	"synacklab/pkg/fuzzy"

	"github.com/spf13/cobra"
	"gopkg.in/ini.v1"
)

var (
	configFile  string
	interactive bool
	noAuth      bool
)

var awsCtxCmd = &cobra.Command{
	Use:   "aws-ctx",
	Short: "Switch AWS SSO context (profile)",
	Long: `Switch between AWS SSO profiles with interactive selection.
This command allows you to select and set a default AWS profile from your existing SSO profiles.
If you are not authenticated, it will automatically prompt you to authenticate first unless --no-auth is specified.

The command provides an interactive fuzzy finder interface for easy profile selection.

Flags:
  --no-auth    Skip automatic authentication and allow profile switching without AWS SSO authentication`,
	RunE: runAWSCtx,
}

func init() {
	awsCtxCmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to configuration file")
	awsCtxCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Force interactive mode even with config file")
	awsCtxCmd.Flags().BoolVar(&noAuth, "no-auth", false, "Skip automatic authentication and allow profile switching without AWS SSO authentication")
}

func runAWSCtx(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	fmt.Println("🔄 Switching AWS SSO context...")

	// Initialize authentication manager
	authManager, err := auth.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize authentication manager: %w", err)
	}

	// Check authentication status
	isAuthenticated, err := authManager.IsAuthenticated(ctx)
	if err != nil {
		// Handle authentication check errors with user-friendly messages
		var authErr *auth.Error
		if errors.As(err, &authErr) {
			fmt.Printf("❌ %s%s\n", authErr.Message, authErr.GetTroubleshootingMessage())
			return fmt.Errorf("authentication check failed")
		}
		return fmt.Errorf("failed to check authentication status: %w", err)
	}

	// Handle authentication based on --no-auth flag
	if !isAuthenticated {
		if noAuth {
			fmt.Println("⚠️  You are not authenticated to AWS SSO")
			fmt.Println("🔄 Proceeding with profile switching without authentication (--no-auth flag specified)")
		} else {
			fmt.Println("🔐 You are not authenticated to AWS SSO")
			fmt.Println("🚀 Starting automatic authentication...")

			// Load configuration for authentication
			appConfig, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Perform authentication with enhanced error handling
			_, err = authManager.Authenticate(ctx, appConfig)
			if err != nil {
				// Handle structured authentication errors
				var authErr *auth.Error
				if errors.As(err, &authErr) {
					fmt.Printf("❌ %s%s\n", authErr.Message, authErr.GetTroubleshootingMessage())
					return fmt.Errorf("authentication failed")
				}
				return fmt.Errorf("authentication failed: %w", err)
			}

			fmt.Println("✅ Authentication successful!")
		}
	}

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

	// Find all profile sections and build options for fuzzy finder
	var options []fuzzy.Option
	var maxProfileLen, maxAccountLen, maxRoleLen int

	// First pass: collect all profiles and calculate max lengths for alignment
	type profileInfo struct {
		name      string
		accountID string
		roleName  string
		region    string
		startURL  string
	}
	var profiles []profileInfo

	for _, section := range cfg.Sections() {
		if section.Name() != "DEFAULT" && section.Name() != "default" {
			// Remove "profile " prefix if present
			profileName := section.Name()
			if len(profileName) > 8 && profileName[:8] == "profile " {
				profileName = profileName[8:]
			}

			// Get profile metadata
			accountID := section.Key("sso_account_id").String()
			roleName := section.Key("sso_role_name").String()
			region := section.Key("region").String()
			startURL := section.Key("sso_start_url").String()

			profiles = append(profiles, profileInfo{
				name:      profileName,
				accountID: accountID,
				roleName:  roleName,
				region:    region,
				startURL:  startURL,
			})

			// Track max lengths for alignment
			if len(profileName) > maxProfileLen {
				maxProfileLen = len(profileName)
			}
			if len(accountID) > maxAccountLen {
				maxAccountLen = len(accountID)
			}
			if len(roleName) > maxRoleLen {
				maxRoleLen = len(roleName)
			}
		}
	}

	// Second pass: build formatted options with proper alignment
	for _, profile := range profiles {
		// Build aligned description with consistent spacing
		description := fmt.Sprintf("Account: %-*s | Role: %-*s | Region: %s",
			maxAccountLen, profile.accountID,
			maxRoleLen, profile.roleName,
			profile.region)

		// Add metadata for consistent display
		metadata := map[string]string{
			"account_id": profile.accountID,
			"role_name":  profile.roleName,
			"region":     profile.region,
			"start_url":  profile.startURL,
		}

		options = append(options, fuzzy.Option{
			Value:       profile.name,
			Description: description,
			Metadata:    metadata,
		})
	}

	if len(options) == 0 {
		fmt.Println("❌ No profiles found in AWS config")
		fmt.Println("💡 Run 'synacklab auth sync' first to create profiles from AWS SSO")
		return nil
	}

	// Use fzf-based fuzzy finder for profile selection
	finder := fuzzy.NewFzf("🔍 Select AWS profile to set as default:")
	if err := finder.SetOptions(options); err != nil {
		return fmt.Errorf("failed to set finder options: %w", err)
	}

	selectedProfile, err := finder.Select()
	if err != nil {
		return fmt.Errorf("profile selection failed: %w", err)
	}

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

	// Check if section exists
	if !cfg.HasSection(profileSectionName) {
		return fmt.Errorf("profile section not found: %s", profileSectionName)
	}

	profileSection := cfg.Section(profileSectionName)

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
