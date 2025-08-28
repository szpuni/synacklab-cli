package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"

	"synacklab/pkg/config"
)

// Manager defines the interface for AWS SSO authentication management
type Manager interface {
	// IsAuthenticated checks if user has valid AWS SSO credentials
	IsAuthenticated(ctx context.Context) (bool, error)

	// Authenticate performs AWS SSO device flow authentication
	Authenticate(ctx context.Context, config *config.Config) (*SSOSession, error)

	// GetStoredCredentials retrieves cached authentication credentials
	GetStoredCredentials() (*SSOSession, error)

	// ClearCredentials removes stored authentication credentials
	ClearCredentials() error

	// ValidateSession checks if the current session is still valid
	ValidateSession(ctx context.Context, session *SSOSession) error
}

// SSOSession represents AWS SSO session information
type SSOSession struct {
	AccessToken string    `json:"access_token"`
	StartURL    string    `json:"start_url"`
	Region      string    `json:"region"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// DefaultManager implements the Manager interface
type DefaultManager struct {
	credentialsPath string
	browserOpener   BrowserOpener
}

// NewManager creates a new authentication manager instance
func NewManager() (*DefaultManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	credentialsPath := filepath.Join(homeDir, ".synacklab", "aws_credentials.json")

	return &DefaultManager{
		credentialsPath: credentialsPath,
		browserOpener:   NewBrowserOpener(),
	}, nil
}

// NewManagerWithBrowserOpener creates a new authentication manager with a custom browser opener
func NewManagerWithBrowserOpener(browserOpener BrowserOpener) (*DefaultManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	credentialsPath := filepath.Join(homeDir, ".synacklab", "aws_credentials.json")

	return &DefaultManager{
		credentialsPath: credentialsPath,
		browserOpener:   browserOpener,
	}, nil
}

// IsAuthenticated checks if user has valid AWS SSO credentials
func (m *DefaultManager) IsAuthenticated(ctx context.Context) (bool, error) {
	session, err := m.GetStoredCredentials()
	if err != nil {
		// Check if it's a file system error that should be reported
		if authErr := ClassifyError(err); authErr != nil && authErr.Type == ErrorTypePermissionDenied {
			return false, authErr
		}
		return false, nil // No stored credentials or benign error reading them
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		// Clear expired credentials
		_ = m.ClearCredentials()
		return false, nil
	}

	// Validate session by making a test API call
	if err := m.ValidateSession(ctx, session); err != nil {
		// Classify the validation error
		if authErr := ClassifyError(err); authErr != nil {
			// For session expiry, clear credentials and return false (not an error)
			if authErr.Type == ErrorTypeSessionExpired {
				_ = m.ClearCredentials()
				return false, nil
			}
			// For other errors (network, etc.), return the error
			if authErr.Type == ErrorTypeNetworkConnectivity || authErr.Type == ErrorTypeNetworkTimeout {
				return false, authErr
			}
		}

		// Clear invalid credentials for unknown errors
		_ = m.ClearCredentials()
		return false, nil
	}

	return true, nil
}

// Authenticate performs AWS SSO device flow authentication
func (m *DefaultManager) Authenticate(ctx context.Context, appConfig *config.Config) (*SSOSession, error) {
	// Validate configuration first
	if err := ValidateAWSConfig(appConfig.AWS.SSO.StartURL, appConfig.AWS.SSO.Region); err != nil {
		return nil, err
	}

	fmt.Printf("🔐 Authenticating with AWS SSO: %s\n", appConfig.AWS.SSO.StartURL)

	// Initialize AWS config
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, ClassifyError(fmt.Errorf("failed to load AWS config: %w", err))
	}

	// Create SSO OIDC client
	ssooidcClient := ssooidc.NewFromConfig(cfg, func(o *ssooidc.Options) {
		o.Region = appConfig.AWS.SSO.Region
	})

	// Register client
	registerResp, err := ssooidcClient.RegisterClient(ctx, &ssooidc.RegisterClientInput{
		ClientName: aws.String("synacklab-cli"),
		ClientType: aws.String("public"),
	})
	if err != nil {
		return nil, ClassifyError(fmt.Errorf("failed to register client: %w", err))
	}

	// Start device authorization
	deviceAuthResp, err := ssooidcClient.StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
		ClientId:     registerResp.ClientId,
		ClientSecret: registerResp.ClientSecret,
		StartUrl:     aws.String(appConfig.AWS.SSO.StartURL),
	})
	if err != nil {
		return nil, ClassifyError(fmt.Errorf("failed to start device authorization: %w", err))
	}

	// Check for nil pointers from AWS response
	if deviceAuthResp.VerificationUriComplete == nil || deviceAuthResp.UserCode == nil {
		return nil, ClassifyError(fmt.Errorf("invalid device authorization response from AWS"))
	}

	fmt.Printf("\n🌐 Opening browser for authorization: %s\n", *deviceAuthResp.VerificationUriComplete)
	fmt.Printf("📋 Verification code: %s\n", *deviceAuthResp.UserCode)

	// Try to open browser automatically
	browserOpened := false
	if err := m.browserOpener.Open(*deviceAuthResp.VerificationUriComplete); err != nil {
		fmt.Printf("⚠️  Failed to open browser automatically: %v\n", err)
		fmt.Printf("🌐 Please manually visit: %s\n", *deviceAuthResp.VerificationUriComplete)
	} else {
		fmt.Println("✅ Browser opened automatically")
		browserOpened = true
	}

	fmt.Println("\n⏳ Waiting for authorization completion...")

	// Poll for token with exponential backoff
	var tokenResp *ssooidc.CreateTokenOutput
	pollInterval := 5 * time.Second
	maxPollInterval := 30 * time.Second
	pollTimeout := time.Now().Add(10 * time.Minute) // 10 minute timeout for polling

	for time.Now().Before(pollTimeout) {
		select {
		case <-ctx.Done():
			return nil, ClassifyError(fmt.Errorf("authentication cancelled: %w", ctx.Err()))
		default:
		}

		tokenResp, err = ssooidcClient.CreateToken(ctx, &ssooidc.CreateTokenInput{
			ClientId:     registerResp.ClientId,
			ClientSecret: registerResp.ClientSecret,
			DeviceCode:   deviceAuthResp.DeviceCode,
			GrantType:    aws.String("urn:ietf:params:oauth:grant-type:device_code"),
		})

		if err == nil {
			fmt.Println("✅ Authorization completed successfully!")
			break
		}

		// Classify the error to determine if we should continue polling
		authErr := ClassifyError(err)
		if authErr != nil {
			// For authorization pending, continue polling
			if authErr.Type == ErrorTypeAuthorizationPending {
				fmt.Print(".")
				time.Sleep(pollInterval)

				// Increase poll interval up to maximum
				if pollInterval < maxPollInterval {
					pollInterval = time.Duration(float64(pollInterval) * 1.2)
					if pollInterval > maxPollInterval {
						pollInterval = maxPollInterval
					}
				}
				continue
			}

			// For slow down errors, increase the poll interval
			if authErr.Type == ErrorTypeSlowDown {
				fmt.Print("⏳")
				pollInterval = pollInterval * 2
				if pollInterval > maxPollInterval {
					pollInterval = maxPollInterval
				}
				time.Sleep(pollInterval)
				continue
			}

			// For expired token or other terminal errors, stop polling
			if authErr.Type == ErrorTypeExpiredToken || authErr.Type == ErrorTypeAccessDenied {
				return nil, authErr
			}
		}

		// For unknown errors, provide fallback instructions
		if !browserOpened {
			fmt.Printf("\n⚠️  Polling failed: %v\n", err)
			fmt.Printf("🌐 Please ensure you've completed authorization at: %s\n", *deviceAuthResp.VerificationUriComplete)
		}

		time.Sleep(pollInterval)
	}

	if tokenResp == nil {
		return nil, ClassifyError(fmt.Errorf("authentication timed out after 10 minutes"))
	}

	// Calculate expiry time (AWS SSO tokens typically expire in 8 hours)
	expiresAt := time.Now().Add(8 * time.Hour)
	if tokenResp.ExpiresIn != 0 {
		expiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	session := &SSOSession{
		AccessToken: *tokenResp.AccessToken,
		StartURL:    appConfig.AWS.SSO.StartURL,
		Region:      appConfig.AWS.SSO.Region,
		ExpiresAt:   expiresAt,
	}

	// Store credentials
	if err := m.storeCredentials(session); err != nil {
		return nil, ClassifyError(fmt.Errorf("failed to store credentials: %w", err))
	}

	fmt.Println("✅ Successfully authenticated with AWS SSO")
	return session, nil
}

// GetStoredCredentials retrieves cached authentication credentials
func (m *DefaultManager) GetStoredCredentials() (*SSOSession, error) {
	if _, err := os.Stat(m.credentialsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no stored credentials found")
	}

	data, err := os.ReadFile(m.credentialsPath)
	if err != nil {
		return nil, ClassifyError(fmt.Errorf("failed to read credentials file: %w", err))
	}

	var session SSOSession
	if err := json.Unmarshal(data, &session); err != nil {
		// If credentials are corrupted, clear them
		_ = m.ClearCredentials()
		return nil, &Error{
			Type:          ErrorTypeInvalidCredentials,
			Message:       "Stored credentials are corrupted",
			OriginalError: err,
			TroubleshootingSteps: []string{
				"Credentials have been cleared automatically",
				"Run 'synacklab auth aws-login' to re-authenticate",
			},
		}
	}

	return &session, nil
}

// ClearCredentials removes stored authentication credentials
func (m *DefaultManager) ClearCredentials() error {
	if _, err := os.Stat(m.credentialsPath); os.IsNotExist(err) {
		return nil // Nothing to clear
	}

	if err := os.Remove(m.credentialsPath); err != nil {
		return fmt.Errorf("failed to remove credentials file: %w", err)
	}

	return nil
}

// ValidateSession checks if the current session is still valid
func (m *DefaultManager) ValidateSession(ctx context.Context, session *SSOSession) error {
	// Initialize AWS config
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return ClassifyError(fmt.Errorf("failed to load AWS config: %w", err))
	}

	// Create SSO client
	ssoClient := sso.NewFromConfig(cfg, func(o *sso.Options) {
		o.Region = session.Region
	})

	// Try to list accounts to validate the session
	_, err = ssoClient.ListAccounts(ctx, &sso.ListAccountsInput{
		AccessToken: aws.String(session.AccessToken),
	})
	if err != nil {
		return ClassifyError(fmt.Errorf("session validation failed: %w", err))
	}

	return nil
}

// storeCredentials saves authentication credentials to disk
func (m *DefaultManager) storeCredentials(session *SSOSession) error {
	// Create credentials directory if it doesn't exist
	credentialsDir := filepath.Dir(m.credentialsPath)
	if err := os.MkdirAll(credentialsDir, 0700); err != nil {
		return ClassifyError(fmt.Errorf("failed to create credentials directory: %w", err))
	}

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Write with restricted permissions (owner only)
	if err := os.WriteFile(m.credentialsPath, data, 0600); err != nil {
		return ClassifyError(fmt.Errorf("failed to write credentials file: %w", err))
	}

	return nil
}
