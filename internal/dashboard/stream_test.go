package dashboard

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

// readFrame reads one SSE `data: {...}` frame from r, skipping comment lines
// (": ...") and blank separators. It returns the decoded JSON payload.
func readFrame(t *testing.T, r *bufio.Reader) StreamFrame {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("read stream: %v", err)
		}
		line = strings.TrimRight(line, "\n")
		if line == "" || strings.HasPrefix(line, ":") {
			continue // separator or heartbeat/comment
		}
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			t.Fatalf("frame line not a data event: %q", line)
		}
		var f StreamFrame
		if err := json.Unmarshal([]byte(data), &f); err != nil {
			t.Fatalf("decode frame %q: %v", data, err)
		}
		return f
	}
	t.Fatal("timed out waiting for an SSE frame")
	return StreamFrame{}
}

func TestStreamContentTypeAndPrimingFrame(t *testing.T) {
	b := NewBroker()
	h := testHandler(WithBroker(b))
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/stream: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("content-type = %q, want text/event-stream", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("cache-control = %q, want no-store", cc)
	}

	// The handler primes the connection with a metrics frame.
	br := bufio.NewReader(resp.Body)
	f := readFrame(t, br)
	if f.Type != "metrics" || f.Metrics == nil {
		t.Fatalf("priming frame = %+v, want a metrics frame", f)
	}
}

func TestStreamDeliversPublishedEvents(t *testing.T) {
	b := NewBroker()
	h := testHandler(WithBroker(b))
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/stream: %v", err)
	}
	defer resp.Body.Close()
	br := bufio.NewReader(resp.Body)
	_ = readFrame(t, br) // consume priming metrics frame

	// Wait until the handler has actually subscribed, then publish.
	waitFor(t, func() bool { return b.Subscribers() == 1 })
	ev := event.Deception(event.KindCanaryTriggered, "9.9.9.9", "/keys", "token replayed")
	b.Publish(EventFrame(ev))

	f := readFrame(t, br)
	if f.Type != "event" || f.Event == nil {
		t.Fatalf("frame = %+v, want an event frame", f)
	}
	if f.Event.Source != "9.9.9.9" || f.Event.Kind != event.KindCanaryTriggered {
		t.Errorf("event = %+v, want canary from 9.9.9.9", f.Event)
	}
}

func TestStreamUnsubscribesOnCancel(t *testing.T) {
	b := NewBroker()
	h := testHandler(WithBroker(b))
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/stream: %v", err)
	}
	br := bufio.NewReader(resp.Body)
	_ = readFrame(t, br)
	waitFor(t, func() bool { return b.Subscribers() == 1 })

	cancel() // client disconnects
	resp.Body.Close()

	// The handler must tear its subscription down (no leak).
	waitFor(t, func() bool { return b.Subscribers() == 0 })
	if n := b.Subscribers(); n != 0 {
		t.Fatalf("subscribers after cancel = %d, want 0 (leak)", n)
	}
}

// waitFor polls cond up to ~2s, failing the test if it never holds.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}
