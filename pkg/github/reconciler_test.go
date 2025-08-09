package github

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAPIClient is a mock implementation of APIClient for testing
type MockAPIClient struct {
	mock.Mock
}

func (m *MockAPIClient) GetRepository(owner, name string) (*Repository, error) {
	args := m.Called(owner, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Repository), args.Error(1)
}

func (m *MockAPIClient) CreateRepository(config RepositoryConfig) (*Repository, error) {
	args := m.Called(config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Repository), args.Error(1)
}

func (m *MockAPIClient) UpdateRepository(owner, name string, config RepositoryConfig) error {
	args := m.Called(owner, name, config)
	return args.Error(0)
}

func (m *MockAPIClient) GetBranchProtection(owner, name, branch string) (*BranchProtection, error) {
	args := m.Called(owner, name, branch)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*BranchProtection), args.Error(1)
}

func (m *MockAPIClient) CreateBranchProtection(owner, name, branch string, rules BranchProtectionRule) error {
	args := m.Called(owner, name, branch, rules)
	return args.Error(0)
}

func (m *MockAPIClient) UpdateBranchProtection(owner, name, branch string, rules BranchProtectionRule) error {
	args := m.Called(owner, name, branch, rules)
	return args.Error(0)
}

func (m *MockAPIClient) DeleteBranchProtection(owner, name, branch string) error {
	args := m.Called(owner, name, branch)
	return args.Error(0)
}

func (m *MockAPIClient) ListCollaborators(owner, name string) ([]Collaborator, error) {
	args := m.Called(owner, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Collaborator), args.Error(1)
}

func (m *MockAPIClient) AddCollaborator(owner, name, username string, permission string) error {
	args := m.Called(owner, name, username, permission)
	return args.Error(0)
}

func (m *MockAPIClient) RemoveCollaborator(owner, name, username string) error {
	args := m.Called(owner, name, username)
	return args.Error(0)
}

func (m *MockAPIClient) ListTeamAccess(owner, name string) ([]TeamAccess, error) {
	args := m.Called(owner, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]TeamAccess), args.Error(1)
}

func (m *MockAPIClient) AddTeamAccess(owner, name string, team TeamAccess) error {
	args := m.Called(owner, name, team)
	return args.Error(0)
}

func (m *MockAPIClient) UpdateTeamAccess(owner, name string, team TeamAccess) error {
	args := m.Called(owner, name, team)
	return args.Error(0)
}

func (m *MockAPIClient) RemoveTeamAccess(owner, name, teamSlug string) error {
	args := m.Called(owner, name, teamSlug)
	return args.Error(0)
}

func (m *MockAPIClient) ListWebhooks(owner, name string) ([]Webhook, error) {
	args := m.Called(owner, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Webhook), args.Error(1)
}

func (m *MockAPIClient) CreateWebhook(owner, name string, webhook Webhook) error {
	args := m.Called(owner, name, webhook)
	return args.Error(0)
}

func (m *MockAPIClient) UpdateWebhook(owner, name string, webhookID int64, webhook Webhook) error {
	args := m.Called(owner, name, webhookID, webhook)
	return args.Error(0)
}

func (m *MockAPIClient) DeleteWebhook(owner, name string, webhookID int64) error {
	args := m.Called(owner, name, webhookID)
	return args.Error(0)
}

func TestNewReconciler(t *testing.T) {
	client := &MockAPIClient{}
	owner := "test-owner"

	reconciler := NewReconciler(client, owner)

	assert.NotNil(t, reconciler)
	// Test that the reconciler implements the interface correctly
	assert.Implements(t, (*Reconciler)(nil), reconciler)
}

func TestReconciler_Plan_NewRepository(t *testing.T) {
	client := &MockAPIClient{}
	reconciler := NewReconciler(client, "test-owner")

	config := RepositoryConfig{
		Name:        "test-repo",
		Description: "Test repository",
		Private:     true,
		Topics:      []string{"test", "golang"},
		Features: RepositoryFeatures{
			Issues: true,
			Wiki:   false,
		},
		Collaborators: []Collaborator{
			{Username: "user1", Permission: "write"},
		},
		BranchRules: []BranchProtectionRule{
			{Pattern: "main", RequiredReviews: 1},
		},
	}

	// Mock repository not found (new repository)
	client.On("GetRepository", "test-owner", "test-repo").Return(nil, errors.New("not found"))

	plan, err := reconciler.Plan(config)

	assert.NoError(t, err)
	assert.NotNil(t, plan)
	assert.NotNil(t, plan.Repository)
	assert.Equal(t, ChangeTypeCreate, plan.Repository.Type)
	assert.Equal(t, "test-repo", plan.Repository.After.Name)
	assert.Equal(t, "Test repository", plan.Repository.After.Description)
	assert.True(t, plan.Repository.After.Private)
	assert.Equal(t, []string{"test", "golang"}, plan.Repository.After.Topics)

	// Check that other resources are planned for creation
	assert.Len(t, plan.BranchRules, 1)
	assert.Equal(t, ChangeTypeCreate, plan.BranchRules[0].Type)
	assert.Equal(t, "main", plan.BranchRules[0].Branch)

	assert.Len(t, plan.Collaborators, 1)
	assert.Equal(t, ChangeTypeCreate, plan.Collaborators[0].Type)
	assert.Equal(t, "user1", plan.Collaborators[0].After.Username)

	client.AssertExpectations(t)
}

func TestReconciler_Plan_ExistingRepositoryNoChanges(t *testing.T) {
	client := &MockAPIClient{}
	reconciler := NewReconciler(client, "test-owner")

	config := RepositoryConfig{
		Name:        "test-repo",
		Description: "Test repository",
		Private:     true,
		Topics:      []string{"test", "golang"},
		Features: RepositoryFeatures{
			Issues: true,
			Wiki:   false,
		},
	}

	existingRepo := &Repository{
		ID:          123,
		Name:        "test-repo",
		Description: "Test repository",
		Private:     true,
		Topics:      []string{"test", "golang"},
		Features: RepositoryFeatures{
			Issues: true,
			Wiki:   false,
		},
	}

	client.On("GetRepository", "test-owner", "test-repo").Return(existingRepo, nil)
	client.On("ListCollaborators", "test-owner", "test-repo").Return([]Collaborator{}, nil)
	client.On("ListTeamAccess", "test-owner", "test-repo").Return([]TeamAccess{}, nil)
	client.On("ListWebhooks", "test-owner", "test-repo").Return([]Webhook{}, nil)

	plan, err := reconciler.Plan(config)

	assert.NoError(t, err)
	assert.NotNil(t, plan)
	assert.Nil(t, plan.Repository) // No changes needed
	assert.Empty(t, plan.BranchRules)
	assert.Empty(t, plan.Collaborators)
	assert.Empty(t, plan.Teams)
	assert.Empty(t, plan.Webhooks)

	client.AssertExpectations(t)
}

func TestReconciler_Plan_ExistingRepositoryWithChanges(t *testing.T) {
	client := &MockAPIClient{}
	reconciler := NewReconciler(client, "test-owner")

	config := RepositoryConfig{
		Name:        "test-repo",
		Description: "Updated description",
		Private:     false,                       // Changed from true
		Topics:      []string{"test", "updated"}, // Changed topics
		Features: RepositoryFeatures{
			Issues: true,
			Wiki:   true, // Changed from false
		},
	}

	existingRepo := &Repository{
		ID:          123,
		Name:        "test-repo",
		Description: "Test repository",
		Private:     true,
		Topics:      []string{"test", "golang"},
		Features: RepositoryFeatures{
			Issues: true,
			Wiki:   false,
		},
	}

	client.On("GetRepository", "test-owner", "test-repo").Return(existingRepo, nil)
	client.On("ListCollaborators", "test-owner", "test-repo").Return([]Collaborator{}, nil)
	client.On("ListTeamAccess", "test-owner", "test-repo").Return([]TeamAccess{}, nil)
	client.On("ListWebhooks", "test-owner", "test-repo").Return([]Webhook{}, nil)

	plan, err := reconciler.Plan(config)

	assert.NoError(t, err)
	assert.NotNil(t, plan)
	assert.NotNil(t, plan.Repository)
	assert.Equal(t, ChangeTypeUpdate, plan.Repository.Type)
	assert.Equal(t, existingRepo, plan.Repository.Before)
	assert.Equal(t, "Updated description", plan.Repository.After.Description)
	assert.False(t, plan.Repository.After.Private)
	assert.Equal(t, []string{"test", "updated"}, plan.Repository.After.Topics)
	assert.True(t, plan.Repository.After.Features.Wiki)

	client.AssertExpectations(t)
}

func TestReconciler_Plan_BranchProtectionChanges(t *testing.T) {
	client := &MockAPIClient{}
	reconciler := NewReconciler(client, "test-owner")

	config := RepositoryConfig{
		Name: "test-repo",
		BranchRules: []BranchProtectionRule{
			{
				Pattern:         "main",
				RequiredReviews: 2,
				RequireUpToDate: true,
			},
			{
				Pattern:         "develop",
				RequiredReviews: 1,
			},
		},
	}

	existingRepo := &Repository{ID: 123, Name: "test-repo"}

	// Mock existing branch protection for main (needs update)
	existingMainProtection := &BranchProtection{
		Pattern:         "main",
		RequiredReviews: 1, // Different from config
		RequireUpToDate: false,
	}

	client.On("GetRepository", "test-owner", "test-repo").Return(existingRepo, nil)
	client.On("GetBranchProtection", "test-owner", "test-repo", "main").Return(existingMainProtection, nil)
	client.On("GetBranchProtection", "test-owner", "test-repo", "develop").Return(nil, errors.New("not found"))
	client.On("ListCollaborators", "test-owner", "test-repo").Return([]Collaborator{}, nil)
	client.On("ListTeamAccess", "test-owner", "test-repo").Return([]TeamAccess{}, nil)
	client.On("ListWebhooks", "test-owner", "test-repo").Return([]Webhook{}, nil)

	plan, err := reconciler.Plan(config)

	assert.NoError(t, err)
	assert.NotNil(t, plan)
	assert.Len(t, plan.BranchRules, 2)

	// Check main branch update
	mainChange := plan.BranchRules[0]
	assert.Equal(t, ChangeTypeUpdate, mainChange.Type)
	assert.Equal(t, "main", mainChange.Branch)
	assert.Equal(t, 1, mainChange.Before.RequiredReviews)
	assert.Equal(t, 2, mainChange.After.RequiredReviews)

	// Check develop branch creation
	developChange := plan.BranchRules[1]
	assert.Equal(t, ChangeTypeCreate, developChange.Type)
	assert.Equal(t, "develop", developChange.Branch)
	assert.Equal(t, 1, developChange.After.RequiredReviews)

	client.AssertExpectations(t)
}

func TestReconciler_Plan_CollaboratorChanges(t *testing.T) {
	client := &MockAPIClient{}
	reconciler := NewReconciler(client, "test-owner")

	config := RepositoryConfig{
		Name: "test-repo",
		Collaborators: []Collaborator{
			{Username: "user1", Permission: "write"},
			{Username: "user2", Permission: "admin"}, // Updated permission
		},
	}

	existingRepo := &Repository{ID: 123, Name: "test-repo"}
	existingCollaborators := []Collaborator{
		{Username: "user2", Permission: "write"}, // Will be updated
		{Username: "user3", Permission: "read"},  // Will be removed
	}

	client.On("GetRepository", "test-owner", "test-repo").Return(existingRepo, nil)
	client.On("ListCollaborators", "test-owner", "test-repo").Return(existingCollaborators, nil)
	client.On("ListTeamAccess", "test-owner", "test-repo").Return([]TeamAccess{}, nil)
	client.On("ListWebhooks", "test-owner", "test-repo").Return([]Webhook{}, nil)

	plan, err := reconciler.Plan(config)

	assert.NoError(t, err)
	assert.NotNil(t, plan)
	assert.Len(t, plan.Collaborators, 3)

	// Find changes by type
	var createChanges, updateChanges, deleteChanges []CollaboratorChange
	for _, change := range plan.Collaborators {
		switch change.Type {
		case ChangeTypeCreate:
			createChanges = append(createChanges, change)
		case ChangeTypeUpdate:
			updateChanges = append(updateChanges, change)
		case ChangeTypeDelete:
			deleteChanges = append(deleteChanges, change)
		}
	}

	assert.Len(t, createChanges, 1)
	assert.Equal(t, "user1", createChanges[0].After.Username)

	assert.Len(t, updateChanges, 1)
	assert.Equal(t, "user2", updateChanges[0].After.Username)
	assert.Equal(t, "admin", updateChanges[0].After.Permission)

	assert.Len(t, deleteChanges, 1)
	assert.Equal(t, "user3", deleteChanges[0].Before.Username)

	client.AssertExpectations(t)
}

func TestReconciler_Apply_CreateRepository(t *testing.T) {
	client := &MockAPIClient{}
	reconciler := NewReconciler(client, "test-owner")

	plan := &ReconciliationPlan{
		Repository: &RepositoryChange{
			Type: ChangeTypeCreate,
			After: &Repository{
				Name:        "test-repo",
				Description: "Test repository",
				Private:     true,
			},
		},
	}

	expectedConfig := RepositoryConfig{
		Name:        "test-repo",
		Description: "Test repository",
		Private:     true,
	}

	client.On("CreateRepository", expectedConfig).Return(&Repository{ID: 123}, nil)

	err := reconciler.Apply(plan)

	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestReconciler_Apply_UpdateRepository(t *testing.T) {
	client := &MockAPIClient{}
	reconciler := NewReconciler(client, "test-owner")

	plan := &ReconciliationPlan{
		Repository: &RepositoryChange{
			Type: ChangeTypeUpdate,
			After: &Repository{
				Name:        "test-repo",
				Description: "Updated description",
				Private:     false,
			},
		},
	}

	expectedConfig := RepositoryConfig{
		Name:        "test-repo",
		Description: "Updated description",
		Private:     false,
	}

	client.On("UpdateRepository", "test-owner", "test-repo", expectedConfig).Return(nil)

	err := reconciler.Apply(plan)

	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestReconciler_Validate(t *testing.T) {
	client := &MockAPIClient{}
	reconciler := NewReconciler(client, "test-owner")

	validConfig := RepositoryConfig{
		Name:        "test-repo",
		Description: "Test repository",
		Private:     true,
		Collaborators: []Collaborator{
			{Username: "user1", Permission: "write"},
		},
	}

	err := reconciler.Validate(validConfig)
	assert.NoError(t, err)

	invalidConfig := RepositoryConfig{
		Name: "", // Invalid: empty name
	}

	err = reconciler.Validate(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository name is required")
}

func TestReconciler_StringSlicesEqual(t *testing.T) {
	client := &MockAPIClient{}
	r := NewReconciler(client, "test-owner").(*reconciler)

	// Test equal slices
	assert.True(t, r.stringSlicesEqual([]string{"a", "b"}, []string{"b", "a"}))
	assert.True(t, r.stringSlicesEqual([]string{}, []string{}))
	assert.True(t, r.stringSlicesEqual(nil, nil))

	// Test unequal slices
	assert.False(t, r.stringSlicesEqual([]string{"a", "b"}, []string{"a", "c"}))
	assert.False(t, r.stringSlicesEqual([]string{"a"}, []string{"a", "b"}))
	assert.False(t, r.stringSlicesEqual([]string{"a"}, nil))
}

func TestReconciler_BranchProtectionsEqual(t *testing.T) {
	client := &MockAPIClient{}
	r := NewReconciler(client, "test-owner").(*reconciler)

	bp1 := &BranchProtection{
		Pattern:                "main",
		RequiredStatusChecks:   []string{"ci", "test"},
		RequireUpToDate:        true,
		RequiredReviews:        2,
		DismissStaleReviews:    true,
		RequireCodeOwnerReview: false,
		RestrictPushes:         []string{"admin"},
	}

	bp2 := &BranchProtection{
		Pattern:                "main",
		RequiredStatusChecks:   []string{"test", "ci"}, // Different order
		RequireUpToDate:        true,
		RequiredReviews:        2,
		DismissStaleReviews:    true,
		RequireCodeOwnerReview: false,
		RestrictPushes:         []string{"admin"},
	}

	bp3 := &BranchProtection{
		Pattern:                "main",
		RequiredStatusChecks:   []string{"ci", "test"},
		RequireUpToDate:        true,
		RequiredReviews:        1, // Different value
		DismissStaleReviews:    true,
		RequireCodeOwnerReview: false,
		RestrictPushes:         []string{"admin"},
	}

	assert.True(t, r.branchProtectionsEqual(bp1, bp2))
	assert.False(t, r.branchProtectionsEqual(bp1, bp3))
}

func TestReconciler_WebhooksEqual(t *testing.T) {
	client := &MockAPIClient{}
	r := NewReconciler(client, "test-owner").(*reconciler)

	wh1 := &Webhook{
		URL:    "https://example.com/webhook",
		Events: []string{"push", "pull_request"},
		Secret: "secret123",
		Active: true,
	}

	wh2 := &Webhook{
		URL:    "https://example.com/webhook",
		Events: []string{"pull_request", "push"}, // Different order
		Secret: "secret123",
		Active: true,
	}

	wh3 := &Webhook{
		URL:    "https://example.com/webhook",
		Events: []string{"push", "pull_request"},
		Secret: "different-secret", // Different secret
		Active: true,
	}

	assert.True(t, r.webhooksEqual(wh1, wh2))
	assert.False(t, r.webhooksEqual(wh1, wh3))
}
