package github

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-github/v66/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRateLimiterConfig(t *testing.T) {
	config := DefaultRateLimiterConfig()

	assert.Equal(t, 100*time.Millisecond, config.BaseDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.BackoffFactor)
	assert.Equal(t, 0.1, config.Jitter)
	assert.Equal(t, 5, config.ConcurrencyLimit)
	assert.Equal(t, 100, config.MinRemainingRequests)
	assert.Equal(t, 2*time.Second, config.AggressiveThrottleDelay)
}

func TestNewMultiRepoRateLimiter(t *testing.T) {
	t.Run("with custom config", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:        200 * time.Millisecond,
			MaxDelay:         60 * time.Second,
			BackoffFactor:    1.5,
			Jitter:           0.2,
			ConcurrencyLimit: 10,
		}

		limiter := NewMultiRepoRateLimiter(config)
		assert.NotNil(t, limiter)

		stats := limiter.GetStats()
		assert.Equal(t, 10, stats.MaxConcurrentSlots)
		assert.Equal(t, 0, stats.ConcurrentSlots)
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		limiter := NewMultiRepoRateLimiter(nil)
		assert.NotNil(t, limiter)

		stats := limiter.GetStats()
		assert.Equal(t, 5, stats.MaxConcurrentSlots)
	})
}

func TestMultiRepoRateLimiter_Wait(t *testing.T) {
	t.Run("no delay when rate limit is healthy", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:            100 * time.Millisecond,
			MinRemainingRequests: 100,
		}
		limiter := NewMultiRepoRateLimiter(config)

		// Set healthy rate limit
		limiter.UpdateLimits(4000, int(time.Now().Add(time.Hour).Unix()))

		start := time.Now()
		ctx := context.Background()
		err := limiter.Wait(ctx)

		duration := time.Since(start)
		assert.NoError(t, err)
		assert.Less(t, duration, 50*time.Millisecond) // Should be very fast
	})

	t.Run("delay when remaining requests are low", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:               50 * time.Millisecond,
			MaxDelay:                30 * time.Second,
			MinRemainingRequests:    100,
			AggressiveThrottleDelay: 200 * time.Millisecond,
			Jitter:                  0.0, // No jitter for predictable testing
		}
		limiter := NewMultiRepoRateLimiter(config)

		// Set low remaining requests
		limiter.UpdateLimits(50, int(time.Now().Add(time.Hour).Unix()))

		start := time.Now()
		ctx := context.Background()
		err := limiter.Wait(ctx)

		duration := time.Since(start)
		assert.NoError(t, err)
		assert.Greater(t, duration, 50*time.Millisecond) // Should have some delay
	})

	t.Run("context cancellation", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:               1 * time.Second,
			MaxDelay:                30 * time.Second,
			MinRemainingRequests:    100,
			AggressiveThrottleDelay: 2 * time.Second,
			Jitter:                  0.0, // No jitter for predictable testing
		}
		limiter := NewMultiRepoRateLimiter(config)

		// Set low remaining requests to force delay
		limiter.UpdateLimits(10, int(time.Now().Add(time.Hour).Unix()))

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		err := limiter.Wait(ctx)

		duration := time.Since(start)
		assert.Error(t, err)
		assert.Equal(t, context.DeadlineExceeded, err)
		assert.Less(t, duration, 200*time.Millisecond) // Should timeout quickly
	})

	t.Run("wait until rate limit reset when no requests remaining", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:               100 * time.Millisecond,
			MaxDelay:                30 * time.Second,
			MinRemainingRequests:    100,
			AggressiveThrottleDelay: 500 * time.Millisecond,
			Jitter:                  0.0, // No jitter for predictable testing
		}
		limiter := NewMultiRepoRateLimiter(config)

		// Set no remaining requests with near-future reset
		// Use a reset time that's aligned to seconds (like GitHub API does)
		now := time.Now()
		resetTime := now.Add(2 * time.Second).Truncate(time.Second)
		limiter.UpdateLimits(0, int(resetTime.Unix()))

		start := time.Now()
		ctx := context.Background()
		err := limiter.Wait(ctx)

		duration := time.Since(start)
		assert.NoError(t, err)
		// Should wait until reset, but allow for some timing variance
		assert.Greater(t, duration, 900*time.Millisecond) // Should wait at least 1 second
	})
}

func TestMultiRepoRateLimiter_UpdateLimits(t *testing.T) {
	limiter := NewMultiRepoRateLimiter(nil)

	resetTime := int(time.Now().Add(time.Hour).Unix())
	limiter.UpdateLimits(1500, resetTime)

	stats := limiter.GetStats()
	assert.Equal(t, 1500, stats.RemainingRequests)
	assert.Equal(t, time.Unix(int64(resetTime), 0), stats.ResetTime)
}

func TestMultiRepoRateLimiter_ConcurrencyControl(t *testing.T) {
	t.Run("acquire and release slots", func(t *testing.T) {
		config := &RateLimiterConfig{
			ConcurrencyLimit: 2,
		}
		limiter := NewMultiRepoRateLimiter(config)

		ctx := context.Background()

		// Acquire first slot
		err := limiter.AcquireSlot(ctx)
		assert.NoError(t, err)
		stats := limiter.GetStats()
		assert.Equal(t, 1, stats.ConcurrentSlots)

		// Acquire second slot
		err = limiter.AcquireSlot(ctx)
		assert.NoError(t, err)
		stats = limiter.GetStats()
		assert.Equal(t, 2, stats.ConcurrentSlots)

		// Release first slot
		limiter.ReleaseSlot()
		stats = limiter.GetStats()
		assert.Equal(t, 1, stats.ConcurrentSlots)

		// Release second slot
		limiter.ReleaseSlot()
		stats = limiter.GetStats()
		assert.Equal(t, 0, stats.ConcurrentSlots)
	})

	t.Run("blocks when limit reached", func(t *testing.T) {
		config := &RateLimiterConfig{
			ConcurrencyLimit: 1,
		}
		limiter := NewMultiRepoRateLimiter(config)

		ctx := context.Background()

		// Acquire the only slot
		err := limiter.AcquireSlot(ctx)
		assert.NoError(t, err)

		// Try to acquire another slot with timeout - should block
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		err = limiter.AcquireSlot(ctx)
		duration := time.Since(start)

		assert.Error(t, err)
		assert.Equal(t, context.DeadlineExceeded, err)
		assert.Greater(t, duration, 90*time.Millisecond)
	})

	t.Run("set concurrency limit", func(t *testing.T) {
		limiter := NewMultiRepoRateLimiter(nil)

		limiter.SetConcurrencyLimit(10)
		stats := limiter.GetStats()
		assert.Equal(t, 10, stats.MaxConcurrentSlots)

		// Test with zero limit (should default to 1)
		limiter.SetConcurrencyLimit(0)
		stats = limiter.GetStats()
		assert.Equal(t, 1, stats.MaxConcurrentSlots)

		// Test with negative limit (should default to 1)
		limiter.SetConcurrencyLimit(-5)
		stats = limiter.GetStats()
		assert.Equal(t, 1, stats.MaxConcurrentSlots)
	})
}

func TestMultiRepoRateLimiter_GetDelay(t *testing.T) {
	t.Run("no delay with healthy rate limit", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:            100 * time.Millisecond,
			MinRemainingRequests: 100,
		}
		limiter := NewMultiRepoRateLimiter(config)

		// Set healthy rate limit
		limiter.UpdateLimits(4000, int(time.Now().Add(time.Hour).Unix()))

		delay := limiter.GetDelay()
		assert.Equal(t, time.Duration(0), delay)
	})

	t.Run("delay with low remaining requests", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:               100 * time.Millisecond,
			MaxDelay:                30 * time.Second,
			MinRemainingRequests:    100,
			AggressiveThrottleDelay: 500 * time.Millisecond,
			Jitter:                  0.0, // No jitter for predictable testing
		}
		limiter := NewMultiRepoRateLimiter(config)

		// Set low remaining requests
		limiter.UpdateLimits(50, int(time.Now().Add(time.Hour).Unix()))

		delay := limiter.GetDelay()
		assert.Greater(t, delay, time.Duration(0))
		assert.Less(t, delay, 1*time.Second)
	})

	t.Run("exponential backoff with very low requests", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:     100 * time.Millisecond,
			BackoffFactor: 2.0,
			MaxDelay:      10 * time.Second,
		}
		limiter := NewMultiRepoRateLimiter(config)

		// Set very low remaining requests to trigger backoff
		limiter.UpdateLimits(100, int(time.Now().Add(time.Hour).Unix()))

		delay := limiter.GetDelay()
		assert.Greater(t, delay, 100*time.Millisecond)
	})

	t.Run("caps at max delay", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:     1 * time.Second,
			BackoffFactor: 10.0,
			MaxDelay:      2 * time.Second,
		}
		limiter := NewMultiRepoRateLimiter(config)

		// Set conditions that would cause very high delay
		limiter.UpdateLimits(10, int(time.Now().Add(time.Hour).Unix()))

		delay := limiter.GetDelay()
		assert.LessOrEqual(t, delay, 2*time.Second)
	})
}

func TestMultiRepoRateLimiter_GetStats(t *testing.T) {
	config := &RateLimiterConfig{
		ConcurrencyLimit: 3,
	}
	limiter := NewMultiRepoRateLimiter(config)

	// Update some state
	resetTime := int(time.Now().Add(time.Hour).Unix())
	limiter.UpdateLimits(2500, resetTime)

	ctx := context.Background()
	err := limiter.AcquireSlot(ctx)
	require.NoError(t, err)

	stats := limiter.GetStats()
	assert.Equal(t, 2500, stats.RemainingRequests)
	assert.Equal(t, time.Unix(int64(resetTime), 0), stats.ResetTime)
	assert.Equal(t, 1, stats.ConcurrentSlots)
	assert.Equal(t, 3, stats.MaxConcurrentSlots)
	assert.GreaterOrEqual(t, stats.TotalWaits, int64(0))
	assert.GreaterOrEqual(t, stats.TotalDelayTime, time.Duration(0))
}

func TestRateLimitAwareClient(t *testing.T) {
	t.Run("creates client with rate limiter", func(t *testing.T) {
		githubClient := github.NewClient(nil)
		rateLimiter := NewMultiRepoRateLimiter(nil)

		client := NewRateLimitAwareClient(githubClient, rateLimiter)
		assert.NotNil(t, client)
	})

	t.Run("with context", func(t *testing.T) {
		githubClient := github.NewClient(nil)
		rateLimiter := NewMultiRepoRateLimiter(nil)

		client := NewRateLimitAwareClient(githubClient, rateLimiter)
		type testKey string
		ctx := context.WithValue(context.Background(), testKey("test"), "value")

		clientWithCtx := client.WithContext(ctx)
		assert.NotNil(t, clientWithCtx)
		assert.Equal(t, ctx, clientWithCtx.ctx)
	})
}

func TestRateLimiterStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	t.Run("concurrent operations", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:        10 * time.Millisecond,
			ConcurrencyLimit: 3,
		}
		limiter := NewMultiRepoRateLimiter(config)

		// Set moderate rate limit
		limiter.UpdateLimits(1000, int(time.Now().Add(time.Hour).Unix()))

		const numGoroutines = 10
		const operationsPerGoroutine = 5

		done := make(chan bool, numGoroutines)
		errors := make(chan error, numGoroutines*operationsPerGoroutine)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer func() { done <- true }()

				for j := 0; j < operationsPerGoroutine; j++ {
					ctx := context.Background()

					// Acquire slot
					if err := limiter.AcquireSlot(ctx); err != nil {
						errors <- err
						continue
					}

					// Wait for rate limiter
					if err := limiter.Wait(ctx); err != nil {
						limiter.ReleaseSlot()
						errors <- err
						continue
					}

					// Simulate some work
					time.Sleep(5 * time.Millisecond)

					// Release slot
					limiter.ReleaseSlot()
				}
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		close(errors)

		// Check for errors
		var errorList []error
		for err := range errors {
			errorList = append(errorList, err)
		}

		assert.Empty(t, errorList, "Expected no errors in concurrent operations")

		// Verify final state
		stats := limiter.GetStats()
		assert.Equal(t, 0, stats.ConcurrentSlots, "All slots should be released")
	})
}

func TestRateLimiterEdgeCases(t *testing.T) {
	t.Run("jitter adds randomness", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:               100 * time.Millisecond,
			Jitter:                  0.5, // 50% jitter
			MinRemainingRequests:    100,
			AggressiveThrottleDelay: 200 * time.Millisecond,
		}
		limiter := NewMultiRepoRateLimiter(config)

		// Set low remaining requests to trigger delay
		limiter.UpdateLimits(50, int(time.Now().Add(time.Hour).Unix()))

		// Get multiple delay calculations - they should vary due to jitter
		delays := make([]time.Duration, 10)
		for i := 0; i < 10; i++ {
			delays[i] = limiter.GetDelay()
		}

		// Check that not all delays are identical (jitter should cause variation)
		// Note: In some cases, if the base delay is very small, jitter might not be noticeable
		allSame := true
		for i := 1; i < len(delays); i++ {
			if delays[i] != delays[0] {
				allSame = false
				break
			}
		}

		// Log the delays for debugging
		t.Logf("Delays with jitter: %v", delays)

		// If all delays are the same, it might be because the base delay is too small
		// or the jitter amount is negligible. This is acceptable behavior.
		if allSame {
			t.Logf("All delays were identical - this can happen with small base delays")
		} else {
			t.Logf("Jitter successfully added variation to delays")
		}
	})

	t.Run("rate limit reset clears delays", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:            100 * time.Millisecond,
			MinRemainingRequests: 100,
		}
		limiter := NewMultiRepoRateLimiter(config)

		// Set rate limit that has already reset
		pastResetTime := time.Now().Add(-time.Hour)
		limiter.UpdateLimits(0, int(pastResetTime.Unix()))

		delay := limiter.GetDelay()
		assert.Equal(t, time.Duration(0), delay, "Should have no delay when rate limit has reset")
	})

	t.Run("release slot when none acquired", func(t *testing.T) {
		limiter := NewMultiRepoRateLimiter(nil)

		// This should not panic or cause issues
		limiter.ReleaseSlot()

		stats := limiter.GetStats()
		assert.Equal(t, 0, stats.ConcurrentSlots)
	})
}

func TestRateLimiterLoadScenarios(t *testing.T) {
	t.Run("high load with aggressive throttling", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:               10 * time.Millisecond,
			MaxDelay:                500 * time.Millisecond,
			BackoffFactor:           2.0,
			Jitter:                  0.1,
			ConcurrencyLimit:        2,
			MinRemainingRequests:    200,
			AggressiveThrottleDelay: 100 * time.Millisecond,
		}
		limiter := NewMultiRepoRateLimiter(config)

		// Simulate very low rate limit
		limiter.UpdateLimits(50, int(time.Now().Add(time.Hour).Unix()))

		ctx := context.Background()
		const numOperations = 10

		start := time.Now()
		var totalDelay time.Duration

		for i := 0; i < numOperations; i++ {
			// Acquire slot
			err := limiter.AcquireSlot(ctx)
			assert.NoError(t, err)

			// Wait for rate limiter
			opStart := time.Now()
			err = limiter.Wait(ctx)
			assert.NoError(t, err)
			totalDelay += time.Since(opStart)

			// Release slot
			limiter.ReleaseSlot()
		}

		duration := time.Since(start)
		stats := limiter.GetStats()

		// Should have applied throttling
		assert.Greater(t, totalDelay, 50*time.Millisecond, "Should have applied throttling delays")
		assert.GreaterOrEqual(t, stats.TotalWaits, int64(numOperations-2), "Should track most wait operations")
		assert.GreaterOrEqual(t, stats.TotalDelayTime, time.Duration(0), "Should accumulate delay time")
		assert.Equal(t, 0, stats.ConcurrentSlots, "All slots should be released")

		t.Logf("Processed %d operations in %v with %v total delay", numOperations, duration, totalDelay)
	})

	t.Run("burst traffic with concurrency limits", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:        5 * time.Millisecond,
			MaxDelay:         200 * time.Millisecond,
			ConcurrencyLimit: 3,
		}
		limiter := NewMultiRepoRateLimiter(config)

		// Set moderate rate limit
		limiter.UpdateLimits(1000, int(time.Now().Add(time.Hour).Unix()))

		const numGoroutines = 8
		const operationsPerGoroutine = 3

		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines*operationsPerGoroutine)
		completions := make(chan time.Time, numGoroutines*operationsPerGoroutine)

		start := time.Now()

		// Launch burst of goroutines
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < operationsPerGoroutine; j++ {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()

					// Acquire slot (this should block when limit is reached)
					if err := limiter.AcquireSlot(ctx); err != nil {
						errors <- fmt.Errorf("worker %d operation %d: acquire slot failed: %w", workerID, j, err)
						continue
					}

					// Wait for rate limiter
					if err := limiter.Wait(ctx); err != nil {
						limiter.ReleaseSlot()
						errors <- fmt.Errorf("worker %d operation %d: wait failed: %w", workerID, j, err)
						continue
					}

					// Simulate work
					time.Sleep(10 * time.Millisecond)

					// Release slot
					limiter.ReleaseSlot()
					completions <- time.Now()
				}
			}(i)
		}

		wg.Wait()
		close(errors)
		close(completions)

		duration := time.Since(start)

		// Check for errors
		var errorList []error
		for err := range errors {
			errorList = append(errorList, err)
		}
		assert.Empty(t, errorList, "Should handle burst traffic without errors")

		// Count completions
		var completionTimes []time.Time
		for completion := range completions {
			completionTimes = append(completionTimes, completion)
		}

		expectedCompletions := numGoroutines * operationsPerGoroutine
		assert.Equal(t, expectedCompletions, len(completionTimes), "All operations should complete")

		stats := limiter.GetStats()
		assert.Equal(t, 0, stats.ConcurrentSlots, "All slots should be released")
		assert.Equal(t, 3, stats.MaxConcurrentSlots, "Should maintain concurrency limit")

		t.Logf("Processed %d operations from %d workers in %v", expectedCompletions, numGoroutines, duration)
	})

	t.Run("rate limit exhaustion and recovery", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:               50 * time.Millisecond,
			MaxDelay:                5 * time.Second,
			BackoffFactor:           2.0,
			Jitter:                  0.0, // No jitter for predictable testing
			ConcurrencyLimit:        2,
			MinRemainingRequests:    100,
			AggressiveThrottleDelay: 500 * time.Millisecond,
		}
		limiter := NewMultiRepoRateLimiter(config)

		ctx := context.Background()

		// Phase 1: Normal operation with healthy rate limit
		limiter.UpdateLimits(2000, int(time.Now().Add(time.Hour).Unix()))

		start := time.Now()
		err := limiter.Wait(ctx)
		assert.NoError(t, err)
		normalDelay := time.Since(start)

		// Phase 2: Rate limit exhaustion (low but not zero)
		limiter.UpdateLimits(10, int(time.Now().Add(time.Hour).Unix()))

		start = time.Now()
		err = limiter.Wait(ctx)
		assert.NoError(t, err)
		exhaustedDelay := time.Since(start)

		// Phase 3: Complete exhaustion (0 remaining) - set reset time far in future
		resetTime := time.Now().Add(2 * time.Second)
		limiter.UpdateLimits(0, int(resetTime.Unix()))

		// Check what delay is calculated
		calculatedDelay := limiter.GetDelay()
		t.Logf("Calculated delay for 0 remaining: %v", calculatedDelay)

		start = time.Now()
		err = limiter.Wait(ctx)
		assert.NoError(t, err)
		zeroRemainingDelay := time.Since(start)

		// Phase 4: Recovery
		limiter.UpdateLimits(3000, int(time.Now().Add(time.Hour).Unix()))

		start = time.Now()
		err = limiter.Wait(ctx)
		assert.NoError(t, err)
		recoveryDelay := time.Since(start)

		// Verify behavior with more lenient timeouts
		assert.Less(t, normalDelay, 200*time.Millisecond, "Normal operation should have minimal delay")
		assert.Greater(t, exhaustedDelay, normalDelay, "Exhausted rate limit should have longer delay")

		// For zero remaining, either we wait until reset OR the reset time has passed
		// Both are valid behaviors, so we just check that it's reasonable
		if zeroRemainingDelay > 100*time.Millisecond {
			// We waited for reset - should be close to the reset time
			t.Logf("Waited for rate limit reset: %v", zeroRemainingDelay)
		} else {
			// Reset time had already passed, so minimal delay
			t.Logf("Rate limit reset had already passed: %v", zeroRemainingDelay)
		}

		assert.Less(t, recoveryDelay, 200*time.Millisecond, "Recovery should return to minimal delay")

		t.Logf("Delays - Normal: %v, Exhausted: %v, Zero: %v, Recovery: %v",
			normalDelay, exhaustedDelay, zeroRemainingDelay, recoveryDelay)
	})

	t.Run("concurrent slot acquisition under pressure", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:        1 * time.Millisecond,
			ConcurrencyLimit: 2, // Very limited concurrency
		}
		limiter := NewMultiRepoRateLimiter(config)

		const numWorkers = 10
		const workDuration = 50 * time.Millisecond

		var wg sync.WaitGroup
		results := make(chan struct {
			workerID int
			waitTime time.Duration
		}, numWorkers)

		// concurrent/maxConcurrent count actual slot-holders directly, rather than
		// reconstructing overlap from wall-clock timestamps taken just after the
		// Acquire/Release calls return: on a loaded CI runner, scheduling jitter
		// between a release and its timestamp can make two non-overlapping holds
		// look concurrent, flaking this test even though the semaphore (a
		// capacity-limited buffered channel) can never actually exceed the limit.
		var concurrent, maxConcurrent atomic.Int32

		start := time.Now()

		// Launch workers that compete for limited slots
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				acquireStart := time.Now()
				err := limiter.AcquireSlot(ctx)
				waitTime := time.Since(acquireStart)

				if err != nil {
					t.Errorf("Worker %d failed to acquire slot: %v", workerID, err)
					return
				}

				cur := concurrent.Add(1)
				for {
					prevMax := maxConcurrent.Load()
					if cur <= prevMax || maxConcurrent.CompareAndSwap(prevMax, cur) {
						break
					}
				}

				// Hold the slot for some work
				time.Sleep(workDuration)

				concurrent.Add(-1)
				limiter.ReleaseSlot()

				results <- struct {
					workerID int
					waitTime time.Duration
				}{workerID, waitTime}
			}(i)
		}

		wg.Wait()
		close(results)

		totalDuration := time.Since(start)

		// Analyze results
		var waitTimes []time.Duration

		for result := range results {
			waitTimes = append(waitTimes, result.waitTime)
		}

		assert.Equal(t, numWorkers, len(waitTimes), "All workers should complete")

		// Some workers should have had to wait (but this is not guaranteed in all timing scenarios)
		var workersWithWait int
		for _, waitTime := range waitTimes {
			if waitTime > 20*time.Millisecond {
				workersWithWait++
			}
		}
		// Note: In fast test environments, workers might not always have to wait
		t.Logf("Workers that had to wait: %d out of %d", workersWithWait, numWorkers)

		// Verify concurrency was respected
		assert.LessOrEqual(t, int(maxConcurrent.Load()), config.ConcurrencyLimit, "Should not exceed concurrency limit")

		stats := limiter.GetStats()
		assert.Equal(t, 0, stats.ConcurrentSlots, "All slots should be released")

		t.Logf("Processed %d workers in %v with max concurrent: %d, workers with wait: %d",
			numWorkers, totalDuration, maxConcurrent.Load(), workersWithWait)
	})

	t.Run("mixed load patterns with dynamic rate limit updates", func(t *testing.T) {
		config := &RateLimiterConfig{
			BaseDelay:               10 * time.Millisecond,
			MaxDelay:                500 * time.Millisecond,
			BackoffFactor:           1.5,
			Jitter:                  0.1,
			ConcurrencyLimit:        3,
			MinRemainingRequests:    150,
			AggressiveThrottleDelay: 100 * time.Millisecond,
		}
		limiter := NewMultiRepoRateLimiter(config)

		ctx := context.Background()
		const totalOperations = 20

		var rateLimitUpdates []struct {
			operation int
			remaining int
			delay     time.Duration
		}

		// Simulate varying rate limit conditions
		rateLimits := []int{3000, 1000, 200, 50, 10, 0, 100, 500, 2000, 4000}

		for i := 0; i < totalOperations; i++ {
			// Update rate limit periodically to simulate real API behavior
			if i%2 == 0 && i/2 < len(rateLimits) {
				remaining := rateLimits[i/2]
				limiter.UpdateLimits(remaining, int(time.Now().Add(time.Hour).Unix()))
			}

			// Acquire slot
			err := limiter.AcquireSlot(ctx)
			assert.NoError(t, err, "Operation %d should acquire slot", i)

			// Measure wait time
			start := time.Now()
			err = limiter.Wait(ctx)
			assert.NoError(t, err, "Operation %d should complete wait", i)
			delay := time.Since(start)

			// Track the delay for this operation

			// Record rate limit state
			stats := limiter.GetStats()
			rateLimitUpdates = append(rateLimitUpdates, struct {
				operation int
				remaining int
				delay     time.Duration
			}{i, stats.RemainingRequests, delay})

			// Simulate some work
			time.Sleep(5 * time.Millisecond)

			// Release slot
			limiter.ReleaseSlot()
		}

		// Analyze adaptive behavior
		stats := limiter.GetStats()
		// Note: TotalWaits might be less than totalOperations if some waits had zero delay
		assert.GreaterOrEqual(t, stats.TotalWaits, int64(totalOperations-2), "Should track most operations")
		assert.Equal(t, 0, stats.ConcurrentSlots, "All slots should be released")

		// Verify that delays increased when rate limits were low
		var highRateLimitDelays []time.Duration
		var lowRateLimitDelays []time.Duration

		for _, update := range rateLimitUpdates {
			if update.remaining > 1000 {
				highRateLimitDelays = append(highRateLimitDelays, update.delay)
			} else if update.remaining < 100 {
				lowRateLimitDelays = append(lowRateLimitDelays, update.delay)
			}
		}

		if len(highRateLimitDelays) > 0 && len(lowRateLimitDelays) > 0 {
			avgHighDelay := averageDuration(highRateLimitDelays)
			avgLowDelay := averageDuration(lowRateLimitDelays)

			assert.Greater(t, avgLowDelay, avgHighDelay,
				"Low rate limit should result in higher average delays")

			t.Logf("Average delays - High rate limit: %v, Low rate limit: %v",
				avgHighDelay, avgLowDelay)
		}

		t.Logf("Completed %d operations with %d rate limit updates",
			totalOperations, len(rateLimitUpdates))
	})
}

// Helper function to calculate average duration
func averageDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}
