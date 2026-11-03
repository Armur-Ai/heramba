package sensor

import (
	"bufio"
	"encoding/json"
	"strings"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/evidence"
)

// The adapters below each read a simple JSONL raw format and normalize it into
// evidence.SecurityEvents. They are intentionally small and dependency-free;
// richer OpenTelemetry/cloud-audit parsing can slot in behind the same
// Collector interface. Raw input is treated as hostile: we never execute it,
// only extract typed fields, and oversized/garbled lines are skipped.

// drainLines turns a JSONL blob into decoded records of type T, skipping blank
// or unparseable lines (a bad line never aborts a collection pass).
func drainLines[T any](raw string) []T {
	var out []T
	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec T
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func parseTime(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	return time.Now().UTC()
}

// --- App access-log collector (OpenTelemetry-ish HTTP spans / access logs) ---

type appRecord struct {
	Time   string `json:"ts"`
	IP     string `json:"ip"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Status int    `json:"status"`
	UA     string `json:"ua"`
	ID     string `json:"id"` // optional idempotency ref
}

// AppLogCollector normalizes HTTP access events.
type AppLogCollector struct {
	Source func() (string, error) // returns a JSONL blob (file read, buffer, etc.)
}

func (c *AppLogCollector) Name() string { return "app" }

func (c *AppLogCollector) Collect() ([]evidence.SecurityEvent, error) {
	raw, err := c.Source()
	if err != nil {
		return nil, err
	}
	var out []evidence.SecurityEvent
	for _, r := range drainLines[appRecord](raw) {
		kind := event.KindProbe
		if r.Status == 404 {
			kind = event.KindProbe
		}
		e := evidence.NewSecurityEvent(kind, event.SeverityLow, r.IP, evidence.Provenance{
			Sensor:     "app",
			RawRef:     r.ID,
			ObservedAt: parseTime(r.Time),
		}).WithAttr("method", r.Method).WithAttr("path", r.Path).WithAttr("ua", r.UA).WithStage(evidence.StageRecon)
		e.Target = r.Path
		e.Confidence = 0.4
		out = append(out, e)
	}
	return out, nil
}

// --- Identity / auth-log collector ---

type authRecord struct {
	Time    string `json:"ts"`
	IP      string `json:"ip"`
	Subject string `json:"subject"`
	Result  string `json:"result"` // success|failure
	ID      string `json:"id"`
}

// AuthLogCollector normalizes authentication events; a failure burst is an
// access-stage signal and the subject is a distinctive identity token.
type AuthLogCollector struct {
	Source func() (string, error)
}

func (c *AuthLogCollector) Name() string { return "auth" }

func (c *AuthLogCollector) Collect() ([]evidence.SecurityEvent, error) {
	raw, err := c.Source()
	if err != nil {
		return nil, err
	}
	var out []evidence.SecurityEvent
	for _, r := range drainLines[authRecord](raw) {
		sev := event.SeverityLow
		if r.Result == "failure" {
			sev = event.SeverityMedium
		}
		e := evidence.NewSecurityEvent(event.KindProbe, sev, r.IP, evidence.Provenance{
			Sensor:     "auth",
			RawRef:     r.ID,
			ObservedAt: parseTime(r.Time),
		}).WithAttr("auth_subject", r.Subject).WithAttr("auth_result", r.Result).WithStage(evidence.StageAccess)
		e.Detail = "auth " + r.Result + " for " + r.Subject
		e.Confidence = 0.5
		out = append(out, e)
	}
	return out, nil
}

// --- Network / DNS metadata collector ---

type netRecord struct {
	Time  string `json:"ts"`
	IP    string `json:"ip"`
	Query string `json:"query"` // DNS name queried, or callback host
	Kind  string `json:"kind"`  // dns|callback
	ID    string `json:"id"`
}

// NetworkCollector normalizes DNS / outbound-callback metadata; a callback to
// an unexpected host is an exfiltration-stage signal.
type NetworkCollector struct {
	Source func() (string, error)
}

func (c *NetworkCollector) Name() string { return "network" }

func (c *NetworkCollector) Collect() ([]evidence.SecurityEvent, error) {
	raw, err := c.Source()
	if err != nil {
		return nil, err
	}
	var out []evidence.SecurityEvent
	for _, r := range drainLines[netRecord](raw) {
		stage := evidence.StageDiscovery
		kind := event.KindProbe
		if r.Kind == "callback" {
			stage = evidence.StageExfiltration
			kind = event.KindEgressBlocked
		}
		e := evidence.NewSecurityEvent(kind, event.SeverityMedium, r.IP, evidence.Provenance{
			Sensor:     "network",
			RawRef:     r.ID,
			ObservedAt: parseTime(r.Time),
		}).WithAttr("query", r.Query).WithStage(stage)
		e.Target = r.Query
		e.Confidence = 0.5
		out = append(out, e)
	}
	return out, nil
}

// StaticSource is a convenience Source backed by a fixed string (tests / piped
// input).
func StaticSource(blob string) func() (string, error) {
	return func() (string, error) { return blob, nil }
}
