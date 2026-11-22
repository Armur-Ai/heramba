package alert

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// LokiSink ships detections to a Grafana Loki push API (/loki/api/v1/push). Each
// event becomes one log entry on a stream labeled by app/kind/severity (kept
// low-cardinality on purpose — the mutable detail rides in the log line, not the
// labels). Fail-soft and severity-floored like the other push sinks. An optional
// tenant sets the X-Scope-OrgID header for multi-tenant Loki.
type LokiSink struct {
	*httpSender
	url    string
	tenant string
	minSev event.Severity
}

// NewLoki creates a Loki sink. url is the full push endpoint, e.g.
// https://loki:3100/loki/api/v1/push. tenant may be "".
func NewLoki(url, tenant string, timeout time.Duration, minSeverity event.Severity) *LokiSink {
	return &LokiSink{httpSender: newHTTPSender(timeout), url: url, tenant: tenant, minSev: minSeverity}
}

// Emit posts the event as a single Loki stream entry.
func (s *LokiSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	body, err := json.Marshal(LokiPayload(e))
	if err != nil {
		return err
	}
	p := httpPost{url: s.url, body: body, contentType: "application/json"}
	if s.tenant != "" {
		p.headers = map[string]string{"X-Scope-OrgID": s.tenant}
	}
	return s.deliver("loki", p)
}

type lokiPush struct {
	Streams []lokiStream `json:"streams"`
}
type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"`
}

// LokiPayload builds the push body for an event. Exported for tests.
func LokiPayload(e event.Event) any {
	line, _ := json.Marshal(e)
	ts := strconv.FormatInt(e.Time.UTC().UnixNano(), 10)
	return lokiPush{
		Streams: []lokiStream{{
			Stream: map[string]string{
				"app":      "heramba",
				"kind":     string(e.Kind),
				"severity": e.Severity.String(),
			},
			Values: [][2]string{{ts, string(line)}},
		}},
	}
}
