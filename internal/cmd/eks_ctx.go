package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"synacklab/pkg/fuzzy"
)

var (
	listContexts bool
	useFilter    bool
)

var eksCtxCmd = &cobra.Command{
	Use:   "eks-ctx",
	Short: "Switch Kubernetes context interactively",
	Long: `Switch between Kubernetes contexts using an interactive fuzzy finder.
This command will:
- Parse your ~/.kube/config file
- Display all available contexts
- Allow you to select a context interactively
- Set the selected context as the current context`,
	RunE: runEKSCtx,
}

func init() {
	eksCtxCmd.Flags().BoolVarP(&listContexts, "list", "l", false, "List all available contexts without switching")
	eksCtxCmd.Flags().BoolVarP(&useFilter, "filter", "f", false, "Use filtering mode for better search experience")
}

// KubeContext represents a Kubernetes context with additional metadata
type KubeContextInfo struct {
	Name      string
	Cluster   string
	User      string
	Namespace string
	IsCurrent bool
}

func runEKSCtx(_ *cobra.Command, _ []string) error {
	fmt.Println("🔍 Loading Kubernetes contexts...")

	// Load kubeconfig
	kubeConfig, configPath, err := loadKubeConfigForCtx()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Parse contexts
	contexts, err := parseContexts(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to parse contexts: %w", err)
	}

	if len(contexts) == 0 {
		fmt.Println("❌ No Kubernetes contexts found in kubeconfig")
		return nil
	}

	fmt.Printf("📋 Found %d Kubernetes context(s)\n", len(contexts))

	// If list mode, just display contexts and exit
	if listContexts {
		return displayContexts(contexts)
	}

	// Use fuzzy finder to select context
	selectedContext, err := selectContextWithFuzzyFinder(contexts)
	if err != nil {
		return fmt.Errorf("failed to select context: %w", err)
	}

	// Update current context in kubeconfig
	err = updateCurrentContext(kubeConfig, selectedContext, configPath)
	if err != nil {
		return fmt.Errorf("failed to update current context: %w", err)
	}

	fmt.Printf("✅ Successfully switched to context: %s\n", selectedContext)
	return nil
}

func loadKubeConfigForCtx() (*KubeConfig, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".kube", "config")

	// Check if kubeconfig exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("kubeconfig not found at %s", configPath)
	}

	// Load kubeconfig
	kubeConfig, err := loadKubeConfig(configPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load kubeconfig from %s: %w", configPath, err)
	}

	return kubeConfig, configPath, nil
}

func parseContexts(kubeConfig *KubeConfig) ([]KubeContextInfo, error) {
	var contexts []KubeContextInfo

	for _, context := range kubeConfig.Contexts {
		contextInfo := KubeContextInfo{
			Name:      context.Name,
			Cluster:   context.Context.Cluster,
			User:      context.Context.User,
			Namespace: context.Context.Namespace,
			IsCurrent: context.Name == kubeConfig.CurrentContext,
		}

		// Set default namespace if not specified
		if contextInfo.Namespace == "" {
			contextInfo.Namespace = "default"
		}

		contexts = append(contexts, contextInfo)
	}

	return contexts, nil
}

func displayContexts(contexts []KubeContextInfo) error {
	fmt.Println("\n📋 Available Kubernetes contexts:")
	fmt.Println(strings.Repeat("-", 60))

	for _, ctx := range contexts {
		currentMarker := " "
		if ctx.IsCurrent {
			currentMarker = "*"
		}

		fmt.Printf("%s %-25s | Cluster: %-20s | Namespace: %s\n",
			currentMarker, ctx.Name, ctx.Cluster, ctx.Namespace)
	}

	// Find and display current context
	for _, ctx := range contexts {
		if ctx.IsCurrent {
			fmt.Printf("\nCurrent context: %s\n", ctx.Name)
			break
		}
	}

	return nil
}

func selectContextWithFuzzyFinder(contexts []KubeContextInfo) (string, error) {
	// Create fzf-based fuzzy finder
	finder := fuzzy.NewFzf("🔍 Select Kubernetes context:")

	// Build options with consistent metadata
	var options []fuzzy.Option
	for _, ctx := range contexts {
		// Build description with cluster and namespace info
		description := fmt.Sprintf("Cluster: %s, Namespace: %s", ctx.Cluster, ctx.Namespace)
		if ctx.IsCurrent {
			description += " (current)"
		}

		// Add metadata for consistent display
		metadata := map[string]string{
			"cluster":   ctx.Cluster,
			"user":      ctx.User,
			"namespace": ctx.Namespace,
			"current":   fmt.Sprintf("%t", ctx.IsCurrent),
		}

		options = append(options, fuzzy.Option{
			Value:       ctx.Name,
			Description: description,
			Metadata:    metadata,
		})
	}

	// Set options and select
	if err := finder.SetOptions(options); err != nil {
		return "", fmt.Errorf("failed to set finder options: %w", err)
	}

	selectedContext, err := finder.Select()
	if err != nil {
		return "", fmt.Errorf("context selection failed: %w", err)
	}

	return selectedContext, nil
}

func updateCurrentContext(kubeConfig *KubeConfig, contextName, configPath string) error {
	// Verify the context exists
	contextExists := false
	for _, context := range kubeConfig.Contexts {
		if context.Name == contextName {
			contextExists = true
			break
		}
	}

	if !contextExists {
		return fmt.Errorf("context '%s' not found in kubeconfig", contextName)
	}

	// Update current context
	kubeConfig.CurrentContext = contextName

	// Save kubeconfig
	err := saveKubeConfig(kubeConfig, configPath)
	if err != nil {
		return fmt.Errorf("failed to save kubeconfig: %w", err)
	}

	return nil
}

// Helper function to get current context information
func getCurrentContextInfo(kubeConfig *KubeConfig) (*KubeContextInfo, error) {
	if kubeConfig.CurrentContext == "" {
		return nil, fmt.Errorf("no current context set")
	}

	for _, context := range kubeConfig.Contexts {
		if context.Name == kubeConfig.CurrentContext {
			return &KubeContextInfo{
				Name:      context.Name,
				Cluster:   context.Context.Cluster,
				User:      context.Context.User,
				Namespace: context.Context.Namespace,
				IsCurrent: true,
			}, nil
		}
	}

	return nil, fmt.Errorf("current context '%s' not found in contexts", kubeConfig.CurrentContext)
}

// Helper function to validate context configuration
func validateContext(kubeConfig *KubeConfig, contextName string) error {
	// Find the context
	var targetContext *KubeContext
	for _, context := range kubeConfig.Contexts {
		if context.Name == contextName {
			targetContext = &context
			break
		}
	}

	if targetContext == nil {
		return fmt.Errorf("context '%s' not found", contextName)
	}

	// Validate cluster exists
	clusterExists := false
	for _, cluster := range kubeConfig.Clusters {
		if cluster.Name == targetContext.Context.Cluster {
			clusterExists = true
			break
		}
	}

	if !clusterExists {
		return fmt.Errorf("cluster '%s' referenced by context '%s' not found",
			targetContext.Context.Cluster, contextName)
	}

	// Validate user exists
	userExists := false
	for _, user := range kubeConfig.Users {
		if user.Name == targetContext.Context.User {
			userExists = true
			break
		}
	}

	if !userExists {
		return fmt.Errorf("user '%s' referenced by context '%s' not found",
			targetContext.Context.User, contextName)
	}

	return nil
}
