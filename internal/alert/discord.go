package alert

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// DiscordSink posts each detection to a Discord webhook as a rich embed — a
// colored sidebar keyed to severity, a title, and who/what/confidence fields.
// It is the Discord-native analogue of SlackSink/TeamsSink.
//
// Like the other push sinks it is fail-soft (an error is returned but the
// engine never lets a telemetry failure break traffic) and honors a minimum-
// severity floor so a busy observe-mode edge does not flood the channel.
//
// Note: Discord returns 204 No Content on a successful webhook post, which the
// shared "< 300 is success" rule already accepts.
type DiscordSink struct {
	*httpSender
	url    string
	minSev event.Severity
}

// NewDiscord creates a Discord sink posting to a Discord webhook URL. Only
// events at or above minSeverity are sent.
func NewDiscord(url string, timeout time.Duration, minSeverity event.Severity) *DiscordSink {
	return &DiscordSink{httpSender: newHTTPSender(timeout), url: url, minSev: minSeverity}
}

// Emit renders the event as an embed and posts it when it meets the floor.
func (s *DiscordSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	body, err := json.Marshal(DiscordMessage(e))
	if err != nil {
		return err
	}
	return s.deliver("discord", httpPost{url: s.url, body: body, contentType: "application/json"})
}

// --- Discord webhook payload (typed, so the shape is unit-testable) ---------

type discordMsg struct {
	Content string         `json:"content"`
	Embeds  []discordEmbed `json:"embeds"`
}
type discordEmbed struct {
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Color       int            `json:"color"`
	Fields      []discordField `json:"fields"`
	Timestamp   string         `json:"timestamp"`
}
type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// DiscordMessage builds the webhook payload for an event. Exported so tests
// (and callers who want to post via their own transport) can inspect it.
func DiscordMessage(e event.Event) any {
	sev := e.Severity.String()
	title := fmt.Sprintf("Heramba — %s (%s)", e.Kind, sev)

	fields := []discordField{
		{Name: "Source", Value: orDash(e.Source), Inline: true},
		{Name: "Target", Value: orDash(e.Target), Inline: true},
		{Name: "Confidence", Value: fmt.Sprintf("%.2f", e.Confidence), Inline: true},
	}
	if c := e.Labels["campaign"]; c != "" {
		fields = append(fields, discordField{Name: "Campaign", Value: c, Inline: true})
	}

	return discordMsg{
		Content: title,
		Embeds: []discordEmbed{{
			Title:       title,
			Description: e.Detail,
			Color:       discordColor(e.Severity),
			Fields:      fields,
			Timestamp:   e.Time.UTC().Format(time.RFC3339),
		}},
	}
}

// discordColor is the embed sidebar color as a decimal integer (Discord's
// format). Values mirror the Slack/Teams severity palette.
func discordColor(s event.Severity) int {
	switch s {
	case event.SeverityCritical:
		return 0xff6b6b
	case event.SeverityHigh:
		return 0xf5a524
	case event.SeverityMedium:
		return 0xa78bfa
	case event.SeverityLow:
		return 0x57c7ff
	default:
		return 0x5c6577
	}
}
