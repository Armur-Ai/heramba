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

func TestSplunkSinkPostsHECEnvelope(t *testing.T) {
	var got map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSplunk(srv.URL, "tok-123", time.Second, event.SeverityLow)
	if err := s.Emit(event.Deception(event.KindDecoyHit, "1.2.3.4", "/.env", "d")); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if auth != "Splunk tok-123" {
		t.Errorf("bad auth header: %q", auth)
	}
	if got["sourcetype"] != "heramba" {
		t.Errorf("missing HEC sourcetype: %v", got)
	}
	if _, ok := got["event"]; !ok {
		t.Error("HEC envelope missing 'event'")
	}
}

func TestSplunkSinkSeverityFloor(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	s := NewSplunk(srv.URL, "t", time.Second, event.SeverityHigh)
	_ = s.Emit(event.New(event.KindProbe, event.SeverityLow, "x", "/x", "d"))
	if calls != 0 {
		t.Fatal("below-floor event must not post")
	}
	_ = s.Emit(event.New(event.KindEgressBlocked, event.SeverityHigh, "x", "y", "d"))
	if calls != 1 {
		t.Fatal("at-floor event must post")
	}
}

func TestSplunkSinkErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	s := NewSplunk(srv.URL, "t", time.Second, event.SeverityInfo)
	if err := s.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "x", "y", "d")); err == nil {
		t.Fatal("expected error on 403")
	}
}
