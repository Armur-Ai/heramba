package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// LokiSink ships detections to a Grafana Loki push API (/loki/api/v1/push). Each
// event becomes one log entry on a stream labeled by app/kind/severity (kept
// low-cardinality on purpose — the mutable detail rides in the log line, not the
// labels). Fail-soft and severity-floored like the other push sinks. An optional
// tenant sets the X-Scope-OrgID header for multi-tenant Loki.
type LokiSink struct {
	url    string
	tenant string
	client *http.Client
	minSev event.Severity
}

// NewLoki creates a Loki sink. url is the full push endpoint, e.g.
// https://loki:3100/loki/api/v1/push. tenant may be "".
func NewLoki(url, tenant string, timeout time.Duration, minSeverity event.Severity) *LokiSink {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &LokiSink{url: url, tenant: tenant, client: &http.Client{Timeout: timeout}, minSev: minSeverity}
}

// Emit posts the event as a single Loki stream entry.
func (s *LokiSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	body, err := json.Marshal(LokiPayload(e))
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
	if s.tenant != "" {
		req.Header.Set("X-Scope-OrgID", s.tenant)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("loki: non-2xx status %d", resp.StatusCode)
	}
	return nil
}

type lokiPush struct {
	Streams []lokiStream `json:"streams"`
}
type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"`
}

// LokiPayload builds the push body for an event. Exported for tests.
func LokiPayload(e event.Event) any {
	line, _ := json.Marshal(e)
	ts := strconv.FormatInt(e.Time.UTC().UnixNano(), 10)
	return lokiPush{
		Streams: []lokiStream{{
			Stream: map[string]string{
				"app":      "heramba",
				"kind":     string(e.Kind),
				"severity": e.Severity.String(),
			},
			Values: [][2]string{{ts, string(line)}},
		}},
	}
}
