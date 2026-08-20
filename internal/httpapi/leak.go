package httpapi

// leak.go: HTTP handlers for leak-detection analysis. The handler pulls the
// pressure history for the upstream and downstream points of a segment and
// runs the detector.

import (
	"net/http"

	"gas-pipeline-operations-control-service/internal/scada"
)

// leakAnalyzeHandler analyzes a segment. It expects the query params
// ?upstream=POINT_ID&downstream=POINT_ID to identify the segment endpoints.
// If omitted, it tries to find two pressure points on the segment.
func leakAnalyzeHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		segID := pathValue(r, "segmentId")
		upID := r.URL.Query().Get("upstream")
		downID := r.URL.Query().Get("downstream")
		if upID == "" || downID == "" {
			// discover pressure points on the segment from network
			pts := deps.Network.ListPoints(ctx)
			var pressurePoints []string
			for _, p := range pts {
				if p.SegmentID == segID && string(p.Type) == string(scada.TypePressure) {
					pressurePoints = append(pressurePoints, p.ID)
				}
			}
			if len(pressurePoints) >= 2 {
				upID = pressurePoints[0]
				downID = pressurePoints[len(pressurePoints)-1]
			}
		}
		var upstream, downstream []scada.Reading
		if upID != "" {
			if hist, err := deps.SCADA.History(ctx, upID, 0); err == nil {
				upstream = hist
			}
		}
		if downID != "" {
			if hist, err := deps.SCADA.History(ctx, downID, 0); err == nil {
				downstream = hist
			}
		}
		alert, err := deps.Leak.AnalyzeSegment(ctx, segID, upstream, downstream)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"segment_id": segID,
			"alert":      alert,
		})
	}
}
