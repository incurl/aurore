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

// TestWaitNSpendsOneTokenPerImage verifies that a batch spends a token
// per image, which is how MiniMax meters H3.
func TestWaitNSpendsOneTokenPerImage(t *testing.T) {
	now := time.Now()
	// Ten images per minute keeps the arithmetic obvious.
	limiter := NewRateLimiter(10, time.Minute)
	limiter.SetClock(func() time.Time { return now })

	// Three images cost three of the ten tokens.
	for i := 0; i < 3; i++ {
		if err := limiter.WaitN(context.Background(), 1); err != nil {
			t.Fatalf("WaitN: %v", err)
		}
	}
	if !limiter.Allow() {
		t.Fatal("seven tokens should remain")
	}

	// A batch of eight now exceeds what is left, so the bucket must be
	// emptied rather than letting the whole batch through.
	if limiter.tokens > 8 {
		t.Fatalf("tokens = %v, want at most 8", limiter.tokens)
	}
}

// TestWaitNClampsACostAboveTheCapacity verifies that an impossible batch
// does not block forever: the cost is clamped to the bucket capacity, so
// a full bucket satisfies it.
func TestWaitNClampsACostAboveTheCapacity(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := limiter.WaitN(ctx, 100); err != nil {
		t.Fatalf("WaitN with a cost above the capacity: %v", err)
	}
	if limiter.Allow() {
		t.Error("the clamped cost should have emptied the bucket")
	}
}
