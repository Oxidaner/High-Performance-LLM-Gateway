package service

import (
	"sync"
	"time"
)

// CircuitBreaker implements a simple consecutive-failure breaker.
type CircuitBreaker struct {
	mu sync.Mutex

	failureThreshold int
	openTimeout      time.Duration

	consecutiveFailures int
	openedAt            time.Time
}

// NewCircuitBreaker creates a circuit breaker.
func NewCircuitBreaker(failureThreshold int, openTimeout time.Duration) *CircuitBreaker {
	if failureThreshold <= 0 {
		failureThreshold = 3
	}
	if openTimeout <= 0 {
		openTimeout = 30 * time.Second
	}
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		openTimeout:      openTimeout,
	}
}

// Allow reports whether calls should be attempted.
func (c *CircuitBreaker) Allow(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.openedAt.IsZero() {
		return true
	}

	if now.Sub(c.openedAt) >= c.openTimeout {
		// Half-open behavior: allow one trial and reset failure count.
		c.openedAt = time.Time{}
		c.consecutiveFailures = 0
		return true
	}

	return false
}

// RecordSuccess closes the breaker and clears failure state.
func (c *CircuitBreaker) RecordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.consecutiveFailures = 0
	c.openedAt = time.Time{}
}

// RecordFailure increments failure state and may open the breaker.
func (c *CircuitBreaker) RecordFailure(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.consecutiveFailures++
	if c.consecutiveFailures >= c.failureThreshold {
		c.openedAt = now
	}
}

// IsOpen reports if the breaker is currently open.
func (c *CircuitBreaker) IsOpen(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.openedAt.IsZero() {
		return false
	}
	return now.Sub(c.openedAt) < c.openTimeout
}
