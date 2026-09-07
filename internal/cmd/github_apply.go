package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"synacklab/pkg/config"
	"synacklab/pkg/github"
)

var (
	githubDryRun bool
	githubOwner  string
	githubRepos  []string
)

var githubApplyCmd = &cobra.Command{
	Use:   "apply <config-file.yaml>",
	Short: "Apply repository configuration to GitHub",
	Long: `Apply repository configuration from a YAML file to GitHub.

This command reads a repository configuration file and applies it to GitHub,
creating new repositories or reconciling existing ones to match the desired state.

CONFIGURATION FORMATS:

Single Repository Format:
  Traditional format with repository settings defined directly in the YAML file.
  All existing single repository configurations continue to work unchanged.

Multi-Repository Format:
  Define multiple repositories in a single YAML file with optional global defaults.
  Supports selective operations using the --repos flag to process specific repositories.
  Global defaults are merged with repository-specific settings for consistency.

MULTI-REPOSITORY FEATURES:

• Batch Operations: Process multiple repositories in a single command
• Global Defaults: Define common settings applied to all repositories
• Selective Processing: Use --repos flag to operate on specific repositories only
• Independent Processing: Failures in one repository don't stop others
• Comprehensive Reporting: Detailed success/failure status for each repository
• Rate Limiting: Intelligent GitHub API rate limit handling across repositories

Examples:
  # Single repository operations
  synacklab github apply my-repo.yaml
  synacklab github apply my-repo.yaml --dry-run --owner myorg

  # Multi-repository operations
  synacklab github apply multi-repos.yaml
  synacklab github apply multi-repos.yaml --owner myorg

  # Selective multi-repository operations
  synacklab github apply multi-repos.yaml --repos repo1,repo2
  synacklab github apply multi-repos.yaml --repos "user-service,payment-service"

  # Preview multi-repository changes
  synacklab github apply multi-repos.yaml --dry-run
  synacklab github apply multi-repos.yaml --dry-run --repos repo1,repo2

Configuration Examples:
  See examples/ directory for sample configurations:
  • examples/github-simple-repo.yaml - Single repository format
  • examples/github-multi-repo-basic.yaml - Basic multi-repository format
  • examples/github-multi-repo-advanced.yaml - Advanced multi-repository with defaults
  • examples/migration-single-to-multi.md - Migration guide`,
	Args: cobra.ExactArgs(1),
	RunE: runGitHubApply,
}

func init() {
	githubApplyCmd.Flags().BoolVar(&githubDryRun, "dry-run", false, "Preview changes without applying them (shows planned changes for all repositories)")
	githubApplyCmd.Flags().StringVar(&githubOwner, "owner", "", "Repository owner (organization or user) - required for team operations")
	githubApplyCmd.Flags().StringSliceVar(&githubRepos, "repos", nil, "Comma-separated list of repository names to process from multi-repository configuration (e.g., --repos repo1,repo2)")
	githubCmd.AddCommand(githubApplyCmd)
}

func runGitHubApply(_ *cobra.Command, args []string) error {
	configFile := args[0]

	// Load configuration and detect format first (before authentication)
	configData, configFormat, err := github.LoadConfigFromFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to load repository config: %w", err)
	}

	// Load synacklab configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load synacklab config: %w", err)
	}

	// Determine repository owner
	repoOwner := githubOwner
	if repoOwner == "" {
		if cfg.GitHub.Organization != "" {
			repoOwner = cfg.GitHub.Organization
		} else {
			return fmt.Errorf("repository owner not specified: use --owner flag or set github.organization in config")
		}
	}

	// Set up GitHub authentication
	authManager := github.NewManager()
	tokenInfo, err := authManager.AuthenticateFromConfig(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Authentication failed: %v\n\n", err)
		fmt.Fprintf(os.Stderr, "%s\n", github.GetAuthInstructions())
		return err
	}

	fmt.Printf("✓ Authenticated as %s\n", tokenInfo.User)

	// Create GitHub client
	token, err := authManager.GetToken(cfg)
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	client := github.NewClient(token)

	switch configFormat {
	case github.FormatSingleRepository:
		return runSingleRepositoryApply(client, repoOwner, configData.(*github.RepositoryConfig))
	case github.FormatMultiRepository:
		return runMultiRepositoryApply(client, repoOwner, configData.(*github.MultiRepositoryConfig))
	default:
		return fmt.Errorf("unsupported configuration format: %s", configFormat)
	}
}

// displayPlan shows the planned changes in a human-readable format
func displayPlan(plan *github.ReconciliationPlan, owner, repoName string, isDryRun bool) error {
	if isDryRun {
		fmt.Printf("\n🔍 Dry-run mode: Showing planned changes for %s/%s\n", owner, repoName)
	} else {
		fmt.Printf("\n📋 Planned changes for %s/%s:\n", owner, repoName)
	}

	changeCount := countPlanChanges(plan)
	destructiveChanges := displayRepositoryPlanChanges(plan, "  ")

	if changeCount == 0 {
		fmt.Printf("  No changes needed - repository is up to date\n")
	} else {
		fmt.Printf("\nTotal changes: %d", changeCount)
		if destructiveChanges > 0 {
			fmt.Printf(" (%d potentially destructive)\n", destructiveChanges)
			if isDryRun {
				fmt.Printf("\n⚠️  WARNING: %d potentially destructive change(s) detected!\n", destructiveChanges)
				fmt.Printf("   Review these changes carefully before applying.\n")
			}
		} else {
			fmt.Printf("\n")
		}
	}

	return nil
}

// displayBranchProtectionDetails shows details of a branch protection rule
func displayBranchProtectionDetails(bp *github.BranchProtection, indent string) {
	if bp.RequiredReviews > 0 {
		fmt.Printf("%s- Required reviews: %d\n", indent, bp.RequiredReviews)
		if bp.DismissStaleReviews {
			fmt.Printf("%s- Dismiss stale reviews: enabled\n", indent)
		}
		if bp.RequireCodeOwnerReview {
			fmt.Printf("%s- Require code owner review: enabled\n", indent)
		}
	}
	if len(bp.RequiredStatusChecks) > 0 {
		fmt.Printf("%s- Required status checks: %s\n", indent, strings.Join(bp.RequiredStatusChecks, ", "))
		if bp.RequireUpToDate {
			fmt.Printf("%s- Require up-to-date branches: enabled\n", indent)
		}
	}
	if len(bp.RestrictPushes) > 0 {
		fmt.Printf("%s- Restrict pushes to: %s\n", indent, strings.Join(bp.RestrictPushes, ", "))
	}
	if bp.EnforceAdmins {
		fmt.Printf("%s- Enforce for admins: enabled\n", indent)
	}
}

// displayBranchProtectionChanges shows changes between two branch protection rules and returns destructive change count
func displayBranchProtectionChanges(before, after *github.BranchProtection, indent string) int {
	destructiveChanges := 0

	if before.RequiredReviews != after.RequiredReviews {
		if before.RequiredReviews > after.RequiredReviews {
			fmt.Printf("%s⚠️  Required reviews: %d → %d (REDUCING PROTECTION)\n", indent, before.RequiredReviews, after.RequiredReviews)
			destructiveChanges++
		} else {
			fmt.Printf("%s~ Required reviews: %d → %d\n", indent, before.RequiredReviews, after.RequiredReviews)
		}
	}
	if before.DismissStaleReviews != after.DismissStaleReviews {
		if before.DismissStaleReviews && !after.DismissStaleReviews {
			fmt.Printf("%s⚠️  Dismiss stale reviews: %t → %t (REDUCING PROTECTION)\n", indent, before.DismissStaleReviews, after.DismissStaleReviews)
			destructiveChanges++
		} else {
			fmt.Printf("%s~ Dismiss stale reviews: %t → %t\n", indent, before.DismissStaleReviews, after.DismissStaleReviews)
		}
	}
	if before.RequireCodeOwnerReview != after.RequireCodeOwnerReview {
		if before.RequireCodeOwnerReview && !after.RequireCodeOwnerReview {
			fmt.Printf("%s⚠️  Require code owner review: %t → %t (REDUCING PROTECTION)\n", indent, before.RequireCodeOwnerReview, after.RequireCodeOwnerReview)
			destructiveChanges++
		} else {
			fmt.Printf("%s~ Require code owner review: %t → %t\n", indent, before.RequireCodeOwnerReview, after.RequireCodeOwnerReview)
		}
	}
	if !stringSlicesEqual(before.RequiredStatusChecks, after.RequiredStatusChecks) {
		if len(before.RequiredStatusChecks) > len(after.RequiredStatusChecks) {
			fmt.Printf("%s⚠️  Required status checks: [%s] → [%s] (REDUCING PROTECTION)\n", indent,
				strings.Join(before.RequiredStatusChecks, ", "),
				strings.Join(after.RequiredStatusChecks, ", "))
			destructiveChanges++
		} else {
			fmt.Printf("%s~ Required status checks: [%s] → [%s]\n", indent,
				strings.Join(before.RequiredStatusChecks, ", "),
				strings.Join(after.RequiredStatusChecks, ", "))
		}
	}
	if before.RequireUpToDate != after.RequireUpToDate {
		if before.RequireUpToDate && !after.RequireUpToDate {
			fmt.Printf("%s⚠️  Require up-to-date branches: %t → %t (REDUCING PROTECTION)\n", indent, before.RequireUpToDate, after.RequireUpToDate)
			destructiveChanges++
		} else {
			fmt.Printf("%s~ Require up-to-date branches: %t → %t\n", indent, before.RequireUpToDate, after.RequireUpToDate)
		}
	}
	if !stringSlicesEqual(before.RestrictPushes, after.RestrictPushes) {
		if len(before.RestrictPushes) > len(after.RestrictPushes) {
			fmt.Printf("%s⚠️  Restrict pushes: [%s] → [%s] (REDUCING PROTECTION)\n", indent,
				strings.Join(before.RestrictPushes, ", "),
				strings.Join(after.RestrictPushes, ", "))
			destructiveChanges++
		} else {
			fmt.Printf("%s~ Restrict pushes: [%s] → [%s]\n", indent,
				strings.Join(before.RestrictPushes, ", "),
				strings.Join(after.RestrictPushes, ", "))
		}
	}
	if before.EnforceAdmins != after.EnforceAdmins {
		if before.EnforceAdmins && !after.EnforceAdmins {
			fmt.Printf("%s⚠️  Enforce for admins: %t → %t (REDUCING PROTECTION)\n", indent, before.EnforceAdmins, after.EnforceAdmins)
			destructiveChanges++
		} else {
			fmt.Printf("%s~ Enforce for admins: %t → %t\n", indent, before.EnforceAdmins, after.EnforceAdmins)
		}
	}

	return destructiveChanges
}

// hasChanges checks if the plan contains any changes
func hasChanges(plan *github.ReconciliationPlan) bool {
	return plan.Repository != nil ||
		len(plan.BranchRules) > 0 ||
		len(plan.Collaborators) > 0 ||
		len(plan.Teams) > 0 ||
		len(plan.Webhooks) > 0
}

// displaySuccessSummary shows a summary after successful application
func displaySuccessSummary(plan *github.ReconciliationPlan, owner, repoName string) {
	fmt.Printf("\n✅ Successfully applied changes to %s/%s\n", owner, repoName)

	if plan.Repository != nil && plan.Repository.Type == github.ChangeTypeCreate {
		fmt.Printf("🎉 Repository created: https://github.com/%s/%s\n", owner, repoName)
	} else {
		fmt.Printf("🔗 Repository: https://github.com/%s/%s\n", owner, repoName)
	}

	// Count applied changes
	changeCount := 0
	if plan.Repository != nil {
		changeCount++
	}
	changeCount += len(plan.BranchRules) + len(plan.Collaborators) + len(plan.Teams) + len(plan.Webhooks)

	fmt.Printf("📊 Applied %d change(s)\n", changeCount)
}

// isPermissionDowngrade checks if the permission change is a downgrade
func isPermissionDowngrade(before, after string) bool {
	// Define permission hierarchy: admin > write > read
	permissionLevels := map[string]int{
		"read":  1,
		"write": 2,
		"admin": 3,
	}

	beforeLevel, beforeExists := permissionLevels[before]
	afterLevel, afterExists := permissionLevels[after]

	// If either permission is unknown, don't consider it a downgrade
	if !beforeExists || !afterExists {
		return false
	}

	return beforeLevel > afterLevel
}

// stringSlicesEqual compares two string slices for equality (order doesn't matter)
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create maps to count occurrences
	countA := make(map[string]int)
	countB := make(map[string]int)

	for _, s := range a {
		countA[s]++
	}
	for _, s := range b {
		countB[s]++
	}

	// Compare maps
	for k, v := range countA {
		if countB[k] != v {
			return false
		}
	}

	return true
}

// runSingleRepositoryApply handles single repository configuration
func runSingleRepositoryApply(client github.APIClient, repoOwner string, repoConfig *github.RepositoryConfig) error {
	// Create single repository reconciler
	reconciler := github.NewReconciler(client, repoOwner)

	// Validate configuration
	if err := reconciler.Validate(*repoConfig); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	fmt.Printf("✓ Configuration validated\n")

	// Create reconciliation plan
	plan, err := reconciler.Plan(*repoConfig)
	if err != nil {
		return fmt.Errorf("failed to create reconciliation plan: %w", err)
	}

	// Display planned changes
	if err := displayPlan(plan, repoOwner, repoConfig.Name, githubDryRun); err != nil {
		return fmt.Errorf("failed to display plan: %w", err)
	}

	// If dry-run, stop here
	if githubDryRun {
		fmt.Printf("\n✓ Dry-run completed. No changes were applied.\n")
		return nil
	}

	// Check if there are any changes to apply
	if !hasChanges(plan) {
		fmt.Printf("\n✓ Repository is already up to date. No changes needed.\n")
		return nil
	}

	// Apply changes
	fmt.Printf("\nApplying changes...\n")
	if err := reconciler.Apply(plan); err != nil {
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	// Display success summary
	displaySuccessSummary(plan, repoOwner, repoConfig.Name)

	return nil
}

// runMultiRepositoryApply handles multi-repository configuration
func runMultiRepositoryApply(client github.APIClient, repoOwner string, multiConfig *github.MultiRepositoryConfig) error {
	// Create multi-repository reconciler
	multiReconciler := github.NewMultiReconciler(client, repoOwner)

	// Validate configuration
	validationResult, err := multiReconciler.ValidateAll(multiConfig, githubRepos)
	if err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Display validation results
	if err := displayMultiRepoValidationResults(validationResult); err != nil {
		return fmt.Errorf("failed to display validation results: %w", err)
	}

	// If there are validation errors, stop here
	if validationResult.Summary.InvalidCount > 0 {
		return fmt.Errorf("configuration validation failed for %d repositories", validationResult.Summary.InvalidCount)
	}

	fmt.Printf("✓ Configuration validated for %d repositories\n", validationResult.Summary.ValidCount)

	// Create reconciliation plans
	plans, planErr := multiReconciler.PlanAll(multiConfig, githubRepos)

	// For dry-run mode, continue even if there are planning errors to show what we can
	if planErr != nil && !githubDryRun {
		return fmt.Errorf("failed to create reconciliation plans: %w", planErr)
	}

	// Display planned changes for all repositories (including partial results if there were errors)
	if err := displayMultiRepoPlan(plans, repoOwner, githubDryRun); err != nil {
		return fmt.Errorf("failed to display plans: %w", err)
	}

	// If there were planning errors during dry-run, report them after showing successful plans
	if planErr != nil && githubDryRun {
		fmt.Printf("\n⚠️  Planning errors encountered during dry-run:\n")
		fmt.Printf("   %v\n", planErr)
		fmt.Printf("\n✓ Dry-run completed with errors. No changes were applied.\n")
		return fmt.Errorf("dry-run completed with planning errors: %w", planErr)
	}

	// If dry-run, stop here
	if githubDryRun {
		fmt.Printf("\n✓ Dry-run completed. No changes were applied.\n")
		return nil
	}

	// Check if there are any changes to apply
	totalChanges := countTotalChanges(plans)
	if totalChanges == 0 {
		fmt.Printf("\n✓ All repositories are already up to date. No changes needed.\n")
		return nil
	}

	// Apply changes to all repositories
	fmt.Printf("\nApplying changes to %d repositories...\n", len(plans))
	result, err := multiReconciler.ApplyAll(plans)
	if err != nil {
		// Handle partial failures gracefully
		if multiErr, ok := err.(*github.MultiRepoError); ok && multiErr.IsPartialFailure() {
			displayMultiRepoResults(result, repoOwner, true)
			return fmt.Errorf("partial failure: %d repositories succeeded, %d failed", result.Summary.SuccessCount, result.Summary.FailureCount)
		}
		displayMultiRepoResults(result, repoOwner, false)
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	// Display success summary
	displayMultiRepoResults(result, repoOwner, false)

	return nil
}

// displayMultiRepoValidationResults displays validation results for multiple repositories
func displayMultiRepoValidationResults(result *github.MultiRepoValidationResult) error {
	if result.Summary.InvalidCount > 0 {
		fmt.Printf("\n❌ Configuration validation failed for %d repositories:\n", result.Summary.InvalidCount)
		for repoName, err := range result.Invalid {
			fmt.Printf("  • %s: %v\n", repoName, err)
		}
	}

	if result.Summary.WarningCount > 0 {
		fmt.Printf("\n⚠️  Configuration warnings for %d repositories:\n", result.Summary.WarningCount)
		for repoName, details := range result.Details {
			if len(details.Warnings) > 0 {
				fmt.Printf("  • %s:\n", repoName)
				for _, warning := range details.Warnings {
					fmt.Printf("    - %s\n", warning.Message)
				}
			}
		}
	}

	return nil
}

// displayMultiRepoPlan displays planned changes for multiple repositories
func displayMultiRepoPlan(plans map[string]*github.ReconciliationPlan, owner string, isDryRun bool) error {
	if isDryRun {
		fmt.Printf("\n🔍 Dry-run mode: Showing planned changes for %d repositories\n", len(plans))
	} else {
		fmt.Printf("\n📋 Planned changes for %d repositories:\n", len(plans))
	}

	totalChanges := 0
	totalDestructiveChanges := 0
	repositoriesWithChanges := 0

	// Sort repository names for consistent output
	repoNames := make([]string, 0, len(plans))
	for repoName := range plans {
		repoNames = append(repoNames, repoName)
	}
	// Simple sort
	for i := 0; i < len(repoNames); i++ {
		for j := i + 1; j < len(repoNames); j++ {
			if repoNames[i] > repoNames[j] {
				repoNames[i], repoNames[j] = repoNames[j], repoNames[i]
			}
		}
	}

	repositoriesWithErrors := 0

	for _, repoName := range repoNames {
		plan := plans[repoName]
		if plan == nil {
			// Repository failed planning - show as error during dry-run
			fmt.Printf("\n📦 %s/%s: ❌ Planning failed (see errors below)\n", owner, repoName)
			repositoriesWithErrors++
			continue
		}

		repoChanges := countPlanChanges(plan)
		if repoChanges == 0 {
			fmt.Printf("\n📦 %s/%s: No changes needed\n", owner, repoName)
			continue
		}

		repositoriesWithChanges++
		totalChanges += repoChanges

		fmt.Printf("\n📦 %s/%s:\n", owner, repoName)

		// Display changes for this repository (reuse existing logic)
		destructiveChanges := displayRepositoryPlanChanges(plan, "  ")
		totalDestructiveChanges += destructiveChanges
	}

	// Display summary
	fmt.Printf("\n📊 Summary:")
	fmt.Printf("\n  • Total repositories: %d", len(plans))
	fmt.Printf("\n  • Repositories with changes: %d", repositoriesWithChanges)
	if repositoriesWithErrors > 0 {
		fmt.Printf("\n  • Repositories with planning errors: %d", repositoriesWithErrors)
	}
	fmt.Printf("\n  • Total changes: %d", totalChanges)

	if totalDestructiveChanges > 0 {
		fmt.Printf("\n  • Potentially destructive changes: %d", totalDestructiveChanges)
		if isDryRun {
			fmt.Printf("\n\n⚠️  WARNING: %d potentially destructive change(s) detected across all repositories!\n", totalDestructiveChanges)
			fmt.Printf("   Review these changes carefully before applying.\n")
		}
	}

	return nil
}

// displayRepositoryPlanChanges displays changes for a single repository plan and returns destructive change count
func displayRepositoryPlanChanges(plan *github.ReconciliationPlan, indent string) int {
	destructiveChanges := 0

	// Repository changes
	if plan.Repository != nil {
		switch plan.Repository.Type {
		case github.ChangeTypeCreate:
			fmt.Printf("%s+ Repository: CREATE new repository\n", indent)
			fmt.Printf("%s  - Name: %s\n", indent, plan.Repository.After.Name)
			fmt.Printf("%s  - Description: %s\n", indent, plan.Repository.After.Description)
			fmt.Printf("%s  - Private: %t\n", indent, plan.Repository.After.Private)
			if len(plan.Repository.After.Topics) > 0 {
				fmt.Printf("%s  - Topics: %s\n", indent, strings.Join(plan.Repository.After.Topics, ", "))
			}
		case github.ChangeTypeUpdate:
			fmt.Printf("%s~ Repository: UPDATE repository settings\n", indent)
			if plan.Repository.Before.Description != plan.Repository.After.Description {
				fmt.Printf("%s  ~ Description: %q → %q\n", indent, plan.Repository.Before.Description, plan.Repository.After.Description)
			}
			if plan.Repository.Before.Private != plan.Repository.After.Private {
				// Highlight making repository public as potentially destructive
				if plan.Repository.Before.Private && !plan.Repository.After.Private {
					fmt.Printf("%s  ⚠️  Private: %t → %t (MAKING REPOSITORY PUBLIC)\n", indent, plan.Repository.Before.Private, plan.Repository.After.Private)
					destructiveChanges++
				} else {
					fmt.Printf("%s  ~ Private: %t → %t\n", indent, plan.Repository.Before.Private, plan.Repository.After.Private)
				}
			}
			if !stringSlicesEqual(plan.Repository.Before.Topics, plan.Repository.After.Topics) {
				fmt.Printf("%s  ~ Topics: [%s] → [%s]\n", indent,
					strings.Join(plan.Repository.Before.Topics, ", "),
					strings.Join(plan.Repository.After.Topics, ", "))
			}
		}
	}

	// Branch protection changes
	for _, change := range plan.BranchRules {
		switch change.Type {
		case github.ChangeTypeCreate:
			fmt.Printf("%s+ Branch Protection: CREATE rule for %s\n", indent, change.Branch)
			displayBranchProtectionDetails(change.After, indent+"  ")
		case github.ChangeTypeUpdate:
			fmt.Printf("%s~ Branch Protection: UPDATE rule for %s\n", indent, change.Branch)
			destructiveChanges += displayBranchProtectionChanges(change.Before, change.After, indent+"  ")
		case github.ChangeTypeDelete:
			fmt.Printf("%s⚠️  Branch Protection: DELETE rule for %s (REMOVING PROTECTION)\n", indent, change.Branch)
			destructiveChanges++
		}
	}

	// Collaborator changes
	for _, change := range plan.Collaborators {
		switch change.Type {
		case github.ChangeTypeCreate:
			fmt.Printf("%s+ Collaborator: ADD %s with %s permission\n", indent, change.After.Username, change.After.Permission)
		case github.ChangeTypeUpdate:
			// Highlight permission downgrades as potentially destructive
			if isPermissionDowngrade(change.Before.Permission, change.After.Permission) {
				fmt.Printf("%s⚠️  Collaborator: UPDATE %s permission %s → %s (REDUCING ACCESS)\n", indent,
					change.After.Username, change.Before.Permission, change.After.Permission)
				destructiveChanges++
			} else {
				fmt.Printf("%s~ Collaborator: UPDATE %s permission %s → %s\n", indent,
					change.After.Username, change.Before.Permission, change.After.Permission)
			}
		case github.ChangeTypeDelete:
			fmt.Printf("%s⚠️  Collaborator: REMOVE %s (REMOVING ACCESS)\n", indent, change.Before.Username)
			destructiveChanges++
		}
	}

	// Team changes
	for _, change := range plan.Teams {
		switch change.Type {
		case github.ChangeTypeCreate:
			fmt.Printf("%s+ Team: ADD %s with %s permission\n", indent, change.After.TeamSlug, change.After.Permission)
		case github.ChangeTypeUpdate:
			// Highlight permission downgrades as potentially destructive
			if isPermissionDowngrade(change.Before.Permission, change.After.Permission) {
				fmt.Printf("%s⚠️  Team: UPDATE %s permission %s → %s (REDUCING ACCESS)\n", indent,
					change.After.TeamSlug, change.Before.Permission, change.After.Permission)
				destructiveChanges++
			} else {
				fmt.Printf("%s~ Team: UPDATE %s permission %s → %s\n", indent,
					change.After.TeamSlug, change.Before.Permission, change.After.Permission)
			}
		case github.ChangeTypeDelete:
			fmt.Printf("%s⚠️  Team: REMOVE %s (REMOVING ACCESS)\n", indent, change.Before.TeamSlug)
			destructiveChanges++
		}
	}

	// Webhook changes
	for _, change := range plan.Webhooks {
		switch change.Type {
		case github.ChangeTypeCreate:
			fmt.Printf("%s+ Webhook: CREATE %s\n", indent, change.After.URL)
			fmt.Printf("%s  - Events: %s\n", indent, strings.Join(change.After.Events, ", "))
			fmt.Printf("%s  - Active: %t\n", indent, change.After.Active)
		case github.ChangeTypeUpdate:
			fmt.Printf("%s~ Webhook: UPDATE %s\n", indent, change.After.URL)
			if !stringSlicesEqual(change.Before.Events, change.After.Events) {
				fmt.Printf("%s  ~ Events: [%s] → [%s]\n", indent,
					strings.Join(change.Before.Events, ", "),
					strings.Join(change.After.Events, ", "))
			}
			if change.Before.Active != change.After.Active {
				fmt.Printf("%s  ~ Active: %t → %t\n", indent, change.Before.Active, change.After.Active)
			}
		case github.ChangeTypeDelete:
			fmt.Printf("%s⚠️  Webhook: DELETE %s (REMOVING WEBHOOK)\n", indent, change.Before.URL)
			destructiveChanges++
		}
	}

	return destructiveChanges
}

// displayMultiRepoResults displays the results of multi-repository operations
func displayMultiRepoResults(result *github.MultiRepoResult, owner string, isPartialFailure bool) {
	if isPartialFailure {
		fmt.Printf("\n⚠️  Partial success: Applied changes to %d repositories\n", result.Summary.SuccessCount)
	} else {
		fmt.Printf("\n✅ Successfully applied changes to %d repositories\n", result.Summary.SuccessCount)
	}

	// Display successful repositories
	if len(result.Succeeded) > 0 {
		fmt.Printf("\n✅ Successful repositories:\n")
		for _, repoName := range result.Succeeded {
			fmt.Printf("  • %s/%s: https://github.com/%s/%s\n", owner, repoName, owner, repoName)
		}
	}

	// Display failed repositories
	if len(result.Failed) > 0 {
		fmt.Printf("\n❌ Failed repositories:\n")
		for repoName, err := range result.Failed {
			fmt.Printf("  • %s/%s: %v\n", owner, repoName, err)
		}
	}

	// Display skipped repositories
	if len(result.Skipped) > 0 {
		fmt.Printf("\n⏭️  Skipped repositories:\n")
		for _, repoName := range result.Skipped {
			fmt.Printf("  • %s/%s\n", owner, repoName)
		}
	}

	// Display summary statistics
	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("  • Total repositories: %d\n", result.Summary.TotalRepositories)
	fmt.Printf("  • Successful: %d\n", result.Summary.SuccessCount)
	fmt.Printf("  • Failed: %d\n", result.Summary.FailureCount)
	fmt.Printf("  • Skipped: %d\n", result.Summary.SkippedCount)
	fmt.Printf("  • Total changes applied: %d\n", result.Summary.TotalChanges)
}

// countTotalChanges counts the total number of changes across all plans
func countTotalChanges(plans map[string]*github.ReconciliationPlan) int {
	total := 0
	for _, plan := range plans {
		total += countPlanChanges(plan)
	}
	return total
}

// countPlanChanges counts the number of changes in a single plan
func countPlanChanges(plan *github.ReconciliationPlan) int {
	if plan == nil {
		return 0
	}

	count := 0
	if plan.Repository != nil {
		count++
	}
	count += len(plan.BranchRules)
	count += len(plan.Collaborators)
	count += len(plan.Teams)
	count += len(plan.Webhooks)
	return count
}
