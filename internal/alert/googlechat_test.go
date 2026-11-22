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

func TestGoogleChatSinkPostsText(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewGoogleChat(srv.URL, time.Second, event.SeverityLow)
	e := event.Deception(event.KindDecoyHit, "10.0.0.3", "/.git/config", "probe").WithLabel("campaign", "sweep")
	if err := s.Emit(e); err != nil {
		t.Fatalf("emit: %v", err)
	}
	text, _ := got["text"].(string)
	for _, want := range []string{"decoy_hit", "10.0.0.3", "/.git/config", "campaign: sweep"} {
		if !strings.Contains(text, want) {
			t.Errorf("message missing %q: %s", want, text)
		}
	}
}

func TestGoogleChatSinkFloorAndError(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	s := NewGoogleChat(srv.URL, time.Second, event.SeverityHigh)
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
	s2 := NewGoogleChat(bad.URL, time.Second, event.SeverityInfo)
	if err := s2.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "x", "y", "d")); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestGoogleChatMessageHandlesEmptyFields(t *testing.T) {
	b, err := json.Marshal(GoogleChatMessage(event.New(event.KindAgenticTraffic, event.SeverityMedium, "", "", "")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "—") {
		t.Errorf("empty fields should render as dash: %s", b)
	}
}
