package service

import (
	"testing"
	"time"
)

func TestCircuitBreaker_OpenAndRecover(t *testing.T) {
	t.Parallel()

	now := time.Unix(100, 0)
	breaker := NewCircuitBreaker(2, 10*time.Second)

	if !breaker.Allow(now) {
		t.Fatalf("expected breaker to allow initially")
	}

	breaker.RecordFailure(now)
	if breaker.IsOpen(now) {
		t.Fatalf("expected breaker closed after first failure")
	}

	breaker.RecordFailure(now)
	if !breaker.IsOpen(now) {
		t.Fatalf("expected breaker open after reaching threshold")
	}
	if breaker.Allow(now) {
		t.Fatalf("expected breaker to block while open")
	}

	recoveryTime := now.Add(11 * time.Second)
	if !breaker.Allow(recoveryTime) {
		t.Fatalf("expected breaker to allow after timeout")
	}

	breaker.RecordSuccess()
	if breaker.IsOpen(recoveryTime) {
		t.Fatalf("expected breaker closed after success")
	}
}
