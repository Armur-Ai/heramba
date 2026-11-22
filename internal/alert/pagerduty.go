package alert

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

const pagerDutyURL = "https://events.pagerduty.com/v2/enqueue"

// PagerDutySink pages via the PagerDuty Events API v2. Paging is disruptive, so
// the default floor is intentionally high — wire it for the events that should
// wake someone (canary hits, blocked exfil), not routine observe-mode noise.
type PagerDutySink struct {
	*httpSender
	routingKey string
	url        string
	minSev     event.Severity
}

// NewPagerDuty creates a PagerDuty sink with the given integration routing key.
func NewPagerDuty(routingKey string, timeout time.Duration, minSeverity event.Severity) *PagerDutySink {
	return &PagerDutySink{httpSender: newHTTPSender(timeout), routingKey: routingKey, url: pagerDutyURL, minSev: minSeverity}
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
	return s.deliver("pagerduty", httpPost{url: s.url, body: body, contentType: "application/json"})
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
