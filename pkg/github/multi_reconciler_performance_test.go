package github

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

// BenchmarkMultiReconcilerApplyAll benchmarks the ApplyAll method with various repository counts
func BenchmarkMultiReconcilerApplyAll(b *testing.B) {
	testCases := []struct {
		name      string
		repoCount int
	}{
		{"10_repos", 10},
		{"50_repos", 50},
		{"100_repos", 100},
		{"500_repos", 500},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkApplyAll(b, tc.repoCount)
		})
	}
}

func benchmarkApplyAll(b *testing.B, repoCount int) {
	b.Helper()
	// Create mock client and reconciler
	mockClient := &PerformanceMockAPIClient{}
	reconciler := NewMultiReconciler(mockClient, "test-org")

	// Generate test plans
	plans := generateTestPlans(repoCount)

	// Reset timer after setup
	b.ResetTimer()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		_, err := reconciler.ApplyAll(plans)
		if err != nil {
			b.Fatalf("ApplyAll failed: %v", err)
		}
	}
}

// BenchmarkMultiReconcilerPlanAll benchmarks the PlanAll method
func BenchmarkMultiReconcilerPlanAll(b *testing.B) {
	testCases := []struct {
		name      string
		repoCount int
	}{
		{"10_repos", 10},
		{"50_repos", 50},
		{"100_repos", 100},
		{"500_repos", 500},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkPlanAll(b, tc.repoCount)
		})
	}
}

func benchmarkPlanAll(b *testing.B, repoCount int) {
	b.Helper()
	// Create mock client and reconciler
	mockClient := &PerformanceMockAPIClient{}
	reconciler := NewMultiReconciler(mockClient, "test-org")

	// Generate test configuration
	config := generateTestMultiRepoConfig(repoCount)

	// Reset timer after setup
	b.ResetTimer()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		_, err := reconciler.PlanAll(config, nil)
		if err != nil {
			b.Fatalf("PlanAll failed: %v", err)
		}
	}
}

// BenchmarkConfigMerging benchmarks configuration merging performance
func BenchmarkConfigMerging(b *testing.B) {
	testCases := []struct {
		name      string
		repoCount int
	}{
		{"10_repos", 10},
		{"50_repos", 50},
		{"100_repos", 100},
		{"500_repos", 500},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkConfigMerging(b, tc.repoCount)
		})
	}
}

func benchmarkConfigMerging(b *testing.B, repoCount int) {
	b.Helper()
	merger := NewConfigMerger()
	defaults := generateTestDefaults()
	repos := generateTestRepositories(repoCount)

	// Reset timer after setup
	b.ResetTimer()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		for j := range repos {
			_, err := merger.MergeDefaults(defaults, &repos[j])
			if err != nil {
				b.Fatalf("MergeDefaults failed: %v", err)
			}
		}
	}
}

// BenchmarkWorkerPoolScaling benchmarks worker pool performance with different worker counts
func BenchmarkWorkerPoolScaling(b *testing.B) {
	testCases := []struct {
		name        string
		repoCount   int
		workerCount int
	}{
		{"100_repos_1_worker", 100, 1},
		{"100_repos_5_workers", 100, 5},
		{"100_repos_10_workers", 100, 10},
		{"100_repos_20_workers", 100, 20},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkWorkerPoolScaling(b, tc.repoCount, tc.workerCount)
		})
	}
}

func benchmarkWorkerPoolScaling(b *testing.B, repoCount, workerCount int) {
	b.Helper()
	// Create mock client with custom rate limiter
	mockClient := &PerformanceMockAPIClient{}
	rateLimiter := NewMultiRepoRateLimiter(&RateLimiterConfig{
		BaseDelay:               1 * time.Millisecond, // Minimal delay for benchmarking
		MaxDelay:                10 * time.Millisecond,
		BackoffFactor:           1.5,
		Jitter:                  0.1,
		ConcurrencyLimit:        workerCount,
		MinRemainingRequests:    100,
		AggressiveThrottleDelay: 5 * time.Millisecond,
	})

	reconciler := NewMultiReconcilerWithRateLimiter(mockClient, "test-org", rateLimiter)
	plans := generateTestPlans(repoCount)

	// Reset timer after setup
	b.ResetTimer()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		_, err := reconciler.ApplyAll(plans)
		if err != nil {
			b.Fatalf("ApplyAll failed: %v", err)
		}
	}
}

// BenchmarkMemoryUsage benchmarks memory usage during large configuration processing
func BenchmarkMemoryUsage(b *testing.B) {
	testCases := []struct {
		name      string
		repoCount int
	}{
		{"1000_repos", 1000},
		{"5000_repos", 5000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkMemoryUsage(b, tc.repoCount)
		})
	}
}

func benchmarkMemoryUsage(b *testing.B, repoCount int) {
	b.Helper()
	// Force GC before starting
	runtime.GC()

	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Create large configuration
	config := generateTestMultiRepoConfig(repoCount)

	// Create reconciler
	mockClient := &PerformanceMockAPIClient{}
	reconciler := NewMultiReconciler(mockClient, "test-org")

	// Reset timer after setup
	b.ResetTimer()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		plans, err := reconciler.PlanAll(config, nil)
		if err != nil {
			b.Fatalf("PlanAll failed: %v", err)
		}

		_, err = reconciler.ApplyAll(plans)
		if err != nil {
			b.Fatalf("ApplyAll failed: %v", err)
		}
	}

	// Measure memory usage
	runtime.ReadMemStats(&m2)
	b.ReportMetric(float64(m2.Alloc-m1.Alloc)/float64(b.N), "bytes/op")
	b.ReportMetric(float64(m2.Mallocs-m1.Mallocs)/float64(b.N), "allocs/op")
}

// Performance test for concurrent operations
func TestConcurrentPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	testCases := []struct {
		name            string
		repoCount       int
		concurrency     int
		expectedMaxTime time.Duration
	}{
		{"100_repos_5_concurrent", 100, 5, 30 * time.Second},
		{"500_repos_10_concurrent", 500, 10, 60 * time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testConcurrentPerformance(t, tc.repoCount, tc.concurrency, tc.expectedMaxTime)
		})
	}
}

func testConcurrentPerformance(t *testing.T, repoCount, concurrency int, maxTime time.Duration) {
	t.Helper()
	// Create mock client with realistic delays
	mockClient := &PerformanceMockAPIClient{
		delay: 10 * time.Millisecond, // Simulate network latency
	}

	rateLimiter := NewMultiRepoRateLimiter(&RateLimiterConfig{
		BaseDelay:               50 * time.Millisecond,
		MaxDelay:                5 * time.Second,
		BackoffFactor:           2.0,
		Jitter:                  0.1,
		ConcurrencyLimit:        concurrency,
		MinRemainingRequests:    100,
		AggressiveThrottleDelay: 1 * time.Second,
	})

	reconciler := NewMultiReconcilerWithRateLimiter(mockClient, "test-org", rateLimiter)
	plans := generateTestPlans(repoCount)

	start := time.Now()
	_, err := reconciler.ApplyAll(plans)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	if duration > maxTime {
		t.Errorf("Performance test failed: took %v, expected max %v", duration, maxTime)
	}

	t.Logf("Processed %d repositories with %d concurrent workers in %v", repoCount, concurrency, duration)
}

// Test memory efficiency with large configurations
func TestMemoryEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory efficiency test in short mode")
	}

	// Force GC before starting
	runtime.GC()

	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Process large configuration
	config := generateTestMultiRepoConfig(1000)
	mockClient := &PerformanceMockAPIClient{}
	reconciler := NewMultiReconciler(mockClient, "test-org")

	plans, err := reconciler.PlanAll(config, nil)
	if err != nil {
		t.Fatalf("PlanAll failed: %v", err)
	}

	_, err = reconciler.ApplyAll(plans)
	if err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}

	runtime.ReadMemStats(&m2)

	memoryUsed := m2.Alloc - m1.Alloc
	allocations := m2.Mallocs - m1.Mallocs

	t.Logf("Memory used: %d bytes, Allocations: %d", memoryUsed, allocations)

	// Memory usage should be reasonable (less than 100MB for 1000 repos)
	maxMemory := uint64(100 * 1024 * 1024) // 100MB
	if memoryUsed > maxMemory {
		t.Errorf("Memory usage too high: %d bytes (max: %d bytes)", memoryUsed, maxMemory)
	}
}

// Helper functions for generating test data

func generateTestPlans(count int) map[string]*ReconciliationPlan {
	plans := make(map[string]*ReconciliationPlan, count)
	for i := 0; i < count; i++ {
		repoName := fmt.Sprintf("test-repo-%d", i)
		plans[repoName] = &ReconciliationPlan{
			Repository: &RepositoryChange{
				Type: ChangeTypeUpdate,
				Before: &Repository{
					Name:        repoName,
					Description: "Test repository",
					Private:     false,
				},
				After: &Repository{
					Name:        repoName,
					Description: "Updated test repository",
					Private:     true,
				},
			},
		}
	}
	return plans
}

func generateTestMultiRepoConfig(count int) *MultiRepositoryConfig {
	repos := generateTestRepositories(count)
	return &MultiRepositoryConfig{
		Version:      "1.0",
		Defaults:     generateTestDefaults(),
		Repositories: repos,
	}
}

func generateTestRepositories(count int) []RepositoryConfig {
	repos := make([]RepositoryConfig, count)
	for i := 0; i < count; i++ {
		repos[i] = RepositoryConfig{
			Name:        fmt.Sprintf("test-repo-%d", i),
			Description: fmt.Sprintf("Test repository %d", i),
			Private:     i%2 == 0,
			Topics:      []string{"test", "benchmark", fmt.Sprintf("repo-%d", i)},
			Features: RepositoryFeatures{
				Issues:      true,
				Wiki:        i%3 == 0,
				Projects:    i%4 == 0,
				Discussions: i%5 == 0,
			},
			BranchRules: []BranchProtectionRule{
				{
					Pattern:         "main",
					RequiredReviews: 2,
				},
			},
		}
	}
	return repos
}

func generateTestDefaults() *RepositoryDefaults {
	privateVal := true
	return &RepositoryDefaults{
		Description: "Default repository description",
		Private:     &privateVal,
		Topics:      []string{"default", "managed"},
		Features: &RepositoryFeatures{
			Issues:      true,
			Wiki:        false,
			Projects:    true,
			Discussions: false,
		},
		BranchRules: []BranchProtectionRule{
			{
				Pattern:         "main",
				RequiredReviews: 1,
			},
		},
	}
}

// PerformanceMockAPIClient with configurable delay for performance testing
type PerformanceMockAPIClient struct {
	delay time.Duration
}

func (m *PerformanceMockAPIClient) GetRepository(_, name string) (*Repository, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	if name == "non-existent-repo-for-auth-check" {
		return nil, fmt.Errorf("repository not found")
	}

	return &Repository{
		Name:        name,
		Description: "Mock repository",
		Private:     false,
	}, nil
}

func (m *PerformanceMockAPIClient) CreateRepository(config RepositoryConfig) (*Repository, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return &Repository{
		Name:        config.Name,
		Description: config.Description,
		Private:     config.Private,
	}, nil
}

func (m *PerformanceMockAPIClient) UpdateRepository(_, _ string, _ RepositoryConfig) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return nil
}

func (m *PerformanceMockAPIClient) GetBranchProtection(_, _, _ string) (*BranchProtection, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return nil, fmt.Errorf("branch protection not found")
}

func (m *PerformanceMockAPIClient) CreateBranchProtection(_, _, _ string, _ BranchProtectionRule) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return nil
}

func (m *PerformanceMockAPIClient) UpdateBranchProtection(_, _, _ string, _ BranchProtectionRule) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return nil
}

func (m *PerformanceMockAPIClient) DeleteBranchProtection(_, _, _ string) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return nil
}

func (m *PerformanceMockAPIClient) ListCollaborators(_, _ string) ([]Collaborator, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return []Collaborator{}, nil
}

func (m *PerformanceMockAPIClient) AddCollaborator(_, _, _, _ string) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return nil
}

func (m *PerformanceMockAPIClient) RemoveCollaborator(_, _, _ string) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return nil
}

func (m *PerformanceMockAPIClient) ListTeamAccess(_, _ string) ([]TeamAccess, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return []TeamAccess{}, nil
}

func (m *PerformanceMockAPIClient) AddTeamAccess(_, _ string, _ TeamAccess) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return nil
}

func (m *PerformanceMockAPIClient) UpdateTeamAccess(_, _ string, _ TeamAccess) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return nil
}

func (m *PerformanceMockAPIClient) RemoveTeamAccess(_, _, _ string) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return nil
}

func (m *PerformanceMockAPIClient) ListWebhooks(_, _ string) ([]Webhook, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return []Webhook{}, nil
}

func (m *PerformanceMockAPIClient) CreateWebhook(_, _ string, _ Webhook) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return nil
}

func (m *PerformanceMockAPIClient) UpdateWebhook(_, _ string, _ int64, _ Webhook) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return nil
}

func (m *PerformanceMockAPIClient) DeleteWebhook(_, _ string, _ int64) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return nil
}
