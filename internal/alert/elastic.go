package alert

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// ElasticSink indexes detections into an Elasticsearch index as documents with
// an @timestamp, so they are immediately searchable in Kibana. Optional API-key
// auth. Fail-soft and severity-floored.
type ElasticSink struct {
	*httpSender
	base   string // Elasticsearch base URL, e.g. https://es:9200
	index  string // index name, e.g. heramba-detections
	apiKey string // optional; sent as "Authorization: ApiKey <key>"
	minSev event.Severity
}

// NewElastic creates an Elasticsearch sink. index defaults to "heramba-detections".
func NewElastic(base, index, apiKey string, timeout time.Duration, minSeverity event.Severity) *ElasticSink {
	if index == "" {
		index = "heramba-detections"
	}
	return &ElasticSink{
		httpSender: newHTTPSender(timeout),
		base:       strings.TrimRight(base, "/"),
		index:      index,
		apiKey:     apiKey,
		minSev:     minSeverity,
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
	p := httpPost{url: url, body: body, contentType: "application/json"}
	if s.apiKey != "" {
		p.headers = map[string]string{"Authorization": "ApiKey " + s.apiKey}
	}
	return s.deliver("elastic", p)
}
