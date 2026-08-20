package nomination

import (
	"testing"
	"time"
)

func storeNomR010(st *Store, id, contractID, date string, vol float64, state State) Nomination {
	n := Nomination{
		ID: id, ContractID: contractID, ShipperID: "SH", Date: date,
		Volume: vol, State: state, CreatedAt: time.Now(),
	}
	st.Put(n)
	return n
}

// TestNominationHeldThenConfirmR010A: a held nomination must be able to move
// to confirmed.
func TestNominationHeldThenConfirmR010A(t *testing.T) {
	st := NewStore()
	storeNomR010(st, "N1", "C1", "2026-09-01", 50, StateSubmitted)
	if _, err := st.Hold("N1"); err != nil {
		t.Fatalf("hold: %v", err)
	}
	if _, err := st.ConfirmHeld("N1"); err != nil {
		t.Fatalf("confirm held: %v", err)
	}
	n, ok := st.Get("N1")
	if !ok {
		t.Fatal("nomination missing")
	}
	if n.State != StateConfirmed {
		t.Fatalf("state = %s, want confirmed", n.State)
	}
}

// TestNominationHeldCountsCapacityR010B: held nominations must count toward
// used contract capacity.
func TestNominationHeldCountsCapacityR010B(t *testing.T) {
	st := NewStore()
	storeNomR010(st, "N1", "C1", "2026-09-01", 40, StateSubmitted)
	storeNomR010(st, "N2", "C1", "2026-09-01", 60, StateSubmitted)
	if _, err := st.Hold("N2"); err != nil {
		t.Fatal(err)
	}
	used := st.CapacityUsed("C1", "2026-09-01")
	if used != 100 {
		t.Fatalf("capacity used = %g, want 100", used)
	}
}

// TestNominationHeldVisibleInListR010C: held nominations must appear in the
// contract-date listing.
func TestNominationHeldVisibleInListR010C(t *testing.T) {
	st := NewStore()
	storeNomR010(st, "N1", "C1", "2026-09-01", 50, StateSubmitted)
	if _, err := st.Hold("N1"); err != nil {
		t.Fatal(err)
	}
	list := st.ForContractDate("C1", "2026-09-01")
	found := false
	for _, n := range list {
		if n.ID == "N1" {
			found = true
		}
	}
	if !found {
		t.Fatal("held nomination missing from contract-date listing")
	}
}

// TestNominationConfirmHeldReturnsStateR010D: ConfirmHeld must return the
// nomination in its confirmed state.
func TestNominationConfirmHeldReturnsStateR010D(t *testing.T) {
	st := NewStore()
	storeNomR010(st, "N1", "C1", "2026-09-01", 50, StateSubmitted)
	if _, err := st.Hold("N1"); err != nil {
		t.Fatal(err)
	}
	out, err := st.ConfirmHeld("N1")
	if err != nil {
		t.Fatal(err)
	}
	if out.State != StateConfirmed {
		t.Fatalf("ConfirmHeld returned state %s, want confirmed", out.State)
	}
}
