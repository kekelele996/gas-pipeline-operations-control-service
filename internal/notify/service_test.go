package notify

import (
	"context"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
)

func newSvc(rate float64) (*Service, *platform.FakeClock) {
	c := platform.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	s := NewService(NewStore(), c, Config{FailureRate: rate, MaxAttempts: 3, Backoff: time.Second})
	return s, c
}

func TestEnqueueValidates(t *testing.T) {
	s, _ := newSvc(0)
	if _, err := s.Enqueue(context.Background(), "", ChannelSMS, "s", "b"); err == nil {
		t.Fatalf("empty recipient should fail")
	}
	if _, err := s.Enqueue(context.Background(), "r", "carrier-pigeon", "s", "b"); err == nil {
		t.Fatalf("unknown channel should fail")
	}
	n, err := s.Enqueue(context.Background(), "r", ChannelSMS, "subject", "body")
	if err != nil {
		t.Fatal(err)
	}
	if n.State != StateQueued {
		t.Fatalf("state = %s", n.State)
	}
}

func TestPushBatchAllSucceedWhenNoFailures(t *testing.T) {
	s, _ := newSvc(0)
	for i := 0; i < 5; i++ {
		s.Enqueue(context.Background(), "r", ChannelEmail, "s", "b")
	}
	sent, failed, _ := s.PushBatch(context.Background())
	if sent != 5 || failed != 0 {
		t.Fatalf("sent=%d failed=%d, want 5/0", sent, failed)
	}
	if s.CountPending(context.Background()) != 0 {
		t.Fatalf("pending should be 0 after all sent")
	}
}

func TestPushBatchFailureRetriesThenTerminates(t *testing.T) {
	s, c := newSvc(1.0) // everything fails
	for i := 0; i < 3; i++ {
		s.Enqueue(context.Background(), "r", ChannelSMS, "s", "b")
	}
	// first batch: all go to retrying
	sent, failed, _ := s.PushBatch(context.Background())
	if sent != 0 || failed != 3 {
		t.Fatalf("first batch sent=%d failed=%d, want 0/3", sent, failed)
	}
	// advance past backoff and retry; after max attempts -> failed
	for i := 0; i < 5; i++ {
		c.Advance(10 * time.Second)
		s.PushBatch(context.Background())
	}
	// all should be failed terminal now
	tally := s.Store().CountByState()
	if tally[StateFailed.String()] != 3 {
		t.Fatalf("failed terminal = %d, want 3 (tally=%v)", tally[StateFailed.String()], tally)
	}
}

func TestRetryFailedResets(t *testing.T) {
	s, c := newSvc(1.0)
	s.Enqueue(context.Background(), "r", ChannelSMS, "s", "b")
	for i := 0; i < 5; i++ {
		c.Advance(10 * time.Second)
		s.PushBatch(context.Background())
	}
	n := s.RetryFailed(context.Background())
	if n != 1 {
		t.Fatalf("retry failed = %d, want 1", n)
	}
	// now back to retrying
	if s.CountPending(context.Background()) != 1 {
		t.Fatalf("pending should be 1 after retry reset")
	}
}

func TestBackoffExponential(t *testing.T) {
	s, _ := newSvc(1.0)
	if s.backoff(1) != time.Second {
		t.Fatalf("backoff(1) = %v", s.backoff(1))
	}
	if s.backoff(3) != 4*time.Second {
		t.Fatalf("backoff(3) = %v", s.backoff(3))
	}
}
