package alert

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestGELFSinkPostsMessage(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	s := NewGELF(srv.URL, "edge-1", time.Second, event.SeverityLow)
	e := event.Deception(event.KindDecoyHit, "10.0.0.1", "/.env", "probe")
	if err := s.Emit(e); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if got["version"] != "1.1" {
		t.Errorf("GELF version = %v, want 1.1", got["version"])
	}
	if got["host"] != "edge-1" {
		t.Errorf("GELF host = %v, want edge-1", got["host"])
	}
	// Custom fields must be underscore-prefixed.
	if got["_kind"] != "decoy_hit" {
		t.Errorf("_kind = %v, want decoy_hit", got["_kind"])
	}
	if _, ok := got["timestamp"]; !ok {
		t.Error("GELF message must have a timestamp")
	}
}

func TestGELFLevelMapping(t *testing.T) {
	cases := map[event.Severity]int{
		event.SeverityCritical: 2,
		event.SeverityHigh:     3,
		event.SeverityMedium:   4,
		event.SeverityLow:      5,
		event.SeverityInfo:     6,
	}
	for sev, want := range cases {
		if got := gelfLevel(sev); got != want {
			t.Errorf("gelfLevel(%v) = %d, want %d", sev, got, want)
		}
	}
}

func TestGELFDefaultHost(t *testing.T) {
	m := GELFMessage(event.New(event.KindProbe, event.SeverityInfo, "", "", ""), "")
	// NewGELF applies the default; GELFMessage takes host verbatim, so check NewGELF.
	s := NewGELF("http://x/gelf", "", time.Second, event.SeverityInfo)
	if s.host != "heramba" {
		t.Errorf("empty host should default to heramba, got %q", s.host)
	}
	if m["version"] != "1.1" {
		t.Errorf("message should still be GELF 1.1")
	}
}

func TestGELFSinkFloorAndError(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	s := NewGELF(srv.URL, "h", time.Second, event.SeverityHigh)
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
	s2 := NewGELF(bad.URL, "h", time.Second, event.SeverityInfo)
	if err := s2.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "x", "y", "d")); err == nil {
		t.Fatal("expected error on 500")
	}
}
