package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	endpoint string
	token    string
	client   *http.Client
	minSev   event.Severity
}

// NewAxiom creates an Axiom sink for a dataset. An empty base uses Axiom Cloud.
func NewAxiom(base, dataset, token string, timeout time.Duration, minSeverity event.Severity) *AxiomSink {
	if base == "" {
		base = defaultAxiomURL
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	endpoint := strings.TrimRight(base, "/") + "/v1/datasets/" + url.PathEscape(dataset) + "/ingest"
	return &AxiomSink{endpoint: endpoint, token: token, client: &http.Client{Timeout: timeout}, minSev: minSeverity}
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
	ctx, cancel := context.WithTimeout(context.Background(), s.client.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.token)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("axiom: non-2xx status %d", resp.StatusCode)
	}
	return nil
}
