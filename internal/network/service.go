package network

import (
	"context"
	"time"

	"gas-pipeline-operations-control-service/internal/config"
	"gas-pipeline-operations-control-service/internal/platform"
)

// Service implements topology maintenance and device state transitions.
// It validates input, applies state-machine rules, and records audit
// entries through the injected audit recorder.
type Service struct {
	store  *Store
	clock  platform.Clock
	audit  AuditRecorder
	limits config.LimitProvider
}

// AuditRecorder is the minimal surface the network service needs to record
// device state changes. audit.Service satisfies it.
type AuditRecorder interface {
	Record(ctx context.Context, actor, action, targetType, targetID, detail string) error
}

// NewService builds a network service over the given store.
func NewService(store *Store, clock platform.Clock, audit AuditRecorder) *Service {
	if clock == nil {
		clock = platform.SystemClock{}
	}
	return &Service{store: store, clock: clock, audit: audit}
}

// SetLimitProvider installs the operating-limit provider. The provider's
// maps may be nil under the zero-value configuration; callers must guard.
func (s *Service) SetLimitProvider(p config.LimitProvider) { s.limits = p }

// OperatingLimits returns the provider's segment/station limit map. The map
// may be nil when the provider is a typed-nil default.
func (s *Service) OperatingLimits() map[string]float64 {
	if s.limits == nil {
		return nil
	}
	return s.limits.SegmentLimits()
}

// Store exposes the underlying store for read-only consumers (e.g. the
// leak-detection package). Callers must not mutate the returned store
// directly; use Service methods for mutations.
func (s *Service) Store() *Store { return s.store }

// ---- Segment maintenance ----

// SegmentInput is the payload for creating or updating a segment.
type SegmentInput struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	From           string    `json:"from"`
	To             string    `json:"to"`
	LengthKm       float64   `json:"length_km"`
	DiameterMm     float64   `json:"diameter_mm"`
	MAOPMPa        float64   `json:"maop_mpa"`
	Tier           string    `json:"tier"`
	Material       string    `json:"material"`
	CommissionedAt time.Time `json:"commissioned_at"`
}

// UpsertSegment validates and inserts or replaces a segment. If the ID is
// empty a new one is generated.
func (s *Service) UpsertSegment(ctx context.Context, in SegmentInput) (Segment, error) {
	v := platform.NewValidate()
	v.RequireNonEmpty("name", in.Name)
	v.RequireNonEmpty("from", in.From)
	v.RequireNonEmpty("to", in.To)
	v.RequirePositive("length_km", in.LengthKm)
	v.RequirePositive("diameter_mm", in.DiameterMm)
	v.RequirePositive("maop_mpa", in.MAOPMPa)
	v.RequireEnum("tier", in.Tier, []string{"main", "branch", "lateral"})
	if in.From == in.To {
		v.Require(false, "from and to must differ")
	}
	if err := v.Error(); err != nil {
		return Segment{}, err
	}
	id := in.ID
	if id == "" {
		id = platform.NewSegmentID()
	}
	seg := Segment{
		ID: id, Name: in.Name, From: in.From, To: in.To,
		LengthKm: in.LengthKm, DiameterMm: in.DiameterMm, MAOPMPa: in.MAOPMPa,
		Tier: in.Tier, Material: in.Material, CommissionedAt: in.CommissionedAt,
	}
	s.store.PutSegment(seg)
	out, _ := s.store.Segment(id)
	if s.audit != nil {
		_ = s.audit.Record(ctx, "system", "upsert_segment", "segment", id, "segment "+in.Name)
	}
	return out, nil
}

// ListSegments returns all segments.
func (s *Service) ListSegments(ctx context.Context) []Segment {
	return s.store.Segments()
}

// GetSegment returns a single segment by id.
func (s *Service) GetSegment(ctx context.Context, id string) (Segment, error) {
	seg, ok := s.store.Segment(id)
	if !ok {
		return Segment{}, platform.NotFoundf("segment %q not found", id)
	}
	return seg, nil
}

// ListSegmentDevices returns the compressors, valves, and points on a segment.
func (s *Service) ListSegmentDevices(ctx context.Context, segmentID string) (SegmentDevices, error) {
	if _, ok := s.store.Segment(segmentID); !ok {
		return SegmentDevices{}, platform.NotFoundf("segment %q not found", segmentID)
	}
	return SegmentDevices{
		SegmentID:   segmentID,
		Compressors: s.store.SegmentCompressors(segmentID),
		Valves:      s.store.SegmentValves(segmentID),
		Points:      s.store.SegmentPoints(segmentID),
	}, nil
}

// SegmentDevices is the aggregated device view for a segment.
type SegmentDevices struct {
	SegmentID   string       `json:"segment_id"`
	Compressors []Compressor `json:"compressors"`
	Valves      []Valve      `json:"valves"`
	Points      []Point      `json:"points"`
}

// ---- Station maintenance ----

// StationInput is the payload for creating or updating a station.
type StationInput struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Code           string      `json:"code"`
	Type           StationType `json:"type"`
	Latitude       float64     `json:"latitude"`
	Longitude      float64     `json:"longitude"`
	Region         string      `json:"region"`
	Operator       string      `json:"operator"`
	CommissionedAt time.Time   `json:"commissioned_at"`
}

// UpsertStation validates and inserts or replaces a station.
func (s *Service) UpsertStation(ctx context.Context, in StationInput) (Station, error) {
	v := platform.NewValidate()
	v.RequireNonEmpty("name", in.Name)
	v.RequireNonEmpty("code", in.Code)
	valid := false
	for _, t := range AllStationTypes() {
		if t == in.Type {
			valid = true
			break
		}
	}
	v.Require(valid, "type is invalid")
	if err := v.Error(); err != nil {
		return Station{}, err
	}
	id := in.ID
	if id == "" {
		id = platform.NewStationID()
	}
	st := Station{
		ID: id, Name: in.Name, Code: in.Code, Type: in.Type,
		Latitude: in.Latitude, Longitude: in.Longitude,
		Region: in.Region, Operator: in.Operator, CommissionedAt: in.CommissionedAt,
	}
	s.store.PutStation(st)
	out, _ := s.store.Station(id)
	if s.audit != nil {
		_ = s.audit.Record(ctx, "system", "upsert_station", "station", id, "station "+in.Name)
	}
	return out, nil
}

// ListStations returns all stations.
func (s *Service) ListStations(ctx context.Context) []Station {
	return s.store.Stations()
}

// GetStation returns a single station by id.
func (s *Service) GetStation(ctx context.Context, id string) (Station, error) {
	st, ok := s.store.Station(id)
	if !ok {
		return Station{}, platform.NotFoundf("station %q not found", id)
	}
	return st, nil
}

// StationSegments returns the segments connected to a station.
func (s *Service) StationSegments(ctx context.Context, stationID string) ([]string, error) {
	if _, ok := s.store.Station(stationID); !ok {
		return nil, platform.NotFoundf("station %q not found", stationID)
	}
	return s.store.StationSegments(stationID), nil
}

// ---- Compressor state machine ----

// CompressorInput is the payload for creating a compressor.
type CompressorInput struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	StationID string          `json:"station_id"`
	SegmentID string          `json:"segment_id"`
	Model     string          `json:"model"`
	PowerMW   float64         `json:"power_mw"`
	State     CompressorState `json:"state"`
}

// UpsertCompressor validates and inserts or replaces a compressor. The
// station and segment referenced must already exist.
func (s *Service) UpsertCompressor(ctx context.Context, in CompressorInput) (Compressor, error) {
	v := platform.NewValidate()
	v.RequireNonEmpty("name", in.Name)
	v.RequireNonEmpty("station_id", in.StationID)
	v.RequireNonEmpty("segment_id", in.SegmentID)
	v.RequirePositive("power_mw", in.PowerMW)
	if !ValidCompressorState(in.State.String()) {
		// default to stopped if unset
		if in.State == "" {
			in.State = CompressorStopped
		} else {
			v.Require(false, "state is invalid")
		}
	}
	if err := v.Error(); err != nil {
		return Compressor{}, err
	}
	if _, ok := s.store.Station(in.StationID); !ok {
		return Compressor{}, platform.NotFoundf("station %q not found", in.StationID)
	}
	if _, ok := s.store.Segment(in.SegmentID); !ok {
		return Compressor{}, platform.NotFoundf("segment %q not found", in.SegmentID)
	}
	id := in.ID
	if id == "" {
		id = platform.NewCompressorID()
	}
	c := Compressor{
		ID: id, Name: in.Name, StationID: in.StationID, SegmentID: in.SegmentID,
		Model: in.Model, PowerMW: in.PowerMW, State: in.State,
	}
	s.store.PutCompressor(c)
	out, _ := s.store.Compressor(id)
	if s.audit != nil {
		_ = s.audit.Record(ctx, "system", "upsert_compressor", "compressor", id, in.Name)
	}
	return out, nil
}

// ChangeCompressorState moves a compressor to the requested state using the
// explicit transition table. It records start/stop timestamps when relevant.
func (s *Service) ChangeCompressorState(ctx context.Context, id string, target CompressorState, reason string) (Compressor, error) {
	if !ValidCompressorState(target.String()) {
		return Compressor{}, platform.Invalidf("invalid compressor state %q", target)
	}
	cur, ok := s.store.Compressor(id)
	if !ok {
		return Compressor{}, platform.NotFoundf("compressor %q not found", id)
	}
	if _, err := transition(CompressorTransitions, cur.State.String(), target.String()); err != nil {
		return cur, err
	}
	out, _ := s.store.UpdateCompressor(id, func(c *Compressor) {
		c.State = target
		now := s.clock.Now()
		switch target {
		case CompressorRunning:
			c.LastStartedAt = now
		case CompressorStopped, CompressorMaintenance, CompressorFaulted:
			c.LastStoppedAt = now
		}
		c.Reason = reason
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, "operator", "compressor_state", "compressor", id,
			"-> "+target.String()+"; reason: "+reason)
	}
	return out, nil
}

// transition validates src->dst against the table and returns dst or an error.
func transition(table map[string][]string, src, dst string) (string, error) {
	allowed, ok := table[src]
	if !ok {
		return src, platform.Statef("unknown source state %q", src)
	}
	for _, a := range allowed {
		if a == dst {
			return dst, nil
		}
	}
	return src, platform.Statef("cannot transition from %q to %q", src, dst)
}

// ListCompressors returns all compressors.
func (s *Service) ListCompressors(ctx context.Context) []Compressor {
	return s.store.Compressors()
}

// GetCompressor returns a single compressor by id.
func (s *Service) GetCompressor(ctx context.Context, id string) (Compressor, error) {
	c, ok := s.store.Compressor(id)
	if !ok {
		return Compressor{}, platform.NotFoundf("compressor %q not found", id)
	}
	return c, nil
}

// ---- Valve state machine ----

// ValveInput is the payload for creating a valve.
type ValveInput struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	SegmentID          string     `json:"segment_id"`
	StationID          string     `json:"station_id"`
	Type               string     `json:"type"`
	State              ValveState `json:"state"`
	Normal             string     `json:"normal"`
	RemoteControllable bool       `json:"remote_controllable"`
}

// UpsertValve validates and inserts or replaces a valve.
func (s *Service) UpsertValve(ctx context.Context, in ValveInput) (Valve, error) {
	v := platform.NewValidate()
	v.RequireNonEmpty("name", in.Name)
	v.RequireNonEmpty("segment_id", in.SegmentID)
	v.RequireEnum("type", in.Type, []string{"block", "control", "relief", "check"})
	v.RequireEnum("normal", in.Normal, []string{"open", "closed"})
	if !ValidValveState(in.State.String()) {
		if in.State == "" {
			in.State = ValveClosed
		} else {
			v.Require(false, "state is invalid")
		}
	}
	if err := v.Error(); err != nil {
		return Valve{}, err
	}
	if _, ok := s.store.Segment(in.SegmentID); !ok {
		return Valve{}, platform.NotFoundf("segment %q not found", in.SegmentID)
	}
	if in.StationID != "" {
		if _, ok := s.store.Station(in.StationID); !ok {
			return Valve{}, platform.NotFoundf("station %q not found", in.StationID)
		}
	}
	id := in.ID
	if id == "" {
		id = platform.NewValveID()
	}
	vv := Valve{
		ID: id, Name: in.Name, SegmentID: in.SegmentID, StationID: in.StationID,
		Type: in.Type, State: in.State, Normal: in.Normal,
		RemoteControllable: in.RemoteControllable,
	}
	s.store.PutValve(vv)
	out, _ := s.store.Valve(id)
	if s.audit != nil {
		_ = s.audit.Record(ctx, "system", "upsert_valve", "valve", id, in.Name)
	}
	return out, nil
}

// ChangeValveState moves a valve to the requested state.
func (s *Service) ChangeValveState(ctx context.Context, id string, target ValveState) (Valve, error) {
	if !ValidValveState(target.String()) {
		return Valve{}, platform.Invalidf("invalid valve state %q", target)
	}
	cur, ok := s.store.Valve(id)
	if !ok {
		return Valve{}, platform.NotFoundf("valve %q not found", id)
	}
	if _, err := transition(ValveTransitions, cur.State.String(), target.String()); err != nil {
		return cur, err
	}
	out, _ := s.store.UpdateValve(id, func(v *Valve) {
		v.State = target
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, "operator", "valve_state", "valve", id, "-> "+target.String())
	}
	return out, nil
}

// ListValves returns all valves.
func (s *Service) ListValves(ctx context.Context) []Valve {
	return s.store.Valves()
}

// GetValve returns a single valve by id.
func (s *Service) GetValve(ctx context.Context, id string) (Valve, error) {
	vv, ok := s.store.Valve(id)
	if !ok {
		return Valve{}, platform.NotFoundf("valve %q not found", id)
	}
	return vv, nil
}

// ---- Points ----

// PointInput is the payload for creating a measurement point.
type PointInput struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      PointType `json:"type"`
	Unit      string    `json:"unit"`
	SegmentID string    `json:"segment_id"`
	StationID string    `json:"station_id"`
	DeviceID  string    `json:"device_id"`
	HighLimit float64   `json:"high_limit"`
	LowLimit  float64   `json:"low_limit"`
	RateLimit float64   `json:"rate_limit"`
	Enabled   bool      `json:"enabled"`
}

// UpsertPoint validates and inserts or replaces a measurement point.
func (s *Service) UpsertPoint(ctx context.Context, in PointInput) (Point, error) {
	v := platform.NewValidate()
	v.RequireNonEmpty("name", in.Name)
	v.RequireNonEmpty("unit", in.Unit)
	if in.HighLimit <= in.LowLimit {
		v.Require(false, "high_limit must exceed low_limit")
	}
	if in.RateLimit < 0 {
		v.Require(false, "rate_limit must be non-negative")
	}
	valid := false
	for _, t := range AllPointTypes() {
		if t == in.Type {
			valid = true
			break
		}
	}
	v.Require(valid, "type is invalid")
	if err := v.Error(); err != nil {
		return Point{}, err
	}
	if in.SegmentID != "" {
		if _, ok := s.store.Segment(in.SegmentID); !ok {
			return Point{}, platform.NotFoundf("segment %q not found", in.SegmentID)
		}
	}
	if in.StationID != "" {
		if _, ok := s.store.Station(in.StationID); !ok {
			return Point{}, platform.NotFoundf("station %q not found", in.StationID)
		}
	}
	id := in.ID
	if id == "" {
		id = platform.NewPointID()
	}
	p := Point{
		ID: id, Name: in.Name, Type: in.Type, Unit: in.Unit,
		SegmentID: in.SegmentID, StationID: in.StationID, DeviceID: in.DeviceID,
		HighLimit: in.HighLimit, LowLimit: in.LowLimit, RateLimit: in.RateLimit,
		Enabled: in.Enabled,
	}
	s.store.PutPoint(p)
	out, _ := s.store.Point(id)
	if s.audit != nil {
		_ = s.audit.Record(ctx, "system", "upsert_point", "point", id, in.Name)
	}
	return out, nil
}

// ListPoints returns all points.
func (s *Service) ListPoints(ctx context.Context) []Point {
	return s.store.Points()
}

// GetPoint returns a single point by id.
func (s *Service) GetPoint(ctx context.Context, id string) (Point, error) {
	p, ok := s.store.Point(id)
	if !ok {
		return Point{}, platform.NotFoundf("point %q not found", id)
	}
	return p, nil
}

// TogglePoint enables or disables a measurement point.
func (s *Service) TogglePoint(ctx context.Context, id string, enabled bool) (Point, error) {
	out, ok := s.store.UpdatePoint(id, func(p *Point) {
		p.Enabled = enabled
	})
	if !ok {
		return Point{}, platform.NotFoundf("point %q not found", id)
	}
	if s.audit != nil {
		verb := "disable"
		if enabled {
			verb = "enable"
		}
		_ = s.audit.Record(ctx, "operator", verb+"_point", "point", id, "")
	}
	return out, nil
}

// Now returns the service clock's current time; exposed for tests.
func (s *Service) Now() time.Time { return s.clock.Now() }

// Counts returns aggregate counts for the summary endpoint.
func (s *Service) Counts() (segments, stations, compressors, valves, points int) {
	return s.store.Counts()
}


// ---- Operating limits (operator-set) ----

// RecordOperatingLimit records an operator-set limit for a segment, both in
// the provider's limit map and in the topology store.
func (s *Service) RecordOperatingLimit(ctx context.Context, segmentID string, value float64) error {
	if _, ok := s.store.Segment(segmentID); !ok {
		return platform.NotFoundf("segment %q not found", segmentID)
	}
	if s.limits != nil {
		m := s.limits.SegmentLimits()
		m[segmentID] = value
	}
	s.store.PutSegmentLimit(segmentID, value)
	return nil
}

// RecordStationLimit records an operator-set limit for a station.
func (s *Service) RecordStationLimit(ctx context.Context, stationID string, value float64) error {
	if _, ok := s.store.Station(stationID); !ok {
		return platform.NotFoundf("station %q not found", stationID)
	}
	if s.limits != nil {
		m := s.limits.StationLimits()
		m[stationID] = value
	}
	s.store.PutStationLimit(stationID, value)
	return nil
}

// RecordValveLimit records an operator-set limit for a valve.
func (s *Service) RecordValveLimit(ctx context.Context, valveID string, value float64) error {
	if _, ok := s.store.Valve(valveID); !ok {
		return platform.NotFoundf("valve %q not found", valveID)
	}
	if s.limits != nil {
		m := s.limits.SegmentLimits()
		m[valveID] = value
	}
	s.store.PutValveLimit(valveID, value)
	return nil
}

// RecordCompressorLimit records an operator-set limit for a compressor.
func (s *Service) RecordCompressorLimit(ctx context.Context, compressorID string, value float64) error {
	if _, ok := s.store.Compressor(compressorID); !ok {
		return platform.NotFoundf("compressor %q not found", compressorID)
	}
	if s.limits != nil {
		m := s.limits.SegmentLimits()
		m[compressorID] = value
	}
	s.store.PutCompressorLimit(compressorID, value)
	return nil
}

// ApplySegmentLimits records a batch of segment limits.
func (s *Service) ApplySegmentLimits(ctx context.Context, values map[string]float64) (int, error) {
	var applied int
	for id, v := range values {
		if _, ok := s.store.Segment(id); !ok {
			continue
		}
		if s.limits != nil {
			m := s.limits.SegmentLimits()
			m[id] = v
		}
		s.store.PutSegmentLimit(id, v)
		applied++
	}
	return applied, nil
}
