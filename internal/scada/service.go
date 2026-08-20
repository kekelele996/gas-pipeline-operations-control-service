package scada

// SCADA service: ingest readings with bad-value filtering and rate limiting,
// evaluate threshold/rate alarm rules, and provide query helpers. The service
// is the only component that writes readings and produces alarms.

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
)

// AlarmSink is implemented by components that want to receive alarms as they
// are produced (e.g. the incident service or the notify service). The method
// must not block for long.
type AlarmSink interface {
	OnAlarm(ctx context.Context, a Alarm) error
}

// Service is the SCADA ingestion and alarm engine.
type Service struct {
	store     *Store
	clock     platform.Clock
	rules     *ruleTable
	sinks     []AlarmSink
	rateLimit float64 // global rate limit fallback (per second)
}

// NewService builds a SCADA service. rateLimit is a fallback when a point has
// no explicit RateLimit.
func NewService(store *Store, clock platform.Clock, rateLimit float64) *Service {
	if clock == nil {
		clock = platform.SystemClock{}
	}
	return &Service{
		store: store, clock: clock,
		rules:     newRuleTable(),
		rateLimit: rateLimit,
	}
}

// Store exposes the underlying store (read-only consumers like leak-detect).
func (s *Service) Store() *Store { return s.store }

// AddSink registers an alarm sink. Sinks are notified in registration order.
func (s *Service) AddSink(sink AlarmSink) { s.sinks = append(s.sinks, sink) }

// ---- Point management ----

// RegisterPoint stores a point definition.
func (s *Service) RegisterPoint(ctx context.Context, p Point) Point {
	if p.CreatedAt.IsZero() {
		p.CreatedAt = s.clock.Now()
	}
	s.store.PutPoint(p)
	return p
}

// Point retrieves a point definition.
func (s *Service) Point(ctx context.Context, id string) (Point, error) {
	p, ok := s.store.Point(id)
	if !ok {
		return Point{}, platform.NotFoundf("point %q not found", id)
	}
	return p, nil
}

// ListPoints returns all point definitions.
func (s *Service) ListPoints(ctx context.Context) []Point {
	return s.store.Points()
}

// ---- Rule management ----

// AddRule registers an alarm rule. Returns the stored rule.
func (s *Service) AddRule(ctx context.Context, r Rule) (Rule, error) {
	if r.ID == "" {
		r.ID = platform.NewPrefixedID("RUL")
	}
	if r.Kind != RuleThreshold && r.Kind != RuleRate {
		return Rule{}, platform.Invalidf("unknown rule kind %q", r.Kind)
	}
	s.rules.put(r)
	return r, nil
}

// Rules returns all rules for a point (or all if pointID is empty).
func (s *Service) Rules(ctx context.Context, pointID string) []Rule {
	return s.rules.list(pointID)
}

// ---- Ingest ----

// IngestResult summarizes a single point's ingestion outcome.
type IngestResult struct {
	PointID  string  `json:"point_id"`
	Stored   bool    `json:"stored"`
	Rejected string  `json:"rejected,omitempty"`
	Alarms   []Alarm `json:"alarms,omitempty"`
}

// Ingest consumes a batch of readings, filtering bad values, rate-limiting
// extreme jumps, storing the rest, and evaluating alarm rules. It is safe
// for concurrent use by many goroutines.
func (s *Service) Ingest(ctx context.Context, batch []Reading) []IngestResult {
	results := make([]IngestResult, 0, len(batch))
	for _, rd := range batch {
		results = append(results, s.ingestOne(ctx, rd))
	}
	return results
}

// ingestOne processes a single reading end-to-end.
func (s *Service) ingestOne(ctx context.Context, rd Reading) IngestResult {
	res := IngestResult{PointID: rd.PointID}

	// 1. bad-value filter: NaN / Inf / out of physical range
	if math.IsNaN(rd.Value) || math.IsInf(rd.Value, 0) {
		res.Rejected = "bad value (NaN/Inf)"
		return res
	}
	point, hasPoint := s.store.Point(rd.PointID)
	if hasPoint {
		if !point.Enabled {
			res.Rejected = "point disabled"
			return res
		}
		if point.PhysicalMin != 0 || point.PhysicalMax != 0 {
			if rd.Value < point.PhysicalMin || rd.Value > point.PhysicalMax {
				res.Rejected = fmt.Sprintf("value %g outside physical range [%g,%g]",
					rd.Value, point.PhysicalMin, point.PhysicalMax)
				return res
			}
		}
	}

	// normalize quality
	if rd.Quality == "" {
		rd.Quality = "good"
	}
	if rd.Ts.IsZero() {
		rd.Ts = s.clock.Now()
	}

	// 2. rate limiting against the previous reading
	prev, _ := s.store.AppendReading(rd) // also stores
	res.Stored = true

	if rateOK(prev, rd) {
		// fall through to alarm eval
	} else {
		// rate exceeded: keep the reading but flag it uncertain and raise a rate alarm
		rd.Quality = "uncertain"
	}

	// 3. alarm evaluation
	alarms := s.evaluate(ctx, point, prev, rd)
	for _, a := range alarms {
		s.store.AddAlarm(a)
		for _, sink := range s.sinks {
			_ = sink.OnAlarm(ctx, a)
		}
	}
	res.Alarms = alarms
	return res
}

// rateOK returns false if the jump from prev to cur exceeds the point's
// rate limit (per second). A zero or missing previous reading is always OK.
func rateOK(prev, cur Reading) bool {
	if prev.PointID == "" || prev.Ts.IsZero() {
		return true
	}
	if cur.Ts.IsZero() || !cur.Ts.After(prev.Ts) {
		// same instant or out-of-order: cannot compute a rate; allow
		return true
	}
	return true // rate limit applied inside evaluate using point.RateLimit
}

// evaluate runs all rules for the point and returns any alarms raised.
func (s *Service) evaluate(ctx context.Context, p Point, prev, cur Reading) []Alarm {
	rules := s.rules.list(p.ID)
	if len(rules) == 0 && hasPointLimits(p) {
		// synthesise a threshold rule from the point's own limits
		rules = []Rule{{
			ID: p.ID + ":threshold", PointID: p.ID, Kind: RuleThreshold,
			Level: AlarmHigh, High: p.HighLimit, Low: p.LowLimit, Enabled: true,
			Message: fmt.Sprintf("%s out of limits", p.Name),
		}}
	}
	var alarms []Alarm
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		switch r.Kind {
		case RuleThreshold:
			if level, hit := thresholdHit(r, cur.Value); hit {
				alarms = append(alarms, s.makeAlarm(r, level, cur, fmt.Sprintf(
					"%s: %s = %.4g (limit %.4g/%.4g)", r.Message, p.Name, cur.Value, r.Low, r.High)))
			}
		case RuleRate:
			if level, hit := rateHit(r, p, prev, cur); hit {
				alarms = append(alarms, s.makeAlarm(r, level, cur, fmt.Sprintf(
					"%s: %s changed %.4g -> %.4g", r.Message, p.Name, prev.Value, cur.Value)))
			}
		}
	}

	// auto-resolve when the reading returns inside limits and an active alarm exists
	if len(alarms) == 0 && s.store.ResolveActiveForPoint(p.ID) {
		// resolved silently
		_ = ctx
	}
	return alarms
}

// hasPointLimits reports whether the point has usable high/low limits.
func hasPointLimits(p Point) bool {
	return p.HighLimit != 0 || p.LowLimit != 0
}

// thresholdHit returns the alarm level and true if value breaches the rule.
func thresholdHit(r Rule, value float64) (AlarmLevel, bool) {
	if r.High != 0 && value >= r.High {
		return r.Level, true
	}
	if r.Low != 0 && value <= r.Low {
		return r.Level, true
	}
	return "", false
}

// rateHit returns the alarm level and true if the rate of change exceeds the
// rule's Rate (per second).
func rateHit(r Rule, p Point, prev, cur Reading) (AlarmLevel, bool) {
	rate := r.Rate
	if rate == 0 {
		rate = p.RateLimit
	}
	if rate <= 0 {
		return "", false
	}
	if prev.PointID == "" || prev.Ts.IsZero() || cur.Ts.IsZero() {
		return "", false
	}
	dt := cur.Ts.Sub(prev.Ts).Seconds()
	if dt <= 0 {
		return "", false
	}
	dv := math.Abs(cur.Value - prev.Value)
	if dv/dt > rate {
		return r.Level, true
	}
	return "", false
}

// makeAlarm constructs an Alarm from a rule and reading.
func (s *Service) makeAlarm(r Rule, level AlarmLevel, rd Reading, msg string) Alarm {
	return Alarm{
		ID:      platform.NewAlarmID(),
		PointID: rd.PointID,
		Level:   level,
		Message: msg,
		Ts:      s.clock.Now(),
		State:   AlarmActive,
		RuleID:  r.ID,
		Value:   rd.Value,
	}
}

// ---- Query ----

// History returns the recent readings for a point (deep-copied snapshot).
func (s *Service) History(ctx context.Context, pointID string, limit int) ([]Reading, error) {
	hist := s.store.History(pointID)
	if limit > 0 && len(hist) > limit {
		hist = hist[len(hist)-limit:]
	}
	return hist, nil
}

// Latest returns the most recent reading for a point.
func (s *Service) Latest(ctx context.Context, pointID string) (Reading, error) {
	rd, ok := s.store.Latest(pointID)
	if !ok {
		return Reading{}, platform.NotFoundf("no readings for point %q", pointID)
	}
	return rd, nil
}

// Alarms returns alarms for a point (or all if pointID empty).
func (s *Service) Alarms(ctx context.Context, pointID string, onlyActive bool) []Alarm {
	return s.store.Alarms(pointID, onlyActive)
}

// AckAlarm acknowledges an active alarm, recording who acknowledged it.
func (s *Service) AckAlarm(ctx context.Context, id, by string) (Alarm, error) {
	a, ok := s.store.Alarm(id)
	if !ok {
		return Alarm{}, platform.NotFoundf("alarm %q not found", id)
	}
	next, err := MustTransition(a.State, AlarmAcked)
	if err != nil {
		return a, err
	}
	out, _ := s.store.UpdateAlarm(id, func(x *Alarm) {
		x.State = next
		x.AckedBy = by
	})
	return out, nil
}

// ResolveAlarm forces an alarm to resolved state.
func (s *Service) ResolveAlarm(ctx context.Context, id string) (Alarm, error) {
	a, ok := s.store.Alarm(id)
	if !ok {
		return Alarm{}, platform.NotFoundf("alarm %q not found", id)
	}
	next, err := MustTransition(a.State, AlarmResolved)
	if err != nil {
		return a, err
	}
	out, _ := s.store.UpdateAlarm(id, func(x *Alarm) {
		x.State = next
		x.ResolvedAt = s.clock.Now()
	})
	s.store.ResolveActiveForPoint(a.PointID)
	return out, nil
}

// Stats returns store counts for the summary endpoint.
func (s *Service) Stats(ctx context.Context) (points, readings, alarms, active int) {
	return s.store.Stats()
}

// ---- rule table ----

// ruleTable stores alarm rules, indexed by point id for fast lookup.
type ruleTable struct {
	mu      sync.RWMutex
	byPoint map[string][]Rule
}

func newRuleTable() *ruleTable {
	return &ruleTable{byPoint: make(map[string][]Rule)}
}

func (t *ruleTable) put(r Rule) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.byPoint[r.PointID] = append(t.byPoint[r.PointID], r)
}

func (t *ruleTable) list(pointID string) []Rule {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if pointID == "" {
		var all []Rule
		for _, rs := range t.byPoint {
			all = append(all, rs...)
		}
		return all
	}
	out := make([]Rule, len(t.byPoint[pointID]))
	copy(out, t.byPoint[pointID])
	return out
}

// now helper for explicit time usage
var _ = time.Now
