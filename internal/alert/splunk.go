package alert

import (
	"encoding/json"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// SplunkSink ships detections to a Splunk HTTP Event Collector (HEC). The event
// is wrapped in HEC's envelope ({time, sourcetype, event}) and authenticated
// with the HEC token. Fail-soft and severity-floored like the other push sinks.
type SplunkSink struct {
	*httpSender
	url    string // full HEC collector URL, e.g. https://host:8088/services/collector/event
	token  string
	minSev event.Severity
}

// NewSplunk creates a Splunk HEC sink. Only events at or above minSeverity are shipped.
func NewSplunk(url, token string, timeout time.Duration, minSeverity event.Severity) *SplunkSink {
	return &SplunkSink{httpSender: newHTTPSender(timeout), url: url, token: token, minSev: minSeverity}
}

// Emit wraps the event in the HEC envelope and posts it.
func (s *SplunkSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	payload := map[string]any{
		"time":       e.Time.Unix(),
		"sourcetype": "heramba",
		"source":     "heramba",
		"event":      e,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.deliver("splunk", httpPost{
		url:         s.url,
		body:        body,
		contentType: "application/json",
		headers:     map[string]string{"Authorization": "Splunk " + s.token},
	})
}
