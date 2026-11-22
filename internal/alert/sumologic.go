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

// SumoLogicSink ships detections to a Sumo Logic HTTP Source. The collector URL
// is itself the secret (it embeds the source token), so the sink needs no extra
// auth: it simply POSTs the event as a JSON object, which Sumo ingests as a
// structured log. Fail-soft and severity-floored like the other push sinks.
type SumoLogicSink struct {
	url    string
	client *http.Client
	minSev event.Severity
}

// NewSumoLogic creates a Sumo Logic sink posting to an HTTP Source URL.
func NewSumoLogic(url string, timeout time.Duration, minSeverity event.Severity) *SumoLogicSink {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &SumoLogicSink{url: url, client: &http.Client{Timeout: timeout}, minSev: minSeverity}
}

// Emit posts the event as a JSON log line.
func (s *SumoLogicSink) Emit(e event.Event) error {
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
	req.Header.Set("X-Sumo-Category", "heramba/detections")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("sumologic: non-2xx status %d", resp.StatusCode)
	}
	return nil
}
