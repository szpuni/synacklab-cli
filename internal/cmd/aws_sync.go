package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/spf13/cobra"
	"gopkg.in/ini.v1"

	"synacklab/pkg/config"
)

var (
	resetProfiles bool
)

var awsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync AWS SSO profiles to local configuration",
	Long: `Authenticate with AWS SSO and sync all available profiles to ~/.aws/config.
By default, this command will add new profiles and update existing ones with remote data.
Use --reset to replace all profiles with only those available in AWS SSO.`,
	RunE: runAWSSync,
}

func init() {
	awsSyncCmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to configuration file")
	awsSyncCmd.Flags().BoolVar(&resetProfiles, "reset", false, "Replace all profiles with AWS SSO profiles only")
}

// AWSProfile represents an AWS profile configuration
type AWSProfile struct {
	Name      string
	AccountID string
	RoleName  string
	Region    string
}

// SSOSession represents AWS SSO session information
type SSOSession struct {
	AccessToken string
	StartURL    string
	Region      string
}

func runAWSSync(_ *cobra.Command, _ []string) error {
	fmt.Println("🔄 Starting AWS SSO profile synchronization...")

	// Load configuration
	appConfig, err := loadAppConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := appConfig.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Authenticate with AWS SSO
	ssoSession, err := authenticateSSO(appConfig)
	if err != nil {
		return fmt.Errorf("failed to authenticate with AWS SSO: %w", err)
	}

	// Fetch all profiles from AWS SSO
	profiles, err := fetchSSOProfiles(ssoSession)
	if err != nil {
		return fmt.Errorf("failed to fetch SSO profiles: %w", err)
	}

	if len(profiles) == 0 {
		fmt.Println("⚠️  No profiles found in AWS SSO")
		return nil
	}

	fmt.Printf("📋 Found %d profiles in AWS SSO\n", len(profiles))

	// Update AWS config file
	err = updateAWSConfigWithProfiles(profiles, ssoSession, resetProfiles)
	if err != nil {
		return fmt.Errorf("failed to update AWS config: %w", err)
	}

	if resetProfiles {
		fmt.Printf("✅ Successfully replaced AWS config with %d SSO profiles\n", len(profiles))
	} else {
		fmt.Printf("✅ Successfully synchronized %d SSO profiles to AWS config\n", len(profiles))
	}

	return nil
}

func loadAppConfig() (*config.Config, error) {
	var appConfig *config.Config
	var err error

	if configFile != "" {
		appConfig, err = config.LoadConfigFromPath(configFile)
	} else {
		appConfig, err = config.LoadConfig()
	}

	if err != nil {
		return nil, err
	}

	// If config is empty, prompt user for basic settings
	if appConfig.AWS.SSO.StartURL == "" || appConfig.AWS.SSO.Region == "" {
		fmt.Println("📝 AWS SSO configuration not found. Please provide the required information:")

		if appConfig.AWS.SSO.StartURL == "" {
			fmt.Print("Enter your AWS SSO start URL: ")
			var startURL string
			if _, err := fmt.Scanln(&startURL); err != nil {
				return nil, fmt.Errorf("failed to read start URL: %w", err)
			}
			appConfig.AWS.SSO.StartURL = startURL
		}

		if appConfig.AWS.SSO.Region == "" {
			fmt.Print("Enter your SSO region (default: us-east-1): ")
			var region string
			fmt.Scanln(&region) // Ignore error for optional input
			if region == "" {
				region = "us-east-1"
			}
			appConfig.AWS.SSO.Region = region
		}

		// Save the configuration
		if err := appConfig.SaveConfig(); err != nil {
			fmt.Printf("⚠️  Warning: Failed to save configuration: %v\n", err)
		} else {
			fmt.Println("💾 Configuration saved")
		}
	}

	return appConfig, nil
}

func authenticateSSO(appConfig *config.Config) (*SSOSession, error) {
	fmt.Printf("🔐 Authenticating with AWS SSO: %s\n", appConfig.AWS.SSO.StartURL)

	// Initialize AWS config
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create SSO OIDC client
	ssooidcClient := ssooidc.NewFromConfig(cfg, func(o *ssooidc.Options) {
		o.Region = appConfig.AWS.SSO.Region
	})

	// Register client
	registerResp, err := ssooidcClient.RegisterClient(context.TODO(), &ssooidc.RegisterClientInput{
		ClientName: aws.String("synacklab-cli"),
		ClientType: aws.String("public"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register client: %w", err)
	}

	// Start device authorization
	deviceAuthResp, err := ssooidcClient.StartDeviceAuthorization(context.TODO(), &ssooidc.StartDeviceAuthorizationInput{
		ClientId:     registerResp.ClientId,
		ClientSecret: registerResp.ClientSecret,
		StartUrl:     aws.String(appConfig.AWS.SSO.StartURL),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start device authorization: %w", err)
	}

	fmt.Printf("\n🌐 Please visit: %s\n", *deviceAuthResp.VerificationUriComplete)
	fmt.Printf("📋 And enter code: %s\n", *deviceAuthResp.UserCode)
	fmt.Println("\nPress Enter after completing the authorization...")
	fmt.Scanln() // Wait for user input

	// Create token
	tokenResp, err := ssooidcClient.CreateToken(context.TODO(), &ssooidc.CreateTokenInput{
		ClientId:     registerResp.ClientId,
		ClientSecret: registerResp.ClientSecret,
		DeviceCode:   deviceAuthResp.DeviceCode,
		GrantType:    aws.String("urn:ietf:params:oauth:grant-type:device_code"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	return &SSOSession{
		AccessToken: *tokenResp.AccessToken,
		StartURL:    appConfig.AWS.SSO.StartURL,
		Region:      appConfig.AWS.SSO.Region,
	}, nil
}

func fetchSSOProfiles(session *SSOSession) ([]AWSProfile, error) {
	// Initialize AWS config
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create SSO client
	ssoClient := sso.NewFromConfig(cfg, func(o *sso.Options) {
		o.Region = session.Region
	})

	// List accounts
	accountsResp, err := ssoClient.ListAccounts(context.TODO(), &sso.ListAccountsInput{
		AccessToken: aws.String(session.AccessToken),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}

	var profiles []AWSProfile

	for _, account := range accountsResp.AccountList {
		// List roles for each account
		rolesResp, err := ssoClient.ListAccountRoles(context.TODO(), &sso.ListAccountRolesInput{
			AccessToken: aws.String(session.AccessToken),
			AccountId:   account.AccountId,
		})
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to list roles for account %s: %v\n", *account.AccountId, err)
			continue
		}

		for _, role := range rolesResp.RoleList {
			// Create profile name: account-name-role-name
			profileName := fmt.Sprintf("%s-%s",
				sanitizeProfileName(*account.AccountName),
				sanitizeProfileName(*role.RoleName))

			profile := AWSProfile{
				Name:      profileName,
				AccountID: *account.AccountId,
				RoleName:  *role.RoleName,
				Region:    session.Region,
			}
			profiles = append(profiles, profile)
		}
	}

	// Sort profiles by name for consistent output
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	return profiles, nil
}

func sanitizeProfileName(name string) string {
	// Replace spaces and underscores with hyphens
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ToLower(name)

	// Remove any characters that aren't alphanumeric or hyphens
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}

	// Remove consecutive hyphens
	sanitized := result.String()
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}

	return sanitized
}

func updateAWSConfigWithProfiles(profiles []AWSProfile, session *SSOSession, reset bool) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	awsDir := filepath.Join(homeDir, ".aws")
	configPath := filepath.Join(awsDir, "config")

	// Create .aws directory if it doesn't exist
	if err := os.MkdirAll(awsDir, 0755); err != nil {
		return fmt.Errorf("failed to create .aws directory: %w", err)
	}

	var cfg *ini.File

	if reset {
		// Create new config file
		cfg = ini.Empty()
		fmt.Println("🔄 Resetting AWS config file")
	} else {
		// Load existing config or create new one
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			cfg = ini.Empty()
		} else {
			cfg, err = ini.Load(configPath)
			if err != nil {
				return fmt.Errorf("failed to load existing AWS config: %w", err)
			}
		}
	}

	// Track which profiles we're adding/updating
	addedCount := 0
	updatedCount := 0

	for _, profile := range profiles {
		sectionName := fmt.Sprintf("profile %s", profile.Name)

		// Check if profile already exists
		var section *ini.Section
		if cfg.HasSection(sectionName) {
			section = cfg.Section(sectionName)
			updatedCount++
		} else {
			section, err = cfg.NewSection(sectionName)
			if err != nil {
				return fmt.Errorf("failed to create section %s: %w", sectionName, err)
			}
			addedCount++
		}

		// Set profile configuration
		section.Key("sso_start_url").SetValue(session.StartURL)
		section.Key("sso_region").SetValue(session.Region)
		section.Key("sso_account_id").SetValue(profile.AccountID)
		section.Key("sso_role_name").SetValue(profile.RoleName)
		section.Key("region").SetValue(profile.Region)
		section.Key("output").SetValue("json")
	}

	// Save the configuration file
	if err := cfg.SaveTo(configPath); err != nil {
		return fmt.Errorf("failed to save AWS config: %w", err)
	}

	if !reset {
		fmt.Printf("📊 Added %d new profiles, updated %d existing profiles\n", addedCount, updatedCount)
	}

	return nil
}
