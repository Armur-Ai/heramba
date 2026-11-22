package alert

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// defaultDatadogURL is the US1 Logs intake; override for other sites (EU, US3…).
const defaultDatadogURL = "https://http-intake.logs.datadoghq.com/api/v2/logs"

// DatadogSink ships detections to the Datadog Logs intake (v2), tagged so they
// land as structured, searchable security events. Fail-soft and severity-floored.
type DatadogSink struct {
	*httpSender
	url    string
	apiKey string
	minSev event.Severity
}

// NewDatadog creates a Datadog Logs sink. An empty url uses the US1 intake.
func NewDatadog(url, apiKey string, timeout time.Duration, minSeverity event.Severity) *DatadogSink {
	if url == "" {
		url = defaultDatadogURL
	}
	return &DatadogSink{httpSender: newHTTPSender(timeout), url: url, apiKey: apiKey, minSev: minSeverity}
}

// Emit posts the event as a single Datadog log entry.
func (s *DatadogSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	entry := map[string]any{
		"ddsource": "heramba",
		"service":  "heramba",
		"ddtags":   "kind:" + string(e.Kind) + ",severity:" + e.SeverityS,
		"status":   ddStatus(e.Severity),
		"message":  fmt.Sprintf("%s %s %s", e.Kind, e.Source, e.Target),
		"event":    e,
	}
	body, err := json.Marshal([]any{entry})
	if err != nil {
		return err
	}
	return s.deliver("datadog", httpPost{
		url:         s.url,
		body:        body,
		contentType: "application/json",
		headers:     map[string]string{"DD-API-KEY": s.apiKey},
	})
}

// ddStatus maps severity to a Datadog log status.
func ddStatus(s event.Severity) string {
	switch s {
	case event.SeverityCritical:
		return "critical"
	case event.SeverityHigh:
		return "error"
	case event.SeverityMedium:
		return "warning"
	case event.SeverityLow:
		return "info"
	default:
		return "notice"
	}
}
