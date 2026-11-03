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

// WebhookSink POSTs each event as a JSON body to a configured URL. It is a real,
// tested integration (verified against an httptest server) — not a stub. It is
// fail-soft by design: a webhook error is returned to the caller but, on the
// edge hot path, the MultiSink/engine never let a telemetry failure break
// traffic (see edge R6 fault-injection tests).
type WebhookSink struct {
	url    string
	client *http.Client
	minSev event.Severity
}

// NewWebhook creates a webhook sink posting to url with the given timeout. Only
// events at or above minSeverity are sent (noise control).
func NewWebhook(url string, timeout time.Duration, minSeverity event.Severity) *WebhookSink {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &WebhookSink{url: url, client: &http.Client{Timeout: timeout}, minSev: minSeverity}
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
		return fmt.Errorf("webhook: non-2xx status %d", resp.StatusCode)
	}
	return nil
}
