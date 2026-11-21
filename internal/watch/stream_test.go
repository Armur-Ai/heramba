package watch

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/alert"
	"github.com/Armur-Ai/heramba/internal/event"
)

// jsonlFor renders events exactly as the alert JSONL sink would, so the tests
// exercise the real wire format the dashboard consumes in production.
func jsonlFor(t *testing.T, events ...event.Event) string {
	t.Helper()
	var sb strings.Builder
	sink := alert.NewJSONL(&sb)
	for _, e := range events {
		if err := sink.Emit(e); err != nil {
			t.Fatalf("emit: %v", err)
		}
	}
	return sb.String()
}

func sampleEvents() []event.Event {
	return []event.Event{
		event.Deception(event.KindDecoyHit, "10.0.0.9", "/.env", "decoy hit"),
		event.New(event.KindAgenticTraffic, event.SeverityMedium, "10.0.0.9", "/api/users", "agentic score 0.82"),
		event.Deception(event.KindCanaryTriggered, "10.0.0.9", "canary-7f3a", "token used"),
	}
}

func TestReadAllDecodesSinkFormat(t *testing.T) {
	data := jsonlFor(t, sampleEvents()...)
	got, err := ReadAll(strings.NewReader(data))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 events, got %d", len(got))
	}
	if got[0].Kind != event.KindDecoyHit || got[0].Confidence != 1.0 {
		t.Errorf("first event wrong: %+v", got[0])
	}
}

func TestReadAllSkipsBlankAndMalformed(t *testing.T) {
	data := jsonlFor(t, sampleEvents()...)
	polluted := "\n" + data + "\n{ not json }\n   \n"
	got, err := ReadAll(strings.NewReader(polluted))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("blank/garbage lines should be skipped; got %d", len(got))
	}
}

func TestStreamDeliversThenCloses(t *testing.T) {
	data := jsonlFor(t, sampleEvents()...)
	ch := Stream(context.Background(), strings.NewReader(data))
	var n int
	for range ch {
		n++
	}
	if n != 3 {
		t.Fatalf("want 3 streamed events, got %d", n)
	}
}

func TestStreamDeliversNothingAfterCancel(t *testing.T) {
	// Contract: once ctx is cancelled, Stream delivers no further events and the
	// channel closes. (It cannot interrupt a read already blocked on the source
	// — only EOF/close does that — so we drive the source ourselves.)
	pr, pw := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	ch := Stream(ctx, pr)

	// Deliver one event, confirm it arrives.
	go func() { _, _ = pw.Write([]byte(jsonlFor(t, sampleEvents()[:1]...))) }()
	select {
	case e := <-ch:
		if e.Kind != event.KindDecoyHit {
			t.Fatalf("unexpected first event: %+v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first event never arrived")
	}

	// Cancel, then push more lines and close; nothing further must be delivered.
	cancel()
	go func() {
		_, _ = pw.Write([]byte(jsonlFor(t, sampleEvents()...)))
		_ = pw.Close()
	}()
	for e := range ch {
		t.Fatalf("event delivered after cancel: %+v", e)
	}
}

func TestTailFileFollowsAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := TailFile(ctx, path, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	// Append after the tail has started.
	go func() {
		time.Sleep(40 * time.Millisecond)
		f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600) //nolint:errcheck
		_, _ = f.WriteString(jsonlFor(t, sampleEvents()...))
		_ = f.Close()
	}()
	var got []event.Event
	timeout := time.After(3 * time.Second)
	for len(got) < 3 {
		select {
		case e := <-ch:
			got = append(got, e)
		case <-timeout:
			t.Fatalf("TailFile only saw %d/3 appended events", len(got))
		}
	}
}

func TestTailFileWaitsForMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "later.jsonl")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := TailFile(ctx, path, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("TailFile on missing path should not error: %v", err)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(path, []byte(jsonlFor(t, sampleEvents()[:1]...)), 0o600)
	}()
	select {
	case e := <-ch:
		if e.Kind != event.KindDecoyHit {
			t.Errorf("unexpected event: %+v", e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("TailFile never picked up the late-created file")
	}
}
