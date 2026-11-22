package alert

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// GoogleChatSink posts each detection to a Google Chat incoming webhook as a
// simple text message with the key detail inline. Fail-soft and severity-floored
// like the other chat sinks.
type GoogleChatSink struct {
	*httpSender
	url    string
	minSev event.Severity
}

// NewGoogleChat creates a Google Chat sink posting to a space webhook URL.
func NewGoogleChat(url string, timeout time.Duration, minSeverity event.Severity) *GoogleChatSink {
	return &GoogleChatSink{httpSender: newHTTPSender(timeout), url: url, minSev: minSeverity}
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
	return s.deliver("googlechat", httpPost{url: s.url, body: body, contentType: "application/json; charset=UTF-8"})
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
