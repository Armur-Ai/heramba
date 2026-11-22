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

const opsgenieURL = "https://api.opsgenie.com/v2/alerts"

// OpsgenieSink raises alerts via the Opsgenie Alerts API. Like PagerDuty it is a
// paging path, so it defaults to a high severity floor.
type OpsgenieSink struct {
	apiKey string
	url    string
	client *http.Client
	minSev event.Severity
}

// NewOpsgenie creates an Opsgenie sink with the given API key.
func NewOpsgenie(apiKey string, timeout time.Duration, minSeverity event.Severity) *OpsgenieSink {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &OpsgenieSink{apiKey: apiKey, url: opsgenieURL, client: &http.Client{Timeout: timeout}, minSev: minSeverity}
}

// Emit raises an Opsgenie alert for the event.
func (s *OpsgenieSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	payload := map[string]any{
		"message":  fmt.Sprintf("Heramba: %s from %s", e.Kind, e.Source),
		"alias":    fmt.Sprintf("heramba-%s-%s", e.Kind, e.Source),
		"priority": ogPriority(e.Severity),
		"source":   "heramba",
		"details": map[string]any{
			"kind": string(e.Kind), "source": e.Source, "target": e.Target,
			"severity": e.SeverityS, "confidence": fmt.Sprintf("%.2f", e.Confidence),
		},
	}
	body, err := json.Marshal(payload)
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
	req.Header.Set("Authorization", "GenieKey "+s.apiKey)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("opsgenie: non-2xx status %d", resp.StatusCode)
	}
	return nil
}

// ogPriority maps to Opsgenie P1..P5.
func ogPriority(s event.Severity) string {
	switch s {
	case event.SeverityCritical:
		return "P1"
	case event.SeverityHigh:
		return "P2"
	case event.SeverityMedium:
		return "P3"
	case event.SeverityLow:
		return "P4"
	default:
		return "P5"
	}
}
