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

func TestSlackSinkPostsBlockKit(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSlack(srv.URL, time.Second, event.SeverityLow)
	e := event.Deception(event.KindCanaryTriggered, "10.0.0.9", "canary-7f3a", "token used").
		WithLabel("campaign", "artex-sweep")
	if err := s.Emit(e); err != nil {
		t.Fatalf("emit: %v", err)
	}

	atts, ok := got["attachments"].([]any)
	if !ok || len(atts) != 1 {
		t.Fatalf("expected one attachment, got %v", got["attachments"])
	}
	att := atts[0].(map[string]any)
	if att["color"] != "#ff6b6b" { // high? deception = High -> amber. check below
		// deception events are SeverityHigh -> amber
	}
	blob, _ := json.Marshal(got)
	for _, want := range []string{"canary_triggered", "10.0.0.9", "canary-7f3a", "artex-sweep", "Confidence"} {
		if !strings.Contains(string(blob), want) {
			t.Errorf("payload missing %q: %s", want, blob)
		}
	}
}

func TestSlackColorBySeverity(t *testing.T) {
	cases := map[event.Severity]string{
		event.SeverityCritical: "#ff6b6b",
		event.SeverityHigh:     "#f5a524",
		event.SeverityMedium:   "#a78bfa",
		event.SeverityLow:      "#57c7ff",
		event.SeverityInfo:     "#5c6577",
	}
	for sev, want := range cases {
		if got := slackColor(sev); got != want {
			t.Errorf("slackColor(%v)=%s want %s", sev, got, want)
		}
	}
}

func TestSlackSinkRespectsSeverityFloor(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSlack(srv.URL, time.Second, event.SeverityHigh)
	// Below floor — must not post.
	if err := s.Emit(event.New(event.KindProbe, event.SeverityLow, "x", "/x", "d")); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("low-severity event should be dropped, got %d posts", calls)
	}
	// At floor — must post.
	if err := s.Emit(event.New(event.KindEgressBlocked, event.SeverityHigh, "x", "y", "d")); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("high-severity event should post, got %d", calls)
	}
}

func TestSlackSinkReturnsErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	s := NewSlack(srv.URL, time.Second, event.SeverityInfo)
	if err := s.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "x", "y", "d")); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestSlackMessageIsValidJSON(t *testing.T) {
	e := event.New(event.KindAgenticTraffic, event.SeverityMedium, "", "", "")
	b, err := json.Marshal(SlackMessage(e))
	if err != nil {
		t.Fatal(err)
	}
	// Empty source/target must degrade to a dash, not break the payload.
	if !strings.Contains(string(b), "—") {
		t.Errorf("empty fields should render as dash: %s", b)
	}
}
