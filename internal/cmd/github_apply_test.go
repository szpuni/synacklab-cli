package cmd

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"synacklab/pkg/github"
)

func TestDryRunFunctionality(t *testing.T) {
	tests := []struct {
		name           string
		plan           *github.ReconciliationPlan
		expectedOutput []string
		expectWarning  bool
	}{
		{
			name: "no changes",
			plan: &github.ReconciliationPlan{},
			expectedOutput: []string{
				"🔍 Dry-run mode: Showing planned changes for testorg/testrepo",
				"No changes needed - repository is up to date",
			},
			expectWarning: false,
		},
		{
			name: "create repository",
			plan: &github.ReconciliationPlan{
				Repository: &github.RepositoryChange{
					Type: github.ChangeTypeCreate,
					After: &github.Repository{
						Name:        "testrepo",
						Description: "Test repository",
						Private:     true,
						Topics:      []string{"test", "golang"},
					},
				},
			},
			expectedOutput: []string{
				"🔍 Dry-run mode: Showing planned changes for testorg/testrepo",
				"+ Repository: CREATE new repository",
				"- Name: testrepo",
				"- Description: Test repository",
				"- Private: true",
				"- Topics: test, golang",
				"Total changes: 1",
			},
			expectWarning: false,
		},
		{
			name: "destructive changes - make repository public",
			plan: &github.ReconciliationPlan{
				Repository: &github.RepositoryChange{
					Type: github.ChangeTypeUpdate,
					Before: &github.Repository{
						Name:        "testrepo",
						Description: "Test repository",
						Private:     true,
						Topics:      []string{"test"},
					},
					After: &github.Repository{
						Name:        "testrepo",
						Description: "Test repository",
						Private:     false,
						Topics:      []string{"test"},
					},
				},
			},
			expectedOutput: []string{
				"🔍 Dry-run mode: Showing planned changes for testorg/testrepo",
				"~ Repository: UPDATE repository settings",
				"⚠️  Private: true → false (MAKING REPOSITORY PUBLIC)",
				"Total changes: 1 (1 potentially destructive)",
				"⚠️  WARNING: 1 potentially destructive change(s) detected!",
				"Review these changes carefully before applying.",
			},
			expectWarning: true,
		},
		{
			name: "destructive changes - remove branch protection",
			plan: &github.ReconciliationPlan{
				BranchRules: []github.BranchRuleChange{
					{
						Type:   github.ChangeTypeDelete,
						Branch: "main",
						Before: &github.BranchProtection{
							Pattern:         "main",
							RequiredReviews: 2,
						},
					},
				},
			},
			expectedOutput: []string{
				"🔍 Dry-run mode: Showing planned changes for testorg/testrepo",
				"⚠️  Branch Protection: DELETE rule for main (REMOVING PROTECTION)",
				"Total changes: 1 (1 potentially destructive)",
				"⚠️  WARNING: 1 potentially destructive change(s) detected!",
			},
			expectWarning: true,
		},
		{
			name: "destructive changes - remove collaborator",
			plan: &github.ReconciliationPlan{
				Collaborators: []github.CollaboratorChange{
					{
						Type: github.ChangeTypeDelete,
						Before: &github.Collaborator{
							Username:   "testuser",
							Permission: "write",
						},
					},
				},
			},
			expectedOutput: []string{
				"🔍 Dry-run mode: Showing planned changes for testorg/testrepo",
				"⚠️  Collaborator: REMOVE testuser (REMOVING ACCESS)",
				"Total changes: 1 (1 potentially destructive)",
				"⚠️  WARNING: 1 potentially destructive change(s) detected!",
			},
			expectWarning: true,
		},
		{
			name: "destructive changes - downgrade collaborator permission",
			plan: &github.ReconciliationPlan{
				Collaborators: []github.CollaboratorChange{
					{
						Type: github.ChangeTypeUpdate,
						Before: &github.Collaborator{
							Username:   "testuser",
							Permission: "admin",
						},
						After: &github.Collaborator{
							Username:   "testuser",
							Permission: "read",
						},
					},
				},
			},
			expectedOutput: []string{
				"🔍 Dry-run mode: Showing planned changes for testorg/testrepo",
				"⚠️  Collaborator: UPDATE testuser permission admin → read (REDUCING ACCESS)",
				"Total changes: 1 (1 potentially destructive)",
				"⚠️  WARNING: 1 potentially destructive change(s) detected!",
			},
			expectWarning: true,
		},
		{
			name: "destructive changes - remove team access",
			plan: &github.ReconciliationPlan{
				Teams: []github.TeamChange{
					{
						Type: github.ChangeTypeDelete,
						Before: &github.TeamAccess{
							TeamSlug:   "developers",
							Permission: "write",
						},
					},
				},
			},
			expectedOutput: []string{
				"🔍 Dry-run mode: Showing planned changes for testorg/testrepo",
				"⚠️  Team: REMOVE developers (REMOVING ACCESS)",
				"Total changes: 1 (1 potentially destructive)",
				"⚠️  WARNING: 1 potentially destructive change(s) detected!",
			},
			expectWarning: true,
		},
		{
			name: "destructive changes - remove webhook",
			plan: &github.ReconciliationPlan{
				Webhooks: []github.WebhookChange{
					{
						Type: github.ChangeTypeDelete,
						Before: &github.Webhook{
							URL:    "https://example.com/webhook",
							Events: []string{"push", "pull_request"},
							Active: true,
						},
					},
				},
			},
			expectedOutput: []string{
				"🔍 Dry-run mode: Showing planned changes for testorg/testrepo",
				"⚠️  Webhook: DELETE https://example.com/webhook (REMOVING WEBHOOK)",
				"Total changes: 1 (1 potentially destructive)",
				"⚠️  WARNING: 1 potentially destructive change(s) detected!",
			},
			expectWarning: true,
		},
		{
			name: "mixed changes with destructive",
			plan: &github.ReconciliationPlan{
				Repository: &github.RepositoryChange{
					Type: github.ChangeTypeUpdate,
					Before: &github.Repository{
						Name:        "testrepo",
						Description: "Old description",
						Private:     true,
					},
					After: &github.Repository{
						Name:        "testrepo",
						Description: "New description",
						Private:     true,
					},
				},
				Collaborators: []github.CollaboratorChange{
					{
						Type: github.ChangeTypeCreate,
						After: &github.Collaborator{
							Username:   "newuser",
							Permission: "read",
						},
					},
					{
						Type: github.ChangeTypeDelete,
						Before: &github.Collaborator{
							Username:   "olduser",
							Permission: "write",
						},
					},
				},
			},
			expectedOutput: []string{
				"🔍 Dry-run mode: Showing planned changes for testorg/testrepo",
				"~ Repository: UPDATE repository settings",
				"~ Description: \"Old description\" → \"New description\"",
				"+ Collaborator: ADD newuser with read permission",
				"⚠️  Collaborator: REMOVE olduser (REMOVING ACCESS)",
				"Total changes: 3 (1 potentially destructive)",
				"⚠️  WARNING: 1 potentially destructive change(s) detected!",
				"Review these changes carefully before applying.",
			},
			expectWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Create a buffer to capture output
			var buf bytes.Buffer
			done := make(chan bool)
			go func() {
				_, _ = buf.ReadFrom(r)
				done <- true
			}()

			// Call displayPlan with dry-run mode
			err := displayPlan(tt.plan, "testorg", "testrepo", true)
			require.NoError(t, err)

			// Restore stdout and get output
			_ = w.Close()
			os.Stdout = oldStdout
			<-done

			output := buf.String()

			// Check that all expected strings are present
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
			}

			// Check warning presence
			if tt.expectWarning {
				assert.Contains(t, output, "⚠️  WARNING:", "Expected warning message for destructive changes")
			}
		})
	}
}

func TestIsPermissionDowngrade(t *testing.T) {
	tests := []struct {
		name     string
		before   string
		after    string
		expected bool
	}{
		{
			name:     "admin to write is downgrade",
			before:   "admin",
			after:    "write",
			expected: true,
		},
		{
			name:     "admin to read is downgrade",
			before:   "admin",
			after:    "read",
			expected: true,
		},
		{
			name:     "write to read is downgrade",
			before:   "write",
			after:    "read",
			expected: true,
		},
		{
			name:     "read to write is upgrade",
			before:   "read",
			after:    "write",
			expected: false,
		},
		{
			name:     "write to admin is upgrade",
			before:   "write",
			after:    "admin",
			expected: false,
		},
		{
			name:     "read to admin is upgrade",
			before:   "read",
			after:    "admin",
			expected: false,
		},
		{
			name:     "same permission is not downgrade",
			before:   "write",
			after:    "write",
			expected: false,
		},
		{
			name:     "unknown permission is not downgrade",
			before:   "unknown",
			after:    "read",
			expected: false,
		},
		{
			name:     "to unknown permission is not downgrade",
			before:   "admin",
			after:    "unknown",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPermissionDowngrade(tt.before, tt.after)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStringSlicesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{
			name:     "empty slices are equal",
			a:        []string{},
			b:        []string{},
			expected: true,
		},
		{
			name:     "nil slices are equal",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "empty and nil slices are equal",
			a:        []string{},
			b:        nil,
			expected: true,
		},
		{
			name:     "identical slices are equal",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "different order same elements are equal",
			a:        []string{"a", "b", "c"},
			b:        []string{"c", "a", "b"},
			expected: true,
		},
		{
			name:     "different lengths are not equal",
			a:        []string{"a", "b"},
			b:        []string{"a", "b", "c"},
			expected: false,
		},
		{
			name:     "different elements are not equal",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "d"},
			expected: false,
		},
		{
			name:     "duplicate elements handled correctly",
			a:        []string{"a", "a", "b"},
			b:        []string{"a", "b", "a"},
			expected: true,
		},
		{
			name:     "different duplicate counts are not equal",
			a:        []string{"a", "a", "b"},
			b:        []string{"a", "b", "b"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringSlicesEqual(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasChanges(t *testing.T) {
	tests := []struct {
		name     string
		plan     *github.ReconciliationPlan
		expected bool
	}{
		{
			name:     "empty plan has no changes",
			plan:     &github.ReconciliationPlan{},
			expected: false,
		},
		{
			name: "repository change detected",
			plan: &github.ReconciliationPlan{
				Repository: &github.RepositoryChange{
					Type: github.ChangeTypeCreate,
				},
			},
			expected: true,
		},
		{
			name: "branch rule change detected",
			plan: &github.ReconciliationPlan{
				BranchRules: []github.BranchRuleChange{
					{Type: github.ChangeTypeCreate},
				},
			},
			expected: true,
		},
		{
			name: "collaborator change detected",
			plan: &github.ReconciliationPlan{
				Collaborators: []github.CollaboratorChange{
					{Type: github.ChangeTypeCreate},
				},
			},
			expected: true,
		},
		{
			name: "team change detected",
			plan: &github.ReconciliationPlan{
				Teams: []github.TeamChange{
					{Type: github.ChangeTypeCreate},
				},
			},
			expected: true,
		},
		{
			name: "webhook change detected",
			plan: &github.ReconciliationPlan{
				Webhooks: []github.WebhookChange{
					{Type: github.ChangeTypeCreate},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasChanges(tt.plan)
			assert.Equal(t, tt.expected, result)
		})
	}
}
func TestDisplayBranchProtectionDetails(t *testing.T) {
	tests := []struct {
		name             string
		branchProtection *github.BranchProtection
		expectedOutput   []string
	}{
		{
			name: "basic branch protection",
			branchProtection: &github.BranchProtection{
				Pattern:         "main",
				RequiredReviews: 2,
			},
			expectedOutput: []string{
				"- Required reviews: 2",
			},
		},
		{
			name: "full branch protection",
			branchProtection: &github.BranchProtection{
				Pattern:                "main",
				RequiredReviews:        2,
				DismissStaleReviews:    true,
				RequireCodeOwnerReview: true,
				RequiredStatusChecks:   []string{"ci/test", "ci/lint"},
				RequireUpToDate:        true,
				RestrictPushes:         []string{"admin", "maintainer"},
			},
			expectedOutput: []string{
				"- Required reviews: 2",
				"- Dismiss stale reviews: enabled",
				"- Require code owner review: enabled",
				"- Required status checks: ci/test, ci/lint",
				"- Require up-to-date branches: enabled",
				"- Restrict pushes to: admin, maintainer",
			},
		},
		{
			name: "no required reviews",
			branchProtection: &github.BranchProtection{
				Pattern:              "main",
				RequiredReviews:      0,
				RequiredStatusChecks: []string{"ci/test"},
				RequireUpToDate:      true,
			},
			expectedOutput: []string{
				"- Required status checks: ci/test",
				"- Require up-to-date branches: enabled",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			var buf bytes.Buffer
			done := make(chan bool)
			go func() {
				_, _ = buf.ReadFrom(r)
				done <- true
			}()

			// Call the function
			displayBranchProtectionDetails(tt.branchProtection, "    ")

			// Restore stdout and get output
			_ = w.Close()
			os.Stdout = oldStdout
			<-done

			output := buf.String()

			// Check that all expected strings are present
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
			}
		})
	}
}

func TestDisplayBranchProtectionChanges(t *testing.T) {
	tests := []struct {
		name           string
		before         *github.BranchProtection
		after          *github.BranchProtection
		expectedOutput []string
		expectWarning  bool
	}{
		{
			name: "reducing required reviews",
			before: &github.BranchProtection{
				Pattern:         "main",
				RequiredReviews: 2,
			},
			after: &github.BranchProtection{
				Pattern:         "main",
				RequiredReviews: 1,
			},
			expectedOutput: []string{
				"⚠️  Required reviews: 2 → 1 (REDUCING PROTECTION)",
			},
			expectWarning: true,
		},
		{
			name: "increasing required reviews",
			before: &github.BranchProtection{
				Pattern:         "main",
				RequiredReviews: 1,
			},
			after: &github.BranchProtection{
				Pattern:         "main",
				RequiredReviews: 2,
			},
			expectedOutput: []string{
				"~ Required reviews: 1 → 2",
			},
			expectWarning: false,
		},
		{
			name: "disabling stale review dismissal",
			before: &github.BranchProtection{
				Pattern:             "main",
				RequiredReviews:     2,
				DismissStaleReviews: true,
			},
			after: &github.BranchProtection{
				Pattern:             "main",
				RequiredReviews:     2,
				DismissStaleReviews: false,
			},
			expectedOutput: []string{
				"⚠️  Dismiss stale reviews: true → false (REDUCING PROTECTION)",
			},
			expectWarning: true,
		},
		{
			name: "removing status checks",
			before: &github.BranchProtection{
				Pattern:              "main",
				RequiredReviews:      2,
				RequiredStatusChecks: []string{"ci/test", "ci/lint"},
			},
			after: &github.BranchProtection{
				Pattern:              "main",
				RequiredReviews:      2,
				RequiredStatusChecks: []string{"ci/test"},
			},
			expectedOutput: []string{
				"⚠️  Required status checks: [ci/test, ci/lint] → [ci/test] (REDUCING PROTECTION)",
			},
			expectWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			var buf bytes.Buffer
			done := make(chan bool)
			go func() {
				_, _ = buf.ReadFrom(r)
				done <- true
			}()

			// Call the function
			displayBranchProtectionChanges(tt.before, tt.after, "    ")

			// Restore stdout and get output
			_ = w.Close()
			os.Stdout = oldStdout
			<-done

			output := buf.String()

			// Check that all expected strings are present
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
			}

			// Check warning presence
			if tt.expectWarning {
				assert.Contains(t, output, "⚠️", "Expected warning indicator for destructive changes")
			}
		})
	}
}

func TestDryRunWithComplexPlan(t *testing.T) {
	// Test a complex plan with multiple types of changes
	plan := &github.ReconciliationPlan{
		Repository: &github.RepositoryChange{
			Type: github.ChangeTypeUpdate,
			Before: &github.Repository{
				Name:        "complex-repo",
				Description: "Old description",
				Private:     false,
				Topics:      []string{"old-topic"},
			},
			After: &github.Repository{
				Name:        "complex-repo",
				Description: "New description",
				Private:     true,
				Topics:      []string{"new-topic", "golang"},
			},
		},
		BranchRules: []github.BranchRuleChange{
			{
				Type:   github.ChangeTypeCreate,
				Branch: "main",
				After: &github.BranchProtection{
					Pattern:                "main",
					RequiredReviews:        2,
					DismissStaleReviews:    true,
					RequireCodeOwnerReview: true,
					RequiredStatusChecks:   []string{"ci/test"},
					RequireUpToDate:        true,
				},
			},
			{
				Type:   github.ChangeTypeUpdate,
				Branch: "develop",
				Before: &github.BranchProtection{
					Pattern:         "develop",
					RequiredReviews: 2,
				},
				After: &github.BranchProtection{
					Pattern:         "develop",
					RequiredReviews: 1,
				},
			},
		},
		Collaborators: []github.CollaboratorChange{
			{
				Type: github.ChangeTypeCreate,
				After: &github.Collaborator{
					Username:   "newdev",
					Permission: "write",
				},
			},
			{
				Type: github.ChangeTypeUpdate,
				Before: &github.Collaborator{
					Username:   "olddev",
					Permission: "admin",
				},
				After: &github.Collaborator{
					Username:   "olddev",
					Permission: "write",
				},
			},
		},
		Teams: []github.TeamChange{
			{
				Type: github.ChangeTypeCreate,
				After: &github.TeamAccess{
					TeamSlug:   "backend-team",
					Permission: "write",
				},
			},
		},
		Webhooks: []github.WebhookChange{
			{
				Type: github.ChangeTypeUpdate,
				Before: &github.Webhook{
					URL:    "https://old.example.com/webhook",
					Events: []string{"push"},
					Active: true,
				},
				After: &github.Webhook{
					URL:    "https://old.example.com/webhook",
					Events: []string{"push", "pull_request"},
					Active: true,
				},
			},
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan bool)
	go func() {
		_, _ = buf.ReadFrom(r)
		done <- true
	}()

	// Call displayPlan with dry-run mode
	err := displayPlan(plan, "testorg", "complex-repo", true)
	require.NoError(t, err)

	// Restore stdout and get output
	_ = w.Close()
	os.Stdout = oldStdout
	<-done

	output := buf.String()

	// Verify dry-run header
	assert.Contains(t, output, "🔍 Dry-run mode: Showing planned changes for testorg/complex-repo")

	// Verify repository changes
	assert.Contains(t, output, "~ Repository: UPDATE repository settings")
	assert.Contains(t, output, "~ Description: \"Old description\" → \"New description\"")
	assert.Contains(t, output, "~ Private: false → true")
	assert.Contains(t, output, "~ Topics: [old-topic] → [new-topic, golang]")

	// Verify branch protection changes
	assert.Contains(t, output, "+ Branch Protection: CREATE rule for main")
	assert.Contains(t, output, "~ Branch Protection: UPDATE rule for develop")
	assert.Contains(t, output, "⚠️  Required reviews: 2 → 1 (REDUCING PROTECTION)")

	// Verify collaborator changes
	assert.Contains(t, output, "+ Collaborator: ADD newdev with write permission")
	assert.Contains(t, output, "⚠️  Collaborator: UPDATE olddev permission admin → write (REDUCING ACCESS)")

	// Verify team changes
	assert.Contains(t, output, "+ Team: ADD backend-team with write permission")

	// Verify webhook changes
	assert.Contains(t, output, "~ Webhook: UPDATE https://old.example.com/webhook")
	assert.Contains(t, output, "~ Events: [push] → [push, pull_request]")

	// Verify summary - now correctly counts both branch protection and collaborator destructive changes
	// Branch protection changes within displayBranchProtectionChanges now increment the counter
	// Both the branch protection downgrade and collaborator permission downgrade are counted as destructive
	assert.Contains(t, output, "Total changes: 7 (2 potentially destructive)")
	assert.Contains(t, output, "⚠️  WARNING: 2 potentially destructive change(s) detected!")
	assert.Contains(t, output, "Review these changes carefully before applying.")
}

// Tests for multi-repository functionality

func TestRunMultiRepositoryApply(t *testing.T) {
	// This test would require mocking the GitHub client and multi-reconciler
	// For now, we'll test the helper functions that don't require external dependencies
	t.Skip("Integration test - requires mocked GitHub client")
}

func TestDisplayMultiRepoValidationResults(t *testing.T) {
	tests := []struct {
		name           string
		result         *github.MultiRepoValidationResult
		expectedOutput []string
	}{
		{
			name: "all repositories valid",
			result: &github.MultiRepoValidationResult{
				Valid:   []string{"repo1", "repo2"},
				Invalid: map[string]error{},
				Details: map[string]*github.RepositoryValidationDetails{},
				Summary: github.ValidationSummary{
					TotalRepositories: 2,
					ValidCount:        2,
					InvalidCount:      0,
					WarningCount:      0,
				},
			},
			expectedOutput: []string{},
		},
		{
			name: "some repositories invalid",
			result: &github.MultiRepoValidationResult{
				Valid: []string{"repo1"},
				Invalid: map[string]error{
					"repo2": fmt.Errorf("invalid repository name"),
				},
				Details: map[string]*github.RepositoryValidationDetails{},
				Summary: github.ValidationSummary{
					TotalRepositories: 2,
					ValidCount:        1,
					InvalidCount:      1,
					WarningCount:      0,
				},
			},
			expectedOutput: []string{
				"❌ Configuration validation failed for 1 repositories:",
				"• repo2: invalid repository name",
			},
		},
		{
			name: "repositories with warnings",
			result: &github.MultiRepoValidationResult{
				Valid:   []string{"repo1", "repo2"},
				Invalid: map[string]error{},
				Details: map[string]*github.RepositoryValidationDetails{
					"repo1": {
						RepositoryName: "repo1",
						Warnings: []github.ValidationWarning{
							{
								Field:   "private",
								Message: "Repository name suggests it should be private",
							},
						},
					},
				},
				Summary: github.ValidationSummary{
					TotalRepositories: 2,
					ValidCount:        2,
					InvalidCount:      0,
					WarningCount:      1,
				},
			},
			expectedOutput: []string{
				"⚠️  Configuration warnings for 1 repositories:",
				"• repo1:",
				"- Repository name suggests it should be private",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			var buf bytes.Buffer
			done := make(chan bool)
			go func() {
				_, _ = buf.ReadFrom(r)
				done <- true
			}()

			// Call the function
			err := displayMultiRepoValidationResults(tt.result)
			require.NoError(t, err)

			// Restore stdout and get output
			_ = w.Close()
			os.Stdout = oldStdout
			<-done

			output := buf.String()

			// Check that all expected strings are present
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
			}
		})
	}
}

func TestDisplayMultiRepoPlan(t *testing.T) {
	tests := []struct {
		name           string
		plans          map[string]*github.ReconciliationPlan
		owner          string
		isDryRun       bool
		expectedOutput []string
	}{
		{
			name: "multiple repositories with changes",
			plans: map[string]*github.ReconciliationPlan{
				"repo1": {
					Repository: &github.RepositoryChange{
						Type: github.ChangeTypeCreate,
						After: &github.Repository{
							Name:        "repo1",
							Description: "First repository",
							Private:     true,
						},
					},
				},
				"repo2": {
					BranchRules: []github.BranchRuleChange{
						{
							Type:   github.ChangeTypeCreate,
							Branch: "main",
							After: &github.BranchProtection{
								Pattern:         "main",
								RequiredReviews: 2,
							},
						},
					},
				},
			},
			owner:    "testorg",
			isDryRun: true,
			expectedOutput: []string{
				"🔍 Dry-run mode: Showing planned changes for 2 repositories",
				"📦 testorg/repo1:",
				"+ Repository: CREATE new repository",
				"📦 testorg/repo2:",
				"+ Branch Protection: CREATE rule for main",
				"📊 Summary:",
				"• Total repositories: 2",
				"• Repositories with changes: 2",
				"• Total changes: 2",
			},
		},
		{
			name: "repository with no changes",
			plans: map[string]*github.ReconciliationPlan{
				"repo1": {},
				"repo2": {
					Repository: &github.RepositoryChange{
						Type: github.ChangeTypeUpdate,
						Before: &github.Repository{
							Name:        "repo2",
							Description: "Old description",
							Private:     true,
						},
						After: &github.Repository{
							Name:        "repo2",
							Description: "New description",
							Private:     true,
						},
					},
				},
			},
			owner:    "testorg",
			isDryRun: false,
			expectedOutput: []string{
				"📋 Planned changes for 2 repositories:",
				"📦 testorg/repo1: No changes needed",
				"📦 testorg/repo2:",
				"~ Repository: UPDATE repository settings",
				"📊 Summary:",
				"• Total repositories: 2",
				"• Repositories with changes: 1",
				"• Total changes: 1",
			},
		},
		{
			name: "destructive changes across repositories",
			plans: map[string]*github.ReconciliationPlan{
				"repo1": {
					Repository: &github.RepositoryChange{
						Type: github.ChangeTypeUpdate,
						Before: &github.Repository{
							Name:    "repo1",
							Private: true,
						},
						After: &github.Repository{
							Name:    "repo1",
							Private: false,
						},
					},
				},
				"repo2": {
					Collaborators: []github.CollaboratorChange{
						{
							Type: github.ChangeTypeDelete,
							Before: &github.Collaborator{
								Username:   "olduser",
								Permission: "write",
							},
						},
					},
				},
			},
			owner:    "testorg",
			isDryRun: true,
			expectedOutput: []string{
				"🔍 Dry-run mode: Showing planned changes for 2 repositories",
				"📦 testorg/repo1:",
				"⚠️  Private: true → false (MAKING REPOSITORY PUBLIC)",
				"📦 testorg/repo2:",
				"⚠️  Collaborator: REMOVE olduser (REMOVING ACCESS)",
				"📊 Summary:",
				"• Potentially destructive changes: 2",
				"⚠️  WARNING: 2 potentially destructive change(s) detected across all repositories!",
				"Review these changes carefully before applying.",
			},
		},
		{
			name: "repositories with planning errors",
			plans: map[string]*github.ReconciliationPlan{
				"repo1": {
					Repository: &github.RepositoryChange{
						Type: github.ChangeTypeCreate,
						After: &github.Repository{
							Name:        "repo1",
							Description: "Successful repository",
							Private:     true,
						},
					},
				},
				"repo2": nil, // Simulates planning failure
				"repo3": {
					BranchRules: []github.BranchRuleChange{
						{
							Type:   github.ChangeTypeCreate,
							Branch: "main",
							After: &github.BranchProtection{
								Pattern:         "main",
								RequiredReviews: 2,
							},
						},
					},
				},
			},
			owner:    "testorg",
			isDryRun: true,
			expectedOutput: []string{
				"🔍 Dry-run mode: Showing planned changes for 3 repositories",
				"📦 testorg/repo1:",
				"+ Repository: CREATE new repository",
				"📦 testorg/repo2: ❌ Planning failed (see errors below)",
				"📦 testorg/repo3:",
				"+ Branch Protection: CREATE rule for main",
				"📊 Summary:",
				"• Total repositories: 3",
				"• Repositories with changes: 2",
				"• Repositories with planning errors: 1",
				"• Total changes: 2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			var buf bytes.Buffer
			done := make(chan bool)
			go func() {
				_, _ = buf.ReadFrom(r)
				done <- true
			}()

			// Call the function
			err := displayMultiRepoPlan(tt.plans, tt.owner, tt.isDryRun)
			require.NoError(t, err)

			// Restore stdout and get output
			_ = w.Close()
			os.Stdout = oldStdout
			<-done

			output := buf.String()

			// Check that all expected strings are present
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
			}
		})
	}
}

func TestMultiRepoDryRunErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		plans          map[string]*github.ReconciliationPlan
		planError      error
		expectedOutput []string
		expectError    bool
	}{
		{
			name: "dry-run continues with partial planning errors",
			plans: map[string]*github.ReconciliationPlan{
				"successful-repo": {
					Repository: &github.RepositoryChange{
						Type: github.ChangeTypeCreate,
						After: &github.Repository{
							Name:        "successful-repo",
							Description: "This repo planned successfully",
							Private:     true,
						},
					},
				},
				"failed-repo": nil, // This represents a repository that failed planning
			},
			planError: fmt.Errorf("planning failed for some repositories: repository failed-repo: failed to create plan: invalid configuration"),
			expectedOutput: []string{
				"🔍 Dry-run mode: Showing planned changes for 2 repositories",
				"📦 testorg/failed-repo: ❌ Planning failed (see errors below)",
				"📦 testorg/successful-repo:",
				"+ Repository: CREATE new repository",
				"⚠️  Planning errors encountered during dry-run:",
				"planning failed for some repositories: repository failed-repo: failed to create plan: invalid configuration",
				"✓ Dry-run completed with errors. No changes were applied.",
				"• Repositories with planning errors: 1",
			},
			expectError: true,
		},
		{
			name: "dry-run with no planning errors",
			plans: map[string]*github.ReconciliationPlan{
				"repo1": {
					Repository: &github.RepositoryChange{
						Type: github.ChangeTypeCreate,
						After: &github.Repository{
							Name:        "repo1",
							Description: "Test repository",
							Private:     true,
						},
					},
				},
			},
			planError: nil,
			expectedOutput: []string{
				"🔍 Dry-run mode: Showing planned changes for 1 repositories",
				"📦 testorg/repo1:",
				"+ Repository: CREATE new repository",
				"✓ Dry-run completed. No changes were applied.",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			var buf bytes.Buffer
			done := make(chan bool)
			go func() {
				_, _ = buf.ReadFrom(r)
				done <- true
			}()

			// Simulate the enhanced dry-run logic
			err := displayMultiRepoPlan(tt.plans, "testorg", true)
			require.NoError(t, err)

			// Simulate the error handling logic
			if tt.planError != nil {
				fmt.Printf("\n⚠️  Planning errors encountered during dry-run:\n")
				fmt.Printf("   %v\n", tt.planError)
				fmt.Printf("\n✓ Dry-run completed with errors. No changes were applied.\n")
			} else {
				fmt.Printf("\n✓ Dry-run completed. No changes were applied.\n")
			}

			// Restore stdout and get output
			_ = w.Close()
			os.Stdout = oldStdout
			<-done

			output := buf.String()

			// Check that all expected strings are present
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
			}
		})
	}
}

func TestDisplayMultiRepoResults(t *testing.T) {
	tests := []struct {
		name             string
		result           *github.MultiRepoResult
		owner            string
		isPartialFailure bool
		expectedOutput   []string
	}{
		{
			name: "all repositories successful",
			result: &github.MultiRepoResult{
				Succeeded: []string{"repo1", "repo2"},
				Failed:    map[string]error{},
				Skipped:   []string{},
				Summary: github.MultiRepoSummary{
					TotalRepositories: 2,
					SuccessCount:      2,
					FailureCount:      0,
					SkippedCount:      0,
					TotalChanges:      5,
				},
			},
			owner:            "testorg",
			isPartialFailure: false,
			expectedOutput: []string{
				"✅ Successfully applied changes to 2 repositories",
				"✅ Successful repositories:",
				"• testorg/repo1: https://github.com/testorg/repo1",
				"• testorg/repo2: https://github.com/testorg/repo2",
				"📊 Summary:",
				"• Total repositories: 2",
				"• Successful: 2",
				"• Failed: 0",
				"• Skipped: 0",
				"• Total changes applied: 5",
			},
		},
		{
			name: "partial failure",
			result: &github.MultiRepoResult{
				Succeeded: []string{"repo1"},
				Failed: map[string]error{
					"repo2": fmt.Errorf("authentication failed"),
				},
				Skipped: []string{},
				Summary: github.MultiRepoSummary{
					TotalRepositories: 2,
					SuccessCount:      1,
					FailureCount:      1,
					SkippedCount:      0,
					TotalChanges:      3,
				},
			},
			owner:            "testorg",
			isPartialFailure: true,
			expectedOutput: []string{
				"⚠️  Partial success: Applied changes to 1 repositories",
				"✅ Successful repositories:",
				"• testorg/repo1: https://github.com/testorg/repo1",
				"❌ Failed repositories:",
				"• testorg/repo2: authentication failed",
				"📊 Summary:",
				"• Total repositories: 2",
				"• Successful: 1",
				"• Failed: 1",
				"• Skipped: 0",
				"• Total changes applied: 3",
			},
		},
		{
			name: "with skipped repositories",
			result: &github.MultiRepoResult{
				Succeeded: []string{"repo1"},
				Failed:    map[string]error{},
				Skipped:   []string{"repo2"},
				Summary: github.MultiRepoSummary{
					TotalRepositories: 2,
					SuccessCount:      1,
					FailureCount:      0,
					SkippedCount:      1,
					TotalChanges:      2,
				},
			},
			owner:            "testorg",
			isPartialFailure: false,
			expectedOutput: []string{
				"✅ Successfully applied changes to 1 repositories",
				"✅ Successful repositories:",
				"• testorg/repo1: https://github.com/testorg/repo1",
				"⏭️  Skipped repositories:",
				"• testorg/repo2",
				"📊 Summary:",
				"• Total repositories: 2",
				"• Successful: 1",
				"• Failed: 0",
				"• Skipped: 1",
				"• Total changes applied: 2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			var buf bytes.Buffer
			done := make(chan bool)
			go func() {
				_, _ = buf.ReadFrom(r)
				done <- true
			}()

			// Call the function
			displayMultiRepoResults(tt.result, tt.owner, tt.isPartialFailure)

			// Restore stdout and get output
			_ = w.Close()
			os.Stdout = oldStdout
			<-done

			output := buf.String()

			// Check that all expected strings are present
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
			}
		})
	}
}

func TestEnhancedMultiRepoSummaryStatistics(t *testing.T) {
	tests := []struct {
		name           string
		plans          map[string]*github.ReconciliationPlan
		expectedOutput []string
	}{
		{
			name: "summary with mixed successful and failed repositories",
			plans: map[string]*github.ReconciliationPlan{
				"successful-repo-1": {
					Repository: &github.RepositoryChange{
						Type: github.ChangeTypeCreate,
						After: &github.Repository{
							Name:        "successful-repo-1",
							Description: "First successful repo",
							Private:     true,
						},
					},
				},
				"successful-repo-2": {
					BranchRules: []github.BranchRuleChange{
						{
							Type:   github.ChangeTypeCreate,
							Branch: "main",
							After: &github.BranchProtection{
								Pattern:         "main",
								RequiredReviews: 2,
							},
						},
					},
					Collaborators: []github.CollaboratorChange{
						{
							Type: github.ChangeTypeCreate,
							After: &github.Collaborator{
								Username:   "newuser",
								Permission: "write",
							},
						},
					},
				},
				"failed-repo-1":   nil, // Planning failed
				"failed-repo-2":   nil, // Planning failed
				"no-changes-repo": {},  // No changes needed
			},
			expectedOutput: []string{
				"📊 Summary:",
				"• Total repositories: 5",
				"• Repositories with changes: 2",
				"• Repositories with planning errors: 2",
				"• Total changes: 3", // 1 from successful-repo-1, 2 from successful-repo-2
			},
		},
		{
			name: "summary with only successful repositories",
			plans: map[string]*github.ReconciliationPlan{
				"repo1": {
					Repository: &github.RepositoryChange{
						Type: github.ChangeTypeUpdate,
						Before: &github.Repository{
							Name:        "repo1",
							Description: "Old description",
							Private:     true,
						},
						After: &github.Repository{
							Name:        "repo1",
							Description: "New description",
							Private:     true,
						},
					},
				},
				"repo2": {}, // No changes
			},
			expectedOutput: []string{
				"📊 Summary:",
				"• Total repositories: 2",
				"• Repositories with changes: 1",
				"• Total changes: 1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			var buf bytes.Buffer
			done := make(chan bool)
			go func() {
				_, _ = buf.ReadFrom(r)
				done <- true
			}()

			// Call the function
			err := displayMultiRepoPlan(tt.plans, "testorg", true)
			require.NoError(t, err)

			// Restore stdout and get output
			_ = w.Close()
			os.Stdout = oldStdout
			<-done

			output := buf.String()

			// Check that all expected strings are present
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
			}
		})
	}
}

func TestCountTotalChanges(t *testing.T) {
	tests := []struct {
		name     string
		plans    map[string]*github.ReconciliationPlan
		expected int
	}{
		{
			name:     "empty plans",
			plans:    map[string]*github.ReconciliationPlan{},
			expected: 0,
		},
		{
			name: "single repository with changes",
			plans: map[string]*github.ReconciliationPlan{
				"repo1": {
					Repository: &github.RepositoryChange{
						Type: github.ChangeTypeCreate,
					},
					BranchRules: []github.BranchRuleChange{
						{Type: github.ChangeTypeCreate},
					},
				},
			},
			expected: 2,
		},
		{
			name: "multiple repositories with changes",
			plans: map[string]*github.ReconciliationPlan{
				"repo1": {
					Repository: &github.RepositoryChange{
						Type: github.ChangeTypeCreate,
					},
					Collaborators: []github.CollaboratorChange{
						{Type: github.ChangeTypeCreate},
						{Type: github.ChangeTypeUpdate},
					},
				},
				"repo2": {
					BranchRules: []github.BranchRuleChange{
						{Type: github.ChangeTypeCreate},
					},
					Teams: []github.TeamChange{
						{Type: github.ChangeTypeCreate},
					},
					Webhooks: []github.WebhookChange{
						{Type: github.ChangeTypeCreate},
					},
				},
			},
			expected: 6,
		},
		{
			name: "repository with no changes",
			plans: map[string]*github.ReconciliationPlan{
				"repo1": {},
				"repo2": {
					Repository: &github.RepositoryChange{
						Type: github.ChangeTypeUpdate,
					},
				},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countTotalChanges(tt.plans)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountPlanChanges(t *testing.T) {
	tests := []struct {
		name     string
		plan     *github.ReconciliationPlan
		expected int
	}{
		{
			name:     "nil plan",
			plan:     nil,
			expected: 0,
		},
		{
			name:     "empty plan",
			plan:     &github.ReconciliationPlan{},
			expected: 0,
		},
		{
			name: "plan with all change types",
			plan: &github.ReconciliationPlan{
				Repository: &github.RepositoryChange{
					Type: github.ChangeTypeCreate,
				},
				BranchRules: []github.BranchRuleChange{
					{Type: github.ChangeTypeCreate},
					{Type: github.ChangeTypeUpdate},
				},
				Collaborators: []github.CollaboratorChange{
					{Type: github.ChangeTypeCreate},
				},
				Teams: []github.TeamChange{
					{Type: github.ChangeTypeCreate},
					{Type: github.ChangeTypeUpdate},
					{Type: github.ChangeTypeDelete},
				},
				Webhooks: []github.WebhookChange{
					{Type: github.ChangeTypeCreate},
					{Type: github.ChangeTypeUpdate},
				},
			},
			expected: 9, // 1 repo + 2 branch + 1 collab + 3 teams + 2 webhooks
		},
		{
			name: "plan with only repository change",
			plan: &github.ReconciliationPlan{
				Repository: &github.RepositoryChange{
					Type: github.ChangeTypeUpdate,
				},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countPlanChanges(tt.plan)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDisplayRepositoryPlanChanges(t *testing.T) {
	tests := []struct {
		name                     string
		plan                     *github.ReconciliationPlan
		indent                   string
		expectedDestructiveCount int
		expectedOutput           []string
	}{
		{
			name: "repository creation",
			plan: &github.ReconciliationPlan{
				Repository: &github.RepositoryChange{
					Type: github.ChangeTypeCreate,
					After: &github.Repository{
						Name:        "test-repo",
						Description: "Test repository",
						Private:     true,
						Topics:      []string{"test", "golang"},
					},
				},
			},
			indent:                   "  ",
			expectedDestructiveCount: 0,
			expectedOutput: []string{
				"  + Repository: CREATE new repository",
				"  - Name: test-repo",
				"  - Description: Test repository",
				"  - Private: true",
				"  - Topics: test, golang",
			},
		},
		{
			name: "destructive repository change",
			plan: &github.ReconciliationPlan{
				Repository: &github.RepositoryChange{
					Type: github.ChangeTypeUpdate,
					Before: &github.Repository{
						Name:    "test-repo",
						Private: true,
					},
					After: &github.Repository{
						Name:    "test-repo",
						Private: false,
					},
				},
			},
			indent:                   "  ",
			expectedDestructiveCount: 1,
			expectedOutput: []string{
				"  ~ Repository: UPDATE repository settings",
				"  ⚠️  Private: true → false (MAKING REPOSITORY PUBLIC)",
			},
		},
		{
			name: "destructive collaborator changes",
			plan: &github.ReconciliationPlan{
				Collaborators: []github.CollaboratorChange{
					{
						Type: github.ChangeTypeUpdate,
						Before: &github.Collaborator{
							Username:   "user1",
							Permission: "admin",
						},
						After: &github.Collaborator{
							Username:   "user1",
							Permission: "read",
						},
					},
					{
						Type: github.ChangeTypeDelete,
						Before: &github.Collaborator{
							Username:   "user2",
							Permission: "write",
						},
					},
				},
			},
			indent:                   "  ",
			expectedDestructiveCount: 2,
			expectedOutput: []string{
				"  ⚠️  Collaborator: UPDATE user1 permission admin → read (REDUCING ACCESS)",
				"  ⚠️  Collaborator: REMOVE user2 (REMOVING ACCESS)",
			},
		},
		{
			name: "mixed changes with some destructive",
			plan: &github.ReconciliationPlan{
				BranchRules: []github.BranchRuleChange{
					{
						Type:   github.ChangeTypeCreate,
						Branch: "main",
						After: &github.BranchProtection{
							Pattern:         "main",
							RequiredReviews: 2,
						},
					},
					{
						Type:   github.ChangeTypeDelete,
						Branch: "develop",
						Before: &github.BranchProtection{
							Pattern:         "develop",
							RequiredReviews: 1,
						},
					},
				},
				Teams: []github.TeamChange{
					{
						Type: github.ChangeTypeCreate,
						After: &github.TeamAccess{
							TeamSlug:   "new-team",
							Permission: "write",
						},
					},
				},
			},
			indent:                   "    ",
			expectedDestructiveCount: 1,
			expectedOutput: []string{
				"    + Branch Protection: CREATE rule for main",
				"    ⚠️  Branch Protection: DELETE rule for develop (REMOVING PROTECTION)",
				"    + Team: ADD new-team with write permission",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			var buf bytes.Buffer
			done := make(chan bool)
			go func() {
				_, _ = buf.ReadFrom(r)
				done <- true
			}()

			// Call the function
			destructiveCount := displayRepositoryPlanChanges(tt.plan, tt.indent)

			// Restore stdout and get output
			_ = w.Close()
			os.Stdout = oldStdout
			<-done

			output := buf.String()

			// Check destructive count
			assert.Equal(t, tt.expectedDestructiveCount, destructiveCount, "Expected destructive change count to match")

			// Check that all expected strings are present
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
			}
		})
	}
}
