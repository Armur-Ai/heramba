package alert

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// BetterStackSink ships detections to BetterStack Telemetry (formerly Logtail).
// Each source has its own ingesting URL, supplied by the operator; the sink
// POSTs a JSON log object authenticated with the source token as a Bearer
// credential. BetterStack reads the event time from the "dt" field. Fail-soft
// and severity-floored like the other push sinks.
type BetterStackSink struct {
	*httpSender
	url    string
	token  string
	minSev event.Severity
}

// NewBetterStack creates a BetterStack sink posting to the source's ingest URL.
func NewBetterStack(url, token string, timeout time.Duration, minSeverity event.Severity) *BetterStackSink {
	return &BetterStackSink{httpSender: newHTTPSender(timeout), url: url, token: token, minSev: minSeverity}
}

// Emit posts the event as a single JSON log object.
func (s *BetterStackSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	record := map[string]any{
		"dt":         e.Time.UTC().Format(time.RFC3339Nano),
		"message":    fmt.Sprintf("%s %s %s", e.Kind, e.Source, e.Target),
		"service":    "heramba",
		"kind":       string(e.Kind),
		"severity":   e.SeverityS,
		"source":     e.Source,
		"target":     e.Target,
		"confidence": e.Confidence,
		"detail":     e.Detail,
	}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return s.deliver("betterstack", httpPost{
		url:         s.url,
		body:        body,
		contentType: "application/json",
		headers:     map[string]string{"Authorization": "Bearer " + s.token},
	})
}
