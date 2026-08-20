package platform

import (
	"context"
	"sync"
	"time"
)

// Clock abstracts wall-clock time so domain services can be tested
// deterministically by injecting a fake clock.
type Clock interface {
	// Now returns the current time according to this clock.
	Now() time.Time
}

// SystemClock returns the real wall-clock time.
type SystemClock struct{}

// Now returns time.Now().
func (SystemClock) Now() time.Time { return time.Now() }

// FakeClock is a manually-advancing clock for tests. It is safe for concurrent
// use. Use Advance to move time forward.
type FakeClock struct {
	mu  sync.RWMutex
	now time.Time
}

// NewFakeClock returns a FakeClock anchored at the given time.
func NewFakeClock(t time.Time) *FakeClock {
	return &FakeClock{now: t}
}

// Now returns the current fake time.
func (f *FakeClock) Now() time.Time {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.now
}

// Advance moves the fake clock forward by d. Negative durations are ignored.
func (f *FakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if d > 0 {
		f.now = f.now.Add(d)
	}
}

// Set sets the fake clock to an absolute time.
func (f *FakeClock) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t
}

// CtxWithTimeout mirrors context.WithTimeout using this clock, so tests can
// exercise timeout behaviour without real waiting.
func (f *FakeClock) CtxWithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithDeadline(parent, f.Now().Add(d))
}

// MaxTime is the largest representable time, used as "no deadline".
var MaxTime = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

// Since returns the elapsed time since t, using the injected clock.
func Since(c Clock, t time.Time) time.Duration {
	if c == nil {
		return time.Since(t)
	}
	return c.Now().Sub(t)
}

// IsZero reports whether a clock is nil (unconfigured).
func IsZero(c Clock) bool { return c == nil }
