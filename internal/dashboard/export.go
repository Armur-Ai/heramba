package dashboard

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// exportFilename stamps a download name with a UTC date so saved files are
// self-describing and sort chronologically.
func exportFilename(ext string) string {
	return "heramba-events-" + time.Now().UTC().Format("20060102-150405") + "." + ext
}

// exportCSVHeader is the column order of the events CSV.
var exportCSVHeader = []string{"time", "kind", "severity", "source", "target", "confidence", "detail"}

// handleExportCSV serves GET /api/export/events.csv as a downloadable CSV of the
// recent events (newest first, `n` honoured and capped).
func (s *server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	evs := s.recentEvents(clampCount(r.URL.Query().Get("n")))

	h := w.Header()
	h.Set("Content-Type", "text/csv; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Disposition", `attachment; filename="`+exportFilename("csv")+`"`)
	w.WriteHeader(http.StatusOK)

	cw := csv.NewWriter(w)
	_ = cw.Write(exportCSVHeader)
	for _, e := range evs {
		_ = cw.Write([]string{
			e.Time.UTC().Format(time.RFC3339),
			string(e.Kind),
			sevLabel(e),
			e.Source,
			e.Target,
			strconv.FormatFloat(e.Confidence, 'f', 2, 64),
			e.Detail,
		})
	}
	cw.Flush()
}

// handleExportJSON serves GET /api/export/events.json as a downloadable,
// pretty-printed JSON array of the recent events.
func (s *server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	evs := s.recentEvents(clampCount(r.URL.Query().Get("n")))

	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Disposition", `attachment; filename="`+exportFilename("json")+`"`)
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(evs)
}
