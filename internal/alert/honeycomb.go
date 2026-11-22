package alert

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// defaultHoneycombURL is the Honeycomb API base; override for the EU instance
// (https://api.eu1.honeycomb.io).
const defaultHoneycombURL = "https://api.honeycomb.io"

// HoneycombSink ships detections to Honeycomb's events API
// (POST {base}/1/events/{dataset}) as a flat JSON event, authenticated with the
// X-Honeycomb-Team header and timestamped via X-Honeycomb-Event-Time. Fail-soft
// and severity-floored like the other push sinks.
type HoneycombSink struct {
	*httpSender
	endpoint string
	apiKey   string
	minSev   event.Severity
}

// NewHoneycomb creates a Honeycomb sink for a dataset. An empty base uses the US
// API; the dataset is required and path-escaped into the events URL.
func NewHoneycomb(base, dataset, apiKey string, timeout time.Duration, minSeverity event.Severity) *HoneycombSink {
	if base == "" {
		base = defaultHoneycombURL
	}
	endpoint := strings.TrimRight(base, "/") + "/1/events/" + url.PathEscape(dataset)
	return &HoneycombSink{httpSender: newHTTPSender(timeout), endpoint: endpoint, apiKey: apiKey, minSev: minSeverity}
}

// Emit posts the event as a single Honeycomb event (flat key/values).
func (s *HoneycombSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	payload := map[string]any{
		"service":    "heramba",
		"kind":       string(e.Kind),
		"severity":   e.SeverityS,
		"source":     e.Source,
		"target":     e.Target,
		"confidence": e.Confidence,
		"detail":     e.Detail,
	}
	for k, v := range e.Labels {
		payload["label."+k] = v
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.deliver("honeycomb", httpPost{
		url:         s.endpoint,
		body:        body,
		contentType: "application/json",
		headers: map[string]string{
			"X-Honeycomb-Team":       s.apiKey,
			"X-Honeycomb-Event-Time": e.Time.UTC().Format(time.RFC3339Nano),
		},
	})
}
