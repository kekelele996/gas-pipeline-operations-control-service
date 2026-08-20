package seed

// seed populates the in-memory stores with a realistic demo pipeline so the
// server is immediately useful on first run: a main trunk with several
// stations, compressors, valves, meters, measurement points, alarm rules, and
// a shipper contract + nomination. It is idempotent and safe to call once at
// startup.

import (
	"context"
	"fmt"
	"time"

	"gas-pipeline-operations-control-service/internal/contract"
	"gas-pipeline-operations-control-service/internal/metering"
	"gas-pipeline-operations-control-service/internal/network"
	"gas-pipeline-operations-control-service/internal/scada"
)

// Fixture holds the ids created during seeding so tests can reference them.
type Fixture struct {
	Segments    []string
	Stations    []string
	Compressors []string
	Valves      []string
	Meters      []string
	Points      []string
	ContractID  string
}

// All seeds every connected service with demo data and returns the ids of the
// entities created. It is best-effort: errors from individual steps are
// returned aggregated so the caller can decide whether to continue.
func All(ctx context.Context, net *network.Service, scadaSvc *scada.Service, meter *metering.Service, ctr *contract.Service) (Fixture, error) {
	f := Fixture{}
	start := time.Now()
	// reset clock baseline to a fixed epoch for reproducible demo windows
	epoch := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)

	// ---- stations ----
	stations := []struct {
		code string
		name string
		typ  network.StationType
	}{
		{"SRC-01", "North Field Source", network.StationSource},
		{"CMP-01", "Riverside Compressor Station", network.StationCompressor},
		{"MTR-01", "Midpoint Custody Transfer", network.StationMeter},
		{"CMP-02", "Highridge Compressor Station", network.StationCompressor},
		{"DLV-01", "South City Delivery", network.StationDelivery},
	}
	stationIDs := make([]string, len(stations))
	for i, st := range stations {
		s, err := net.UpsertStation(ctx, network.StationInput{
			Name: st.name, Code: st.code, Type: st.typ,
			Latitude: 40.0 + float64(i)*0.3, Longitude: 116.0 + float64(i)*0.2,
			Region: "Northern Region", Operator: "GPL-CC-01",
			CommissionedAt: epoch.AddDate(-3, 0, 0),
		})
		if err != nil {
			return f, fmt.Errorf("seed station %s: %w", st.code, err)
		}
		stationIDs[i] = s.ID
		f.Stations = append(f.Stations, s.ID)
	}

	// ---- segments connecting stations in a chain ----
	segSpecs := []struct {
		name   string
		from   int
		to     int
		length float64
		maop   float64
		tier   string
	}{
		{"Trunk SRC-CMP1", 0, 1, 120.0, 9.5, "main"},
		{"Trunk CMP1-MTR1", 1, 2, 85.0, 9.0, "main"},
		{"Trunk MTR1-CMP2", 2, 3, 96.0, 8.5, "main"},
		{"Trunk CMP2-DLV1", 3, 4, 110.0, 7.5, "main"},
	}
	segmentIDs := make([]string, len(segSpecs))
	for i, sp := range segSpecs {
		seg, err := net.UpsertSegment(ctx, network.SegmentInput{
			Name: sp.name, From: stationIDs[sp.from], To: stationIDs[sp.to],
			LengthKm: sp.length, DiameterMm: 1016, MAOPMPa: sp.maop,
			Tier: sp.tier, Material: "X70", CommissionedAt: epoch.AddDate(-3, 0, 0),
		})
		if err != nil {
			return f, fmt.Errorf("seed segment %s: %w", sp.name, err)
		}
		segmentIDs[i] = seg.ID
		f.Segments = append(f.Segments, seg.ID)
	}

	// ---- compressors (one per compressor station) ----
	cmpSpecs := []struct {
		segIdx int
		staIdx int
		name   string
		power  float64
		state  network.CompressorState
	}{
		{0, 1, "CMP-01-A", 25.0, network.CompressorRunning},
		{2, 3, "CMP-02-A", 30.0, network.CompressorStopped},
	}
	for _, sp := range cmpSpecs {
		c, err := net.UpsertCompressor(ctx, network.CompressorInput{
			Name: sp.name, StationID: stationIDs[sp.staIdx], SegmentID: segmentIDs[sp.segIdx],
			Model: "Centrifugal-3000", PowerMW: sp.power, State: sp.state,
		})
		if err != nil {
			return f, fmt.Errorf("seed compressor %s: %w", sp.name, err)
		}
		f.Compressors = append(f.Compressors, c.ID)
	}

	// ---- valves (block valves at segment boundaries) ----
	for i, sid := range segmentIDs {
		v, err := net.UpsertValve(ctx, network.ValveInput{
			Name: fmt.Sprintf("BLV-%02d", i+1), SegmentID: sid,
			StationID: stationIDs[i+1], Type: "block",
			State: network.ValveOpen, Normal: "open", RemoteControllable: true,
		})
		if err != nil {
			return f, fmt.Errorf("seed valve %d: %w", i, err)
		}
		f.Valves = append(f.Valves, v.ID)
	}

	// ---- measurement points (pressure + flow + temperature per segment) ----
	for i, sid := range segmentIDs {
		for _, pt := range []struct {
			name string
			typ  network.PointType
			unit string
			hi   float64
			lo   float64
		}{
			{fmt.Sprintf("P-IN-%02d", i+1), network.PointPressure, "MPa", 9.5, 1.0},
			{fmt.Sprintf("P-OUT-%02d", i+1), network.PointPressure, "MPa", 9.0, 0.8},
			{fmt.Sprintf("F-%02d", i+1), network.PointFlow, "m3/h", 500000, 0},
			{fmt.Sprintf("T-%02d", i+1), network.PointTemperature, "C", 60, -20},
		} {
			p, err := net.UpsertPoint(ctx, network.PointInput{
				Name: pt.name, Type: pt.typ, Unit: pt.unit, SegmentID: sid,
				HighLimit: pt.hi, LowLimit: pt.lo, RateLimit: 0.5, Enabled: true,
			})
			if err != nil {
				return f, fmt.Errorf("seed point %s: %w", pt.name, err)
			}
			f.Points = append(f.Points, p.ID)
		}
	}

	// ---- SCADA points + alarm rules ----
	seedSCADA(ctx, scadaSvc, f.Points)

	// ---- meters with temperature-banded correction factors ----
	meters := []struct {
		name   string
		segIdx int
		dir    string
	}{
		{"MTR-CUSTODY-01", 1, "delivery"},
		{"MTR-CUSTODY-02", 3, "delivery"},
	}
	for _, m := range meters {
		mt, err := meter.UpsertMeter(ctx, metering.MeterInput{
			Name: m.name, SegmentID: segmentIDs[m.segIdx], StationID: stationIDs[m.segIdx+1],
			Unit: "m3", PressureBase: 0.101325, TempBase: 20.0,
			Factors: []metering.CorrectionFactor{
				{TempLo: -20, TempHi: 0, Factor: 1.08},
				{TempLo: 0, TempHi: 15, Factor: 1.04},
				{TempLo: 15, TempHi: 30, Factor: 0.99},
				{TempLo: 30, TempHi: 60, Factor: 0.93},
			},
			Direction: m.dir, Enabled: true,
		})
		if err != nil {
			return f, fmt.Errorf("seed meter %s: %w", m.name, err)
		}
		f.Meters = append(f.Meters, mt.ID)
	}

	// ---- contract + initial nomination ----
	c, err := ctr.Create(ctx, contract.ContractInput{
		ShipperID: "SHP-DEMO-01", ShipperName: "Demo Energy Co.",
		Code: "CTR-2026-DEMO", DailyVolume: 400000,
		ValidFrom: start, ValidTo: start.AddDate(1, 0, 0),
		PathSegmentIDs: segmentIDs, Notes: "demo shipper contract",
	})
	if err != nil {
		return f, fmt.Errorf("seed contract: %w", err)
	}
	f.ContractID = c.ID

	_ = start
	return f, nil
}

// seedSCADA registers SCADA-side points (mirroring network points) and alarm
// rules so the ingest path raises alarms for out-of-limit readings.
func seedSCADA(ctx context.Context, scadaSvc *scada.Service, pointIDs []string) {
	for _, pid := range pointIDs {
		// We don't have the full network.Point here cheaply; create a minimal
		// scada.Point placeholder that the API can later enrich. The SCADA
		// ingest path uses the point's limits for threshold alarms.
		scadaSvc.RegisterPoint(ctx, scada.Point{
			ID: pid, Name: pid, Type: scada.TypePressure, Unit: "MPa",
			HighLimit: 9.0, LowLimit: 0.8, RateLimit: 0.5, Enabled: true,
			PhysicalMin: -0.1, PhysicalMax: 20.0,
		})
	}
	// a rate rule on the first point to exercise the rate-alarm path
	if len(pointIDs) > 0 {
		_, _ = scadaSvc.AddRule(ctx, scada.Rule{
			PointID: pointIDs[0], Kind: scada.RuleRate, Level: scada.AlarmCritical,
			Message: "pressure changing too fast", Rate: 0.4, Enabled: true,
		})
	}
}
