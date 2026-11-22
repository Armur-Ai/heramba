package alert

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// TemplateWebhookSink POSTs a body rendered from an operator-supplied Go
// text/template to a URL, so any SIEM or service with its own expected payload
// shape can be targeted without a bespoke sink. The template is executed with
// the event.Event as its data (e.g. `{"kind":"{{.Kind}}","src":"{{.Source}}"}`).
// Fail-soft and severity-floored like the other push sinks.
type TemplateWebhookSink struct {
	*httpSender
	url         string
	tmpl        *template.Template
	contentType string
	minSev      event.Severity
}

// NewTemplateWebhook parses tmplStr and returns a sink. contentType defaults to
// application/json when empty. A malformed template is reported here (at build
// time), not silently on every event.
func NewTemplateWebhook(url, tmplStr, contentType string, timeout time.Duration, minSeverity event.Severity) (*TemplateWebhookSink, error) {
	if tmplStr == "" {
		return nil, fmt.Errorf("template_webhook: template is required")
	}
	t, err := template.New("webhook").Option("missingkey=zero").Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("template_webhook: parse template: %w", err)
	}
	if contentType == "" {
		contentType = "application/json"
	}
	return &TemplateWebhookSink{
		httpSender: newHTTPSender(timeout),
		url:        url, tmpl: t, contentType: contentType,
		minSev: minSeverity,
	}, nil
}

// Render executes the template for an event, returning the body bytes. Exported
// so callers/tests can inspect the rendered payload without a transport.
func (s *TemplateWebhookSink) Render(e event.Event) ([]byte, error) {
	var buf bytes.Buffer
	if err := s.tmpl.Execute(&buf, e); err != nil {
		return nil, fmt.Errorf("template_webhook: render: %w", err)
	}
	return buf.Bytes(), nil
}

// Emit renders the template and posts the result when the event meets the floor.
func (s *TemplateWebhookSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	body, err := s.Render(e)
	if err != nil {
		return err
	}
	return s.deliver("template_webhook", httpPost{url: s.url, body: body, contentType: s.contentType})
}
