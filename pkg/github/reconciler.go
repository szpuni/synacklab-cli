package github

import (
	"fmt"
	"reflect"
	"sort"
)

// reconciler implements the Reconciler interface
type reconciler struct {
	client   APIClient
	owner    string
	repoName string
}

// NewReconciler creates a new reconciler instance
func NewReconciler(client APIClient, owner string) Reconciler {
	return &reconciler{
		client: client,
		owner:  owner,
	}
}

// Plan creates a reconciliation plan by comparing desired configuration with current state
func (r *reconciler) Plan(config RepositoryConfig) (*ReconciliationPlan, error) {
	plan := &ReconciliationPlan{}

	// Store repository name for use in apply operations
	r.repoName = config.Name

	// Get current repository state
	currentRepo, err := r.client.GetRepository(r.owner, config.Name)
	if err != nil {
		// Repository doesn't exist, plan to create it
		plan.Repository = &RepositoryChange{
			Type: ChangeTypeCreate,
			After: &Repository{
				Name:        config.Name,
				Description: config.Description,
				Private:     config.Private,
				Topics:      config.Topics,
				Features:    config.Features,
			},
		}
	} else {
		// Repository exists, check for changes
		if repoChange := r.compareRepository(currentRepo, config); repoChange != nil {
			plan.Repository = repoChange
		}
	}

	// Only plan other changes if repository exists (not for new repositories)
	if currentRepo != nil {
		// Plan branch protection changes
		branchChanges, err := r.planBranchProtectionChanges(config)
		if err != nil {
			return nil, fmt.Errorf("failed to plan branch protection changes: %w", err)
		}
		plan.BranchRules = branchChanges

		// Plan collaborator changes
		collaboratorChanges, err := r.planCollaboratorChanges(config)
		if err != nil {
			return nil, fmt.Errorf("failed to plan collaborator changes: %w", err)
		}
		plan.Collaborators = collaboratorChanges

		// Plan team access changes
		teamChanges, err := r.planTeamChanges(config)
		if err != nil {
			return nil, fmt.Errorf("failed to plan team changes: %w", err)
		}
		plan.Teams = teamChanges

		// Plan webhook changes
		webhookChanges, err := r.planWebhookChanges(config)
		if err != nil {
			return nil, fmt.Errorf("failed to plan webhook changes: %w", err)
		}
		plan.Webhooks = webhookChanges
	} else if plan.Repository != nil && plan.Repository.Type == ChangeTypeCreate {
		// For new repositories, plan to add all configured resources after creation
		for _, rule := range config.BranchRules {
			plan.BranchRules = append(plan.BranchRules, BranchRuleChange{
				Type:   ChangeTypeCreate,
				Branch: rule.Pattern,
				After: &BranchProtection{
					Pattern:                rule.Pattern,
					RequiredStatusChecks:   rule.RequiredStatusChecks,
					RequireUpToDate:        rule.RequireUpToDate,
					RequiredReviews:        rule.RequiredReviews,
					DismissStaleReviews:    rule.DismissStaleReviews,
					RequireCodeOwnerReview: rule.RequireCodeOwnerReview,
					RestrictPushes:         rule.RestrictPushes,
				},
			})
		}

		for _, collab := range config.Collaborators {
			plan.Collaborators = append(plan.Collaborators, CollaboratorChange{
				Type:  ChangeTypeCreate,
				After: &collab,
			})
		}

		for _, team := range config.Teams {
			plan.Teams = append(plan.Teams, TeamChange{
				Type:  ChangeTypeCreate,
				After: &team,
			})
		}

		for _, webhook := range config.Webhooks {
			plan.Webhooks = append(plan.Webhooks, WebhookChange{
				Type:  ChangeTypeCreate,
				After: &webhook,
			})
		}
	}

	return plan, nil
}

// Apply executes the reconciliation plan
func (r *reconciler) Apply(plan *ReconciliationPlan) error {
	var succeeded []string
	failed := make(map[string]error)

	// Apply repository changes first
	if plan.Repository != nil {
		if err := r.applyRepositoryChange(plan.Repository); err != nil {
			failed["repository"] = err
			// Repository creation/update failure is critical, return immediately
			return WrapGitHubError(err, "repository")
		}
		succeeded = append(succeeded, "repository")
	}

	// Apply branch protection changes
	for _, change := range plan.BranchRules {
		operation := fmt.Sprintf("branch protection for %s", change.Branch)
		if err := r.applyBranchRuleChange(change); err != nil {
			failed[operation] = err
		} else {
			succeeded = append(succeeded, operation)
		}
	}

	// Apply collaborator changes
	for _, change := range plan.Collaborators {
		var operation string
		if change.After != nil {
			operation = fmt.Sprintf("collaborator %s", change.After.Username)
		} else if change.Before != nil {
			operation = fmt.Sprintf("collaborator %s", change.Before.Username)
		} else {
			operation = "collaborator (unknown)"
		}

		if err := r.applyCollaboratorChange(change); err != nil {
			failed[operation] = err
		} else {
			succeeded = append(succeeded, operation)
		}
	}

	// Apply team changes
	for _, change := range plan.Teams {
		var operation string
		if change.After != nil {
			operation = fmt.Sprintf("team %s", change.After.TeamSlug)
		} else if change.Before != nil {
			operation = fmt.Sprintf("team %s", change.Before.TeamSlug)
		} else {
			operation = "team (unknown)"
		}

		if err := r.applyTeamChange(change); err != nil {
			failed[operation] = err
		} else {
			succeeded = append(succeeded, operation)
		}
	}

	// Apply webhook changes
	for _, change := range plan.Webhooks {
		var operation string
		if change.After != nil {
			operation = fmt.Sprintf("webhook %s", change.After.URL)
		} else if change.Before != nil {
			operation = fmt.Sprintf("webhook %s", change.Before.URL)
		} else {
			operation = "webhook (unknown)"
		}

		if err := r.applyWebhookChange(change); err != nil {
			failed[operation] = err
		} else {
			succeeded = append(succeeded, operation)
		}
	}

	// If there were failures, return a partial failure error
	if len(failed) > 0 {
		return NewPartialFailureError(succeeded, failed)
	}

	return nil
}

// Validate validates the configuration against GitHub constraints
func (r *reconciler) Validate(config RepositoryConfig) error {
	// First validate the configuration structure
	if err := config.Validate(); err != nil {
		return err
	}

	// Additional GitHub-specific validation could be added here
	// For example, checking if users/teams exist, webhook URLs are reachable, etc.

	return nil
}

// compareRepository compares current repository state with desired configuration
func (r *reconciler) compareRepository(current *Repository, config RepositoryConfig) *RepositoryChange {
	desired := &Repository{
		ID:          current.ID,
		Name:        config.Name,
		FullName:    current.FullName,
		Description: config.Description,
		Private:     config.Private,
		Topics:      config.Topics,
		Features:    config.Features,
		CreatedAt:   current.CreatedAt,
		UpdatedAt:   current.UpdatedAt,
	}

	if !r.repositoriesEqual(current, desired) {
		return &RepositoryChange{
			Type:   ChangeTypeUpdate,
			Before: current,
			After:  desired,
		}
	}

	return nil
}

// repositoriesEqual compares two repositories for equality
func (r *reconciler) repositoriesEqual(a, b *Repository) bool {
	if a.Description != b.Description || a.Private != b.Private {
		return false
	}

	// Compare topics (order doesn't matter)
	if !r.stringSlicesEqual(a.Topics, b.Topics) {
		return false
	}

	// Compare features
	return a.Features == b.Features
}

// planBranchProtectionChanges plans changes for branch protection rules
func (r *reconciler) planBranchProtectionChanges(config RepositoryConfig) ([]BranchRuleChange, error) {
	var changes []BranchRuleChange

	for _, rule := range config.BranchRules {
		current, err := r.client.GetBranchProtection(r.owner, config.Name, rule.Pattern)
		if err != nil {
			// Branch protection doesn't exist, plan to create it
			changes = append(changes, BranchRuleChange{
				Type:   ChangeTypeCreate,
				Branch: rule.Pattern,
				After: &BranchProtection{
					Pattern:                rule.Pattern,
					RequiredStatusChecks:   rule.RequiredStatusChecks,
					RequireUpToDate:        rule.RequireUpToDate,
					RequiredReviews:        rule.RequiredReviews,
					DismissStaleReviews:    rule.DismissStaleReviews,
					RequireCodeOwnerReview: rule.RequireCodeOwnerReview,
					RestrictPushes:         rule.RestrictPushes,
				},
			})
		} else {
			// Branch protection exists, check for changes
			desired := &BranchProtection{
				Pattern:                rule.Pattern,
				RequiredStatusChecks:   rule.RequiredStatusChecks,
				RequireUpToDate:        rule.RequireUpToDate,
				RequiredReviews:        rule.RequiredReviews,
				DismissStaleReviews:    rule.DismissStaleReviews,
				RequireCodeOwnerReview: rule.RequireCodeOwnerReview,
				RestrictPushes:         rule.RestrictPushes,
			}

			if !r.branchProtectionsEqual(current, desired) {
				changes = append(changes, BranchRuleChange{
					Type:   ChangeTypeUpdate,
					Branch: rule.Pattern,
					Before: current,
					After:  desired,
				})
			}
		}
	}

	return changes, nil
}

// planCollaboratorChanges plans changes for repository collaborators
func (r *reconciler) planCollaboratorChanges(config RepositoryConfig) ([]CollaboratorChange, error) {
	var changes []CollaboratorChange

	// Get current collaborators
	currentCollaborators, err := r.client.ListCollaborators(r.owner, config.Name)
	if err != nil {
		return nil, err
	}

	// Create maps for easier comparison
	currentMap := make(map[string]*Collaborator)
	for i := range currentCollaborators {
		currentMap[currentCollaborators[i].Username] = &currentCollaborators[i]
	}

	desiredMap := make(map[string]*Collaborator)
	for i := range config.Collaborators {
		desiredMap[config.Collaborators[i].Username] = &config.Collaborators[i]
	}

	// Find collaborators to add or update
	for username, desired := range desiredMap {
		if current, exists := currentMap[username]; exists {
			// Collaborator exists, check for permission changes
			if current.Permission != desired.Permission {
				changes = append(changes, CollaboratorChange{
					Type:   ChangeTypeUpdate,
					Before: current,
					After:  desired,
				})
			}
		} else {
			// Collaborator doesn't exist, plan to add
			changes = append(changes, CollaboratorChange{
				Type:  ChangeTypeCreate,
				After: desired,
			})
		}
	}

	// Find collaborators to remove
	for username, current := range currentMap {
		if _, exists := desiredMap[username]; !exists {
			changes = append(changes, CollaboratorChange{
				Type:   ChangeTypeDelete,
				Before: current,
			})
		}
	}

	return changes, nil
}

// planTeamChanges plans changes for team access
func (r *reconciler) planTeamChanges(config RepositoryConfig) ([]TeamChange, error) {
	var changes []TeamChange

	// Get current team access
	currentTeams, err := r.client.ListTeamAccess(r.owner, config.Name)
	if err != nil {
		return nil, err
	}

	// Create maps for easier comparison
	currentMap := make(map[string]*TeamAccess)
	for i := range currentTeams {
		currentMap[currentTeams[i].TeamSlug] = &currentTeams[i]
	}

	desiredMap := make(map[string]*TeamAccess)
	for i := range config.Teams {
		desiredMap[config.Teams[i].TeamSlug] = &config.Teams[i]
	}

	// Find teams to add or update
	for teamSlug, desired := range desiredMap {
		if current, exists := currentMap[teamSlug]; exists {
			// Team access exists, check for permission changes
			if current.Permission != desired.Permission {
				changes = append(changes, TeamChange{
					Type:   ChangeTypeUpdate,
					Before: current,
					After:  desired,
				})
			}
		} else {
			// Team access doesn't exist, plan to add
			changes = append(changes, TeamChange{
				Type:  ChangeTypeCreate,
				After: desired,
			})
		}
	}

	// Find teams to remove
	for teamSlug, current := range currentMap {
		if _, exists := desiredMap[teamSlug]; !exists {
			changes = append(changes, TeamChange{
				Type:   ChangeTypeDelete,
				Before: current,
			})
		}
	}

	return changes, nil
}

// planWebhookChanges plans changes for webhooks
func (r *reconciler) planWebhookChanges(config RepositoryConfig) ([]WebhookChange, error) {
	var changes []WebhookChange

	// Get current webhooks
	currentWebhooks, err := r.client.ListWebhooks(r.owner, config.Name)
	if err != nil {
		return nil, err
	}

	// Create maps for easier comparison (using URL as key since webhooks don't have unique names)
	currentMap := make(map[string]*Webhook)
	for i := range currentWebhooks {
		currentMap[currentWebhooks[i].URL] = &currentWebhooks[i]
	}

	desiredMap := make(map[string]*Webhook)
	for i := range config.Webhooks {
		desiredMap[config.Webhooks[i].URL] = &config.Webhooks[i]
	}

	// Find webhooks to add or update
	for url, desired := range desiredMap {
		if current, exists := currentMap[url]; exists {
			// Webhook exists, check for changes
			if !r.webhooksEqual(current, desired) {
				// Copy the ID from current webhook for updates
				updatedWebhook := *desired
				updatedWebhook.ID = current.ID
				changes = append(changes, WebhookChange{
					Type:   ChangeTypeUpdate,
					Before: current,
					After:  &updatedWebhook,
				})
			}
		} else {
			// Webhook doesn't exist, plan to add
			changes = append(changes, WebhookChange{
				Type:  ChangeTypeCreate,
				After: desired,
			})
		}
	}

	// Find webhooks to remove
	for url, current := range currentMap {
		if _, exists := desiredMap[url]; !exists {
			changes = append(changes, WebhookChange{
				Type:   ChangeTypeDelete,
				Before: current,
			})
		}
	}

	return changes, nil
}

// Helper functions for applying changes

func (r *reconciler) applyRepositoryChange(change *RepositoryChange) error {
	switch change.Type {
	case ChangeTypeCreate:
		config := RepositoryConfig{
			Name:        change.After.Name,
			Description: change.After.Description,
			Private:     change.After.Private,
			Topics:      change.After.Topics,
			Features:    change.After.Features,
		}
		_, err := r.client.CreateRepository(config)
		return err
	case ChangeTypeUpdate:
		config := RepositoryConfig{
			Name:        change.After.Name,
			Description: change.After.Description,
			Private:     change.After.Private,
			Topics:      change.After.Topics,
			Features:    change.After.Features,
		}
		return r.client.UpdateRepository(r.owner, change.After.Name, config)
	default:
		return fmt.Errorf("unsupported repository change type: %s", change.Type)
	}
}

func (r *reconciler) applyBranchRuleChange(change BranchRuleChange) error {

	switch change.Type {
	case ChangeTypeCreate:
		rule := BranchProtectionRule{
			Pattern:                change.After.Pattern,
			RequiredStatusChecks:   change.After.RequiredStatusChecks,
			RequireUpToDate:        change.After.RequireUpToDate,
			RequiredReviews:        change.After.RequiredReviews,
			DismissStaleReviews:    change.After.DismissStaleReviews,
			RequireCodeOwnerReview: change.After.RequireCodeOwnerReview,
			RestrictPushes:         change.After.RestrictPushes,
		}
		return r.client.CreateBranchProtection(r.owner, r.repoName, change.Branch, rule)
	case ChangeTypeUpdate:
		rule := BranchProtectionRule{
			Pattern:                change.After.Pattern,
			RequiredStatusChecks:   change.After.RequiredStatusChecks,
			RequireUpToDate:        change.After.RequireUpToDate,
			RequiredReviews:        change.After.RequiredReviews,
			DismissStaleReviews:    change.After.DismissStaleReviews,
			RequireCodeOwnerReview: change.After.RequireCodeOwnerReview,
			RestrictPushes:         change.After.RestrictPushes,
		}
		return r.client.UpdateBranchProtection(r.owner, r.repoName, change.Branch, rule)
	case ChangeTypeDelete:
		return r.client.DeleteBranchProtection(r.owner, r.repoName, change.Branch)
	default:
		return fmt.Errorf("unsupported branch rule change type: %s", change.Type)
	}
}

func (r *reconciler) applyCollaboratorChange(change CollaboratorChange) error {
	switch change.Type {
	case ChangeTypeCreate:
		return r.client.AddCollaborator(r.owner, r.repoName, change.After.Username, change.After.Permission)
	case ChangeTypeUpdate:
		return r.client.AddCollaborator(r.owner, r.repoName, change.After.Username, change.After.Permission)
	case ChangeTypeDelete:
		return r.client.RemoveCollaborator(r.owner, r.repoName, change.Before.Username)
	default:
		return fmt.Errorf("unsupported collaborator change type: %s", change.Type)
	}
}

func (r *reconciler) applyTeamChange(change TeamChange) error {
	switch change.Type {
	case ChangeTypeCreate:
		return r.client.AddTeamAccess(r.owner, r.repoName, *change.After)
	case ChangeTypeUpdate:
		return r.client.UpdateTeamAccess(r.owner, r.repoName, *change.After)
	case ChangeTypeDelete:
		return r.client.RemoveTeamAccess(r.owner, r.repoName, change.Before.TeamSlug)
	default:
		return fmt.Errorf("unsupported team change type: %s", change.Type)
	}
}

func (r *reconciler) applyWebhookChange(change WebhookChange) error {
	switch change.Type {
	case ChangeTypeCreate:
		return r.client.CreateWebhook(r.owner, r.repoName, *change.After)
	case ChangeTypeUpdate:
		return r.client.UpdateWebhook(r.owner, r.repoName, change.After.ID, *change.After)
	case ChangeTypeDelete:
		return r.client.DeleteWebhook(r.owner, r.repoName, change.Before.ID)
	default:
		return fmt.Errorf("unsupported webhook change type: %s", change.Type)
	}
}

// Helper comparison functions

func (r *reconciler) stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Sort both slices for comparison
	sortedA := make([]string, len(a))
	sortedB := make([]string, len(b))
	copy(sortedA, a)
	copy(sortedB, b)
	sort.Strings(sortedA)
	sort.Strings(sortedB)

	return reflect.DeepEqual(sortedA, sortedB)
}

func (r *reconciler) branchProtectionsEqual(a, b *BranchProtection) bool {
	return a.Pattern == b.Pattern &&
		r.stringSlicesEqual(a.RequiredStatusChecks, b.RequiredStatusChecks) &&
		a.RequireUpToDate == b.RequireUpToDate &&
		a.RequiredReviews == b.RequiredReviews &&
		a.DismissStaleReviews == b.DismissStaleReviews &&
		a.RequireCodeOwnerReview == b.RequireCodeOwnerReview &&
		r.stringSlicesEqual(a.RestrictPushes, b.RestrictPushes)
}

func (r *reconciler) webhooksEqual(a, b *Webhook) bool {
	return a.URL == b.URL &&
		r.stringSlicesEqual(a.Events, b.Events) &&
		a.Secret == b.Secret &&
		a.Active == b.Active
}
