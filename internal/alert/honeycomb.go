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

// defaultHoneycombURL is the Honeycomb API base; override for the EU instance
// (https://api.eu1.honeycomb.io).
const defaultHoneycombURL = "https://api.honeycomb.io"

// HoneycombSink ships detections to Honeycomb's events API
// (POST {base}/1/events/{dataset}) as a flat JSON event, authenticated with the
// X-Honeycomb-Team header and timestamped via X-Honeycomb-Event-Time. Fail-soft
// and severity-floored like the other push sinks.
type HoneycombSink struct {
	endpoint string
	apiKey   string
	client   *http.Client
	minSev   event.Severity
}

// NewHoneycomb creates a Honeycomb sink for a dataset. An empty base uses the US
// API; the dataset is required and path-escaped into the events URL.
func NewHoneycomb(base, dataset, apiKey string, timeout time.Duration, minSeverity event.Severity) *HoneycombSink {
	if base == "" {
		base = defaultHoneycombURL
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	endpoint := strings.TrimRight(base, "/") + "/1/events/" + url.PathEscape(dataset)
	return &HoneycombSink{endpoint: endpoint, apiKey: apiKey, client: &http.Client{Timeout: timeout}, minSev: minSeverity}
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
	ctx, cancel := context.WithTimeout(context.Background(), s.client.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Honeycomb-Team", s.apiKey)
	req.Header.Set("X-Honeycomb-Event-Time", e.Time.UTC().Format(time.RFC3339Nano))
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("honeycomb: non-2xx status %d", resp.StatusCode)
	}
	return nil
}
