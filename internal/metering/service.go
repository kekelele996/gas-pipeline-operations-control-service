package metering

// Metering service: records raw readings, applies temperature-banded
// correction to standard volume, rolls up daily totals using delta accounting,
// and produces/advances daily settlement documents.

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
)

// AuditRecorder mirrors the network package's recorder interface.
type AuditRecorder interface {
	Record(ctx context.Context, actor, action, targetType, targetID, detail string) error
}

// Service implements metering business logic.
type Service struct {
	store *Store
	clock platform.Clock
	audit AuditRecorder
}

// NewService builds a metering service.
func NewService(store *Store, clock platform.Clock, audit AuditRecorder) *Service {
	if clock == nil {
		clock = platform.SystemClock{}
	}
	return &Service{store: store, clock: clock, audit: audit}
}

// Store exposes the underlying store for read-only consumers.
func (s *Service) Store() *Store { return s.store }

// ---- Meter management ----

// MeterInput is the payload for creating or updating a meter.
type MeterInput struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	SegmentID    string             `json:"segment_id"`
	StationID    string             `json:"station_id"`
	Unit         string             `json:"unit"`
	PressureBase float64            `json:"pressure_base"`
	TempBase     float64            `json:"temp_base"`
	Factors      []CorrectionFactor `json:"factors"`
	Direction    string             `json:"direction"`
	CustomerID   string             `json:"customer_id"`
	Enabled      bool               `json:"enabled"`
}

// UpsertMeter validates and stores a meter.
func (s *Service) UpsertMeter(ctx context.Context, in MeterInput) (Meter, error) {
	v := platform.NewValidate()
	v.RequireNonEmpty("name", in.Name)
	v.RequireNonEmpty("segment_id", in.SegmentID)
	v.RequireNonEmpty("unit", in.Unit)
	v.RequireEnum("direction", in.Direction, []string{"receipt", "delivery"})
	if in.PressureBase <= 0 {
		v.Require(false, "pressure_base must be positive")
	}
	for i, f := range in.Factors {
		if f.Factor <= 0 {
			v.Require(false, fmt.Sprintf("factors[%d].factor must be positive", i))
		}
		if f.TempHi <= f.TempLo {
			v.Require(false, fmt.Sprintf("factors[%d].temp_hi must exceed temp_lo", i))
		}
	}
	if err := v.Error(); err != nil {
		return Meter{}, err
	}
	id := in.ID
	if id == "" {
		id = platform.NewMeterID()
	}
	m := Meter{
		ID: id, Name: in.Name, SegmentID: in.SegmentID, StationID: in.StationID,
		Unit: in.Unit, PressureBase: in.PressureBase, TempBase: in.TempBase,
		Factors: in.Factors, Direction: in.Direction, CustomerID: in.CustomerID,
		Enabled: in.Enabled,
	}
	s.store.PutMeter(m)
	out, _ := s.store.Meter(id)
	if s.audit != nil {
		_ = s.audit.Record(ctx, "system", "upsert_meter", "meter", id, in.Name)
	}
	return out, nil
}

// ListMeters returns all meters.
func (s *Service) ListMeters(ctx context.Context) []Meter {
	return s.store.Meters()
}

// GetMeter returns a single meter by id.
func (s *Service) GetMeter(ctx context.Context, id string) (Meter, error) {
	m, ok := s.store.Meter(id)
	if !ok {
		return Meter{}, platform.NotFoundf("meter %q not found", id)
	}
	return m, nil
}

// ---- Correction ----

// Correct applies the meter's temperature-banded correction factor to a raw
// volume. If no band matches the temperature, the factor defaults to 1.0
// (uncorrected) and the second return value is false.
func Correct(m Meter, temperature float64) (float64, bool) {
	for _, f := range m.Factors {
		if temperature >= f.TempLo && temperature < f.TempHi {
			return f.Factor, true
		}
	}
	// bands are half-open; handle the upper bound of the last band
	if n := len(m.Factors); n > 0 {
		last := m.Factors[n-1]
		if temperature >= last.TempLo && temperature <= last.TempHi {
			return last.Factor, true
		}
	}
	return 1.0, false
}

// ---- Reading ingestion ----

// RecordReading records a raw reading, computes the corrected standard volume,
// and rolls up the daily total using delta accounting: the daily total is the
// sum of (reading[i] - reading[i-1]) over the day, which supports totalizer
// style meters. The first reading of a day sets the base.
func (s *Service) RecordReading(ctx context.Context, rd RawReading) (DailyTotal, error) {
	if math.IsNaN(rd.Value) || math.IsInf(rd.Value, 0) {
		return DailyTotal{}, platform.Invalidf("bad reading value")
	}
	m, ok := s.store.Meter(rd.MeterID)
	if !ok {
		return DailyTotal{}, platform.NotFoundf("meter %q not found", rd.MeterID)
	}
	if !m.Enabled {
		return DailyTotal{}, platform.Invalidf("meter %q disabled", rd.MeterID)
	}
	if rd.Ts.IsZero() {
		rd.Ts = s.clock.Now()
	}
	factor, _ := Correct(m, rd.Temperature)
	std := rd.Value * factor
	date := dateOf(rd.Ts)

	var out DailyTotal
	out = s.store.UpsertDaily(rd.MeterID, date, func(t *DailyTotal) {
		if t.ReadingCount == 0 {
			t.FirstTs = rd.Ts
			t.LastTs = rd.Ts
			t.LastRaw = rd.Value
			t.LastStd = std
			t.RawVolume = 0
			t.StdVolume = 0
			t.ReadingCount = 1
			return
		}
		// delta accounting on the raw totalizer
		dRaw := rd.Value - t.LastRaw
		if dRaw < 0 {
			// totalizer rolled over or was reset — treat the new value as the
			// delta baseline rather than crediting a large jump.
			dRaw = 0
		}
		t.RawVolume += dRaw
		t.StdVolume += dRaw * factor
		t.LastRaw = rd.Value
		t.LastStd = std
		t.LastTs = rd.Ts
		t.ReadingCount++
	})
	s.store.AppendReading(rd)
	return out, nil
}

// GetDailyTotal returns the daily total for a meter/date.
func (s *Service) GetDailyTotal(ctx context.Context, meterID string, date string) (DailyTotal, error) {
	t, ok := s.store.DailyTotal(meterID, date)
	if !ok {
		return DailyTotal{}, platform.NotFoundf("no daily total for meter %q on %s", meterID, date)
	}
	return t, nil
}

// DailyForDate returns all meters' totals for a date.
func (s *Service) DailyForDate(ctx context.Context, date string) []DailyTotal {
	return s.store.AllDailyForDate(date)
}

// ---- Settlement ----

// SettleDaily produces or refreshes the settlement document for a date by
// aggregating every meter's daily total. The document starts in draft.
func (s *Service) SettleDaily(ctx context.Context, date string) (Settlement, error) {
	if date == "" {
		return Settlement{}, platform.Invalidf("date must not be empty")
	}
	totals := s.store.AllDailyForDate(date)
	items := make([]LineItem, 0, len(totals))
	var totalRaw, totalStd float64
	for _, t := range totals {
		m, _ := s.store.Meter(t.MeterID)
		name := t.MeterID
		if m.ID != "" {
			name = m.Name
		}
		items = append(items, LineItem{
			MeterID: t.MeterID, Name: name,
			RawVolume: t.RawVolume, StdVolume: t.StdVolume,
			ReadingCount: t.ReadingCount,
		})
		totalRaw += t.RawVolume
		totalStd += t.StdVolume
	}
	sort.Slice(items, func(i, j int) bool { return items[i].MeterID < items[j].MeterID })
	st := Settlement{
		ID: platform.NewSettlementID(), Date: date, State: SettlementDraft,
		Items: items, TotalRaw: totalRaw, TotalStd: totalStd,
	}
	s.store.PutSettlement(st)
	out, _ := s.store.Settlement(date)
	if s.audit != nil {
		_ = s.audit.Record(ctx, "system", "settle_daily", "settlement", out.ID,
			fmt.Sprintf("date=%s meters=%d total_std=%.4g", date, len(items), totalStd))
	}
	return out, nil
}

// Confirm advances a draft settlement to confirmed.
func (s *Service) Confirm(ctx context.Context, date, by string) (Settlement, error) {
	st, ok := s.store.Settlement(date)
	if !ok {
		return Settlement{}, platform.NotFoundf("settlement for %s not found", date)
	}
	next, err := MustTransition(st.State, SettlementConfirmed)
	if err != nil {
		return st, err
	}
	out, _ := s.store.UpdateSettlement(date, func(x *Settlement) {
		x.State = next
		x.ConfirmedBy = by
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, by, "confirm_settlement", "settlement", out.ID, date)
	}
	return out, nil
}

// Reconcile advances a confirmed settlement to reconciled (final).
func (s *Service) Reconcile(ctx context.Context, date, by string) (Settlement, error) {
	st, ok := s.store.Settlement(date)
	if !ok {
		return Settlement{}, platform.NotFoundf("settlement for %s not found", date)
	}
	next, err := MustTransition(st.State, SettlementReconciled)
	if err != nil {
		return st, err
	}
	out, _ := s.store.UpdateSettlement(date, func(x *Settlement) {
		x.State = next
		x.ReconciledBy = by
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, by, "reconcile_settlement", "settlement", out.ID, date)
	}
	return out, nil
}

// GetSettlement returns the settlement for a date.
func (s *Service) GetSettlement(ctx context.Context, date string) (Settlement, error) {
	st, ok := s.store.Settlement(date)
	if !ok {
		return Settlement{}, platform.NotFoundf("settlement for %s not found", date)
	}
	return st, nil
}

// ListSettlements returns all settlements.
func (s *Service) ListSettlements(ctx context.Context) []Settlement {
	return s.store.Settlements()
}

// Now returns the service clock's current time; for tests.
func (s *Service) Now() time.Time { return s.clock.Now() }
