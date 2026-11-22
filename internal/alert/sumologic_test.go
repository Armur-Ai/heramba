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

func TestSumoLogicSinkPostsJSON(t *testing.T) {
	var got event.Event
	var category string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		category = r.Header.Get("X-Sumo-Category")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSumoLogic(srv.URL, time.Second, event.SeverityLow)
	e := event.Deception(event.KindCanaryTriggered, "10.0.0.2", "hrmb_x", "used")
	if err := s.Emit(e); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if got.Source != "10.0.0.2" || got.Kind != event.KindCanaryTriggered {
		t.Errorf("sumo did not receive the event: %+v", got)
	}
	if !strings.Contains(category, "heramba") {
		t.Errorf("missing X-Sumo-Category: %q", category)
	}
}

func TestSumoLogicSinkFloorAndError(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	s := NewSumoLogic(srv.URL, time.Second, event.SeverityHigh)
	if err := s.Emit(event.New(event.KindProbe, event.SeverityLow, "x", "/x", "d")); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("below-floor event should be dropped, got %d", calls)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer bad.Close()
	s2 := NewSumoLogic(bad.URL, time.Second, event.SeverityInfo)
	if err := s2.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "x", "y", "d")); err == nil {
		t.Fatal("expected error on 502")
	}
}
