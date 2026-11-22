package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// BetterStackSink ships detections to BetterStack Telemetry (formerly Logtail).
// Each source has its own ingesting URL, supplied by the operator; the sink
// POSTs a JSON log object authenticated with the source token as a Bearer
// credential. BetterStack reads the event time from the "dt" field. Fail-soft
// and severity-floored like the other push sinks.
type BetterStackSink struct {
	url    string
	token  string
	client *http.Client
	minSev event.Severity
}

// NewBetterStack creates a BetterStack sink posting to the source's ingest URL.
func NewBetterStack(url, token string, timeout time.Duration, minSeverity event.Severity) *BetterStackSink {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &BetterStackSink{url: url, token: token, client: &http.Client{Timeout: timeout}, minSev: minSeverity}
}

// Emit posts the event as a single JSON log object.
func (s *BetterStackSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	record := map[string]any{
		"dt":         e.Time.UTC().Format(time.RFC3339Nano),
		"message":    fmt.Sprintf("%s %s %s", e.Kind, e.Source, e.Target),
		"service":    "heramba",
		"kind":       string(e.Kind),
		"severity":   e.SeverityS,
		"source":     e.Source,
		"target":     e.Target,
		"confidence": e.Confidence,
		"detail":     e.Detail,
	}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.client.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
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
		return fmt.Errorf("betterstack: non-2xx status %d", resp.StatusCode)
	}
	return nil
}
