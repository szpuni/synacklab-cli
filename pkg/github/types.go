package github

import "time"

// Repository represents a GitHub repository
type Repository struct {
	ID          int64              `json:"id"`
	Name        string             `json:"name"`
	FullName    string             `json:"full_name"`
	Description string             `json:"description"`
	Private     bool               `json:"private"`
	Topics      []string           `json:"topics"`
	Features    RepositoryFeatures `json:"features"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// RepositoryFeatures represents repository feature settings
type RepositoryFeatures struct {
	Issues      bool `json:"has_issues" yaml:"issues"`
	Wiki        bool `json:"has_wiki" yaml:"wiki"`
	Projects    bool `json:"has_projects" yaml:"projects"`
	Discussions bool `json:"has_discussions" yaml:"discussions"`
}

// BranchProtection represents branch protection rule settings
type BranchProtection struct {
	Pattern                string   `json:"pattern"`
	RequiredStatusChecks   []string `json:"required_status_checks"`
	RequireUpToDate        bool     `json:"require_up_to_date"`
	RequiredReviews        int      `json:"required_reviews"`
	DismissStaleReviews    bool     `json:"dismiss_stale_reviews"`
	RequireCodeOwnerReview bool     `json:"require_code_owner_review"`
	RestrictPushes         []string `json:"restrict_pushes"`
}

// Collaborator represents a repository collaborator
type Collaborator struct {
	Username   string `json:"username" yaml:"username"`
	Permission string `json:"permission" yaml:"permission"` // read, write, admin
}

// TeamAccess represents team access to a repository
type TeamAccess struct {
	TeamSlug   string `json:"team_slug" yaml:"team"`
	Permission string `json:"permission" yaml:"permission"` // read, write, admin
}

// Webhook represents a repository webhook
type Webhook struct {
	ID     int64    `json:"id,omitempty"`
	URL    string   `json:"url" yaml:"url"`
	Events []string `json:"events" yaml:"events"`
	Secret string   `json:"secret,omitempty" yaml:"secret,omitempty"`
	Active bool     `json:"active" yaml:"active"`
}
