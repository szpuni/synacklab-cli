package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-github/v66/github"
)

// mockGitHubServer creates a test HTTP server that mocks GitHub API responses
func mockGitHubServer(_ *testing.T, responses map[string]interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set common headers
		w.Header().Set("Content-Type", "application/json")

		// Route based on method and path
		key := fmt.Sprintf("%s %s", r.Method, r.URL.Path)

		if response, exists := responses[key]; exists {
			if err, ok := response.(error); ok {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"message": err.Error()})
				return
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		} else {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
		}
	}))
}

// createTestClient creates a GitHub client configured to use the test server
func createTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	client := NewClient("test-token")

	// Parse the test server URL and ensure it has a trailing slash
	serverURL, err := url.Parse(server.URL + "/")
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Override the base URL to point to our test server
	client.client.BaseURL = serverURL

	return client
}

func TestNewClient(t *testing.T) {
	client := NewClient("test-token")

	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}

	if client.client == nil {
		t.Fatal("Expected GitHub client to be initialized")
	}

	if client.ctx == nil {
		t.Fatal("Expected context to be initialized")
	}
}

func TestGetRepository(t *testing.T) {
	tests := []struct {
		name          string
		owner         string
		repoName      string
		mockResponse  interface{}
		expectedRepo  *Repository
		expectedError bool
	}{
		{
			name:     "successful get repository",
			owner:    "testowner",
			repoName: "testrepo",
			mockResponse: &github.Repository{
				ID:          github.Int64(123),
				Name:        github.String("testrepo"),
				FullName:    github.String("testowner/testrepo"),
				Description: github.String("Test repository"),
				Private:     github.Bool(false),
				Topics:      []string{"go", "testing"},
				HasIssues:   github.Bool(true),
				HasWiki:     github.Bool(false),
				HasProjects: github.Bool(true),
				CreatedAt:   &github.Timestamp{Time: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)},
				UpdatedAt:   &github.Timestamp{Time: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)},
			},
			expectedRepo: &Repository{
				ID:          123,
				Name:        "testrepo",
				FullName:    "testowner/testrepo",
				Description: "Test repository",
				Private:     false,
				Topics:      []string{"go", "testing"},
				Features: RepositoryFeatures{
					Issues:   true,
					Wiki:     false,
					Projects: true,
				},
				CreatedAt: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
			},
			expectedError: false,
		},
		{
			name:          "repository not found",
			owner:         "testowner",
			repoName:      "nonexistent",
			mockResponse:  fmt.Errorf("repository not found"),
			expectedRepo:  nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := map[string]interface{}{
				fmt.Sprintf("GET /repos/%s/%s", tt.owner, tt.repoName): tt.mockResponse,
			}

			server := mockGitHubServer(t, responses)
			defer server.Close()

			client := createTestClient(t, server)

			repo, err := client.GetRepository(tt.owner, tt.repoName)

			if tt.expectedError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if repo.ID != tt.expectedRepo.ID {
				t.Errorf("Expected ID %d, got %d", tt.expectedRepo.ID, repo.ID)
			}

			if repo.Name != tt.expectedRepo.Name {
				t.Errorf("Expected name %s, got %s", tt.expectedRepo.Name, repo.Name)
			}

			if repo.Description != tt.expectedRepo.Description {
				t.Errorf("Expected description %s, got %s", tt.expectedRepo.Description, repo.Description)
			}
		})
	}
}

func TestCreateRepository(t *testing.T) {
	config := RepositoryConfig{
		Name:        "newrepo",
		Description: "A new repository",
		Private:     true,
		Topics:      []string{"go", "api"},
		Features: RepositoryFeatures{
			Issues:   true,
			Wiki:     false,
			Projects: true,
		},
	}

	mockResponse := &github.Repository{
		ID:          github.Int64(456),
		Name:        github.String("newrepo"),
		FullName:    github.String("testowner/newrepo"),
		Description: github.String("A new repository"),
		Private:     github.Bool(true),
		Topics:      []string{"go", "api"},
		HasIssues:   github.Bool(true),
		HasWiki:     github.Bool(false),
		HasProjects: github.Bool(true),
		CreatedAt:   &github.Timestamp{Time: time.Now()},
		UpdatedAt:   &github.Timestamp{Time: time.Now()},
	}

	responses := map[string]interface{}{
		"POST /user/repos": mockResponse,
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	repo, err := client.CreateRepository(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if repo.Name != config.Name {
		t.Errorf("Expected name %s, got %s", config.Name, repo.Name)
	}

	if repo.Private != config.Private {
		t.Errorf("Expected private %t, got %t", config.Private, repo.Private)
	}
}

func TestUpdateRepository(t *testing.T) {
	owner := "testowner"
	name := "testrepo"
	config := RepositoryConfig{
		Name:        "testrepo",
		Description: "Updated description",
		Private:     false,
		Features: RepositoryFeatures{
			Issues: false,
			Wiki:   true,
		},
	}

	responses := map[string]interface{}{
		fmt.Sprintf("PATCH /repos/%s/%s", owner, name): &github.Repository{
			Name:        github.String(config.Name),
			Description: github.String(config.Description),
			Private:     github.Bool(config.Private),
		},
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	err := client.UpdateRepository(owner, name, config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestListCollaborators(t *testing.T) {
	owner := "testowner"
	name := "testrepo"

	mockResponse := []*github.User{
		{
			Login:    github.String("user1"),
			RoleName: github.String("admin"),
		},
		{
			Login:    github.String("user2"),
			RoleName: github.String("write"),
		},
	}

	responses := map[string]interface{}{
		fmt.Sprintf("GET /repos/%s/%s/collaborators", owner, name): mockResponse,
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	collaborators, err := client.ListCollaborators(owner, name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(collaborators) != 2 {
		t.Errorf("Expected 2 collaborators, got %d", len(collaborators))
	}

	if collaborators[0].Username != "user1" {
		t.Errorf("Expected username user1, got %s", collaborators[0].Username)
	}

	if collaborators[0].Permission != "admin" {
		t.Errorf("Expected permission admin, got %s", collaborators[0].Permission)
	}
}

func TestAddCollaborator(t *testing.T) {
	owner := "testowner"
	name := "testrepo"
	username := "newuser"
	permission := "write"

	responses := map[string]interface{}{
		fmt.Sprintf("PUT /repos/%s/%s/collaborators/%s", owner, name, username): map[string]string{
			"message": "success",
		},
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	err := client.AddCollaborator(owner, name, username, permission)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestRemoveCollaborator(t *testing.T) {
	owner := "testowner"
	name := "testrepo"
	username := "olduser"

	responses := map[string]interface{}{
		fmt.Sprintf("DELETE /repos/%s/%s/collaborators/%s", owner, name, username): map[string]string{
			"message": "success",
		},
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	err := client.RemoveCollaborator(owner, name, username)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestListTeamAccess(t *testing.T) {
	owner := "testowner"
	name := "testrepo"

	mockResponse := []*github.Team{
		{
			Slug:       github.String("backend-team"),
			Permission: github.String("admin"),
		},
		{
			Slug:       github.String("frontend-team"),
			Permission: github.String("write"),
		},
		{
			Slug:       github.String("qa-team"),
			Permission: github.String("read"),
		},
	}

	responses := map[string]interface{}{
		fmt.Sprintf("GET /repos/%s/%s/teams", owner, name): mockResponse,
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	teams, err := client.ListTeamAccess(owner, name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(teams) != 3 {
		t.Errorf("Expected 3 teams, got %d", len(teams))
	}

	if teams[0].TeamSlug != "backend-team" {
		t.Errorf("Expected team slug backend-team, got %s", teams[0].TeamSlug)
	}

	if teams[0].Permission != "admin" {
		t.Errorf("Expected permission admin, got %s", teams[0].Permission)
	}

	if teams[1].TeamSlug != "frontend-team" {
		t.Errorf("Expected team slug frontend-team, got %s", teams[1].TeamSlug)
	}

	if teams[1].Permission != "write" {
		t.Errorf("Expected permission write, got %s", teams[1].Permission)
	}

	if teams[2].TeamSlug != "qa-team" {
		t.Errorf("Expected team slug qa-team, got %s", teams[2].TeamSlug)
	}

	if teams[2].Permission != "read" {
		t.Errorf("Expected permission read, got %s", teams[2].Permission)
	}
}

func TestAddTeamAccess(t *testing.T) {
	owner := "testowner"
	name := "testrepo"
	team := TeamAccess{
		TeamSlug:   "new-team",
		Permission: "write",
	}

	responses := map[string]interface{}{
		fmt.Sprintf("PUT /orgs/%s/teams/%s/repos/%s/%s", owner, team.TeamSlug, owner, name): map[string]string{
			"message": "success",
		},
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	err := client.AddTeamAccess(owner, name, team)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestUpdateTeamAccess(t *testing.T) {
	owner := "testowner"
	name := "testrepo"
	team := TeamAccess{
		TeamSlug:   "existing-team",
		Permission: "admin",
	}

	responses := map[string]interface{}{
		fmt.Sprintf("PUT /orgs/%s/teams/%s/repos/%s/%s", owner, team.TeamSlug, owner, name): map[string]string{
			"message": "success",
		},
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	err := client.UpdateTeamAccess(owner, name, team)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestRemoveTeamAccess(t *testing.T) {
	owner := "testowner"
	name := "testrepo"
	teamSlug := "old-team"

	responses := map[string]interface{}{
		fmt.Sprintf("DELETE /orgs/%s/teams/%s/repos/%s/%s", owner, teamSlug, owner, name): map[string]string{
			"message": "success",
		},
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	err := client.RemoveTeamAccess(owner, name, teamSlug)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestListWebhooks(t *testing.T) {
	owner := "testowner"
	name := "testrepo"

	mockResponse := []*github.Hook{
		{
			ID:     github.Int64(1),
			Events: []string{"push", "pull_request"},
			Active: github.Bool(true),
			Config: &github.HookConfig{
				URL: github.String("https://example.com/webhook1"),
			},
		},
		{
			ID:     github.Int64(2),
			Events: []string{"issues"},
			Active: github.Bool(false),
			Config: &github.HookConfig{
				URL: github.String("https://example.com/webhook2"),
			},
		},
	}

	responses := map[string]interface{}{
		fmt.Sprintf("GET /repos/%s/%s/hooks", owner, name): mockResponse,
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	webhooks, err := client.ListWebhooks(owner, name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(webhooks) != 2 {
		t.Errorf("Expected 2 webhooks, got %d", len(webhooks))
	}

	if webhooks[0].ID != 1 {
		t.Errorf("Expected webhook ID 1, got %d", webhooks[0].ID)
	}

	if webhooks[0].URL != "https://example.com/webhook1" {
		t.Errorf("Expected URL https://example.com/webhook1, got %s", webhooks[0].URL)
	}

	if !webhooks[0].Active {
		t.Error("Expected webhook to be active")
	}
}

func TestCreateWebhook(t *testing.T) {
	owner := "testowner"
	name := "testrepo"
	webhook := Webhook{
		URL:    "https://example.com/webhook",
		Events: []string{"push", "pull_request"},
		Secret: "secret123",
		Active: true,
	}

	responses := map[string]interface{}{
		fmt.Sprintf("POST /repos/%s/%s/hooks", owner, name): &github.Hook{
			ID:     github.Int64(123),
			Events: webhook.Events,
			Active: github.Bool(webhook.Active),
		},
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	err := client.CreateWebhook(owner, name, webhook)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestUpdateWebhook(t *testing.T) {
	owner := "testowner"
	name := "testrepo"
	webhookID := int64(123)
	webhook := Webhook{
		URL:    "https://example.com/updated-webhook",
		Events: []string{"push"},
		Active: false,
	}

	responses := map[string]interface{}{
		fmt.Sprintf("PATCH /repos/%s/%s/hooks/%d", owner, name, webhookID): &github.Hook{
			ID:     github.Int64(webhookID),
			Events: webhook.Events,
			Active: github.Bool(webhook.Active),
		},
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	err := client.UpdateWebhook(owner, name, webhookID, webhook)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestDeleteWebhook(t *testing.T) {
	owner := "testowner"
	name := "testrepo"
	webhookID := int64(123)

	responses := map[string]interface{}{
		fmt.Sprintf("DELETE /repos/%s/%s/hooks/%d", owner, name, webhookID): map[string]string{
			"message": "success",
		},
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	err := client.DeleteWebhook(owner, name, webhookID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestGetBranchProtection(t *testing.T) {
	owner := "testowner"
	name := "testrepo"
	branch := "main"

	contexts := []string{"ci/test", "ci/build"}
	mockResponse := &github.Protection{
		RequiredStatusChecks: &github.RequiredStatusChecks{
			Strict:   true,
			Contexts: &contexts,
		},
		RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{
			RequiredApprovingReviewCount: 2,
			DismissStaleReviews:          true,
			RequireCodeOwnerReviews:      false,
		},
		Restrictions: &github.BranchRestrictions{
			Users: []*github.User{
				{Login: github.String("admin1")},
				{Login: github.String("admin2")},
			},
		},
	}

	responses := map[string]interface{}{
		fmt.Sprintf("GET /repos/%s/%s/branches/%s/protection", owner, name, branch): mockResponse,
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	protection, err := client.GetBranchProtection(owner, name, branch)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if protection.Pattern != branch {
		t.Errorf("Expected pattern %s, got %s", branch, protection.Pattern)
	}

	if !protection.RequireUpToDate {
		t.Error("Expected RequireUpToDate to be true")
	}

	if len(protection.RequiredStatusChecks) != 2 {
		t.Errorf("Expected 2 status checks, got %d", len(protection.RequiredStatusChecks))
	}

	if protection.RequiredReviews != 2 {
		t.Errorf("Expected 2 required reviews, got %d", protection.RequiredReviews)
	}
}

func TestCreateBranchProtection(t *testing.T) {
	owner := "testowner"
	name := "testrepo"
	branch := "main"
	rules := BranchProtectionRule{
		Pattern:                "main",
		RequiredStatusChecks:   []string{"ci/test", "ci/build"},
		RequireUpToDate:        true,
		RequiredReviews:        2,
		DismissStaleReviews:    true,
		RequireCodeOwnerReview: false,
		RestrictPushes:         []string{"admin1", "admin2"},
	}

	responses := map[string]interface{}{
		fmt.Sprintf("PUT /repos/%s/%s/branches/%s/protection", owner, name, branch): &github.Protection{
			RequiredStatusChecks: &github.RequiredStatusChecks{
				Strict:   rules.RequireUpToDate,
				Contexts: &rules.RequiredStatusChecks,
			},
			RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{
				RequiredApprovingReviewCount: rules.RequiredReviews,
				DismissStaleReviews:          rules.DismissStaleReviews,
				RequireCodeOwnerReviews:      rules.RequireCodeOwnerReview,
			},
		},
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	err := client.CreateBranchProtection(owner, name, branch, rules)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestUpdateBranchProtection(t *testing.T) {
	owner := "testowner"
	name := "testrepo"
	branch := "main"
	rules := BranchProtectionRule{
		Pattern:                "main",
		RequiredStatusChecks:   []string{"ci/test"},
		RequireUpToDate:        true,
		RequiredReviews:        1,
		DismissStaleReviews:    false,
		RequireCodeOwnerReview: true,
		RestrictPushes:         []string{"admin"},
	}

	responses := map[string]interface{}{
		fmt.Sprintf("PUT /repos/%s/%s/branches/%s/protection", owner, name, branch): &github.Protection{
			RequiredStatusChecks: &github.RequiredStatusChecks{
				Strict:   rules.RequireUpToDate,
				Contexts: &rules.RequiredStatusChecks,
			},
		},
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	err := client.UpdateBranchProtection(owner, name, branch, rules)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestDeleteBranchProtection(t *testing.T) {
	owner := "testowner"
	name := "testrepo"
	branch := "main"

	responses := map[string]interface{}{
		fmt.Sprintf("DELETE /repos/%s/%s/branches/%s/protection", owner, name, branch): map[string]string{
			"message": "Branch protection rule deleted",
		},
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	err := client.DeleteBranchProtection(owner, name, branch)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestBuildProtectionRequest(t *testing.T) {
	client := NewClient("test-token")

	tests := []struct {
		name     string
		rules    BranchProtectionRule
		expected *github.ProtectionRequest
	}{
		{
			name: "minimal protection rules",
			rules: BranchProtectionRule{
				Pattern: "main",
			},
			expected: &github.ProtectionRequest{
				EnforceAdmins: true,
			},
		},
		{
			name: "with status checks",
			rules: BranchProtectionRule{
				Pattern:              "main",
				RequiredStatusChecks: []string{"ci/test", "ci/build"},
				RequireUpToDate:      true,
			},
			expected: &github.ProtectionRequest{
				EnforceAdmins: true,
				RequiredStatusChecks: &github.RequiredStatusChecks{
					Strict:   true,
					Contexts: &[]string{"ci/test", "ci/build"},
				},
			},
		},
		{
			name: "with required reviews",
			rules: BranchProtectionRule{
				Pattern:                "main",
				RequiredReviews:        2,
				DismissStaleReviews:    true,
				RequireCodeOwnerReview: false,
			},
			expected: &github.ProtectionRequest{
				EnforceAdmins: true,
				RequiredPullRequestReviews: &github.PullRequestReviewsEnforcementRequest{
					RequiredApprovingReviewCount: 2,
					DismissStaleReviews:          true,
					RequireCodeOwnerReviews:      false,
				},
			},
		},
		{
			name: "with push restrictions",
			rules: BranchProtectionRule{
				Pattern:        "main",
				RestrictPushes: []string{"admin1", "admin2"},
			},
			expected: &github.ProtectionRequest{
				EnforceAdmins: true,
				Restrictions: &github.BranchRestrictionsRequest{
					Users: []string{"admin1", "admin2"},
				},
			},
		},
		{
			name: "complete protection rules",
			rules: BranchProtectionRule{
				Pattern:                "main",
				RequiredStatusChecks:   []string{"ci/test"},
				RequireUpToDate:        true,
				RequiredReviews:        1,
				DismissStaleReviews:    false,
				RequireCodeOwnerReview: true,
				RestrictPushes:         []string{"admin"},
			},
			expected: &github.ProtectionRequest{
				EnforceAdmins: true,
				RequiredStatusChecks: &github.RequiredStatusChecks{
					Strict:   true,
					Contexts: &[]string{"ci/test"},
				},
				RequiredPullRequestReviews: &github.PullRequestReviewsEnforcementRequest{
					RequiredApprovingReviewCount: 1,
					DismissStaleReviews:          false,
					RequireCodeOwnerReviews:      true,
				},
				Restrictions: &github.BranchRestrictionsRequest{
					Users: []string{"admin"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.buildProtectionRequest(tt.rules)

			if result.EnforceAdmins != tt.expected.EnforceAdmins {
				t.Errorf("Expected EnforceAdmins %t, got %t", tt.expected.EnforceAdmins, result.EnforceAdmins)
			}

			// Check status checks
			if tt.expected.RequiredStatusChecks == nil {
				if result.RequiredStatusChecks != nil {
					t.Error("Expected RequiredStatusChecks to be nil")
				}
			} else {
				if result.RequiredStatusChecks == nil {
					t.Error("Expected RequiredStatusChecks to be set")
				} else {
					if result.RequiredStatusChecks.Strict != tt.expected.RequiredStatusChecks.Strict {
						t.Errorf("Expected Strict %t, got %t", tt.expected.RequiredStatusChecks.Strict, result.RequiredStatusChecks.Strict)
					}
					if len(*result.RequiredStatusChecks.Contexts) != len(*tt.expected.RequiredStatusChecks.Contexts) {
						t.Errorf("Expected %d contexts, got %d", len(*tt.expected.RequiredStatusChecks.Contexts), len(*result.RequiredStatusChecks.Contexts))
					}
				}
			}

			// Check pull request reviews
			if tt.expected.RequiredPullRequestReviews == nil {
				if result.RequiredPullRequestReviews != nil {
					t.Error("Expected RequiredPullRequestReviews to be nil")
				}
			} else {
				if result.RequiredPullRequestReviews == nil {
					t.Error("Expected RequiredPullRequestReviews to be set")
				} else {
					if result.RequiredPullRequestReviews.RequiredApprovingReviewCount != tt.expected.RequiredPullRequestReviews.RequiredApprovingReviewCount {
						t.Errorf("Expected RequiredApprovingReviewCount %d, got %d", tt.expected.RequiredPullRequestReviews.RequiredApprovingReviewCount, result.RequiredPullRequestReviews.RequiredApprovingReviewCount)
					}
				}
			}

			// Check restrictions
			if tt.expected.Restrictions == nil {
				if result.Restrictions != nil {
					t.Error("Expected Restrictions to be nil")
				}
			} else {
				if result.Restrictions == nil {
					t.Error("Expected Restrictions to be set")
				} else {
					if len(result.Restrictions.Users) != len(tt.expected.Restrictions.Users) {
						t.Errorf("Expected %d restricted users, got %d", len(tt.expected.Restrictions.Users), len(result.Restrictions.Users))
					}
				}
			}
		})
	}
}
func TestCollaboratorErrors(t *testing.T) {
	tests := []struct {
		name      string
		operation func(*Client) error
		responses map[string]interface{}
	}{
		{
			name: "list collaborators - repository not found",
			operation: func(c *Client) error {
				_, err := c.ListCollaborators("owner", "nonexistent")
				return err
			},
			responses: map[string]interface{}{
				"GET /repos/owner/nonexistent/collaborators": fmt.Errorf("repository not found"),
			},
		},
		{
			name: "add collaborator - user not found",
			operation: func(c *Client) error {
				return c.AddCollaborator("owner", "repo", "nonexistentuser", "write")
			},
			responses: map[string]interface{}{
				"PUT /repos/owner/repo/collaborators/nonexistentuser": fmt.Errorf("user not found"),
			},
		},
		{
			name: "add collaborator - insufficient permissions",
			operation: func(c *Client) error {
				return c.AddCollaborator("owner", "repo", "user", "admin")
			},
			responses: map[string]interface{}{
				"PUT /repos/owner/repo/collaborators/user": fmt.Errorf("insufficient permissions"),
			},
		},
		{
			name: "remove collaborator - user not a collaborator",
			operation: func(c *Client) error {
				return c.RemoveCollaborator("owner", "repo", "notacollaborator")
			},
			responses: map[string]interface{}{
				"DELETE /repos/owner/repo/collaborators/notacollaborator": fmt.Errorf("user is not a collaborator"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mockGitHubServer(t, tt.responses)
			defer server.Close()

			client := createTestClient(t, server)

			err := tt.operation(client)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
		})
	}
}

func TestTeamAccessErrors(t *testing.T) {
	tests := []struct {
		name      string
		operation func(*Client) error
		responses map[string]interface{}
	}{
		{
			name: "list team access - repository not found",
			operation: func(c *Client) error {
				_, err := c.ListTeamAccess("owner", "nonexistent")
				return err
			},
			responses: map[string]interface{}{
				"GET /repos/owner/nonexistent/teams": fmt.Errorf("repository not found"),
			},
		},
		{
			name: "add team access - team not found",
			operation: func(c *Client) error {
				return c.AddTeamAccess("owner", "repo", TeamAccess{
					TeamSlug:   "nonexistent-team",
					Permission: "write",
				})
			},
			responses: map[string]interface{}{
				"PUT /orgs/owner/teams/nonexistent-team/repos/owner/repo": fmt.Errorf("team not found"),
			},
		},
		{
			name: "add team access - insufficient permissions",
			operation: func(c *Client) error {
				return c.AddTeamAccess("owner", "repo", TeamAccess{
					TeamSlug:   "team",
					Permission: "admin",
				})
			},
			responses: map[string]interface{}{
				"PUT /orgs/owner/teams/team/repos/owner/repo": fmt.Errorf("insufficient permissions"),
			},
		},
		{
			name: "update team access - team not found",
			operation: func(c *Client) error {
				return c.UpdateTeamAccess("owner", "repo", TeamAccess{
					TeamSlug:   "nonexistent-team",
					Permission: "read",
				})
			},
			responses: map[string]interface{}{
				"PUT /orgs/owner/teams/nonexistent-team/repos/owner/repo": fmt.Errorf("team not found"),
			},
		},
		{
			name: "remove team access - team not found",
			operation: func(c *Client) error {
				return c.RemoveTeamAccess("owner", "repo", "nonexistent-team")
			},
			responses: map[string]interface{}{
				"DELETE /orgs/owner/teams/nonexistent-team/repos/owner/repo": fmt.Errorf("team not found"),
			},
		},
		{
			name: "remove team access - team not associated with repo",
			operation: func(c *Client) error {
				return c.RemoveTeamAccess("owner", "repo", "unassociated-team")
			},
			responses: map[string]interface{}{
				"DELETE /orgs/owner/teams/unassociated-team/repos/owner/repo": fmt.Errorf("team is not associated with repository"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mockGitHubServer(t, tt.responses)
			defer server.Close()

			client := createTestClient(t, server)

			err := tt.operation(client)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
		})
	}
}

func TestBranchProtectionErrors(t *testing.T) {
	tests := []struct {
		name      string
		operation func(*Client) error
		responses map[string]interface{}
	}{
		{
			name: "get branch protection - not found",
			operation: func(c *Client) error {
				_, err := c.GetBranchProtection("owner", "repo", "main")
				return err
			},
			responses: map[string]interface{}{
				"GET /repos/owner/repo/branches/main/protection": fmt.Errorf("branch protection not found"),
			},
		},
		{
			name: "create branch protection - insufficient permissions",
			operation: func(c *Client) error {
				return c.CreateBranchProtection("owner", "repo", "main", BranchProtectionRule{
					Pattern: "main",
				})
			},
			responses: map[string]interface{}{
				"PUT /repos/owner/repo/branches/main/protection": fmt.Errorf("insufficient permissions"),
			},
		},
		{
			name: "update branch protection - repository not found",
			operation: func(c *Client) error {
				return c.UpdateBranchProtection("owner", "nonexistent", "main", BranchProtectionRule{
					Pattern: "main",
				})
			},
			responses: map[string]interface{}{
				"PUT /repos/owner/nonexistent/branches/main/protection": fmt.Errorf("repository not found"),
			},
		},
		{
			name: "delete branch protection - branch not found",
			operation: func(c *Client) error {
				return c.DeleteBranchProtection("owner", "repo", "nonexistent")
			},
			responses: map[string]interface{}{
				"DELETE /repos/owner/repo/branches/nonexistent/protection": fmt.Errorf("branch not found"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := mockGitHubServer(t, tt.responses)
			defer server.Close()

			client := createTestClient(t, server)

			err := tt.operation(client)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
		})
	}
}

func TestListCollaboratorsEmpty(t *testing.T) {
	owner := "testowner"
	name := "testrepo"

	mockResponse := []*github.User{}

	responses := map[string]interface{}{
		fmt.Sprintf("GET /repos/%s/%s/collaborators", owner, name): mockResponse,
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	collaborators, err := client.ListCollaborators(owner, name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(collaborators) != 0 {
		t.Errorf("Expected 0 collaborators for empty repository, got %d", len(collaborators))
	}
}

func TestListTeamAccessEmpty(t *testing.T) {
	owner := "testowner"
	name := "testrepo"

	mockResponse := []*github.Team{}

	responses := map[string]interface{}{
		fmt.Sprintf("GET /repos/%s/%s/teams", owner, name): mockResponse,
	}

	server := mockGitHubServer(t, responses)
	defer server.Close()

	client := createTestClient(t, server)

	teams, err := client.ListTeamAccess(owner, name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(teams) != 0 {
		t.Errorf("Expected 0 teams for repository with no team access, got %d", len(teams))
	}
}

func TestCollaboratorPermissionMapping(t *testing.T) {
	owner := "testowner"
	name := "testrepo"

	tests := []struct {
		name               string
		githubPermission   string
		expectedPermission string
	}{
		{
			name:               "admin permission",
			githubPermission:   "ADMIN",
			expectedPermission: "admin",
		},
		{
			name:               "write permission",
			githubPermission:   "WRITE",
			expectedPermission: "write",
		},
		{
			name:               "read permission",
			githubPermission:   "READ",
			expectedPermission: "read",
		},
		{
			name:               "maintain permission",
			githubPermission:   "MAINTAIN",
			expectedPermission: "maintain",
		},
		{
			name:               "triage permission",
			githubPermission:   "TRIAGE",
			expectedPermission: "triage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResponse := []*github.User{
				{
					Login:    github.String("testuser"),
					RoleName: github.String(tt.githubPermission),
				},
			}

			responses := map[string]interface{}{
				fmt.Sprintf("GET /repos/%s/%s/collaborators", owner, name): mockResponse,
			}

			server := mockGitHubServer(t, responses)
			defer server.Close()

			client := createTestClient(t, server)

			collaborators, err := client.ListCollaborators(owner, name)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(collaborators) != 1 {
				t.Fatalf("Expected 1 collaborator, got %d", len(collaborators))
			}

			if collaborators[0].Permission != tt.expectedPermission {
				t.Errorf("Expected permission %s, got %s", tt.expectedPermission, collaborators[0].Permission)
			}
		})
	}
}

func TestTeamPermissionMapping(t *testing.T) {
	owner := "testowner"
	name := "testrepo"

	tests := []struct {
		name               string
		githubPermission   string
		expectedPermission string
	}{
		{
			name:               "admin permission",
			githubPermission:   "ADMIN",
			expectedPermission: "admin",
		},
		{
			name:               "push permission",
			githubPermission:   "PUSH",
			expectedPermission: "push",
		},
		{
			name:               "pull permission",
			githubPermission:   "PULL",
			expectedPermission: "pull",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResponse := []*github.Team{
				{
					Slug:       github.String("testteam"),
					Permission: github.String(tt.githubPermission),
				},
			}

			responses := map[string]interface{}{
				fmt.Sprintf("GET /repos/%s/%s/teams", owner, name): mockResponse,
			}

			server := mockGitHubServer(t, responses)
			defer server.Close()

			client := createTestClient(t, server)

			teams, err := client.ListTeamAccess(owner, name)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(teams) != 1 {
				t.Fatalf("Expected 1 team, got %d", len(teams))
			}

			if teams[0].Permission != tt.expectedPermission {
				t.Errorf("Expected permission %s, got %s", tt.expectedPermission, teams[0].Permission)
			}
		})
	}
}

func TestAddCollaboratorWithDifferentPermissions(t *testing.T) {
	owner := "testowner"
	name := "testrepo"
	username := "testuser"

	tests := []struct {
		name       string
		permission string
	}{
		{
			name:       "read permission",
			permission: "read",
		},
		{
			name:       "write permission",
			permission: "write",
		},
		{
			name:       "admin permission",
			permission: "admin",
		},
		{
			name:       "maintain permission",
			permission: "maintain",
		},
		{
			name:       "triage permission",
			permission: "triage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := map[string]interface{}{
				fmt.Sprintf("PUT /repos/%s/%s/collaborators/%s", owner, name, username): map[string]string{
					"message": "success",
				},
			}

			server := mockGitHubServer(t, responses)
			defer server.Close()

			client := createTestClient(t, server)

			err := client.AddCollaborator(owner, name, username, tt.permission)
			if err != nil {
				t.Fatalf("Unexpected error for permission %s: %v", tt.permission, err)
			}
		})
	}
}

func TestAddTeamAccessWithDifferentPermissions(t *testing.T) {
	owner := "testowner"
	name := "testrepo"
	teamSlug := "testteam"

	tests := []struct {
		name       string
		permission string
	}{
		{
			name:       "pull permission",
			permission: "pull",
		},
		{
			name:       "push permission",
			permission: "push",
		},
		{
			name:       "admin permission",
			permission: "admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			team := TeamAccess{
				TeamSlug:   teamSlug,
				Permission: tt.permission,
			}

			responses := map[string]interface{}{
				fmt.Sprintf("PUT /orgs/%s/teams/%s/repos/%s/%s", owner, teamSlug, owner, name): map[string]string{
					"message": "success",
				},
			}

			server := mockGitHubServer(t, responses)
			defer server.Close()

			client := createTestClient(t, server)

			err := client.AddTeamAccess(owner, name, team)
			if err != nil {
				t.Fatalf("Unexpected error for permission %s: %v", tt.permission, err)
			}
		})
	}
}
