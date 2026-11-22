package alert

import (
	"encoding/json"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// SumoLogicSink ships detections to a Sumo Logic HTTP Source. The collector URL
// is itself the secret (it embeds the source token), so the sink needs no extra
// auth: it simply POSTs the event as a JSON object, which Sumo ingests as a
// structured log. Fail-soft and severity-floored like the other push sinks.
type SumoLogicSink struct {
	*httpSender
	url    string
	minSev event.Severity
}

// NewSumoLogic creates a Sumo Logic sink posting to an HTTP Source URL.
func NewSumoLogic(url string, timeout time.Duration, minSeverity event.Severity) *SumoLogicSink {
	return &SumoLogicSink{httpSender: newHTTPSender(timeout), url: url, minSev: minSeverity}
}

// Emit posts the event as a JSON log line.
func (s *SumoLogicSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return s.deliver("sumologic", httpPost{
		url:         s.url,
		body:        body,
		contentType: "application/json",
		headers:     map[string]string{"X-Sumo-Category": "heramba/detections"},
	})
}
