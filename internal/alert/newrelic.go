package alert

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// defaultNewRelicURL is the US Log API endpoint; override for EU
// (https://log-api.eu.newrelic.com/log/v1).
const defaultNewRelicURL = "https://log-api.newrelic.com/log/v1"

// NewRelicSink ships detections to the New Relic Log API. Each event becomes one
// log record with a message, millisecond timestamp, and the detection fields as
// attributes. Authenticated with the Api-Key header (a New Relic license or
// ingest key). Fail-soft and severity-floored like the other push sinks.
type NewRelicSink struct {
	*httpSender
	url    string
	apiKey string
	minSev event.Severity
}

// NewNewRelic creates a New Relic Logs sink. An empty url uses the US endpoint.
func NewNewRelic(url, apiKey string, timeout time.Duration, minSeverity event.Severity) *NewRelicSink {
	if url == "" {
		url = defaultNewRelicURL
	}
	return &NewRelicSink{httpSender: newHTTPSender(timeout), url: url, apiKey: apiKey, minSev: minSeverity}
}

// Emit posts the event as a single New Relic log record.
func (s *NewRelicSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	record := map[string]any{
		"message":   fmt.Sprintf("%s %s %s", e.Kind, e.Source, e.Target),
		"timestamp": e.Time.UnixMilli(),
		"attributes": map[string]any{
			"logtype":    "heramba",
			"kind":       string(e.Kind),
			"severity":   e.SeverityS,
			"source":     e.Source,
			"target":     e.Target,
			"confidence": e.Confidence,
			"detail":     e.Detail,
		},
	}
	// The Log API accepts a JSON array of log records.
	body, err := json.Marshal([]any{record})
	if err != nil {
		return err
	}
	return s.deliver("newrelic", httpPost{
		url:         s.url,
		body:        body,
		contentType: "application/json",
		headers:     map[string]string{"Api-Key": s.apiKey},
	})
}
