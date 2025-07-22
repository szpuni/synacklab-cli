package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/spf13/cobra"
	"gopkg.in/ini.v1"

	"synacklab/pkg/config"
)

var (
	configFile  string
	interactive bool
)

var awsConfigCmd = &cobra.Command{
	Use:   "aws-config",
	Short: "Configure AWS SSO authentication",
	Long:  "Authenticate with AWS SSO and configure default profile",
	RunE:  runAWSConfig,
}

func init() {
	awsConfigCmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to configuration file")
	awsConfigCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Force interactive mode even with config file")
}

type AWSProfile struct {
	Name      string
	AccountID string
	RoleName  string
	Region    string
}

func runAWSConfig(cmd *cobra.Command, args []string) error {
	fmt.Println("🔐 Starting AWS SSO authentication...")

	// Load configuration
	var appConfig *config.Config
	var err error

	if configFile != "" {
		appConfig, err = config.LoadConfigFromPath(configFile)
		if err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
	} else {
		appConfig, err = config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	// Initialize AWS config
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	var ssoStartURL, ssoRegion string

	// Use config file values or prompt user
	if !interactive && appConfig.AWS.SSO.StartURL != "" && appConfig.AWS.SSO.Region != "" {
		ssoStartURL = appConfig.AWS.SSO.StartURL
		ssoRegion = appConfig.AWS.SSO.Region
		fmt.Printf("📋 Using configuration: %s (region: %s)\n", ssoStartURL, ssoRegion)
	} else {
		// Get SSO start URL from user
		if appConfig.AWS.SSO.StartURL != "" {
			fmt.Printf("Enter your AWS SSO start URL (default: %s): ", appConfig.AWS.SSO.StartURL)
		} else {
			fmt.Print("Enter your AWS SSO start URL: ")
		}
		fmt.Scanln(&ssoStartURL)
		if ssoStartURL == "" && appConfig.AWS.SSO.StartURL != "" {
			ssoStartURL = appConfig.AWS.SSO.StartURL
		}

		// Get SSO region
		defaultRegion := "us-east-1"
		if appConfig.AWS.SSO.Region != "" {
			defaultRegion = appConfig.AWS.SSO.Region
		}
		fmt.Printf("Enter your SSO region (default: %s): ", defaultRegion)
		fmt.Scanln(&ssoRegion)
		if ssoRegion == "" {
			ssoRegion = defaultRegion
		}

		// Save configuration for future use
		appConfig.AWS.SSO.StartURL = ssoStartURL
		appConfig.AWS.SSO.Region = ssoRegion
		if err := appConfig.SaveConfig(); err != nil {
			fmt.Printf("⚠️  Warning: Failed to save configuration: %v\n", err)
		} else {
			fmt.Println("💾 Configuration saved for future use")
		}
	}

	// Initiate device authorization
	ssooidcClient := ssooidc.NewFromConfig(cfg, func(o *ssooidc.Options) {
		o.Region = ssoRegion
	})

	registerResp, err := ssooidcClient.RegisterClient(context.TODO(), &ssooidc.RegisterClientInput{
		ClientName: aws.String("synacklab-cli"),
		ClientType: aws.String("public"),
	})
	if err != nil {
		return fmt.Errorf("failed to register client: %w", err)
	}

	deviceAuthResp, err := ssooidcClient.StartDeviceAuthorization(context.TODO(), &ssooidc.StartDeviceAuthorizationInput{
		ClientId:     registerResp.ClientId,
		ClientSecret: registerResp.ClientSecret,
		StartUrl:     aws.String(ssoStartURL),
	})
	if err != nil {
		return fmt.Errorf("failed to start device authorization: %w", err)
	}

	fmt.Printf("\n🌐 Please visit: %s\n", *deviceAuthResp.VerificationUriComplete)
	fmt.Printf("📋 And enter code: %s\n", *deviceAuthResp.UserCode)
	fmt.Println("\nPress Enter after completing the authorization...")
	fmt.Scanln()

	// Poll for token
	tokenResp, err := ssooidcClient.CreateToken(context.TODO(), &ssooidc.CreateTokenInput{
		ClientId:     registerResp.ClientId,
		ClientSecret: registerResp.ClientSecret,
		DeviceCode:   deviceAuthResp.DeviceCode,
		GrantType:    aws.String("urn:ietf:params:oauth:grant-type:device_code"),
	})
	if err != nil {
		return fmt.Errorf("failed to create token: %w", err)
	}

	// List available accounts and roles
	ssoClient := sso.NewFromConfig(cfg, func(o *sso.Options) {
		o.Region = ssoRegion
	})

	accountsResp, err := ssoClient.ListAccounts(context.TODO(), &sso.ListAccountsInput{
		AccessToken: tokenResp.AccessToken,
	})
	if err != nil {
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	var profiles []AWSProfile
	fmt.Println("\n📋 Available AWS profiles:")

	for _, account := range accountsResp.AccountList {
		rolesResp, err := ssoClient.ListAccountRoles(context.TODO(), &sso.ListAccountRolesInput{
			AccessToken: tokenResp.AccessToken,
			AccountId:   account.AccountId,
		})
		if err != nil {
			continue
		}

		for _, role := range rolesResp.RoleList {
			profile := AWSProfile{
				Name:      fmt.Sprintf("%s-%s", *account.AccountName, *role.RoleName),
				AccountID: *account.AccountId,
				RoleName:  *role.RoleName,
				Region:    ssoRegion,
			}
			profiles = append(profiles, profile)
			fmt.Printf("%d. %s (Account: %s, Role: %s)\n",
				len(profiles), profile.Name, profile.AccountID, profile.RoleName)
		}
	}

	if len(profiles) == 0 {
		return fmt.Errorf("no profiles found")
	}

	// Let user choose profile
	fmt.Print("\nSelect profile number to set as default: ")
	var choice int
	fmt.Scanln(&choice)

	if choice < 1 || choice > len(profiles) {
		return fmt.Errorf("invalid selection")
	}

	selectedProfile := profiles[choice-1]

	// Update AWS config file
	err = updateAWSConfig(selectedProfile, ssoStartURL, ssoRegion)
	if err != nil {
		return fmt.Errorf("failed to update AWS config: %w", err)
	}

	fmt.Printf("✅ Successfully configured profile '%s' as default\n", selectedProfile.Name)
	return nil
}

func updateAWSConfig(profile AWSProfile, ssoStartURL, ssoRegion string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	awsDir := filepath.Join(homeDir, ".aws")
	configPath := filepath.Join(awsDir, "config")

	// Create .aws directory if it doesn't exist
	if err := os.MkdirAll(awsDir, 0755); err != nil {
		return err
	}

	// Load or create config file
	var cfg *ini.File
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg = ini.Empty()
	} else {
		cfg, err = ini.Load(configPath)
		if err != nil {
			return err
		}
	}

	// Update default profile
	defaultSection, err := cfg.NewSection("default")
	if err != nil {
		defaultSection = cfg.Section("default")
	}

	defaultSection.Key("sso_start_url").SetValue(ssoStartURL)
	defaultSection.Key("sso_region").SetValue(ssoRegion)
	defaultSection.Key("sso_account_id").SetValue(profile.AccountID)
	defaultSection.Key("sso_role_name").SetValue(profile.RoleName)
	defaultSection.Key("region").SetValue(profile.Region)
	defaultSection.Key("output").SetValue("json")

	// Save the file
	return cfg.SaveTo(configPath)
}
