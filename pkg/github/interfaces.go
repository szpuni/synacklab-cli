package github

// APIClient defines the interface for GitHub API operations
type APIClient interface {
	// Repository operations
	GetRepository(owner, name string) (*Repository, error)
	CreateRepository(config RepositoryConfig) (*Repository, error)
	UpdateRepository(owner, name string, config RepositoryConfig) error

	// Branch protection operations
	GetBranchProtection(owner, name, branch string) (*BranchProtection, error)
	CreateBranchProtection(owner, name, branch string, rules BranchProtectionRule) error
	UpdateBranchProtection(owner, name, branch string, rules BranchProtectionRule) error
	DeleteBranchProtection(owner, name, branch string) error

	// Collaborator operations
	ListCollaborators(owner, name string) ([]Collaborator, error)
	AddCollaborator(owner, name, username string, permission string) error
	RemoveCollaborator(owner, name, username string) error

	// Team operations
	ListTeamAccess(owner, name string) ([]TeamAccess, error)
	AddTeamAccess(owner, name string, team TeamAccess) error
	UpdateTeamAccess(owner, name string, team TeamAccess) error
	RemoveTeamAccess(owner, name, teamSlug string) error

	// Webhook operations
	ListWebhooks(owner, name string) ([]Webhook, error)
	CreateWebhook(owner, name string, webhook Webhook) error
	UpdateWebhook(owner, name string, webhookID int64, webhook Webhook) error
	DeleteWebhook(owner, name string, webhookID int64) error
}

// Reconciler defines the interface for state reconciliation operations
type Reconciler interface {
	Plan(config RepositoryConfig) (*ReconciliationPlan, error)
	Apply(plan *ReconciliationPlan) error
	Validate(config RepositoryConfig) error
}

// ChangeType represents the type of change in a reconciliation plan
type ChangeType string

const (
	ChangeTypeCreate ChangeType = "create"
	ChangeTypeUpdate ChangeType = "update"
	ChangeTypeDelete ChangeType = "delete"
)

// ReconciliationPlan represents a plan of changes to be applied
type ReconciliationPlan struct {
	Repository    *RepositoryChange    `json:"repository,omitempty"`
	BranchRules   []BranchRuleChange   `json:"branch_rules,omitempty"`
	Collaborators []CollaboratorChange `json:"collaborators,omitempty"`
	Teams         []TeamChange         `json:"teams,omitempty"`
	Webhooks      []WebhookChange      `json:"webhooks,omitempty"`
}

// RepositoryChange represents a change to repository settings
type RepositoryChange struct {
	Type   ChangeType  `json:"type"`
	Before *Repository `json:"before,omitempty"`
	After  *Repository `json:"after,omitempty"`
}

// BranchRuleChange represents a change to branch protection rules
type BranchRuleChange struct {
	Type   ChangeType        `json:"type"`
	Branch string            `json:"branch"`
	Before *BranchProtection `json:"before,omitempty"`
	After  *BranchProtection `json:"after,omitempty"`
}

// CollaboratorChange represents a change to collaborator access
type CollaboratorChange struct {
	Type   ChangeType    `json:"type"`
	Before *Collaborator `json:"before,omitempty"`
	After  *Collaborator `json:"after,omitempty"`
}

// TeamChange represents a change to team access
type TeamChange struct {
	Type   ChangeType  `json:"type"`
	Before *TeamAccess `json:"before,omitempty"`
	After  *TeamAccess `json:"after,omitempty"`
}

// WebhookChange represents a change to webhook configuration
type WebhookChange struct {
	Type   ChangeType `json:"type"`
	Before *Webhook   `json:"before,omitempty"`
	After  *Webhook   `json:"after,omitempty"`
}
