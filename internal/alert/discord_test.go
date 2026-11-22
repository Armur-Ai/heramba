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

func TestDiscordSinkPostsEmbed(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusNoContent) // Discord replies 204
	}))
	defer srv.Close()

	s := NewDiscord(srv.URL, time.Second, event.SeverityLow)
	e := event.Deception(event.KindCanaryTriggered, "10.0.0.9", "canary-7f3a", "token used").
		WithLabel("campaign", "artex-sweep")
	if err := s.Emit(e); err != nil {
		t.Fatalf("emit: %v", err)
	}

	embeds, ok := got["embeds"].([]any)
	if !ok || len(embeds) != 1 {
		t.Fatalf("expected one embed, got %v", got["embeds"])
	}
	blob, _ := json.Marshal(got)
	for _, want := range []string{"canary_triggered", "10.0.0.9", "canary-7f3a", "artex-sweep", "Confidence"} {
		if !strings.Contains(string(blob), want) {
			t.Errorf("payload missing %q: %s", want, blob)
		}
	}
}

func TestDiscordColorBySeverity(t *testing.T) {
	cases := map[event.Severity]int{
		event.SeverityCritical: 0xff6b6b,
		event.SeverityHigh:     0xf5a524,
		event.SeverityMedium:   0xa78bfa,
		event.SeverityLow:      0x57c7ff,
		event.SeverityInfo:     0x5c6577,
	}
	for sev, want := range cases {
		if got := discordColor(sev); got != want {
			t.Errorf("discordColor(%v)=%d want %d", sev, got, want)
		}
	}
}

func TestDiscordSinkRespectsSeverityFloor(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	s := NewDiscord(srv.URL, time.Second, event.SeverityHigh)
	if err := s.Emit(event.New(event.KindProbe, event.SeverityLow, "x", "/x", "d")); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("low-severity event should be dropped, got %d posts", calls)
	}
	if err := s.Emit(event.New(event.KindEgressBlocked, event.SeverityHigh, "x", "y", "d")); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("high-severity event should post, got %d", calls)
	}
}

func TestDiscordSinkReturnsErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	s := NewDiscord(srv.URL, time.Second, event.SeverityInfo)
	if err := s.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "x", "y", "d")); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestDiscordMessageIsValidJSON(t *testing.T) {
	e := event.New(event.KindAgenticTraffic, event.SeverityMedium, "", "", "")
	b, err := json.Marshal(DiscordMessage(e))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "—") {
		t.Errorf("empty fields should render as dash: %s", b)
	}
}
