package notify

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"
)

// advClock is a controllable clock so retry backoff can be advanced.
type advClock struct {
	mu sync.Mutex
	t  time.Time
}

func newAdvClock() *advClock { return &advClock{t: time.Now()} }

func (c *advClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *advClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newNotifySvcR002(t *testing.T, clk platformClock, cfg Config) *Service {
	t.Helper()
	return NewService(NewStore(), clk, cfg)
}

// platformClock is the minimal interface both real and fake clocks satisfy.
type platformClock interface {
	Now() time.Time
}

// enqueueConcurrentR002 enqueues n notifications from two concurrent
// goroutines behind a shared start barrier, then waits for both to finish.
func enqueueConcurrentR002(t *testing.T, svc *Service, n int) {
	t.Helper()
	half := n / 2
	start := make(chan struct{})
	var wg sync.WaitGroup
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start
			for i := 0; i < half; i++ {
				if _, err := svc.Enqueue(context.Background(), "ops", ChannelSMS, "subject", "body"); err != nil {
					t.Errorf("enqueue: %v", err)
					return
				}
			}
		}(g)
	}
	close(start)
	wg.Wait()
}

// TestNotifyBatchAllDeliveredR002A: after exhausting attempts every
// notification must reach a terminal state (sent/failed).
func TestNotifyBatchAllDeliveredR002A(t *testing.T) {
	clk := newAdvClock()
	svc := newNotifySvcR002(t, clk, Config{FailureRate: 1, MaxAttempts: 2, Backoff: time.Second})
	enqueueConcurrentR002(t, svc, 10)

	if _, _, err := svc.PushBatch(context.Background()); err != nil {
		t.Fatalf("first push: %v", err)
	}
	clk.Advance(3 * time.Second)
	if _, _, err := svc.PushBatch(context.Background()); err != nil {
		t.Fatalf("second push: %v", err)
	}
	for _, n := range svc.List(context.Background()) {
		if n.State != StateSent && n.State != StateFailed {
			t.Fatalf("notification %s still in %s after attempts exhausted", n.ID, n.State)
		}
	}
}

// TestNotifyBatchCancelNoLeakR002B: cancelling a batch push must not leak
// worker goroutines.
func TestNotifyBatchCancelNoLeakR002B(t *testing.T) {
	clk := newAdvClock()
	svc := newNotifySvcR002(t, clk, Config{FailureRate: 0, MaxAttempts: 3, Backoff: time.Second})
	enqueueConcurrentR002(t, svc, 20)

	before := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, _ = svc.PushBatch(ctx)
	time.Sleep(300 * time.Millisecond)
	after := runtime.NumGoroutine()
	if after > before+2 {
		t.Fatalf("goroutines leaked: before=%d after=%d", before, after)
	}
}

// TestNotifyBatchWorkerRaceR002C: batch push must not register workers
// concurrently with Wait (run with -race).
func TestNotifyBatchWorkerRaceR002C(t *testing.T) {
	clk := newAdvClock()
	svc := newNotifySvcR002(t, clk, Config{FailureRate: 0, MaxAttempts: 3, Backoff: time.Second})
	enqueueConcurrentR002(t, svc, 20)

	sent, failed, err := svc.PushBatch(context.Background())
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if sent != 20 || failed != 0 {
		t.Fatalf("sent=%d failed=%d, want 20/0", sent, failed)
	}
	for _, n := range svc.List(context.Background()) {
		if n.State != StateSent {
			t.Fatalf("notification %s not sent: %s", n.ID, n.State)
		}
	}
}

// TestNotifyBatchRetryBackoffRespectedR002D: a retrying notification must
// not be re-attempted before its backoff window elapses.
func TestNotifyBatchRetryBackoffRespectedR002D(t *testing.T) {
	clk := newAdvClock()
	svc := newNotifySvcR002(t, clk, Config{FailureRate: 1, MaxAttempts: 3, Backoff: time.Hour})
	enqueueConcurrentR002(t, svc, 6)

	if _, _, err := svc.PushBatch(context.Background()); err != nil {
		t.Fatalf("first push: %v", err)
	}
	// immediately push again: nothing should be due yet
	if _, _, err := svc.PushBatch(context.Background()); err != nil {
		t.Fatalf("second push: %v", err)
	}
	for _, n := range svc.List(context.Background()) {
		if n.State != StateRetrying || n.Attempt != 1 {
			t.Fatalf("notification %s re-attempted before backoff: state=%s attempt=%d",
				n.ID, n.State, n.Attempt)
		}
	}
}
