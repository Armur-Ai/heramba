package alert

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// OpenObserveSink ships detections to an OpenObserve JSON ingest endpoint
// (POST {base}/api/{org}/{stream}/_json) as a JSON array of records, with HTTP
// basic auth (email + password/token). Fail-soft and severity-floored like the
// other push sinks.
type OpenObserveSink struct {
	*httpSender
	endpoint string
	user     string
	pass     string
	minSev   event.Severity
}

// NewOpenObserve creates an OpenObserve sink. base is the instance root (e.g.
// http://localhost:5080); org defaults to "default" and stream to "heramba".
func NewOpenObserve(base, org, stream, user, pass string, timeout time.Duration, minSeverity event.Severity) *OpenObserveSink {
	if org == "" {
		org = "default"
	}
	if stream == "" {
		stream = "heramba"
	}
	endpoint := strings.TrimRight(base, "/") + "/api/" + org + "/" + stream + "/_json"
	return &OpenObserveSink{httpSender: newHTTPSender(timeout), endpoint: endpoint, user: user, pass: pass, minSev: minSeverity}
}

// Emit posts the event as a single-record JSON array.
func (s *OpenObserveSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	record := map[string]any{
		"_timestamp": e.Time.UnixMicro(), // OpenObserve's microsecond ingest time
		"service":    "heramba",
		"kind":       string(e.Kind),
		"severity":   e.SeverityS,
		"source":     e.Source,
		"target":     e.Target,
		"confidence": e.Confidence,
		"detail":     e.Detail,
	}
	body, err := json.Marshal([]any{record})
	if err != nil {
		return err
	}
	p := httpPost{url: s.endpoint, body: body, contentType: "application/json"}
	if s.user != "" || s.pass != "" {
		p.useBasic = true
		p.basicUser = s.user
		p.basicPass = s.pass
	}
	return s.deliver("openobserve", p)
}
