package minimax

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRateLimiterAllowsBurstUpToLimit(t *testing.T) {
	limiter := NewRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if err := limiter.Wait(context.Background()); err != nil {
			t.Fatalf("Wait #%d: %v", i, err)
		}
	}
	if limiter.Allow() {
		t.Error("the bucket must be empty after consuming the full burst")
	}
}

func TestRateLimiterRefillsOverTime(t *testing.T) {
	now := time.Now()
	limiter := NewRateLimiter(10, time.Minute)
	limiter.SetClock(func() time.Time { return now })

	// Drain the bucket.
	for i := 0; i < 10; i++ {
		if err := limiter.Wait(context.Background()); err != nil {
			t.Fatalf("Wait: %v", err)
		}
	}
	if limiter.Allow() {
		t.Fatal("the bucket must be empty")
	}

	// Advance by 30 seconds: 10 tokens per minute means 5 tokens.
	now = now.Add(30 * time.Second)
	if !limiter.Allow() {
		t.Fatal("the bucket must have refilled")
	}
}

func TestRateLimiterWaitsForRefill(t *testing.T) {
	now := time.Now()
	limiter := NewRateLimiter(60, time.Minute) // 1 token per second
	limiter.SetClock(func() time.Time { return now })

	for i := 0; i < 60; i++ {
		if err := limiter.Wait(context.Background()); err != nil {
			t.Fatalf("Wait: %v", err)
		}
	}

	// The next token arrives one second later.
	now = now.Add(time.Second)

	done := make(chan error, 1)
	go func() { done <- limiter.Wait(context.Background()) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Wait: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Wait did not return after the bucket refilled")
	}
}

func TestRateLimiterRespectsContextCancellation(t *testing.T) {
	limiter := NewRateLimiter(1, time.Hour)
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := limiter.Wait(ctx); err == nil {
		t.Fatal("expected the wait to be cancelled")
	}
}

func TestRateLimiterIsConcurrencySafe(t *testing.T) {
	limiter := NewRateLimiter(1000, time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				if err := limiter.Wait(context.Background()); err != nil {
					t.Errorf("Wait: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
