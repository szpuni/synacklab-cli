package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"synacklab/internal/auth"
	"synacklab/pkg/config"
)

var awsLoginCmd = &cobra.Command{
	Use:   "aws-login",
	Short: "Authenticate with AWS SSO",
	Long: `Authenticate with AWS SSO using device authorization flow.

This command initiates the AWS SSO device authorization flow to authenticate
your session. You will be prompted to visit a URL and enter a verification
code in your browser.

Once authenticated, your session credentials will be stored locally and can
be used by other commands like 'aws-ctx' to switch between AWS profiles.

Examples:
  synacklab auth aws-login
  synacklab auth aws-login --timeout 300`,
	RunE: runAWSLogin,
}

var (
	loginTimeout int
)

func init() {
	awsLoginCmd.Flags().IntVar(&loginTimeout, "timeout", 300, "Timeout in seconds for the authentication process")
}

// runAWSLogin handles the AWS SSO authentication process
func runAWSLogin(_ *cobra.Command, _ []string) error {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(loginTimeout)*time.Second)
	defer cancel()

	// Load configuration
	appConfig, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create authentication manager
	authManager, err := auth.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create authentication manager: %w", err)
	}

	// Check if already authenticated
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

	if isAuthenticated {
		fmt.Println("✅ Already authenticated with AWS SSO")

		// Get stored credentials to show session info
		session, err := authManager.GetStoredCredentials()
		if err == nil {
			fmt.Printf("📍 SSO URL: %s\n", session.StartURL)
			fmt.Printf("🌍 Region: %s\n", session.Region)
			fmt.Printf("⏰ Session expires: %s\n", session.ExpiresAt.Format("2006-01-02 15:04:05 MST"))
		}

		return nil
	}

	// Perform authentication
	fmt.Println("🚀 Starting AWS SSO authentication...")
	session, err := authManager.Authenticate(ctx, appConfig)
	if err != nil {
		// Handle timeout specifically
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Printf("⏰ Authentication timed out after %d seconds\n", loginTimeout)
			fmt.Println("\nTroubleshooting steps:")
			fmt.Println("1. Try again with a longer timeout: --timeout 600")
			fmt.Println("2. Check your internet connection")
			fmt.Println("3. Complete the browser authorization more quickly")
			return fmt.Errorf("authentication timeout")
		}

		// Handle structured authentication errors
		var authErr *auth.Error
		if errors.As(err, &authErr) {
			fmt.Printf("❌ %s%s\n", authErr.Message, authErr.GetTroubleshootingMessage())

			// Clear credentials for certain error types
			if authErr.Type == auth.ErrorTypeInvalidCredentials || authErr.Type == auth.ErrorTypeSessionExpired {
				_ = authManager.ClearCredentials()
			}

			return fmt.Errorf("authentication failed")
		}

		// Clear any potentially corrupted credentials for unknown errors
		_ = authManager.ClearCredentials()
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Display success information
	fmt.Printf("\n🎉 Authentication successful!\n")
	fmt.Printf("📍 SSO URL: %s\n", session.StartURL)
	fmt.Printf("🌍 Region: %s\n", session.Region)
	fmt.Printf("⏰ Session expires: %s\n", session.ExpiresAt.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("\n💡 You can now use 'synacklab auth aws-ctx' to switch between AWS profiles\n")

	return nil
}
