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

// defaultDatadogURL is the US1 Logs intake; override for other sites (EU, US3…).
const defaultDatadogURL = "https://http-intake.logs.datadoghq.com/api/v2/logs"

// DatadogSink ships detections to the Datadog Logs intake (v2), tagged so they
// land as structured, searchable security events. Fail-soft and severity-floored.
type DatadogSink struct {
	url    string
	apiKey string
	client *http.Client
	minSev event.Severity
}

// NewDatadog creates a Datadog Logs sink. An empty url uses the US1 intake.
func NewDatadog(url, apiKey string, timeout time.Duration, minSeverity event.Severity) *DatadogSink {
	if url == "" {
		url = defaultDatadogURL
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &DatadogSink{url: url, apiKey: apiKey, client: &http.Client{Timeout: timeout}, minSev: minSeverity}
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
	ctx, cancel := context.WithTimeout(context.Background(), s.client.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", s.apiKey)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("datadog: non-2xx status %d", resp.StatusCode)
	}
	return nil
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
