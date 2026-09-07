package github

import (
	"fmt"
	"time"

	"github.com/google/go-github/v66/github"
)

// RetryConfig defines configuration for retry logic
type RetryConfig struct {
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	RetryableErrors []ErrorType
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []ErrorType{
			ErrorTypeRateLimit,
			ErrorTypeNetwork,
		},
	}
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation func() error

// minDuration returns the minimum of two durations
func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// WithRetry executes an operation with retry logic
func WithRetry(operation RetryableOperation, config *RetryConfig) error {
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
