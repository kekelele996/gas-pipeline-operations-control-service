package httpapi

// types.go defines the narrow service interfaces each HTTP handler depends
// on. They mirror the real service method signatures so the concrete services
// satisfy them with no adapter code. Keeping them here means the handler
// files don't need to import the domain packages.

import (
	"context"
	"time"

	"gas-pipeline-operations-control-service/internal/audit"
	"gas-pipeline-operations-control-service/internal/contract"
	"gas-pipeline-operations-control-service/internal/dispatch"
	"gas-pipeline-operations-control-service/internal/incident"
	"gas-pipeline-operations-control-service/internal/leakdetect"
	"gas-pipeline-operations-control-service/internal/metering"
	"gas-pipeline-operations-control-service/internal/network"
	"gas-pipeline-operations-control-service/internal/nomination"
	"gas-pipeline-operations-control-service/internal/notify"
	"gas-pipeline-operations-control-service/internal/permit"
	"gas-pipeline-operations-control-service/internal/scada"
)

// ---- Network ----

type NetworkService interface {
	ListSegments(ctx context.Context) []network.Segment
	GetSegment(ctx context.Context, id string) (network.Segment, error)
	UpsertSegment(ctx context.Context, in network.SegmentInput) (network.Segment, error)
	ListSegmentDevices(ctx context.Context, segmentID string) (network.SegmentDevices, error)
	ListStations(ctx context.Context) []network.Station
	GetStation(ctx context.Context, id string) (network.Station, error)
	UpsertStation(ctx context.Context, in network.StationInput) (network.Station, error)
	ListCompressors(ctx context.Context) []network.Compressor
	GetCompressor(ctx context.Context, id string) (network.Compressor, error)
	UpsertCompressor(ctx context.Context, in network.CompressorInput) (network.Compressor, error)
	ChangeCompressorState(ctx context.Context, id string, target network.CompressorState, reason string) (network.Compressor, error)
	ListValves(ctx context.Context) []network.Valve
	GetValve(ctx context.Context, id string) (network.Valve, error)
	UpsertValve(ctx context.Context, in network.ValveInput) (network.Valve, error)
	ChangeValveState(ctx context.Context, id string, target network.ValveState) (network.Valve, error)
	ListPoints(ctx context.Context) []network.Point
	GetPoint(ctx context.Context, id string) (network.Point, error)
	UpsertPoint(ctx context.Context, in network.PointInput) (network.Point, error)
	TogglePoint(ctx context.Context, id string, enabled bool) (network.Point, error)
	Counts() (segments, stations, compressors, valves, points int)
}

// ---- SCADA ----

type ScadaService interface {
	ListPoints(ctx context.Context) []scada.Point
	Point(ctx context.Context, id string) (scada.Point, error)
	RegisterPoint(ctx context.Context, p scada.Point) scada.Point
	Ingest(ctx context.Context, batch []scada.Reading) []scada.IngestResult
	History(ctx context.Context, pointID string, limit int) ([]scada.Reading, error)
	Latest(ctx context.Context, pointID string) (scada.Reading, error)
	Alarms(ctx context.Context, pointID string, onlyActive bool) []scada.Alarm
	AckAlarm(ctx context.Context, id, by string) (scada.Alarm, error)
	ResolveAlarm(ctx context.Context, id string) (scada.Alarm, error)
	Stats(ctx context.Context) (points, readings, alarms, active int)
}

// ---- Metering ----

type MeteringService interface {
	UpsertMeter(ctx context.Context, in metering.MeterInput) (metering.Meter, error)
	ListMeters(ctx context.Context) []metering.Meter
	GetMeter(ctx context.Context, id string) (metering.Meter, error)
	RecordReading(ctx context.Context, rd metering.RawReading) (metering.DailyTotal, error)
	GetDailyTotal(ctx context.Context, meterID, date string) (metering.DailyTotal, error)
	DailyForDate(ctx context.Context, date string) []metering.DailyTotal
	SettleDaily(ctx context.Context, date string) (metering.Settlement, error)
	Confirm(ctx context.Context, date, by string) (metering.Settlement, error)
	Reconcile(ctx context.Context, date, by string) (metering.Settlement, error)
	GetSettlement(ctx context.Context, date string) (metering.Settlement, error)
	ListSettlements(ctx context.Context) []metering.Settlement
}

// ---- Contract ----

type ContractService interface {
	Create(ctx context.Context, in contract.ContractInput) (contract.Contract, error)
	Get(ctx context.Context, id string) (contract.Contract, error)
	List(ctx context.Context) []contract.Contract
	ListByShipper(ctx context.Context, shipperID string) []contract.Contract
	Remaining(ctx context.Context, id string) (float64, error)
	Reserve(ctx context.Context, id string, amount float64) (contract.Contract, error)
	Release(ctx context.Context, id string, amount float64) (contract.Contract, error)
	SetState(ctx context.Context, id string, target contract.State) (contract.Contract, error)
}

// ---- Nomination ----

type NominationService interface {
	Create(ctx context.Context, in nomination.Input) (nomination.Nomination, error)
	Submit(ctx context.Context, id, by string) (nomination.Nomination, error)
	Confirm(ctx context.Context, id, by string) (nomination.Nomination, error)
	Execute(ctx context.Context, id string) (nomination.Nomination, error)
	Cancel(ctx context.Context, id string) (nomination.Nomination, error)
	Get(ctx context.Context, id string) (nomination.Nomination, error)
	List(ctx context.Context) []nomination.Nomination
}

// ---- Permit ----

type PermitService interface {
	Apply(ctx context.Context, in permit.Input) (permit.Permit, error)
	Approve(ctx context.Context, id, approver string) (permit.Permit, error)
	Start(ctx context.Context, id string) (permit.Permit, error)
	Complete(ctx context.Context, id string) (permit.Permit, error)
	Cancel(ctx context.Context, id, reason string) (permit.Permit, error)
	ExpireScan(ctx context.Context) []string
	Get(ctx context.Context, id string) (permit.Permit, error)
	List(ctx context.Context) []permit.Permit
}

// ---- Dispatch ----

type DispatchService interface {
	Create(ctx context.Context, in dispatch.Input) (dispatch.Order, error)
	Issue(ctx context.Context, id, by string) (dispatch.Order, error)
	Execute(ctx context.Context, id string) (dispatch.Order, error)
	Revoke(ctx context.Context, id string) (dispatch.Order, error)
	Get(ctx context.Context, id string) (dispatch.Order, error)
	List(ctx context.Context) []dispatch.Order
}

// ---- Incident ----

type IncidentService interface {
	Report(ctx context.Context, in incident.Input) (incident.Incident, error)
	Confirm(ctx context.Context, id, assignee string) (incident.Incident, error)
	AddAction(ctx context.Context, id, description, owner string) (incident.Incident, error)
	CompleteAction(ctx context.Context, id, actionID string) (incident.Incident, error)
	Close(ctx context.Context, id string) (incident.Incident, error)
	Get(ctx context.Context, id string) (incident.Incident, error)
	List(ctx context.Context) []incident.Incident
	OpenCount(ctx context.Context) int
}

// ---- Leak detect ----

type LeakService interface {
	AnalyzeSegment(ctx context.Context, segmentID string, upstream, downstream []scada.Reading) (*leakdetect.LeakAlert, error)
	Thresholds() leakdetect.Thresholds
}

// ---- Audit ----

type AuditService interface {
	Query(ctx context.Context, q audit.Query) []audit.Entry
	Count(ctx context.Context) int
	Summary(ctx context.Context) audit.Summary
}

// ---- Notify ----

type NotifyService interface {
	Enqueue(ctx context.Context, recipient string, ch notify.Channel, subject, body string) (notify.Notification, error)
	PushBatch(ctx context.Context) (sent, failed int, err error)
	RetryFailed(ctx context.Context) int
	CountPending(ctx context.Context) int
	List(ctx context.Context) []notify.Notification
}

// summaryTS is used only to keep the time import referenced when no domain
// handler needs it directly.
var _ = time.Now
