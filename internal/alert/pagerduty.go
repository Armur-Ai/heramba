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

const pagerDutyURL = "https://events.pagerduty.com/v2/enqueue"

// PagerDutySink pages via the PagerDuty Events API v2. Paging is disruptive, so
// the default floor is intentionally high — wire it for the events that should
// wake someone (canary hits, blocked exfil), not routine observe-mode noise.
type PagerDutySink struct {
	routingKey string
	url        string
	client     *http.Client
	minSev     event.Severity
}

// NewPagerDuty creates a PagerDuty sink with the given integration routing key.
func NewPagerDuty(routingKey string, timeout time.Duration, minSeverity event.Severity) *PagerDutySink {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &PagerDutySink{routingKey: routingKey, url: pagerDutyURL, client: &http.Client{Timeout: timeout}, minSev: minSeverity}
}

// Emit triggers a PagerDuty incident for the event.
func (s *PagerDutySink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	payload := map[string]any{
		"routing_key":  s.routingKey,
		"event_action": "trigger",
		"dedup_key":    fmt.Sprintf("heramba-%s-%s", e.Kind, e.Source),
		"payload": map[string]any{
			"summary":  fmt.Sprintf("Heramba: %s from %s (%s)", e.Kind, e.Source, e.SeverityS),
			"source":   e.Source,
			"severity": pdSeverity(e.Severity),
			"custom_details": map[string]any{
				"kind": e.Kind, "target": e.Target, "confidence": e.Confidence,
				"detail": e.Detail, "labels": e.Labels,
			},
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
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("pagerduty: non-2xx status %d", resp.StatusCode)
	}
	return nil
}

// pdSeverity maps to PagerDuty's allowed severities.
func pdSeverity(s event.Severity) string {
	switch s {
	case event.SeverityCritical:
		return "critical"
	case event.SeverityHigh:
		return "error"
	case event.SeverityMedium:
		return "warning"
	default:
		return "info"
	}
}
