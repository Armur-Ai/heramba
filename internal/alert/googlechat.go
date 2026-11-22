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

// GoogleChatSink posts each detection to a Google Chat incoming webhook as a
// simple text message with the key detail inline. Fail-soft and severity-floored
// like the other chat sinks.
type GoogleChatSink struct {
	url    string
	client *http.Client
	minSev event.Severity
}

// NewGoogleChat creates a Google Chat sink posting to a space webhook URL.
func NewGoogleChat(url string, timeout time.Duration, minSeverity event.Severity) *GoogleChatSink {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &GoogleChatSink{url: url, client: &http.Client{Timeout: timeout}, minSev: minSeverity}
}

// Emit posts the event as a Google Chat text message.
func (s *GoogleChatSink) Emit(e event.Event) error {
	if e.Severity < s.minSev {
		return nil
	}
	body, err := json.Marshal(GoogleChatMessage(e))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.client.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("googlechat: non-2xx status %d", resp.StatusCode)
	}
	return nil
}

type googleChatMsg struct {
	Text string `json:"text"`
}

// GoogleChatMessage builds the payload for an event. Exported for tests.
func GoogleChatMessage(e event.Event) any {
	text := fmt.Sprintf("*Heramba — %s (%s)*\nsource: `%s`  target: `%s`  confidence: %.2f",
		e.Kind, e.Severity.String(), orDash(e.Source), orDash(e.Target), e.Confidence)
	if e.Detail != "" {
		text += "\n" + e.Detail
	}
	if c := e.Labels["campaign"]; c != "" {
		text += "\ncampaign: " + c
	}
	return googleChatMsg{Text: text}
}
