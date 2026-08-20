package seed

import (
	"context"
	"testing"

	"gas-pipeline-operations-control-service/internal/audit"
	"gas-pipeline-operations-control-service/internal/contract"
	"gas-pipeline-operations-control-service/internal/metering"
	"gas-pipeline-operations-control-service/internal/network"
	"gas-pipeline-operations-control-service/internal/platform"
	"gas-pipeline-operations-control-service/internal/scada"
)

func TestSeedAllCreatesTopology(t *testing.T) {
	c := platform.NewFakeClock(platform.MaxTime)
	auditSvc := audit.NewService(audit.NewStore(1000), c)
	ns := network.NewService(network.NewStore(), c, auditSvc)
	scadaSvc := scada.NewService(scada.NewStore(200), c, 0.5)
	meterSvc := metering.NewService(metering.NewStore(1000), c, auditSvc)
	contractSvc := contract.NewService(contract.NewStore(), c, auditSvc)

	f, err := All(context.Background(), ns, scadaSvc, meterSvc, contractSvc)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if len(f.Segments) != 4 {
		t.Fatalf("segments = %d, want 4", len(f.Segments))
	}
	if len(f.Stations) != 5 {
		t.Fatalf("stations = %d, want 5", len(f.Stations))
	}
	if len(f.Meters) != 2 {
		t.Fatalf("meters = %d, want 2", len(f.Meters))
	}
	if len(f.Points) != 16 {
		t.Fatalf("points = %d, want 16", len(f.Points))
	}
	if f.ContractID == "" {
		t.Fatalf("contract not seeded")
	}
	// contract remaining should equal daily volume (no nomination reserved yet)
	rem, _ := contractSvc.Remaining(context.Background(), f.ContractID)
	if rem != 400000 {
		t.Fatalf("remaining = %g, want 400000", rem)
	}
}
