package alert

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestJSONLSinkEmitsOnePerLine(t *testing.T) {
	var buf bytes.Buffer
	s := NewJSONL(&buf)
	_ = s.Emit(event.Deception(event.KindDecoyHit, "1.1.1.1", "/decoy", "hit"))
	_ = s.Emit(event.New(event.KindProbe, event.SeverityLow, "2.2.2.2", "/real", "probe"))
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d", len(lines))
	}
	var e event.Event
	if err := json.Unmarshal([]byte(lines[0]), &e); err != nil {
		t.Fatalf("line 0 not valid JSON: %v", err)
	}
	if e.Confidence != 1.0 {
		t.Fatalf("decoy confidence = %v", e.Confidence)
	}
}

func TestTextSinkHumanReadable(t *testing.T) {
	var buf bytes.Buffer
	_ = NewText(&buf).Emit(event.Deception(event.KindCanaryTriggered, "3.3.3.3", "tok-abc", "canary used"))
	out := buf.String()
	if !strings.Contains(out, "high") || !strings.Contains(out, "canary_triggered") || !strings.Contains(out, "tok-abc") {
		t.Fatalf("text output missing fields: %q", out)
	}
}

type countSink struct{ n int }

func (c *countSink) Emit(event.Event) error { c.n++; return nil }

func TestMultiSinkFansOut(t *testing.T) {
	a, b := &countSink{}, &countSink{}
	m := MultiSink{a, b, Discard}
	_ = m.Emit(event.New(event.KindProbe, event.SeverityInfo, "", "", ""))
	if a.n != 1 || b.n != 1 {
		t.Fatalf("fan-out failed: a=%d b=%d", a.n, b.n)
	}
}
