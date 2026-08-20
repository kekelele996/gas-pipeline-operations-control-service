package platform

import (
	"context"
	"testing"
	"time"
)

func TestNewIDUnique(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := NewID()
		if seen[id] {
			t.Fatalf("duplicate id %q at %d", id, i)
		}
		seen[id] = true
	}
}

func TestNewPrefixedIDHasPrefix(t *testing.T) {
	id := NewPrefixedID("SEG")
	if len(id) < 4 || id[:3] != "SEG" {
		t.Fatalf("bad prefixed id %q", id)
	}
}

func TestFakeClockAdvance(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewFakeClock(base)
	if !c.Now().Equal(base) {
		t.Fatalf("now != base")
	}
	c.Advance(5 * time.Second)
	if c.Now().Sub(base) != 5*time.Second {
		t.Fatalf("advance wrong: %v", c.Now().Sub(base))
	}
}

func TestFakeClockCtxDeadline(t *testing.T) {
	// CtxWithTimeout computes a deadline of (clock.Now()+d). Because Go's
	// context deadline uses the real monotonic clock, advancing the fake
	// clock does not fire it; we only assert the deadline is computed
	// relative to the fake now and not already expired.
	base := time.Now().Add(24 * time.Hour)
	c := NewFakeClock(base)
	ctx, cancel := c.CtxWithTimeout(context.Background(), time.Hour)
	defer cancel()
	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatalf("ctx has no deadline")
	}
	want := base.Add(time.Hour)
	if !dl.Equal(want) {
		t.Fatalf("deadline = %v, want %v", dl, want)
	}
	select {
	case <-ctx.Done():
		t.Fatalf("ctx should not be done yet")
	default:
	}
}

func TestSinceWithClock(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewFakeClock(base)
	c.Advance(3 * time.Second)
	d := Since(c, base)
	if d != 3*time.Second {
		t.Fatalf("since wrong: %v", d)
	}
}
