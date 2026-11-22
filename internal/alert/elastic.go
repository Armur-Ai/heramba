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

// ElasticSink indexes detections into an Elasticsearch index as documents with
// an @timestamp, so they are immediately searchable in Kibana. Optional API-key
// auth. Fail-soft and severity-floored.
type ElasticSink struct {
	base   string // Elasticsearch base URL, e.g. https://es:9200
	index  string // index name, e.g. heramba-detections
	apiKey string // optional; sent as "Authorization: ApiKey <key>"
	client *http.Client
	minSev event.Severity
}

// NewElastic creates an Elasticsearch sink. index defaults to "heramba-detections".
func NewElastic(base, index, apiKey string, timeout time.Duration, minSeverity event.Severity) *ElasticSink {
	if index == "" {
		index = "heramba-detections"
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &ElasticSink{
		base:   strings.TrimRight(base, "/"),
		index:  index,
		apiKey: apiKey,
		client: &http.Client{Timeout: timeout},
		minSev: minSeverity,
	}
}

// Emit indexes the event as a new document.
func (s *ElasticSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	doc := map[string]any{
		"@timestamp": e.Time.UTC().Format(time.RFC3339Nano),
		"kind":       e.Kind,
		"severity":   e.SeverityS,
		"source":     e.Source,
		"target":     e.Target,
		"confidence": e.Confidence,
		"detail":     e.Detail,
		"labels":     e.Labels,
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/%s/_doc", s.base, s.index)
	ctx, cancel := context.WithTimeout(context.Background(), s.client.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+s.apiKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("elastic: non-2xx status %d", resp.StatusCode)
	}
	return nil
}
