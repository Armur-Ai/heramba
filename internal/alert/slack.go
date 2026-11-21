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

// SlackSink posts each detection to a Slack incoming webhook as a rich Block Kit
// message — a colored severity bar, who/what/confidence fields, and a context
// line with the campaign and time. It is the "I feel protected" moment: a real
// alert in the channel the team already watches, not a log line.
//
// Like WebhookSink it is fail-soft (an error is returned but the engine never
// lets a telemetry failure break traffic) and honors a minimum-severity floor
// so a busy observe-mode edge does not flood the channel.
type SlackSink struct {
	url    string
	client *http.Client
	minSev event.Severity
}

// NewSlack creates a Slack sink posting to a Slack incoming-webhook URL. Only
// events at or above minSeverity are sent.
func NewSlack(url string, timeout time.Duration, minSeverity event.Severity) *SlackSink {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &SlackSink{url: url, client: &http.Client{Timeout: timeout}, minSev: minSeverity}
}

// Emit renders the event as Block Kit and posts it when it meets the floor.
func (s *SlackSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	body, err := json.Marshal(SlackMessage(e))
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
		return fmt.Errorf("slack: non-2xx status %d", resp.StatusCode)
	}
	return nil
}

// --- Block Kit payload (typed, so the shape is unit-testable) ---------------

type slackMsg struct {
	Text        string            `json:"text"` // fallback/notification text
	Attachments []slackAttachment `json:"attachments"`
}
type slackAttachment struct {
	Color  string       `json:"color"`
	Blocks []slackBlock `json:"blocks"`
}
type slackBlock struct {
	Type     string      `json:"type"`
	Text     *slackText  `json:"text,omitempty"`
	Fields   []slackText `json:"fields,omitempty"`
	Elements []slackText `json:"elements,omitempty"`
}
type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SlackMessage builds the Block Kit payload for an event. Exported so tests (and
// callers who want to post via their own transport) can inspect it.
func SlackMessage(e event.Event) any {
	sev := e.Severity.String()
	headline := fmt.Sprintf("%s Heramba — %s (%s)", slackEmoji(e.Severity), e.Kind, sev)

	fields := []slackText{
		{Type: "mrkdwn", Text: "*Source:*\n`" + orDash(e.Source) + "`"},
		{Type: "mrkdwn", Text: "*Target:*\n`" + orDash(e.Target) + "`"},
		{Type: "mrkdwn", Text: fmt.Sprintf("*Confidence:*\n%.2f", e.Confidence)},
	}
	if e.Detail != "" {
		fields = append(fields, slackText{Type: "mrkdwn", Text: "*Detail:*\n" + e.Detail})
	}

	ctxLine := e.Time.UTC().Format(time.RFC3339)
	if c := e.Labels["campaign"]; c != "" {
		ctxLine = "campaign `" + c + "` · " + ctxLine
	}

	blocks := []slackBlock{
		{Type: "header", Text: &slackText{Type: "plain_text", Text: headline}},
		{Type: "section", Fields: fields},
		{Type: "context", Elements: []slackText{{Type: "mrkdwn", Text: ctxLine}}},
	}
	return slackMsg{
		Text:        headline, // notification/fallback text
		Attachments: []slackAttachment{{Color: slackColor(e.Severity), Blocks: blocks}},
	}
}

func slackColor(s event.Severity) string {
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

func slackEmoji(s event.Severity) string {
	switch s {
	case event.SeverityCritical:
		return "🔴"
	case event.SeverityHigh:
		return "🟠"
	case event.SeverityMedium:
		return "🟣"
	case event.SeverityLow:
		return "🔵"
	default:
		return "⚪"
	}
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
