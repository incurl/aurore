package minimax

import (
	"context"
	"sync"
	"time"
)

// DefaultRateLimit is the documented MiniMax image generation limit:
// 10 requests per minute for both free and paid tiers.
const DefaultRateLimit = 10

// DefaultRateWindow is the window over which the rate limit applies.
const DefaultRateWindow = time.Minute

// DefaultMaxConcurrent is the number of generation requests Aurore will
// have in flight at once. MiniMax documents a per-account RPM limit but
// no explicit concurrency ceiling, so this is a conservative local
// guard that also bounds memory for base64 responses.
const DefaultMaxConcurrent = 2

// RateLimiter is a token bucket that enforces the MiniMax RPM limit.
//
// The bucket refills continuously at RateLimit tokens per RateWindow. A
// caller must acquire a token before issuing a request. The limiter is
// safe for concurrent use.
type RateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	tokens   float64
	capacity float64
	last     time.Time

	// now is the clock, overridable in tests.
	now func() time.Time
}

// NewRateLimiter creates a limiter allowing limit requests per window.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = DefaultRateLimit
	}
	if window <= 0 {
		window = DefaultRateWindow
	}
	return &RateLimiter{
		limit:    limit,
		window:   window,
		tokens:   float64(limit),
		capacity: float64(limit),
		last:     time.Now(),
		now:      time.Now,
	}
}

// NewDefaultRateLimiter creates a limiter with the documented MiniMax
// defaults of 10 requests per minute.
func NewDefaultRateLimiter() *RateLimiter {
	return NewRateLimiter(DefaultRateLimit, DefaultRateWindow)
}

// refillLocked adds the tokens accrued since the last call. The caller
// must hold the mutex.
func (r *RateLimiter) refillLocked() {
	now := r.now()
	elapsed := now.Sub(r.last)
	if elapsed <= 0 {
		return
	}
	r.last = now
	refill := float64(r.limit) * elapsed.Seconds() / r.window.Seconds()
	r.tokens = min(r.tokens+refill, r.capacity)
}

// Wait blocks until a token is available or the context is cancelled.
// It returns ctx.Err() if the context is cancelled first.
func (r *RateLimiter) Wait(ctx context.Context) error {
	for {
		r.mu.Lock()
		r.refillLocked()
		if r.tokens >= 1 {
			r.tokens--
			r.mu.Unlock()
			return nil
		}
		// Compute how long until the next token is available.
		needed := 1 - r.tokens
		perSecond := float64(r.limit) / r.window.Seconds()
		wait := time.Duration(needed / perSecond * float64(time.Second))
		r.mu.Unlock()

		if wait <= 0 {
			wait = time.Millisecond
		}
		// Cap the sleep so cancellation is observed promptly.
		if wait > 250*time.Millisecond {
			wait = 250 * time.Millisecond
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// Allow reports whether a token is available right now, without
// consuming one.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refillLocked()
	return r.tokens >= 1
}

// SetClock replaces the limiter clock and rebases the refill window. It
// exists for deterministic tests.
func (r *RateLimiter) SetClock(now func() time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.now = now
	// Rebase so that the elapsed time is measured against the new clock
	// rather than the one that was active when the limiter was created.
	r.last = r.now()
}

// min returns the smaller of two floats.
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
