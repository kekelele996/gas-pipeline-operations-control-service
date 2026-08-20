package permit

// ConflictCheckers define the surfaces the permit service consults before
// approving a permit. They are narrow interfaces implemented by the dispatch
// and incident packages, declared here to avoid an import cycle (permit does
// not import dispatch or incident).

// DispatchConflictChecker reports pending/issued-but-unexecuted dispatch
// orders for a segment. The dispatch service implements this.
type DispatchConflictChecker interface {
	// PendingOrdersForSegment returns the ids of orders for the segment that
	// are in a non-terminal state (issued but not executed, etc.).
	PendingOrdersForSegment(segmentID string) []string
}

// IncidentConflictChecker reports open incidents for a segment.
type IncidentConflictChecker interface {
	OpenIncidentsForSegment(segmentID string) []string
}

// noopDispatchChecker / noopIncidentChecker are safe defaults used when the
// permit service is constructed without cross-domain checkers (e.g. in tests
// that exercise the permit flow in isolation).
type noopDispatchChecker struct{}

func (noopDispatchChecker) PendingOrdersForSegment(string) []string { return nil }

type noopIncidentChecker struct{}

func (noopIncidentChecker) OpenIncidentsForSegment(string) []string { return nil }
