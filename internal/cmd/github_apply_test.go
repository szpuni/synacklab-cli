package cmd

import (
	"bytes"
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

	// Verify summary - the current implementation only counts top-level destructive changes
	// Branch protection changes within displayBranchProtectionChanges don't increment the counter
	// Only the collaborator permission downgrade is counted as destructive
	assert.Contains(t, output, "Total changes: 7 (1 potentially destructive)")
	assert.Contains(t, output, "⚠️  WARNING: 1 potentially destructive change(s) detected!")
	assert.Contains(t, output, "Review these changes carefully before applying.")
}
