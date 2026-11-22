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

// TeamsSink posts each detection to a Microsoft Teams incoming webhook as a
// legacy MessageCard — a themed color bar, a title, and who/what/confidence
// facts. It is the Teams-native analogue of SlackSink: a real alert in the
// channel the team already watches, not a log line.
//
// Like the other push sinks it is fail-soft (an error is returned but the
// engine never lets a telemetry failure break traffic) and honors a minimum-
// severity floor so a busy observe-mode edge does not flood the channel.
type TeamsSink struct {
	url    string
	client *http.Client
	minSev event.Severity
}

// NewTeams creates a Teams sink posting to a Teams incoming-webhook URL. Only
// events at or above minSeverity are sent.
func NewTeams(url string, timeout time.Duration, minSeverity event.Severity) *TeamsSink {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &TeamsSink{url: url, client: &http.Client{Timeout: timeout}, minSev: minSeverity}
}

// Emit renders the event as a MessageCard and posts it when it meets the floor.
func (s *TeamsSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	body, err := json.Marshal(TeamsMessage(e))
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
		return fmt.Errorf("teams: non-2xx status %d", resp.StatusCode)
	}
	return nil
}

// --- MessageCard payload (typed, so the shape is unit-testable) -------------

type teamsCard struct {
	Type       string         `json:"@type"`
	Context    string         `json:"@context"`
	ThemeColor string         `json:"themeColor"`
	Summary    string         `json:"summary"`
	Sections   []teamsSection `json:"sections"`
}
type teamsSection struct {
	ActivityTitle string      `json:"activityTitle"`
	Facts         []teamsFact `json:"facts"`
	Markdown      bool        `json:"markdown"`
}
type teamsFact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// TeamsMessage builds the MessageCard payload for an event. Exported so tests
// (and callers who want to post via their own transport) can inspect it.
func TeamsMessage(e event.Event) any {
	sev := e.Severity.String()
	headline := fmt.Sprintf("Heramba — %s (%s)", e.Kind, sev)

	facts := []teamsFact{
		{Name: "Source", Value: orDash(e.Source)},
		{Name: "Target", Value: orDash(e.Target)},
		{Name: "Confidence", Value: fmt.Sprintf("%.2f", e.Confidence)},
	}
	if e.Detail != "" {
		facts = append(facts, teamsFact{Name: "Detail", Value: e.Detail})
	}
	if c := e.Labels["campaign"]; c != "" {
		facts = append(facts, teamsFact{Name: "Campaign", Value: c})
	}
	facts = append(facts, teamsFact{Name: "Time", Value: e.Time.UTC().Format(time.RFC3339)})

	return teamsCard{
		Type:       "MessageCard",
		Context:    "http://schema.org/extensions",
		ThemeColor: teamsColor(e.Severity),
		Summary:    headline,
		Sections: []teamsSection{{
			ActivityTitle: headline,
			Facts:         facts,
			Markdown:      true,
		}},
	}
}

// teamsColor is the hex theme color (no leading #, as MessageCard expects).
func teamsColor(s event.Severity) string {
	switch s {
	case event.SeverityCritical:
		return "ff6b6b"
	case event.SeverityHigh:
		return "f5a524"
	case event.SeverityMedium:
		return "a78bfa"
	case event.SeverityLow:
		return "57c7ff"
	default:
		return "5c6577"
	}
}
