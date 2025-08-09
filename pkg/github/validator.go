package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v66/github"
)

// Validator provides enhanced validation with GitHub API access
type Validator struct {
	client *github.Client
	ctx    context.Context
}

// NewValidator creates a new validator with GitHub API access
func NewValidator(token string) *Validator {
	client := NewClient(token)
	return &Validator{
		client: client.client,
		ctx:    client.ctx,
	}
}

// ValidateConfig performs comprehensive validation including GitHub API checks
func (v *Validator) ValidateConfig(config *RepositoryConfig, owner string) error {
	// First perform basic validation
	if err := config.Validate(); err != nil {
		return err
	}

	// Then perform GitHub API validation
	if err := v.validateCollaboratorsExist(config.Collaborators); err != nil {
		return err
	}

	if err := v.validateTeamsExist(config.Teams, owner); err != nil {
		return err
	}

	return nil
}

// validateCollaboratorsExist checks if all collaborator usernames exist on GitHub
func (v *Validator) validateCollaboratorsExist(collaborators []Collaborator) error {
	for i, collab := range collaborators {
		_, _, err := v.client.Users.Get(v.ctx, collab.Username)
		if err != nil {
			// Check if it's a 404 error (user not found)
			if githubErr, ok := err.(*github.ErrorResponse); ok && githubErr.Response.StatusCode == 404 {
				return fmt.Errorf("collaborator %d: user '%s' does not exist on GitHub", i+1, collab.Username)
			}
			return fmt.Errorf("collaborator %d: failed to validate user '%s': %w", i+1, collab.Username, err)
		}
	}
	return nil
}

// validateTeamsExist checks if all team slugs exist in the organization
func (v *Validator) validateTeamsExist(teams []TeamAccess, owner string) error {
	for i, team := range teams {
		_, _, err := v.client.Teams.GetTeamBySlug(v.ctx, owner, team.TeamSlug)
		if err != nil {
			// Check if it's a 404 error (team not found)
			if githubErr, ok := err.(*github.ErrorResponse); ok && githubErr.Response.StatusCode == 404 {
				return fmt.Errorf("team %d: team '%s' does not exist in organization '%s'", i+1, team.TeamSlug, owner)
			}
			return fmt.Errorf("team %d: failed to validate team '%s': %w", i+1, team.TeamSlug, err)
		}
	}
	return nil
}

// ValidatePermissions checks if the current token has the required permissions
func (v *Validator) ValidatePermissions(owner, repo string) error {
	// Check if we can access the repository (or if it doesn't exist yet, check org permissions)
	_, resp, err := v.client.Repositories.Get(v.ctx, owner, repo)
	if err != nil {
		// If repo doesn't exist, check if we have permissions to create repos in the org
		if resp != nil && resp.StatusCode == 404 {
			return v.validateCreatePermissions(owner)
		}
		return fmt.Errorf("failed to check repository permissions: %w", err)
	}

	// If repo exists, check if we have admin permissions
	return v.validateAdminPermissions(owner, repo)
}

// validateCreatePermissions checks if we can create repositories in the organization
func (v *Validator) validateCreatePermissions(owner string) error {
	// Try to get the organization to see if we have access
	_, _, err := v.client.Organizations.Get(v.ctx, owner)
	if err != nil {
		if githubErr, ok := err.(*github.ErrorResponse); ok && githubErr.Response.StatusCode == 404 {
			return fmt.Errorf("organization '%s' does not exist or you don't have access to it", owner)
		}
		return fmt.Errorf("failed to validate organization access: %w", err)
	}

	// Check if we can list repositories (indicates we have some level of access)
	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 1},
	}
	_, _, err = v.client.Repositories.ListByOrg(v.ctx, owner, opts)
	if err != nil {
		return fmt.Errorf("insufficient permissions to create repositories in organization '%s'", owner)
	}

	return nil
}

// validateAdminPermissions checks if we have admin permissions on the repository
func (v *Validator) validateAdminPermissions(owner, repo string) error {
	// Try to list collaborators - this requires admin permissions
	opts := &github.ListCollaboratorsOptions{
		ListOptions: github.ListOptions{PerPage: 1},
	}
	_, _, err := v.client.Repositories.ListCollaborators(v.ctx, owner, repo, opts)
	if err != nil {
		if githubErr, ok := err.(*github.ErrorResponse); ok && githubErr.Response.StatusCode == 403 {
			return fmt.Errorf("insufficient permissions: admin access required for repository '%s/%s'", owner, repo)
		}
		return fmt.Errorf("failed to validate repository permissions: %w", err)
	}

	return nil
}
