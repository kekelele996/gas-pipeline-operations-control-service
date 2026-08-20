package nomination

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/contract"
	"gas-pipeline-operations-control-service/internal/platform"
)

// fakeContractService is a real contract.Service-backed test double: we use
// the actual contract.Service so capacity reservation is genuinely atomic.
type testEnv struct {
	contracts *contract.Service
	noms      *Service
	clock     *platform.FakeClock
}

func newEnv(t *testing.T) (*testEnv, string) {
	t.Helper()
	c := platform.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	cstore := contract.NewStore()
	csvc := contract.NewService(cstore, c, nil)
	nstore := NewStore()
	nsvc := NewService(nstore, csvc, c, nil)
	// a contract with daily capacity 1000
	ct, err := csvc.Create(context.Background(), contract.ContractInput{
		ShipperID: "S1", ShipperName: "Shipper One", Code: "C1",
		DailyVolume: 1000, ValidFrom: c.Now(), ValidTo: c.Now().Add(365 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &testEnv{contracts: csvc, noms: nsvc, clock: c}, ct.ID
}

func TestNominationStateFlow(t *testing.T) {
	env, cid := newEnv(t)
	n, err := env.noms.Create(context.Background(), Input{
		ContractID: cid, Date: "2026-01-01", Volume: 200, SubmittedBy: "alice",
	})
	if err != nil {
		t.Fatal(err)
	}
	if n.State != StateDraft {
		t.Fatalf("state = %s", n.State)
	}
	if _, err := env.noms.Submit(context.Background(), n.ID, "alice"); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := env.noms.Confirm(context.Background(), n.ID, "bob"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := env.noms.Execute(context.Background(), n.ID); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// cannot submit an executed nomination
	if _, err := env.noms.Submit(context.Background(), n.ID, "x"); err == nil {
		t.Fatalf("submit after execute should fail")
	}
	rem, _ := env.contracts.Remaining(context.Background(), cid)
	if rem != 800 {
		t.Fatalf("remaining = %g, want 800", rem)
	}
}

func TestNominationCancelReleasesCapacity(t *testing.T) {
	env, cid := newEnv(t)
	n, _ := env.noms.Create(context.Background(), Input{
		ContractID: cid, Date: "2026-01-01", Volume: 300, SubmittedBy: "a",
	})
	env.noms.Submit(context.Background(), n.ID, "a")
	rem, _ := env.contracts.Remaining(context.Background(), cid)
	if rem != 700 {
		t.Fatalf("after submit remaining = %g, want 700", rem)
	}
	env.noms.Cancel(context.Background(), n.ID)
	rem, _ = env.contracts.Remaining(context.Background(), cid)
	if rem != 1000 {
		t.Fatalf("after cancel remaining = %g, want 1000", rem)
	}
}

func TestNominationOversellRejected(t *testing.T) {
	env, cid := newEnv(t)
	n1, _ := env.noms.Create(context.Background(), Input{ContractID: cid, Date: "2026-01-01", Volume: 800})
	env.noms.Submit(context.Background(), n1.ID, "a")
	n2, _ := env.noms.Create(context.Background(), Input{ContractID: cid, Date: "2026-01-01", Volume: 300})
	_, err := env.noms.Submit(context.Background(), n2.ID, "a")
	if err == nil {
		t.Fatalf("oversell should be rejected")
	}
}

func TestNominationInvalidTransition(t *testing.T) {
	env, cid := newEnv(t)
	n, _ := env.noms.Create(context.Background(), Input{ContractID: cid, Date: "2026-01-01", Volume: 100})
	// cannot confirm a draft (must submit first)
	if _, err := env.noms.Confirm(context.Background(), n.ID, "x"); err == nil {
		t.Fatalf("confirm from draft should fail")
	}
}

// TestNominationConcurrentCapacityNoOverbook submits many concurrent
// nominations against the same contract and asserts the total reserved
// volume never exceeds the contract's daily capacity.
func TestNominationConcurrentCapacityNoOverbook(t *testing.T) {
	env, cid := newEnv(t)
	const goroutines = 50
	const volume = 50.0 // total demand 2500 against capacity 1000
	var wg sync.WaitGroup
	start := make(chan struct{})
	var ok int64
	var fail int64
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			n, _ := env.noms.Create(context.Background(), Input{
				ContractID: cid, Date: "2026-01-01", Volume: volume,
				SubmittedBy: fmt.Sprintf("g%d", i),
			})
			_, err := env.noms.Submit(context.Background(), n.ID, fmt.Sprintf("g%d", i))
			if err == nil {
				atomic.AddInt64(&ok, 1)
			} else {
				atomic.AddInt64(&fail, 1)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	if ok+fail != goroutines {
		t.Fatalf("lost results: ok=%d fail=%d", ok, fail)
	}
	// total reserved must be <= capacity
	var reserved float64
	for _, n := range env.noms.List(context.Background()) {
		if n.Reserved {
			reserved += n.Volume
		}
	}
	if reserved > 1000.0 {
		t.Fatalf("overbooked: reserved=%g capacity=1000", reserved)
	}
	if reserved != 1000.0 && ok > 0 {
		// at least some should succeed; the first 20 (20*50=1000) should win
		if reserved < 1000.0 {
			t.Fatalf("under-reserved: %g (expected 1000)", reserved)
		}
	}
}
