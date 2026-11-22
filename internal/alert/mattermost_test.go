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

func TestMattermostSinkPostsAttachment(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewMattermost(srv.URL, time.Second, event.SeverityLow)
	e := event.Deception(event.KindCanaryTriggered, "10.0.0.4", "hrmb_z", "used").WithLabel("campaign", "sweep")
	if err := s.Emit(e); err != nil {
		t.Fatalf("emit: %v", err)
	}
	atts, ok := got["attachments"].([]any)
	if !ok || len(atts) != 1 {
		t.Fatalf("expected one attachment, got %v", got["attachments"])
	}
	blob, _ := json.Marshal(got)
	for _, want := range []string{"canary_triggered", "10.0.0.4", "sweep", "#ff6b6b", "heramba"} {
		if !strings.Contains(string(blob), want) {
			t.Errorf("payload missing %q: %s", want, blob)
		}
	}
}

func TestMattermostColorBySeverity(t *testing.T) {
	cases := map[event.Severity]string{
		event.SeverityCritical: "#ff6b6b",
		event.SeverityHigh:     "#f5a524",
		event.SeverityMedium:   "#a78bfa",
		event.SeverityLow:      "#57c7ff",
		event.SeverityInfo:     "#5c6577",
	}
	for sev, want := range cases {
		if got := mattermostColor(sev); got != want {
			t.Errorf("mattermostColor(%v)=%s want %s", sev, got, want)
		}
	}
}

func TestMattermostSinkFloorAndError(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	s := NewMattermost(srv.URL, time.Second, event.SeverityHigh)
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
	s2 := NewMattermost(bad.URL, time.Second, event.SeverityInfo)
	if err := s2.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "x", "y", "d")); err == nil {
		t.Fatal("expected error on 500")
	}
}
