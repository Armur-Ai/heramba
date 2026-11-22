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

// SplunkSink ships detections to a Splunk HTTP Event Collector (HEC). The event
// is wrapped in HEC's envelope ({time, sourcetype, event}) and authenticated
// with the HEC token. Fail-soft and severity-floored like the other push sinks.
type SplunkSink struct {
	url    string // full HEC collector URL, e.g. https://host:8088/services/collector/event
	token  string
	client *http.Client
	minSev event.Severity
}

// NewSplunk creates a Splunk HEC sink. Only events at or above minSeverity are shipped.
func NewSplunk(url, token string, timeout time.Duration, minSeverity event.Severity) *SplunkSink {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &SplunkSink{url: url, token: token, client: &http.Client{Timeout: timeout}, minSev: minSeverity}
}

// Emit wraps the event in the HEC envelope and posts it.
func (s *SplunkSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	payload := map[string]any{
		"time":       e.Time.Unix(),
		"sourcetype": "heramba",
		"source":     "heramba",
		"event":      e,
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
	req.Header.Set("Authorization", "Splunk "+s.token)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("splunk: non-2xx status %d", resp.StatusCode)
	}
	return nil
}
