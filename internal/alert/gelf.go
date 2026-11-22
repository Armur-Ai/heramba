package alert

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// GELFSink ships detections to a Graylog GELF HTTP input (POST {url}/gelf, where
// url is typically http://graylog:12201/gelf) as a GELF 1.1 JSON message. Custom
// fields are prefixed with "_" per the GELF spec, and the level is a syslog
// numeric. Fail-soft and severity-floored like the other push sinks.
type GELFSink struct {
	*httpSender
	url    string
	host   string
	minSev event.Severity
}

// NewGELF creates a Graylog GELF-over-HTTP sink. host labels the GELF "host"
// field (defaults to "heramba").
func NewGELF(url, host string, timeout time.Duration, minSeverity event.Severity) *GELFSink {
	if host == "" {
		host = "heramba"
	}
	return &GELFSink{httpSender: newHTTPSender(timeout), url: url, host: host, minSev: minSeverity}
}

// Emit posts the event as a GELF 1.1 message.
func (s *GELFSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	body, err := json.Marshal(GELFMessage(e, s.host))
	if err != nil {
		return err
	}
	return s.deliver("gelf", httpPost{url: s.url, body: body, contentType: "application/json"})
}

// GELFMessage builds a GELF 1.1 payload for an event. Exported for tests.
func GELFMessage(e event.Event, host string) map[string]any {
	msg := map[string]any{
		"version":       "1.1",
		"host":          host,
		"short_message": fmt.Sprintf("%s %s %s", e.Kind, e.Source, e.Target),
		"timestamp":     float64(e.Time.UnixNano()) / 1e9, // GELF wants seconds (fractional ok)
		"level":         gelfLevel(e.Severity),
		// Custom fields MUST be prefixed with an underscore per the GELF spec.
		"_kind":       string(e.Kind),
		"_severity":   e.SeverityS,
		"_source":     e.Source,
		"_target":     e.Target,
		"_confidence": e.Confidence,
		"_detail":     e.Detail,
	}
	if e.Detail != "" {
		msg["full_message"] = e.Detail
	}
	return msg
}

// gelfLevel maps severity to a syslog numeric level (GELF's "level").
func gelfLevel(s event.Severity) int {
	switch s {
	case event.SeverityCritical:
		return 2 // critical
	case event.SeverityHigh:
		return 3 // error
	case event.SeverityMedium:
		return 4 // warning
	case event.SeverityLow:
		return 5 // notice
	default:
		return 6 // informational
	}
}
