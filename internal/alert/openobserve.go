package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// OpenObserveSink ships detections to an OpenObserve JSON ingest endpoint
// (POST {base}/api/{org}/{stream}/_json) as a JSON array of records, with HTTP
// basic auth (email + password/token). Fail-soft and severity-floored like the
// other push sinks.
type OpenObserveSink struct {
	endpoint string
	user     string
	pass     string
	client   *http.Client
	minSev   event.Severity
}

// NewOpenObserve creates an OpenObserve sink. base is the instance root (e.g.
// http://localhost:5080); org defaults to "default" and stream to "heramba".
func NewOpenObserve(base, org, stream, user, pass string, timeout time.Duration, minSeverity event.Severity) *OpenObserveSink {
	if org == "" {
		org = "default"
	}
	if stream == "" {
		stream = "heramba"
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	endpoint := strings.TrimRight(base, "/") + "/api/" + org + "/" + stream + "/_json"
	return &OpenObserveSink{endpoint: endpoint, user: user, pass: pass, client: &http.Client{Timeout: timeout}, minSev: minSeverity}
}

// Emit posts the event as a single-record JSON array.
func (s *OpenObserveSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	record := map[string]any{
		"_timestamp": e.Time.UnixMicro(), // OpenObserve's microsecond ingest time
		"service":    "heramba",
		"kind":       string(e.Kind),
		"severity":   e.SeverityS,
		"source":     e.Source,
		"target":     e.Target,
		"confidence": e.Confidence,
		"detail":     e.Detail,
	}
	body, err := json.Marshal([]any{record})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.client.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.user != "" || s.pass != "" {
		req.SetBasicAuth(s.user, s.pass)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("openobserve: non-2xx status %d", resp.StatusCode)
	}
	return nil
}
