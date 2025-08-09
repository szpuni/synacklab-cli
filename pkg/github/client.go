package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
)

// Client implements the APIClient interface using the GitHub REST API
type Client struct {
	client *github.Client
	ctx    context.Context
}

// NewClient creates a new GitHub API client with the provided token
func NewClient(token string) *Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &Client{
		client: github.NewClient(tc),
		ctx:    ctx,
	}
}

// GetRepository retrieves a repository by owner and name
func (c *Client) GetRepository(owner, name string) (*Repository, error) {
	var repo *github.Repository

	err := WithRetry(func() error {
		var err error
		repo, _, err = c.client.Repositories.Get(c.ctx, owner, name)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("repository %s/%s", owner, name))
		}
		return nil
	}, DefaultRetryConfig())

	if err != nil {
		return nil, err
	}

	return c.convertGitHubRepository(repo), nil
}

// CreateRepository creates a new repository with the given configuration
func (c *Client) CreateRepository(config RepositoryConfig) (*Repository, error) {
	repo := &github.Repository{
		Name:        github.String(config.Name),
		Description: github.String(config.Description),
		Private:     github.Bool(config.Private),
		HasIssues:   github.Bool(config.Features.Issues),
		HasWiki:     github.Bool(config.Features.Wiki),
		HasProjects: github.Bool(config.Features.Projects),
	}

	// Set topics if provided
	if len(config.Topics) > 0 {
		repo.Topics = config.Topics
	}

	var createdRepo *github.Repository

	err := WithRetry(func() error {
		var err error
		createdRepo, _, err = c.client.Repositories.Create(c.ctx, "", repo)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("repository %s", config.Name))
		}
		return nil
	}, DefaultRetryConfig())

	if err != nil {
		return nil, err
	}

	return c.convertGitHubRepository(createdRepo), nil
}

// UpdateRepository updates an existing repository with the given configuration
func (c *Client) UpdateRepository(owner, name string, config RepositoryConfig) error {
	repo := &github.Repository{
		Name:        github.String(config.Name),
		Description: github.String(config.Description),
		Private:     github.Bool(config.Private),
		HasIssues:   github.Bool(config.Features.Issues),
		HasWiki:     github.Bool(config.Features.Wiki),
		HasProjects: github.Bool(config.Features.Projects),
	}

	// Set topics if provided
	if len(config.Topics) > 0 {
		repo.Topics = config.Topics
	}

	return WithRetry(func() error {
		_, _, err := c.client.Repositories.Edit(c.ctx, owner, name, repo)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("repository %s/%s", owner, name))
		}
		return nil
	}, DefaultRetryConfig())
}

// GetBranchProtection retrieves branch protection rules for a specific branch
func (c *Client) GetBranchProtection(owner, name, branch string) (*BranchProtection, error) {
	var protection *github.Protection

	err := WithRetry(func() error {
		var err error
		protection, _, err = c.client.Repositories.GetBranchProtection(c.ctx, owner, name, branch)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("branch protection %s/%s:%s", owner, name, branch))
		}
		return nil
	}, DefaultRetryConfig())

	if err != nil {
		return nil, err
	}

	return c.convertGitHubBranchProtection(protection, branch), nil
}

// CreateBranchProtection creates branch protection rules for a specific branch
func (c *Client) CreateBranchProtection(owner, name, branch string, rules BranchProtectionRule) error {
	protection := c.buildProtectionRequest(rules)

	return WithRetry(func() error {
		_, _, err := c.client.Repositories.UpdateBranchProtection(c.ctx, owner, name, branch, protection)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("branch protection %s/%s:%s", owner, name, branch))
		}
		return nil
	}, DefaultRetryConfig())
}

// UpdateBranchProtection updates branch protection rules for a specific branch
func (c *Client) UpdateBranchProtection(owner, name, branch string, rules BranchProtectionRule) error {
	protection := c.buildProtectionRequest(rules)

	return WithRetry(func() error {
		_, _, err := c.client.Repositories.UpdateBranchProtection(c.ctx, owner, name, branch, protection)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("branch protection %s/%s:%s", owner, name, branch))
		}
		return nil
	}, DefaultRetryConfig())
}

// DeleteBranchProtection removes branch protection rules for a specific branch
func (c *Client) DeleteBranchProtection(owner, name, branch string) error {
	return WithRetry(func() error {
		_, err := c.client.Repositories.RemoveBranchProtection(c.ctx, owner, name, branch)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("branch protection %s/%s:%s", owner, name, branch))
		}
		return nil
	}, DefaultRetryConfig())
}

// buildProtectionRequest builds a GitHub API ProtectionRequest from our BranchProtectionRule
func (c *Client) buildProtectionRequest(rules BranchProtectionRule) *github.ProtectionRequest {
	protection := &github.ProtectionRequest{
		EnforceAdmins: true,
	}

	// Set required status checks if specified
	if len(rules.RequiredStatusChecks) > 0 || rules.RequireUpToDate {
		protection.RequiredStatusChecks = &github.RequiredStatusChecks{
			Strict:   rules.RequireUpToDate,
			Contexts: &rules.RequiredStatusChecks,
		}
	}

	// Set required reviews if specified
	if rules.RequiredReviews > 0 {
		protection.RequiredPullRequestReviews = &github.PullRequestReviewsEnforcementRequest{
			RequiredApprovingReviewCount: rules.RequiredReviews,
			DismissStaleReviews:          rules.DismissStaleReviews,
			RequireCodeOwnerReviews:      rules.RequireCodeOwnerReview,
		}
	}

	// Set push restrictions if specified
	if len(rules.RestrictPushes) > 0 {
		protection.Restrictions = &github.BranchRestrictionsRequest{
			Users: rules.RestrictPushes,
		}
	}

	return protection
}

// ListCollaborators lists all collaborators for a repository
func (c *Client) ListCollaborators(owner, name string) ([]Collaborator, error) {
	opts := &github.ListCollaboratorsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allCollaborators []Collaborator

	err := WithRetry(func() error {
		allCollaborators = nil // Reset on retry
		opts.Page = 0          // Reset pagination on retry

		for {
			collaborators, resp, err := c.client.Repositories.ListCollaborators(c.ctx, owner, name, opts)
			if err != nil {
				return WrapGitHubError(err, fmt.Sprintf("collaborators for %s/%s", owner, name))
			}

			for _, collab := range collaborators {
				allCollaborators = append(allCollaborators, Collaborator{
					Username:   collab.GetLogin(),
					Permission: strings.ToLower(collab.GetRoleName()),
				})
			}

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}
		return nil
	}, DefaultRetryConfig())

	return allCollaborators, err
}

// AddCollaborator adds a collaborator to a repository
func (c *Client) AddCollaborator(owner, name, username, permission string) error {
	opts := &github.RepositoryAddCollaboratorOptions{
		Permission: permission,
	}

	return WithRetry(func() error {
		_, _, err := c.client.Repositories.AddCollaborator(c.ctx, owner, name, username, opts)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("collaborator %s for %s/%s", username, owner, name))
		}
		return nil
	}, DefaultRetryConfig())
}

// RemoveCollaborator removes a collaborator from a repository
func (c *Client) RemoveCollaborator(owner, name, username string) error {
	return WithRetry(func() error {
		_, err := c.client.Repositories.RemoveCollaborator(c.ctx, owner, name, username)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("collaborator %s for %s/%s", username, owner, name))
		}
		return nil
	}, DefaultRetryConfig())
}

// ListTeamAccess lists all team access for a repository
func (c *Client) ListTeamAccess(owner, name string) ([]TeamAccess, error) {
	opts := &github.ListOptions{PerPage: 100}

	var allTeams []TeamAccess

	err := WithRetry(func() error {
		allTeams = nil // Reset on retry
		opts.Page = 0  // Reset pagination on retry

		for {
			teams, resp, err := c.client.Repositories.ListTeams(c.ctx, owner, name, opts)
			if err != nil {
				return WrapGitHubError(err, fmt.Sprintf("teams for %s/%s", owner, name))
			}

			for _, team := range teams {
				allTeams = append(allTeams, TeamAccess{
					TeamSlug:   team.GetSlug(),
					Permission: strings.ToLower(team.GetPermission()),
				})
			}

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}
		return nil
	}, DefaultRetryConfig())

	return allTeams, err
}

// AddTeamAccess adds team access to a repository
func (c *Client) AddTeamAccess(owner, name string, team TeamAccess) error {
	opts := &github.TeamAddTeamRepoOptions{
		Permission: team.Permission,
	}

	return WithRetry(func() error {
		_, err := c.client.Teams.AddTeamRepoBySlug(c.ctx, owner, team.TeamSlug, owner, name, opts)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("team %s for %s/%s", team.TeamSlug, owner, name))
		}
		return nil
	}, DefaultRetryConfig())
}

// UpdateTeamAccess updates team access for a repository
func (c *Client) UpdateTeamAccess(owner, name string, team TeamAccess) error {
	// GitHub API doesn't have a separate update method, so we use add which updates if exists
	return c.AddTeamAccess(owner, name, team)
}

// RemoveTeamAccess removes team access from a repository
func (c *Client) RemoveTeamAccess(owner, name, teamSlug string) error {
	return WithRetry(func() error {
		_, err := c.client.Teams.RemoveTeamRepoBySlug(c.ctx, owner, teamSlug, owner, name)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("team %s for %s/%s", teamSlug, owner, name))
		}
		return nil
	}, DefaultRetryConfig())
}

// ListWebhooks lists all webhooks for a repository
func (c *Client) ListWebhooks(owner, name string) ([]Webhook, error) {
	opts := &github.ListOptions{PerPage: 100}

	var allWebhooks []Webhook

	err := WithRetry(func() error {
		allWebhooks = nil // Reset on retry
		opts.Page = 0     // Reset pagination on retry

		for {
			webhooks, resp, err := c.client.Repositories.ListHooks(c.ctx, owner, name, opts)
			if err != nil {
				return WrapGitHubError(err, fmt.Sprintf("webhooks for %s/%s", owner, name))
			}

			for _, hook := range webhooks {
				webhook := Webhook{
					ID:     hook.GetID(),
					Active: hook.GetActive(),
					Events: hook.Events,
				}

				// Extract URL from config
				if hook.Config != nil {
					webhook.URL = hook.Config.GetURL()
				}

				allWebhooks = append(allWebhooks, webhook)
			}

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}
		return nil
	}, DefaultRetryConfig())

	return allWebhooks, err
}

// CreateWebhook creates a new webhook for a repository
func (c *Client) CreateWebhook(owner, name string, webhook Webhook) error {
	config := &github.HookConfig{
		URL:         github.String(webhook.URL),
		ContentType: github.String("json"),
	}

	if webhook.Secret != "" {
		config.Secret = github.String(webhook.Secret)
	}

	hook := &github.Hook{
		Name:   github.String("web"),
		Config: config,
		Events: webhook.Events,
		Active: github.Bool(webhook.Active),
	}

	return WithRetry(func() error {
		_, _, err := c.client.Repositories.CreateHook(c.ctx, owner, name, hook)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("webhook for %s/%s", owner, name))
		}
		return nil
	}, DefaultRetryConfig())
}

// UpdateWebhook updates an existing webhook
func (c *Client) UpdateWebhook(owner, name string, webhookID int64, webhook Webhook) error {
	config := &github.HookConfig{
		URL:         github.String(webhook.URL),
		ContentType: github.String("json"),
	}

	if webhook.Secret != "" {
		config.Secret = github.String(webhook.Secret)
	}

	hook := &github.Hook{
		Config: config,
		Events: webhook.Events,
		Active: github.Bool(webhook.Active),
	}

	return WithRetry(func() error {
		_, _, err := c.client.Repositories.EditHook(c.ctx, owner, name, webhookID, hook)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("webhook %d for %s/%s", webhookID, owner, name))
		}
		return nil
	}, DefaultRetryConfig())
}

// DeleteWebhook deletes a webhook from a repository
func (c *Client) DeleteWebhook(owner, name string, webhookID int64) error {
	return WithRetry(func() error {
		_, err := c.client.Repositories.DeleteHook(c.ctx, owner, name, webhookID)
		if err != nil {
			return WrapGitHubError(err, fmt.Sprintf("webhook %d for %s/%s", webhookID, owner, name))
		}
		return nil
	}, DefaultRetryConfig())
}

// convertGitHubRepository converts a GitHub API repository to our internal type
func (c *Client) convertGitHubRepository(repo *github.Repository) *Repository {
	return &Repository{
		ID:          repo.GetID(),
		Name:        repo.GetName(),
		FullName:    repo.GetFullName(),
		Description: repo.GetDescription(),
		Private:     repo.GetPrivate(),
		Topics:      repo.Topics,
		Features: RepositoryFeatures{
			Issues:      repo.GetHasIssues(),
			Wiki:        repo.GetHasWiki(),
			Projects:    repo.GetHasProjects(),
			Discussions: repo.GetHasDiscussions(),
		},
		CreatedAt: repo.GetCreatedAt().Time,
		UpdatedAt: repo.GetUpdatedAt().Time,
	}
}

// convertGitHubBranchProtection converts GitHub API branch protection to our internal type
func (c *Client) convertGitHubBranchProtection(protection *github.Protection, branch string) *BranchProtection {
	bp := &BranchProtection{
		Pattern: branch,
	}

	if protection.RequiredStatusChecks != nil {
		if protection.RequiredStatusChecks.Contexts != nil {
			bp.RequiredStatusChecks = *protection.RequiredStatusChecks.Contexts
		}
		bp.RequireUpToDate = protection.RequiredStatusChecks.Strict
	}

	if protection.RequiredPullRequestReviews != nil {
		bp.RequiredReviews = protection.RequiredPullRequestReviews.RequiredApprovingReviewCount
		bp.DismissStaleReviews = protection.RequiredPullRequestReviews.DismissStaleReviews
		bp.RequireCodeOwnerReview = protection.RequiredPullRequestReviews.RequireCodeOwnerReviews
	}

	if protection.Restrictions != nil && len(protection.Restrictions.Users) > 0 {
		for _, user := range protection.Restrictions.Users {
			bp.RestrictPushes = append(bp.RestrictPushes, user.GetLogin())
		}
	}

	return bp
}
