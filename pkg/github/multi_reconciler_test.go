package github

import (
	"errors"
	"strings"
	"testing"
)

// mockAPIClient implements APIClient interface for testing
type mockAPIClient struct {
	repositories      map[string]*Repository
	branchProtections map[string]map[string]*BranchProtection
	collaborators     map[string][]Collaborator
	teams             map[string][]TeamAccess
	webhooks          map[string][]Webhook
	errors            map[string]error
}

func newMockAPIClient() *mockAPIClient {
	return &mockAPIClient{
		repositories:      make(map[string]*Repository),
		branchProtections: make(map[string]map[string]*BranchProtection),
		collaborators:     make(map[string][]Collaborator),
		teams:             make(map[string][]TeamAccess),
		webhooks:          make(map[string][]Webhook),
		errors:            make(map[string]error),
	}
}

func (m *mockAPIClient) GetRepository(owner, name string) (*Repository, error) {
	key := owner + "/" + name
	if err, exists := m.errors[key]; exists {
		return nil, err
	}
	if repo, exists := m.repositories[key]; exists {
		return repo, nil
	}
	return nil, errors.New("repository not found")
}

func (m *mockAPIClient) CreateRepository(config RepositoryConfig) (*Repository, error) {
	key := "create/" + config.Name
	if err, exists := m.errors[key]; exists {
		return nil, err
	}

	repo := &Repository{
		Name:        config.Name,
		Description: config.Description,
		Private:     config.Private,
		Topics:      config.Topics,
		Features:    config.Features,
	}

	// Store the created repository
	repoKey := "test-owner/" + config.Name
	m.repositories[repoKey] = repo

	return repo, nil
}

func (m *mockAPIClient) UpdateRepository(_, _ string, _ RepositoryConfig) error {
	return nil
}

func (m *mockAPIClient) GetBranchProtection(owner, name, branch string) (*BranchProtection, error) {
	key := owner + "/" + name
	if branchMap, exists := m.branchProtections[key]; exists {
		if protection, exists := branchMap[branch]; exists {
			return protection, nil
		}
	}
	return nil, errors.New("branch protection not found")
}

func (m *mockAPIClient) CreateBranchProtection(_, _, _ string, _ BranchProtectionRule) error {
	return nil
}

func (m *mockAPIClient) UpdateBranchProtection(_, _, _ string, _ BranchProtectionRule) error {
	return nil
}

func (m *mockAPIClient) DeleteBranchProtection(_, _, _ string) error {
	return nil
}

func (m *mockAPIClient) ListCollaborators(owner, name string) ([]Collaborator, error) {
	key := owner + "/" + name
	if collaborators, exists := m.collaborators[key]; exists {
		return collaborators, nil
	}
	return []Collaborator{}, nil
}

func (m *mockAPIClient) AddCollaborator(_, _, _ string, _ string) error {
	return nil
}

func (m *mockAPIClient) RemoveCollaborator(_, _, _ string) error {
	return nil
}

func (m *mockAPIClient) ListTeamAccess(owner, name string) ([]TeamAccess, error) {
	key := owner + "/" + name
	if teams, exists := m.teams[key]; exists {
		return teams, nil
	}
	return []TeamAccess{}, nil
}

func (m *mockAPIClient) AddTeamAccess(_, _ string, _ TeamAccess) error {
	return nil
}

func (m *mockAPIClient) UpdateTeamAccess(_, _ string, _ TeamAccess) error {
	return nil
}

func (m *mockAPIClient) RemoveTeamAccess(_, _, _ string) error {
	return nil
}

func (m *mockAPIClient) ListWebhooks(owner, name string) ([]Webhook, error) {
	key := owner + "/" + name
	if webhooks, exists := m.webhooks[key]; exists {
		return webhooks, nil
	}
	return []Webhook{}, nil
}

func (m *mockAPIClient) CreateWebhook(_, _ string, _ Webhook) error {
	return nil
}

func (m *mockAPIClient) UpdateWebhook(_, _ string, _ int64, _ Webhook) error {
	return nil
}

func (m *mockAPIClient) DeleteWebhook(_, _ string, _ int64) error {
	return nil
}

func TestNewMultiReconciler(t *testing.T) {
	client := newMockAPIClient()
	owner := "test-owner"

	reconciler := NewMultiReconciler(client, owner)

	if reconciler == nil {
		t.Fatal("NewMultiReconciler returned nil")
	}
}

func TestMultiReconciler_PlanAll_NilConfig(t *testing.T) {
	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	_, err := reconciler.PlanAll(nil, nil)

	if err == nil {
		t.Fatal("Expected error for nil configuration")
	}
	if err.Error() != "multi-repository configuration cannot be nil" {
		t.Errorf("Expected specific error message, got: %s", err.Error())
	}
}

func TestMultiReconciler_PlanAll_InvalidRepositoryFilter(t *testing.T) {
	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	config := &MultiRepositoryConfig{
		Repositories: []RepositoryConfig{
			{Name: "repo1"},
			{Name: "repo2"},
		},
	}

	_, err := reconciler.PlanAll(config, []string{"repo1", "nonexistent"})

	if err == nil {
		t.Fatal("Expected error for invalid repository filter")
	}
	if !contains(err.Error(), "repositories not found in configuration: nonexistent") {
		t.Errorf("Expected specific error message, got: %s", err.Error())
	}
}

func TestMultiReconciler_PlanAll_AllRepositories(t *testing.T) {
	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	// Set up mock repositories that don't exist (will be created)
	config := &MultiRepositoryConfig{
		Repositories: []RepositoryConfig{
			{
				Name:        "repo1",
				Description: "First repository",
				Private:     true,
			},
			{
				Name:        "repo2",
				Description: "Second repository",
				Private:     false,
			},
		},
	}

	plans, err := reconciler.PlanAll(config, nil)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(plans) != 2 {
		t.Errorf("Expected 2 plans, got %d", len(plans))
	}

	if _, exists := plans["repo1"]; !exists {
		t.Error("Expected plan for repo1")
	}

	if _, exists := plans["repo2"]; !exists {
		t.Error("Expected plan for repo2")
	}
}

func TestMultiReconciler_PlanAll_FilteredRepositories(t *testing.T) {
	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	config := &MultiRepositoryConfig{
		Repositories: []RepositoryConfig{
			{Name: "repo1", Description: "First repository"},
			{Name: "repo2", Description: "Second repository"},
			{Name: "repo3", Description: "Third repository"},
		},
	}

	plans, err := reconciler.PlanAll(config, []string{"repo1", "repo3"})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(plans) != 2 {
		t.Errorf("Expected 2 plans, got %d", len(plans))
	}

	if _, exists := plans["repo1"]; !exists {
		t.Error("Expected plan for repo1")
	}

	if _, exists := plans["repo3"]; !exists {
		t.Error("Expected plan for repo3")
	}

	if _, exists := plans["repo2"]; exists {
		t.Error("Did not expect plan for repo2")
	}
}

func TestMultiReconciler_PlanAll_WithDefaults(t *testing.T) {
	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	private := true
	config := &MultiRepositoryConfig{
		Defaults: &RepositoryDefaults{
			Description: "Default description",
			Private:     &private,
			Topics:      []string{"default-topic"},
		},
		Repositories: []RepositoryConfig{
			{
				Name: "repo1",
				// Will inherit defaults
			},
			{
				Name:        "repo2",
				Description: "Custom description", // Override default
				Topics:      []string{"custom-topic"},
			},
		},
	}

	plans, err := reconciler.PlanAll(config, nil)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(plans) != 2 {
		t.Errorf("Expected 2 plans, got %d", len(plans))
	}

	// Verify plans exist
	repo1Plan, exists := plans["repo1"]
	if !exists {
		t.Fatal("Expected plan for repo1")
	}

	repo2Plan, exists := plans["repo2"]
	if !exists {
		t.Fatal("Expected plan for repo2")
	}

	// Both should have repository creation plans since they don't exist
	if repo1Plan.Repository == nil || repo1Plan.Repository.Type != ChangeTypeCreate {
		t.Error("Expected repo1 to have create repository change")
	}

	if repo2Plan.Repository == nil || repo2Plan.Repository.Type != ChangeTypeCreate {
		t.Error("Expected repo2 to have create repository change")
	}
}

func TestMultiReconciler_ApplyAll_NilPlans(t *testing.T) {
	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	_, err := reconciler.ApplyAll(nil)

	if err == nil {
		t.Fatal("Expected error for nil plans")
	}
	if err.Error() != "reconciliation plans cannot be nil" {
		t.Errorf("Expected specific error message, got: %s", err.Error())
	}
}

func TestMultiReconciler_ApplyAll_EmptyPlans(t *testing.T) {
	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	plans := make(map[string]*ReconciliationPlan)

	result, err := reconciler.ApplyAll(plans)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.TotalRepositories != 0 {
		t.Errorf("Expected 0 total repositories, got %d", result.Summary.TotalRepositories)
	}
}

func TestMultiReconciler_ApplyAll_SuccessfulApplication(t *testing.T) {
	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	plans := map[string]*ReconciliationPlan{
		"repo1": {
			Repository: &RepositoryChange{
				Type: ChangeTypeCreate,
				After: &Repository{
					Name:        "repo1",
					Description: "Test repository",
				},
			},
		},
		"repo2": {
			Repository: &RepositoryChange{
				Type: ChangeTypeCreate,
				After: &Repository{
					Name:        "repo2",
					Description: "Another test repository",
				},
			},
		},
	}

	result, err := reconciler.ApplyAll(plans)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.TotalRepositories != 2 {
		t.Errorf("Expected 2 total repositories, got %d", result.Summary.TotalRepositories)
	}

	if result.Summary.SuccessCount != 2 {
		t.Errorf("Expected 2 successful repositories, got %d", result.Summary.SuccessCount)
	}

	if result.Summary.FailureCount != 0 {
		t.Errorf("Expected 0 failed repositories, got %d", result.Summary.FailureCount)
	}

	if len(result.Succeeded) != 2 {
		t.Errorf("Expected 2 succeeded repositories, got %d", len(result.Succeeded))
	}

	if len(result.Failed) != 0 {
		t.Errorf("Expected 0 failed repositories, got %d", len(result.Failed))
	}
}

func TestMultiReconciler_ApplyAll_WithSkippedPlans(t *testing.T) {
	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	plans := map[string]*ReconciliationPlan{
		"repo1": {
			Repository: &RepositoryChange{
				Type: ChangeTypeCreate,
				After: &Repository{
					Name: "repo1",
				},
			},
		},
		"repo2": nil, // This should be skipped
	}

	result, err := reconciler.ApplyAll(plans)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.TotalRepositories != 2 {
		t.Errorf("Expected 2 total repositories, got %d", result.Summary.TotalRepositories)
	}

	if result.Summary.SuccessCount != 1 {
		t.Errorf("Expected 1 successful repository, got %d", result.Summary.SuccessCount)
	}

	if result.Summary.SkippedCount != 1 {
		t.Errorf("Expected 1 skipped repository, got %d", result.Summary.SkippedCount)
	}

	if len(result.Skipped) != 1 || result.Skipped[0] != "repo2" {
		t.Errorf("Expected repo2 to be skipped, got: %v", result.Skipped)
	}
}

func TestMultiReconciler_ApplyAll_MixedSuccessFailure(t *testing.T) {
	client := newMockAPIClient()

	// Set up mock to fail for repo2
	client.errors["create/repo2"] = errors.New("repository creation failed")

	reconciler := NewMultiReconciler(client, "test-owner")

	plans := map[string]*ReconciliationPlan{
		"repo1": {
			Repository: &RepositoryChange{
				Type: ChangeTypeCreate,
				After: &Repository{
					Name: "repo1",
				},
			},
		},
		"repo2": {
			Repository: &RepositoryChange{
				Type: ChangeTypeCreate,
				After: &Repository{
					Name: "repo2",
				},
			},
		},
		"repo3": {
			Repository: &RepositoryChange{
				Type: ChangeTypeCreate,
				After: &Repository{
					Name: "repo3",
				},
			},
		},
	}

	result, err := reconciler.ApplyAll(plans)

	// Should return partial failure error
	if err == nil {
		t.Fatal("Expected partial failure error")
	}

	multiErr, ok := err.(*MultiRepoError)
	if !ok {
		t.Fatalf("Expected MultiRepoError, got %T", err)
	}

	if !multiErr.IsPartialFailure() {
		t.Error("Expected partial failure")
	}

	if result.Summary.TotalRepositories != 3 {
		t.Errorf("Expected 3 total repositories, got %d", result.Summary.TotalRepositories)
	}

	if result.Summary.SuccessCount != 2 {
		t.Errorf("Expected 2 successful repositories, got %d", result.Summary.SuccessCount)
	}

	if result.Summary.FailureCount != 1 {
		t.Errorf("Expected 1 failed repository, got %d", result.Summary.FailureCount)
	}

	if len(result.Succeeded) != 2 {
		t.Errorf("Expected 2 succeeded repositories, got %d", len(result.Succeeded))
	}

	if len(result.Failed) != 1 {
		t.Errorf("Expected 1 failed repository, got %d", len(result.Failed))
	}

	if _, exists := result.Failed["repo2"]; !exists {
		t.Error("Expected repo2 to be in failed list")
	}
}

func TestMultiReconciler_ApplyAll_CompleteFailure(t *testing.T) {
	client := newMockAPIClient()

	// Set up mock to fail for all repositories
	client.errors["create/repo1"] = errors.New("repository creation failed")
	client.errors["create/repo2"] = errors.New("authentication failed")

	reconciler := NewMultiReconciler(client, "test-owner")

	plans := map[string]*ReconciliationPlan{
		"repo1": {
			Repository: &RepositoryChange{
				Type: ChangeTypeCreate,
				After: &Repository{
					Name: "repo1",
				},
			},
		},
		"repo2": {
			Repository: &RepositoryChange{
				Type: ChangeTypeCreate,
				After: &Repository{
					Name: "repo2",
				},
			},
		},
	}

	result, err := reconciler.ApplyAll(plans)

	// Should return complete failure error
	if err == nil {
		t.Fatal("Expected complete failure error")
	}

	multiErr, ok := err.(*MultiRepoError)
	if !ok {
		t.Fatalf("Expected MultiRepoError, got %T", err)
	}

	if multiErr.IsPartialFailure() {
		t.Error("Expected complete failure, not partial")
	}

	if result.Summary.TotalRepositories != 2 {
		t.Errorf("Expected 2 total repositories, got %d", result.Summary.TotalRepositories)
	}

	if result.Summary.SuccessCount != 0 {
		t.Errorf("Expected 0 successful repositories, got %d", result.Summary.SuccessCount)
	}

	if result.Summary.FailureCount != 2 {
		t.Errorf("Expected 2 failed repositories, got %d", result.Summary.FailureCount)
	}

	if len(result.Failed) != 2 {
		t.Errorf("Expected 2 failed repositories, got %d", len(result.Failed))
	}
}

func TestMultiReconciler_ApplyAll_GracefulErrorHandling(t *testing.T) {
	client := newMockAPIClient()

	// Set up mock to fail for repo2 but succeed for others
	client.errors["create/repo2"] = errors.New("network timeout")

	reconciler := NewMultiReconciler(client, "test-owner")

	plans := map[string]*ReconciliationPlan{
		"repo1": {
			Repository: &RepositoryChange{
				Type: ChangeTypeCreate,
				After: &Repository{
					Name: "repo1",
				},
			},
		},
		"repo2": {
			Repository: &RepositoryChange{
				Type: ChangeTypeCreate,
				After: &Repository{
					Name: "repo2",
				},
			},
		},
		"repo3": {
			Repository: &RepositoryChange{
				Type: ChangeTypeCreate,
				After: &Repository{
					Name: "repo3",
				},
			},
		},
		"repo4": nil, // Should be skipped
	}

	result, err := reconciler.ApplyAll(plans)

	// Should continue processing despite repo2 failure
	if err == nil {
		t.Fatal("Expected partial failure error")
	}

	if result.Summary.TotalRepositories != 4 {
		t.Errorf("Expected 4 total repositories, got %d", result.Summary.TotalRepositories)
	}

	if result.Summary.SuccessCount != 2 {
		t.Errorf("Expected 2 successful repositories, got %d", result.Summary.SuccessCount)
	}

	if result.Summary.FailureCount != 1 {
		t.Errorf("Expected 1 failed repository, got %d", result.Summary.FailureCount)
	}

	if result.Summary.SkippedCount != 1 {
		t.Errorf("Expected 1 skipped repository, got %d", result.Summary.SkippedCount)
	}

	// Verify that repo1 and repo3 succeeded despite repo2 failing
	successSet := make(map[string]bool)
	for _, repo := range result.Succeeded {
		successSet[repo] = true
	}

	if !successSet["repo1"] {
		t.Error("Expected repo1 to succeed")
	}

	if !successSet["repo3"] {
		t.Error("Expected repo3 to succeed")
	}

	if _, exists := result.Failed["repo2"]; !exists {
		t.Error("Expected repo2 to be in failed list")
	}

	if len(result.Skipped) != 1 || result.Skipped[0] != "repo4" {
		t.Errorf("Expected repo4 to be skipped, got: %v", result.Skipped)
	}
}

func TestMultiReconciler_ApplyAll_ErrorContextEnhancement(t *testing.T) {
	client := newMockAPIClient()

	// Set up mock to return a specific error
	client.errors["create/repo1"] = errors.New("validation failed")

	reconciler := NewMultiReconciler(client, "test-owner")

	plans := map[string]*ReconciliationPlan{
		"repo1": {
			Repository: &RepositoryChange{
				Type: ChangeTypeCreate,
				After: &Repository{
					Name: "repo1",
				},
			},
		},
	}

	result, err := reconciler.ApplyAll(plans)

	if err == nil {
		t.Fatal("Expected error")
	}

	if result.Summary.FailureCount != 1 {
		t.Errorf("Expected 1 failed repository, got %d", result.Summary.FailureCount)
	}

	// Check that error contains repository context
	repoErr, exists := result.Failed["repo1"]
	if !exists {
		t.Fatal("Expected error for repo1")
	}

	if !strings.Contains(repoErr.Error(), "repo1") {
		t.Errorf("Expected error to contain repository name, got: %s", repoErr.Error())
	}
}

func TestMultiRepoError_Methods(t *testing.T) {
	result := &MultiRepoResult{
		Succeeded: []string{"repo1", "repo3"},
		Failed: map[string]error{
			"repo2": errors.New("failed"),
			"repo4": errors.New("also failed"),
		},
		Summary: MultiRepoSummary{
			TotalRepositories: 4,
			SuccessCount:      2,
			FailureCount:      2,
		},
	}

	partialErr := NewMultiRepoPartialFailureError(result)

	if !partialErr.IsPartialFailure() {
		t.Error("Expected partial failure")
	}

	failedRepos := partialErr.GetFailedRepositories()
	if len(failedRepos) != 2 {
		t.Errorf("Expected 2 failed repositories, got %d", len(failedRepos))
	}

	// Check that error message contains summary
	if !strings.Contains(partialErr.Error(), "2 succeeded") {
		t.Errorf("Expected error message to contain success count, got: %s", partialErr.Error())
	}

	if !strings.Contains(partialErr.Error(), "2 failed") {
		t.Errorf("Expected error message to contain failure count, got: %s", partialErr.Error())
	}

	completeErr := NewMultiRepoCompleteFailureError(result)
	if completeErr.IsPartialFailure() {
		t.Error("Expected complete failure, not partial")
	}
}

func TestMultiReconciler_ValidateAll_NilConfig(t *testing.T) {
	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	_, err := reconciler.ValidateAll(nil, nil)

	if err == nil {
		t.Fatal("Expected error for nil configuration")
	}
	if err.Error() != "multi-repository configuration cannot be nil" {
		t.Errorf("Expected specific error message, got: %s", err.Error())
	}
}

func TestMultiReconciler_ValidateAll_InvalidRepositoryFilter(t *testing.T) {
	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	config := &MultiRepositoryConfig{
		Repositories: []RepositoryConfig{
			{Name: "repo1"},
		},
	}

	_, err := reconciler.ValidateAll(config, []string{"nonexistent"})

	if err == nil {
		t.Fatal("Expected error for invalid repository filter")
	}
	if !contains(err.Error(), "repositories not found in configuration: nonexistent") {
		t.Errorf("Expected specific error message, got: %s", err.Error())
	}
}

func TestMultiReconciler_ValidateAll_ValidConfiguration(t *testing.T) {
	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	config := &MultiRepositoryConfig{
		Repositories: []RepositoryConfig{
			{
				Name:        "repo1",
				Description: "Valid repository",
			},
			{
				Name:        "repo2",
				Description: "Another valid repository",
			},
		},
	}

	result, err := reconciler.ValidateAll(config, nil)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.TotalRepositories != 2 {
		t.Errorf("Expected 2 total repositories, got %d", result.Summary.TotalRepositories)
	}

	if result.Summary.ValidCount != 2 {
		t.Errorf("Expected 2 valid repositories, got %d", result.Summary.ValidCount)
	}

	if result.Summary.InvalidCount != 0 {
		t.Errorf("Expected 0 invalid repositories, got %d", result.Summary.InvalidCount)
	}

	if len(result.Valid) != 2 {
		t.Errorf("Expected 2 valid repositories, got %d", len(result.Valid))
	}

	if len(result.Invalid) != 0 {
		t.Errorf("Expected 0 invalid repositories, got %d", len(result.Invalid))
	}
}

func TestMultiReconciler_ValidateAll_FilteredRepositories(t *testing.T) {
	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner")

	config := &MultiRepositoryConfig{
		Repositories: []RepositoryConfig{
			{Name: "repo1", Description: "Valid repository"},
			{Name: "repo2", Description: "Another valid repository"},
			{Name: "repo3", Description: "Third valid repository"},
		},
	}

	result, err := reconciler.ValidateAll(config, []string{"repo1", "repo3"})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Summary.TotalRepositories != 2 {
		t.Errorf("Expected 2 total repositories, got %d", result.Summary.TotalRepositories)
	}

	if result.Summary.ValidCount != 2 {
		t.Errorf("Expected 2 valid repositories, got %d", result.Summary.ValidCount)
	}

	if len(result.Valid) != 2 {
		t.Errorf("Expected 2 valid repositories, got %d", len(result.Valid))
	}

	// Check that only filtered repositories are included
	validSet := make(map[string]bool)
	for _, repo := range result.Valid {
		validSet[repo] = true
	}

	if !validSet["repo1"] {
		t.Error("Expected repo1 to be validated")
	}

	if !validSet["repo3"] {
		t.Error("Expected repo3 to be validated")
	}

	if validSet["repo2"] {
		t.Error("Did not expect repo2 to be validated")
	}
}

func TestMultiReconciler_countPlanChanges(t *testing.T) {
	client := newMockAPIClient()
	reconciler := NewMultiReconciler(client, "test-owner").(*multiReconciler)

	tests := []struct {
		name     string
		plan     *ReconciliationPlan
		expected int
	}{
		{
			name:     "nil plan",
			plan:     nil,
			expected: 0,
		},
		{
			name:     "empty plan",
			plan:     &ReconciliationPlan{},
			expected: 0,
		},
		{
			name: "plan with repository change only",
			plan: &ReconciliationPlan{
				Repository: &RepositoryChange{Type: ChangeTypeCreate},
			},
			expected: 1,
		},
		{
			name: "plan with multiple changes",
			plan: &ReconciliationPlan{
				Repository: &RepositoryChange{Type: ChangeTypeCreate},
				BranchRules: []BranchRuleChange{
					{Type: ChangeTypeCreate},
					{Type: ChangeTypeUpdate},
				},
				Collaborators: []CollaboratorChange{
					{Type: ChangeTypeCreate},
				},
				Teams: []TeamChange{
					{Type: ChangeTypeCreate},
					{Type: ChangeTypeUpdate},
					{Type: ChangeTypeDelete},
				},
				Webhooks: []WebhookChange{
					{Type: ChangeTypeCreate},
				},
			},
			expected: 8, // 1 repo + 2 branch + 1 collab + 3 teams + 1 webhook
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reconciler.countPlanChanges(tt.plan)
			if result != tt.expected {
				t.Errorf("Expected %d changes, got %d", tt.expected, result)
			}
		})
	}
}
