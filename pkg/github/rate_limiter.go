package github

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/google/go-github/v66/github"
)

// MultiRepoRateLimiter manages GitHub API rate limits across multiple repositories
type MultiRepoRateLimiter interface {
	// Wait blocks until it's safe to make an API call
	Wait(ctx context.Context) error

	// UpdateLimits updates the rate limiter with current GitHub API rate limit information
	UpdateLimits(remaining, resetTime int)

	// GetDelay returns the current delay before the next API call
	GetDelay() time.Duration

	// SetConcurrencyLimit sets the maximum number of concurrent repository operations
	SetConcurrencyLimit(limit int)

	// AcquireSlot acquires a slot for concurrent processing (blocks if limit reached)
	AcquireSlot(ctx context.Context) error

	// ReleaseSlot releases a slot for concurrent processing
	ReleaseSlot()

	// GetStats returns current rate limiter statistics
	GetStats() RateLimiterStats
}

// RateLimiterStats provides statistics about rate limiter usage
type RateLimiterStats struct {
	RemainingRequests  int           `json:"remaining_requests"`
	ResetTime          time.Time     `json:"reset_time"`
	CurrentDelay       time.Duration `json:"current_delay"`
	ConcurrentSlots    int           `json:"concurrent_slots"`
	MaxConcurrentSlots int           `json:"max_concurrent_slots"`
	TotalWaits         int64         `json:"total_waits"`
	TotalDelayTime     time.Duration `json:"total_delay_time"`
}

// RateLimiterConfig configures the rate limiter behavior
type RateLimiterConfig struct {
	// BaseDelay is the minimum delay between requests
	BaseDelay time.Duration

	// MaxDelay is the maximum delay between requests
	MaxDelay time.Duration

	// BackoffFactor is the exponential backoff multiplier
	BackoffFactor float64

	// Jitter adds randomness to delays to avoid thundering herd
	Jitter float64

	// ConcurrencyLimit is the maximum number of concurrent operations
	ConcurrencyLimit int

	// MinRemainingRequests is the threshold below which we start aggressive throttling
	MinRemainingRequests int

	// AggressiveThrottleDelay is the delay when remaining requests are low
	AggressiveThrottleDelay time.Duration

	// AdaptiveConcurrency enables dynamic concurrency adjustment based on rate limits
	AdaptiveConcurrency bool

	// MinConcurrency is the minimum number of concurrent operations when adaptive
	MinConcurrency int

	// MaxConcurrency is the maximum number of concurrent operations when adaptive
	MaxConcurrency int
}

// DefaultRateLimiterConfig returns a default rate limiter configuration
func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		BaseDelay:               100 * time.Millisecond,
		MaxDelay:                30 * time.Second,
		BackoffFactor:           2.0,
		Jitter:                  0.1,
		ConcurrencyLimit:        5,
		MinRemainingRequests:    100,
		AggressiveThrottleDelay: 2 * time.Second,
		AdaptiveConcurrency:     true,
		MinConcurrency:          1,
		MaxConcurrency:          20,
	}
}

// multiRepoRateLimiter implements the MultiRepoRateLimiter interface
type multiRepoRateLimiter struct {
	config *RateLimiterConfig
	mu     sync.RWMutex

	// Rate limit tracking
	remaining int
	resetTime time.Time
	lastCall  time.Time

	// Concurrency control
	semaphore chan struct{}

	// Statistics
	stats RateLimiterStats

	// Random source for jitter
	rand *rand.Rand
}

// NewMultiRepoRateLimiter creates a new rate limiter for multi-repository operations
func NewMultiRepoRateLimiter(config *RateLimiterConfig) MultiRepoRateLimiter {
	if config == nil {
		config = DefaultRateLimiterConfig()
	}

	limiter := &multiRepoRateLimiter{
		config:    config,
		remaining: 5000, // GitHub's default rate limit
		resetTime: time.Now().Add(time.Hour),
		semaphore: make(chan struct{}, config.ConcurrencyLimit),
		rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	limiter.stats.MaxConcurrentSlots = config.ConcurrencyLimit

	return limiter
}

// Wait blocks until it's safe to make an API call
func (rl *multiRepoRateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()

	delay := rl.calculateDelay()
	if delay > 0 {
		rl.stats.TotalWaits++
		rl.stats.TotalDelayTime += delay

		// Release the lock while waiting
		rl.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue
		}

		// Re-acquire lock to update lastCall
		rl.mu.Lock()
	}

	rl.lastCall = time.Now()
	rl.mu.Unlock()
	return nil
}

// UpdateLimits updates the rate limiter with current GitHub API rate limit information
func (rl *multiRepoRateLimiter) UpdateLimits(remaining, resetTime int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.remaining = remaining
	rl.resetTime = time.Unix(int64(resetTime), 0)
	rl.stats.RemainingRequests = remaining
	rl.stats.ResetTime = rl.resetTime

	// Adaptive concurrency adjustment based on remaining rate limit
	if rl.config.AdaptiveConcurrency {
		rl.adjustConcurrencyBasedOnRateLimit(remaining)
	}
}

// adjustConcurrencyBasedOnRateLimit dynamically adjusts concurrency based on rate limit status
func (rl *multiRepoRateLimiter) adjustConcurrencyBasedOnRateLimit(remaining int) {
	// Calculate optimal concurrency based on remaining requests
	// More remaining requests = higher concurrency allowed
	// Fewer remaining requests = lower concurrency to preserve rate limit

	var newLimit int

	if remaining > 2000 { // High rate limit remaining
		newLimit = rl.config.MaxConcurrency
	} else if remaining > 1000 { // Medium rate limit remaining
		newLimit = (rl.config.MaxConcurrency + rl.config.MinConcurrency) / 2
	} else if remaining > 500 { // Low rate limit remaining
		newLimit = rl.config.MinConcurrency + 2
	} else { // Very low rate limit remaining
		newLimit = rl.config.MinConcurrency
	}

	// Only adjust if the new limit is different and within bounds
	if newLimit != rl.config.ConcurrencyLimit &&
		newLimit >= rl.config.MinConcurrency &&
		newLimit <= rl.config.MaxConcurrency {

		// Create new semaphore with adjusted limit
		rl.semaphore = make(chan struct{}, newLimit)
		rl.config.ConcurrencyLimit = newLimit
		rl.stats.MaxConcurrentSlots = newLimit
	}
}

// GetDelay returns the current delay before the next API call
func (rl *multiRepoRateLimiter) GetDelay() time.Duration {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return rl.calculateDelay()
}

// SetConcurrencyLimit sets the maximum number of concurrent repository operations
func (rl *multiRepoRateLimiter) SetConcurrencyLimit(limit int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if limit <= 0 {
		limit = 1
	}

	// Only recreate semaphore if limit actually changed
	if limit != rl.config.ConcurrencyLimit {
		// Create new semaphore with updated limit
		rl.semaphore = make(chan struct{}, limit)
		rl.config.ConcurrencyLimit = limit
		rl.stats.MaxConcurrentSlots = limit
	}
}

// AcquireSlot acquires a slot for concurrent processing (blocks if limit reached)
func (rl *multiRepoRateLimiter) AcquireSlot(ctx context.Context) error {
	select {
	case rl.semaphore <- struct{}{}:
		rl.mu.Lock()
		rl.stats.ConcurrentSlots++
		rl.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ReleaseSlot releases a slot for concurrent processing
func (rl *multiRepoRateLimiter) ReleaseSlot() {
	select {
	case <-rl.semaphore:
		rl.mu.Lock()
		rl.stats.ConcurrentSlots--
		rl.mu.Unlock()
	default:
		// No slot to release
	}
}

// GetStats returns current rate limiter statistics
func (rl *multiRepoRateLimiter) GetStats() RateLimiterStats {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	stats := rl.stats
	stats.CurrentDelay = rl.calculateDelay()
	return stats
}

// calculateDelay calculates the delay needed before the next API call
func (rl *multiRepoRateLimiter) calculateDelay() time.Duration {
	now := time.Now()

	// If rate limit has reset, no delay needed
	if now.After(rl.resetTime) {
		return 0
	}

	var totalDelay time.Duration

	// Calculate base delay from last call
	if !rl.lastCall.IsZero() {
		timeSinceLastCall := now.Sub(rl.lastCall)
		if timeSinceLastCall < rl.config.BaseDelay {
			totalDelay = rl.config.BaseDelay - timeSinceLastCall
		}
	}

	// Apply aggressive throttling if remaining requests are low
	if rl.remaining < rl.config.MinRemainingRequests {
		aggressiveDelay := rl.calculateAggressiveDelay()
		if aggressiveDelay > totalDelay {
			totalDelay = aggressiveDelay
		}
	}

	// Apply exponential backoff if we're hitting rate limits frequently
	if rl.remaining < 500 { // Less than 10% of default limit
		backoffMultiplier := math.Pow(rl.config.BackoffFactor, float64(5000-rl.remaining)/1000)
		backoffDelay := time.Duration(float64(rl.config.BaseDelay) * backoffMultiplier)
		if backoffDelay > totalDelay {
			totalDelay = backoffDelay
		}
	}

	// Apply jitter to avoid thundering herd
	if rl.config.Jitter > 0 && totalDelay > 0 {
		jitterAmount := float64(totalDelay) * rl.config.Jitter
		jitter := time.Duration(rl.rand.Float64() * jitterAmount)
		totalDelay += jitter
	}

	// Cap at maximum delay
	totalDelay = minDuration(totalDelay, rl.config.MaxDelay)

	return totalDelay
}

// calculateAggressiveDelay calculates delay when remaining requests are low
func (rl *multiRepoRateLimiter) calculateAggressiveDelay() time.Duration {
	if rl.remaining <= 0 {
		// No requests remaining, wait until reset
		waitTime := time.Until(rl.resetTime)
		if waitTime > 0 {
			return waitTime
		}
		return 0
	}

	// Calculate proportional delay based on remaining requests
	remainingRatio := float64(rl.remaining) / float64(rl.config.MinRemainingRequests)
	if remainingRatio >= 1.0 {
		return 0 // No aggressive throttling needed
	}

	// Inverse relationship: fewer remaining requests = longer delay
	// When remaining = 0, delayMultiplier = 1.0 (full delay)
	// When remaining = MinRemainingRequests, delayMultiplier = 0.0 (no delay)
	delayMultiplier := 1.0 - remainingRatio
	delay := time.Duration(float64(rl.config.AggressiveThrottleDelay) * delayMultiplier)

	return delay
}

// RateLimitAwareClient wraps a GitHub client with rate limiting
type RateLimitAwareClient struct {
	client      *github.Client
	rateLimiter MultiRepoRateLimiter
	ctx         context.Context
}

// NewRateLimitAwareClient creates a new rate limit aware GitHub client
func NewRateLimitAwareClient(client *github.Client, rateLimiter MultiRepoRateLimiter) *RateLimitAwareClient {
	return &RateLimitAwareClient{
		client:      client,
		rateLimiter: rateLimiter,
		ctx:         context.Background(),
	}
}

// WithContext returns a new client with the given context
func (c *RateLimitAwareClient) WithContext(ctx context.Context) *RateLimitAwareClient {
	return &RateLimitAwareClient{
		client:      c.client,
		rateLimiter: c.rateLimiter,
		ctx:         ctx,
	}
}

// executeWithRateLimit executes a GitHub API call with rate limiting
func (c *RateLimitAwareClient) executeWithRateLimit(operation func() (*github.Response, error)) (*github.Response, error) {
	// Wait for rate limiter
	if err := c.rateLimiter.Wait(c.ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait failed: %w", err)
	}

	// Execute the operation
	resp, err := operation()

	// Update rate limiter with response headers
	if resp != nil && resp.Rate.Remaining != 0 {
		c.rateLimiter.UpdateLimits(resp.Rate.Remaining, int(resp.Rate.Reset.Unix()))
	}

	return resp, err
}

// GetRepository retrieves a repository with rate limiting
func (c *RateLimitAwareClient) GetRepository(owner, name string) (*github.Repository, *github.Response, error) {
	var repo *github.Repository
	var resp *github.Response
	var err error

	resp, err = c.executeWithRateLimit(func() (*github.Response, error) {
		repo, resp, err = c.client.Repositories.Get(c.ctx, owner, name)
		return resp, err
	})

	return repo, resp, err
}

// CreateRepository creates a repository with rate limiting
func (c *RateLimitAwareClient) CreateRepository(org string, repo *github.Repository) (*github.Repository, *github.Response, error) {
	var createdRepo *github.Repository
	var resp *github.Response
	var err error

	resp, err = c.executeWithRateLimit(func() (*github.Response, error) {
		createdRepo, resp, err = c.client.Repositories.Create(c.ctx, org, repo)
		return resp, err
	})

	return createdRepo, resp, err
}

// UpdateRepository updates a repository with rate limiting
func (c *RateLimitAwareClient) UpdateRepository(owner, name string, repo *github.Repository) (*github.Repository, *github.Response, error) {
	var updatedRepo *github.Repository
	var resp *github.Response
	var err error

	resp, err = c.executeWithRateLimit(func() (*github.Response, error) {
		updatedRepo, resp, err = c.client.Repositories.Edit(c.ctx, owner, name, repo)
		return resp, err
	})

	return updatedRepo, resp, err
}

// RetryWithRateLimit executes an operation with both retry logic and rate limiting
func RetryWithRateLimit(operation RetryableOperation, rateLimiter MultiRepoRateLimiter, config *RetryConfig) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(delay)

			// Exponential backoff with jitter
			delay = time.Duration(float64(delay) * config.BackoffFactor)
			delay = minDuration(delay, config.MaxDelay)
		}

		// Wait for rate limiter before each attempt
		ctx := context.Background()
		if err := rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limiter wait failed: %w", err)
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if ghErr, ok := err.(*Error); ok {
			if !ghErr.IsRetryable() {
				return err
			}

			// Special handling for rate limit errors
			if ghErr.Type == ErrorTypeRateLimit {
				if rateLimitErr, ok := ghErr.Cause.(*github.RateLimitError); ok {
					// Update rate limiter with rate limit information
					rateLimiter.UpdateLimits(0, int(rateLimitErr.Rate.Reset.Unix()))

					// Wait until rate limit resets
					resetTime := rateLimitErr.Rate.Reset.Time
					waitTime := time.Until(resetTime)
					if waitTime > 0 && waitTime < 5*time.Minute {
						time.Sleep(waitTime)
						continue
					}
				}
			}
		} else {
			// For non-GitHubError types, don't retry
			return err
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", config.MaxRetries, lastErr)
}
