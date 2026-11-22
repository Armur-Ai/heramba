package alert

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// defaultAxiomURL is the Axiom Cloud API base; override for self-hosted.
const defaultAxiomURL = "https://api.axiom.co"

// AxiomSink ships detections to Axiom's ingest API
// (POST {base}/v1/datasets/{dataset}/ingest) as a JSON array of events,
// authenticated with a Bearer API token. Fail-soft and severity-floored like
// the other push sinks.
type AxiomSink struct {
	*httpSender
	endpoint string
	token    string
	minSev   event.Severity
}

// NewAxiom creates an Axiom sink for a dataset. An empty base uses Axiom Cloud.
func NewAxiom(base, dataset, token string, timeout time.Duration, minSeverity event.Severity) *AxiomSink {
	if base == "" {
		base = defaultAxiomURL
	}
	endpoint := strings.TrimRight(base, "/") + "/v1/datasets/" + url.PathEscape(dataset) + "/ingest"
	return &AxiomSink{httpSender: newHTTPSender(timeout), endpoint: endpoint, token: token, minSev: minSeverity}
}

// Emit posts the event as a single-element JSON array.
func (s *AxiomSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	// Axiom stamps ingest time from the "_time" field when present.
	record := map[string]any{
		"_time":      e.Time.UTC().Format(time.RFC3339Nano),
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
	return s.deliver("axiom", httpPost{
		url:         s.endpoint,
		body:        body,
		contentType: "application/json",
		headers:     map[string]string{"Authorization": "Bearer " + s.token},
	})
}
