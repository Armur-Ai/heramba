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

func TestBetterStackSinkPostsLog(t *testing.T) {
	var got map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	s := NewBetterStack(srv.URL, "bs-source-token", time.Second, event.SeverityLow)
	e := event.Deception(event.KindCanaryTriggered, "10.0.0.2", "hrmb_x", "used")
	if err := s.Emit(e); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if auth != "Bearer bs-source-token" {
		t.Errorf("Authorization = %q, want Bearer bs-source-token", auth)
	}
	if got["dt"] == nil {
		t.Error("payload should carry a dt timestamp field")
	}
	blob, _ := json.Marshal(got)
	for _, want := range []string{"canary_triggered", "10.0.0.2", "message"} {
		if !strings.Contains(string(blob), want) {
			t.Errorf("payload missing %q: %s", want, blob)
		}
	}
}

func TestBetterStackSinkFloorAndError(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	s := NewBetterStack(srv.URL, "t", time.Second, event.SeverityHigh)
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
	s2 := NewBetterStack(bad.URL, "t", time.Second, event.SeverityInfo)
	if err := s2.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "x", "y", "d")); err == nil {
		t.Fatal("expected error on 401")
	}
}
