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

// MattermostSink posts each detection to a Mattermost incoming webhook. The
// payload uses Mattermost's text + attachment shape (a colored attachment with
// the detection fields), which renders as a tidy card in the channel. Fail-soft
// and severity-floored like the other chat sinks.
type MattermostSink struct {
	url    string
	client *http.Client
	minSev event.Severity
}

// NewMattermost creates a Mattermost sink posting to an incoming-webhook URL.
func NewMattermost(url string, timeout time.Duration, minSeverity event.Severity) *MattermostSink {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &MattermostSink{url: url, client: &http.Client{Timeout: timeout}, minSev: minSeverity}
}

// Emit posts the event as a Mattermost message.
func (s *MattermostSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	body, err := json.Marshal(MattermostMessage(e))
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
		return fmt.Errorf("mattermost: non-2xx status %d", resp.StatusCode)
	}
	return nil
}

type mattermostMsg struct {
	Text        string                 `json:"text"`
	Username    string                 `json:"username"`
	Attachments []mattermostAttachment `json:"attachments"`
}
type mattermostAttachment struct {
	Color  string            `json:"color"`
	Title  string            `json:"title"`
	Text   string            `json:"text,omitempty"`
	Fields []mattermostField `json:"fields"`
}
type mattermostField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// MattermostMessage builds the payload for an event. Exported for tests.
func MattermostMessage(e event.Event) any {
	title := fmt.Sprintf("Heramba — %s (%s)", e.Kind, e.Severity.String())
	fields := []mattermostField{
		{Title: "Source", Value: orDash(e.Source), Short: true},
		{Title: "Target", Value: orDash(e.Target), Short: true},
		{Title: "Confidence", Value: fmt.Sprintf("%.2f", e.Confidence), Short: true},
	}
	if c := e.Labels["campaign"]; c != "" {
		fields = append(fields, mattermostField{Title: "Campaign", Value: c, Short: true})
	}
	return mattermostMsg{
		Text:     title,
		Username: "heramba",
		Attachments: []mattermostAttachment{{
			Color:  mattermostColor(e.Severity),
			Title:  title,
			Text:   e.Detail,
			Fields: fields,
		}},
	}
}

// mattermostColor reuses the shared severity palette (hex with leading #).
func mattermostColor(s event.Severity) string {
	switch s {
	case event.SeverityCritical:
		return "#ff6b6b"
	case event.SeverityHigh:
		return "#f5a524"
	case event.SeverityMedium:
		return "#a78bfa"
	case event.SeverityLow:
		return "#57c7ff"
	default:
		return "#5c6577"
	}
}
