package audit

import (
	"context"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
)

func newSvc() (*Service, *platform.FakeClock) {
	c := platform.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	return NewService(NewStore(100), c), c
}

func TestRecordAndQuery(t *testing.T) {
	s, c := newSvc()
	c.Advance(time.Second)
	s.Record(context.Background(), "alice", "create_contract", "contract", "C1", "new")
	c.Advance(time.Second)
	s.Record(context.Background(), "bob", "upsert_segment", "segment", "S1", "")
	c.Advance(time.Second)
	s.Record(context.Background(), "alice", "submit_nomination", "nomination", "N1", "")

	all := s.Query(context.Background(), Query{Limit: 10})
	if len(all) != 3 {
		t.Fatalf("got %d entries", len(all))
	}
	// newest first
	if all[0].Action != "submit_nomination" {
		t.Fatalf("ordering wrong: %s", all[0].Action)
	}
	byActor := s.Query(context.Background(), Query{Actor: "alice"})
	if len(byActor) != 2 {
		t.Fatalf("alice entries = %d", len(byActor))
	}
	byTarget := s.Query(context.Background(), Query{TargetType: "segment"})
	if len(byTarget) != 1 {
		t.Fatalf("segment entries = %d", len(byTarget))
	}
}

func TestRetentionDropsOldest(t *testing.T) {
	s, c := newSvc()
	for i := 0; i < 200; i++ {
		c.Advance(time.Second)
		s.Record(context.Background(), "u", "act", "t", "id", "")
	}
	if s.Count(context.Background()) > 100 {
		t.Fatalf("retention exceeded: %d", s.Count(context.Background()))
	}
}

func TestSummaryTally(t *testing.T) {
	s, _ := newSvc()
	s.Record(context.Background(), "u", "create_contract", "contract", "C1", "")
	s.Record(context.Background(), "u", "create_contract", "contract", "C2", "")
	s.Record(context.Background(), "u", "upsert_segment", "segment", "S1", "")
	sum := s.Summary(context.Background())
	if sum.Total != 3 {
		t.Fatalf("total = %d", sum.Total)
	}
	if sum.ByAction["create_contract"] != 2 {
		t.Fatalf("tally = %v", sum.ByAction)
	}
}
