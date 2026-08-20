package audit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
)

func newSvcR006(t *testing.T, retention int) *Service {
	t.Helper()
	return NewService(NewStore(retention), platform.SystemClock{})
}

func addN(svc *Service, n int) []string {
	var ids []string
	for i := 0; i < n; i++ {
		svc.Record(context.Background(), "actor", "action", "target", fmt.Sprintf("T-%d", i),
			fmt.Sprintf("entry %d", i))
		e, _ := svc.Store().Get("")
		_ = e
	}
	for _, e := range svc.Store().All() {
		ids = append(ids, e.ID)
	}
	return ids
}

// TestAuditRetentionKeepsNewestR006A: retention must keep the newest entries,
// dropping the oldest.
func TestAuditRetentionKeepsNewestR006A(t *testing.T) {
	svc := newSvcR006(t, 50)
	for i := 0; i < 60; i++ {
		svc.Record(context.Background(), "actor", "action", "target", fmt.Sprintf("T-%d", i),
			fmt.Sprintf("entry %d", i))
	}
	all := svc.Store().All()
	if len(all) > 50 {
		t.Fatalf("log grew past retention: %d", len(all))
	}
	for _, e := range all {
		if e.TargetID == "T-0" {
			t.Fatal("ancient entry survived retention; oldest entries must be evicted")
		}
	}
}

// TestAuditQueryIsolatedR006B: mutating a query result must not corrupt the
// stored log.
func TestAuditQueryIsolatedR006B(t *testing.T) {
	svc := newSvcR006(t, 0)
	for i := 0; i < 10; i++ {
		svc.Record(context.Background(), "actor", "action", "target", fmt.Sprintf("T-%d", i),
			fmt.Sprintf("entry %d", i))
	}
	rows := svc.Store().Query(Query{})
	if len(rows) < 2 {
		t.Fatal("expected multiple rows")
	}
	rows[0].Actor = "MUTATED"
	again := svc.Store().Query(Query{})
	for _, e := range again {
		if e.Actor == "MUTATED" {
			t.Fatal("caller edit corrupted the stored log")
		}
	}
}

// TestAuditAllIsolatedR006C: mutating the slice returned by All must not
// corrupt the stored log.
func TestAuditAllIsolatedR006C(t *testing.T) {
	svc := newSvcR006(t, 0)
	for i := 0; i < 10; i++ {
		svc.Record(context.Background(), "actor", "action", "target", fmt.Sprintf("T-%d", i),
			fmt.Sprintf("entry %d", i))
	}
	all := svc.Store().All()
	if len(all) < 1 {
		t.Fatal("expected entries")
	}
	firstID := all[0].ID
	all[0].Actor = "MUTATED"
	got, ok := svc.Store().Get(firstID)
	if !ok {
		t.Fatal("entry not found")
	}
	if got.Actor == "MUTATED" {
		t.Fatal("caller edit corrupted the stored log")
	}
}

// TestAuditRecentIsolatedR006D: the recent view must be independent of later
// writes and of caller edits.
func TestAuditRecentIsolatedR006D(t *testing.T) {
	svc := newSvcR006(t, 0)
	for i := 0; i < 10; i++ {
		svc.Record(context.Background(), "actor", "action", "target", fmt.Sprintf("T-%d", i),
			fmt.Sprintf("entry %d", i))
	}
	recent := svc.Recent(context.Background(), 5)
	if len(recent) != 5 {
		t.Fatalf("recent len = %d, want 5", len(recent))
	}
	// caller edits must not corrupt the log
	recent[0].Actor = "MUTATED"
	all := svc.Store().All()
	for _, e := range all {
		if e.Actor == "MUTATED" {
			t.Fatal("recent edit corrupted the stored log")
		}
	}
}

// silence time import when unused in some builds
var _ = time.Now
