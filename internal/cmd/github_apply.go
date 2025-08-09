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
)

var githubApplyCmd = &cobra.Command{
	Use:   "apply <config-file.yaml>",
	Short: "Apply repository configuration to GitHub",
	Long: `Apply repository configuration from a YAML file to GitHub.

This command reads a repository configuration file and applies it to GitHub,
creating new repositories or reconciling existing ones to match the desired state.

The configuration file should contain repository settings, branch protection rules,
collaborators, teams, and webhooks as defined in the YAML schema.

Examples:
  # Apply configuration to create or update a repository
  synacklab github apply my-repo.yaml

  # Preview changes without applying them
  synacklab github apply my-repo.yaml --dry-run

  # Apply configuration for a specific organization
  synacklab github apply my-repo.yaml --owner myorg`,
	Args: cobra.ExactArgs(1),
	RunE: runGitHubApply,
}

func init() {
	githubApplyCmd.Flags().BoolVar(&githubDryRun, "dry-run", false, "Preview changes without applying them")
	githubApplyCmd.Flags().StringVar(&githubOwner, "owner", "", "Repository owner (organization or user)")
	githubCmd.AddCommand(githubApplyCmd)
}

func runGitHubApply(cmd *cobra.Command, args []string) error {
	configFile := args[0]

	// Load synacklab configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load synacklab config: %w", err)
	}

	// Load repository configuration from file
	repoConfig, err := github.LoadRepositoryConfigFromFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to load repository config: %w", err)
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
	authManager := github.NewAuthManager()
	tokenInfo, err := authManager.AuthenticateFromConfig(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Authentication failed: %v\n\n", err)
		fmt.Fprintf(os.Stderr, "%s\n", github.GetAuthInstructions())
		return err
	}

	fmt.Printf("✓ Authenticated as %s\n", tokenInfo.User)

	// Create GitHub client and reconciler
	token, err := authManager.GetToken(cfg)
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}

	client := github.NewClient(token)
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

// displayPlan shows the planned changes in a human-readable format
func displayPlan(plan *github.ReconciliationPlan, owner, repoName string, isDryRun bool) error {
	if isDryRun {
		fmt.Printf("\n🔍 Dry-run mode: Showing planned changes for %s/%s\n", owner, repoName)
	} else {
		fmt.Printf("\n📋 Planned changes for %s/%s:\n", owner, repoName)
	}

	changeCount := 0
	destructiveChanges := 0

	// Repository changes
	if plan.Repository != nil {
		changeCount++
		switch plan.Repository.Type {
		case github.ChangeTypeCreate:
			fmt.Printf("  + Repository: CREATE new repository\n")
			fmt.Printf("    - Name: %s\n", plan.Repository.After.Name)
			fmt.Printf("    - Description: %s\n", plan.Repository.After.Description)
			fmt.Printf("    - Private: %t\n", plan.Repository.After.Private)
			if len(plan.Repository.After.Topics) > 0 {
				fmt.Printf("    - Topics: %s\n", strings.Join(plan.Repository.After.Topics, ", "))
			}
		case github.ChangeTypeUpdate:
			fmt.Printf("  ~ Repository: UPDATE repository settings\n")
			if plan.Repository.Before.Description != plan.Repository.After.Description {
				fmt.Printf("    ~ Description: %q → %q\n", plan.Repository.Before.Description, plan.Repository.After.Description)
			}
			if plan.Repository.Before.Private != plan.Repository.After.Private {
				// Highlight making repository public as potentially destructive
				if plan.Repository.Before.Private && !plan.Repository.After.Private {
					fmt.Printf("    ⚠️  Private: %t → %t (MAKING REPOSITORY PUBLIC)\n", plan.Repository.Before.Private, plan.Repository.After.Private)
					destructiveChanges++
				} else {
					fmt.Printf("    ~ Private: %t → %t\n", plan.Repository.Before.Private, plan.Repository.After.Private)
				}
			}
			if !stringSlicesEqual(plan.Repository.Before.Topics, plan.Repository.After.Topics) {
				fmt.Printf("    ~ Topics: [%s] → [%s]\n",
					strings.Join(plan.Repository.Before.Topics, ", "),
					strings.Join(plan.Repository.After.Topics, ", "))
			}
		}
	}

	// Branch protection changes
	for _, change := range plan.BranchRules {
		changeCount++
		switch change.Type {
		case github.ChangeTypeCreate:
			fmt.Printf("  + Branch Protection: CREATE rule for %s\n", change.Branch)
			displayBranchProtectionDetails(change.After, "    ")
		case github.ChangeTypeUpdate:
			fmt.Printf("  ~ Branch Protection: UPDATE rule for %s\n", change.Branch)
			displayBranchProtectionChanges(change.Before, change.After, "    ")
		case github.ChangeTypeDelete:
			fmt.Printf("  ⚠️  Branch Protection: DELETE rule for %s (REMOVING PROTECTION)\n", change.Branch)
			destructiveChanges++
		}
	}

	// Collaborator changes
	for _, change := range plan.Collaborators {
		changeCount++
		switch change.Type {
		case github.ChangeTypeCreate:
			fmt.Printf("  + Collaborator: ADD %s with %s permission\n", change.After.Username, change.After.Permission)
		case github.ChangeTypeUpdate:
			// Highlight permission downgrades as potentially destructive
			if isPermissionDowngrade(change.Before.Permission, change.After.Permission) {
				fmt.Printf("  ⚠️  Collaborator: UPDATE %s permission %s → %s (REDUCING ACCESS)\n",
					change.After.Username, change.Before.Permission, change.After.Permission)
				destructiveChanges++
			} else {
				fmt.Printf("  ~ Collaborator: UPDATE %s permission %s → %s\n",
					change.After.Username, change.Before.Permission, change.After.Permission)
			}
		case github.ChangeTypeDelete:
			fmt.Printf("  ⚠️  Collaborator: REMOVE %s (REMOVING ACCESS)\n", change.Before.Username)
			destructiveChanges++
		}
	}

	// Team changes
	for _, change := range plan.Teams {
		changeCount++
		switch change.Type {
		case github.ChangeTypeCreate:
			fmt.Printf("  + Team: ADD %s with %s permission\n", change.After.TeamSlug, change.After.Permission)
		case github.ChangeTypeUpdate:
			// Highlight permission downgrades as potentially destructive
			if isPermissionDowngrade(change.Before.Permission, change.After.Permission) {
				fmt.Printf("  ⚠️  Team: UPDATE %s permission %s → %s (REDUCING ACCESS)\n",
					change.After.TeamSlug, change.Before.Permission, change.After.Permission)
				destructiveChanges++
			} else {
				fmt.Printf("  ~ Team: UPDATE %s permission %s → %s\n",
					change.After.TeamSlug, change.Before.Permission, change.After.Permission)
			}
		case github.ChangeTypeDelete:
			fmt.Printf("  ⚠️  Team: REMOVE %s (REMOVING ACCESS)\n", change.Before.TeamSlug)
			destructiveChanges++
		}
	}

	// Webhook changes
	for _, change := range plan.Webhooks {
		changeCount++
		switch change.Type {
		case github.ChangeTypeCreate:
			fmt.Printf("  + Webhook: CREATE %s\n", change.After.URL)
			fmt.Printf("    - Events: %s\n", strings.Join(change.After.Events, ", "))
			fmt.Printf("    - Active: %t\n", change.After.Active)
		case github.ChangeTypeUpdate:
			fmt.Printf("  ~ Webhook: UPDATE %s\n", change.After.URL)
			if !stringSlicesEqual(change.Before.Events, change.After.Events) {
				fmt.Printf("    ~ Events: [%s] → [%s]\n",
					strings.Join(change.Before.Events, ", "),
					strings.Join(change.After.Events, ", "))
			}
			if change.Before.Active != change.After.Active {
				fmt.Printf("    ~ Active: %t → %t\n", change.Before.Active, change.After.Active)
			}
		case github.ChangeTypeDelete:
			fmt.Printf("  ⚠️  Webhook: DELETE %s (REMOVING WEBHOOK)\n", change.Before.URL)
			destructiveChanges++
		}
	}

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
}

// displayBranchProtectionChanges shows changes between two branch protection rules
func displayBranchProtectionChanges(before, after *github.BranchProtection, indent string) {
	if before.RequiredReviews != after.RequiredReviews {
		if before.RequiredReviews > after.RequiredReviews {
			fmt.Printf("%s⚠️  Required reviews: %d → %d (REDUCING PROTECTION)\n", indent, before.RequiredReviews, after.RequiredReviews)
		} else {
			fmt.Printf("%s~ Required reviews: %d → %d\n", indent, before.RequiredReviews, after.RequiredReviews)
		}
	}
	if before.DismissStaleReviews != after.DismissStaleReviews {
		if before.DismissStaleReviews && !after.DismissStaleReviews {
			fmt.Printf("%s⚠️  Dismiss stale reviews: %t → %t (REDUCING PROTECTION)\n", indent, before.DismissStaleReviews, after.DismissStaleReviews)
		} else {
			fmt.Printf("%s~ Dismiss stale reviews: %t → %t\n", indent, before.DismissStaleReviews, after.DismissStaleReviews)
		}
	}
	if before.RequireCodeOwnerReview != after.RequireCodeOwnerReview {
		if before.RequireCodeOwnerReview && !after.RequireCodeOwnerReview {
			fmt.Printf("%s⚠️  Require code owner review: %t → %t (REDUCING PROTECTION)\n", indent, before.RequireCodeOwnerReview, after.RequireCodeOwnerReview)
		} else {
			fmt.Printf("%s~ Require code owner review: %t → %t\n", indent, before.RequireCodeOwnerReview, after.RequireCodeOwnerReview)
		}
	}
	if !stringSlicesEqual(before.RequiredStatusChecks, after.RequiredStatusChecks) {
		if len(before.RequiredStatusChecks) > len(after.RequiredStatusChecks) {
			fmt.Printf("%s⚠️  Required status checks: [%s] → [%s] (REDUCING PROTECTION)\n", indent,
				strings.Join(before.RequiredStatusChecks, ", "),
				strings.Join(after.RequiredStatusChecks, ", "))
		} else {
			fmt.Printf("%s~ Required status checks: [%s] → [%s]\n", indent,
				strings.Join(before.RequiredStatusChecks, ", "),
				strings.Join(after.RequiredStatusChecks, ", "))
		}
	}
	if before.RequireUpToDate != after.RequireUpToDate {
		if before.RequireUpToDate && !after.RequireUpToDate {
			fmt.Printf("%s⚠️  Require up-to-date branches: %t → %t (REDUCING PROTECTION)\n", indent, before.RequireUpToDate, after.RequireUpToDate)
		} else {
			fmt.Printf("%s~ Require up-to-date branches: %t → %t\n", indent, before.RequireUpToDate, after.RequireUpToDate)
		}
	}
	if !stringSlicesEqual(before.RestrictPushes, after.RestrictPushes) {
		if len(before.RestrictPushes) > len(after.RestrictPushes) {
			fmt.Printf("%s⚠️  Restrict pushes: [%s] → [%s] (REDUCING PROTECTION)\n", indent,
				strings.Join(before.RestrictPushes, ", "),
				strings.Join(after.RestrictPushes, ", "))
		} else {
			fmt.Printf("%s~ Restrict pushes: [%s] → [%s]\n", indent,
				strings.Join(before.RestrictPushes, ", "),
				strings.Join(after.RestrictPushes, ", "))
		}
	}
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
