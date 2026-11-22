package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/Armur-Ai/heramba/internal/event"
)

// defaultEventCount is how many events the feed requests when n is unset.
const defaultEventCount = 100

// maxEventCount caps the per-request event count to bound work and payload size.
const maxEventCount = 2000

// writeJSON encodes v as JSON with the right content type.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// recentEvents fetches up to n events from the source, never returning nil.
func (s *server) recentEvents(n int) []event.Event {
	var evs []event.Event
	if s.events != nil {
		evs = s.events(n)
	}
	if evs == nil {
		evs = []event.Event{}
	}
	return evs
}

// snapshot fetches the metrics snapshot from the source, or a zero snapshot.
func (s *server) snapshot() Metrics {
	if s.metrics != nil {
		return s.metrics()
	}
	return Metrics{ByKind: map[string]int{}, BySeverity: map[string]int{}}
}

// clampCount parses the ?n= query, applying the default and cap.
func clampCount(q string) int {
	n := defaultEventCount
	if q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	if n > maxEventCount {
		n = maxEventCount
	}
	return n
}

// handleEvents serves GET /api/events?n= as a JSON array (never null).
func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	n := clampCount(r.URL.Query().Get("n"))
	writeJSON(w, http.StatusOK, s.recentEvents(n))
}

// metricsJSON is the wire shape for GET /api/metrics: the snapshot plus a
// JSON-friendly uptime in seconds.
type metricsJSON struct {
	Mode          string         `json:"mode"`
	Total         int            `json:"total"`
	ByKind        map[string]int `json:"by_kind"`
	BySeverity    map[string]int `json:"by_severity"`
	Sources       int            `json:"sources"`
	UptimeSeconds int            `json:"uptime_seconds"`
	Score         ScoreHistogram `json:"score"`
}

// handleMetrics serves GET /api/metrics as a JSON snapshot.
func (s *server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	m := s.snapshot()
	if m.ByKind == nil {
		m.ByKind = map[string]int{}
	}
	if m.BySeverity == nil {
		m.BySeverity = map[string]int{}
	}
	out := metricsJSON{
		Mode:          m.Mode,
		Total:         m.Total,
		ByKind:        m.ByKind,
		BySeverity:    m.BySeverity,
		Sources:       m.Sources,
		UptimeSeconds: int(s.uptime().Seconds()),
		Score:         m.Score,
	}
	writeJSON(w, http.StatusOK, out)
}

// handleTimeline serves GET /api/timeline?window= as bucketed event counts.
func (s *server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	window := parseWindow(r.URL.Query().Get("window"))
	evs := s.recentEvents(maxEventCount)
	writeJSON(w, http.StatusOK, buildTimeline(evs, window))
}

// handleSources serves GET /api/sources: per-source aggregates, worst first.
func (s *server) handleSources(w http.ResponseWriter, _ *http.Request) {
	evs := s.recentEvents(maxEventCount)
	writeJSON(w, http.StatusOK, topSources(evs))
}

// handleCampaigns serves GET /api/campaigns: coordinated multi-IP swarms.
func (s *server) handleCampaigns(w http.ResponseWriter, _ *http.Request) {
	evs := s.recentEvents(maxEventCount)
	writeJSON(w, http.StatusOK, detectCampaigns(evs))
}

// handleDecoys serves GET /api/decoys: per-decoy (target) hit aggregates.
func (s *server) handleDecoys(w http.ResponseWriter, _ *http.Request) {
	evs := s.recentEvents(maxEventCount)
	writeJSON(w, http.StatusOK, topDecoys(evs))
}

// --- Write controls -------------------------------------------------------

// controlResult is the JSON echo returned by control endpoints.
type controlResult struct {
	OK     bool   `json:"ok"`
	Mode   string `json:"mode,omitempty"`
	Paused bool   `json:"paused"`
	Preset string `json:"preset,omitempty"`
	Error  string `json:"error,omitempty"`
}

// authorized reports whether the request carries the configured bearer token.
// When no token is configured, writes are always allowed.
func (s *server) authorized(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return false
	}
	return strings.TrimSpace(h[len(prefix):]) == s.token
}

// guardWrite enforces the bearer token; it returns false (and writes 401) when
// the request is not authorized.
func (s *server) guardWrite(w http.ResponseWriter, r *http.Request) bool {
	if s.authorized(r) {
		return true
	}
	writeJSON(w, http.StatusUnauthorized, controlResult{OK: false, Error: "unauthorized"})
	return false
}

// decodeBody decodes the JSON request body into v, tolerating an empty body.
func decodeBody(r *http.Request, v any) error {
	if r.Body == nil {
		return nil
	}
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 64<<10))
	err := dec.Decode(v)
	if err != nil && err.Error() == "EOF" {
		return nil
	}
	return err
}

// controlState builds the echo from the controller's state when available.
func (s *server) controlState() controlResult {
	res := controlResult{OK: true}
	if mc, ok := s.controller.(*MemoryController); ok {
		mode, paused, preset, _ := mc.State()
		res.Mode, res.Paused, res.Preset = mode, paused, preset
	}
	return res
}

// handleControlMode serves POST /api/control/mode {"mode":"observe|protect"}.
func (s *server) handleControlMode(w http.ResponseWriter, r *http.Request) {
	if !s.guardWrite(w, r) {
		return
	}
	var body struct {
		Mode string `json:"mode"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, controlResult{OK: false, Error: "bad request"})
		return
	}
	mode := strings.ToLower(strings.TrimSpace(body.Mode))
	if mode != "observe" && mode != "protect" {
		writeJSON(w, http.StatusBadRequest, controlResult{OK: false, Error: "mode must be observe or protect"})
		return
	}
	if err := s.controller.SetMode(mode); err != nil {
		writeJSON(w, http.StatusInternalServerError, controlResult{OK: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.controlState())
}

// handleControlClear serves POST /api/control/clear.
func (s *server) handleControlClear(w http.ResponseWriter, r *http.Request) {
	if !s.guardWrite(w, r) {
		return
	}
	if err := s.controller.Clear(); err != nil {
		writeJSON(w, http.StatusInternalServerError, controlResult{OK: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.controlState())
}

// handleControlPause serves POST /api/control/pause {"paused":true|false}.
func (s *server) handleControlPause(w http.ResponseWriter, r *http.Request) {
	if !s.guardWrite(w, r) {
		return
	}
	var body struct {
		Paused bool `json:"paused"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, controlResult{OK: false, Error: "bad request"})
		return
	}
	if err := s.controller.SetPaused(body.Paused); err != nil {
		writeJSON(w, http.StatusInternalServerError, controlResult{OK: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.controlState())
}

// handleControlDecoyPreset serves POST /api/control/decoy-preset {"preset":"..."}.
func (s *server) handleControlDecoyPreset(w http.ResponseWriter, r *http.Request) {
	if !s.guardWrite(w, r) {
		return
	}
	var body struct {
		Preset string `json:"preset"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, controlResult{OK: false, Error: "bad request"})
		return
	}
	preset := strings.TrimSpace(body.Preset)
	if preset == "" {
		writeJSON(w, http.StatusBadRequest, controlResult{OK: false, Error: "preset required"})
		return
	}
	if err := s.controller.ApplyDecoyPreset(preset); err != nil {
		writeJSON(w, http.StatusInternalServerError, controlResult{OK: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.controlState())
}
