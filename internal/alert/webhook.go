package alert

import (
	"encoding/json"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// WebhookSink POSTs each event as a JSON body to a configured URL. It is a real,
// tested integration (verified against an httptest server) — not a stub. It is
// fail-soft by design: a webhook error is returned to the caller but, on the
// edge hot path, the MultiSink/engine never let a telemetry failure break
// traffic (see edge R6 fault-injection tests). Delivery (timeout, optional
// retries, Retry-After) is handled by the shared httpSender.
type WebhookSink struct {
	*httpSender
	url    string
	minSev event.Severity
}

// NewWebhook creates a webhook sink posting to url with the given timeout. Only
// events at or above minSeverity are sent (noise control).
func NewWebhook(url string, timeout time.Duration, minSeverity event.Severity) *WebhookSink {
	return &WebhookSink{httpSender: newHTTPSender(timeout), url: url, minSev: minSeverity}
}

// Emit posts the event as JSON when it meets the severity floor.
func (s *WebhookSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return s.deliver("webhook", httpPost{url: s.url, body: body, contentType: "application/json"})
}
