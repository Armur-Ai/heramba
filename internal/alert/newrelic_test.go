package alert

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestNewRelicSinkPostsLogRecord(t *testing.T) {
	var got []map[string]any
	var apiKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey = r.Header.Get("Api-Key")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	s := NewNewRelic(srv.URL, "nr-key-123", time.Second, event.SeverityLow)
	e := event.Deception(event.KindDecoyHit, "10.0.0.1", "/.env", "probe")
	if err := s.Emit(e); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if apiKey != "nr-key-123" {
		t.Errorf("Api-Key header = %q, want nr-key-123", apiKey)
	}
	if len(got) != 1 {
		t.Fatalf("expected one log record, got %v", got)
	}
	blob, _ := json.Marshal(got)
	for _, want := range []string{"decoy_hit", "10.0.0.1", "timestamp", "attributes", "heramba"} {
		if !strings.Contains(string(blob), want) {
			t.Errorf("payload missing %q: %s", want, blob)
		}
	}
}

func TestNewRelicDefaultURL(t *testing.T) {
	s := NewNewRelic("", "k", time.Second, event.SeverityInfo)
	if s.url != defaultNewRelicURL {
		t.Errorf("empty url should default to US endpoint, got %q", s.url)
	}
}

func TestNewRelicSinkFloorAndError(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	s := NewNewRelic(srv.URL, "k", time.Second, event.SeverityHigh)
	if err := s.Emit(event.New(event.KindProbe, event.SeverityLow, "x", "/x", "d")); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("below-floor event should be dropped, got %d", calls)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	s2 := NewNewRelic(bad.URL, "k", time.Second, event.SeverityInfo)
	if err := s2.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "x", "y", "d")); err == nil {
		t.Fatal("expected error on 500")
	}
}
