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

func TestHoneycombSinkPostsEvent(t *testing.T) {
	var got map[string]any
	var team, path, eventTime string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		team = r.Header.Get("X-Honeycomb-Team")
		eventTime = r.Header.Get("X-Honeycomb-Event-Time")
		path = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewHoneycomb(srv.URL, "heramba-detections", "hc-key", time.Second, event.SeverityLow)
	e := event.Deception(event.KindCanaryTriggered, "10.0.0.2", "hrmb_x", "used").WithLabel("campaign", "sweep")
	if err := s.Emit(e); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if team != "hc-key" {
		t.Errorf("X-Honeycomb-Team = %q, want hc-key", team)
	}
	if eventTime == "" {
		t.Error("X-Honeycomb-Event-Time header should be set")
	}
	if path != "/1/events/heramba-detections" {
		t.Errorf("events path = %q, want /1/events/heramba-detections", path)
	}
	blob, _ := json.Marshal(got)
	for _, want := range []string{"canary_triggered", "10.0.0.2", "label.campaign", "sweep"} {
		if !strings.Contains(string(blob), want) {
			t.Errorf("payload missing %q: %s", want, blob)
		}
	}
}

func TestHoneycombDefaultBase(t *testing.T) {
	s := NewHoneycomb("", "ds", "k", time.Second, event.SeverityInfo)
	if !strings.HasPrefix(s.endpoint, defaultHoneycombURL+"/1/events/") {
		t.Errorf("empty base should default to US API, got %q", s.endpoint)
	}
}

func TestHoneycombSinkFloorAndError(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	s := NewHoneycomb(srv.URL, "ds", "k", time.Second, event.SeverityHigh)
	if err := s.Emit(event.New(event.KindProbe, event.SeverityLow, "x", "/x", "d")); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("below-floor event should be dropped, got %d", calls)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer bad.Close()
	s2 := NewHoneycomb(bad.URL, "ds", "k", time.Second, event.SeverityInfo)
	if err := s2.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "x", "y", "d")); err == nil {
		t.Fatal("expected error on 401")
	}
}
