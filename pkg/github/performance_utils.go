package github

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// PerformanceOptimizer provides utilities for optimizing multi-repository operations
type PerformanceOptimizer struct {
	// Configuration
	batchSize           int
	memoryThreshold     uint64
	gcInterval          time.Duration
	enableMemoryMonitor bool

	// State
	mu             sync.RWMutex
	lastGC         time.Time
	processedCount int
	memoryStats    runtime.MemStats
}

// NewPerformanceOptimizer creates a new performance optimizer
func NewPerformanceOptimizer() *PerformanceOptimizer {
	return &PerformanceOptimizer{
		batchSize:           100,
		memoryThreshold:     500 * 1024 * 1024, // 500MB
		gcInterval:          30 * time.Second,
		enableMemoryMonitor: true,
		lastGC:              time.Now(),
	}
}

// OptimizeBatchSize calculates optimal batch size based on available memory and repository count
func (po *PerformanceOptimizer) OptimizeBatchSize(repoCount int, avgRepoSize int) int {
	po.mu.RLock()
	defer po.mu.RUnlock()

	// Get current memory stats
	runtime.ReadMemStats(&po.memoryStats)

	// Calculate available memory (rough estimate)
	availableMemory := po.memoryStats.Sys - po.memoryStats.Alloc

	// Estimate memory per repository (including overhead)
	estimatedMemoryPerRepo := uint64(avgRepoSize * 2) // 2x for processing overhead

	// Calculate optimal batch size based on available memory
	optimalBatchSize := int(availableMemory / (estimatedMemoryPerRepo * 4)) // Use 1/4 of available memory

	// Apply constraints
	if optimalBatchSize < 10 {
		optimalBatchSize = 10
	} else if optimalBatchSize > 500 {
		optimalBatchSize = 500
	}

	// For small repository counts, use smaller batches
	if repoCount < 50 {
		optimalBatchSize = minInt(optimalBatchSize, repoCount/2+1)
	}

	return optimalBatchSize
}

// ShouldTriggerGC determines if garbage collection should be triggered
func (po *PerformanceOptimizer) ShouldTriggerGC() bool {
	if !po.enableMemoryMonitor {
		return false
	}

	po.mu.RLock()
	defer po.mu.RUnlock()

	// Check if enough time has passed since last GC
	if time.Since(po.lastGC) < po.gcInterval {
		return false
	}

	// Check memory usage
	runtime.ReadMemStats(&po.memoryStats)
	return po.memoryStats.Alloc > po.memoryThreshold
}

// TriggerGC triggers garbage collection and updates tracking
func (po *PerformanceOptimizer) TriggerGC() {
	po.mu.Lock()
	defer po.mu.Unlock()

	runtime.GC()
	po.lastGC = time.Now()
}

// UpdateProcessedCount updates the count of processed repositories
func (po *PerformanceOptimizer) UpdateProcessedCount(count int) {
	po.mu.Lock()
	defer po.mu.Unlock()

	po.processedCount += count

	// Trigger GC periodically for large processing jobs
	if po.processedCount%1000 == 0 && po.ShouldTriggerGC() {
		go po.TriggerGC() // Async GC to avoid blocking
	}
}

// GetMemoryStats returns current memory statistics
func (po *PerformanceOptimizer) GetMemoryStats() runtime.MemStats {
	po.mu.RLock()
	defer po.mu.RUnlock()

	runtime.ReadMemStats(&po.memoryStats)
	return po.memoryStats
}

// OptimalWorkerCount calculates the optimal number of workers based on system resources
func (po *PerformanceOptimizer) OptimalWorkerCount(repoCount int, rateLimiterMaxSlots int) int {
	// Get number of CPU cores
	numCPU := runtime.NumCPU()

	// For I/O bound operations (GitHub API calls), we can use more workers than CPU cores
	// But we're limited by rate limiter slots
	optimalWorkers := minInt(numCPU*2, rateLimiterMaxSlots)

	// For small repository counts, use fewer workers to reduce overhead
	if repoCount < 20 {
		optimalWorkers = minInt(optimalWorkers, 3)
	} else if repoCount < 100 {
		optimalWorkers = minInt(optimalWorkers, numCPU)
	}

	// Ensure at least 1 worker
	if optimalWorkers < 1 {
		optimalWorkers = 1
	}

	return optimalWorkers
}

// ResultAggregator provides optimized result collection for multi-repository operations
type ResultAggregator struct {
	mu           sync.RWMutex
	succeeded    []string
	failed       map[string]error
	skipped      []string
	totalChanges int

	// Pre-allocated capacity to reduce memory allocations
	expectedResults int
}

// NewResultAggregator creates a new result aggregator with pre-allocated capacity
func NewResultAggregator(expectedResults int) *ResultAggregator {
	return &ResultAggregator{
		succeeded:       make([]string, 0, expectedResults),
		failed:          make(map[string]error, expectedResults),
		skipped:         make([]string, 0, expectedResults/10), // Assume 10% skip rate
		expectedResults: expectedResults,
	}
}

// AddSuccess adds a successful repository result
func (ra *ResultAggregator) AddSuccess(repoName string) {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	ra.succeeded = append(ra.succeeded, repoName)
}

// AddFailure adds a failed repository result
func (ra *ResultAggregator) AddFailure(repoName string, err error) {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	ra.failed[repoName] = err
}

// AddSkipped adds a skipped repository result
func (ra *ResultAggregator) AddSkipped(repoName string) {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	ra.skipped = append(ra.skipped, repoName)
}

// AddChanges adds to the total change count
func (ra *ResultAggregator) AddChanges(count int) {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	ra.totalChanges += count
}

// GetResult returns the aggregated result
func (ra *ResultAggregator) GetResult() *MultiRepoResult {
	ra.mu.RLock()
	defer ra.mu.RUnlock()

	return &MultiRepoResult{
		Succeeded: ra.succeeded,
		Failed:    ra.failed,
		Skipped:   ra.skipped,
		Summary: MultiRepoSummary{
			TotalRepositories: ra.expectedResults,
			SuccessCount:      len(ra.succeeded),
			FailureCount:      len(ra.failed),
			SkippedCount:      len(ra.skipped),
			TotalChanges:      ra.totalChanges,
		},
	}
}

// BatchProcessor processes repositories in optimized batches
type BatchProcessor struct {
	optimizer  *PerformanceOptimizer
	aggregator *ResultAggregator
	batchSize  int
	processor  func([]RepositoryConfig) error
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(expectedResults int, processor func([]RepositoryConfig) error) *BatchProcessor {
	optimizer := NewPerformanceOptimizer()
	return &BatchProcessor{
		optimizer:  optimizer,
		aggregator: NewResultAggregator(expectedResults),
		batchSize:  optimizer.OptimizeBatchSize(expectedResults, 1024), // Assume 1KB avg repo size
		processor:  processor,
	}
}

// ProcessRepositories processes repositories in optimized batches
func (bp *BatchProcessor) ProcessRepositories(ctx context.Context, repositories []RepositoryConfig) error {
	totalRepos := len(repositories)

	for i := 0; i < totalRepos; i += bp.batchSize {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Calculate batch end
		end := minInt(i+bp.batchSize, totalRepos)
		batch := repositories[i:end]

		// Process batch
		if err := bp.processor(batch); err != nil {
			return err
		}

		// Update processed count
		bp.optimizer.UpdateProcessedCount(len(batch))

		// Optional: trigger GC if needed
		if bp.optimizer.ShouldTriggerGC() {
			bp.optimizer.TriggerGC()
		}
	}

	return nil
}

// MemoryMonitor monitors memory usage during operations
type MemoryMonitor struct {
	mu              sync.RWMutex
	maxMemoryUsage  uint64
	currentUsage    uint64
	allocationCount uint64
	gcCount         uint32
	startTime       time.Time
}

// NewMemoryMonitor creates a new memory monitor
func NewMemoryMonitor() *MemoryMonitor {
	return &MemoryMonitor{
		startTime: time.Now(),
	}
}

// UpdateStats updates memory statistics
func (mm *MemoryMonitor) UpdateStats() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	mm.currentUsage = m.Alloc
	if m.Alloc > mm.maxMemoryUsage {
		mm.maxMemoryUsage = m.Alloc
	}
	mm.allocationCount = m.Mallocs
	mm.gcCount = m.NumGC
}

// GetStats returns current memory statistics
func (mm *MemoryMonitor) GetStats() MemoryStats {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	return MemoryStats{
		MaxMemoryUsage:  mm.maxMemoryUsage,
		CurrentUsage:    mm.currentUsage,
		AllocationCount: mm.allocationCount,
		GCCount:         mm.gcCount,
		Duration:        time.Since(mm.startTime),
	}
}

// MemoryStats represents memory usage statistics
type MemoryStats struct {
	MaxMemoryUsage  uint64        `json:"max_memory_usage"`
	CurrentUsage    uint64        `json:"current_usage"`
	AllocationCount uint64        `json:"allocation_count"`
	GCCount         uint32        `json:"gc_count"`
	Duration        time.Duration `json:"duration"`
}

// String returns a string representation of memory stats
func (ms MemoryStats) String() string {
	return fmt.Sprintf("Memory: %d bytes (max: %d), Allocations: %d, GC: %d, Duration: %v",
		ms.CurrentUsage, ms.MaxMemoryUsage, ms.AllocationCount, ms.GCCount, ms.Duration)
}
